#!/usr/bin/env python3
"""Test publisher recovery with real catalog tools and isolated Git repositories."""

import base64
import copy
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import catalog_publication as publication


SOURCE = Path(__file__).resolve().parents[1]


class ReleaseFixture(publication.Publisher):
    """Simulate GitHub transport outcomes without claiming remote qualification."""

    def __init__(self, root, fixture, *, cloud=None):
        super().__init__(root, "agentstation/starmap", 42, source=SOURCE)
        self.fixture = fixture
        self.publish_tool = fixture["publish_tool"]
        self.release_tool = fixture["release_tool"]
        self.cloud = cloud if cloud is not None else {}
        self.events = []
        self.fail_edit = False
        self.refuse_attestation = False

    def attest(self, path):
        self.events.append(("verify", path.name))
        if self.refuse_attestation or publication.checksum(path) not in self.fixture["accepted_digests"]:
            raise publication.PublicationError("fixture authenticator refused publication input")

    def api(self, endpoint, *, missing=False, method="GET"):
        if method == "DELETE" and endpoint.startswith("releases/assets/"):
            identity = int(endpoint.rsplit("/", 1)[1])
            for release in self.cloud.values():
                release["assets"] = [asset for asset in release["assets"] if asset["id"] != identity]
            self.events.append(("delete-starter", identity))
            return None
        if not endpoint.startswith("releases/tags/"):
            raise AssertionError(endpoint)
        tag = endpoint.removeprefix("releases/tags/")
        if tag not in self.cloud:
            if missing:
                return None
            raise publication.PublicationError("fixture release is missing")
        return copy.deepcopy(self.cloud[tag])

    def gh(self, *args, **options):
        self.events.append(tuple(str(value) for value in args[:3]))
        if args[:2] == ("run", "download"):
            target = Path(args[args.index("--dir") + 1])
            shutil.copytree(self.fixture["stage"], target, dirs_exist_ok=True)
        elif args[:2] == ("release", "create"):
            self.cloud[args[2]] = {"draft": True, "assets": [], "data": {}, "published_at": None}
        elif args[:2] == ("release", "upload"):
            tag, path = args[2], Path(args[3])
            release = self.cloud[tag]
            if not release["draft"] or path.name in release["data"]:
                raise AssertionError("controller attempted to replace a retained asset")
            release["data"][path.name] = path.read_bytes()
            release["assets"].append({"id": len(release["assets"]) + 1, "name": path.name, "state": "uploaded"})
        elif args[:2] == ("release", "download"):
            tag = args[2]
            filename = args[args.index("--pattern") + 1]
            target = Path(args[args.index("--dir") + 1])
            target.mkdir(parents=True, exist_ok=True)
            (target / filename).write_bytes(self.cloud[tag]["data"][filename])
        elif args[:2] == ("release", "edit"):
            self.cloud[args[2]]["draft"] = False
            self.cloud[args[2]]["published_at"] = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
            if self.fail_edit:
                self.fail_edit = False
                raise publication.PublicationError("fixture lost the successful publication response")
        else:
            raise AssertionError(args)
        return subprocess.CompletedProcess(args, 0, "", "")


