import hashlib
import io
import json
import subprocess
import tarfile
import tempfile
import unittest
import urllib.error
import urllib.request
from pathlib import Path
from unittest.mock import patch

import catalog_product_verify as verifier


SITE = 'https://starport.agentstation.ai'
RELEASE, EARLIER = 'v1.3.0', 'v1.2.0'
ARCHIVE = f'https://github.com/agentstation/starport/releases/download/{RELEASE}/starport-docs-{RELEASE}.tar.gz'
DESCRIBE = ['git', 'describe', '--tags', '--abbrev=0', '--match', 'v[0-9]*', '--exclude', 'v*-*', 'HEAD']
REVISION, EARLIER_REVISION, MODULE = 'a' * 64, 'b' * 64, 'v0.17.0'
CURRENT, OLDER = '1' * 40, '2' * 40
# Each served path maps to the address that the host serves it at.
PAGES = {'index.html': ('/', b'<home>'), 'docs/index.html': ('/docs', b'<docs>'),
         'docs/install.html': ('/docs/install', b'<install>'), 'docs/assets/site.css': ('/docs/assets/site.css', b'body{}')}
MANIFEST_ENTRY = {'kind': 'public_site_manifest', 'repository': 'starport', 'url': SITE}
# The tag that Cloudflare Web Analytics adds to an HTML response at the edge. The attribute values change per deploy.
BEACON = (b'<script type="module" src="https://static.cloudflareinsights.com/beacon.min.js/v31edd6df95cf4e85bb4c19e7'
          b'a9bdbcba1788362987495" integrity="sha512-iIg7k2xntmwu6/uSb5tpc/hySgZc4eoL31yB29W6tJFo2akwjPWcEqnCEdJvGexC'
          b'L0KEQwVYv5BlowfhVz26hg==" data-cf-beacon=\'{"version":"2024.11.0","token":"' + b'0' * 32
          + b'","r":1,"spa":2}\' crossorigin="anonymous"></script>')


def digest(data):
    return hashlib.sha256(data).hexdigest()


def observation(release, revision, generated_at):
    return {'starport_release': release, 'content_revision': revision, 'generated_at': generated_at}


class SiteFake(unittest.TestCase):
    """Fake git, gh, and HTTP across a Starport checkout, the public site, and the release assets."""

    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.base = Path(self.directory.name)
        self.consumer = self.base / 'starport'
        self.consumer.mkdir()
        self.tag = RELEASE
        self.served = {SITE + address: body for address, body in PAGES.values()}
        self.manifest = observation(RELEASE, REVISION, '2026-10-05T12:00:00Z') | {
            'starmap_module_version': MODULE, 'files': {name: digest(body) for name, (_, body) in PAGES.items()}}
        self.archived = observation(RELEASE, REVISION, '2026-10-05T11:00:00Z') | {'starmap_module_version': MODULE}
        self.failures = {}
        self.commits, self.runs = set(), {}
        self.requests = []

    def tearDown(self):
        self.directory.cleanup()

    def archive(self):
        output = io.BytesIO()
        with tarfile.open(fileobj=output, mode='w:gz') as archive:
            for name, data in ((f'starport-docs-{RELEASE}/manifest.json', json.dumps(self.archived).encode()),
                               (f'starport-docs-{RELEASE}/index.html', b'<home>')):
                member = tarfile.TarInfo(name)
                member.size = len(data)
                archive.addfile(member, io.BytesIO(data))
        return output.getvalue()

    def urlopen(self, request, timeout):
        self.assertEqual(timeout, 30)
        self.assertEqual(request.get_header('User-agent'), 'starmap-catalog-verify')
        url = request.full_url
        self.requests.append(url)
        if url in self.failures:
            raise self.failures[url]
        if url == SITE + '/docs/manifest.json':
            return io.BytesIO(json.dumps(self.manifest).encode())
        if url == ARCHIVE:
            return io.BytesIO(self.archive())
        if url in self.served:
            return io.BytesIO(self.served[url])
        raise urllib.error.HTTPError(url, 404, 'Not Found', {}, None)

    def command(self, args, **kwargs):
        if args == DESCRIBE:
            self.assertEqual(kwargs['cwd'], self.consumer)
            if self.tag is None:
                raise subprocess.CalledProcessError(128, args, '', 'fatal: No names found, cannot describe anything.')
            return subprocess.CompletedProcess(args, 0, self.tag + '\n', '')
        if args[:3] == ['git', '-C', str(self.consumer)]:
            self.assertEqual(args[3:5], ['cat-file', '-e'])
            if args[5].removesuffix('^{commit}') not in self.commits:
                raise subprocess.CalledProcessError(128, args, '', 'fatal: Not a valid object name')
            return subprocess.CompletedProcess(args, 0, '', '')
        if args[:3] == ['gh', 'run', 'view']:
            self.assertEqual(args[4:], ['-R', 'agentstation/starport', '--json', 'conclusion,event,workflowName,headSha'])
            if args[3] not in self.runs:
                raise subprocess.CalledProcessError(1, args, '', 'HTTP 502: Bad Gateway')
            return subprocess.CompletedProcess(args, 0, json.dumps(self.runs[args[3]]), '')
        raise AssertionError(args)

    def check(self, identity, entry):
        with (patch.object(verifier.subprocess, 'run', side_effect=self.command),
              patch.object(urllib.request, 'urlopen', side_effect=self.urlopen)):
            return verifier.run_check(identity, entry, {'starmap': verifier.ROOT, 'starport': self.consumer})


