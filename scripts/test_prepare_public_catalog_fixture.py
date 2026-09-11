#!/usr/bin/env python3
"""Check fixture integrity, cache reuse, and failed publication."""

import hashlib
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import prepare_public_catalog_fixture as fixture


class PublicCatalogFixtureTests(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory(prefix="public-catalog-fixture-test-")
        self.addCleanup(directory.cleanup)
        self.root = Path(directory.name)
        self.destination = self.root / fixture.ARCHIVE
        self.data = b"x" * fixture.SIZE
        capture = {
            "repository": "https://example.test/repository",
            "release": "immutable-release",
            "sha256": {fixture.ARCHIVE: hashlib.sha256(self.data).hexdigest()},
        }
        (self.root / "capture.json").write_text(json.dumps(capture))
        patch = mock.patch.object(fixture, "FIXTURE", self.root)
        patch.start()
        self.addCleanup(patch.stop)

    def test_valid_download_publishes_exact_bytes(self):
        with mock.patch.object(fixture.urllib.request, "urlopen", return_value=io.BytesIO(self.data)):
            self.assertEqual(fixture.prepare(self.destination), "downloaded-and-verified")
        self.assertEqual(self.destination.read_bytes(), self.data)

    def test_valid_cache_requires_no_network(self):
        self.destination.write_bytes(self.data)
        with mock.patch.object(fixture.urllib.request, "urlopen") as request:
            self.assertEqual(fixture.prepare(self.destination), "verified-cache")
            request.assert_not_called()

    def test_invalid_cache_is_preserved(self):
        corrupt = b"y" * fixture.SIZE
        self.destination.write_bytes(corrupt)
        with mock.patch.object(fixture.urllib.request, "urlopen") as request:
            with self.assertRaisesRegex(ValueError, "checksum"):
                fixture.prepare(self.destination)
            request.assert_not_called()
        self.assertEqual(self.destination.read_bytes(), corrupt)

    def test_short_download_never_publishes(self):
        self.assert_download_rejected(self.data[:-1], "size")

    def test_large_download_never_publishes(self):
        self.assert_download_rejected(self.data + b"x", "size")

    def test_corrupt_download_never_publishes(self):
        self.assert_download_rejected(b"y" * fixture.SIZE, "checksum")

    def test_failed_publication_removes_temporary_file(self):
        with mock.patch.object(fixture.urllib.request, "urlopen", return_value=io.BytesIO(self.data)):
            with mock.patch.object(fixture.os, "replace", side_effect=OSError("publication refused")):
                with self.assertRaisesRegex(OSError, "publication refused"):
                    fixture.prepare(self.destination)
        self.assertEqual([p.name for p in self.root.iterdir()], ["capture.json"])

    def assert_download_rejected(self, data, reason):
        with mock.patch.object(fixture.urllib.request, "urlopen", return_value=io.BytesIO(data)):
            with self.assertRaisesRegex(ValueError, reason):
                fixture.prepare(self.destination)
        self.assertFalse(self.destination.exists())


if __name__ == "__main__":
    unittest.main(verbosity=2)
