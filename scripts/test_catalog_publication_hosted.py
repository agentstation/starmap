"""Reject incomplete, stale, or unrelated hosted publication evidence."""

import copy
import unittest

import catalog_publication_hosted as hosted


class PromotionEvidenceFixture:
    def setUp(self):
        self.pull = {
            "user": {"login": hosted.BOT, "type": "Bot"},
            "base": {"repo": {"full_name": hosted.REPOSITORY}, "ref": "main"},
            "head": {"repo": {"full_name": hosted.REPOSITORY}, "sha": "a" * 40},
            "merged": True, "state": "closed", "merge_commit_sha": "b" * 40,
            "merged_at": "2026-09-17T12:00:00Z",
        }
        self.protection = {"required_status_checks": {"strict": True, "checks": [
            {"context": name, "app_id": hosted.ACTIONS_APP} for name in hosted.REQUIRED_CHECKS[:2]]}}
        self.pages = [{"check_runs": [
            {"id": index + 1, "name": name, "head_sha": "a" * 40, "app": {"id": hosted.ACTIONS_APP},
             "status": "completed", "conclusion": "success", "started_at": "2026-09-17T11:00:00Z",
             "completed_at": "2026-09-17T11:59:00Z"}
            for index, name in enumerate(hosted.REQUIRED_CHECKS)]}]

class PromotionEvidenceTests(PromotionEvidenceFixture, unittest.TestCase):
    def test_checked_bot_merge_records_exact_check_identities(self):
        result = hosted.validate_promotion(self.pull, self.protection, self.pages)
        self.assertEqual("b" * 40, result["merge"])
        self.assertEqual(set(hosted.REQUIRED_CHECKS), set(result["checks"]))

    def test_missing_stale_wrong_app_and_late_checks_fail(self):
        for field, value in (("head_sha", "c" * 40), ("app", {"id": 7}),
                             ("conclusion", "failure"), ("status", "in_progress"),
                             ("completed_at", "2026-09-17T12:01:00Z"),
                             ("started_at", "2026-09-17T12:01:00Z")):
            with self.subTest(field=field):
                pages = copy.deepcopy(self.pages)
                pages[0]["check_runs"][0][field] = value
                with self.assertRaises(ValueError):
                    hosted.validate_promotion(self.pull, self.protection, pages)
        with self.assertRaises(ValueError):
            hosted.validate_promotion(self.pull, self.protection, [])

    def test_latest_pre_merge_attempt_controls_acceptance(self):
        check = copy.deepcopy(self.pages[0]["check_runs"][0])
        check.update(id=99, conclusion="failure")
        self.pages.append({"check_runs": [check]})
        with self.assertRaises(ValueError):
            hosted.validate_promotion(self.pull, self.protection, self.pages)
        check["started_at"] = "2026-09-17T12:01:00Z"
        self.assertEqual(1, hosted.validate_promotion(self.pull, self.protection, self.pages)["checks"][check["name"]])

    def test_unmerged_foreign_or_unprotected_promotion_fails(self):
        for mutation in (lambda p, r: p.update(merged=False),
                         lambda p, r: p["user"].update(login="human"),
                         lambda p, r: p["base"].update(ref="development"),
                         lambda p, r: p["head"]["repo"].update(full_name="fork/starmap"),
                         lambda p, r: r["required_status_checks"].update(strict=False),
                         lambda p, r: r["required_status_checks"].update(checks=[])):
            pull, rules = copy.deepcopy(self.pull), copy.deepcopy(self.protection)
            mutation(pull, rules)
            with self.assertRaises(ValueError):
                hosted.validate_promotion(pull, rules, self.pages)


