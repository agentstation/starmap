#!/usr/bin/env python3
"""Run the catalog plan's registered behavior checks without granting missing evidence a pass."""

import argparse
import base64
import gzip
import hashlib
import importlib.util
import json
import math
import os
import re
import subprocess
import sys
import tempfile
import time
import zipfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
ROSTER = ROOT / "docs/plans/proof/starport-production-catalog/acceptance-map.json"
REGISTRY = ROOT / "scripts/catalog-product-checks.json"
COMPONENT_CASES = {"CSP5": ("A22", "A23"), "CSP7": ("A11", "A12")}
EMBEDDED_CATALOG = "internal/embedded/catalog"
STARMAP_MODULE = "github.com/agentstation/starmap"
STARPORT_MODULE = "github.com/agentstation/starport"
STABLE_VERSION = re.compile(r"v\d+\.\d+\.\d+")


def read_json(path):
    return json.loads(path.read_text())


def validate_roster(roster):
    cases = {f"A{number:02}" for number in range(1, 51)}
    required = roster["required_subcases"]
    if roster["condition_count"] != 50 or set(required) != cases:
        raise ValueError("The roster must contain exactly A01 through A50.")
    if set(roster["primary_task"]) != cases:
        raise ValueError("Every primary case needs one owner.")
    subcases = [item for group in required.values() for item in group]
    if len(subcases) != len(set(subcases)):
        raise ValueError("Required subcase identifiers must be unique.")
    for case, group in required.items():
        if not group or any(not item.startswith(case + ".") for item in group):
            raise ValueError("Each primary case needs its own named subcases.")
    checks = set(subcases) | set(roster["early_checks"]) | set(roster["rehearsal_checks"])
    for task, group in roster["task_checks"].items():
        if not group or len(group) != len(set(group)) or not set(group) <= checks:
            raise ValueError(f"Invalid check roster for {task}.")
    for case, task in roster["primary_task"].items():
        if not set(required[case]) <= set(roster["task_checks"][task]):
            raise ValueError(f"The owner of {case} omits required subcases.")
    qualification = roster["qualification"]
    if set(qualification["final_required_primary_cases"]) != cases:
        raise ValueError("Final qualification must include all 50 primary cases.")
    candidate = set(qualification["candidate_required_primary_cases"])
    published = set(qualification["requires_published_assets"])
    if len(candidate) != 43 or len(published) != 7 or candidate & published or candidate | published != cases:
        raise ValueError("Candidate and publication rosters must partition the 50 cases.")
    extra = qualification["candidate_additional_subcases"]
    if len(extra) != len(set(extra)) or not set(extra) <= set(subcases):
        raise ValueError("Invalid additional candidate subcases.")
    if any(item.split(".")[0] not in published for item in extra):
        raise ValueError("Additional candidate subcases must belong to publication-dependent parents.")
    candidate_checks = {item for case in candidate for item in required[case]} | set(extra)
    if set(roster["task_checks"]["CSP22"]) != candidate_checks:
        raise ValueError("CSP22 must include complete candidate cases and additional local subcases.")
    if set(roster["task_checks"]["CSP24"]) != set(subcases):
        raise ValueError("CSP24 must include every required subcase.")
    return checks


def select_checks(args, roster):
    if args.task:
        if args.task == "CSP0":
            return sorted(item for group in roster["required_subcases"].values() for item in group)
        if args.task not in roster["task_checks"]:
            raise ValueError(f"Unknown task: {args.task}")
        return roster["task_checks"][args.task]
    if args.gate == "candidate":
        return roster["task_checks"]["CSP22"]
    if args.case:
        if any(case not in roster["required_subcases"] for case in args.case):
            raise ValueError("Unknown primary acceptance case.")
        return sorted({item for case in args.case for item in roster["required_subcases"][case]})
    return sorted(item for group in roster["required_subcases"].values() for item in group)


def validate_registry(registry, roster, checks):
    if registry.get("schema_version") != 1 or not set(registry["checks"]) <= checks:
        raise ValueError("Invalid behavior-check registry.")
    components = registry.get("task_component_checks", {})
    if not isinstance(components, dict) or set(components) - set(COMPONENT_CASES):
        raise ValueError("Only CSP5 and CSP7 have approved producer component contracts.")
    for task, entries in components.items():
        allowed = {identity for case in COMPONENT_CASES[task] for identity in roster["required_subcases"][case]}
        if not isinstance(entries, dict) or not set(entries) <= allowed.intersection(roster["task_checks"][task]):
            raise ValueError("Producer component checks exceed their approved task contract.")


def registered_check(args, registry, identity):
    # Producer tasks qualify their components. Consumer tasks and release gates require product evidence.
    if args.task in COMPONENT_CASES and not (args.gate or args.released_assets or args.recipes or args.backends):
        components = registry.get("task_component_checks", {}).get(args.task, {})
        if identity in components:
            return components[identity], "producer_component"
    return registry["checks"].get(identity), "product"


GO_PROCESS_BUDGET_SECONDS = 300
GO_TEST_BUDGET_SECONDS = 60
GO_TEST_BUDGET_LIMIT_SECONDS = 1800


def go_check_budget(entry):
    """Return the declared per-test process budget in seconds, or None when it is invalid."""
    value = entry.get("timeout", f"{GO_TEST_BUDGET_SECONDS}s")
    match = re.fullmatch(r"([1-9][0-9]{0,3})([sm])", value) if isinstance(value, str) else None
    if match is None:
        return None
    seconds = int(match.group(1)) * (60 if match.group(2) == "m" else 1)
    return seconds if seconds <= GO_TEST_BUDGET_LIMIT_SECONDS else None


def go_check_input(entry, roots):
    root = roots.get(entry.get("repository"))
    package, test = entry.get("package", ""), entry.get("test", "")
    if root is None or not (root / "go.mod").is_file():
        return None, {"status": "UNVERIFIED", "reason": "The required repository is unavailable."}
    if (not isinstance(test, str) or not isinstance(package, str) or type(entry.get("batch", True)) is not bool
            or go_check_budget(entry) is None
            or not re.fullmatch(r"Test[A-Za-z0-9_]+", test) or not package.startswith("./")
            or any(part in ("..", "...") for part in package.split("/")[1:])):
        return None, {"status": "FAIL", "reason": "Invalid named Go behavior check."}
    required = entry.get("required_subtests", [])
    if (not isinstance(required, list) or len(required) > 64
            or any(not isinstance(name, str) or len(name) > 256
                   or not re.fullmatch(r"[A-Za-z0-9_-]+(?:/[A-Za-z0-9_-]+)*", name) for name in required)
            or len(required) != len(set(required))):
        return None, {"status": "FAIL", "reason": "Invalid required Go subtests."}
    return (root.resolve(), package, test), None


def leaf_checks(entry):
    if not isinstance(entry, dict):
        return
    if entry.get("kind") == "all":
        for child in entry.get("checks", []):
            yield from leaf_checks(child)
    else:
        yield entry


class GoEvidence:
    """Batch selected tests and preserve separate process budgets where required."""

    def __init__(self, entries, roots):
        self.groups = {}
        self.results = {}
        self.isolated = set()
        self.budgets = {}
        for entry in entries:
            for check in leaf_checks(entry):
                if check.get("kind") != "go_test":
                    continue
                inputs, error = go_check_input(check, roots)
                if error is None:
                    root, package, test = inputs
                    self.groups.setdefault((root, package), set()).add(test)
                    self.budgets[inputs] = max(self.budgets.get(inputs, 0), go_check_budget(check))
                    if not check.get("batch", True):
                        self.isolated.add(inputs)
        for root, package, test in self.isolated:
            self.groups[(root, package)].discard(test)

    def check(self, root, package, test):
        key = (root, package, test)
        if key not in self.results:
            names = [test] if key in self.isolated else sorted(self.groups.get((root, package), {test}))
            budget = go_process_budget(self.budgets.get((root, package, name), GO_TEST_BUDGET_SECONDS) for name in names)
            for name, result in run_go_tests(root, package, names, budget).items():
                self.results[(root, package, name)] = result
        return dict(self.results[key])


def go_process_budget(budgets):
    """One process budget covers every selected test, with the shared floor for short batches."""
    return max(GO_PROCESS_BUDGET_SECONDS, sum(budgets))


def retain_go_run(evidence, results):
    directory = os.environ.get("CATALOG_PRODUCT_GO_EVIDENCE_DIR")
    if not directory:
        return results
    try:
        Path(directory).mkdir(parents=True, exist_ok=True)
        with tempfile.NamedTemporaryFile(mode="w", prefix="go-run-", suffix=".json", dir=directory, delete=False) as output:
            json.dump({"execution": evidence, "tests": {
                name: {"status": result["status"], "reason": result["reason"]}
                for name, result in results.items()}}, output)
            output.write("\n")
        for result in results.values():
            result["evidence_file"] = output.name
    except OSError as error:
        for result in results.values():
            result.update(status="UNVERIFIED", reason=f"Cannot retain Go run evidence: {type(error).__name__}")
    return results


