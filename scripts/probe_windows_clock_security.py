#!/usr/bin/env python3
"""Compare Windows time-service access with isolated Go source overlays."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[1]
PACKAGE = Path("pkg/catalogs/permission/hostclock/internal/w32timerpc")
PROFILES = (
    ("identification", False, False),
    ("pipe-impersonation", True, False),
    ("rpc-impersonation", False, True),
    ("pipe-and-rpc-impersonation", True, True),
)


def replace_exact(text, old, new, expected=1):
    if text.count(old) != expected:
        raise ValueError(f"probe source changed: expected {expected} matches for {old!r}")
    return text.replace(old, new)


def prepare_profile(output, name, pipe, rpc):
    directory = output / name
    directory.mkdir(parents=True, exist_ok=False)
    replacements = {}
    sources = []
    for filename in ("pipe_windows.go", "security_windows.go", "sspi_windows.go"):
        source = ROOT / PACKAGE / filename
        original = source.read_text(encoding="utf-8")
        selected = original
        if pipe and filename == "pipe_windows.go":
            selected = replace_exact(selected, "winio.PipeImpLevelIdentification", "winio.PipeImpLevelImpersonation")
        if rpc and filename == "security_windows.go":
            selected = replace_exact(selected, "dcerpc.Identify()", "dcerpc.Impersonate()")
            selected = replace_exact(selected, " | gssapi.Identify", "", expected=2)
        if rpc and filename == "sspi_windows.go":
            selected = replace_exact(selected, " | 0x00020000", "")
        if selected != original:
            target = directory / filename
            target.write_text(selected, encoding="utf-8")
            replacements[str(source)] = str(target)
        sources.append({
            "path": str(PACKAGE / filename),
            "original_sha256": hashlib.sha256(original.encode()).hexdigest(),
            "overlay_sha256": hashlib.sha256(selected.encode()).hexdigest(),
        })
    overlay = directory / "overlay.json"
    overlay.write_text(json.dumps({"Replace": replacements}, indent=2) + "\n")
    return {"profile": name, "overlay": str(overlay), "sources": sources}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--prepare-only", action="store_true")
    args = parser.parse_args()
    if not args.prepare_only and os.name != "nt":
        parser.error("native execution requires a disposable Windows test process")
    output = args.output.resolve()
    records = [prepare_profile(output, *profile) for profile in PROFILES]
    manifest = output / "security-probe.json"
    manifest.write_text(json.dumps(records, indent=2) + "\n")
    if args.prepare_only:
        return
    environment = os.environ.copy()
    environment["STARMAP_WINDOWS_CLOCK_PRIVILEGE_PROBE"] = "1"
    for record in records:
        command = ["go", "test", "-overlay", record["overlay"], "-json", "-count=1", "-timeout=2m",
                   "-run", "^TestWindowsTimeServicePreparedPrivilege$", "./" + PACKAGE.as_posix()]
        log = output / record["profile"] / "probe.jsonl"
        with log.open("w", encoding="utf-8") as stream:
            result = subprocess.run(command, cwd=ROOT, env=environment, stdout=stream,
                                    stderr=subprocess.STDOUT, timeout=240, check=False)
        record.update(command=command, exit_code=result.returncode)
        manifest.write_text(json.dumps(records, indent=2) + "\n")
        print(record["profile"], "exit", result.returncode, flush=True)


if __name__ == "__main__":
    main()
