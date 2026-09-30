#!/usr/bin/env python3
"""Run disjoint native test groups and verify their complete combined evidence."""

import argparse
import json
from pathlib import Path
import shutil
import subprocess
import sys

import verification_tests as verification

ROOT = Path(__file__).resolve().parents[1]
MODULE = verification.MODULE
GROUPS = ("client", "application", "filesystem", "runtime-1", "runtime-2", "runtime-3")
PACKAGES = (
    ".", "./runtime", "./server", "./server/administration", "./internal/server/middleware",
    "./internal/cli/commands/serve", "./internal/auth", "./internal/bootstrap",
    "./internal/filepublish", "./internal/privatefiles", "./internal/runtimeacl/windows",
    "./internal/sources/github", "./internal/cli/app", "./pkg/productfiles",
    "./pkg/productpaths", "./pkg/productpaths/policy", "./pkg/catalogs/storage",
    "./pkg/catalogs/permission/...", "./internal/catalog/workspace", "./internal/test/gitfixture",
)
CLIENT = {"", "server", "server/administration", "internal/auth", "internal/bootstrap",
          "internal/sources/github", "internal/test/gitfixture"}
APPLICATION = {"internal/server/middleware", "internal/cli/commands/serve", "internal/cli/app"}


def group_for(package):
    """Keep native package ownership independent of compiler and platform."""
    if package != MODULE and not package.startswith(MODULE + "/"):
        raise ValueError("native package is outside the Starmap module")
    relative = package.removeprefix(MODULE).lstrip("/")
    if relative == "runtime":
        return "runtime"
    if relative in CLIENT:
        return "client"
    if relative in APPLICATION:
        return "application"
    return "filesystem"


def selected_packages(packages, group):
    if group not in GROUPS or not packages or len(packages) != len(set(packages)):
        raise ValueError("invalid native package inventory or group")
    owner = "runtime" if group.startswith("runtime-") else group
    selected = [package for package in packages if group_for(package) == owner]
    if not selected:
        raise ValueError("native test group is empty")
    return selected


def inventory_tests(events, packages):
    # Reuse the race-suite inventory contract and exact name selector.
    return set.union(*(verification.select_tests(events, packages, shard) for shard in (1, 2, 3)))


def selected_tests(inventory, group):
    if group.startswith("runtime-"):
        shard = int(group.removeprefix("runtime-"))
        events = [{"Action": "output", "Package": package, "Output": name + "\n"}
                  for package, name in sorted(inventory)]
        events += [{"Action": "pass", "Package": package} for package in sorted({package for package, _ in inventory})]
        return verification.select_tests(events, sorted({package for package, _ in inventory}), shard)
    return inventory


def validate_toolchain(lines, system, arch):
    if (system, arch) not in {("linux", "amd64"), ("linux", "arm64"), ("windows", "amd64"), ("windows", "arm64"), ("darwin", "arm64")} or lines != [f"go version go1.27.1 {system}/{arch}", system, arch, system, arch, "0"]:
        raise ValueError("native toolchain, target, host, or pure-Go mode differs")


def test_command(group, packages, tests):
    args = ["go", "test", "-json", "-count=1", "-timeout=30m", "-p=1"]
    if group.startswith("runtime-"):
        args.append(verification.shard_filter(tests))
    return args + packages


def run(group, system, arch, directory):
    directory.mkdir(parents=True, exist_ok=True)
    toolchain = subprocess.run(["go", "version"], cwd=ROOT, check=True, capture_output=True, text=True).stdout
    toolchain += subprocess.run(["go", "env", "GOOS", "GOARCH", "GOHOSTOS", "GOHOSTARCH", "CGO_ENABLED"],
                               cwd=ROOT, check=True, capture_output=True, text=True).stdout
    (directory / "toolchain.txt").write_text(toolchain, encoding="utf-8")
    validate_toolchain(toolchain.splitlines(), system, arch)
    patterns = list(PACKAGES)
    if system == "windows":
        patterns.append("./internal/test/windowstoken")
    packages = subprocess.run(["go", "list", *patterns], cwd=ROOT, check=True,
                              capture_output=True, text=True, timeout=120).stdout.splitlines()
    chosen = selected_packages(packages, group)
    listing = subprocess.run(["go", "test", "-json", "-p=1", "-list", ".", *chosen], cwd=ROOT,
                             check=True, capture_output=True, text=True, timeout=600)
    inventory = inventory_tests([json.loads(line) for line in listing.stdout.splitlines()], chosen)
    tests = selected_tests(inventory, group)
    manifest = {"version": 1, "group": group, "system": system, "arch": arch,
                "packages": packages, "inventory": sorted(inventory), "selected": sorted(tests), "status": "incomplete"}
    path = directory / "inventory.json"
    path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    events = directory / "tests.jsonl"
    command = test_command(group, chosen, tests)
    print("Running:", " ".join(command), flush=True)
    with events.open("w", encoding="utf-8") as output:
        result = subprocess.run(command, cwd=ROOT, stdout=output, timeout=5400, check=False)
    if result.returncode:
        print(events.read_text(encoding="utf-8"), file=sys.stderr)
        return result.returncode
    validate_events(events, chosen, tests)
    manifest["status"] = "success"
    path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    return 0