def run_go_tests(root, package, names, budget_seconds=GO_PROCESS_BUDGET_SECONDS):
    pattern = "^(" + "|".join(names) + ")$"
    command = ["go", "test", "-race", "-count=1", "-timeout", f"{budget_seconds}s", "-json", "-run", pattern, package]
    started = time.monotonic()
    try:
        module = re.search(r"(?m)^module[ \t]+([^\s]+)", (root / "go.mod").read_text())
        if module is None:
            raise ValueError("The repository has no module declaration.")
        import_path = module.group(1).strip('"')
        if package.rstrip("/") != ".":
            import_path += "/" + package[2:].rstrip("/")
        result = subprocess.run(command, cwd=root, capture_output=True, text=True, timeout=budget_seconds + 30)
    except (OSError, ValueError, subprocess.TimeoutExpired) as error:
        evidence = {"command": command, "cwd": str(root), "elapsed_seconds": time.monotonic() - started}
        if isinstance(error, subprocess.TimeoutExpired):
            for field in ("stdout", "stderr"):
                value = getattr(error, field) or ""
                evidence[field] = value.decode(errors="replace") if isinstance(value, bytes) else value
        return retain_go_run(evidence, {
            name: dict(evidence, status="UNVERIFIED", reason=type(error).__name__) for name in names})
    evidence = {"command": command, "cwd": str(root), "exit_code": result.returncode,
                "stdout": result.stdout, "stderr": result.stderr, "elapsed_seconds": time.monotonic() - started}
    events, invalid = [], False
    for line in result.stdout.splitlines():
        if not line.strip():
            continue
        try:
            event = json.loads(line)
            if (not isinstance(event, dict) or not isinstance(event.get("Action"), str)
                    or ("Test" in event and not isinstance(event["Test"], str))
                    or ("Package" in event and not isinstance(event["Package"], str))):
                invalid = True
            else:
                events.append(event)
        except json.JSONDecodeError:
            invalid = True
    failed = result.returncode or any(event.get("Action") == "fail" for event in events)
    package_passes = sum(event.get("Action") == "pass" and event.get("Package") == import_path
                         and not event.get("Test") for event in events)
    test_events = {}
    for event in events:
        if event.get("Package") == import_path and event.get("Test"):
            test_events.setdefault(event["Test"], []).append(event)
    results = {}
    for name in names:
        matched = test_events.get(name, [])
        skipped = any(event.get("Action") == "skip" and event.get("Package") == import_path
                      and (event.get("Test") == name or event.get("Test", "").startswith(name + "/"))
                      for event in events)
        if failed:
            status, reason = "FAIL", "The behavior command failed."
        elif invalid or package_passes != 1:
            status, reason = "UNVERIFIED", "The package has no complete valid event stream."
        elif skipped:
            status, reason = "UNVERIFIED", "The named behavior test or a subtest was skipped."
        elif (sum(event.get("Action") == "run" for event in matched) != 1
              or sum(event.get("Action") == "pass" for event in matched) != 1):
            status, reason = "UNVERIFIED", "The named behavior test did not run and pass exactly once."
        else:
            status, reason = "PASS", "The named behavior test passed in this invocation."
        subtests = {}
        for child, child_events in sorted(test_events.items()):
            if not child.startswith(name + "/"):
                continue
            actions = [event["Action"] for event in child_events]
            subtests[child[len(name) + 1:]] = (actions.count("run") == 1 and actions.count("pass") == 1
                                            and "skip" not in actions and "fail" not in actions)
        results[name] = dict(evidence, status=status, reason=reason, subtests=subtests)
    return retain_go_run(evidence, results)


def run_check(identity, entry, roots, go_evidence=None):
    if entry is None:
        return {"status": "UNVERIFIED", "reason": "No behavior check is registered."}
    if entry.get("kind") == "all":
        children = entry.get("checks", [])
        if not children:
            return {"status": "FAIL", "reason": "A combined check needs evidence."}
        results = [run_check(identity, child, roots, go_evidence) for child in children]
        states = [result["status"] for result in results]
        status = "FAIL" if "FAIL" in states else "UNVERIFIED" if "UNVERIFIED" in states else "PASS"
        return {"status": status, "checks": results}
    if entry.get("kind") == "catalog_sdk":
        root = roots.get("starport")
        if root is None:
            return {"status": "UNVERIFIED", "reason": "The Starport repository is unavailable."}
        spec = importlib.util.spec_from_file_location("catalog_sdk", ROOT / "scripts/catalog_sdk.py")
        adapter = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(adapter)
        return adapter.verify(root)
    if entry.get("kind") == "native_ci":
        root = roots.get(entry.get("repository"))
        if root is None:
            return {"status": "UNVERIFIED", "reason": "The native evidence repository is unavailable."}
        try:
            spec = importlib.util.spec_from_file_location("catalog_native_ci", root / "scripts/native_catalog.py")
            adapter = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(adapter)
            return adapter.verify(root, entry)
        except (OSError, ImportError) as error:
            return {"status": "UNVERIFIED", "reason": str(error)}
    if entry.get("kind") == "publication_hosted":
        root = roots.get(entry.get("repository"))
        if root is None:
            return {"status": "UNVERIFIED", "reason": "The publisher evidence repository is unavailable."}
        try:
            spec = importlib.util.spec_from_file_location(
                "catalog_publication_capture", root / "scripts/catalog_publication_capture.py")
            adapter = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(adapter)
            return adapter.verify(root, entry)
        except (OSError, ImportError) as error:
            return {"status": "UNVERIFIED", "reason": str(error)}
    if entry.get("kind") == "publication_recovery":
        root = roots.get(entry.get("repository"))
        if root is None:
            return {"status": "UNVERIFIED", "reason": "The publisher evidence repository is unavailable."}
        try:
            spec = importlib.util.spec_from_file_location(
                "catalog_publication_qualification", root / "scripts/catalog_publication_qualification.py")
            adapter = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(adapter)
            return adapter.verify_recovery(root)
        except (OSError, ImportError) as error:
            return {"status": "UNVERIFIED", "reason": str(error)}
    if entry.get("kind") == "vitest":
        return run_vitest(entry, roots)
    if entry.get("kind") == "reviewed_ui":
        return reviewed_artifacts(entry, roots, "browser")
    if entry.get("kind") == "reviewed_first_use":
        return reviewed_first_use(entry, roots)
    if entry.get("kind") == "reviewed_demo":
        return reviewed_demo(entry, roots)
    if entry.get("kind") == "rehearsal_demo":
        return rehearsal_demo(entry, roots)
    if entry.get("kind") == "performance_baseline":
        return run_performance_baseline(entry, roots)
    if entry.get("kind") == "performance_profile":
        return reviewed_performance_profile(entry, roots)
    if entry.get("kind") == "measurement_record":
        return reviewed_measurement_record(entry, roots)
    if entry.get("kind") == "constructor_network":
        root = roots.get(entry.get("repository"))
        if root is None or not (root / "scripts/testdata/constructor-probe/main.go").is_file():
            return {"status": "UNVERIFIED", "reason": "The constructor probe is unavailable."}
        try:
            spec = importlib.util.spec_from_file_location("catalog_constructor_network", root / "scripts/constructor_network.py")
            adapter = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(adapter)
            return adapter.verify(root)
        except (OSError, ImportError) as error:
            return {"status": "UNVERIFIED", "reason": str(error)}
    if entry.get("kind") == "cold_server":
        root = roots.get(entry.get("repository"))
        if root is None or not (root / "scripts/testdata/cold-server-probe/main.go").is_file():
            return {"status": "UNVERIFIED", "reason": "The offline server probe is unavailable."}
        try:
            spec = importlib.util.spec_from_file_location("catalog_cold_server", root / "scripts/cold_server.py")
            adapter = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(adapter)
            return adapter.verify(root)
        except (OSError, ImportError) as error:
            return {"status": "UNVERIFIED", "reason": str(error)}
    if entry.get("kind") == "promoted_checkout":
        return promoted_checkout(entry, roots)
    if entry.get("kind") == "pinned_module_baseline":
        return pinned_module_baseline(entry, roots)
    if entry.get("kind") == "released_module":
        return released_module(entry, roots)
    if entry.get("kind") == "released_module_pin":
        return released_module_pin(entry, roots)
    if entry.get("kind") != "go_test":
        return {"status": "UNVERIFIED", "reason": "This evidence adapter has not been implemented."}
    inputs, error = go_check_input(entry, roots)
    if error is not None:
        return error
    root, package, test = inputs
    if isinstance(go_evidence, GoEvidence):
        evidence = go_evidence.check(root, package, test)
    else:
        evidence_key = (root, package, test)
        if go_evidence is not None and evidence_key in go_evidence:
            evidence = dict(go_evidence[evidence_key])
        else:
            evidence = run_go_tests(root, package, [test], go_process_budget([go_check_budget(entry)]))[test]
            if go_evidence is not None:
                go_evidence[evidence_key] = evidence
    result = dict(evidence)
    missing = [name for name in entry.get("required_subtests", []) if evidence.get("subtests", {}).get(name) is not True]
    if missing and result["status"] == "PASS":
        result.update(status="UNVERIFIED", reason="Required Go subtests did not run and pass exactly once.",
                      missing_subtests=missing)
    return result


class EmbeddingMismatch(Exception):
    """Report readable catalog bytes that contradict their own record."""


def checked_output(root, args):
    return subprocess.run(args, cwd=root, check=True, capture_output=True, text=True, timeout=300,
                          env=dict(os.environ, GOTOOLCHAIN="go1.27.1", GOWORK="off", GOFLAGS="")).stdout


