import base64
import hashlib
import json
import subprocess
import tempfile
import unittest
import zipfile
from pathlib import Path
from unittest.mock import patch

import catalog_product_verify as verifier
from test_catalog_product_verify import PROMOTED, SELECTED, generation_files, write_generation


RELEASE, PREVIOUS, CONSUMER_RELEASE = 'v0.17.0', 'v0.16.5', 'v1.3.0'
DESCRIBE = ['git', 'describe', '--tags', '--abbrev=0', '--match', 'v[0-9]*', '--exclude', 'v*-*', 'HEAD']
TAG_COMMIT, TAG_TIME, TAG_MOMENT = '2b2944be736b1c2fb2907904f06b5f9c8fb6ecb7', '2026-10-05T18:33:39-05:00', '2026-10-05T23:33:39Z'
AT_TAG, AFTER_TAG = ('c55f31dd9' + 'a' * 31, '2026-10-05T18:27:53Z'), ('130a540d4' + 'b' * 31, '2026-10-06T05:10:46Z')


def starport_go_mod(version, replace=False):
    text = (f'module {verifier.STARPORT_MODULE}\n\ngo 1.27.1\n\n'
            f'require (\n\tgithub.com/rs/zerolog v1.34.0\n\t{verifier.STARMAP_MODULE} {version}\n)\n\n'
            'require golang.org/x/mod v0.30.0 // indirect\n')
    if replace:
        text += f'\nreplace {verifier.STARMAP_MODULE} => ../starmap\n'
    return text


class ModuleFake(unittest.TestCase):
    """Fake git and go commands across a Starmap source checkout, a Starport consumer, and the module proxy."""

    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.base = Path(self.directory.name)
        self.source, self.consumer = self.base / 'starmap', self.base / 'starport'
        write_generation(self.source / verifier.EMBEDDED_CATALOG, *SELECTED)
        self.consumer.mkdir()
        self.tags = {self.source: RELEASE, self.consumer: CONSUMER_RELEASE}
        self.modules, self.sums = {}, {}
        self.history, self.files = [], {}
        self.requirements = {'Require': [{'Path': verifier.STARMAP_MODULE, 'Version': RELEASE}]}

    def tearDown(self):
        self.directory.cleanup()

    def module(self, path, version, files):
        archive_path = self.base / f'{path.rsplit("/", 1)[-1]}@{version}.zip'
        with zipfile.ZipFile(archive_path, 'w') as archive:
            for name, data in files.items():
                archive.writestr(f'{path}@{version}/{name}', data)
        with zipfile.ZipFile(archive_path) as archive:
            self.sums[f'{path}@{version}'] = verifier.module_hash(archive)
        self.modules[f'{path}@{version}'] = archive_path
        return self.sums[f'{path}@{version}']

    def starmap_module(self, version, selection, **options):
        files = {'go.mod': f'module {verifier.STARMAP_MODULE}\n'}
        for name, data in generation_files(*selection, **options).items():
            files[f'{verifier.EMBEDDED_CATALOG}/{name}'] = data
        return self.module(verifier.STARMAP_MODULE, version, files)

    def go_sum(self, version, record):
        return f'{verifier.STARMAP_MODULE} {version} {record}\n{verifier.STARMAP_MODULE} {version}/go.mod h1:{"A" * 43}=\n'

    def command(self, args, **kwargs):
        cwd = kwargs['cwd']
        if args == DESCRIBE:
            tag = self.tags.get(cwd)
            if tag is None:
                raise subprocess.CalledProcessError(128, args, '', 'fatal: No names found, cannot describe anything.')
            return subprocess.CompletedProcess(args, 0, tag + '\n', '')
        if args[:2] == ['git', 'status']:
            return subprocess.CompletedProcess(args, 0, '', '')
        if args[:2] == ['git', 'log']:
            self.assertEqual((cwd, args), (self.consumer, ['git', 'log', '--format=%H', '-n', '200', '--', 'go.mod']))
            return subprocess.CompletedProcess(args, 0, ''.join(commit + '\n' for commit in self.history), '')
        if args[:2] == ['git', 'show']:
            self.assertEqual(cwd, self.consumer)
            if args[2] not in self.files:
                raise subprocess.CalledProcessError(128, args, '', 'fatal: path does not exist')
            return subprocess.CompletedProcess(args, 0, self.files[args[2]], '')
        if args[:3] == ['go', 'mod', 'edit']:
            self.assertEqual(cwd, self.consumer)
            return subprocess.CompletedProcess(args, 0, json.dumps(self.requirements), '')
        if args[:4] == ['go', 'mod', 'download', '-json']:
            self.assertNotIn(cwd, (self.source, self.consumer))
            self.assertEqual(list(cwd.iterdir()), [])
            if args[4] not in self.modules:
                raise subprocess.CalledProcessError(1, args, json.dumps({'Path': args[4], 'Error': 'unknown revision'}), '')
            return subprocess.CompletedProcess(args, 0, json.dumps(
                {'Path': args[4], 'Zip': str(self.modules[args[4]]), 'Sum': self.sums[args[4]]}), '')
        raise AssertionError(args)

    def check(self, identity, kind, repository):
        with patch.object(verifier.subprocess, 'run', side_effect=self.command):
            return verifier.run_check(identity, {'kind': kind, 'repository': repository},
                                      {'starmap': self.source, 'starport': self.consumer})


