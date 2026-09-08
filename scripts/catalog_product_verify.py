#!/usr/bin/env python3
"""Run the catalog plan's registered behavior checks without granting missing evidence a pass."""

import argparse
import hashlib
import importlib.util
import json
import math
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
ROSTER = ROOT / "docs/plans/proof/starport-production-catalog/acceptance-map.json"
REGISTRY = ROOT / "scripts/catalog-product-checks.json"


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


def run_check(identity, entry, roots):
    if entry is None:
        return {"status": "UNVERIFIED", "reason": "No behavior check is registered."}
    if entry.get("kind") == "all":
        children = entry.get("checks", [])
        if not children:
            return {"status": "FAIL", "reason": "A combined check needs evidence."}
        results = [run_check(identity, child, roots) for child in children]
        states = [result["status"] for result in results]
        status = "FAIL" if "FAIL" in states else "UNVERIFIED" if "UNVERIFIED" in states else "PASS"
        return {"status": status, "checks": results}
    if entry.get("kind") == "vitest":
        return run_vitest(entry, roots)
    if entry.get("kind") == "reviewed_ui":
        return reviewed_artifacts(entry, roots, "browser")
    if entry.get("kind") == "reviewed_first_use":
        return reviewed_first_use(entry, roots)
    if entry.get("kind") == "reviewed_demo":
        return reviewed_demo(entry, roots)
    if entry.get("kind") == "performance_baseline":
        return run_performance_baseline(entry, roots)
    if entry.get("kind") == "performance_profile":
        return reviewed_performance_profile(entry, roots)
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
    if entry.get("kind") != "go_test":
        return {"status": "UNVERIFIED", "reason": "This evidence adapter has not been implemented."}
    root = roots.get(entry.get("repository"))
    package = entry.get("package", "")
    test = entry.get("test", "")
    if root is None or not (root / "go.mod").is_file():
        return {"status": "UNVERIFIED", "reason": "The required repository is unavailable."}
    if not re.fullmatch(r"Test[A-Za-z0-9_]+", test) or not package.startswith("./") or ".." in package.split("/")[1:]:
        return {"status": "FAIL", "reason": "Invalid named Go behavior check."}
    command = ["go", "test", "-race", "-count=1", "-timeout", "5m", "-json", "-run", f"^{test}$", package]
    try:
        result = subprocess.run(command, cwd=root, capture_output=True, text=True, timeout=330)
    except (OSError, subprocess.TimeoutExpired) as error:
        return {"status": "UNVERIFIED", "reason": type(error).__name__, "command": command}
    events = []
    for line in result.stdout.splitlines():
        try:
            events.append(json.loads(line))
        except json.JSONDecodeError:
            continue
    matched = [event for event in events if event.get("Test") == test]
    passed = any(event.get("Action") == "pass" for event in matched)
    skipped = any(event.get("Action") == "skip" for event in events)
    if result.returncode:
        status, reason = "FAIL", "The behavior command failed."
    elif not matched or skipped:
        status, reason = "UNVERIFIED", "The named behavior test is missing or a test was skipped."
    elif not passed:
        status, reason = "UNVERIFIED", "The named behavior test has no passing result."
    else:
        status, reason = "PASS", "The named behavior test passed in this invocation."
    return {"status": status, "reason": reason, "command": command, "cwd": str(root),
            "exit_code": result.returncode, "stdout": result.stdout, "stderr": result.stderr}


def run_vitest(entry, roots):
    root = roots.get(entry.get("repository"))
    files, tests = entry.get("files", []), entry.get("tests", [])
    if root is None or not (root / "console/package.json").is_file():
        return {"status": "UNVERIFIED", "reason": "The required console is unavailable."}
    if not files or not tests or len(tests) != len(set(tests)) or any(
        not file.startswith("src/") or not file.endswith(".test.tsx") or ".." in file.split("/") for file in files
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
    if (correctness["permission_validity_seconds"] != 300 or correctness["maximum_clock_uncertainty_seconds"] != 30
            or correctness["admission_mode"] != "atomic-per-attempt" or correctness["unknown_required_budget"] != "refuse-retryable"
            or correctness["authority_activation_failure"] != "block-new-inference" or correctness["unknown_clock"] != "refuse"):
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
        states = [results.get(item, {"status": "UNVERIFIED"})["status"] for item in group]
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
        if registry.get("schema_version") != 1 or not set(registry["checks"]) <= checks:
            raise ValueError("Invalid behavior-check registry.")
        selected = select_checks(args, roster)
        roots = {"starmap": ROOT, "starport": args.starport_root.resolve()}
        results = {item: run_check(item, registry["checks"].get(item), roots) for item in selected}
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
