"""Qualify publisher recovery behavior without claiming hosted publication."""

import json
import subprocess
import sys


RECOVERY_TESTS = (
    "PublicationRecoveryTests.test_recover_from_retained_workflow_without_new_acquisition",
    "PublicationRecoveryTests.test_public_checkpoint_survives_loss_of_original_working_directory",
    "PublicationRecoveryTests.test_ambiguous_publish_response_reuses_uploaded_assets",
    "PublicationRecoveryTests.test_partial_draft_asset_recovers_without_replacing_uploaded_data",
    "PublicationRecoveryTests.test_each_release_write_boundary_recovers_without_duplicate_assets",
    "PublicationRecoveryTests.test_changed_public_asset_stops_without_overwrite",
    "PublicationRecoveryTests.test_authenticator_refusal_stops_before_staging",
    "PublicationRecoveryTests.test_pending_completion_requires_both_channels_and_exact_checkpoint",
    "PublicationRecoveryTests.test_interrupted_promotion_merges_exact_artifact_before_both_channels",
    "GitPublicationTests.test_concurrent_branch_change_rejects_stale_publication",
)
WRITE_BOUNDARY_TEST = "__main__." + RECOVERY_TESTS[4]


def validate_recovery(report):
    expected = {"__main__." + name for name in RECOVERY_TESTS}
    passed = report["passed"]
    if (type(report["tests_run"]) is not int or report["tests_run"] != len(expected)
            or not isinstance(passed, list) or len(passed) != len(expected) or set(passed) != expected):
        raise ValueError("Publisher recovery evidence omits or duplicates a required test.")
    for name in ("skipped", "failures", "errors", "expected_failures", "unexpected_successes"):
        if report[name] != []:
            raise ValueError("Publisher recovery evidence includes an incomplete or failed test.")
    subcases = report["subcases"]
    if not isinstance(subcases, list) or any(item.get("passed") is not True for item in subcases):
        raise ValueError("Publisher recovery evidence includes an unsuccessful subcase.")
    boundaries = [item for item in subcases if item.get("test") == WRITE_BOUNDARY_TEST]
    expected_boundaries = {
        f"{WRITE_BOUNDARY_TEST} (kind='{kind}', operation='{operation}', after_write={after})"
        for kind in ("receipt", "artifact")
        for operation in ("create", "upload", "edit")
        for after in (False, True)
    }
    if len(boundaries) != 12 or {item["id"] for item in boundaries} != expected_boundaries:
        raise ValueError("Publisher recovery evidence omits or duplicates a release-write boundary.")


def verify_recovery(root):
    command = [sys.executable, str(root / "scripts/test_catalog_publication.py"), "--json", *RECOVERY_TESTS]
    try:
        result = subprocess.run(command, cwd=root, capture_output=True, text=True, timeout=300)
        if result.returncode:
            return {"status": "FAIL", "command": command, "exit_code": result.returncode,
                    "stdout": result.stdout, "stderr": result.stderr}
        report = json.loads(result.stdout)
        validate_recovery(report)
        return {"status": "PASS", "scope": "Real catalog tools and Git with simulated GitHub transport.",
                "command": command, "report": report, "hosted_publication": "UNVERIFIED"}
    except (OSError, ValueError, KeyError, TypeError, AttributeError, subprocess.TimeoutExpired) as error:
        return {"status": "UNVERIFIED", "reason": str(error), "command": command}
