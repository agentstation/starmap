#!/usr/bin/env python3
"""Check that verification preserves package coverage and rejects missing evidence."""

import contextlib
import io
import json
from pathlib import Path
import tempfile
import unittest

import verification_tests as verification


class TestVerification(unittest.TestCase):
    def test_groups_cover_each_package_once_including_new_packages(self):
        packages = [verification.MODULE + suffix for suffix in (
            "", "/runtime", "/runtime/new-child", "/acquisition", "/internal/bootstrap",
            "/internal/bootstrap/budget", "/cmd/example", "/internal/cli/app",
            "/internal/server", "/internal/server/new-child", "/pkg/new-contract", "/new-package",
        )]
        groups = [verification.select_packages(packages, group) for group in verification.GROUPS]
        actual = [package for group in groups for package in group]
        self.assertCountEqual(packages, actual)
        self.assertEqual(len(packages), len(set(actual)))
        self.assertEqual(packages, verification.select_packages(packages, "all"))

    def test_invalid_inventory_and_empty_groups_refuse(self):
        for packages, group in (([], "all"), ([verification.MODULE] * 2, "all"),
                                (["example.invalid/module"], "all"),
                                ([verification.MODULE], "unknown"), ([verification.MODULE], "runtime")):
            with self.subTest(packages=packages, group=group), self.assertRaises(ValueError):
                verification.select_packages(packages, group)

    def test_race_runs_fresh_without_blanket_short_mode(self):
        args = verification.test_command("race", [verification.MODULE])
        self.assertIn("-race", args)
        self.assertIn("-count=1", args)
        self.assertIn("-p=1", args)
        self.assertNotIn("-short", args)
        self.assertEqual(["-skip=^" + verification.CAPACITY_TEST + "$"], [arg for arg in args if arg.startswith("-skip")])

    def test_regular_and_capacity_preserve_scale_coverage(self):
        regular = verification.test_command("regular", [verification.MODULE])
        self.assertFalse(any(arg.startswith(("-skip", "-run", "-short")) for arg in regular))
        capacity = verification.test_command("capacity", [verification.MODULE])
        self.assertEqual(verification.CAPACITY_PACKAGE, capacity[-1])
        self.assertIn("-run=^" + verification.CAPACITY_TEST + "$", capacity)
        with self.assertRaises(ValueError):
            verification.test_command("unrecognized", [])

    def summarize(self, events, suite="capacity", packages=None):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "events.jsonl"
            path.write_text("".join(json.dumps(event) + "\n" for event in events), encoding="utf-8")
            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                verification.summarize(path, suite, packages or [verification.CAPACITY_PACKAGE])

    def test_capacity_needs_the_exact_passing_test(self):
        success = {"Package": verification.CAPACITY_PACKAGE, "Test": verification.CAPACITY_TEST, "Action": "pass"}
        completion = {"Package": verification.CAPACITY_PACKAGE, "Action": "pass"}
        self.summarize([success, completion])
        for events in ([], [dict(success, Action="skip")], [dict(success, Test="TestOther")], [dict(success, Package="other")]):
            with self.subTest(events=events), self.assertRaises(ValueError):
                self.summarize(events)

    def test_missing_or_duplicate_package_results_refuse(self):
        test = {"Package": verification.MODULE, "Test": "TestContract", "Action": "pass"}
        completion = {"Package": verification.MODULE, "Action": "pass"}
        self.summarize([test, completion], "race", [verification.MODULE])
        for events in ([], [test], [completion], [test, completion, completion],
                       [test, dict(completion, Package="other")]):
            with self.subTest(events=events), self.assertRaises(ValueError):
                self.summarize(events, "race", [verification.MODULE])

    def test_failure_records_do_not_become_success(self):
        for event in ({"Action": "fail", "Package": verification.MODULE},
                      {"Action": "fail", "Package": verification.MODULE, "Test": "TestBroken"}):
            with self.subTest(event=event), self.assertRaises(ValueError):
                self.summarize([event], "regular")


if __name__ == "__main__":
    unittest.main()