class PublicManifestTests(SiteFake):
    def run_check(self):
        return self.check('A29.public_url_content_manifest', MANIFEST_ENTRY)

    def test_served_release_archive_and_bytes_pass(self):
        self.assertEqual(self.run_check(), {
            'status': 'PASS', 'version': RELEASE, 'url': SITE, 'content_revision': REVISION,
            'starmap_module_version': MODULE, 'archive_sha256': digest(self.archive()), 'files': len(PAGES),
            'beacon_pages': 0})
        self.assertEqual(sorted(self.requests), sorted([SITE + '/docs/manifest.json', ARCHIVE, *self.served]))

    def test_one_beacon_tag_in_each_html_page_passes(self):
        for address in (SITE + '/', SITE + '/docs', SITE + '/docs/install'):
            self.served[address] = self.served[address].replace(b'>', b'>' + BEACON)
        result = self.run_check()
        self.assertEqual((result['status'], result['files'], result['beacon_pages']), ('PASS', len(PAGES), 3))

    def test_two_beacon_tags_in_one_page_fail(self):
        self.served[SITE + '/docs'] += BEACON + BEACON
        result = self.run_check()
        self.assertEqual((result['status'], result['mismatches']), ('FAIL', ['docs/index.html']))

    def test_beacon_tag_in_a_non_html_file_fails(self):
        self.served[SITE + '/docs/assets/site.css'] += BEACON
        result = self.run_check()
        self.assertEqual((result['status'], result['mismatches']), ('FAIL', ['docs/assets/site.css']))

    def test_beacon_tag_with_another_change_fails(self):
        self.served[SITE + '/'] = b'<changed>' + BEACON
        self.served[SITE + '/docs/install'] += b'<script src="https://static.cloudflareinsights.com/other.js"></script>'
        result = self.run_check()
        self.assertEqual((result['status'], result['mismatches']), ('FAIL', ['docs/install.html', 'index.html']))

    def test_checkout_without_a_stable_tag_is_unverified(self):
        self.tag = None
        self.assertEqual(self.run_check(), {
            'status': 'UNVERIFIED', 'reason': 'No stable release tag is an ancestor of the consumer checkout.'})
        self.assertEqual(self.requests, [])

    def test_unreachable_manifest_is_unverified(self):
        self.failures[SITE + '/docs/manifest.json'] = urllib.error.URLError('Connection refused')
        result = self.run_check()
        self.assertEqual(result['status'], 'UNVERIFIED')
        self.assertIn('Connection refused', result['reason'])

    def test_manifest_without_a_required_key_is_unverified(self):
        del self.manifest['generated_at']
        self.assertEqual(self.run_check()['reason'], 'The public manifest lacks a required key.')

    def test_different_served_release_fails(self):
        self.manifest['starport_release'] = EARLIER
        result = self.run_check()
        self.assertEqual((result['status'], result['reason'], result['served_release']),
                         ('FAIL', 'The public site serves a different Starport release.', EARLIER))
        self.assertNotIn(ARCHIVE, self.requests)

    def test_archive_with_a_different_revision_fails(self):
        self.archived['content_revision'] = EARLIER_REVISION
        result = self.run_check()
        self.assertEqual((result['status'], result['reason']), (
            'FAIL', 'The public site content differs from the released documentation archive in content_revision.'))
        self.assertEqual(result['archive_sha256'], digest(self.archive()))

    def test_unreachable_archive_is_unverified(self):
        self.failures[ARCHIVE] = urllib.error.HTTPError(ARCHIVE, 404, 'Not Found', {}, None)
        result = self.run_check()
        self.assertEqual(result['status'], 'UNVERIFIED')
        self.assertIn('404', result['reason'])

    def test_served_bytes_that_differ_from_the_manifest_fail(self):
        self.served[SITE + '/docs/install'] = b'<changed>'
        self.served[SITE + '/'] = b'<changed>'
        result = self.run_check()
        self.assertEqual((result['status'], result['reason'], result['mismatches']), (
            'FAIL', 'The public site serves bytes that its manifest does not list.', ['docs/install.html', 'index.html']))

    def test_unreachable_page_is_unverified(self):
        self.failures[SITE + '/docs'] = TimeoutError('timed out')
        self.assertEqual(self.run_check()['reason'], 'timed out')