class ReleasedModuleTests(ModuleFake):
    """Fake the release tag tree and the catalog/v1 history that the GitHub API returns."""

    def setUp(self):
        super().setUp()
        # The adapter loads the publication capture from the repository under test.
        (self.source / 'scripts').symlink_to(Path(verifier.__file__).resolve().parent)
        self.starmap_module(RELEASE, SELECTED)
        self.tag_files = generation_files(*SELECTED)
        self.channels = [(*AT_TAG, 44, SELECTED)]
        self.attested = None

    def channel(self, sequence, selection):
        return json.dumps({'schema_version': 1, 'channel': 'catalog/v1', 'sequence': sequence,
                           'generation_id': selection[0], 'catalog_digest': selection[1]}).encode()

    def command(self, args, **kwargs):
        if kwargs['cwd'] != self.source or args == DESCRIBE:
            return super().command(args, **kwargs)
        if args == ['git', 'rev-parse', RELEASE + '^{commit}']:
            return subprocess.CompletedProcess(args, 0, TAG_COMMIT + '\n', '')
        if args == ['git', 'log', '-1', '--format=%cI', RELEASE + '^{commit}']:
            return subprocess.CompletedProcess(args, 0, TAG_TIME + '\n', '')
        if args[:2] == ['git', 'show']:
            self.assertIs(kwargs['text'], False)
            prefix = f'{RELEASE}:{verifier.EMBEDDED_CATALOG}/'
            self.assertTrue(args[2].startswith(prefix))
            return subprocess.CompletedProcess(args, 0, self.tag_files[args[2].removeprefix(prefix)], b'')
        if args[:2] == ['gh', 'api']:
            endpoint = args[2].removeprefix('repos/agentstation/starmap/')
            if endpoint.startswith('commits?'):
                self.assertEqual(endpoint, f'commits?sha=catalog%2Fv1&path=channel.json&until={TAG_MOMENT}&per_page=1')
                before = [{'sha': commit} for commit, time, _, _ in reversed(self.channels) if time <= TAG_MOMENT]
                return subprocess.CompletedProcess(args, 0, json.dumps(before[:1]), '')
            # Only a channel commit at or before the tag is readable. The channel head is not.
            ref = endpoint.removeprefix('contents/channel.json?ref=')
            raw = next(self.channel(sequence, selection) for commit, _, sequence, selection in self.channels if commit == ref)
            return subprocess.CompletedProcess(args, 0, json.dumps({'content': base64.b64encode(raw).decode()}), '')
        if args[:3] == ['gh', 'attestation', 'verify']:
            self.assertIn('--deny-self-hosted-runners', args)
            digest = self.attested or hashlib.sha256(Path(args[3]).read_bytes()).hexdigest()
            reports = [{'verificationResult': {'statement': {'subject': [{'digest': {'sha256': digest}}]}}}]
            return subprocess.CompletedProcess(args, 0, json.dumps(reports), '')
        raise AssertionError(args)

    def run_check(self):
        return self.check('A06.new_released_module', 'released_module', 'starmap')

    def test_released_module_that_carries_the_channel_at_its_tag_passes(self):
        result = self.run_check()
        identity = {'generation_id': SELECTED[0], 'semantic_checksum': SELECTED[1]}
        self.assertEqual(result, {
            'status': 'PASS', 'version': RELEASE, 'module': f'{verifier.STARMAP_MODULE}@{RELEASE}',
            'sum': self.sums[f'{verifier.STARMAP_MODULE}@{RELEASE}'], 'released': identity,
            'tag_commit': TAG_COMMIT, 'tag_committed_at': TAG_MOMENT, 'tag_embedded': identity,
            'channel_commit': AT_TAG[0], 'channel_sequence': 44, 'promoted': identity,
            'channel_sha256': 'sha256:' + hashlib.sha256(self.channel(44, SELECTED)).hexdigest()})

    def test_later_promotion_on_the_channel_head_passes(self):
        self.channels.append((*AFTER_TAG, 45, PROMOTED))
        result = self.run_check()
        self.assertEqual(result['status'], 'PASS')
        self.assertEqual((result['channel_commit'], result['channel_sequence']), (AT_TAG[0], 44))

    def test_checkout_that_moved_past_the_tag_passes(self):
        write_generation(self.source / verifier.EMBEDDED_CATALOG, *PROMOTED)
        self.assertEqual(self.run_check()['status'], 'PASS')

    def test_checkout_without_a_stable_ancestor_tag_is_unverified(self):
        for tag in ('v0.17.0-rc.1', 'v0.17', None):
            with self.subTest(tag=tag):
                self.tags[self.source] = tag
                self.assertEqual(self.run_check(), {
                    'status': 'UNVERIFIED', 'reason': 'No stable release tag is an ancestor of the checkout.'})

    def test_download_failure_is_unverified_with_the_go_error(self):
        self.modules.clear()
        result = self.run_check()
        self.assertEqual(result['status'], 'UNVERIFIED')
        self.assertIn('unknown revision', result['reason'])

    def test_module_bytes_that_differ_from_the_checksum_database_fail(self):
        self.sums[f'{verifier.STARMAP_MODULE}@{RELEASE}'] = 'h1:' + 'B' * 43 + '='
        self.assertEqual(self.run_check(), {
            'status': 'FAIL', 'reason': 'The released Starmap module bytes differ from the checksum database record.'})

    def test_released_generation_that_catalog_v1_had_not_promoted_at_its_tag_fails(self):
        # catalog/v1 promotes the released generation only after the tag.
        self.channels = [(*AT_TAG, 44, PROMOTED), (*AFTER_TAG, 45, SELECTED)]
        result = self.run_check()
        self.assertEqual(result['status'], 'FAIL')
        self.assertEqual(result['reason'], 'The released module embeds a generation that catalog/v1 had not promoted at its tag.')
        self.assertEqual(result['promoted']['generation_id'], PROMOTED[0])
        self.assertEqual(result['channel_commit'], AT_TAG[0])

    def test_tag_that_differs_from_the_released_module_fails(self):
        self.tag_files = generation_files(*PROMOTED)
        result = self.run_check()
        self.assertEqual(result['status'], 'FAIL')
        self.assertEqual(result['reason'], 'The release tag embeds a generation that the released module does not carry.')
        self.assertEqual(result['tag_embedded']['generation_id'], PROMOTED[0])

    def test_channel_without_a_promotion_before_the_tag_is_unverified(self):
        self.channels = [(*AFTER_TAG, 45, SELECTED)]
        self.assertEqual(self.run_check(), {
            'status': 'UNVERIFIED', 'reason': 'catalog/v1 has no promotion at or before the release tag.'})

    def test_unattested_channel_bytes_are_unverified(self):
        self.attested = 'f' * 64
        self.assertEqual(self.run_check(), {
            'status': 'UNVERIFIED', 'reason': 'No attestation binds the catalog/v1 channel bytes.'})

    def test_released_payload_that_contradicts_its_record_fails(self):
        self.starmap_module(RELEASE, SELECTED, declared='sha256:' + '0' * 64)
        self.assertEqual(self.run_check(), {
            'status': 'FAIL', 'reason': 'The embedded payload differs from its generation record.'})

    def test_tag_payload_that_contradicts_its_record_fails(self):
        self.tag_files = generation_files(*SELECTED, declared='sha256:' + '0' * 64)
        self.assertEqual(self.run_check(), {
            'status': 'FAIL', 'reason': 'The embedded payload differs from its generation record.'})


