"""Check the catalog recording result and story evidence."""

import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
import xml.etree.ElementTree as ET


SPEC = importlib.util.spec_from_file_location('record_demo', Path(__file__).with_name('record-demo.py'))
RECORDER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RECORDER)


class RecordingEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        payload = b'{"models":[]}'
        (self.directory / 'catalog.json').write_bytes(payload)
        self.outcome = {'model_id': 'openai/gpt-6.1-sol', 'generation_id': 'generation-fixture', 'checksum': 'verified'}
        documents = {'ready.json': {'data': {'status': 'ready'}},
                     'model.json': {'data': {'id': self.outcome['model_id'], 'name': 'GPT-6.1 Sol', 'author_ids': ['openai']}},
                     'manifest.json': {'generation_id': self.outcome['generation_id'], 'schema_version': 19,
                                       'validation': {'status': 'passed'},
                                       'payload': {'checksum': 'sha256:' + hashlib.sha256(payload).hexdigest(), 'size_bytes': len(payload)}},
                     'outcome.json': self.outcome}
        for name, value in documents.items():
            (self.directory / name).write_text(json.dumps(value))
        (self.directory / 'payload.headers').write_text('X-Starmap-Generation-ID: generation-fixture\n')
        (self.directory / 'complete').touch()

    def test_catalog_change_refuses_publication(self):
        RECORDER.verify_catalog_inputs({'embedded_tree_sha256': 'before'}, {'embedded_tree_sha256': 'before'})
        with self.assertRaisesRegex(ValueError, 'catalog inputs changed'):
            RECORDER.verify_catalog_inputs({'embedded_tree_sha256': 'before'}, {'embedded_tree_sha256': 'after'})

    def test_catalog_snapshot_binds_local_records_and_retained_observations(self):
        catalog = self.directory / 'internal/embedded/catalog'
        catalog.mkdir(parents=True)
        author = catalog / 'authors/openai/models/gpt-6.1-sol.yaml'
        provider = catalog / 'providers/openai/models/gpt-6.1-sol.yaml'
        for path in [author, provider]:
            path.parent.mkdir(parents=True)
            path.write_text('id: openai/gpt-6.1-sol\n')
        manifest = {'generation_id': 'generation-test', 'schema_version': 19, 'generated_at': '2026-10-07T00:00:00Z',
                    'payload': {'checksum': 'sha256:test'}, 'source_observations': [{'source': 'providers', 'id': 'retained'}],
                    'review_candidates': [{'id': 'retained-review'}]}
        (catalog / 'generation-manifest.json').write_text(json.dumps(manifest))
        (catalog / 'generation.json').write_text(json.dumps({'generation_id': manifest['generation_id']}))
        subprocess.run(['git', 'init', '-q', str(self.directory)], check=True, capture_output=True)
        subprocess.run(['git', '-c', 'user.name=Test', '-c', 'user.email=test@example.com', 'commit', '-q', '--allow-empty', '-m', 'Test'],
                       cwd=self.directory, check=True, capture_output=True)
        before = RECORDER.catalog_inputs(self.directory)
        relative_author = str(author.relative_to(self.directory))
        self.assertEqual(before['input_files'][relative_author]['sha256'], RECORDER.sha256(author))
        self.assertIn(relative_author, before['working_tree_changes'])
        self.assertEqual(before['provider_observations'], manifest['source_observations'])
        self.assertEqual(before['review_candidate_count'], 1)
        self.assertEqual(before['committed_generation']['generation_id'], manifest['generation_id'])
        author.write_text('id: openai/gpt-6.1-sol\nname: GPT-6.1 Sol\n')
        after = RECORDER.catalog_inputs(self.directory)
        self.assertNotEqual(before['embedded_tree_sha256'], after['embedded_tree_sha256'])
        with self.assertRaisesRegex(ValueError, 'catalog inputs changed'):
            RECORDER.verify_catalog_inputs(before, after)

    def test_model_name_matches_selected_model(self):
        path = self.directory / 'model.json'
        document = json.loads(path.read_text())
        document['data']['name'] = 'Old model name'
        path.write_text(json.dumps(document))
        with self.assertRaisesRegex(ValueError, 'display name'):
            RECORDER.verify(self.directory)

    def test_served_generation_matches_reviewed_catalog(self):
        result = RECORDER.verify(self.directory)
        catalog = {'bootstrap': {'generation_id': result['generation_id'],
                                 'payload': {'checksum': 'sha256:' + result['payload_sha256'], 'size_bytes': result['payload_size_bytes']}}}
        RECORDER.verify_generation_snapshot(result, catalog)
        for field in ['generation_id', 'payload_sha256', 'payload_size_bytes']:
            with self.subTest(field=field):
                changed = dict(result, **{field: 'wrong'})
                with self.assertRaisesRegex(ValueError, 'differs from the reviewed catalog'):
                    RECORDER.verify_generation_snapshot(changed, catalog)

    def test_result_identifies_observed_model_and_generation(self):
        self.assertEqual(RECORDER.verify(self.directory)['outcome'], self.outcome)

    def test_result_refuses_wrong_model_or_generation(self):
        for field in ['model_id', 'generation_id']:
            with self.subTest(field=field):
                wrong = dict(self.outcome, **{field: 'wrong'})
                (self.directory / 'outcome.json').write_text(json.dumps(wrong))
                with self.assertRaisesRegex(ValueError, 'Ending does not identify'):
                    RECORDER.verify(self.directory)

    def test_checksum_refuses_modified_payload(self):
        (self.directory / 'catalog.json').write_bytes(b'changed')
        with self.assertRaisesRegex(ValueError, 'Payload digest mismatch'):
            RECORDER.verify(self.directory)

    def test_incomplete_tape_cannot_claim_success(self):
        (self.directory / 'complete').unlink()
        with self.assertRaisesRegex(ValueError, 'Tape did not finish'):
            RECORDER.verify(self.directory)

    def test_story_requires_order_and_observed_ending(self):
        rows = ['Starmap / Serve a model catalog', '1 / Start the catalog server',
                '2 / Read a model over HTTP', '3 / Fetch and verify a generation',
                'Result / Model read and payload verified', 'Model: openai/gpt-6.1-sol',
                'Generation: generation-fixture...tion-fixture', 'Payload checksum: OK',
                'Next: configure a catalog subscriber.']
        def root(values):
            tree = ET.Element('{http://www.w3.org/2000/svg}svg')
            for value in values:
                ET.SubElement(tree, '{http://www.w3.org/2000/svg}text').text = value
            return tree
        self.assertTrue(RECORDER.verify_story(root(rows), self.outcome)['ending_matches_observed_data'])
        with self.assertRaisesRegex(ValueError, 'out of order'):
            RECORDER.verify_story(root([rows[1], rows[0]] + rows[2:]), self.outcome)
        with self.assertRaisesRegex(ValueError, 'omits the verified result'):
            RECORDER.verify_story(root(rows[:-1]), self.outcome)
        with self.assertRaisesRegex(ValueError, 'internal capture command'):
            RECORDER.verify_story(root(rows + ['$ touch complete']), self.outcome)


if __name__ == '__main__':
    unittest.main()
