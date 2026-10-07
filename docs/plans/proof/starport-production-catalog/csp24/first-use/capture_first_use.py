#!/usr/bin/env python3
"""Capture native first-use installation evidence for one Starport release without paid inference or credentials."""
import argparse
import base64
import hashlib
import json
import os
import re
import secrets
import shutil
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
from pathlib import Path

REPO = "agentstation/starport"
TAG = "v1.3.0"
VERSION = TAG[1:]
SOURCE_COMMIT = "b649dbf5184e53b2c13449b96fc887bcb144e18c"
RELEASE_RUN = "37437368384"
NATIVE_RUN = "37475148346"
NATIVE_COMMIT = "0af089d3195cb5ee5b05e734a3019b8afc098d11"
CLONE_URL = "https://github.com/agentstation/starport.git"
IMAGE = "ghcr.io/agentstation/starport:" + VERSION
SIGNER = "agentstation/starport/.github/workflows/release.yaml"
PLATFORM = ["docker", "version", "--format", "{{.Server.Os}}/{{.Server.Arch}}"]
PROJECT = "starport-csp24-compose"
RESERVED_PORTS = range(54806, 54812)
CATALOG_ARGS = [["models", "search", "gpt-4o", "--json"], ["models", "show", "openai/gpt-4o-mini", "--json"]]


def capture(command, **kwargs):
    result = subprocess.run(command, capture_output=True, text=True, **kwargs)
    return {"command": command, "exit_code": result.returncode, "stdout": result.stdout, "stderr": result.stderr}


def write(output, name, record):
    (output / name).write_text(json.dumps(record, indent=2) + "\n")
    print(f"{name}: {record.get('verdict')}" + (f" ({record['error']})" if record.get("error") else ""))