class PromotionFixture(ReleaseFixture):
    """Use real Git promotion and simulate the GitHub PR and check responses."""

    def __init__(self, root, fixture, checkout, platform):
        super().__init__(root, fixture, cloud=platform["releases"])
        self.source = checkout
        self.platform = platform
        self.emitted = {}

    def outputs(self, **values):
        self.emitted.update(values)

    def api(self, endpoint, **options):
        if endpoint == "branches/main/protection":
            return {"required_status_checks": {"strict": True, "checks": [
                {"context": name, "app_id": 15368} for name in publication.REQUIRED_CHECKS[:2]]}}
        if endpoint.startswith("pulls/"):
            return {"user": {"login": os.environ["CATALOG_BOT_NAME"]}}
        return super().api(endpoint, **options)

    def gh(self, *args, **options):
        if args[:2] == ("pr", "list"):
            return subprocess.CompletedProcess(args, 0, json.dumps(self.platform["pulls"]), "")
        if args[:2] == ("pr", "create"):
            if self.platform.pop("fail_create", False):
                raise publication.PublicationError("fixture interrupted before PR creation")
            branch = args[args.index("--head") + 1]
            head = self.git("ls-remote", "origin", f"refs/heads/{branch}").stdout.split()[0]
            self.platform["pulls"] = [{"number": 1, "state": "OPEN", "headRefOid": head,
                "mergeCommit": None, "reviewDecision": "", "url": "https://example.invalid/pull/1"}]
            self.platform["creates"] += 1
            return subprocess.CompletedProcess(args, 0, self.platform["pulls"][0]["url"] + "\n", "")
        if args[:2] == ("pr", "merge"):
            pull = self.platform["pulls"][0]
            if args[args.index("--match-head-commit") + 1] != pull["headRefOid"]:
                raise AssertionError("merge omitted the verified head constraint")
            self.git("fetch", "origin", "main")
            main = self.git("rev-parse", "FETCH_HEAD").stdout.strip()
            checkout = self.promotion_checkout(main, "server-merge")
            publication.command(["git", "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid",
                "merge", "--no-ff", "--no-edit", pull["headRefOid"]], cwd=checkout)
            merged = publication.command(["git", "rev-parse", "HEAD"], cwd=checkout).stdout.strip()
            self.git("push", "origin", f"{merged}:refs/heads/main")
            pull.update(state="MERGED", mergeCommit={"oid": merged})
            self.platform["merges"] += 1
            return subprocess.CompletedProcess(args, 0, "", "")
        if args[:2] == ("pr", "view"):
            return subprocess.CompletedProcess(args, 0, json.dumps(self.platform["pulls"][0]), "")
        return super().gh(*args, **options)


class PublicationRecoveryTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temporary = tempfile.TemporaryDirectory(prefix="starmap-publication-tests-")
        cls.addClassCleanup(cls.temporary.cleanup)
        root = Path(cls.temporary.name)
        publish_tool, release_tool = root / "publish", root / "release"
        if os.name == "nt":
            publish_tool, release_tool = root / "publish.exe", root / "release.exe"
        for package, target in (("starmap-catalog-publish", publish_tool), ("starmap-catalog-release", release_tool)):
            publication.command(["go", "build", "-o", target, f"./cmd/{package}"], cwd=SOURCE)
        profile = root / "offline.yaml"
        profile.write_text("""policy_version: workflow-fixture-v1
scopes:
  - source: embedded_catalog
    required: false
    enabled: false
    allow_missing: true
    max_retained_age: 0s
    disabled_action: preserve
""", encoding="utf-8")
        checkpoint = json.loads(publication.command(["go", "run", "./cmd/starmap-catalog-publish/testdata/workflow-state"], cwd=SOURCE).stdout)
        baseline = root / "baseline.json"
        baseline.write_bytes(base64.b64decode(checkpoint["Data"], validate=True))
        report = json.loads(publication.command([publish_tool, "-profile", profile,
            "-state", baseline, "-state-checksum", checkpoint["Checksum"],
            "-publisher-id", "starmap-public", "-run-id", "github-42", "-output-dir", root / "prepared"], cwd=SOURCE).stdout)
        stage = root / "fixture"
        stage.mkdir()
        for name in publication.ASSETS:
            shutil.copyfile(Path(report["artifact_directory"]) / name, stage / name)
        shutil.copyfile(report["receipt_path"], stage / publication.RECEIPT)
        shutil.copyfile(report["state_path"], stage / publication.CHECKPOINT)
        verified = json.loads(publication.command([release_tool, "--verify-dir", report["artifact_directory"]]).stdout)
        record = {
            "schema_version": 1, "workflow_run_id": 42, "run_id": "github-42", "preparation_commit": "a" * 40,
            "profile_checksum": publication.checksum(profile), "receipt_checksum": report["receipt_checksum"],
            "checkpoint_checksum": report["state_checksum"], "archive_checksum": report["archive_checksum"],
            "catalog_checksum": verified["semantic_checksum"], "generation_id": report["generation_id"],
            "artifact_tag": "catalog-" + publication.digest_hex(verified["semantic_checksum"]),
            "receipt_tag": "catalog-run-" + publication.digest_hex(report["receipt_checksum"]),
        }
        publication.write_json(stage / "pending.json", record)
        cls.fixture = {"stage": stage, "record": record, "publish_tool": publish_tool, "release_tool": release_tool,
                       "accepted_digests": {publication.checksum(path) for path in stage.iterdir()}}

    def setUp(self):
        self.working = tempfile.TemporaryDirectory(prefix="starmap-publication-case-")
        self.addCleanup(self.working.cleanup)
        self.root = Path(self.working.name)

    def client(self, name="run", cloud=None):
        return ReleaseFixture(self.root / name, self.fixture, cloud=cloud)

    def copy_stage(self, client):
        shutil.copytree(self.fixture["stage"], client.stage)

    def test_recover_from_retained_workflow_without_new_acquisition(self):
        client = self.client()
        client.recover(self.fixture["record"])
        self.assertEqual(publication.checksum(client.stage / publication.CHECKPOINT), self.fixture["record"]["checkpoint_checksum"])
        self.assertIn(("run", "download", "42"), client.events)
        self.assertTrue((client.root / "restored").is_dir())

    def test_public_checkpoint_survives_loss_of_original_working_directory(self):
        first = self.client("original")
        self.copy_stage(first)
        record = self.fixture["record"]
        first.publish_release(record["receipt_tag"], (publication.RECEIPT, publication.CHECKPOINT), record)
        shutil.rmtree(first.root)
        restored = self.client("recovered", first.cloud)
        restored.recover(record)
        self.assertFalse(any(event[:2] == ("run", "download") for event in restored.events))
        self.assertEqual(publication.checksum(restored.stage / publication.ARCHIVE), record["archive_checksum"])

    def test_ambiguous_publish_response_reuses_uploaded_assets(self):
        client = self.client()
        self.copy_stage(client)
        record = self.fixture["record"]
        client.fail_edit = True
        with self.assertRaises(publication.PublicationError):
            client.publish_release(record["receipt_tag"], (publication.RECEIPT, publication.CHECKPOINT), record)
        original = copy.deepcopy(client.cloud)
        before = sum(event[:2] == ("release", "upload") for event in client.events)
        client.publish_release(record["receipt_tag"], (publication.RECEIPT, publication.CHECKPOINT), record)
        self.assertEqual(original, client.cloud)
        self.assertEqual(before, sum(event[:2] == ("release", "upload") for event in client.events))

    def test_partial_draft_asset_recovers_without_replacing_uploaded_data(self):
        client = self.client()
        self.copy_stage(client)
        record = self.fixture["record"]
        client.cloud[record["receipt_tag"]] = {"draft": True, "data": {}, "published_at": None,
            "assets": [{"id": 27, "name": publication.RECEIPT, "state": "starter"}]}
        client.publish_release(record["receipt_tag"], (publication.RECEIPT, publication.CHECKPOINT), record)
        self.assertIn(("delete-starter", 27), client.events)
        self.assertFalse(client.cloud[record["receipt_tag"]]["draft"])

    def test_changed_public_asset_stops_without_overwrite(self):
        client = self.client()
        self.copy_stage(client)
        record = self.fixture["record"]
        client.publish_release(record["receipt_tag"], (publication.RECEIPT, publication.CHECKPOINT), record)
        client.cloud[record["receipt_tag"]]["data"][publication.RECEIPT] = b"invalid replacement"
        retained = copy.deepcopy(client.cloud)
        with self.assertRaises(publication.PublicationError):
            client.publish_release(record["receipt_tag"], (publication.RECEIPT, publication.CHECKPOINT), record)
        self.assertEqual(retained, client.cloud)

    def test_authenticator_refusal_stops_before_staging(self):
        client = self.client()
        client.refuse_attestation = True
        with self.assertRaises(publication.PublicationError):
            client.recover(self.fixture["record"])
        self.assertFalse((client.root / "restored").exists())

    def test_pending_completion_requires_both_channels_and_exact_checkpoint(self):
        record = self.fixture["record"]
        channels = {"catalog/v2": {"document": {"channel": "catalog/v2", "tag": record["artifact_tag"],
            "publication": {"receipt_tag": record["receipt_tag"], "receipt": {"checksum": record["receipt_checksum"]},
                            "checkpoint": {"checksum": record["checkpoint_checksum"]}}}}}
        self.assertFalse(publication.completed(record, channels))
        channels["catalog/v1"] = {"document": {"channel": "catalog/v1", "tag": record["artifact_tag"]}}
        self.assertTrue(publication.completed(record, channels))
        channels["catalog/v2"]["document"]["publication"]["checkpoint"]["checksum"] = "sha256:" + "0" * 64
        self.assertFalse(publication.completed(record, channels))

    def test_interrupted_promotion_merges_exact_artifact_before_both_channels(self):
        remote, checkout = self.root / "origin.git", self.root / "source"
        publication.command(["git", "init", "--bare", remote])
        publication.command(["git", "clone", remote, checkout])
        publication.command(["git", "checkout", "-b", "main"], cwd=checkout)
        embedded = checkout / "internal/embedded/catalog"
        embedded.mkdir(parents=True)
        (embedded / "old.txt").write_text("previous baseline\n", encoding="utf-8")
        (checkout / ".github").mkdir()
        (checkout / publication.PROFILE).write_text("fixture public profile\n", encoding="utf-8")
        publication.command(["git", "add", "."], cwd=checkout)
        publication.command(["git", "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid",
            "commit", "-m", "initial baseline"], cwd=checkout)
        publication.command(["git", "push", "origin", "main"], cwd=checkout)
        record = dict(self.fixture["record"], preparation_commit=publication.command(
            ["git", "rev-parse", "HEAD"], cwd=checkout).stdout.strip())
        platform = {"releases": {}, "pulls": [], "creates": 0, "merges": 0, "fail_create": True}
        channel_state = {name: {"commit": "", "document": None, "path": ""}
                         for name in ("catalog/v1", "catalog/v2")}
        clients = []

        def resume(name):
            client = PromotionFixture(self.root / name, self.fixture, checkout, platform)
            self.copy_stage(client)
            publication.write_json(client.stage / "pending.json", record)
            client.fixture = dict(self.fixture, accepted_digests=set(self.fixture["accepted_digests"]))
            client.fixture["accepted_digests"].add(publication.checksum(client.stage / "pending.json"))
            publication.write_json(client.control, {"pending": record, "pending_head": "", "channels": channel_state})
            clients.append(client)
            return client

        environment = {"CATALOG_BOT_NAME": "catalog-fixture[bot]", "CATALOG_BOT_EMAIL": "fixture@example.invalid"}
        with patch.dict(os.environ, environment):
            refused = resume("validation-refusal")
            actual_command = publication.command
            validation_calls = []

            def validate_candidate(args, **options):
                if args[0] == "make":
                    validation_calls.append(args[1])
                    self.assertTrue(refused.verify_embedding(Path(options["cwd"])))
                    self.assertEqual({}, platform["releases"])
                    return subprocess.CompletedProcess(args, 1, "", "fixture rejected candidate")
                return actual_command(args, **options)

            with patch.object(publication, "command", side_effect=validate_candidate):
                with self.assertRaisesRegex(publication.PublicationError, "catalog-generation-check failed"):
                    refused.validate()
            self.assertEqual(["catalog-generation-check"], validation_calls)
            self.assertEqual({}, platform["releases"])
            self.assertEqual("", refused.git("ls-remote", "origin", "refs/heads/catalog/*").stdout)
            first = resume("interrupted")

            def accept_candidate(args, **options):
                if args[0] == "make":
                    validation_calls.append(args[1])
                    self.assertTrue(first.verify_embedding(Path(options["cwd"])))
                    return subprocess.CompletedProcess(args, 0, "fixture accepted candidate", "")
                return actual_command(args, **options)

            with patch.object(publication, "command", side_effect=accept_candidate):
                first.validate()
            self.assertEqual(["catalog-generation-check", "catalog-generation-check", "embedded-catalog-budget-check"], validation_calls)
            first.begin()
            first.publish()
            with self.assertRaisesRegex(publication.PublicationError, "before PR creation"):
                first.promote()
            branch = "catalog/promotion/" + publication.digest_hex(record["receipt_checksum"])
            pushed = first.git("ls-remote", "origin", f"refs/heads/{branch}").stdout.split()[0]
            self.assertEqual(0, platform["creates"])
            self.assertEqual(2, len(platform["releases"]))
            self.assertEqual("", first.git("ls-remote", "origin", "refs/heads/catalog/v*").stdout)

            second = resume("pr-recovery")
            second.promote()
            self.assertEqual(pushed, platform["pulls"][0]["headRefOid"])
            self.assertEqual("awaiting_checks", second.emitted["status"])
            self.assertEqual(1, platform["creates"])
            self.assertEqual(0, platform["merges"])

            checked = resume("checked-merge")
            actual_command = publication.command
            checks = [{"id": index + 1, "name": name, "head_sha": pushed, "app": {"id": 15368},
                       "status": "completed", "conclusion": "success"}
                      for index, name in enumerate(publication.REQUIRED_CHECKS)]

            def dispatch(args, **options):
                if args[:2] == ["gh", "api"]:
                    return subprocess.CompletedProcess(args, 0, json.dumps([{"check_runs": checks}]), "")
                return actual_command(args, **options)

            with patch.object(publication, "command", side_effect=dispatch):
                checks[-1]["app"]["id"] = 1
                self.assertFalse(checked.checks_pass(pushed))
                checked.promote()
                self.assertEqual("awaiting_checks", checked.emitted["status"])
                self.assertEqual(0, platform["merges"])
                checks[-1]["app"]["id"] = 15368
                checks.append(dict(checks[-1], id=99, status="in_progress", conclusion=None))
                self.assertFalse(checked.checks_pass(pushed))
                checks.pop()
                self.assertTrue(checked.checks_pass(pushed))
                platform["pulls"][0]["reviewDecision"] = "REVIEW_REQUIRED"
                checked.promote()
                self.assertEqual("awaiting_review", checked.emitted["status"])
                self.assertEqual(0, platform["merges"])
                platform["pulls"][0]["reviewDecision"] = ""
                checked.promote()
            self.assertTrue(checked.emitted["ready"])
            self.assertEqual(1, platform["merges"])
            self.assertEqual("", checked.git("ls-remote", "origin", "refs/heads/catalog/v*").stdout)
            with patch.dict(os.environ, {
                "CATALOG_PROMOTED_COMMIT": checked.emitted["source_commit"],
                "CATALOG_PROMOTED_CHECKOUT": str(checked.emitted["checkout"]),
            }):
                checked.channels()
            for path in (checked.root / "channels").iterdir():
                checked.fixture["accepted_digests"].add(publication.checksum(path))
            checked.finish()
            final = {name: checked.read_branch(name, "channel.json") for name in channel_state}
            self.assertTrue(publication.completed(record, final))
            self.assertEqual(checked.emitted["source_commit"], final["catalog/v2"]["document"]["publication"]["source_commit"])
            self.assertEqual("published", checked.emitted["status"])
            self.assertEqual(1, platform["creates"])
            self.assertEqual(1, platform["merges"])

    def test_pending_schema_and_duplicate_fields_refuse_ambiguous_inputs(self):
        for field, value in (("workflow_run_id", True), ("schema_version", True), ("receipt_tag", "../wrong"), ("preparation_commit", "main")):
            with self.subTest(field=field):
                record = dict(self.fixture["record"], **{field: value})
                with self.assertRaises(publication.PublicationError):
                    publication.validate_pending(record)
        path = self.root / "duplicate.json"
        path.write_text('{"version":1,"version":2}', encoding="utf-8")
        with self.assertRaises(publication.PublicationError):
            publication.read_json(path)


