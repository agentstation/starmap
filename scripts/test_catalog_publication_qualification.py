"""Test the evidence boundary for publication recovery qualification."""

import copy
import json
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

import catalog_publication_qualification as qualification


class RecoveryEvidenceTests(unittest.TestCase):
    def setUp(self):
        boundary = qualification.WRITE_BOUNDARY_TEST
        self.report = {
            "tests_run": len(qualification.RECOVERY_TESTS),
            "passed": ["__main__." + name for name in qualification.RECOVERY_TESTS],
            "skipped": [], "failures": [], "errors": [], "expected_failures": [], "unexpected_successes": [],
            "subcases": [
                {"test": boundary, "id": f"{boundary} (kind='{kind}', operation='{operation}', after_write={after})",
                 "passed": True}
                for kind in ("receipt", "artifact") for operation in ("create", "upload", "edit")
                for after in (False, True)
            ],
        }

    def test_complete_report_qualifies_only_local_recovery(self):
        output = subprocess.CompletedProcess([], 0, json.dumps(self.report), "")
        with patch.object(qualification.subprocess, "run", return_value=output):
            result = qualification.verify_recovery(Path("."))
        self.assertEqual("PASS", result["status"])
        self.assertEqual("UNVERIFIED", result["hosted_publication"])

    def test_missing_duplicate_or_skipped_test_cannot_qualify(self):
        for name in ("missing", "duplicate", "skipped", "expected_failures", "failures", "errors", "unexpected_successes"):
            with self.subTest(name=name):
                report = copy.deepcopy(self.report)
                if name == "missing":
                    report["passed"].pop()
                elif name == "duplicate":
                    report["passed"][-1] = report["passed"][0]
                else:
                    report[name] = [report["passed"][0]]
                with self.assertRaises(ValueError):
                    qualification.validate_recovery(report)

    def test_missing_duplicate_or_failed_write_boundary_cannot_qualify(self):
        for name in ("missing", "duplicate", "failed"):
            with self.subTest(name=name):
                report = copy.deepcopy(self.report)
                if name == "missing":
                    report["subcases"].pop()
                elif name == "duplicate":
                    report["subcases"][-1] = report["subcases"][0]
                else:
                    report["subcases"][0]["passed"] = False
                with self.assertRaises(ValueError):
                    qualification.validate_recovery(report)

    def test_failed_command_cannot_qualify_passing_output(self):
        output = subprocess.CompletedProcess([], 1, json.dumps(self.report), "failed process")
        with patch.object(qualification.subprocess, "run", return_value=output):
            self.assertEqual("FAIL", qualification.verify_recovery(Path("."))["status"])

    def test_invalid_or_incomplete_output_is_unverified(self):
        for value in ("{", "{}", '{"tests_run": true}', "null"):
            with self.subTest(value=value):
                output = subprocess.CompletedProcess([], 0, value, "")
                with patch.object(qualification.subprocess, "run", return_value=output):
                    self.assertEqual("UNVERIFIED", qualification.verify_recovery(Path("."))["status"])


if __name__ == "__main__":
    unittest.main()