def sha256(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def isolated_env(home):
    """Return a minimal environment. No provider credential passes through."""
    return {"PATH": "/usr/bin:/bin:/usr/sbin:/sbin", "HOME": str(home), "TMPDIR": str(home / "tmp"),
            "XDG_CONFIG_HOME": str(home / ".config"), "XDG_STATE_HOME": str(home / ".local/state"),
            "XDG_DATA_HOME": str(home / ".local/share"), "XDG_CACHE_HOME": str(home / ".cache")}


class Recorder:
    """Run commands and record exit codes and timing. The record keeps only a short summary of stdout."""

    def __init__(self):
        self.commands = []

    def run(self, command, timeout=600, **kwargs):
        started = time.monotonic()
        result = subprocess.run(command, capture_output=True, text=True, timeout=timeout, **kwargs)
        entry = {"command": command, "exit_code": result.returncode,
                 "elapsed_seconds": round(time.monotonic() - started, 3)}
        self.commands.append(entry)
        if result.returncode:
            raise RuntimeError(f"command failed with exit {result.returncode}: {' '.join(command)}: "
                               + result.stderr.strip()[-400:])
        entry.update(summarize(command, result.stdout) or {})
        return result.stdout


def summarize(command, stdout):
    """Keep the version line and the model identity instead of large catalog output."""
    if "--version" in command:
        return {"version": stdout.strip().splitlines()[0]}
    if command[-4:-1] == ["models", "search", "gpt-4o"]:
        ids = [model.get("id") for model in json.loads(stdout).get("data", [])]
        return {"result_count": len(ids), "includes_model": "openai/gpt-4o" in ids}
    if command[-4:-1] == ["models", "show", "openai/gpt-4o-mini"]:
        return {"model_id": json.loads(stdout).get("id")}


def check_catalog(recorder, version_line, version=VERSION):
    by_kind = {tuple(c["command"][-4:-1]): c for c in recorder.commands}
    assert version_line == f"starport version {version}", f"unexpected version: {version_line!r}"
    assert by_kind[("models", "search", "gpt-4o")]["includes_model"], "search omits openai/gpt-4o"
    assert by_kind[("models", "show", "openai/gpt-4o-mini")]["model_id"] == "openai/gpt-4o-mini", "show id mismatch"


def run_binary(recorder, binary, home):
    env = isolated_env(home)
    (home / "tmp").mkdir(parents=True, exist_ok=True)
    version = recorder.run([str(binary), "--version"], env=env)
    for args in CATALOG_ARGS:
        recorder.run([str(binary)] + args, env=env)
    return version.strip().splitlines()[0]


def user_files(home):
    return sorted(str(p.relative_to(home)) for p in home.rglob("*") if p.is_file() and p.parent != home / "tmp")


def installation(output, name, record, body):
    """Run one installation method and record its commands and verdict, also on failure."""
    recorder, error = Recorder(), None
    try:
        body(record, recorder)
    except Exception as exc:  # Record every failure in the capture file.
        error = exc
    record["commands"] = recorder.commands
    write(output, name, finish(record, error))


def finish(record, error=None):
    """Record PASS only when every command exited 0 and every assertion held."""
    record["verdict"] = "FAIL" if error else "PASS"
    if error:
        record["error"] = str(error) or type(error).__name__
    return record


def checked(output, name, record, check):
    try:
        assert record["exit_code"] == 0, " ".join(record["command"][:3]) + " failed"
        check(record, json.loads(record["stdout"]))
        error = None
    except (AssertionError, KeyError, ValueError, OSError) as exc:
        error = exc
    write(output, name, finish(record, error))


def step_release(output):
    def release(record, data):
        assert data["tagName"] == TAG and not data["isDraft"] and not data["isPrerelease"], "release identity"
        assert any(a["name"] == "checksums.txt" for a in data["assets"]), "checksums.txt asset missing"

    def release_run(record, data):
        assert data["headSha"] == SOURCE_COMMIT, f"release run headSha {data['headSha']}"
        conclusions = {job["name"]: job["conclusion"] for job in data["jobs"]}
        for name in ("Verify Homebrew (macos-latest)", "Verify Homebrew (ubuntu-latest)"):
            assert conclusions.get(name) == "success", f"{name}: {conclusions.get(name)}"

    def cask(record, data):
        text = base64.b64decode(data["content"]).decode()
        assert f'version "{VERSION}"' in text, "cask version is not " + VERSION
        urls = re.findall(r'url "([^"]+)"', text)
        assert urls and all("/download/v#{version}/" in url for url in urls), "a cask url does not use v#{version}"
        match = re.search(r'sha256 "([0-9a-f]{64})"\s+url "[^"]*_darwin_arm64\.tar\.gz"', text)
        assert match, "cask has no darwin_arm64 sha256"
        download = capture(["gh", "release", "download", TAG, "--repo", REPO, "--pattern", "checksums.txt", "-O", "-"])
        assert download["exit_code"] == 0, "checksums.txt download failed"
        published = [line.split()[0] for line in download["stdout"].splitlines()
                     if line.split()[-1:] == [f"starport_{VERSION}_darwin_arm64.tar.gz"]]
        assert published == [match.group(1)], "cask darwin_arm64 sha256 differs from checksums.txt"
        record.update({"cask_version": VERSION, "darwin_arm64_sha256": match.group(1)})

    checked(output, "release.json", capture(["gh", "release", "view", TAG, "--repo", REPO, "--json",
                                             "assets,tagName,targetCommitish,publishedAt,url,isDraft,isPrerelease"]),
            release)
    checked(output, "release-run.json",
            capture(["gh", "run", "view", RELEASE_RUN, "--repo", REPO, "--json", "jobs,headSha,url"]), release_run)
    checked(output, "homebrew-cask.json",
            capture(["gh", "api", "repos/agentstation/homebrew-tap/contents/Casks/starport.rb"]), cask)

    review = {"scope": "", "head_commit": NATIVE_COMMIT, "event": "workflow_dispatch", "tag": TAG,
              "run_url": f"https://github.com/{REPO}/actions/runs/{NATIVE_RUN}", "results": []}
    try:
        run = capture(["gh", "run", "view", NATIVE_RUN, "--repo", REPO, "--json", "event,headSha,conclusion"])
        assert run["exit_code"] == 0, "gh run view of the native run failed"
        data = json.loads(run["stdout"])
        assert (data["event"], data["headSha"], data["conclusion"]) == (
            "workflow_dispatch", NATIVE_COMMIT, "success"), f"native run identity {data}"
        paths = sorted((output / f"native-run-{NATIVE_RUN}").glob("*/result.json"))
        assert paths, "no native result files"
        for path in paths:
            result = json.loads(path.read_text())
            identity = [result[k] for k in ("verdict", "release", "mode", "workflow_run_id", "workflow_commit")]
            assert identity == ["PASS", TAG, "release", NATIVE_RUN, NATIVE_COMMIT], f"{path.parent.name}: {identity}"
            review["results"].append({"artifact": path.parent.name, "archive": result["archive"],
                                      "model_count": result["model_count"]})
        review["scope"] = (f"{len(paths)} native archive records from a release-mode workflow_dispatch run and "
                           "GitHub workflow identity. No paid inference. E02 registration remains separate.")
        error = None
    except (AssertionError, KeyError, ValueError, OSError) as exc:
        error = exc
    write(output, f"native-run-{NATIVE_RUN}-review.json", finish(review, error))


def step_homebrew(output):
    brew = shutil.which("brew", path="/opt/homebrew/bin:/usr/local/bin") or "brew"
    record = {"schema_version": 1, "method": "readme-homebrew", "release": TAG, "platform": "darwin/arm64"}
    recorder = Recorder()
    before = capture([brew, "list", "--cask", "--versions", "starport"])
    record["preexisting_install"] = before["exit_code"] == 0
    if record["preexisting_install"]:
        record["preexisting_version"] = before["stdout"].strip()
    home = Path(tempfile.mkdtemp(prefix="starport-csp24-brew-"))
    error = None
    try:
        if capture([brew, "tap"])["stdout"].split().count("agentstation/tap") == 0:
            recorder.run([brew, "tap", "agentstation/tap"])
        recorder.run([brew, "update"], timeout=900)
        recorder.run([brew, "trust", "--cask", "agentstation/tap/starport"])
        verb = "reinstall" if record["preexisting_install"] else "install"
        recorder.run([brew, verb, "--cask", "agentstation/tap/starport"], timeout=900)
        binary = Path(capture([brew, "--prefix"])["stdout"].strip()) / "bin" / "starport"
        record["binary"] = str(binary)
        record["binary_sha256"] = sha256(binary.resolve())
        record["version"] = run_binary(recorder, binary, home)
        check_catalog(recorder, record["version"])
        record["user_files"] = user_files(home)
    except Exception as exc:  # Record every failure in the capture file.
        error = exc
    finally:
        shutil.rmtree(home, ignore_errors=True)
        if not record["preexisting_install"]:
            removal = capture([brew, "uninstall", "--cask", "starport"])
            record["uninstalled_after_capture"] = removal["exit_code"] == 0
    record["commands"] = recorder.commands
    write(output, "homebrew-macos.json", finish(record, error))


def clone(parent, recorder):
    recorder.run(["git", "clone", "--depth", "1", "--branch", TAG, CLONE_URL, "starport"], cwd=parent)
    root = parent / "starport"
    head = capture(["git", "rev-parse", "HEAD"], cwd=root)["stdout"].strip()
    assert head == SOURCE_COMMIT, f"clone HEAD {head} is not {SOURCE_COMMIT}"
    return root


def step_source(output):
    def body(record, recorder):
        parent = Path(tempfile.mkdtemp(prefix="starport-csp24-source-"))
        try:
            root = clone(parent, recorder)
            recorder.run(["make", "build"], cwd=root, timeout=1800)
            record.update(source_commit=SOURCE_COMMIT, go_mod_sha256=sha256(root / "go.mod"),
                          binary_sha256=sha256(root / "starport"))
            record["version"] = run_binary(recorder, root / "starport", parent / "home")
            # make build stamps `git describe --tags --dirty`, so a clean tag checkout reports the tag itself.
            check_catalog(recorder, record["version"], TAG)
            record["user_files"] = user_files(parent / "home")
        finally:
            shutil.rmtree(parent, ignore_errors=True)

    installation(output, "source-install.json", {
        "schema_version": 1, "method": "readme-source-build", "release": TAG,
        "started_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}, body)


def step_container(output):
    def body(record, recorder):
        record["native_platform"] = capture(PLATFORM)["stdout"].strip()
        recorder.run(["docker", "pull", IMAGE])
        recorder.run(["gh", "attestation", "verify", "oci://" + IMAGE, "--repo", REPO, "--signer-workflow", SIGNER])
        record["version"] = recorder.run(["docker", "run", "--rm", IMAGE, "--version"]).strip().splitlines()[0]
        for args in CATALOG_ARGS:
            recorder.run(["docker", "run", "--rm", "--network", "none", IMAGE] + args)
        check_catalog(recorder, record["version"])
        inspect = capture(["docker", "image", "inspect", "--format", "{{json .RepoDigests}}", IMAGE])
        record["image"] = [d for d in json.loads(inspect["stdout"]) if d.startswith("ghcr.io/agentstation/starport@sha256:")]
        assert record["image"], "pulled image has no repository digest"

    installation(output, "container-install.json",
                 {"schema_version": 1, "method": "readme-versioned-container", "release": TAG}, body)


def free_port():
    while True:
        with socket.socket() as probe:
            probe.bind(("127.0.0.1", 0))
            if probe.getsockname()[1] not in RESERVED_PORTS:
                return probe.getsockname()[1]


def compose_leftovers():
    """List the project's remaining containers, volumes, and networks. A listing failure counts as a leftover."""
    label = f"label=com.docker.compose.project={PROJECT}"
    found = []
    for command in (["docker", "ps", "-a", "-q", "--filter", label], ["docker", "volume", "ls", "-q", "--filter", label],
                    ["docker", "volume", "ls", "-q", "--filter", f"name={PROJECT}"],
                    ["docker", "network", "ls", "-q", "--filter", label]):
        result = capture(command)
        found += result["stdout"].split() if result["exit_code"] == 0 else ["unlisted: " + " ".join(command)]
    return found


def step_compose(output):
    port = free_port()
    after = {"source_commit": SOURCE_COMMIT, "platform": "", "commands": [], "http": []}
    review = {"build_source_commit": SOURCE_COMMIT, "port": port, "provider_credentials_used": False,
              "test_adjustments": [f"Loopback host port {port} replaces host port 8080 through STARPORT_PORT.",
                  "A generated temporary master key in a mode 0600 .env replaces the placeholder.",
                  "Catalog source embedded and acquisition disabled isolate persistence from public egress.",
                  "JSON output keeps generated credentials in driver memory."],
              "proof_scope": "One native Compose process on Docker Desktop. Not replicated storage qualification."}
    parent = Path(tempfile.mkdtemp(prefix="starport-csp24-compose-"))
    env_file, key, error = None, "", None  # A set env_file means a possible project start.

    def compose(args, timeout=300):
        result = subprocess.run(["docker", "compose", "-p", PROJECT, "-f", "docker-compose.yml"] + args, cwd=root,
                                env=compose_env, capture_output=True, text=True, timeout=timeout)
        after["commands"].append({"args": args, "exit_code": result.returncode})
        if result.returncode:
            raise RuntimeError(f"compose {' '.join(args)} failed: {result.stderr.strip()[-400:]}")
        return result.stdout

    def request(path, body=None):
        data = None if body is None else json.dumps(body).encode()
        headers = {"Authorization": "Bearer " + key, **({"Content-Type": "application/json"} if data else {})}
        req = urllib.request.Request(f"http://127.0.0.1:{port}{path}", headers=headers, data=data)
        try:
            with urllib.request.urlopen(req, timeout=5) as response:
                status = response.status
        except urllib.error.HTTPError as http_error:
            status = http_error.code
        after["http"].append({"path": path, "method": "POST" if data else "GET", "status": status, "authenticated": True})
        return status

    def ready():
        for _ in range(240):
            try:
                with urllib.request.urlopen(f"http://127.0.0.1:{port}/health/ready", timeout=1) as response:
                    if response.status == 200:
                        return
            except OSError:  # URLError is an OSError.
                pass
            time.sleep(0.5)
        raise RuntimeError("gateway did not become ready")

    compose_env = {**{k: v for k, v in os.environ.items() if not k.startswith("STARPORT_")}, "STARPORT_PORT": str(port)}
    try:
        root = clone(parent, Recorder())
        after["platform"] = capture(PLATFORM)["stdout"].strip()
        review["recipe_commit"] = capture(["git", "log", "-1", "--format=%H", "--", "docker-compose.yml"],
                                          cwd=root)["stdout"].strip()
        review["inputs"] = {name: sha256(root / name)
                            for name in ("docker-compose.yml", "Dockerfile", ".dockerignore", ".env.example")}
        assert not compose_leftovers(), f"resources for project {PROJECT} already exist"
        env_file = root / ".env"
        env_file.touch(mode=0o600, exist_ok=False)
        env_file.write_text(f"STARPORT_SECURITY_MASTER_KEY={secrets.token_hex(32)}\n"
                            "STARPORT_CATALOG_SOURCE=embedded\nSTARPORT_CATALOG_ACQUISITION_ENABLED=false\n")
        compose(["build", "starport"], timeout=1800)
        key = json.loads(compose(["run", "--rm", "starport", "init", "--configured-storage", "--name",
                                  "primary-admin", "--json"]))["api_key"]
        compose(["run", "--rm", "starport", "auth", "rotate", "--json"])
        compose(["up", "-d", "starport"])
        ready()
        assert request("/api/v1/models") == 200, "models list failed"
        assert request("/api/v1/admin/account-templates",
                       {"id": "compose-persistence", "name": "Compose persistence"}) == 201, "template create failed"
        assert request("/api/v1/admin/account-templates/compose-persistence") == 200, "template read failed"
        compose(["up", "-d", "--force-recreate", "starport"])
        ready()
        assert request("/api/v1/models") == 200, "models list after recreate failed"
        after["relational_record_retained"] = request("/api/v1/admin/account-templates/compose-persistence") == 200
        assert after["relational_record_retained"], "template missing after container replacement"
    except Exception as exc:  # Record every failure in the capture file.
        error = exc
    finally:
        key = ""
        if env_file is not None:
            try:
                compose(["down", "-v", "--rmi", "local", "--remove-orphans"])
            except Exception as exc:  # The teardown failure must not hide the first failure.
                error = error or exc
            env_file.unlink(missing_ok=True)
        review["temporary_credential_file_removed"] = env_file is None or not env_file.exists()
        leftovers = compose_leftovers()
        review["resources_removed"] = not leftovers
        if leftovers:
            error = error or RuntimeError(f"resources remain for project {PROJECT}: {leftovers}")
        shutil.rmtree(parent, ignore_errors=True)
    write(output, "compose-after.json", finish(after, error))
    write(output, "compose-review.json", finish(review, error))


STEPS = {"release": step_release, "homebrew": step_homebrew, "source": step_source,
         "container": step_container, "compose": step_compose}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--step", default="all", choices=["all"] + list(STEPS))
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    started = time.monotonic()
    for name in (STEPS if args.step == "all" else [args.step]):
        STEPS[name](args.output.resolve())
    print(f"elapsed_seconds: {time.monotonic() - started:.1f}")


if __name__ == "__main__":
    main()
