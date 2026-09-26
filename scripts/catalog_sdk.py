"""Qualify the pinned SDK roster through Starport's live transition test."""

import re
import subprocess


TEST = "TestSDKCanonicalRemovalAfterSuccessfulInference"
STATES = ("present", "alias-removed", "removed")
CLIENTS = ("Python", "TypeScript", "Go")


def verify(root):
    script = root / "scripts/smoke-openrouter-sdks.sh"
    if not script.is_file():
        return {"status": "UNVERIFIED", "reason": "The SDK smoke script is unavailable."}
    command = ["bash", str(script)]
    try:
        result = subprocess.run(command, cwd=root, capture_output=True, text=True, timeout=600)
    except (OSError, subprocess.TimeoutExpired) as error:
        return {"status": "UNVERIFIED", "reason": type(error).__name__, "command": command}
    evidence = {"command": command, "cwd": str(root), "exit_code": result.returncode,
                "stdout": result.stdout, "stderr": result.stderr}
    if result.returncode:
        return {"status": "FAIL", "reason": "The SDK command failed.", **evidence}
    if re.search(r"^--- SKIP: " + TEST + r"\b", result.stdout, re.MULTILINE):
        return {"status": "UNVERIFIED", "reason": "The live SDK transition test was skipped.", **evidence}
    passed = re.search(r"^--- PASS: " + TEST + r" \(", result.stdout, re.MULTILINE)
    missing = [f"PASS {client} SDK catalog transition: {state}"
               for client in CLIENTS for state in STATES
               if not re.search(r": PASS " + client + r" SDK catalog transition: " + state + r"$",
                                result.stdout, re.MULTILINE)]
    if not passed or missing:
        return {"status": "UNVERIFIED", "reason": "The live SDK transition evidence is incomplete.",
                "missing": missing, **evidence}
    return {"status": "PASS", "reason": "All pinned SDKs passed the live catalog transitions.", **evidence}