class GitPublicationTests(unittest.TestCase):
    def test_concurrent_branch_change_rejects_stale_publication(self):
        with tempfile.TemporaryDirectory(prefix="starmap-publication-git-") as temporary:
            root = Path(temporary)
            remote, checkout = root / "origin.git", root / "checkout"
            publication.command(["git", "init", "--bare", remote])
            publication.command(["git", "clone", remote, checkout])
            client = publication.Publisher(root / "publisher", "agentstation/starmap", 42, source=checkout)
            document = root / "channel.json"
            with patch.object(client, "attest"):
                publication.write_json(document, {"sequence": 1})
                original = client.push_document("catalog/v2", "channel.json", document, "")
                publication.write_json(document, {"sequence": 2})
                accepted = client.push_document("catalog/v2", "channel.json", document, original)
                publication.write_json(document, {"sequence": 99})
                with self.assertRaises(publication.PublicationError):
                    client.push_document("catalog/v2", "channel.json", document, original)
            actual = publication.command(["git", "--git-dir", remote, "rev-parse", "refs/heads/catalog/v2"]).stdout.strip()
            self.assertEqual(accepted, actual)
            payload = publication.command(["git", "--git-dir", remote, "show", f"{actual}:channel.json"]).stdout
            self.assertEqual({"sequence": 2}, json.loads(payload))


if __name__ == "__main__":
    unittest.main()
