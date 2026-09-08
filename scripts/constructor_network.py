"""Verify passive constructors with an enforced Linux socket restriction."""

import hashlib
import json
import os
import re
import subprocess
import tempfile
import uuid
from pathlib import Path


CONSTRUCTORS = ["New", "NewContext", "NewContextMemory", "NewContextFilesystem"]
NETWORK_PROFILE = {
    "defaultAction": "SCMP_ACT_ALLOW",
    "syscalls": [{
        "names": ["socket", "socketpair", "socketcall", "connect", "accept", "accept4",
                  "bind", "listen", "sendto", "sendmsg", "sendmmsg", "recvfrom",
                  "recvmsg", "recvmmsg", "io_uring_setup", "io_uring_enter", "io_uring_register"],
        "action": "SCMP_ACT_KILL_PROCESS",
    }],
}


def classify(read_only, attempted, arch):
    """Require successful construction and a terminated positive control."""
    for observation in [read_only, attempted]:
        if not isinstance(observation.get("state"), dict):
            return "UNVERIFIED", "The container state is missing."
        if observation["state"].get("OOMKilled") or observation["state"].get("Error"):
            return "UNVERIFIED", "The container did not complete under normal resource conditions."
    if attempted["exit_code"] != 159 or attempted["state"].get("ExitCode") != 159:
        return "UNVERIFIED", "The positive control did not terminate with SIGSYS."
    try:
        control = json.loads(attempted["stdout"])
    except json.JSONDecodeError:
        return "UNVERIFIED", "The probe did not return complete structured evidence."
    if control != {"phase": "network-attempt"}:
        return "UNVERIFIED", "The positive control did not reach its socket attempt."
    if read_only["exit_code"] != 0 or read_only["state"].get("ExitCode") != 0:
        return "FAIL", "Passive construction failed under the enforced socket restriction."
    try:
        result = json.loads(read_only["stdout"])
    except json.JSONDecodeError:
        return "UNVERIFIED", "The constructors did not return structured evidence."
    if not isinstance(result, dict):
        return "UNVERIFIED", "The constructor result is not an object."
    if (result.get("constructors") != CONSTRUCTORS or result.get("os") != "linux"
            or result.get("arch") != arch or type(result.get("files_after")) is not int
            or result["files_after"] != 0 or not isinstance(result.get("generation_id"), str)
            or not result["generation_id"] or not isinstance(result.get("payload_checksum"), str)
            or not re.fullmatch(r"sha256:[0-9a-f]{64}", result["payload_checksum"])):
        return "FAIL", "The constructor results do not satisfy the passive baseline contract."
    return "PASS", "Four constructors retained the baseline without a socket attempt or local initialization."


def verify(root):
    """Build the current repository and execute both controls in fresh containers."""
    root = Path(root)
    commands = []
    containers = []
    image = "starmap-constructor-probe:" + uuid.uuid4().hex
    image_attempted = False
    result = {"status": "UNVERIFIED", "qualification": "Linux component evidence only", "commands": commands}

    def command(args, *, env=None, timeout=180):
        try:
            completed = subprocess.run(args, cwd=root, env=env, capture_output=True, text=True, timeout=timeout)
            observation = {"command": args, "exit_code": completed.returncode,
                           "stdout": completed.stdout, "stderr": completed.stderr}
        except (OSError, subprocess.TimeoutExpired) as error:
            observation = {"command": args, "exit_code": None, "stdout": "", "stderr": str(error)}
        commands.append(observation)
        return observation

    def required(args, **kwargs):
        observation = command(args, **kwargs)
        if observation["exit_code"] != 0:
            raise RuntimeError("A required build or container operation failed.")
        return observation

    try:
        server = json.loads(required(["docker", "version", "--format", "{{json .Server}}"])["stdout"])
        arch = server.get("Arch")
        if server.get("Os") != "linux" or arch not in ["amd64", "arm64"]:
            raise RuntimeError("The probe requires a native Linux AMD64 or ARM64 Docker engine.")
        result["server"] = server
        with tempfile.TemporaryDirectory(prefix="starmap-constructor-network-") as temporary:
            directory = Path(temporary)
            profile = directory / "network.json"
            profile.write_text(json.dumps(NETWORK_PROFILE))
            result["profile"] = NETWORK_PROFILE
            binary = directory / "probe"
            environment = dict(os.environ, GOOS="linux", GOARCH=arch, CGO_ENABLED="0", GOTOOLCHAIN="go1.25.12",
                               GOWORK="off", GOFLAGS="")
            required(["go", "build", "-mod=readonly", "-trimpath", "-o", str(binary), "./scripts/testdata/constructor-probe"],
                     env=environment, timeout=300)
            result["binary_sha256"] = hashlib.sha256(binary.read_bytes()).hexdigest()
            result["build_info"] = required(["go", "version", "-m", str(binary)])["stdout"]
            (directory / "Dockerfile").write_text("FROM scratch\nCOPY probe /probe\nENTRYPOINT [\"/probe\"]\n")
            image_attempted = True
            required(["docker", "build", "--platform", "linux/" + arch, "--network", "none", "--tag", image, str(directory)], timeout=300)
            observations = {}
            for mode in ["read-only", "network-attempt"]:
                name = "starmap-constructor-" + uuid.uuid4().hex
                containers.append(name)
                required(["docker", "create", "--platform", "linux/" + arch, "--name", name, "--network", "none", "--read-only",
                          "--cap-drop", "ALL", "--security-opt", "no-new-privileges=true",
                          "--security-opt", "seccomp=" + str(profile), "--user", "65532:65532",
                          "--tmpfs", "/work:rw,nosuid,nodev,noexec,size=16m,mode=0700,uid=65532,gid=65532",
                          "--memory", "512m", "--cpus", "2", "--pids-limit", "64", "--ulimit", "core=0",
                          "--env", "HOME=/work", "--env", "STARMAP_HOME=/work",
                          "--env", "STARMAP_CATALOG_SOURCE=starmap",
                          "--env", "STARMAP_CATALOG_SOURCE_URL=http://192.0.2.1/catalog",
                          "--env", "STARMAP_CATALOG_ACQUISITION_ENABLED=true",
                          "--env", "OPENAI_API_KEY=not-a-real-key", image, mode])
                observation = command(["docker", "start", "--attach", name])
                inspection = json.loads(required(["docker", "inspect", name])["stdout"])[0]
                observation["state"] = inspection["State"]
                host = inspection["HostConfig"]
                if host["NetworkMode"] != "none" or not host["ReadonlyRootfs"] or host["Privileged"]:
                    raise RuntimeError("The container did not retain its required isolation settings.")
                observations[mode] = observation
            result["observations"] = observations
            result["status"], result["reason"] = classify(observations["read-only"], observations["network-attempt"], arch)
    except (OSError, subprocess.TimeoutExpired, RuntimeError, ValueError, KeyError, IndexError, TypeError) as error:
        result["reason"] = str(error)
    finally:
        cleanup = []
        for name in containers:
            cleanup.append(command(["docker", "rm", "--force", name], timeout=30))
        if image_attempted:
            cleanup.append(command(["docker", "image", "rm", image], timeout=30))
        result["cleanup_complete"] = all(item["exit_code"] == 0 for item in cleanup)
        if not result["cleanup_complete"]:
            result["status"], result["reason"] = "FAIL", "Probe cleanup did not complete."
    return result


if __name__ == "__main__":
    report = verify(Path(__file__).resolve().parents[1])
    print(json.dumps(report, indent=2))
    raise SystemExit(0 if report["status"] == "PASS" else 1)
