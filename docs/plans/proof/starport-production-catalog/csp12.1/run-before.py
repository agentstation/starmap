"""Run retained cache regressions against the recorded Starport candidate."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

source = Path(sys.argv[1]).resolve()
proof = Path(__file__).resolve().parent
with tempfile.TemporaryDirectory(prefix="csp121-before-") as scratch:
    overlay = Path(scratch) / "overlay.json"
    overlay.write_text(json.dumps({"Replace": {
        str(source / "internal/cache/csp121_contract_test.go"):
            str(proof / "cache_contract_test.go.txt")
    }}))
    command = ["go", "test", "-overlay", str(overlay), "-race", "-count=1",
               "-timeout", "1m", "-json", "-run", "^TestCSP121", "./internal/cache"]
    result = subprocess.run(command, cwd=source, text=True, capture_output=True)
    (proof / "before.jsonl").write_text(result.stdout)
    (proof / "before.stderr").write_text(result.stderr)
    events = [json.loads(line) for line in result.stdout.splitlines() if line.startswith("{")]
    report = {
        "status": "FAIL" if result.returncode else "PASS",
        "exit_code": result.returncode,
        "source": str(source),
        "commit": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=source, text=True).strip(),
        "toolchain": subprocess.check_output(["go", "version"], cwd=source, text=True).strip(),
        "workspace": os.environ.get("GOWORK"),
        "command": command,
        "failed_tests": [e["Test"] for e in events if e.get("Action") == "fail" and "Test" in e],
        "passed_tests": [e["Test"] for e in events if e.get("Action") == "pass" and "Test" in e],
        "skipped_tests": [e["Test"] for e in events if e.get("Action") == "skip" and "Test" in e],
    }
    (proof / "before.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report, indent=2))
    sys.exit(result.returncode)