def embedded_generation(read):
    """Return one embedded generation identity after the payload matches its declared digest."""
    generation = json.loads(read("generation.json"))
    payload = gzip.decompress(read("generation-payload.json.gz"))
    declared = generation["payload"]
    if ("sha256:" + hashlib.sha256(payload).hexdigest() != declared["checksum"]
            or len(payload) != declared["size_bytes"]):
        raise EmbeddingMismatch("The embedded payload differs from its generation record.")
    return {"generation_id": generation["generation_id"], "semantic_checksum": generation["semantic_checksum"]}


def checkout_generation(root):
    if checked_output(root, ["git", "status", "--porcelain", "--", EMBEDDED_CATALOG]).strip():
        raise ValueError("The checkout changes the embedded catalog.")
    return embedded_generation(lambda name: (root / EMBEDDED_CATALOG / name).read_bytes())


def attested_channel(root):
    """Read catalog/v1 through the publication capture path and require attested bytes."""
    spec = importlib.util.spec_from_file_location(
        "catalog_publication_capture", root / "scripts/catalog_publication_capture.py")
    capture = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(capture)
    raw = base64.b64decode(capture.api(root, "contents/channel.json?ref=catalog%2Fv1")["content"], validate=False)
    with tempfile.TemporaryDirectory(prefix="starmap-channel-") as temporary:
        path = Path(temporary) / "catalog-v1.json"
        path.write_bytes(raw)
        reports = json.loads(capture.command(root, ["gh", "attestation", "verify", str(path), "--repo", capture.REPOSITORY,
                             "--signer-workflow", capture.REPOSITORY + "/.github/workflows/catalog-generation.yaml",
                             "--source-ref", "refs/heads/main", "--deny-self-hosted-runners", "--format", "json"]))
    digest = hashlib.sha256(raw).hexdigest()
    if not any(subject.get("digest", {}).get("sha256") == digest
               for report in reports for subject in report["verificationResult"]["statement"]["subject"]):
        raise ValueError("No attestation binds the catalog/v1 channel bytes.")
    channel = json.loads(raw)
    if (channel.get("schema_version") != 1 or channel.get("channel") != "catalog/v1"
            or not isinstance(channel.get("generation_id"), str)
            or not re.fullmatch(r"sha256:[0-9a-f]{64}", str(channel.get("catalog_digest")))):
        raise ValueError("The catalog/v1 channel does not select one generation.")
    return channel, "sha256:" + digest


def promoted_checkout(entry, roots):
    """Compare the attested catalog/v1 promotion with the clean checkout embedding."""
    root = roots.get(entry.get("repository"))
    if root is None:
        return {"status": "UNVERIFIED", "reason": "The checkout repository is unavailable."}
    try:
        embedded = checkout_generation(root)
        channel, channel_digest = attested_channel(root)
    except EmbeddingMismatch as error:
        return {"status": "FAIL", "reason": str(error)}
    except (OSError, ImportError, ValueError, KeyError, TypeError, AttributeError, subprocess.SubprocessError) as error:
        return {"status": "UNVERIFIED", "reason": str(error)}
    promoted = {"generation_id": channel["generation_id"], "semantic_checksum": channel["catalog_digest"]}
    result = {"status": "PASS" if promoted == embedded else "FAIL", "channel_sha256": channel_digest,
              "channel_sequence": channel.get("sequence"), "promoted": promoted, "embedded": embedded}
    if promoted != embedded:
        result["reason"] = "The checkout embeds a generation that catalog/v1 does not promote."
    return result


def module_hash(archive):
    """Return the Go h1 directory hash of one module zip."""
    lines = "".join(f"{hashlib.sha256(archive.read(name)).hexdigest()}  {name}\n" for name in sorted(archive.namelist()))
    return "h1:" + base64.b64encode(hashlib.sha256(lines.encode("utf-8")).digest()).decode("ascii")


ENVIRONMENT_ERRORS = (OSError, ValueError, KeyError, TypeError, AttributeError, zipfile.BadZipFile, subprocess.SubprocessError)


def release_version(root):
    """Return the latest stable release tag that is an ancestor of HEAD, or None when no such tag exists."""
    try:
        version = checked_output(root, ["git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9]*",
                                        "--exclude", "v*-*", "HEAD"]).strip()
    except subprocess.CalledProcessError:
        return None
    return version if STABLE_VERSION.fullmatch(version) else None


def download_module(path, version):
    """Download one module version and return the Go report with its Zip path and checksum database Sum."""
    # An empty directory keeps the download from changing a repository go.mod or go.sum.
    with tempfile.TemporaryDirectory(prefix="catalog-module-") as temporary:
        try:
            download = json.loads(checked_output(Path(temporary), ["go", "mod", "download", "-json", f"{path}@{version}"]))
        except subprocess.CalledProcessError as error:
            try:
                detail = json.loads(error.stdout)["Error"]
            except (ValueError, KeyError, TypeError):
                detail = (error.stderr or "").strip() or str(error)
            raise ValueError(f"Cannot download {path}@{version}: {detail}") from error
    return download


def module_generation(archive, version):
    """Return the generation that one Starmap module zip embeds."""
    prefix = f"{STARMAP_MODULE}@{version}/{EMBEDDED_CATALOG}/"
    return embedded_generation(lambda name: archive.read(prefix + name))


def go_mod_directives(text, verb):
    """Return the fields of each require or replace line of go.mod text, in single-line and block form."""
    entries, block = [], False
    for line in text.splitlines():
        fields = line.split("//")[0].split()
        if not fields:
            continue
        if block:
            if fields == [")"]:
                block = False
            else:
                entries.append(fields)
        elif fields[0] == verb:
            if fields[1:] == ["("]:
                block = True
            else:
                entries.append(fields[1:])
    return [[entry[0].strip('"'), *entry[1:]] for entry in entries if entry]


def required_starmap(text):
    """Return the one Starmap version that go.mod text requires, or None."""
    versions = [fields[1] for fields in go_mod_directives(text, "require") if fields[0] == STARMAP_MODULE and len(fields) > 1]
    return versions[0] if len(versions) == 1 else None


def go_sum_record(text, version):
    """Return the one go.sum h1 record of the Starmap module zip at version, or None."""
    recorded = [line.split()[2] for line in text.splitlines()
                if line.split()[:2] == [STARMAP_MODULE, version] and len(line.split()) == 3]
    return recorded[0] if len(recorded) == 1 else None


def consumer_pin(consumer):
    """Return the one Starmap version that the consumer go.mod requires without a replacement."""
    module = json.loads(checked_output(consumer, ["go", "mod", "edit", "-json"]))
    if any(item["Old"]["Path"] == STARMAP_MODULE for item in module.get("Replace") or []):
        raise ValueError("The consumer replaces the pinned Starmap module.")
    versions = [item["Version"] for item in module.get("Require") or [] if item["Path"] == STARMAP_MODULE]
    if len(versions) != 1:
        raise ValueError("The consumer does not pin one Starmap module version.")
    return versions[0]


def previous_pin(consumer, version):
    """Return the latest consumer commit, Starmap version, and go.sum record of the pin before version."""
    for commit in checked_output(consumer, ["git", "log", "--format=%H", "-n", "200", "--", "go.mod"]).split():
        old = required_starmap(checked_output(consumer, ["git", "show", f"{commit}:go.mod"]))
        if old == version:
            continue
        if old is None:
            return None
        try:
            record = go_sum_record(checked_output(consumer, ["git", "show", f"{commit}:go.sum"]), old)
        except subprocess.CalledProcessError:
            return None
        return None if record is None else (commit, old, record)
    return None


def pinned_module_baseline(entry, roots):
    """Compare the Starmap module bytes that Starport pins with the newly selected embedding."""
    consumer, source = roots.get(entry.get("repository")), roots.get("starmap")
    if consumer is None or source is None:
        return {"status": "UNVERIFIED", "reason": "The consumer or source repository is unavailable."}
    try:
        version = consumer_pin(consumer)
        recorded = go_sum_record((consumer / "go.sum").read_text(), version)
        if recorded is None:
            raise ValueError("The consumer go.sum has no record of the pinned Starmap module.")
        selected = checkout_generation(source)
        with zipfile.ZipFile(download_module(STARMAP_MODULE, version)["Zip"]) as archive:
            if module_hash(archive) != recorded:
                raise EmbeddingMismatch("The pinned Starmap module bytes differ from the consumer go.sum record.")
            pinned = module_generation(archive, version)
    except EmbeddingMismatch as error:
        return {"status": "FAIL", "reason": str(error)}
    except ENVIRONMENT_ERRORS as error:
        return {"status": "UNVERIFIED", "reason": str(error)}
    result = {"module": f"{STARMAP_MODULE}@{version}", "go_sum": recorded, "pinned": pinned, "selected": selected}
    if pinned != selected:
        return result | {"status": "PASS", "scope": "The pinned module bytes match the consumer go.sum record and keep their original generation."}
    # The current pin carries the selected generation. The pin before it holds the old bytes.
    try:
        previous = previous_pin(consumer, version)
        if previous is None:
            return result | {"status": "UNVERIFIED", "reason": "The pinned module embeds the selected generation "
                             "and the consumer history has no earlier pin to compare."}
        commit, old, record = previous
        with zipfile.ZipFile(download_module(STARMAP_MODULE, old)["Zip"]) as archive:
            if module_hash(archive) != record:
                raise EmbeddingMismatch("The previous pinned Starmap module bytes differ from the consumer go.sum record at that pin.")
            earlier = module_generation(archive, old)
    except EmbeddingMismatch as error:
        return {"status": "FAIL", "reason": str(error)}
    except ENVIRONMENT_ERRORS as error:
        return {"status": "UNVERIFIED", "reason": str(error)}
    result["previous"] = {"module": f"{STARMAP_MODULE}@{old}", "go_sum": record, "commit": commit, "pinned": earlier}
    if earlier == selected:
        return result | {"status": "UNVERIFIED", "reason": "No earlier pinned generation differs from the selected generation."}
    return result | {"status": "PASS", "scope": "The previous pinned module bytes match the consumer go.sum record at that pin "
                     "and keep their original generation. The current pin embeds the selected generation."}