def validate_events(path, packages, tests):
    verification.summarize(path, "regular", packages, tests)
    started = set()
    with path.open(encoding="utf-8") as stream:
        for line in stream:
            event = json.loads(line)
            name = event.get("Test")
            if event.get("Action") == "run" and name and "/" not in name:
                key = (event.get("Package"), name)
                if key not in tests or key in started:
                    raise ValueError("native evidence contains an unexpected or duplicate test start")
                started.add(key)
    if started != tests:
        raise ValueError("native evidence omits a selected test start")


def validate_groups(groups, system, arch):
    """Require all native groups with disjoint tests and complete inventories."""
    if len(groups) != len(GROUPS) or {group["manifest"].get("group") for group in groups} != set(GROUPS):
        raise ValueError("native evidence has missing, duplicate, or unexpected groups")
    packages = None
    inventories = {}
    seen = set()
    for group in groups:
        manifest = group["manifest"]
        name = manifest["group"]
        if manifest.get("version") != 1 or manifest.get("status") != "success" or group.get("job_status") != "success" or manifest.get("system") != system or manifest.get("arch") != arch:
            raise ValueError("native group failed, did not complete, or belongs to another platform")
        validate_toolchain(group["toolchain"], system, arch)
        declared_packages = manifest["packages"]
        if packages is None:
            packages = declared_packages
        if declared_packages != packages:
            raise ValueError("native groups have different complete package inventories")
        chosen = selected_packages(packages, name)
        inventory_list, selected_list = manifest["inventory"], manifest["selected"]
        inventory = {tuple(test) for test in inventory_list}
        tests = {tuple(test) for test in selected_list}
        if len(inventory) != len(inventory_list) or len(tests) != len(selected_list):
            raise ValueError("native inventory contains duplicate tests")
        if any(package not in chosen for package, _ in inventory):
            raise ValueError("native inventory differs from its package ownership")
        if selected_tests(inventory, name) != tests or seen.intersection(tests):
            raise ValueError("native test selection is incomplete or overlaps another group")
        seen.update(tests)
        owner = "runtime" if name.startswith("runtime-") else name
        if owner in inventories and inventories[owner] != inventory:
            raise ValueError("native runtime shards have different complete inventories")
        inventories[owner] = inventory
        validate_events(group["events"], chosen, tests)
    complete = set.union(*inventories.values())
    owned_packages = {package for name in GROUPS for package in selected_packages(packages, name)}
    if seen != complete or owned_packages != set(packages):
        raise ValueError("native groups omit a package or test")
    return {"version": 1, "system": system, "arch": arch, "groups": list(GROUPS),
            "packages": len(packages), "top_level_tests": len(complete), "status": "success"}


def combine(source, directory, system, arch):
    groups = []
    for path in sorted(source.iterdir()):
        if not path.is_dir():
            raise ValueError("native evidence contains an unexpected input")
        manifest = json.loads((path / "inventory.json").read_text(encoding="utf-8"))
        groups.append({"manifest": manifest, "toolchain": (path / "toolchain.txt").read_text(encoding="utf-8").splitlines(),
                       "events": path / "tests.jsonl", "directory": path,
                       "job_status": (path / "job-status.txt").read_text(encoding="utf-8").strip()})
    summary = validate_groups(groups, system, arch)
    directory.mkdir(parents=True, exist_ok=True)
    ordered = sorted(groups, key=lambda group: GROUPS.index(group["manifest"]["group"]))
    with (directory / "tests.jsonl").open("wb") as output:
        for group in ordered:
            with group["events"].open("rb") as stream:
                shutil.copyfileobj(stream, output)
            shutil.copyfile(group["directory"] / "inventory.json", directory / (group["manifest"]["group"] + "-inventory.json"))
    shutil.copyfile(ordered[0]["directory"] / "toolchain.txt", directory / "toolchain.txt")
    filesystem = next(group["directory"] for group in groups if group["manifest"]["group"] == "filesystem")
    for path in filesystem.iterdir():
        if path.name not in ("inventory.json", "tests.jsonl", "toolchain.txt"):
            if not path.is_file():
                raise ValueError("native filesystem evidence contains an unexpected directory")
            shutil.copyfile(path, directory / path.name)
    if system == "linux" and not (directory / "service-owner.txt").is_file():
        raise ValueError("native Linux evidence omits administrator-owned configuration")
    (directory / "qualification.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(summary))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    for name in ("run", "combine"):
        command = commands.add_parser(name)
        command.add_argument("--system", required=True, choices=("linux", "darwin", "windows"))
        command.add_argument("--arch", required=True, choices=("amd64", "arm64"))
        command.add_argument("--output", required=True, type=Path)
        if name == "run":
            command.add_argument("--group", required=True, choices=GROUPS)
        else:
            command.add_argument("--source", required=True, type=Path)
    args = parser.parse_args()
    if args.command == "run":
        return run(args.group, args.system, args.arch, args.output)
    combine(args.source, args.output, args.system, args.arch)
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        print(f"native test qualification failed: {error}", file=sys.stderr)
        sys.exit(1)
