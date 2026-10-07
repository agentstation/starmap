"""Test the release demonstration adapters against a synthetic Starport tree."""

import hashlib
import json
import subprocess
import tempfile
import unittest
from copy import deepcopy
from pathlib import Path
from unittest.mock import patch

import catalog_product_verify as verifier


class ReleaseRecordFixture:
    """Build a synthetic Starport tree with a release record and a stub demonstration verifier."""

    MANIFEST = 'docs/assets/starport-demo.json'
    RECORD = 'docs/assets/first-use-v1.3.0'

    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.record_directory = self.root / self.RECORD
        self.record_directory.mkdir(parents=True)
        files = {name: name.encode() for name in verifier.REHEARSAL_OUTPUTS + ('first-use-uncut.gif',)}
        for name, data in files.items():
            (self.record_directory / name).write_bytes(data)
        digest = lambda name: hashlib.sha256(files[name]).hexdigest()
        self.manifest = {'schema_version': 1, 'scenes': ['install', 'catalog', 'setup', 'answer', 'next'],
                         'readme_binding': {name: f'{self.RECORD}/{file}' for name, file in (
                             ('gif', 'first-use.gif'), ('poster', 'poster.png'), ('uncut', 'first-use-uncut.gif'),
                             ('transcript', 'TRANSCRIPT.md'))} | {'alt_text_required': True},
                         'current_rehearsal': 'docs/proof/readme-demo/rehearsal-2026-10-04'}
        self.record = {
            'schema_version': 1, 'kind': 'release', 'qualifies_release_cases': True, 'real_provider': True,
            'candidate': {'source': 'release', 'run_id': None, 'pull_request': None, 'head_commit': 'a' * 40,
                          'release_tag': 'v1.3.0', 'archive_name': 'starport_1.3.0_darwin_arm64.tar.gz',
                          'archive_sha256': 'b' * 64, 'binary_sha256': 'c' * 64, 'checksum_verified': True,
                          'attestation_verified': True, 'attestation': {'signer_workflow': 'release.yaml'}},
            'fixtures': [], 'outputs': {name: {'sha256': digest(name)} for name in verifier.REHEARSAL_OUTPUTS},
            'uncut': {'path': 'first-use-uncut.gif', 'sha256': digest('first-use-uncut.gif')}}
        self.readme = f'![Starport first use]({self.RECORD}/first-use.gif)\n<img src="{self.RECORD}/poster.png" alt="Poster">\n'
        self.report = {'status': 'PASS', 'manifest': self.MANIFEST, 'record': self.RECORD, 'kind': 'release',
                       'candidate_head': 'a' * 40,
                       'checks': [{'id': name, 'status': 'PASS', 'detail': 'ok'} for name in verifier.REHEARSAL_VERIFIER_CHECKS],
                       'readme_media': [f'{self.RECORD}/first-use.gif', f'{self.RECORD}/poster.png']}
        self.exit_code = 0
        self.observations = {name: True for name in ('readable_at_900px', 'human_pacing_review', 'no_secret_material')}
        self.review = None
        script = self.root / 'scripts/verify-readme-demo.sh'
        script.parent.mkdir()
        script.write_text('printf "%s\\n" "$@" > arguments.out\necho run >> calls.out\ncat report.out\nexit "$(cat exit.code)"\n')

    def write(self):
        (self.root / self.MANIFEST).write_text(json.dumps(self.manifest))
        (self.record_directory / 'record.json').write_text(json.dumps(self.record))
        (self.root / 'README.md').write_text(self.readme)
        (self.root / 'report.out').write_text(self.report if isinstance(self.report, str) else json.dumps(self.report))
        (self.root / 'exit.code').write_text(str(self.exit_code))
        review = self.review or {
            'schema_version': 1, 'verdict': 'PASS', 'release': 'v1.3.0', 'reviewer': 'A reviewer', 'date': '2026-10-06',
            'observations': self.observations,
            'inputs': {name: hashlib.sha256((self.root / name).read_bytes()).hexdigest()
                       for name in ['README.md'] + [f'{self.RECORD}/{name}' for name in verifier.REHEARSAL_OUTPUTS]}}
        (self.record_directory / 'review.json').write_text(json.dumps(review))