def released_module(entry, roots):
    """Compare the released Starmap module with the checksum database, the checkout, and catalog/v1."""
    root = roots.get(entry.get("repository"))
    if root is None:
        return {"status": "UNVERIFIED", "reason": "The release repository is unavailable."}
    try:
        version = release_version(root)
        if version is None:
            return {"status": "UNVERIFIED", "reason": "No stable release tag is an ancestor of the checkout."}
        download = download_module(STARMAP_MODULE, version)
        digest = download["Sum"]
        with zipfile.ZipFile(download["Zip"]) as archive:
            if module_hash(archive) != digest:
                raise EmbeddingMismatch("The released Starmap module bytes differ from the checksum database record.")
            released = module_generation(archive, version)
        checkout = checkout_generation(root)
        channel, channel_digest = attested_channel(root)
    except EmbeddingMismatch as error:
        return {"status": "FAIL", "reason": str(error)}
    except (ImportError, *ENVIRONMENT_ERRORS) as error:
        return {"status": "UNVERIFIED", "reason": str(error)}
    promoted = {"generation_id": channel["generation_id"], "semantic_checksum": channel["catalog_digest"]}
    result = {"version": version, "module": f"{STARMAP_MODULE}@{version}", "sum": digest, "released": released,
              "checkout": checkout, "promoted": promoted, "channel_sha256": channel_digest}
    if released != promoted:
        return result | {"status": "FAIL", "reason": "The released module embeds a generation that catalog/v1 does not promote."}
    if checkout != released:
        return result | {"status": "FAIL", "reason": "The checkout embeds a generation that the released module does not carry."}
    return result | {"status": "PASS"}


def released_module_pin(entry, roots):
    """Prove that the released Starport module pins the released Starmap module."""
    consumer, source = roots.get(entry.get("repository")), roots.get("starmap")
    if consumer is None or source is None:
        return {"status": "UNVERIFIED", "reason": "The consumer or source repository is unavailable."}
    try:
        version = consumer_pin(consumer)
        if not STABLE_VERSION.fullmatch(version):
            return {"status": "FAIL", "reason": "The consumer does not pin a stable Starmap release.", "pinned_version": version}
        released = release_version(source)
        if released is None:
            return {"status": "UNVERIFIED", "reason": "No stable release tag is an ancestor of the checkout."}
        if version != released:
            return {"status": "FAIL", "reason": "The consumer pins a Starmap version that is not the released pair version.",
                    "pinned_version": version, "release_version": released}
        recorded = go_sum_record((consumer / "go.sum").read_text(), version)
        if recorded is None:
            raise ValueError("The consumer go.sum has no record of the pinned Starmap module.")
        download = download_module(STARMAP_MODULE, version)
        digest = download["Sum"]
        with zipfile.ZipFile(download["Zip"]) as archive:
            pinned = module_hash(archive)
        if pinned != recorded:
            raise EmbeddingMismatch("The released Starmap module bytes differ from the consumer go.sum record.")
        if pinned != digest:
            raise EmbeddingMismatch("The released Starmap module bytes differ from the checksum database record.")
        consumer_version = release_version(consumer)
        if consumer_version is None:
            return {"status": "UNVERIFIED", "reason": "No stable release tag is an ancestor of the consumer checkout."}
        consumer_download = download_module(STARPORT_MODULE, consumer_version)
        consumer_digest = consumer_download["Sum"]
        with zipfile.ZipFile(consumer_download["Zip"]) as archive:
            if module_hash(archive) != consumer_digest:
                raise EmbeddingMismatch("The released Starport module bytes differ from the checksum database record.")
            requirements = archive.read(f"{STARPORT_MODULE}@{consumer_version}/go.mod").decode()
    except EmbeddingMismatch as error:
        return {"status": "FAIL", "reason": str(error)}
    except ENVIRONMENT_ERRORS as error:
        return {"status": "UNVERIFIED", "reason": str(error)}
    result = {"consumer_module": f"{STARPORT_MODULE}@{consumer_version}", "consumer_sum": consumer_digest,
              "module": f"{STARMAP_MODULE}@{version}", "go_sum": recorded, "sum": digest}
    if (required_starmap(requirements) != version
            or any(fields[0] == STARMAP_MODULE for fields in go_mod_directives(requirements, "replace"))):
        return result | {"status": "FAIL", "reason": "The released Starport module does not pin the released Starmap module."}
    return result | {"status": "PASS"}


def run_vitest(entry, roots):
    root = roots.get(entry.get("repository"))
    files, tests = entry.get("files", []), entry.get("tests", [])
    if root is None or not (root / "console/package.json").is_file():
        return {"status": "UNVERIFIED", "reason": "The required console is unavailable."}
    if not files or not tests or len(tests) != len(set(tests)) or any(
        not file.startswith("src/") or not file.endswith((".test.ts", ".test.tsx")) or ".." in file.split("/") for file in files
    ):
        return {"status": "FAIL", "reason": "Invalid named console behavior checks."}
    with tempfile.TemporaryDirectory(prefix="catalog-console-check-") as directory:
        output = Path(directory) / "results.json"
        command = ["pnpm", "--dir", "console", "test", *files, "--reporter=json", f"--outputFile={output}"]
        try:
            result = subprocess.run(command, cwd=root, capture_output=True, text=True, timeout=120)
            evidence = {"command": command, "cwd": str(root), "exit_code": result.returncode,
                        "stdout": result.stdout, "stderr": result.stderr}
            if result.returncode:
                return {"status": "FAIL", "reason": "Console behavior tests failed.", **evidence}
            report = read_json(output)
        except (OSError, ValueError, subprocess.TimeoutExpired) as error:
            return {"status": "UNVERIFIED", "reason": type(error).__name__, "command": command}
    assertions = [item for suite in report.get("testResults", []) for item in suite.get("assertionResults", [])]
    matches = {item["fullName"] for item in assertions if item.get("status") == "passed"}
    passed = (report.get("success") is True and set(tests) <= matches
              and all(item.get("status") == "passed" for item in assertions))
    return {"status": "PASS" if passed else "UNVERIFIED", "report": report, **evidence}


def reviewed_artifacts(entry, roots, medium):
    """Require recorded observations of current source and retained artifacts."""
    try:
        proof = ROOT / entry["proof"]
        review = read_json(proof)
        root = roots[entry["repository"]]
        if review.get("schema_version") != 1 or review.get("verdict") != "PASS":
            raise ValueError(f"A passing {medium} review is required.")
        required = set(entry["observations"])
        if not required or not all(review["observations"].get(name) is True for name in required):
            raise ValueError(f"The {medium} review omits a required observation.")
        inputs = review["inputs"]
        if not set(entry["required_inputs"]) <= inputs.keys():
            raise ValueError(f"The {medium} review omits required source inputs.")
        for path, digest in inputs.items():
            if hashlib.sha256((root / path).read_bytes()).hexdigest() != digest:
                raise ValueError(f"The {medium} review is stale: {path}")
        captures = review["captures"]
        if not captures:
            raise ValueError(f"The {medium} review has no retained captures.")
        for path, digest in captures.items():
            if hashlib.sha256((proof.parent / path).read_bytes()).hexdigest() != digest:
                raise ValueError(f"A reviewed capture changed: {path}")
        return {"status": "PASS", "reason": f"Recorded {medium} review matches the current inputs.",
                "proof": str(proof), "proof_sha256": hashlib.sha256(proof.read_bytes()).hexdigest(),
                "scope": f"Recorded manual {medium} observations. This invocation did not repeat the capture."}
    except (OSError, ValueError, KeyError, TypeError) as error:
        return {"status": "UNVERIFIED", "reason": str(error)}


