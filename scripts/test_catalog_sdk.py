"""Test the SDK verifier's evidence boundary."""

import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import catalog_sdk


class CatalogSDKTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        (self.root / "scripts").mkdir()
        (self.root / "scripts/smoke-openrouter-sdks.sh").write_text("exit 0\n")
        self.output = "\n".join(
            f"    catalog_model_removal_test.go:34: PASS {client} SDK catalog transition: {state}"
            for client in catalog_sdk.CLIENTS for state in catalog_sdk.STATES
        ) + "\n--- PASS: " + catalog_sdk.TEST + " (5.0s)\n"

    def invoke(self, output, code=0):
        result = subprocess.CompletedProcess([], code, output, "")
        with patch.object(catalog_sdk.subprocess, "run", return_value=result) as run:
            evidence = catalog_sdk.verify(self.root)
        self.assertEqual(run.call_args.args[0], ["bash", str(self.root / "scripts/smoke-openrouter-sdks.sh")])
        self.assertEqual(run.call_args.kwargs["cwd"], self.root)
        return evidence

    def test_complete_live_roster_passes(self):
        self.assertEqual(self.invoke(self.output)["status"], "PASS")

    def test_exit_failure_overrides_pass_markers(self):
        self.assertEqual(self.invoke(self.output, 1)["status"], "FAIL")

    def test_skip_never_passes(self):
        output = self.output + "--- SKIP: " + catalog_sdk.TEST + " (0.0s)\n"
        self.assertEqual(self.invoke(output)["status"], "UNVERIFIED")

    def test_missing_state_or_client_never_passes(self):
        for line in self.output.splitlines():
            with self.subTest(line=line):
                self.assertEqual(self.invoke(self.output.replace(line, ""))["status"], "UNVERIFIED")

    def test_empty_success_does_not_qualify(self):
        self.assertEqual(self.invoke("")["status"], "UNVERIFIED")

    def test_missing_script_is_unverified(self):
        (self.root / "scripts/smoke-openrouter-sdks.sh").unlink()
        self.assertEqual(catalog_sdk.verify(self.root)["status"], "UNVERIFIED")

    def test_timeout_is_unverified(self):
        with patch.object(catalog_sdk.subprocess, "run", side_effect=subprocess.TimeoutExpired("bash", 600)):
            self.assertEqual(catalog_sdk.verify(self.root)["status"], "UNVERIFIED")
