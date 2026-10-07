"""Check captured-file integrity and fail-closed publication qualification."""

import base64
import copy
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import catalog_publication_capture as capture
from test_catalog_publication_hosted import PromotionEvidenceFixture, RetryEvidenceFixture


class CaptureEvidenceTests(unittest.TestCase):
    def setUp(self):
        PromotionEvidenceFixture.setUp(self)
        RetryEvidenceFixture.setUp(self)
        self.channels = {}
        for version in ("v1", "v2"):
            self.channels["catalog/" + version] = {
                "channel": "catalog/" + version, "sequence": 1, "generation_id": self.record["generation_id"],
                "tag": self.record["artifact_tag"], "catalog_digest": self.record["catalog_checksum"],
                "assets": [{"name": "starmap-catalog.tar.gz", "checksum": self.record["archive_checksum"]}],
            }
        self.channels["catalog/v2"]["publication"] = {
            "source_commit": self.pull["merge_commit_sha"], "receipt_tag": self.record["receipt_tag"],
            "receipt": {"checksum": self.record["receipt_checksum"]},
            "checkpoint": {"checksum": self.record["checkpoint_checksum"]},
        }
        self.documents = {"pending.json": self.record, "pull.json": self.pull,
                          "protection.json": self.protection, "checks.json": self.pages,
                          "completion.json": self.completion, "retry.json": self.retry,
                          "releases-before.json": self.releases, "releases-after.json": copy.deepcopy(self.releases),
                          "channels-before.json": self.channels, "channels-after.json": copy.deepcopy(self.channels)}
        for version in ("v1", "v2"):
            self.documents["channel-" + version + ".raw.json"] = json.dumps(self.channels["catalog/" + version]) + "\n"

    def test_bound_capture_preserves_original_channel_bytes(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            capture.write_capture(directory, "b" * 40, self.documents)
            record, documents = capture.read_capture(directory, capture.BEFORE_FILES + capture.AFTER_FILES)
            self.assertEqual("b" * 40, record["source_commit"])
            self.assertEqual(self.documents, documents)
            capture.validate_metadata(documents)
            with self.assertRaises(ValueError):
                capture.write_capture(directory, "b" * 40, self.documents)
            (directory / "pending.json").write_text("{}")
            with self.assertRaises(ValueError):
                capture.read_capture(directory, capture.BEFORE_FILES)

    def test_partial_channels_changed_retry_and_wrong_merge_fail(self):
        for mutation in (lambda d: d["channels-before.json"].pop("catalog/v1"),
                         lambda d: d["channels-after.json"]["catalog/v1"].update(sequence=2),
                         lambda d: d["channels-after.json"]["catalog/v2"]["publication"].update(source_commit="c" * 40),
                         lambda d: d["pull.json"].update(merged_at="2026-09-17T12:01:00Z")):
            documents = copy.deepcopy(self.documents)
            mutation(documents)
            with self.assertRaises(ValueError):
                capture.validate_metadata(documents)

    def test_missing_or_changed_source_cannot_start_downloads(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            entry = {"proof": "proof"}
            with patch.object(capture, "command") as command:
                self.assertEqual("UNVERIFIED", capture.verify(root, entry)["status"])
                command.assert_not_called()
            directory = root / "proof"
            directory.mkdir()
            capture.write_capture(directory, "b" * 40, self.documents)
            with patch.object(capture, "unchanged_tooling", side_effect=ValueError("tooling changed")), patch.object(capture, "command") as command:
                self.assertEqual("UNVERIFIED", capture.verify(root, entry)["status"])
                command.assert_not_called()

    def test_download_or_attestation_failure_cannot_qualify(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            directory = root / "proof"
            directory.mkdir()
            capture.write_capture(directory, "b" * 40, self.documents)
            with patch.object(capture, "unchanged_tooling"), patch.object(capture, "api") as api, \
                    patch.object(capture, "command", side_effect=subprocess.CalledProcessError(1, ["gh"])):
                self.assertEqual("UNVERIFIED", capture.verify(root, {"proof": "proof"})["status"])
                api.assert_not_called()

    def test_qualification_requires_download_attestation_replay_and_embedding(self):
        # The fake transport does not qualify a hosted publication. It proves
        # that no verification phase can fail while the adapter reports PASS.
        data = {asset["name"]: asset["name"].encode() for release in self.releases for asset in release["assets"]}
        for release in self.releases:
            for asset in release["assets"]:
                asset["digest"] = "sha256:" + hashlib.sha256(data[asset["name"]]).hexdigest()
                asset["size"] = len(data[asset["name"]])
        for name, key in (("starmap-catalog.tar.gz", "archive_checksum"), ("starmap-catalog-run.json", "receipt_checksum"),
                          ("starmap-catalog-state.json", "checkpoint_checksum")):
            self.record[key] = "sha256:" + hashlib.sha256(data[name]).hexdigest()
        self.record["receipt_tag"] = "catalog-run-" + self.record["receipt_checksum"][7:]
        self.releases[1]["tag_name"] = self.record["receipt_tag"]
        self.documents["releases-after.json"] = copy.deepcopy(self.releases)
        for channel in self.channels.values():
            channel["assets"][0]["checksum"] = self.record["archive_checksum"]
        publication = self.channels["catalog/v2"]["publication"]
        publication.update(receipt_tag=self.record["receipt_tag"], receipt={"checksum": self.record["receipt_checksum"]},
                           checkpoint={"checksum": self.record["checkpoint_checksum"]})
        self.documents["channels-after.json"] = copy.deepcopy(self.channels)
        for version in ("v1", "v2"):
            self.documents["channel-" + version + ".raw.json"] = json.dumps(self.channels["catalog/" + version])
        for failure in (None, "attestation", "attestation_identity", "replay", "replay_bytes",
                        "current_record", "current_promotion", "current_channels", "current_assets",
                        "current_attestation", "embedding", "embedding_identity"):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                directory = root / "proof"
                directory.mkdir()
                capture.write_capture(directory, "b" * 40, self.documents)
                phases = []
                # The live publication equals the capture unless one failure changes it.
                live = {"record": copy.deepcopy(self.record), "pull": copy.deepcopy(self.pull),
                        "channels": copy.deepcopy(self.channels), "releases": copy.deepcopy(self.releases)}
                if failure == "current_record":
                    live["record"]["schema_version"] = 2
                if failure == "current_promotion":
                    live["pull"]["user"]["login"] = "human"
                if failure == "current_channels":
                    live["channels"]["catalog/v2"]["tag"] = "catalog-" + "e" * 64
                if failure == "current_assets":
                    live["releases"][0]["assets"][0]["digest"] = "sha256:" + "e" * 64
                merge = self.pull["merge_commit_sha"]

                def api(repository, endpoint, pages=False):
                    phases.append("api")
                    if endpoint == "contents/pending.json?ref=catalog%2Fpublication":
                        return {"content": base64.b64encode(json.dumps(live["record"]).encode()).decode()}
                    if endpoint.startswith("contents/channel.json?ref=catalog%2F"):
                        channel = live["channels"]["catalog/" + endpoint[-2:]]
                        return {"content": base64.b64encode(json.dumps(channel).encode()).decode()}
                    if endpoint == f"commits/{merge}/pulls?per_page=100":
                        return [{"number": 7, "merge_commit_sha": merge}]
                    if endpoint == "pulls/7":
                        return live["pull"]
                    if endpoint == "branches/main/protection":
                        return self.protection
                    if endpoint == "commits/" + self.pull["head"]["sha"] + "/check-runs?filter=all&per_page=100" and pages:
                        return self.pages
                    if endpoint.startswith("releases/tags/"):
                        return next(release for release in live["releases"] if release["tag_name"] == endpoint[len("releases/tags/"):])
                    raise AssertionError("unexpected endpoint " + endpoint)

                def command(repository, args):
                    current = "/current/" in str(args)
                    if args[:3] == ["gh", "release", "download"]:
                        target = Path(args[args.index("--dir") + 1])
                        for name, raw in data.items():
                            (target / name).write_bytes(raw)
                        return ""
                    if args[:3] == ["gh", "attestation", "verify"]:
                        phase = "current_attestation" if current else "attestation"
                    elif args[:3] == ["go", "run", "./cmd/starmap-catalog-publish"]:
                        phase = "replay"
                    else:
                        phase = "embedding"
                    phases.append(phase)
                    if phase == failure:
                        raise subprocess.CalledProcessError(1, args)
                    if phase == "replay":
                        restored = Path(args[args.index("-output-dir") + 1])
                        restored.mkdir()
                        for name in ("starmap-catalog.tar.gz", "starmap-catalog.tar.gz.sha256", "starmap-catalog.intoto.json"):
                            (restored / name).write_bytes(b"changed" if failure == "replay_bytes" else data[name])
                        return json.dumps({"artifact_directory": str(restored)})
                    if phase == "embedding":
                        release = Path(args[args.index("--promotion-release-dir") + 1])
                        self.assertTrue(current)
                        self.assertEqual(3, len(list(release.iterdir())))
                        return json.dumps({"generation_id": self.record["generation_id"], "semantic_checksum": "wrong" if failure == "embedding_identity" else self.record["catalog_checksum"],
                                           "archive_checksum": self.record["archive_checksum"]})
                    path = Path(args[3])
                    return json.dumps([{"verificationResult": {"signature": {"certificate": {
                        "sourceRepositoryDigest": "c" * 40 if failure == "attestation_identity" else "a" * 40,
                        "runInvocationURI": "https://github.com/agentstation/starmap/actions/runs/10/attempts/1"}},
                        "statement": {"subject": [{"digest": {"sha256": hashlib.sha256(path.read_bytes()).hexdigest()}}]}}}])

                with patch.object(capture, "unchanged_tooling"), patch.object(capture, "api", side_effect=api), \
                        patch.object(capture, "command", side_effect=command):
                    result = capture.verify(root, {"proof": "proof"})
                self.assertEqual("UNVERIFIED" if failure else "PASS", result["status"], result)
                if failure is None:
                    self.assertEqual(["attestation"] * 5 + ["replay"] + ["api"] * 9 + ["current_attestation"] * 3 + ["embedding"], phases)
                    self.assertEqual(7, result["current"]["pull"])
                    self.assertEqual(3, result["current"]["attested_subjects"])
                    self.assertEqual(self.pull["merge_commit_sha"], result["current"]["promotion"]["merge"])

    def test_tooling_binding_ignores_catalog_and_other_source_changes(self):
        with tempfile.TemporaryDirectory() as temporary:
            repository = Path(temporary)
            git = ["git", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false"]
            capture.command(repository, ["git", "init", "-q"])
            tooling = repository / "scripts/catalog_publication.py"
            catalog = repository / "internal/embedded/catalog/providers.yaml"
            runtime = repository / "runtime/layers.go"
            for path in (tooling, catalog, runtime):
                path.parent.mkdir(parents=True)
                path.write_text("original\n")
            capture.command(repository, ["git", "add", "."])
            capture.command(repository, git + ["commit", "-qm", "fixture"])
            revision = capture.command(repository, ["git", "rev-parse", "HEAD"]).strip()
            with self.assertRaises(ValueError):
                capture.unchanged_tooling(repository, revision[:12])
            capture.unchanged_tooling(repository, revision)
            catalog.write_text("promoted\n")
            runtime.write_text("changed\n")
            (repository / "docs/plans").mkdir(parents=True)
            (repository / "docs/plans/proof.json").write_text("{}")
            capture.unchanged_tooling(repository, revision)
            tooling.write_text("changed\n")
            with self.assertRaises(ValueError):
                capture.unchanged_tooling(repository, revision)
            tooling.write_text("original\n")
            capture.unchanged_tooling(repository, revision)
            added = repository / "cmd/starmap-catalog-publish/new.go"
            added.parent.mkdir(parents=True)
            added.write_text("package main\n")
            with self.assertRaises(ValueError):
                capture.unchanged_tooling(repository, revision)


if __name__ == "__main__":
    unittest.main()