def reviewed_demo(entry, roots):
    """Require readable media and edits that preserve the real inference interval."""
    checked = reviewed_artifacts(entry, roots, "media")
    if checked["status"] != "PASS":
        return checked
    try:
        proof = contained_path(ROOT, entry["proof"])
        review = read_json(proof)
        if not {review["capture"], review["render"]} <= review["captures"].keys():
            raise ValueError("The media review must retain capture and render records.")
        capture = read_json(contained_path(proof.parent, review["capture"]))
        render = read_json(contained_path(proof.parent, review["render"]))
        if capture.get("verdict") != "PASS" or capture.get("release") != review["release"]:
            raise ValueError("The demonstration needs a successful capture of its release.")
        if capture.get("response_status") != 200 or capture.get("stream_events", [{}])[-1].get("data") != "[DONE]":
            raise ValueError("The demonstration needs a complete real inference stream.")
        chunks = [json.loads(event["data"]) for event in capture["stream_events"][:-1]]
        if not any(choice.get("delta", {}).get("content") for chunk in chunks for choice in chunk.get("choices", [])):
            raise ValueError("The demonstration contains no model response.")
        if (capture.get("persistent_selectors_present") != [] or capture.get("remaining_home_files") != []
                or capture.get("catalog_environment_has_provider_key") is not False
                or capture.get("shutdown_exit_code") != 0 or capture.get("scratch_removed") is not True):
            raise ValueError("The capture does not establish temporary, keyless first use and cleanup.")
        start, end = capture["inference_start_seconds"], capture["inference_end_seconds"]
        if not 0 <= start < end:
            raise ValueError("The inference interval is invalid.")
        if render["capture_sha256"] != hashlib.sha256(contained_path(proof.parent, review["capture"]).read_bytes()).hexdigest():
            raise ValueError("The renderer used a different capture.")
        if render["width"] < 1280 or render["effective_font_at_900px"] < 14:
            raise ValueError("The demonstration does not meet the readable dimensions.")
        for edit in render["edits"]:
            index = edit["before_event"]
            if not isinstance(index, int) or not 0 < index < len(capture["events"]):
                raise ValueError("The edit does not identify a captured interval.")
            left, right = capture["events"][index - 1]["seconds"], capture["events"][index]["seconds"]
            if (abs(edit["original_gap_seconds"] - (right - left)) > 0.000001
                    or not math.isfinite(edit["edited_gap_seconds"]) or edit["edited_gap_seconds"] < 0):
                raise ValueError("The edit does not match the captured interval.")
            if left < end and right > start:
                raise ValueError("An edit changes the inference interval.")
        for name in ("first-use.gif", "first-use-uncut.gif"):
            artifact = contained_path(roots[entry["repository"]], entry["asset_directory"] + "/" + name)
            output = render["outputs"][name]
            if output["sha256"] != hashlib.sha256(artifact.read_bytes()).hexdigest() or output["bytes"] != artifact.stat().st_size:
                raise ValueError("The rendered artifact changed.")
            header = artifact.read_bytes()[:10]
            if (header[:6] not in (b"GIF87a", b"GIF89a")
                    or int.from_bytes(header[6:8], "little") != render["width"]
                    or int.from_bytes(header[8:10], "little") != render["height"]):
                raise ValueError("The GIF dimensions do not match the render record.")
        if render["outputs"]["first-use.gif"]["bytes"] >= 10 * 1024 * 1024:
            raise ValueError("The GIF exceeds the project size budget.")
        return checked
    except (OSError, ValueError, KeyError, TypeError, IndexError) as error:
        return {"status": "UNVERIFIED", "reason": str(error)}


REHEARSAL_OUTPUTS = ("first-use.gif", "poster.png", "events.json", "render.json", "TRANSCRIPT.md")
REHEARSAL_VERIFIER = "scripts/verify-readme-demo.sh"
REHEARSAL_VERIFIER_CHECKS = (
    "output_hashes", "gif_dimensions", "gif_budget", "effective_font", "scene_order", "catalog_keyless",
    "answer_stream", "cuts_outside_inference", "uncut_source", "no_fixture_token", "poster_dimensions",
    "transcript_fixtures", "human_review", "readme_not_linking_rehearsal", "invalidation")
REHEARSAL_VERIFIER_TIMEOUT_SECONDS = 300


def rehearsal_demo(entry, roots):
    """Check the candidate rehearsal that the Starport demonstration manifest names.

    An absent repository or manifest gives UNVERIFIED because no rehearsal exists yet.
    After the manifest exists, every claim in it must hold, and a broken claim gives FAIL.
    """
    root = roots.get(entry.get("repository"))
    if root is None:
        return {"status": "UNVERIFIED", "reason": "The Starport repository is unavailable."}
    try:
        manifest_path = contained_path(root, entry["manifest"])
    except (KeyError, TypeError, ValueError):
        return {"status": "FAIL", "reason": "The rehearsal check needs a manifest inside the repository."}
    if not manifest_path.is_file():
        return {"status": "UNVERIFIED", "reason": f"The demonstration manifest is absent: {entry['manifest']}"}
    if entry.get("mode") == "record":
        return rehearsal_record(root, manifest_path)
    if entry.get("mode") == "verifier":
        return rehearsal_verifier(root, entry["manifest"])
    return {"status": "FAIL", "reason": "Unknown rehearsal check mode."}


def rehearsal_identity(value):
    return (isinstance(value, str) and bool(value.strip())) or (type(value) is int and value > 0)


def validate_rehearsal_record(record, scenes):
    if record.get("kind") != "rehearsal":
        raise ValueError("The rehearsal record must have kind rehearsal.")
    if record.get("qualifies_release_cases") is not False:
        raise ValueError("A rehearsal record cannot qualify release cases.")
    candidate = record.get("candidate")
    if not isinstance(candidate, dict) or candidate.get("source") != "ci-run":
        raise ValueError("The rehearsal candidate must come from a CI run.")
    for name in ("run_id", "pull_request", "snapshot_version", "archive_name"):
        if not rehearsal_identity(candidate.get(name)):
            raise ValueError(f"The rehearsal candidate omits {name}.")
    if not isinstance(candidate.get("head_commit"), str) or not re.fullmatch(r"[0-9a-f]{40}", candidate["head_commit"]):
        raise ValueError("The rehearsal candidate needs a 40-character head commit.")
    for name in ("archive_sha256", "binary_sha256"):
        if not isinstance(candidate.get(name), str) or not re.fullmatch(r"[0-9a-f]{64}", candidate[name]):
            raise ValueError(f"The rehearsal candidate needs a SHA-256 {name}.")
    fixtures = record.get("fixtures")
    if not isinstance(fixtures, list) or not fixtures:
        raise ValueError("The rehearsal record must disclose its fixtures.")
    cuts = record.get("cuts")
    if not isinstance(cuts, list) or not cuts:
        raise ValueError("The rehearsal record must name at least one cut.")
    for cut in cuts:
        boundary = cut.get("boundary") if isinstance(cut, dict) else None
        parts = boundary.split("/") if isinstance(boundary, str) else []
        if len(parts) != 2 or not all(part in scenes for part in parts):
            raise ValueError(f"A rehearsal cut has no scene boundary: {boundary!r}")


def rehearsal_record(root, manifest_path):
    """Require an identified, disclosed candidate rehearsal whose retained files match their hashes."""
    try:
        manifest = read_json(manifest_path)
        scenes = manifest.get("scenes")
        directory = manifest.get("current_rehearsal")
        if (not isinstance(scenes, list) or not scenes or not all(isinstance(scene, str) for scene in scenes)
                or not isinstance(directory, str) or not directory):
            raise ValueError("The demonstration manifest must name its scenes and current rehearsal.")
        record_directory = contained_path(root, directory)
        record_path = contained_path(record_directory, "record.json")
        if not record_path.is_file():
            raise ValueError(f"The current rehearsal has no record: {directory}/record.json")
        record = read_json(record_path)
        if not isinstance(record, dict):
            raise ValueError("The rehearsal record must be a JSON object.")
        validate_rehearsal_record(record, scenes)
        outputs = record.get("outputs")
        if not isinstance(outputs, dict) or not set(REHEARSAL_OUTPUTS) <= outputs.keys():
            raise ValueError("The rehearsal record omits a required output.")
        uncut = record.get("uncut")
        if not isinstance(uncut, dict) or not isinstance(uncut.get("path"), str) or not uncut["path"]:
            raise ValueError("The rehearsal record must name its uncut source.")
        for name, output in [*outputs.items(), (uncut["path"], uncut)]:
            path = contained_path(record_directory, name)
            if not path.is_file():
                raise ValueError(f"A rehearsal file is absent: {name}")
            if not isinstance(output, dict) or output.get("sha256") != hashlib.sha256(path.read_bytes()).hexdigest():
                raise ValueError(f"A rehearsal file does not match its recorded hash: {name}")
        candidate = record["candidate"]
        return {"status": "PASS", "reason": "The current rehearsal record is an identified, disclosed candidate rehearsal.",
                "record": str(record_path), "record_sha256": hashlib.sha256(record_path.read_bytes()).hexdigest(),
                "candidate_head": candidate["head_commit"], "run_id": candidate["run_id"],
                "scope": "Candidate rehearsal identity and retained file hashes. It cannot qualify final release cases."}
    except json.JSONDecodeError:
        return {"status": "FAIL", "reason": "The demonstration manifest or rehearsal record contains invalid JSON."}
    except UnicodeError:
        return {"status": "FAIL", "reason": "The demonstration manifest or rehearsal record contains invalid text."}
    except ValueError as error:
        return {"status": "FAIL", "reason": str(error)}
    except (OSError, AttributeError, TypeError) as error:
        return {"status": "FAIL", "reason": f"The rehearsal record cannot be read: {type(error).__name__}"}


def rehearsal_report(stdout):
    """Parse the JSON document that ends the demonstration verifier output."""
    try:
        return json.loads(stdout)
    except json.JSONDecodeError:
        return json.loads(stdout[stdout.rfind("\n{") + 1:])