class ReleaseDemoTests(ReleaseRecordFixture, unittest.TestCase):
    """Exercise the release demonstration adapter against a synthetic Starport tree."""

    def entry(self, **changes):
        return {'kind': 'release_demo', 'repository': 'starport', 'manifest': self.MANIFEST, 'record': self.RECORD,
                'release': 'v1.3.0', 'checks': ['scene_order', 'catalog_keyless'], **changes}

    def check(self, entry=None, write=True):
        verifier.RELEASE_DEMO_REPORTS.clear()
        if write:
            self.write()
        return verifier.run_check('A36.install_catalog_setup_answer_order', entry or self.entry(), {'starport': self.root})

    def test_passing_record_and_report_pass_with_selected_checks(self):
        result = self.check()
        self.assertEqual(result['status'], 'PASS', result)
        self.assertEqual(result['checks'], [{'id': name, 'status': 'PASS', 'detail': 'ok'} for name in ('scene_order', 'catalog_keyless')])
        self.assertEqual((result['release'], result['candidate_head'], result['archive_sha256']), ('v1.3.0', 'a' * 40, 'b' * 64))
        self.assertEqual(result['record_sha256'], hashlib.sha256((self.record_directory / 'record.json').read_bytes()).hexdigest())
        self.assertIn('did not reinstall software or call a provider', result['scope'])
        self.assertEqual((self.root / 'arguments.out').read_text().splitlines(),
                         ['--manifest', self.MANIFEST, '--record', self.RECORD, '--json'])

    def test_named_observations_pass_with_a_current_review(self):
        result = self.check(self.entry(checks=['human_review'], observations=['human_pacing_review']))
        self.assertEqual(result['status'], 'PASS', result)
        self.assertEqual(result['observations'], ['human_pacing_review'])
        self.assertIn('recorded human review', result['scope'])

    def test_absent_evidence_is_unverified(self):
        self.assertEqual(verifier.run_check('A36', self.entry(), {})['status'], 'UNVERIFIED')
        result = self.check(write=False)
        self.assertEqual(result['status'], 'UNVERIFIED')
        self.assertIn('manifest is absent', result['reason'])
        self.write()
        (self.record_directory / 'record.json').unlink()
        result = self.check(write=False)
        self.assertEqual(result, {'status': 'UNVERIFIED', 'reason': f'The release record is absent: {self.RECORD}/record.json'})
        self.assertFalse((self.root / 'calls.out').exists())
        self.write()
        (self.record_directory / 'review.json').unlink()
        result = self.check(self.entry(observations=['readable_at_900px']), write=False)
        self.assertEqual(result['status'], 'UNVERIFIED')
        self.assertIn('review is absent', result['reason'])

    def test_failed_named_check_fails_and_names_it(self):
        self.report['status'] = 'FAIL'
        self.report['checks'][4]['status'] = 'FAIL'
        self.exit_code = 1
        result = self.check()
        self.assertEqual(result['status'], 'FAIL')
        self.assertIn('Failed checks: scene_order', result['reason'])

    def test_invalid_or_incomplete_report_fails(self):
        incomplete = dict(self.report, checks=self.report['checks'][1:])
        rehearsal = dict(self.report, kind='rehearsal')
        cases = {
            'exit 2': (2, self.report, 'reports INVALID'),
            'status INVALID': (0, dict(self.report, status='INVALID'), 'reports INVALID'),
            'missing check': (0, incomplete, 'Missing checks: output_hashes'),
            'rehearsal report': (0, rehearsal, "'rehearsal' record"),
            'no JSON': (0, 'progress only', 'no valid JSON report'),
        }
        for name, (code, report, reason) in cases.items():
            with self.subTest(name):
                self.exit_code, self.report = code, report
                result = self.check()
                self.assertEqual(result['status'], 'FAIL', result)
                self.assertIn(reason, result['reason'])

    def test_unqualified_release_record_fails(self):
        original = self.record

        def mutate(change):
            record = deepcopy(original)
            change(record)
            return record
        cases = {
            'kind rehearsal': (lambda r: r.update(kind='rehearsal'), 'kind release'),
            'does not qualify': (lambda r: r.update(qualifies_release_cases=False), 'qualify release cases'),
            'fixtures': (lambda r: r.update(fixtures=[{'name': 'fixture_upstream'}]), 'fixtures'),
            'no real provider': (lambda r: r.update(real_provider=False), 'real provider'),
            'CI source': (lambda r: r['candidate'].update(source='ci-run'), 'published release'),
            'tag mismatch': (lambda r: r['candidate'].update(release_tag='v1.2.0'), "'v1.2.0', not v1.3.0"),
            'other archive': (lambda r: r['candidate'].update(archive_name='starport_1.3.0_linux_arm64.tar.gz'), 'darwin arm64'),
            'unverified checksum': (lambda r: r['candidate'].update(checksum_verified=False), 'verified checksum'),
            'unverified attestation': (lambda r: r['candidate'].pop('attestation_verified'), 'verified attestation'),
            'short head': (lambda r: r['candidate'].update(head_commit='a' * 12), 'tag commit'),
            'short archive hash': (lambda r: r['candidate'].update(archive_sha256='b' * 12), 'archive_sha256'),
        }
        for name, (change, reason) in cases.items():
            with self.subTest(name):
                self.record = mutate(change)
                result = self.check()
                self.assertEqual(result['status'], 'FAIL', result)
                self.assertIn(reason, result['reason'])
        self.record = original
        self.assertEqual(self.check()['status'], 'PASS')

    def test_invalid_record_json_fails(self):
        self.write()
        (self.record_directory / 'record.json').write_text('{')
        result = self.check(write=False)
        self.assertEqual(result['status'], 'FAIL')
        self.assertIn('invalid JSON', result['reason'])

    def test_readme_bound_or_linked_elsewhere_fails(self):
        self.manifest['readme_binding']['gif'] = 'docs/assets/first-use-v1.2.0/first-use.gif'
        result = self.check()
        self.assertEqual(result['status'], 'FAIL')
        self.assertIn('outside the release record: docs/assets/first-use-v1.2.0/first-use.gif', result['reason'])
        self.manifest['readme_binding']['gif'] = f'{self.RECORD}/first-use.gif'
        self.report['readme_media'] = ['docs/assets/first-use-v1.2.0/first-use.gif', f'{self.RECORD}/poster.png']
        result = self.check()
        self.assertEqual(result['status'], 'FAIL')
        self.assertIn(f'does not link the bound release media: {self.RECORD}/first-use.gif', result['reason'])
        self.manifest.pop('readme_binding')
        self.assertIn('bind the README GIF and poster', self.check()['reason'])

    def test_stale_or_incomplete_review_fails(self):
        entry = self.entry(observations=['readable_at_900px', 'no_secret_material'])
        self.write()
        complete = json.loads((self.record_directory / 'review.json').read_text())

        def changed(**fields):
            return complete | fields
        cases = {
            'FAIL verdict': (changed(verdict='FAIL'), 'passing release media review'),
            'other release': (changed(release='v1.2.0'), 'not a review of v1.3.0'),
            'no reviewer': (changed(reviewer=' '), 'names no reviewer'),
            'bad date': (changed(date='2026-13-01'), 'ISO date'),
            'missing observation': (changed(observations={'readable_at_900px': True}), 'omits observations: no_secret_material'),
            'false observation': (changed(observations={'readable_at_900px': True, 'no_secret_material': False}),
                                  'no_secret_material'),
            'missing input': (changed(inputs={'README.md': complete['inputs']['README.md']}), f'{self.RECORD}/first-use.gif'),
            'stale input': (changed(inputs=complete['inputs'] | {'README.md': '0' * 64}), 'stale: README.md'),
            'escaping input': (changed(inputs=complete['inputs'] | {'../outside': '0' * 64}), 'inside their repository'),
        }
        for name, (review, reason) in cases.items():
            with self.subTest(name):
                self.review = review
                result = self.check(entry)
                self.assertEqual(result['status'], 'FAIL', result)
                self.assertIn(reason, result['reason'])
        self.review = None
        self.assertEqual(self.check(entry)['status'], 'PASS')
        self.write()
        (self.root / 'README.md').write_text(self.readme + 'Changed after review.\n')
        result = self.check(entry, write=False)
        self.assertEqual(result['status'], 'FAIL')
        self.assertIn('stale: README.md', result['reason'])

    def test_invalid_entry_fails(self):
        cases = {
            'unknown check': (self.entry(checks=['scene_order', 'secret_scan']), 'Unknown checks: secret_scan'),
            'no checks': (self.entry(checks=[]), 'distinct Starport verifier checks'),
            'duplicate check': (self.entry(checks=['scene_order', 'scene_order']), 'distinct Starport verifier checks'),
            'prerelease tag': (self.entry(release='v1.3.0-rc.1'), 'stable release tag'),
            'no release': ({key: value for key, value in self.entry().items() if key != 'release'}, 'stable release tag'),
            'empty observation': (self.entry(observations=['']), 'named observations'),
            'escaping record': (self.entry(record='../first-use-v1.3.0'), 'inside the repository'),
        }
        for name, (entry, reason) in cases.items():
            with self.subTest(name):
                result = self.check(entry)
                self.assertEqual(result['status'], 'FAIL', result)
                self.assertIn(reason, result['reason'])

    def test_verifier_timeout_is_unverified(self):
        self.write()
        verifier.RELEASE_DEMO_REPORTS.clear()
        timeout = subprocess.TimeoutExpired(['bash'], verifier.REHEARSAL_VERIFIER_TIMEOUT_SECONDS)
        with patch.object(verifier.subprocess, 'run', side_effect=timeout):
            result = verifier.run_check('A36', self.entry(), {'starport': self.root})
        self.assertEqual(result['status'], 'UNVERIFIED')
        self.assertIn('TimeoutExpired', result['reason'])

    def test_registered_roster_runs_the_verifier_once(self):
        registry = verifier.read_json(verifier.REGISTRY)
        entries = {identity: entry for identity, entry in registry['checks'].items() if entry.get('kind') == 'release_demo'}
        self.assertEqual(len(entries), 15)
        self.observations = {name: True for entry in entries.values() for name in entry.get('observations', [])}
        self.write()
        verifier.RELEASE_DEMO_REPORTS.clear()
        for identity, entry in entries.items():
            with self.subTest(identity):
                result = verifier.run_check(identity, entry, {'starport': self.root})
                self.assertEqual(result['status'], 'PASS', result)
                self.assertEqual([row['id'] for row in result['checks']], entry['checks'])
        self.assertEqual((self.root / 'calls.out').read_text().splitlines(), ['run'])

    def test_registry_covers_every_release_demonstration_subcase(self):
        roster = verifier.read_json(verifier.ROSTER)
        registry = verifier.read_json(verifier.REGISTRY)
        verifier.validate_registry(registry, roster, verifier.validate_roster(roster))
        for case in ('A35', 'A36', 'A37', 'A38', 'A39'):
            for identity in roster['required_subcases'][case]:
                with self.subTest(identity):
                    self.assertIn(identity, registry['checks'])
        for identity, entry in registry['checks'].items():
            if entry.get('kind') != 'release_demo':
                continue
            with self.subTest(identity):
                self.assertEqual({key: entry[key] for key in ('repository', 'manifest', 'record', 'release')}, {
                    'repository': 'starport', 'manifest': self.MANIFEST, 'record': self.RECORD, 'release': 'v1.3.0'})
                self.assertTrue(set(entry['checks']) <= set(verifier.REHEARSAL_VERIFIER_CHECKS))
                self.assertEqual(verifier.release_demo(entry, {}), {
                    'status': 'UNVERIFIED', 'reason': 'The Starport repository is unavailable.'})