class RetryEvidenceFixture:
    def setUp(self):
        digest = "sha256:" + "d" * 64
        self.record = {"schema_version": 1, "run_id": "github-10", "workflow_run_id": 10,
                       "preparation_commit": "a" * 40, "profile_checksum": digest,
                       "receipt_checksum": digest, "checkpoint_checksum": digest,
                       "archive_checksum": digest, "catalog_checksum": digest,
                       "generation_id": "generation", "artifact_tag": "catalog-" + "d" * 64,
                       "receipt_tag": "catalog-run-" + "d" * 64}
        self.releases = []
        identifier = 1
        for tag, names in ((self.record["artifact_tag"], ("starmap-catalog.tar.gz", "starmap-catalog.tar.gz.sha256", "starmap-catalog.intoto.json")),
                           (self.record["receipt_tag"], ("starmap-catalog-run.json", "starmap-catalog-state.json"))):
            assets = []
            for name in names:
                assets.append({"id": identifier, "name": name, "state": "uploaded", "size": 100,
                               "digest": digest, "created_at": "2026-09-17T10:00:00Z"})
                identifier += 1
            self.releases.append({"tag_name": tag, "draft": False, "prerelease": False, "assets": assets})
        names = ("Restore accepted or pending publication", "Publish and verify immutable public inputs",
                 "Promote exact input through checked pull request", "Stage channels after verified merge",
                 "Attest both discovery channels", "Publish and verify both discovery channels")
        self.completion = {
            "id": 10, "run_attempt": 1, "repository": {"full_name": hosted.REPOSITORY},
            "head_branch": "main", "path": ".github/workflows/catalog-generation.yaml",
            "event": "workflow_dispatch", "status": "completed", "conclusion": "success", "head_sha": "a" * 40,
            "created_at": "2026-09-17T09:00:00Z", "run_started_at": "2026-09-17T09:00:00Z",
            "updated_at": "2026-09-17T12:00:00Z",
            "jobs": [{"name": "generate", "run_id": 10, "run_attempt": 1, "status": "completed", "conclusion": "success",
                      "steps": [{"name": name, "status": "completed", "conclusion": "success"} for name in names]}],
        }
        self.retry = copy.deepcopy(self.completion)
        self.retry.update(run_attempt=2, run_started_at="2026-09-17T13:00:00Z", updated_at="2026-09-17T14:00:00Z")
        self.retry["jobs"][0]["run_attempt"] = 2

class RetryEvidenceTests(RetryEvidenceFixture, unittest.TestCase):
    def test_same_run_new_attempt_can_prove_unchanged_assets(self):
        result = hosted.validate_same_bytes_retry(self.record, self.releases, self.releases, self.completion, self.retry)
        self.assertEqual([10, 2], result["retry_run"])
        self.assertEqual([1, 2, 3, 4, 5], result["asset_ids"])

    def test_replaced_changed_missing_and_duplicate_assets_fail(self):
        for mutation in (lambda r: r[0]["assets"][0].update(id=99),
                         lambda r: r[0]["assets"][0].update(digest="sha256:" + "e" * 64),
                         lambda r: r[0]["assets"].pop(),
                         lambda r: r[0]["assets"].append(r[0]["assets"][0]),
                         lambda r: r[0].update(draft=True),
                         lambda r: r[0]["assets"][0].update(size=0)):
            releases = copy.deepcopy(self.releases)
            mutation(releases)
            with self.assertRaises(ValueError):
                hosted.validate_same_bytes_retry(self.record, self.releases, releases, self.completion, self.retry)

    def test_skipped_publication_or_unrelated_retry_cannot_qualify(self):
        for mutation in (lambda r: r.update(event="pull_request"),
                         lambda r: r.update(head_branch="untrusted"),
                         lambda r: r.update(conclusion="failure"),
                         lambda r: r.update(run_started_at="2026-09-17T11:00:00Z"),
                         lambda r: r["jobs"][0].update(run_attempt=1),
                         lambda r: r["jobs"][0]["steps"][-1].update(conclusion="skipped")):
            retry = copy.deepcopy(self.retry)
            mutation(retry)
            with self.assertRaises(ValueError):
                hosted.validate_same_bytes_retry(self.record, self.releases, self.releases, self.completion, retry)


if __name__ == "__main__":
    unittest.main()
