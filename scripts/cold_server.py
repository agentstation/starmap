"""Verify persistent offline startup through the actual Starmap HTTP server."""

import hashlib
import json
import os
import re
import subprocess
import tempfile
import time
import uuid
from pathlib import Path
from urllib.parse import quote


class ServerFailure(RuntimeError):
    """The server failed inside an available isolated environment."""


def classify(observations):
    """Require two isolated processes with the same valid saved and served catalog."""
    if not isinstance(observations, list) or len(observations) != 2:
        return "UNVERIFIED", "Both startup observations are required."
    try:
        identities = []
        for phase, item in zip(["cold", "restart"], observations):
            isolation = item["isolation"]
            if (item["phase"] != phase or isolation["network_mode"] != "none"
                    or isolation["user"] != "65532:65532" or isolation["readonly_rootfs"] is not True
                    or isolation["privileged"] is not False or not {"HOME", "STARMAP_HOME"} <= set(isolation["environment_names"])
                    or not set(isolation["environment_names"]) <= {"HOME", "STARMAP_HOME", "PATH"}
                    or not isinstance(isolation["volume"], str) or not isolation["volume"] or item["network"].get("denied") is not True):
                return "UNVERIFIED", "The offline environment did not retain its required isolation."
            state = item["exit_state"]
            if state.get("OOMKilled") or state.get("Error"):
                return "UNVERIFIED", "The server did not complete under normal resource conditions."
            if state["ExitCode"] != 0 or state["Running"]:
                return "FAIL", "The server did not stop normally."
            if any(item[name]["status_code"] != 200 for name in ["ready", "manifest", "payload"]):
                return "FAIL", "A required server endpoint was unavailable."
            runtime = item["ready"]["body"]["data"]["runtime"]
            if runtime["usable"] is not True or runtime["source_kind"] != "public" or runtime["fallback"] is not True:
                return "FAIL", "Default public-source startup did not serve its offline baseline."
            served = item["manifest"]
            saved = item["baseline_manifest"]
            if served.get("manifest_validated") is not True or saved.get("manifest_validated") is not True:
                return "UNVERIFIED", "Both manifests need schema validation."
            manifest = served["body"]
            generation = manifest["generation_id"]
            checksum = manifest["payload"]["checksum"]
            size = manifest["payload"]["size_bytes"]
            if (not isinstance(generation, str) or not generation or served["generation_header"] != generation
                    or runtime["generation_id"] != generation or not isinstance(checksum, str)
                    or not re.fullmatch(r"sha256:[0-9a-f]{64}", checksum)
                    or type(size) is not int or size <= 0 or manifest != saved["body"]):
                return "FAIL", "The served and saved manifests do not identify the same valid generation."
            for name in ["payload", "baseline_payload"]:
                payload = item[name]
                if payload["checksum"] != checksum or type(payload["bytes"]) is not int or payload["bytes"] != size:
                    return "FAIL", "Catalog bytes do not match the manifest."
            if saved["mode"] != "0600" or item["baseline_payload"]["mode"] != "0600":
                return "FAIL", "The saved baseline files are not private."
            if item["payload"]["generation_header"] != generation:
                return "FAIL", "The payload endpoint returned a different generation."
            identity = runtime["instance_identity"]
            if not isinstance(identity, str) or not identity:
                return "UNVERIFIED", "The runtime instance identity is missing."
            identities.append((manifest, identity, isolation["volume"]))
        if identities[0] != identities[1]:
            return "FAIL", "Restart changed the catalog, instance identity, or persistent volume."
    except (KeyError, TypeError, AttributeError):
        return "UNVERIFIED", "The server observations are incomplete."
    return "PASS", "Two fresh offline server processes retained the same private baseline and served digest."