class ReviewedDemoTests(ReleaseRecordFixture, unittest.TestCase):
    """Exercise the README media review of a release record against synthetic Starmap and Starport trees."""

    PROOF = 'proof/media-review.json'

    def setUp(self):
        super().setUp()
        starmap = tempfile.TemporaryDirectory()
        self.addCleanup(starmap.cleanup)
        self.starmap = Path(starmap.name)
        (self.starmap / 'proof').mkdir()
        (self.starmap / 'proof/readme-light.png').write_bytes(b'light')
        patcher = patch.object(verifier, 'ROOT', self.starmap)
        patcher.start()
        self.addCleanup(patcher.stop)
        self.media_review = {}

    def entry(self, **changes):
        inputs = ['README.md', self.MANIFEST, f'{self.RECORD}/record.json', f'{self.RECORD}/first-use-uncut.gif',
                  *(f'{self.RECORD}/{name}' for name in verifier.REHEARSAL_OUTPUTS)]
        return {'kind': 'reviewed_demo', 'repository': 'starport', 'proof': self.PROOF, 'manifest': self.MANIFEST,
                'record': self.RECORD, 'release': 'v1.3.0', 'required_inputs': inputs,
                'observations': ['readable_at_900px'], **changes}

    def write(self):
        super().write()
        digest = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
        review = {'schema_version': 1, 'verdict': 'PASS', 'release': 'v1.3.0', 'observations': {'readable_at_900px': True},
                  'inputs': {name: digest(self.root / name) for name in self.entry()['required_inputs']},
                  'captures': {'readme-light.png': digest(self.starmap / 'proof/readme-light.png')}}
        (self.starmap / self.PROOF).write_text(json.dumps(review | self.media_review))

    def check(self, entry=None, write=True):
        verifier.RELEASE_DEMO_REPORTS.clear()
        if write:
            self.write()
        return verifier.run_check('E03', entry or self.entry(), {'starport': self.root})

    def test_current_review_and_release_record_pass(self):
        result = self.check()
        self.assertEqual(result['status'], 'PASS', result)
        proof = self.starmap / self.PROOF
        self.assertEqual((result['proof'], result['proof_sha256']), (str(proof), hashlib.sha256(proof.read_bytes()).hexdigest()))
        self.assertEqual(result['checks'], [{'id': name, 'status': 'PASS', 'detail': 'ok'} for name in verifier.REHEARSAL_VERIFIER_CHECKS])
        self.assertEqual((result['release'], result['candidate_head']), ('v1.3.0', 'a' * 40))
        self.assertIn('did not repeat the capture or call a provider', result['scope'])
        self.assertEqual((self.root / 'arguments.out').read_text().splitlines(),
                         ['--manifest', self.MANIFEST, '--record', self.RECORD, '--json'])

    def test_failed_verifier_check_or_unqualified_record_fails(self):
        failed = deepcopy(self.report)
        failed['status'] = 'FAIL'
        failed['checks'][verifier.REHEARSAL_VERIFIER_CHECKS.index('human_review')]['status'] = 'FAIL'
        incomplete = dict(self.report, checks=[row for row in self.report['checks'] if row['id'] != 'human_review'])
        report, record = self.report, self.record
        cases = {
            'failed human review': (1, failed, record, 'Failed checks: human_review'),
            'missing human review': (0, incomplete, record, 'Missing checks: human_review'),
            'rehearsal report': (0, dict(report, kind='rehearsal'), record, "'rehearsal' record"),
            'rehearsal record': (0, report, dict(record, kind='rehearsal'), 'kind release'),
            'other release': (0, report, record | {'candidate': record['candidate'] | {'release_tag': 'v1.2.0'}},
                              "'v1.2.0', not v1.3.0"),
        }
        for name, (code, changed_report, changed_record, reason) in cases.items():
            with self.subTest(name):
                self.exit_code, self.report, self.record = code, changed_report, changed_record
                result = self.check()
                self.assertEqual(result['status'], 'FAIL', result)
                self.assertIn(reason, result['reason'])

    def test_absent_evidence_is_unverified(self):
        self.assertEqual(verifier.run_check('E03', self.entry(), {}),
                         {'status': 'UNVERIFIED', 'reason': 'The Starport repository is unavailable.'})
        result = self.check(write=False)
        self.assertEqual(result['status'], 'UNVERIFIED')
        self.assertIn('manifest is absent', result['reason'])
        self.write()
        (self.record_directory / 'record.json').unlink()
        result = self.check(write=False)
        self.assertEqual(result, {'status': 'UNVERIFIED', 'reason': f'The release record is absent: {self.RECORD}/record.json'})
        self.write()
        (self.starmap / self.PROOF).unlink()
        result = self.check(write=False)
        self.assertEqual(result['status'], 'UNVERIFIED')
        self.assertIn('media-review.json', result['reason'])
        self.assertFalse((self.root / 'calls.out').exists())

    def test_stale_review_is_unverified(self):
        cases = {
            'changed README': (lambda: (self.root / 'README.md').write_text(self.readme + 'Changed after review.\n'),
                               'stale: README.md'),
            'changed record': (lambda: (self.record_directory / 'record.json').write_text(json.dumps(self.record | {'notes': 'x'})),
                               f'stale: {self.RECORD}/record.json'),
            'changed capture': (lambda: (self.starmap / 'proof/readme-light.png').write_bytes(b'changed'),
                                'reviewed capture changed: readme-light.png'),
        }
        for name, (change, reason) in cases.items():
            with self.subTest(name):
                self.write()
                change()
                result = self.check(write=False)
                self.assertEqual(result['status'], 'UNVERIFIED', result)
                self.assertIn(reason, result['reason'])
        self.media_review = {'inputs': {'README.md': hashlib.sha256(self.readme.encode()).hexdigest()}}
        result = self.check()
        self.assertEqual(result['status'], 'UNVERIFIED', result)
        self.assertIn('omits required source inputs', result['reason'])
        self.assertFalse((self.root / 'calls.out').exists())

    def test_review_of_another_release_is_unverified(self):
        self.media_review = {'release': 'v1.2.0'}
        result = self.check()
        self.assertEqual(result['status'], 'UNVERIFIED', result)
        self.assertEqual(result['reason'], 'The media review is not a review of v1.3.0.')
        self.assertFalse((self.root / 'calls.out').exists())
        self.media_review = {}
        self.assertEqual(self.check()['status'], 'PASS')

    def test_invalid_entry_fails(self):
        entry = self.entry()
        cases = {
            'legacy asset directory': (self.entry(asset_directory='docs/assets/first-use-v1.2.0'), 'not an asset directory'),
            'no manifest': ({key: value for key, value in entry.items() if key != 'manifest'}, 'relative manifest path'),
            'no record': ({key: value for key, value in entry.items() if key != 'record'}, 'relative record path'),
            'escaping record': (self.entry(record='../first-use-v1.3.0'), 'relative record path'),
            'no release': ({key: value for key, value in entry.items() if key != 'release'}, 'stable release tag'),
            'prerelease tag': (self.entry(release='v1.3.0-rc.1'), 'stable release tag'),
            'unhashed record': (self.entry(required_inputs=['README.md', self.MANIFEST]), 'hash the demonstration manifest'),
            'no observations': (self.entry(observations=[]), 'named observations'),
        }
        for name, (changed, reason) in cases.items():
            with self.subTest(name):
                result = self.check(changed)
                self.assertEqual(result['status'], 'FAIL', result)
                self.assertIn(reason, result['reason'])

    def test_registry_binds_e03_to_the_release_record(self):
        roster = verifier.read_json(verifier.ROSTER)
        registry = verifier.read_json(verifier.REGISTRY)
        checks = verifier.validate_roster(roster)
        verifier.validate_registry(registry, roster, checks)
        entry = registry['checks']['E03']
        self.assertEqual({key: entry[key] for key in ('kind', 'repository', 'manifest', 'record', 'release')}, {
            'kind': 'reviewed_demo', 'repository': 'starport', 'manifest': self.MANIFEST, 'record': self.RECORD,
            'release': 'v1.3.0'})
        self.assertEqual(entry['required_inputs'], [
            'README.md', self.MANIFEST, *(f'{self.RECORD}/{name}' for name in (
                'record.json', 'render.json', 'events.json', 'TRANSCRIPT.md', 'first-use.gif', 'first-use-uncut.gif',
                'poster.png')),
            *(f'scripts/readme-demo/{name}' for name in ('capture.py', 'render.py', 'verify.py'))])
        self.assertEqual(verifier.reviewed_demo(entry, {}), {
            'status': 'UNVERIFIED', 'reason': 'The Starport repository is unavailable.'})
        legacy = {key: value for key, value in entry.items() if key != 'manifest'}
        cases = {
            'no manifest': (legacy, 'E03: A README media review needs a relative manifest path.'),
            'asset directory': (entry | {'asset_directory': 'docs/assets/first-use-v1.2.0'}, 'E03: .*not an asset directory'),
            'nested entry': ({'kind': 'all', 'checks': [legacy]}, 'E03: .*manifest path'),
        }
        for name, (changed, reason) in cases.items():
            with self.subTest(name):
                broken = deepcopy(registry)
                broken['checks']['E03'] = changed
                with self.assertRaisesRegex(ValueError, reason):
                    verifier.validate_registry(broken, roster, checks)


if __name__ == '__main__':
    unittest.main()