class ReleasedModulePinTests(ModuleFake):
    def setUp(self):
        super().setUp()
        self.record = self.starmap_module(RELEASE, SELECTED)
        self.release_consumer(starport_go_mod(RELEASE))

    def release_consumer(self, go_mod):
        self.consumer_sum = self.module(verifier.STARPORT_MODULE, CONSUMER_RELEASE, {'go.mod': go_mod})

    def run_check(self, record=None):
        (self.consumer / 'go.sum').write_text(self.go_sum(RELEASE, record or self.record))
        return self.check('A06.starport_released_module_pin', 'released_module_pin', 'starport')

    def test_released_consumer_that_pins_the_released_module_passes(self):
        self.assertEqual(self.run_check(), {
            'status': 'PASS', 'consumer_module': f'{verifier.STARPORT_MODULE}@{CONSUMER_RELEASE}',
            'consumer_sum': self.consumer_sum, 'module': f'{verifier.STARMAP_MODULE}@{RELEASE}',
            'go_sum': self.record, 'sum': self.record})

    def test_pseudo_version_pin_fails(self):
        self.requirements['Require'][0]['Version'] = 'v0.16.6-0.20261003000954-595e3c7ba959'
        result = self.run_check()
        self.assertEqual(result['status'], 'FAIL')
        self.assertEqual(result['reason'], 'The consumer does not pin a stable Starmap release.')

    def test_pin_that_differs_from_the_starmap_release_fails(self):
        self.requirements['Require'][0]['Version'] = PREVIOUS
        self.assertEqual(self.run_check(), {
            'status': 'FAIL', 'reason': 'The consumer pins a Starmap version that is not the released pair version.',
            'pinned_version': PREVIOUS, 'release_version': RELEASE})

    def test_go_sum_record_that_differs_from_the_module_bytes_fails(self):
        result = self.run_check(record='h1:' + 'B' * 43 + '=')
        self.assertEqual(result, {
            'status': 'FAIL', 'reason': 'The released Starmap module bytes differ from the consumer go.sum record.'})

    def test_module_bytes_that_differ_from_the_checksum_database_fail(self):
        self.sums[f'{verifier.STARMAP_MODULE}@{RELEASE}'] = 'h1:' + 'B' * 43 + '='
        self.assertEqual(self.run_check()['reason'],
                         'The released Starmap module bytes differ from the checksum database record.')

    def test_released_consumer_that_pins_another_version_fails(self):
        for go_mod in (starport_go_mod(PREVIOUS), starport_go_mod(RELEASE, replace=True),
                       f'module {verifier.STARPORT_MODULE}\n\nrequire github.com/rs/zerolog v1.34.0\n'):
            with self.subTest(go_mod=go_mod):
                self.release_consumer(go_mod)
                result = self.run_check()
                self.assertEqual(result['status'], 'FAIL')
                self.assertEqual(result['reason'], 'The released Starport module does not pin the released Starmap module.')

    def test_released_consumer_single_line_requirement_passes(self):
        self.release_consumer(f'module {verifier.STARPORT_MODULE}\n\nrequire {verifier.STARMAP_MODULE} {RELEASE}\n')
        self.assertEqual(self.run_check()['status'], 'PASS')

    def test_consumer_without_a_stable_ancestor_tag_is_unverified(self):
        self.tags[self.consumer] = None
        self.assertEqual(self.run_check(), {
            'status': 'UNVERIFIED', 'reason': 'No stable release tag is an ancestor of the consumer checkout.'})

    def test_replaced_pin_is_unverified(self):
        self.requirements['Replace'] = [{'Old': {'Path': verifier.STARMAP_MODULE}, 'New': {'Path': '../starmap'}}]
        self.assertEqual(self.run_check(), {'status': 'UNVERIFIED', 'reason': 'The consumer replaces the pinned Starmap module.'})

    def test_unreleased_consumer_module_is_unverified(self):
        del self.modules[f'{verifier.STARPORT_MODULE}@{CONSUMER_RELEASE}']
        result = self.run_check()
        self.assertEqual(result['status'], 'UNVERIFIED')
        self.assertIn('unknown revision', result['reason'])