def verify(root):
    """Build both binaries and exercise cold startup and restart in fresh containers."""
    root = Path(root)
    suffix = uuid.uuid4().hex
    image, volume = "starmap-cold-server:" + suffix, "starmap-cold-server-" + suffix
    containers, commands, observations = [], [], []
    image_attempted = volume_attempted = False
    report = {"status": "UNVERIFIED", "qualification": "Linux component evidence only",
              "commands": commands, "observations": observations}

    def command(args, *, timeout=120, env=None):
        try:
            result = subprocess.run(args, cwd=root, env=env, capture_output=True, text=True, timeout=timeout)
            value = {"command": args, "exit_code": result.returncode, "stdout": result.stdout, "stderr": result.stderr}
        except (OSError, subprocess.TimeoutExpired) as error:
            value = {"command": args, "exit_code": None, "stdout": "", "stderr": str(error)}
        commands.append(value)
        return value

    def required(args, **kwargs):
        value = command(args, **kwargs)
        if value["exit_code"] != 0:
            raise RuntimeError("A required build or container operation failed.")
        return value

    def inspect(name):
        return json.loads(required(["docker", "inspect", name])["stdout"])[0]

    try:
        server = json.loads(required(["docker", "version", "--format", "{{json .Server}}"])["stdout"])
        arch = server["Arch"]
        if server["Os"] != "linux" or arch not in ["amd64", "arm64"]:
            raise RuntimeError("The check requires a native Linux AMD64 or ARM64 Docker engine.")
        report["server"] = server
        with tempfile.TemporaryDirectory(prefix="starmap-cold-server-") as temporary:
            directory = Path(temporary)
            environment = dict(os.environ, GOOS="linux", GOARCH=arch, CGO_ENABLED="0", GOTOOLCHAIN="go1.25.12", GOWORK="off", GOFLAGS="")
            for binary, package in [("starmap", "./cmd/starmap"), ("observe", "./scripts/testdata/cold-server-probe")]:
                target = directory / binary
                required(["go", "build", "-mod=readonly", "-trimpath", "-o", str(target), package], env=environment, timeout=300)
                report[binary + "_sha256"] = hashlib.sha256(target.read_bytes()).hexdigest()
                report[binary + "_build_info"] = required(["go", "version", "-m", str(target)])["stdout"]
            (directory / "home").mkdir(mode=0o700)
            (directory / "Dockerfile").write_text(
                'FROM scratch\nCOPY starmap /starmap\nCOPY observe /observe\n'
                'COPY --chown=65532:65532 --chmod=0700 home/ /work/\nUSER 65532:65532\n'
                'ENTRYPOINT ["/starmap", "serve", "--host", "127.0.0.1", "--port", "8080"]\n')
            image_attempted = True
            required(["docker", "build", "--platform", "linux/" + arch, "--network", "none", "--tag", image, str(directory)], timeout=300)
            volume_attempted = True
            required(["docker", "volume", "create", volume])
            for phase in ["cold", "restart"]:
                name = "starmap-cold-server-" + phase + "-" + suffix
                containers.append(name)
                required(["docker", "create", "--name", name, "--platform", "linux/" + arch, "--network", "none", "--read-only",
                          "--cap-drop", "ALL", "--security-opt", "no-new-privileges=true", "--user", "65532:65532",
                          "--memory", "512m", "--cpus", "2", "--pids-limit", "64",
                          "--mount", "type=volume,src=" + volume + ",dst=/work", "--env", "HOME=/work", "--env", "STARMAP_HOME=/work/product", image])
                required(["docker", "start", name])
                deadline = time.monotonic() + 45
                while True:
                    attempt = command(["docker", "exec", name, "/observe", "http", "http://127.0.0.1:8080/api/v1/ready"], timeout=10)
                    if attempt["exit_code"] == 0:
                        ready = json.loads(attempt["stdout"])
                        if ready["status_code"] == 200:
                            break
                    if not inspect(name)["State"]["Running"] or time.monotonic() >= deadline:
                        raise ServerFailure("The server did not become ready within 45 seconds.")
                    time.sleep(0.25)

                def observe(mode, target):
                    value = command(["docker", "exec", name, "/observe", mode, target], timeout=15)
                    if value["exit_code"] != 0:
                        raise ServerFailure("A required server or filesystem observation failed.")
                    return json.loads(value["stdout"])

                item = {"phase": phase, "ready": ready, "network": observe("network", "192.0.2.1:9")}
                item["manifest"] = observe("http", "http://127.0.0.1:8080/api/v1/catalog/manifest")
                generation = item["manifest"]["generation_header"]
                item["payload"] = observe("http", "http://127.0.0.1:8080/api/v1/catalog/generations/" + quote(generation, safe="") + "/payload")
                baseline = "/work/product/data/catalog/baseline/" + hashlib.sha256(generation.encode()).hexdigest() + "/"
                item["baseline_manifest"] = observe("file", baseline + "manifest.json")
                item["baseline_payload"] = observe("file", baseline + "catalog.json")
                inspection = inspect(name)
                mounts = [mount for mount in inspection["Mounts"] if mount["Destination"] == "/work" and mount["Type"] == "volume"]
                if len(mounts) != 1 or mounts[0]["Name"] != volume:
                    raise RuntimeError("The server did not retain its private test volume.")
                host = inspection["HostConfig"]
                item["isolation"] = {"network_mode": host["NetworkMode"], "readonly_rootfs": host["ReadonlyRootfs"],
                                     "privileged": host["Privileged"], "user": inspection["Config"]["User"], "volume": volume,
                                     "environment_names": [entry.split("=", 1)[0] for entry in inspection["Config"]["Env"]]}
                required(["docker", "stop", "--time", "15", name], timeout=30)
                item["exit_state"] = inspect(name)["State"]
                observations.append(item)
            report["status"], report["reason"] = classify(observations)
    except ServerFailure as error:
        report["status"], report["reason"] = "FAIL", str(error)
    except (RuntimeError, OSError, ValueError, KeyError, IndexError, TypeError, AttributeError) as error:
        report["reason"] = str(error)
    finally:
        cleanup = []
        report["server_logs"] = [command(["docker", "logs", name], timeout=15) for name in containers]
        for name in containers:
            cleanup.append(command(["docker", "rm", "--force", name], timeout=30))
        if volume_attempted:
            cleanup.append(command(["docker", "volume", "rm", volume], timeout=30))
        if image_attempted:
            cleanup.append(command(["docker", "image", "rm", image], timeout=30))
        report["cleanup_complete"] = all(item["exit_code"] == 0 for item in cleanup)
        if not report["cleanup_complete"]:
            report["status"], report["reason"] = "FAIL", "Offline server cleanup did not complete."
    return report


if __name__ == "__main__":
    result = verify(Path(__file__).resolve().parents[1])
    print(json.dumps(result, indent=2))
    raise SystemExit(0 if result["status"] == "PASS" else 1)