class HostingRollbackTests(SiteFake):
    def setUp(self):
        super().setUp()
        self.proof = self.base / 'rollback' / 'rollback.json'
        self.proof.parent.mkdir()
        self.commits = {CURRENT, OLDER}
        self.steps = [
            {'ref': RELEASE, 'commit': CURRENT, 'deploy': {'method': 'wrangler', 'version_id': 'f3b1c2d4'},
             'capture': 'manifest-1.json', 'observed': observation(RELEASE, REVISION, '2026-10-05T12:00:00Z')},
            {'ref': OLDER, 'commit': OLDER, 'deploy': {'method': 'workflow_dispatch', 'run_id': '123'},
             'capture': 'manifest-2.json', 'observed': observation(EARLIER, EARLIER_REVISION, '2026-10-05T12:10:00Z')},
            {'ref': RELEASE, 'commit': CURRENT, 'deploy': {'method': 'workflow_dispatch', 'run_id': '124'},
             'capture': 'manifest-3.json', 'observed': observation(RELEASE, REVISION, '2026-10-05T12:20:00Z')}]
        self.runs = {
            '123': {'conclusion': 'success', 'event': 'workflow_dispatch', 'workflowName': 'Site', 'headSha': OLDER},
            '124': {'conclusion': 'success', 'event': 'workflow_dispatch', 'workflowName': 'Site', 'headSha': CURRENT}}
        self.manifest |= self.steps[-1]['observed']

    def run_check(self):
        for step in self.steps:
            capture = self.proof.parent / step['capture']
            if not capture.exists():
                capture.write_text(json.dumps(step['observed'] | {'starmap_module_version': MODULE, 'files': {}}))
        self.proof.write_text(json.dumps({'schema_version': 1, 'url': SITE, 'steps': self.steps}))
        return self.check('A29.hosting_rollback', {'kind': 'public_site_rollback', 'repository': 'starport',
                                                   'url': SITE, 'proof': str(self.proof)})

    def test_recorded_rollback_and_live_site_pass(self):
        self.assertEqual(self.run_check(), {
            'status': 'PASS', 'proof': str(self.proof), 'proof_sha256': digest(self.proof.read_bytes()), 'steps': 3,
            'scope': 'Recorded rollback exercise plus the live manifest. This invocation did not deploy.'})
        self.assertEqual(self.requests, [SITE + '/docs/manifest.json'])

    def test_record_with_fewer_than_three_steps_is_unverified(self):
        del self.steps[2]
        self.assertEqual(self.run_check(), {
            'status': 'UNVERIFIED', 'reason': 'The rollback record needs at least three steps.'})

    def test_commit_that_the_checkout_lacks_is_unverified(self):
        self.commits.discard(OLDER)
        self.assertEqual(self.run_check()['reason'], 'The rollback record names a commit that the consumer checkout lacks.')

    def test_capture_that_differs_from_its_observation_is_unverified(self):
        (self.proof.parent / 'manifest-2.json').write_text(json.dumps(observation(EARLIER, EARLIER_REVISION, 'later')))
        self.assertEqual(self.run_check(), {
            'status': 'UNVERIFIED', 'reason': 'A rollback capture differs from its recorded observation.'})

    def test_failed_dispatch_run_fails(self):
        self.runs['123']['conclusion'] = 'failure'
        self.assertEqual(self.run_check(), {
            'status': 'FAIL', 'reason': 'A recorded rollback deploy did not succeed as recorded.'})

    def test_unreachable_run_record_is_unverified(self):
        del self.runs['124']
        self.assertEqual(self.run_check()['status'], 'UNVERIFIED')

    def test_rollback_without_a_content_change_fails(self):
        self.steps[1]['observed'] = observation(RELEASE, REVISION, '2026-10-05T12:10:00Z')
        self.assertEqual(self.run_check(), {
            'status': 'FAIL', 'reason': 'The rollback record does not show a restored earlier site.'})

    def test_live_site_that_differs_from_the_last_deploy_fails(self):
        self.manifest['generated_at'] = '2026-10-05T13:00:00Z'
        self.assertEqual(self.run_check(), {
            'status': 'FAIL', 'reason': 'The public site no longer matches the last recorded deploy.'})


if __name__ == '__main__':
    unittest.main(verbosity=2)
