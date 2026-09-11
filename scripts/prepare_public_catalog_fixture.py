#!/usr/bin/env python3
"""Download and verify the immutable public catalog fixture before tests."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import tempfile
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
FIXTURE = ROOT / "runtime/testdata/public-catalog-20260908"
ARCHIVE = "starmap-catalog.tar.gz"
SIZE = 411974


def verify(data, expected):
    if len(data) != SIZE:
        raise ValueError("public catalog fixture size does not match the capture")
    if hashlib.sha256(data).hexdigest() != expected:
        raise ValueError("public catalog fixture checksum does not match the capture")


def prepare(destination):
    capture = json.loads((FIXTURE / "capture.json").read_text())
    expected = capture["sha256"][ARCHIVE]
    if destination.exists():
        with destination.open("rb") as stream:
            verify(stream.read(SIZE + 1), expected)
        return "verified-cache"
    url = f"{capture['repository']}/releases/download/{capture['release']}/{ARCHIVE}"
    request = urllib.request.Request(url, headers={"User-Agent": "starmap-test-fixture"})
    with urllib.request.urlopen(request, timeout=30) as stream:
        data = stream.read(SIZE + 1)
    verify(data, expected)
    destination.parent.mkdir(parents=True, exist_ok=True)
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(dir=destination.parent, delete=False) as stream:
            temporary = Path(stream.name)
            stream.write(data)
        os.replace(temporary, destination)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)
    return "downloaded-and-verified"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=FIXTURE / ARCHIVE)
    args = parser.parse_args()
    outcome = prepare(args.output)
    print(json.dumps({"outcome": outcome, "path": str(args.output), "size_bytes": SIZE}))


if __name__ == "__main__":
    main()
