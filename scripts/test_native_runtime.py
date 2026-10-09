#!/usr/bin/env python3
"""Check native partition coverage, failure evidence, and aggregate compatibility."""

import contextlib
import copy
import io
import json
from pathlib import Path
import tempfile
import unittest

import native_runtime as native


class TestNativeRuntime(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.packages = [native.MODULE + suffix for suffix in (
            "", "/runtime", "/internal/cli/app", "/internal/privatefiles", "/internal/runtimeacl/windows",
        )]
        self.groups = []
        for group in native.GROUPS:
            packages = native.selected_packages(self.packages, group)
            inventory = {(package, "TestContract" + str(index)) for package in packages for index in range(60)
                         if not package.endswith("/internal/runtimeacl/windows")}
            tests = native.selected_tests(inventory, group)
            path = self.directory / group
            path.mkdir()
            events = [{"Action": "run", "Package": package, "Test": test} for package, test in sorted(tests)]
            events += [{"Action": "pass", "Package": package, "Test": test} for package, test in sorted(tests)]
            events += [{"Action": "pass", "Package": package} for package in packages]
            (path / "tests.jsonl").write_text("".join(json.dumps(event) + "\n" for event in events))
            manifest = {"version": 1, "group": group, "system": "darwin", "arch": "arm64", "status": "success",
                        "packages": self.packages, "inventory": sorted(inventory), "selected": sorted(tests)}
            toolchain = ["go version go1.27.2 darwin/arm64", "darwin", "arm64", "darwin", "arm64", "0"]
            (path / "inventory.json").write_text(json.dumps(manifest))
            (path / "toolchain.txt").write_text("\n".join(toolchain) + "\n")
            (path / "job-status.txt").write_text("success\n")
            self.groups.append({"manifest": manifest, "toolchain": toolchain, "events": path / "tests.jsonl", "directory": path,
                                "job_status": "success"})

    def validate(self, groups=None):
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            return native.validate_groups(self.groups if groups is None else groups, "darwin", "arm64")

    def test_complete_test_and_package_union_is_disjoint(self):
        result = self.validate()
        self.assertEqual(result["top_level_tests"], 240)
        self.assertEqual(result["packages"], 5)
        self.assertEqual(result["status"], "success")
        self.assertEqual(self.validate(list(reversed(self.groups))), result)
        with self.assertRaises(ValueError):
            native.selected_packages([native.MODULE] * 2, "client")
        with self.assertRaises(ValueError):
            native.group_for("other/module")

    def test_missing_duplicate_or_unknown_groups_refuse(self):
        broken = [self.groups[:-1], self.groups + [self.groups[0]], self.groups[:-1] + [self.groups[0]]]
        unknown = copy.deepcopy(self.groups)
        unknown[0]["manifest"]["group"] = "future"
        broken.append(unknown)
        for groups in broken:
            with self.subTest(groups=len(groups)), self.assertRaises(ValueError):
                self.validate(groups)

    def test_omitted_duplicate_extra_and_cross_owner_tests_refuse(self):
        for mutation in ("omitted", "duplicate", "extra", "owner", "runtime-inventory"):
            groups = copy.deepcopy(self.groups)
            manifest = groups[3 if mutation == "runtime-inventory" else 0]["manifest"]
            if mutation == "omitted":
                manifest["selected"] = manifest["selected"][1:]
            elif mutation == "duplicate":
                manifest["inventory"].append(manifest["inventory"][0])
            elif mutation == "extra":
                manifest["selected"].append([native.MODULE, "TestUnexpected"])
            elif mutation == "owner":
                manifest["inventory"].append([native.MODULE + "/runtime", "TestForeign"])
            else:
                manifest["inventory"].append([native.MODULE + "/runtime", "TestUnexpected"])
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                self.validate(groups)

    def test_truncated_failed_and_duplicate_results_refuse(self):
        path = self.groups[0]["events"]
        original = path.read_text()
        rows = [json.loads(line) for line in original.splitlines()]
        broken = [rows[:-1], rows[1:], rows + [rows[0]], rows + [rows[60]], rows + [{"Action": "fail", "Package": native.MODULE}]]
        for events in broken:
            path.write_text("".join(json.dumps(event) + "\n" for event in events))
            with self.subTest(rows=len(events)), self.assertRaises(ValueError):
                self.validate()
        path.write_text(original)

    def test_every_unsuccessful_job_and_manifest_state_refuses(self):
        for status in ("failure", "cancelled", "skipped", "incomplete", ""):
            for field in ("job_status", "manifest"):
                groups = copy.deepcopy(self.groups)
                if field == "manifest":
                    groups[0][field]["status"] = status
                else:
                    groups[0][field] = status
                with self.subTest(status=status, field=field), self.assertRaises(ValueError):
                    self.validate(groups)

    def test_wrong_target_host_toolchain_and_package_inventory_refuse(self):
        for index, value in ((0, "go version go1.26.2 darwin/arm64"), (1, "windows"), (2, "amd64"),
                             (3, "linux"), (4, "amd64"), (5, "1")):
            groups = copy.deepcopy(self.groups)
            groups[0]["toolchain"][index] = value
            with self.subTest(index=index), self.assertRaises(ValueError):
                self.validate(groups)
        for field, value in (("system", "linux"), ("arch", "amd64"), ("packages", self.packages[:-1])):
            groups = copy.deepcopy(self.groups)
            groups[0]["manifest"][field] = value
            with self.subTest(field=field), self.assertRaises(ValueError):
                self.validate(groups)
        with self.assertRaises(ValueError):
            native.validate_toolchain(["go version go1.27.2 darwin/amd64", "darwin", "amd64", "darwin", "amd64", "0"], "darwin", "amd64")

    def test_complete_native_commands_keep_original_limits_and_behavior(self):
        for group in native.GROUPS:
            chosen = native.selected_packages(self.packages, group)
            args = native.test_command(group, chosen, set(map(tuple, next(g["manifest"]["selected"] for g in self.groups if g["manifest"]["group"] == group))))
            self.assertIn("-count=1", args)
            self.assertIn("-timeout=30m", args)
            self.assertIn("-p=1", args)
            self.assertFalse(any(value.startswith(("-skip", "-short")) for value in args))
            self.assertEqual(any(value.startswith("-run=") for value in args), group.startswith("runtime-"))

    def test_combined_artifact_keeps_existing_named_test_contract(self):
        output = self.directory / "combined"
        source = self.directory / "input"
        source.mkdir()
        for group in self.groups:
            group["directory"].rename(source / group["manifest"]["group"])
        with contextlib.redirect_stdout(io.StringIO()):
            native.combine(source, output, "darwin", "arm64")
        rows = [json.loads(line) for line in (output / "tests.jsonl").read_text().splitlines()]
        terminal = [(r["Package"], r["Test"]) for r in rows if r["Action"] == "pass" and r.get("Test")]
        self.assertEqual(len(terminal), 240)
        self.assertEqual(len(set(terminal)), len(terminal))
        self.assertEqual((output / "toolchain.txt").read_text(), (source / "client" / "toolchain.txt").read_text())
        self.assertEqual(json.loads((output / "qualification.json").read_text())["status"], "success")
        self.assertEqual(len(list(output.glob("*-inventory.json"))), 6)


if __name__ == "__main__":
    unittest.main()
