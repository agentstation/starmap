"""Keep completed publication retries separate from fresh acquisition."""

import copy
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import catalog_publication as publication
from test_catalog_publication_hosted import RetryEvidenceFixture


class CompletedRetryTests(RetryEvidenceFixture, unittest.TestCase):
    def setUp(self):
        super().setUp()
        self.channels = {
            name: {"commit": "c" * 40, "document": {"channel": name, "tag": self.record["artifact_tag"]}}
            for name in ("catalog/v1", "catalog/v2")
        }
        self.channels["catalog/v2"]["document"]["publication"] = {
            "receipt_tag": self.record["receipt_tag"],
            "receipt": {"checksum": self.record["receipt_checksum"]},
            "checkpoint": {"checksum": self.record["checkpoint_checksum"]},
        }

    def inspect(self, run_id, *, receipt="", event="workflow_dispatch"):
        with tempfile.TemporaryDirectory() as directory:
            client = publication.Publisher(Path(directory), "agentstation/starmap", run_id)
            branches = copy.deepcopy(self.channels)
            branches[publication.PENDING_BRANCH] = {"commit": "b" * 40, "document": self.record}
            with (patch.dict(os.environ, {"GITHUB_EVENT_NAME": event, "CATALOG_RETRY_RECEIPT": receipt}),
                  patch.object(client, "build"),
                  patch.object(client, "read_branch", side_effect=lambda name, _: branches[name]),
                  patch.object(client, "rejected", return_value=False),
                  patch.object(client, "api", return_value={"artifacts": []}) as api,
                  patch.object(client, "recover") as recover):
                client.inspect()
                return publication.read_json(client.control), recover.call_args_list, api.call_args_list

    def test_completed_original_run_recovers_without_workflow_artifacts(self):
        control, recovered, requests = self.inspect(self.record["workflow_run_id"])
        self.assertTrue(control["active"])
        self.assertFalse(control["acquire"])
        self.assertEqual(self.record, recovered[0].args[0])
        self.assertEqual([], requests)

    def test_explicit_receipt_replays_with_current_publisher(self):
        control, recovered, requests = self.inspect(99, receipt=self.record["receipt_checksum"])
        self.assertTrue(control["active"])
        self.assertFalse(control["acquire"])
        self.assertEqual(self.record, recovered[0].args[0])
        self.assertEqual([], requests)

    def test_new_scheduled_run_still_acquires(self):
        control, recovered, _ = self.inspect(99, event="schedule")
        self.assertTrue(control["acquire"])
        self.assertEqual([], recovered)

    def test_retry_rejects_wrong_receipt_and_nonmanual_event(self):
        for receipt, event in (("invalid", "workflow_dispatch"),
                               ("sha256:" + "e" * 64, "workflow_dispatch"),
                               (self.record["receipt_checksum"], "schedule")):
            with self.subTest(receipt=receipt, event=event), self.assertRaises(publication.PublicationError):
                self.inspect(99, receipt=receipt, event=event)

    def test_new_receipt_for_same_artifact_is_not_a_completed_retry(self):
        newer = dict(self.record, receipt_checksum="sha256:" + "e" * 64, receipt_tag="catalog-run-" + "e" * 64)
        self.assertFalse(publication.completed(newer, self.channels))
        self.assertTrue(publication.completed(self.record, self.channels))

    def test_partial_same_artifact_run_finishes_before_new_acquisition(self):
        # Legacy freshness can publish before the new receipt without changing the artifact tag.
        self.record = dict(self.record, receipt_checksum="sha256:" + "e" * 64, receipt_tag="catalog-run-" + "e" * 64)
        control, recovered, requests = self.inspect(99, event="schedule")
        self.assertTrue(control["active"])
        self.assertFalse(control["acquire"])
        self.assertEqual(self.record, recovered[0].args[0])
        self.assertEqual([], requests)

    def test_explicit_retry_requires_both_channels_completed(self):
        self.channels["catalog/v1"]["document"] = None
        with self.assertRaises(publication.PublicationError):
            self.inspect(99, receipt=self.record["receipt_checksum"])


if __name__ == "__main__":
    unittest.main()
