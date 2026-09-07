"""Verify the released macOS archive and an explicitly authorized short inference request."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import signal
import socket
import subprocess
import tarfile
import tempfile
import threading
import time
import urllib.error
import urllib.request


PROOF = Path(__file__).resolve().parent
TAG = "v1.2.0"
ARCHIVE = "starport_1.2.0_darwin_arm64.tar.gz"
BODY = {"model": "openai/gpt-4o-mini", "max_tokens": 32, "stream": True,
        "messages": [{"role": "user", "content": "Hello"}]}


def read_key(path):
    for line in path.read_text().splitlines():
        match = re.match(r"^\s*(?:export\s+)?OPENAI_API_KEY\s*=\s*(.*)$", line)
        if match:
            words = shlex.split(match[1], comments=True)
            if len(words) == 1 and words[0] and "$" not in words[0]:
                return words[0]
    raise ValueError("The selected file has no literal OPENAI_API_KEY.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--authorized-provider-env-file", required=True, type=Path)
    args = parser.parse_args()
    key = read_key(args.authorized_provider_env_file)
    report = {"release": TAG, "platform": "darwin/arm64", "request": BODY,
              "credential_source": str(args.authorized_provider_env_file),
              "credential_name": "OPENAI_API_KEY", "verdict": "UNVERIFIED"}
    secrets = [key]
    process = None
    with tempfile.TemporaryDirectory(prefix="starport-real-inference-") as directory:
        work = Path(directory)
        try:
            subprocess.run(["gh", "release", "download", TAG, "--repo", "agentstation/starport",
                            "--pattern", ARCHIVE, "--pattern", "checksums.txt", "--dir", directory], check=True)
            archive = work / ARCHIVE
            digest = hashlib.sha256(archive.read_bytes()).hexdigest()
            checksums = (work / "checksums.txt").read_text().splitlines()
            assert any(line.split() == [digest, ARCHIVE] for line in checksums)
            release = json.loads(json.loads((PROOF / "release.json").read_text())["stdout"])
            assert any(asset["name"] == ARCHIVE and asset["digest"] == "sha256:" + digest for asset in release["assets"])
            report["archive_sha256"] = digest
            with tarfile.open(archive) as tar:
                tar.extractall(work / "distribution", filter="data")
            binary = work / "distribution/starport"
            report["binary_sha256"] = hashlib.sha256(binary.read_bytes()).hexdigest()
            home = work / "home"
            home.mkdir()
            with socket.socket() as probe:
                probe.bind(("127.0.0.1", 0))
                port = probe.getsockname()[1]
            environment = {"PATH": os.environ["PATH"], "HOME": str(home),
                           "XDG_CONFIG_HOME": str(home / "config"),
                           "STARPORT_SERVER_PORT": str(port), "OPENAI_API_KEY": key}
            report["version"] = subprocess.check_output([str(binary), "--version"], env=environment, cwd=home, text=True).strip()
            process = subprocess.Popen([str(binary), "dev", "--no-open"], env=environment, cwd=home,
                                       stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
            lines = []
            def collect():
                for line in process.stdout:
                    lines.append(line)
            reader = threading.Thread(target=collect, daemon=True)
            reader.start()
            base = f"http://127.0.0.1:{port}"
            gateway_key = None
            deadline = time.monotonic() + 60
            while time.monotonic() < deadline:
                for line in list(lines):
                    if line.startswith("Gateway API key (shown once): "):
                        gateway_key = line.split(": ", 1)[1].strip()
                        secrets.append(gateway_key)
                if process.poll() is not None:
                    raise RuntimeError("The released development server exited during startup.")
                if gateway_key:
                    try:
                        with urllib.request.urlopen(base + "/health/ready", timeout=1) as response:
                            report["readiness_status"] = response.status
                            break
                    except (OSError, urllib.error.URLError):
                        pass
                time.sleep(0.1)
            else:
                raise RuntimeError("The released development server did not become ready.")
            headers = {"Authorization": "Bearer " + gateway_key}
            with urllib.request.urlopen(urllib.request.Request(base + "/api/v1/models", headers=headers), timeout=15) as response:
                catalog = json.load(response)
                report["catalog_status"] = response.status
                report["model_count"] = len(catalog["data"])
                assert any(model["id"] == BODY["model"] for model in catalog["data"])
            request = urllib.request.Request(base + "/api/v1/chat/completions", data=json.dumps(BODY).encode(),
                                             headers={**headers, "Content-Type": "application/json"})
            start = time.monotonic()
            with urllib.request.urlopen(request, timeout=90) as response:
                stream = response.read(65536).decode()
                report["response_status"] = response.status
                report["content_type"] = response.headers.get("Content-Type")
                report["stream"] = stream
                report["elapsed_seconds"] = time.monotonic() - start
            chunks = [json.loads(line[6:]) for line in stream.splitlines()
                      if line.startswith("data: ") and line != "data: [DONE]"]
            report["answer"] = "".join(choice.get("delta", {}).get("content", "") or ""
                                       for chunk in chunks for choice in chunk.get("choices", []))
            assert report["answer"].strip() and "data: [DONE]" in stream
            with urllib.request.urlopen(urllib.request.Request(base + "/api/v1/activity", headers=headers), timeout=15) as response:
                report["activity"] = json.load(response)
            report["verdict"] = "PASS"
        except Exception as error:
            report["verdict"] = "FAIL"
            report["error"] = str(error)
            if isinstance(error, urllib.error.HTTPError):
                report["error_body"] = error.read(16384).decode(errors="replace")
        finally:
            if process is not None:
                if process.poll() is None:
                    process.send_signal(signal.SIGINT)
                    try:
                        process.wait(timeout=15)
                    except subprocess.TimeoutExpired:
                        process.kill()
                        process.wait()
                report["shutdown_exit_code"] = process.returncode
                report["home_files_after_shutdown"] = [str(path.relative_to(home)) for path in home.rglob("*")]
    serialized = json.dumps(report, indent=2)
    for secret in secrets:
        serialized = serialized.replace(secret, "[REDACTED]")
    (PROOF / "real-inference.json").write_text(serialized + "\n")
    print(json.dumps({name: report.get(name) for name in ["verdict", "release", "response_status", "answer", "elapsed_seconds", "error"]}))
    return 0 if report["verdict"] == "PASS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
