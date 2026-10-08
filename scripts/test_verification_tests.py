#!/usr/bin/env python3
"""Check that verification preserves package coverage and rejects missing evidence."""

import contextlib
import io
import json
import re
from pathlib import Path
import tempfile
import unittest

import verification_tests as verification


class TestVerification(unittest.TestCase):
    def test_groups_cover_each_package_once_including_new_packages(self):
        packages = [verification.MODULE + suffix for suffix in (
            "", "/internal/ciworkflow", "/internal/ciworkflow/future", "/runtime", "/runtime/new-child", "/acquisition", "/internal/bootstrap",
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
        self.assertEqual(["-skip=" + verification.capacity_pattern()], [arg for arg in args if arg.startswith("-skip")])
        for package, names in verification.CAPACITY_TESTS.items():
            for name in names:
                self.assertTrue(re.fullmatch(verification.capacity_pattern(), name), (package, name))
        self.assertIsNone(re.fullmatch(verification.capacity_pattern(), "TestPublicPublicationProfileRetainsBoundedStateX"))

    def test_regular_and_capacity_preserve_scale_coverage(self):
        regular = verification.test_command("regular", [verification.MODULE])
        self.assertFalse(any(arg.startswith(("-skip", "-run", "-short")) for arg in regular))
        capacity = verification.test_command("capacity", [verification.MODULE])
        self.assertEqual(sorted(verification.CAPACITY_TESTS), capacity[-len(verification.CAPACITY_TESTS):])
        self.assertIn("-run=" + verification.capacity_pattern(), capacity)
        with self.assertRaises(ValueError):
            verification.test_command("unrecognized", [])

    def summarize(self, events, suite="capacity", packages=None, expected_tests=None):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "events.jsonl"
            path.write_text("".join(json.dumps(event) + "\n" for event in events), encoding="utf-8")
            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                verification.summarize(path, suite, packages or sorted(verification.CAPACITY_TESTS), expected_tests)

    def test_shards_cover_tests_examples_and_fuzz_seeds_exactly_once(self):
        packages = [verification.MODULE + suffix for suffix in (
            "/runtime", "/runtime/future", "/cmd/example", "/internal/cli/app", "/internal/server",
        )]
        names = [prefix + str(i) for prefix in ("TestCase", "ExampleCase", "FuzzCase") for i in range(20)]
        names += ["Test日本語", "ExampleÉ", "FuzzΩ"]
        inventory = {(package, name) for package in packages for name in names}
        events = [{"Package": package, "Action": "output", "Output": name + "\n"}
                  for package, name in sorted(inventory)]
        events += [{"Package": package, "Action": "pass"} for package in packages]
        shards = [verification.select_tests(events, packages, shard) for shard in (1, 2, 3)]
        self.assertEqual(inventory, set.union(*shards))
        self.assertEqual(len(inventory), sum(map(len, shards)))
        for selected in shards:
            self.assertEqual(selected, verification.select_tests(list(reversed(events)), packages, shards.index(selected) + 1))
            pattern = verification.shard_filter(selected).removeprefix("-run=")
            actual = {key for key in inventory if re.fullmatch(pattern, key[1])}
            self.assertEqual(selected, actual)
        for broken in (events[:-1], events + [events[0]], events + [{"Action": "fail"}], []):
            with self.subTest(broken=broken[-1:]), self.assertRaises(ValueError):
                verification.select_tests(broken, packages, 1)

    def test_race_shards_exclude_only_the_capacity_tests(self):
        package, name = next((package, names[0]) for package, names in sorted(verification.CAPACITY_TESTS.items()))
        tests = {(package, name), (package, "TestOther"), (verification.MODULE + "/other", name)}
        self.assertEqual({(package, "TestOther"), (verification.MODULE + "/other", name)}, verification.exclude_capacity(tests))

    def test_shard_evidence_rejects_omitted_extra_and_duplicate_tests(self):
        package = verification.MODULE + "/runtime"
        expected = {(package, "TestA"), (package, "ExampleB")}
        events = [{"Package": package, "Test": "TestA", "Action": "pass"},
                  {"Package": package, "Test": "ExampleB", "Action": "skip"},
                  {"Package": package, "Action": "pass"}]
        self.summarize(events, "race", [package], expected)
        for broken in (events[1:], events + [events[0]],
                       events + [dict(events[0], Test="FuzzUnexpected")]):
            with self.subTest(broken=broken), self.assertRaises(ValueError):
                self.summarize(broken, "race", [package], expected)

    def test_capacity_needs_every_exact_passing_test(self):
        successes = [{"Package": package, "Test": name, "Action": "pass"}
                     for package, names in sorted(verification.CAPACITY_TESTS.items()) for name in names]
        completions = [{"Package": package, "Action": "pass"} for package in sorted(verification.CAPACITY_TESTS)]
        self.summarize(successes + completions)
        first = successes[0]
        for events in ([], successes[1:] + completions, [dict(first, Action="skip")] + successes[1:] + completions,
                       [dict(first, Test="TestOther")] + successes[1:] + completions,
                       [dict(first, Package="other")] + successes[1:] + completions):
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
