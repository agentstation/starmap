#!/usr/bin/env python3
"""Run complete, disjoint Go test groups and retain their timing evidence."""

import argparse
import hashlib
import json
import os
import re
from pathlib import Path
import subprocess
import sys
import tempfile


ROOT = Path(__file__).resolve().parents[1]
MODULE = "github.com/agentstation/starmap"
GROUPS = ("checks", "runtime", "client", "application", "contracts")
CAPACITY_PACKAGE = MODULE + "/internal/catalog/publication"
CAPACITY_TEST = "TestPublicPublicationProfileRetainsBoundedState"


def group_for(package):
    """Assign every module package to exactly one resource group."""
    if package != MODULE and not package.startswith(MODULE + "/"):
        raise ValueError("test package is outside the Starmap module")
    relative = package.removeprefix(MODULE).lstrip("/")
    if relative == "internal/ciworkflow" or relative.startswith("internal/ciworkflow/"):
        return "checks"
    if relative == "runtime" or relative.startswith("runtime/"):
        return "runtime"
    if relative in ("", "acquisition") or relative.startswith(("acquisition/", "internal/bootstrap/")) or relative == "internal/bootstrap":
        return "client"
    if relative.startswith(("cmd/", "internal/cli/", "internal/server/")) or relative == "internal/server":
        return "application"
    return "contracts"


def select_packages(packages, group):
    if group != "all" and group not in GROUPS:
        raise ValueError("unknown test group")
    if not packages or len(set(packages)) != len(packages):
        raise ValueError("test inventory is empty or contains duplicates")
    selected = [package for package in packages if group_for(package) == group or group == "all"]
    if not selected:
        raise ValueError("selected test group is empty")
    return selected


def test_command(suite, packages):
    # A package owns its goroutines. Separate hosted runners bound package memory.
    args = ["go", "test", "-json", "-count=1", "-timeout=30m", "-p=1"]
    if suite == "race":
        args += ["-race", "-skip=^" + CAPACITY_TEST + "$"]
    elif suite == "capacity":
        args += ["-run=^" + CAPACITY_TEST + "$"]
        packages = [CAPACITY_PACKAGE]
    elif suite != "regular":
        raise ValueError("unknown test suite")
    return args + packages


def select_tests(events, packages, shard):
    """Partition top-level tests, examples, and fuzz seeds without omissions."""
    if shard not in (1, 2, 3):
        raise ValueError("test shard must be 1, 2, or 3")
    inventory = set()
    completed = set()
    for event in events:
        package = event.get("Package")
        if event.get("Action") == "fail":
            raise ValueError("test inventory failed")
        if event.get("Action") in ("pass", "skip") and not event.get("Test"):
            if package in completed:
                raise ValueError("duplicate inventory package completion")
            completed.add(package)
        name = event.get("Output", "").strip()
        if event.get("Action") == "output" and re.fullmatch(r"(?:Test|Example|Fuzz)\w*", name):
            key = (package, name)
            if package not in packages or key in inventory:
                raise ValueError("invalid or duplicate test inventory entry")
            inventory.add(key)
    if completed != set(packages) or not inventory:
        raise ValueError("test inventory is incomplete")
    # The same name in different packages must select the same global -run filter.
    selected = {key for key in inventory
                if int.from_bytes(hashlib.sha256(key[1].encode()).digest()[:8], "big") % 3 == shard - 1}
    if not selected:
        raise ValueError("selected test shard is empty")
    return selected


def shard_filter(tests):
    return "-run=^(" + "|".join(re.escape(name) for name in sorted({name for _, name in tests})) + ")$"


def summarize(path, suite, packages, expected_tests=None):
    counts = {"pass": 0, "fail": 0, "skip": 0}
    slow = []
    capacity_passed = False
    completed = set()
    completed_tests = set()
    failed = False
    with path.open(encoding="utf-8") as stream:
        for line in stream:
            event = json.loads(line)
            action, name = event.get("Action"), event.get("Test")
            package = event.get("Package")
            if not name and action in ("pass", "skip"):
                if package in completed:
                    raise ValueError("duplicate package completion in test evidence")
                completed.add(package)
            if name and "/" not in name and action in counts:
                key = (package, name)
                if key in completed_tests:
                    raise ValueError("duplicate test completion")
                completed_tests.add(key)
            if name and action in counts:
                counts[action] += 1
                if "/" not in name and action != "skip":
                    slow.append((event.get("Elapsed", 0), event.get("Package"), name))
                capacity_passed |= action == "pass" and name == CAPACITY_TEST and event.get("Package") == CAPACITY_PACKAGE
            if action == "fail":
                failed = True
                print("FAILED:", event.get("Package"), name or "package", file=sys.stderr)
    if failed:
        raise ValueError("test evidence contains a failure")
    if completed != set(packages):
        raise ValueError("test evidence does not complete every selected package")
    if expected_tests is not None and completed_tests != expected_tests:
        raise ValueError("test evidence differs from the selected shard inventory")
    if counts["pass"] == 0:
        raise ValueError("test evidence contains no passing tests")
    if suite == "capacity" and not capacity_passed:
        raise ValueError("the full-catalog capacity test did not pass")
    print(json.dumps({"tests": counts, "slowest": sorted(slow, reverse=True)[:15], "events": str(path)}))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("suite", choices=("regular", "race", "capacity"))
    parser.add_argument("--group", choices=("all",) + GROUPS, default="all")
    parser.add_argument("--output", type=Path)
    parser.add_argument("--shard", type=int, choices=(0, 1, 2, 3), default=0)
    args = parser.parse_args()
    if args.suite == "capacity" and args.group != "all":
        parser.error("capacity runs as one complete test")
    if args.shard and (args.group not in ("runtime", "application") or args.suite != "race"):
        parser.error("shards apply only to the runtime and application race groups")
    inventory = subprocess.run(["go", "list", "./..."], cwd=ROOT, check=True,
                               capture_output=True, text=True, timeout=120).stdout.splitlines()
    packages = select_packages(inventory, args.group)
    selected = test_command(args.suite, packages)
    environment = os.environ.copy()
    if args.suite == "race":
        environment["CGO_ENABLED"] = "1"
    expected_tests = None
    if args.shard:
        listing = subprocess.run(["go", "test", "-race", "-json", "-list", ".", *packages],
                                 cwd=ROOT, env=environment, capture_output=True, text=True,
                                 check=True, timeout=600)
        expected_tests = select_tests([json.loads(line) for line in listing.stdout.splitlines()], packages, args.shard)
        selected.insert(2, shard_filter(expected_tests))
    path = args.output
    if path is None:
        descriptor, name = tempfile.mkstemp(prefix="starmap-tests-", suffix=".jsonl")
        os.close(descriptor)
        path = Path(name)
    path.parent.mkdir(parents=True, exist_ok=True)
    if expected_tests is not None:
        path.with_suffix(".inventory.json").write_text(json.dumps(sorted(expected_tests), indent=2) + "\n", encoding="utf-8")
    print("Running:", " ".join(selected), flush=True)
    print("Test events:", path, flush=True)
    with path.open("w", encoding="utf-8") as output:
        result = subprocess.run(selected, cwd=ROOT, env=environment, stdout=output, timeout=5400, check=False)
    if result.returncode:
        # Preserve Go's diagnostics and exit status, including package build failures.
        print(path.read_text(encoding="utf-8"), file=sys.stderr)
        return result.returncode
    summarize(path, args.suite, [CAPACITY_PACKAGE] if args.suite == "capacity" else packages, expected_tests)
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        print(f"test verification failed: {error}", file=sys.stderr)
        sys.exit(1)