def rehearsal_verifier(root, manifest):
    """Run the Starport demonstration verifier and require a complete passing report."""
    if not (root / REHEARSAL_VERIFIER).is_file():
        return {"status": "FAIL", "reason": f"The demonstration verifier is absent: {REHEARSAL_VERIFIER}"}
    command = ["bash", REHEARSAL_VERIFIER, "--manifest", manifest, "--json"]
    try:
        result = subprocess.run(command, cwd=root, capture_output=True, text=True,
                                timeout=REHEARSAL_VERIFIER_TIMEOUT_SECONDS)
    except (OSError, subprocess.TimeoutExpired) as error:
        return {"status": "UNVERIFIED", "reason": f"The demonstration verifier did not complete: {type(error).__name__}",
                "command": command, "cwd": str(root)}
    evidence = {"command": command, "cwd": str(root), "exit_code": result.returncode, "stderr": result.stderr}
    try:
        report = rehearsal_report(result.stdout)
        if not isinstance(report, dict) or not isinstance(report.get("checks"), list):
            raise ValueError
    except ValueError:
        return {"status": "FAIL", "reason": "The demonstration verifier printed no valid JSON report.",
                "stdout": result.stdout, **evidence}
    evidence.update(report=report, checks=report["checks"])
    status = report.get("status")
    checks = [check if isinstance(check, dict) else {} for check in report["checks"]]
    failed = sorted(str(check.get("id")) for check in checks if check.get("status") != "PASS")
    if result.returncode == 2 or status == "INVALID":
        return {"status": "FAIL", "reason": "The demonstration verifier reports INVALID: "
                "a product path changed after the candidate head.", **evidence}
    if result.returncode != 0 or status != "PASS":
        return {"status": "FAIL", "reason": f"The demonstration verifier reports {status} with exit code "
                f"{result.returncode}. Failed checks: {', '.join(failed) or 'none named'}.", **evidence}
    missing = sorted(set(REHEARSAL_VERIFIER_CHECKS) - {str(check.get("id")) for check in checks})
    if failed or missing:
        return {"status": "FAIL", "reason": "The demonstration verifier report is incomplete. "
                f"Failed checks: {', '.join(failed) or 'none'}. Missing checks: {', '.join(missing) or 'none'}.", **evidence}
    return {"status": "PASS", "reason": "The demonstration verifier passed every required check in this invocation.",
            "scope": "Candidate rehearsal media and invalidation checks. It cannot qualify final release cases.",
            **evidence}


def reviewed_first_use(entry, roots):
    """Validate retained installer and inference evidence against the reviewed README inputs."""
    try:
        proof = contained_path(ROOT, entry["proof"])
        review = read_json(proof)
        root = roots[entry["repository"]]
        if review.get("schema_version") != 1 or review.get("verdict") != "PASS":
            raise ValueError("A passing first-use review is required.")
        required = set(entry["observations"])
        if not required or not all(review["observations"].get(name) is True for name in required):
            raise ValueError("The first-use review omits a required observation.")
        inputs = review["inputs"]
        if not entry["required_inputs"] or not set(entry["required_inputs"]) <= inputs.keys():
            raise ValueError("The first-use review omits required source inputs.")
        for name, digest in inputs.items():
            if hashlib.sha256(contained_path(root, name).read_bytes()).hexdigest() != digest:
                raise ValueError(f"The first-use review is stale: {name}")
        captures = review["captures"]
        if not captures:
            raise ValueError("The first-use review has no retained captures.")
        for name, digest in captures.items():
            if hashlib.sha256(contained_path(proof.parent, name).read_bytes()).hexdigest() != digest:
                raise ValueError(f"A reviewed capture changed: {name}")
        methods = review["methods"]
        if not entry["methods"] or set(methods) != set(entry["methods"]):
            raise ValueError("Every advertised installation method needs its native evidence.")
        release = review["release"]
        if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", release):
            raise ValueError("The review must identify the published release.")
        for name, method in methods.items():
            if method.get("verdict") != "PASS" or method.get("native") is not True or not method.get("platform"):
                raise ValueError(f"Native installation evidence is incomplete: {name}")
            if not method.get("captures") or not set(method["captures"]) <= captures.keys():
                raise ValueError(f"The installation method has no retained evidence: {name}")
            artifact_kind = method.get("artifact_kind")
            if artifact_kind not in ("release", "source"):
                raise ValueError(f"Unknown installation artifact kind: {name}")
            if artifact_kind == "release" and method.get("release") != release:
                raise ValueError(f"The installation release does not match the review: {name}")
            if artifact_kind == "source" and not re.fullmatch(r"[0-9a-f]{40}", method.get("source_commit", "")):
                raise ValueError(f"The installation artifact has no matching identity: {name}")
        inference_name = review["inference_capture"]
        if inference_name not in captures:
            raise ValueError("Real inference evidence must be retained.")
        inference = read_json(contained_path(proof.parent, inference_name))
        if (inference.get("verdict") != "PASS" or inference.get("release") != release
                or inference.get("response_status") != 200 or inference.get("content_type") != "text/event-stream"
                or inference.get("request", {}).get("stream") is not True):
            raise ValueError("The release needs successful real streamed inference.")
        stream = inference.get("stream", "")
        events = [line[6:] for line in stream.splitlines() if line.startswith("data: ")]
        if not events or events[-1] != "[DONE]":
            raise ValueError("The inference stream did not complete.")
        chunks = [json.loads(event) for event in events[:-1]]
        if not any(choice.get("delta", {}).get("content") for chunk in chunks for choice in chunk.get("choices", [])):
            raise ValueError("The inference stream contains no model response.")
        return {"status": "PASS", "proof": str(proof),
                "proof_sha256": hashlib.sha256(proof.read_bytes()).hexdigest(),
                "scope": "Recorded native installation and README review. This invocation did not reinstall software or call a paid provider."}
    except (OSError, ValueError, KeyError, TypeError, AttributeError) as error:
        return {"status": "UNVERIFIED", "reason": str(error)}


def contained_path(root, name):
    candidate = (root / name).resolve()
    if Path(name).is_absolute() or not candidate.is_relative_to(root.resolve()):
        raise ValueError("Evidence paths must stay inside their repository.")
    return candidate


def validate_measurement_record(measurement, decision):
    if type(measurement.get("version")) is not int or measurement["version"] != 1:
        raise ValueError("The measurement record needs version 1.")
    if measurement.get("operator_vcs_modified") is not False or measurement.get("operator_race") is not True:
        raise ValueError("The operator binary must be unmodified and use race instrumentation.")
    runs = measurement.get("runs")
    if not isinstance(runs, list) or len(runs) != 9:
        raise ValueError("The measurement record needs nine runs.")
    scenarios = {"empty-target-import": 0, "valkey-restart-persistent": 0,
                 "replica-promotion-acknowledged-loss": 0}
    outages = []
    for run in runs:
        scenario = run.get("scenario")
        if scenario not in scenarios:
            raise ValueError("The measurement record contains an unknown scenario.")
        scenarios[scenario] += 1
        if run.get("complete") is not True:
            raise ValueError("Every measurement run must be complete.")
        commands = run.get("commands")
        if (not isinstance(commands, list) or not commands
                or any(type(command.get("exit_status")) is not int or command["exit_status"] != 0
                       for command in commands)):
            raise ValueError("Every measurement command must exit with status zero.")
        if type(run.get("readiness_http_status")) is not int or run["readiness_http_status"] != 200:
            raise ValueError("Every measurement run needs HTTP readiness status 200.")
        expected_loss = 1 if scenario == "replica-promotion-acknowledged-loss" else 0
        if type(run.get("lost_writes")) is not int or run["lost_writes"] != expected_loss:
            raise ValueError("A measurement run violates its scenario loss rule.")
        if scenario == "replica-promotion-acknowledged-loss" and (
                type(run.get("acknowledged_writes")) is not int or run["acknowledged_writes"] != 2):
            raise ValueError("Every promotion run needs two acknowledged writes.")
        outage = run.get("outage_to_ready_seconds")
        if type(outage) not in (int, float) or not math.isfinite(outage) or outage < 0:
            raise ValueError("Every measurement run needs a finite nonnegative readiness duration.")
        outages.append(outage)
    if any(count != 3 for count in scenarios.values()):
        raise ValueError("Every measurement scenario needs three runs.")
    if decision.get("status") != "CONFIRMED" or decision.get("decision") != "D42":
        raise ValueError("The measurement decision must confirm D42.")
    source_head, prefix = measurement.get("source_head"), decision.get("measurement_source_head")
    if (not isinstance(source_head, str) or not isinstance(prefix, str) or not prefix
            or not source_head.startswith(prefix)):
        raise ValueError("The measurement decision must match the source head prefix.")
    rto = decision.get("approved_targets", {}).get("reference_rto_seconds")
    if type(rto) not in (int, float) or not math.isfinite(rto) or rto <= 0:
        raise ValueError("The measurement decision needs a positive finite RTO target.")
    maximum = max(outages)
    if maximum > rto:
        raise ValueError("A measurement run exceeds the approved RTO target.")
    return {"rto_seconds": rto, "max_outage_to_ready_seconds": maximum, "runs": len(runs)}


def reviewed_measurement_record(entry, roots):
    root = roots.get(entry.get("repository"))
    if root is None:
        return {"status": "UNVERIFIED", "reason": "The measurement repository is unavailable."}
    try:
        record = contained_path(root, entry["record"])
        measurement = read_json(contained_path(record, "measurement.json"))
        decision = read_json(contained_path(record, "decision.json"))
        proof = validate_measurement_record(measurement, decision)
        return dict(proof, status="PASS", reason="The measurement record meets the confirmed loss and RTO targets.")
    except FileNotFoundError:
        return {"status": "UNVERIFIED", "reason": "A measurement record file is absent."}
    except json.JSONDecodeError:
        return {"status": "FAIL", "reason": "A measurement record file contains invalid JSON."}
    except UnicodeError:
        return {"status": "FAIL", "reason": "A measurement record file contains invalid text."}
    except ValueError as error:
        return {"status": "FAIL", "reason": str(error)}
    except (OSError, KeyError, TypeError, AttributeError, OverflowError):
        return {"status": "FAIL", "reason": "The measurement record cannot be read or has invalid fields."}


