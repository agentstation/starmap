"""Verify explicit rejection at the publication recovery boundary."""

import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import catalog_publication as publication


class RejectionTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.publisher = publication.Publisher(self.root / 'run', 'agentstation/starmap', 43, source=self.root)
        self.record = json.loads((Path(__file__).resolve().parents[1] / '.github/catalog-rejections.json').read_text())[0]['pending']
        self.entry = {'pending': self.record, 'pull_request': 165, 'reason': 'Billing and protocol contract failures.'}
        (self.root / '.github').mkdir()
        self.path = self.root / '.github/catalog-rejections.json'
        self.path.write_text(json.dumps([self.entry]))
        self.channels = {name: {'document': None} for name in ('catalog/v1', 'catalog/v2')}

    def test_unlisted_candidate_is_not_rejected(self):
        other = copy.deepcopy(self.record)
        other['receipt_checksum'] = 'sha256:' + 'a' * 64
        self.assertFalse(self.publisher.rejected(other, self.channels))

    def test_exact_candidate_requires_retained_authenticated_inputs(self):
        with patch.object(self.publisher, 'download') as download, patch.object(self.publisher, 'attest') as attest, patch.object(publication, 'checksum', side_effect=[self.record[k] for k in ('archive_checksum', 'receipt_checksum', 'checkpoint_checksum')]):
            self.assertTrue(self.publisher.rejected(self.record, self.channels))
        self.assertEqual(download.call_count, 2)
        self.assertEqual(attest.call_count, 3)

    def test_changed_candidate_cannot_use_rejection(self):
        changed = dict(self.record, generation_id='different')
        with self.assertRaisesRegex(publication.PublicationError, 'differs'):
            self.publisher.rejected(changed, self.channels)

    def test_either_channel_prevents_rejection(self):
        for name in self.channels:
            with self.subTest(channel=name):
                channels = copy.deepcopy(self.channels)
                channels[name]['document'] = {'tag': self.record['artifact_tag']}
                with self.assertRaisesRegex(publication.PublicationError, 'channel'):
                    self.publisher.rejected(self.record, channels)

    def test_missing_evidence_prevents_rejection(self):
        with patch.object(self.publisher, 'download', side_effect=publication.PublicationError('missing evidence')):
            with self.assertRaisesRegex(publication.PublicationError, 'missing evidence'):
                self.publisher.rejected(self.record, self.channels)

    def test_modified_evidence_prevents_rejection(self):
        with patch.object(self.publisher, 'download'), patch.object(publication, 'checksum', return_value='sha256:' + 'f' * 64):
            with self.assertRaisesRegex(publication.PublicationError, 'digest'):
                self.publisher.rejected(self.record, self.channels)

    def test_inspection_selects_new_acquisition_without_restoring_rejected_state(self):
        states = [self.channels['catalog/v1'], self.channels['catalog/v2'], {'document': self.record, 'commit': 'old-head'}]
        with patch.object(self.publisher, 'build'), patch.object(self.publisher, 'read_branch', side_effect=states), patch.object(self.publisher, 'rejected', return_value=True), patch.object(self.publisher, 'api', return_value={'artifacts': []}), patch.object(self.publisher, 'recover') as recover, patch.dict('os.environ', {'GITHUB_EVENT_NAME': 'workflow_dispatch'}):
            self.publisher.inspect()
        control = publication.read_json(self.publisher.control)
        self.assertTrue(control['acquire'])
        self.assertEqual(control['pending_head'], 'old-head')
        self.assertEqual(control['channels'], self.channels)
        recover.assert_not_called()

    def test_workflow_completion_does_not_acquire_after_rejection(self):
        states = [self.channels['catalog/v1'], self.channels['catalog/v2'], {'document': self.record, 'commit': 'old-head'}]
        with patch.object(self.publisher, 'build'), patch.object(self.publisher, 'read_branch', side_effect=states), patch.object(self.publisher, 'rejected', return_value=True), patch.object(self.publisher, 'recover') as recover, patch.dict('os.environ', {'GITHUB_EVENT_NAME': 'workflow_run'}):
            self.publisher.inspect()
        control = publication.read_json(self.publisher.control)
        self.assertFalse(control['active'])
        self.assertFalse(control['acquire'])
        recover.assert_not_called()

    def test_duplicate_rejection_is_invalid(self):
        self.path.write_text(json.dumps([self.entry, self.entry]))
        with self.assertRaisesRegex(publication.PublicationError, 'duplicate'):
            self.publisher.rejected(self.record, self.channels)


if __name__ == '__main__':
    unittest.main()
