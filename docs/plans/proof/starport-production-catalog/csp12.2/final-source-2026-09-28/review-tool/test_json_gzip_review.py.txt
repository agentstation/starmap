"""Contract tests for committed JSON-gzip review inputs."""
from __future__ import annotations

import gzip
import json
import runpy
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

SCRIPT = Path(__file__).resolve().parents[1] / 'scripts' / 'autoreview'
H = runpy.run_path(str(SCRIPT), run_name='gzip_review_tests')


def git(repo, *args):
    return subprocess.check_output(['git', '-c', 'commit.gpgsign=false', *args], cwd=repo, text=True).strip()


class JsonGzipReviewTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name) / 'repo'
        self.repo.mkdir()
        git(self.repo, 'init', '-q')
        git(self.repo, 'config', 'user.name', 'Review Test')
        git(self.repo, 'config', 'user.email', 'review@example.invalid')
        self.path = self.repo / 'catalog.json.gz'
        self.write('{"model":"old","price":1.234567890123456789}')
        self.commit()
        self.base = git(self.repo, 'rev-parse', 'HEAD')

    def write(self, text, *, mtime=0):
        self.path.write_bytes(gzip.compress(text.encode(), mtime=mtime))

    def commit(self):
        git(self.repo, 'add', '-A')
        git(self.repo, 'commit', '-qm', 'fixture')

    def bundle(self):
        return H['branch_bundle'](self.repo, self.base)[0]

    def test_modification_preserves_precision_and_readable_diff(self):
        self.write('{"model":"new","price":1.234567890123456789}')
        self.commit()
        for bundle in (self.bundle(), H['commit_bundle'](self.repo, 'HEAD')[0]):
            self.assertIn('-  "model": "old",', bundle)
            self.assertIn('+  "model": "new",', bundle)
            self.assertIn('1.234567890123456789', bundle)
            self.assertIn('compressed objects:', bundle)
            self.assertNotIn('Binary files ', bundle)

    def test_added_and_deleted_blobs(self):
        self.path.unlink()
        new = self.repo / 'new.json.gz'
        new.write_bytes(gzip.compress(b'{"added":true}', mtime=0))
        self.commit()
        bundle = self.bundle()
        self.assertIn('-  "model": "old",', bundle)
        self.assertIn('+  "added": true', bundle)

    def test_root_commit(self):
        bundle = H['commit_bundle'](self.repo, 'HEAD')[0]
        self.assertIn('+  "model": "old",', bundle)

    def test_repacked_content_reports_representation_only(self):
        self.write('{"model":"old","price":1.234567890123456789}', mtime=8)
        self.commit()
        self.assertIn('Decoded JSON unchanged', self.bundle())

    def test_binary_rename_parsing_does_not_bypass_guard(self):
        H['require_no_binary_diff']('fixture', '-\t-\t\0old.json.gz\0new.json.gz\0', allowed_paths={'old.json.gz', 'new.json.gz'})
        with self.assertRaisesRegex(SystemExit, 'refusing binary changes'):
            H['require_no_binary_diff']('fixture', '-\t-\t\0old.bin\0new.bin\0')

    def test_rename_retains_metadata(self):
        self.path.rename(self.repo / 'renamed.json.gz')
        self.commit()
        self.assertIn('rename to renamed.json.gz', self.bundle())

    def test_cross_format_rename_refused(self):
        self.path.rename(self.repo / 'renamed.bin')
        self.commit()
        with self.assertRaisesRegex(SystemExit, 'renames across file formats'):
            self.bundle()

    def test_unrelated_binary_remains_refused(self):
        self.write('{"model":"new"}')
        (self.repo / 'image.bin').write_bytes(b'\0not-json')
        self.commit()
        with self.assertRaisesRegex(SystemExit, 'refusing binary changes'):
            self.bundle()

    def test_local_binary_review_remains_fail_closed(self):
        self.write('{"model":"new"}')
        with self.assertRaisesRegex(SystemExit, 'refusing binary changes'):
            H['local_bundle'](self.repo)

    def test_git_attributes_cannot_hide_json_content(self):
        (self.repo / '.gitattributes').write_text('*.json.gz -diff\n')
        self.write('{"model":"new"}')
        self.commit()
        self.assertIn('+  "model": "new"', self.bundle())

    def test_symlink_is_not_decoded(self):
        self.path.unlink()
        self.path.symlink_to('/outside/repo.json.gz')
        self.commit()
        with self.assertRaisesRegex(SystemExit, 'requires regular files'):
            self.bundle()

    def test_invalid_inputs_fail_closed(self):
        for payload in (b'not gzip', gzip.compress(b'not json'), gzip.compress(b'NaN'), gzip.compress(b'"\xff"'), gzip.compress(b'{}')[:-3], gzip.compress(b'{}') + b'bad trailer'):
            with self.subTest(payload=payload[:10]), self.assertRaises(SystemExit):
                H['decode_review_json_gzip'](payload)

    def test_optional_headers_and_concatenated_members_are_refused(self):
        member = gzip.compress(b'{}', mtime=0)
        comment = member[:3] + b'\x10' + member[4:10] + b'hidden-content\0' + member[10:]
        for payload in (comment, member + member):
            with self.subTest(payload=payload[:10]), self.assertRaises(SystemExit):
                H['decode_review_json_gzip'](payload)

    def test_rename_scan_retains_old_path_content(self):
        self.path.rename(self.repo / 'renamed.json.gz')
        self.commit()
        root = Path(self.temp.name) / 'scan'
        root.mkdir()
        H['prepare_trufflehog_history'](self.repo, 'branch', self.base, 'HEAD', root)
        self.assertIn('"model": "old"', git(root, 'show', 'HEAD:catalog.json.gz'))

    def test_size_limits_fail_without_truncation(self):
        decode = H['decode_review_json_gzip']
        for setting in ('MAX_JSON_GZIP_BYTES', 'MAX_JSON_DECODED_BYTES', 'MAX_JSON_RENDERED_BYTES'):
            with self.subTest(setting=setting), mock.patch.dict(decode.__globals__, {setting: 4}), self.assertRaisesRegex(SystemExit, 'exceeds the review limit'):
                decode(gzip.compress(b'{"long":"content"}'))

    def test_depth_limit(self):
        with self.assertRaisesRegex(SystemExit, 'nesting exceeds'):
            H['decode_review_json_gzip'](gzip.compress(('['*65 + '0' + ']'*65).encode()))

    def test_tokens_and_duplicate_keys_are_preserved(self):
        result = H['decode_review_json_gzip'](gzip.compress(b'{"key":1,"key":1e-999,"string":"\\u0041,\\\"["}'))
        self.assertEqual(result.count('"key":'), 2)
        self.assertIn('1e-999', result)
        self.assertIn('\\u0041,\\\"[', result)

    def test_known_secret_in_decoded_addition_is_refused(self):
        self.write(json.dumps({'api_key': 'A7f9K2m4Q8v6' + 'N3x5R1p0T9z8'}))
        self.commit()
        with self.assertRaisesRegex(SystemExit, 'secret'):
            self.bundle()

    def test_trufflehog_history_scans_decoded_additions_and_deletions(self):
        self.write('{"model":"new"}')
        self.commit()
        root = Path(self.temp.name) / 'scan'
        root.mkdir()
        H['prepare_trufflehog_history'](self.repo, 'branch', self.base, 'HEAD', root)
        log = git(root, 'log', '-p', '--format=')
        self.assertIn('+  "model": "new"', log)
        self.assertIn('+  "model": "old",', log)
        self.assertNotIn('Binary files ', log)

    def test_quoted_filename(self):
        self.path.rename(self.repo / 'odd "name.json.gz')
        self.commit()
        self.base = git(self.repo, 'rev-parse', 'HEAD')
        self.path = self.repo / 'odd "name.json.gz'
        self.write('{"model":"new"}')
        self.commit()
        self.assertIn('+  "model": "new"', self.bundle())


if __name__ == '__main__':
    unittest.main()