def validate_performance_baseline(report, count):
    if report.get("schema_version") != 1 or report.get("profile") != "local-quiescent-baseline-v1":
        raise ValueError("Unknown baseline schema or measurement profile.")
    if report.get("qualification") != "UNVERIFIED":
        raise ValueError("A local baseline cannot grant production qualification.")
    for key in ("go_version", "os", "architecture", "catalog_generation", "network", "storage", "limitations"):
        if not isinstance(report.get(key), str) or not report[key].strip():
            raise ValueError(f"The baseline omits {key}.")
    if not re.fullmatch(r"sha256:[a-f0-9]{64}", report.get("catalog_checksum", "")):
        raise ValueError("The baseline needs an exact catalog checksum.")
    if report.get("concurrency") != 1 or report.get("response_cache") != "disabled":
        raise ValueError("The baseline must retain its declared uncached serial workload.")
    if report.get("metrics") != "on" or report.get("usage_capture") != "on":
        raise ValueError("The baseline must include its declared observability.")
    if any(type(report.get(key)) is not int or report[key] <= 0 for key in ("catalog_routes", "gomaxprocs", "controlled_wait_per_event_ns")):
        raise ValueError("The baseline needs catalog, process and deliberate wait bounds.")
    pairs = report.get("warm_pairs", [])
    if len(pairs) != 2 * count or len(report.get("initial_pairs", [])) != 2:
        raise ValueError("The baseline omits required request pairs.")
    initial = report["initial_pairs"]
    if {pair.get("stream") for pair in initial} != {False, True}:
        raise ValueError("Initial samples must cover both response variants.")
    for stream in (False, True):
        group = [pair for pair in pairs if pair.get("stream") is stream]
        if len(group) != count or {pair.get("order") for pair in group} != {"direct-first", "proxy-first"}:
            raise ValueError("Each response variant needs all pairs and both request orders.")
        for pair in group + [pair for pair in initial if pair.get("stream") is stream]:
            for side in ("direct", "proxied"):
                sample = pair[side]
                for key in ("elapsed_ns", "before_upstream_ns", "controlled_wait_ns", "wait_adjusted_elapsed_ns",
                            "first_byte_ns", "headers_received_ns", "client_connection_acquisition_ns", "response_bytes", "request_bytes",
                            "gateway_handler_ns", "first_token_ns"):
                    value = sample[key]
                    if type(value) is not int or value < 0:
                        raise ValueError(f"Invalid baseline sample field: {key}")
                if not 0 < sample["first_byte_ns"] <= sample["headers_received_ns"] <= sample["elapsed_ns"]:
                    raise ValueError("First byte, headers and completion are inconsistent.")
                if sample["wait_adjusted_elapsed_ns"] != sample["elapsed_ns"] - sample["controlled_wait_ns"]:
                    raise ValueError("The baseline subtracts more than the measured controlled wait.")
                if (not 0 < sample["request_bytes"] or not 0 < sample["response_bytes"]
                        or type(sample["client_connection_reused"]) is not bool
                        or not sample["client_connection_acquisition_ns"] <= sample["before_upstream_ns"] <= sample["first_byte_ns"]
                        or sample["controlled_wait_ns"] < report["controlled_wait_per_event_ns"] * (3 if stream else 1)):
                    raise ValueError("The request, connection or deliberate wait evidence is inconsistent.")
                if side == "proxied" and sample["gateway_handler_ns"] <= sample["controlled_wait_ns"]:
                    raise ValueError("Complete gateway handler evidence is absent.")
                events = sample["event_forwarding_ns"] or []
                if len(events) != (3 if stream else 0) or any(type(v) is not int or v < 0 for v in events):
                    raise ValueError("Stream forwarding samples are missing or invalid.")
                if not stream and sample["first_token_ns"] != 0:
                    raise ValueError("A non-stream response cannot have a content-event timestamp.")
                if stream and not sample["first_byte_ns"] <= sample["first_token_ns"] <= sample["elapsed_ns"]:
                    raise ValueError("First content event timing is inconsistent.")
            expected = pair["proxied"]["wait_adjusted_elapsed_ns"] - pair["direct"]["wait_adjusted_elapsed_ns"]
            if type(pair["paired_adjusted_delta_ns"]) is not int or pair["paired_adjusted_delta_ns"] != expected:
                raise ValueError("The baseline must retain each paired difference, including negative differences.")
    return {"warm_pairs": len(pairs), "initial_pairs": 2, "measurement_scope": report["limitations"]}


def run_performance_baseline(entry, roots):
    root = roots.get(entry.get("repository"))
    if root is None or not (root / "internal/app/performance_test.go").is_file():
        return {"status": "UNVERIFIED", "reason": "The complete HTTP measurement harness is unavailable."}
    with tempfile.TemporaryDirectory(prefix="catalog-performance-") as directory:
        output = Path(directory) / "baseline.json"
        binary = Path(directory) / "app.test"
        command = ["go", "test", "-count=1", "-timeout", "180s", "-json", "-run",
                   "^TestFullPathMeasurementBaseline$", "-o", str(binary), "./internal/app"]
        environment = {**os.environ, "STARPORT_FULL_PATH_SAMPLES": "100", "STARPORT_FULL_PATH_REPORT": str(output)}
        try:
            result = subprocess.run(command, cwd=root, env=environment, capture_output=True, text=True, timeout=210)
            evidence = {"command": command, "cwd": str(root), "exit_code": result.returncode,
                        "stdout": result.stdout, "stderr": result.stderr}
            if result.returncode:
                return {"status": "FAIL", "reason": "The complete HTTP baseline failed.", **evidence}
            events = [json.loads(line) for line in result.stdout.splitlines() if line.startswith("{")]
            if any(event.get("Action") == "skip" for event in events) or not any(
                event.get("Test") == "TestFullPathMeasurementBaseline" and event.get("Action") == "pass" for event in events
            ):
                raise ValueError("The complete HTTP measurement test did not pass in this invocation.")
            report = read_json(output)
            counts = validate_performance_baseline(report, 100)
            return {"status": "PASS", "reason": "Fresh complete local HTTP baseline, not production qualification.",
                    "report": report, "test_binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
                    "measurement_sha256": hashlib.sha256(output.read_bytes()).hexdigest(), **counts, **evidence}
        except (OSError, ValueError, KeyError, TypeError, subprocess.TimeoutExpired) as error:
            return {"status": "UNVERIFIED", "reason": str(error), "command": command}