class PinnedModuleHistoryTests(ModuleFake):
    def setUp(self):
        super().setUp()
        (self.consumer / 'go.sum').write_text(self.go_sum(RELEASE, self.starmap_module(RELEASE, SELECTED)))
        self.old_record = self.starmap_module(PREVIOUS, PROMOTED)
        self.history = ['c3', 'c2', 'c1']
        self.pin('c3', RELEASE, self.go_sum(RELEASE, self.sums[f'{verifier.STARMAP_MODULE}@{RELEASE}']))
        self.pin('c2', PREVIOUS, self.go_sum(PREVIOUS, self.old_record))
        self.pin('c1', 'v0.16.4', self.go_sum('v0.16.4', 'h1:' + 'C' * 43 + '='))

    def pin(self, commit, version, go_sum):
        self.files[f'{commit}:go.mod'] = starport_go_mod(version)
        self.files[f'{commit}:go.sum'] = go_sum

    def run_check(self):
        return self.check('A06.old_pinned_bytes_unchanged', 'pinned_module_baseline', 'starport')

    def test_previous_pin_with_an_older_generation_passes(self):
        result = self.run_check()
        self.assertEqual(result['status'], 'PASS')
        self.assertEqual(result['pinned'], result['selected'])
        self.assertEqual(result['previous'], {
            'module': f'{verifier.STARMAP_MODULE}@{PREVIOUS}', 'go_sum': self.old_record, 'commit': 'c2',
            'pinned': {'generation_id': PROMOTED[0], 'semantic_checksum': PROMOTED[1]}})
        self.assertIn('The current pin embeds the selected generation.', result['scope'])

    def test_history_without_an_earlier_pin_is_unverified(self):
        for history in ([], ['c3'], ['c3', 'c0']):
            with self.subTest(history=history):
                self.history = history
                self.files['c0:go.mod'] = f'module {verifier.STARPORT_MODULE}\n'
                result = self.run_check()
                self.assertEqual(result['status'], 'UNVERIFIED')
                self.assertEqual(result['reason'], 'The pinned module embeds the selected generation '
                                 'and the consumer history has no earlier pin to compare.')
                self.assertNotIn('previous', result)

    def test_previous_pin_without_a_go_sum_record_is_unverified(self):
        del self.files['c2:go.sum']
        self.assertIn('no earlier pin', self.run_check()['reason'])

    def test_previous_bytes_that_differ_from_the_old_go_sum_fail(self):
        self.files['c2:go.sum'] = self.go_sum(PREVIOUS, 'h1:' + 'B' * 43 + '=')
        self.assertEqual(self.run_check(), {
            'status': 'FAIL',
            'reason': 'The previous pinned Starmap module bytes differ from the consumer go.sum record at that pin.'})

    def test_previous_payload_that_contradicts_its_record_fails(self):
        self.files['c2:go.sum'] = self.go_sum(PREVIOUS, self.starmap_module(PREVIOUS, PROMOTED, declared='sha256:' + '0' * 64))
        self.assertEqual(self.run_check()['status'], 'FAIL')

    def test_previous_pin_with_the_selected_generation_is_unverified(self):
        self.files['c2:go.sum'] = self.go_sum(PREVIOUS, self.starmap_module(PREVIOUS, SELECTED))
        result = self.run_check()
        self.assertEqual(result['status'], 'UNVERIFIED')
        self.assertEqual(result['reason'], 'No earlier pinned generation differs from the selected generation.')
        self.assertEqual(result['previous']['commit'], 'c2')


class PublicSiteRegistrationTests(unittest.TestCase):
    def test_registry_binds_the_public_site_adapters(self):
        checks = verifier.read_json(verifier.REGISTRY)['checks']
        self.assertEqual({identity: checks[identity]['kind'] for identity in ('A29.public_url_content_manifest', 'A29.hosting_rollback')},
                         {'A29.public_url_content_manifest': 'public_site_manifest', 'A29.hosting_rollback': 'public_site_rollback'})


if __name__ == '__main__':
    unittest.main(verbosity=2)