def validate_performance_profile(profile):
    if (profile.get("schema_version") != 1 or profile.get("status") != "engineering_targets"
            or profile.get("qualification") != "UNVERIFIED"):
        raise ValueError("The numeric profile must state unqualified engineering targets.")
    required = {
        "resources": ("request_allocated_bytes", "request_allocations", "stream_event_allocated_bytes", "stream_event_allocations",
                      "active_stream_retained_bytes", "replica_live_heap_bytes", "replica_rss_bytes", "cpu_utilization_percent",
                      "gc_cpu_percent", "gc_pause_p99_ms", "retained_generations", "cold_load_concurrency", "optional_queue_entries",
                      "optional_queue_bytes", "response_cache_bytes"),
        "deadlines_ms": ("local_optional_cache_read", "remote_optional_cache_read", "optional_enqueue", "cold_policy_load",
                         "admission", "cancellation_propagation_p99", "stream_idle", "cold_connection"),
        "workload": ("input_bytes", "output_bytes", "prompt_tokens", "completion_tokens", "catalog_routes", "tenants",
                     "concurrency_per_replica", "offered_requests_per_second_per_replica", "stream_events", "event_content_bytes",
                     "offered_stream_requests_per_second_per_replica", "controlled_upstream_first_response_ms",
                     "controlled_upstream_inter_event_ms", "max_stream_seconds"),
    }
    for group, keys in required.items():
        for key in keys:
            value = profile[group][key]
            if type(value) not in (int, float) or not math.isfinite(value) or value <= 0:
                raise ValueError(f"The numeric profile needs a positive finite {group}.{key}.")
    for name, stores in (("local", ["badger", "sqlite", "filesystem"]), ("fleet", ["valkey", "postgresql", "objectstore"])):
        recipe = profile["recipes"][name]
        if recipe["stores"] != stores or recipe["replicas"] != (1 if name == "local" else 3):
            raise ValueError("A primary performance recipe is incomplete.")
        limits = recipe["latency_ms"]
        for key in ("p50", "p95", "p99", "p999", "first_byte_added_p99", "first_token_added_p99", "stream_forwarding_p99"):
            value = limits[key]
            if type(value) not in (int, float) or not math.isfinite(value) or value <= 0:
                raise ValueError("Latency targets must be positive finite numbers.")
        if not limits["p50"] <= limits["p95"] <= limits["p99"] <= limits["p999"]:
            raise ValueError("Latency percentile targets must be ordered.")
    correctness = profile["correctness"]
    if (correctness["gateway_authorization_lifetime_seconds"] != 60 or correctness["revocation_propagation_target_seconds"] != 2
            or correctness["authority_receipt_maximum_clock_uncertainty_seconds"] != 30
            or correctness["gateway_authorization_clock"] != "suspend-aware-elapsed"
            or correctness["authority_receipt_clock_contract"] != "qualified-utc-unless-receipt-allows-conservative-elapsed-deadline"
            or correctness["admission_mode"] != "atomic-per-attempt" or correctness["unknown_required_budget"] != "refuse-retryable"
            or correctness["authority_activation_failure"] != "block-new-inference" or correctness["unknown_clock"] != "refuse-affected-operation"):
        raise ValueError("Performance cannot weaken the accepted correctness contract.")
    evidence = profile["evidence"]
    if (evidence["runs"] < 3 or evidence["minimum_samples_per_variant"] < 100_000 or evidence["minimum_seconds_per_run"] < 600
            or evidence["raw_samples_required"] is not True or evidence["open_loop_load"] is not True
            or evidence["paired_measurement"] is not True or evidence["final_artifact_binding"] is not True
            or evidence["negative_deltas"] != "retain"):
        raise ValueError("The numeric profile lacks required qualification evidence rules.")
    if profile["runner"]["dedicated"] is not True or profile["runner"]["exact_cpu_os_toolchain_artifact_required"] is not True:
        raise ValueError("Qualification needs identified dedicated runners.")
    if correctness["controlled_backend_recovery"] is not True:
        raise ValueError("Performance qualification must include controlled backend recovery.")
    if correctness["retained_generation_limit_action"] != "delay-nonrestrictive-activation-and-block-new-inference-when-required-policy-cannot-activate":
        raise ValueError("Generation resource bounds cannot permit stale policy.")
    expected_sets = {"response_cache_states": {"disabled", "miss", "hit"},
                     "credential_sources": {"environment", "shared", "byok"},
                     "authority_modes": {"public", "internal"}, "connection_states": {"pooled", "cold"}}
    for name, expected in expected_sets.items():
        values = profile["workload"][name]
        if len(values) != len(set(values)) or set(values) != expected:
            raise ValueError(f"The workload omits required {name} variants.")
    exercises = {"large-input-1MiB", "long-stream-10000-events", "retry-before-stream-commit", "generation-refresh-pressure",
                 "cold-start", "backend-outage", "saturation", "slow-client", "connection-reconnect", "cancellation",
                 "cache-failure", "credential-rotation", "restrictive-policy-activation"}
    actual = profile["qualification_exercises"]
    if len(actual) != len(set(actual)) or not exercises <= set(actual):
        raise ValueError("The performance profile omits a required failure or load exercise.")
    if any(profile["observability"].get(name) != "on" for name in ("request_logging", "metrics", "usage_capture")):
        raise ValueError("Performance qualification must retain mandatory observability.")
    for name in ("runs", "minimum_samples_per_variant", "minimum_seconds_per_run"):
        if type(evidence[name]) is not int:
            raise ValueError("Qualification sample and run counts must be integers.")
    if (evidence["confidence"] != 0.95 or not 0 < evidence["maximum_regression_percent"] <= 10
            or evidence["percentile_method"] != "nearest-rank-on-per-pair-differences"
            or evidence["provider_wait_method"] != "subtract-only-observed-controlled-waits"
            or evidence["cpu_and_allocation_profiles"] is not True or evidence["compare_each_variant_separately"] is not True):
        raise ValueError("The profile weakens its timing or uncertainty evidence rules.")
    resources = profile["resources"]
    if (not 0 < resources["gc_cpu_percent"] <= resources["cpu_utilization_percent"] <= 100
            or resources["replica_live_heap_bytes"] > resources["replica_rss_bytes"]
            or resources["response_cache_bytes"] > resources["replica_live_heap_bytes"]):
        raise ValueError("The resource limits contradict each other.")
    rtt = profile["recipes"]["fleet"]["max_store_rtt_p99_ms"]
    if type(rtt) not in (int, float) or not math.isfinite(rtt) or rtt <= 0:
        raise ValueError("Fleet targets require a positive finite store RTT bound.")
    for group, names in {"workload": ("size_definition", "latency_target_state"),
                         "resources": ("measurement_definition",),
                         "evidence": ("boundary_definition", "confidence_method", "regression_rule", "minimum_duration_rule"),
                         "rationale": ("latency", "allocations", "correctness", "measurement")}.items():
        if any(not isinstance(profile[group].get(name), str) or not profile[group][name].strip() for name in names):
            raise ValueError("The profile omits a measurement boundary or rationale.")


def reviewed_performance_profile(entry, roots):
    try:
        root = roots[entry["repository"]]
        profile_path = contained_path(root, entry["profile"])
        proof_path = contained_path(ROOT, entry["proof"])
        profile, review = read_json(profile_path), read_json(proof_path)
        validate_performance_profile(profile)
        digest = hashlib.sha256(profile_path.read_bytes()).hexdigest()
        observations = ("timing_boundaries", "latency_and_workload", "resources_and_deadlines", "correctness",
                        "qualification_method", "baseline_costs", "no_current_release_claim")
        if (review.get("schema_version") != 1 or review.get("verdict") != "PASS" or review.get("profile_sha256") != digest
                or not all(review.get("observations", {}).get(name) is True for name in observations)):
            raise ValueError("The numeric profile needs a current engineering review.")
        details = review.get("assessment", {})
        if any(not isinstance(details.get(name), str) or not details[name].strip() for name in observations):
            raise ValueError("The numeric review needs an assessment for each observation.")
        evidence = review.get("evidence", {})
        if not evidence:
            raise ValueError("The numeric review has no baseline evidence.")
        for name, expected in evidence.items():
            path = contained_path(proof_path.parent, name)
            if hashlib.sha256(path.read_bytes()).hexdigest() != expected:
                raise ValueError("Reviewed baseline evidence changed.")
        return {"status": "PASS", "reason": "Reviewed engineering targets, not a claim that current performance meets them.",
                "profile_sha256": digest, "review_sha256": hashlib.sha256(proof_path.read_bytes()).hexdigest()}
    except (OSError, ValueError, KeyError, TypeError) as error:
        return {"status": "UNVERIFIED", "reason": str(error)}


def aggregate(roster, selected, results, qualification_required):
    cases = []
    for case, group in roster["required_subcases"].items():
        evidence = [results.get(item, {"status": "UNVERIFIED"}) for item in group]
        states = ["UNVERIFIED" if result["status"] == "PASS" and result.get("evidence_scope") == "producer_component"
                  else result["status"] for result in evidence]
        status = "FAIL" if "FAIL" in states else "UNVERIFIED" if "UNVERIFIED" in states else "PASS"
        cases.append({"id": case, "condition": "CSP-V" + case[1:], "status": status, "required_subcases": len(group)})
    passed = sum(case["status"] == "PASS" for case in cases)
    failed = 50 - passed
    gate_ok = all(results[item]["status"] == "PASS" for item in selected)
    if qualification_required:
        gate_ok = False
    return {"cases": cases, "subcases": results, "selected_subcases": len(selected),
            "summary": f"Summary: {passed} passed, {failed} failed",
            "unverified_cases": sum(case["status"] == "UNVERIFIED" for case in cases),
            "gate_status": "PASS" if gate_ok else "FAIL",
            "qualification": "UNVERIFIED" if qualification_required else "component checks only"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--starport-root", type=Path, default=ROOT.parent / "starport")
    choice = parser.add_mutually_exclusive_group()
    choice.add_argument("--all", action="store_true")
    choice.add_argument("--case", action="append")
    choice.add_argument("--task")
    choice.add_argument("--gate", choices=["candidate", "final"])
    parser.add_argument("--released-assets", action="store_true")
    parser.add_argument("--recipes", action="store_true")
    parser.add_argument("--backends", choices=["primary", "all"])
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()
    try:
        roster, registry = read_json(ROSTER), read_json(REGISTRY)
        checks = validate_roster(roster)
        validate_registry(registry, roster, checks)
        selected = select_checks(args, roster)
        roots = {"starmap": ROOT, "starport": args.starport_root.resolve()}
        results = {}
        go_evidence = GoEvidence([registered_check(args, registry, item)[0] for item in selected], roots)
        for item in selected:
            entry, scope = registered_check(args, registry, item)
            results[item] = run_check(item, entry, roots, go_evidence)
            results[item]["evidence_scope"] = scope
        publication_cases = set(roster["qualification"]["requires_published_assets"])
        qualification_required = bool(
            args.gate or args.released_assets or args.recipes or args.backends
            or (not args.task and not args.case)
            or publication_cases.intersection(args.case or [])
            or args.task in ("CSP15", "CSP19", "CSP22", "CSP23", "CSP24")
        )
        report = aggregate(roster, selected, results, qualification_required)
        report["roster_sha256"] = hashlib.sha256(ROSTER.read_bytes()).hexdigest()
        report["registry_sha256"] = hashlib.sha256(REGISTRY.read_bytes()).hexdigest()
        report["backend_scope"] = args.backends or "primary"
        if qualification_required:
            report["qualification_reason"] = "Real environment and final artifact qualification remain unimplemented. Baseline and target-profile checks cannot qualify a release."
        if args.json:
            print(json.dumps(report, indent=2))
        else:
            for case in report["cases"]:
                print(f"{case['condition']} {case['status']}: {case['id']}")
            print(report["summary"])
            print(f"UNVERIFIED: {report['unverified_cases']} cases. Gate: {report['gate_status']}.")
        return 0 if report["gate_status"] == "PASS" else 1
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(json.dumps({"gate_status": "FAIL", "error": str(error)}) if args.json else f"FAIL: {error}")
        return 2


if __name__ == "__main__":
    sys.exit(main())
