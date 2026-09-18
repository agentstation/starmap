"""Capture and recheck hosted publication evidence without publishing anything."""

import argparse
import base64
import hashlib
import json
import os
import re
import shutil
from pathlib import Path
import subprocess
import tempfile

from catalog_publication import validate_pending
from catalog_publication_hosted import REPOSITORY, release_assets, timestamp, validate_attestation, validate_channels, validate_promotion, validate_same_bytes_retry, validate_workflow
from native_catalog import unchanged_source


BEFORE_FILES = ("pending.json", "pull.json", "protection.json", "checks.json", "completion.json",
                "releases-before.json", "channels-before.json")
AFTER_FILES = ("retry.json", "releases-after.json", "channels-after.json", "channel-v1.raw.json", "channel-v2.raw.json")


def command(root, args):
    return subprocess.run(args, cwd=root, check=True, capture_output=True, text=True, timeout=300,
                          env=dict(os.environ, GOTOOLCHAIN="go1.27.1", GOWORK="off", GOFLAGS="")).stdout


def api(root, endpoint, pages=False):
    flags = ["--paginate", "--slurp"] if pages else []
    return json.loads(command(root, ["gh", "api", *flags, "repos/" + REPOSITORY + "/" + endpoint]))


def workflow(root, run, attempt):
    prefix = f"actions/runs/{run}/attempts/{attempt}"
    result = api(root, prefix)
    result["jobs"] = [job for page in api(root, prefix + "/jobs?per_page=100", True) for job in page["jobs"]]
    return result


def releases(root, record):
    return [api(root, "releases/tags/" + record[key]) for key in ("artifact_tag", "receipt_tag")]


def channels(root, include_bytes=False):
    result = {}
    originals = {}
    for version in ("v1", "v2"):
        encoded = api(root, "contents/channel.json?ref=catalog%2F" + version)
        raw = base64.b64decode(encoded["content"], validate=False).decode("utf-8")
        result["catalog/" + version] = json.loads(raw)
        originals["channel-" + version + ".raw.json"] = raw
    return (result, originals) if include_bytes else result


def write_capture(directory, source, documents):
    for name, value in documents.items():
        path = directory / name
        if path.exists():
            raise ValueError("Capture would replace existing evidence: " + name)
        path.write_text(json.dumps(value, indent=2) + "\n")
    names = [name for name in BEFORE_FILES + AFTER_FILES if (directory / name).exists()]
    digests = {name: hashlib.sha256((directory / name).read_bytes()).hexdigest() for name in names}
    (directory / "capture.json").write_text(json.dumps({"source_commit": source, "sha256": digests}, indent=2) + "\n")


def read_capture(directory, names):
    capture = json.loads((directory / "capture.json").read_text())
    documents = {}
    for name in names:
        raw = (directory / name).read_bytes()
        if hashlib.sha256(raw).hexdigest() != capture["sha256"][name]:
            raise ValueError("A captured publication file has changed: " + name)
        documents[name] = json.loads(raw)
    return capture, documents


def capture_before(root, directory, pending, pull_number, run, attempt):
    if directory.exists():
        raise ValueError("Select a new evidence directory.")
    record = validate_pending(json.loads(pending.read_text()))
    pull = api(root, f"pulls/{pull_number}")
    protection = api(root, "branches/main/protection")
    checks = api(root, "commits/" + pull["head"]["sha"] + "/check-runs?filter=all&per_page=100", True)
    promotion = validate_promotion(pull, protection, checks)
    source = command(root, ["git", "rev-parse", "HEAD"]).strip()
    unchanged_source(root, promotion["merge"])
    documents = {"pending.json": record, "pull.json": pull, "protection.json": protection, "checks.json": checks,
                 "completion.json": workflow(root, run, attempt), "releases-before.json": releases(root, record),
                 "channels-before.json": channels(root)}
    validate_channels(record, documents["channels-before.json"], promotion)
    validate_workflow(documents["completion.json"], run)
    release_assets(documents["releases-before.json"], record)
    directory.mkdir(parents=True)
    write_capture(directory, source, documents)


def capture_after(root, directory, run, attempt):
    capture, before = read_capture(directory, BEFORE_FILES)
    unchanged_source(root, capture["source_commit"])
    record = validate_pending(before["pending.json"])
    selected, originals = channels(root, include_bytes=True)
    documents = {"retry.json": workflow(root, run, attempt), "releases-after.json": releases(root, record),
                 "channels-after.json": selected} | originals
    validate_metadata(before | documents)
    write_capture(directory, capture["source_commit"], documents)


def validate_metadata(documents):
    record = validate_pending(documents["pending.json"])
    promotion = validate_promotion(documents["pull.json"], documents["protection.json"], documents["checks.json"])
    if timestamp(promotion["merged_at"]) > timestamp(documents["completion.json"]["updated_at"]):
        raise ValueError("The publication run completed before the checked merge.")
    retry = validate_same_bytes_retry(record, documents["releases-before.json"], documents["releases-after.json"],
                                     documents["completion.json"], documents["retry.json"])
    for phase in ("before", "after"):
        validate_channels(record, documents["channels-" + phase + ".json"], promotion)
    if documents["channels-before.json"] != documents["channels-after.json"]:
        raise ValueError("Retry changed an already completed discovery publication.")
    return record, promotion, retry


def verify(root, entry):
    """Verify captured checks, immutable bytes, attestations, and current embedding."""
    try:
        directory = root / entry["proof"]
        capture, documents = read_capture(directory, BEFORE_FILES + AFTER_FILES)
        unchanged_source(root, capture["source_commit"])
        record, promotion, retry = validate_metadata(documents)
        unchanged_source(root, promotion["merge"])
        with tempfile.TemporaryDirectory(prefix="starmap-publication-proof-") as temporary:
            assets = Path(temporary)
            for tag in (record["artifact_tag"], record["receipt_tag"]):
                command(root, ["gh", "release", "download", tag, "--repo", REPOSITORY, "--dir", str(assets)])
            digests = {asset["name"]: asset["digest"] for release in documents["releases-after.json"] for asset in release["assets"]}
            for name, digest in digests.items():
                if "sha256:" + hashlib.sha256((assets / name).read_bytes()).hexdigest() != digest:
                    raise ValueError("Downloaded asset differs from captured immutable bytes: " + name)
            subjects = [assets / name for name in ("starmap-catalog.tar.gz", "starmap-catalog-run.json", "starmap-catalog-state.json")]
            for version, channel in documents["channels-after.json"].items():
                raw = documents["channel-" + version.split("/")[1] + ".raw.json"].encode("utf-8")
                if json.loads(raw) != channel:
                    raise ValueError("Attested channel bytes differ from the captured discovery document.")
                path = assets / (version.replace("/", "-") + ".json")
                path.write_bytes(raw)
                subjects.append(path)
            for path in subjects:
                reports = json.loads(command(root, ["gh", "attestation", "verify", str(path), "--repo", REPOSITORY,
                               "--signer-workflow", REPOSITORY + "/.github/workflows/catalog-generation.yaml",
                               "--source-ref", "refs/heads/main", "--deny-self-hosted-runners", "--format", "json"]))
                if path.name.startswith("catalog-v"):
                    allowed = [(run["head_sha"], re.escape(f"https://github.com/{REPOSITORY}/actions/runs/{run['id']}/attempts/{run['run_attempt']}"))
                               for run in (documents["completion.json"], documents["retry.json"])]
                else:
                    allowed = [(record["preparation_commit"], re.escape(f"https://github.com/{REPOSITORY}/actions/runs/{record['workflow_run_id']}/attempts/") + r"[1-9][0-9]*")]
                validate_attestation(reports, "sha256:" + hashlib.sha256(path.read_bytes()).hexdigest(), allowed)
            restored = json.loads(command(root, ["go", "run", "./cmd/starmap-catalog-publish",
                "-restore-receipt", str(assets / "starmap-catalog-run.json"), "-restore-receipt-checksum", record["receipt_checksum"],
                "-state", str(assets / "starmap-catalog-state.json"), "-state-checksum", record["checkpoint_checksum"],
                "-publisher-id", "starmap-public", "-run-id", record["run_id"], "-output-dir", str(assets / "restored")]))
            for name in ("starmap-catalog.tar.gz", "starmap-catalog.tar.gz.sha256", "starmap-catalog.intoto.json"):
                restored_file = Path(restored["artifact_directory"]) / name
                if "sha256:" + hashlib.sha256(restored_file.read_bytes()).hexdigest() != digests[name]:
                    raise ValueError("Checkpoint recovery changed the published artifact: " + name)
            release = assets / "release"
            release.mkdir()
            for name in ("starmap-catalog.tar.gz", "starmap-catalog.tar.gz.sha256", "starmap-catalog.intoto.json"):
                shutil.copyfile(assets / name, release / name)
            embedding = json.loads(command(root, ["go", "run", "./cmd/starmap-catalog-release", "--verify-promotion-dir",
                           str(root / "internal/embedded/catalog"), "--promotion-release-dir", str(release)]))
            if (embedding["generation_id"] != record["generation_id"] or embedding["semantic_checksum"] != record["catalog_checksum"]
                    or embedding["archive_checksum"] != record["archive_checksum"]):
                raise ValueError("The current embedding differs from the admitted publication.")
        return {"status": "PASS", "promotion": promotion, "retry": retry, "attested_subjects": 5,
                "source_commit": capture["source_commit"], "catalog_checksum": record["catalog_checksum"]}
    except (OSError, ValueError, KeyError, TypeError, AttributeError, RuntimeError, subprocess.SubprocessError) as error:
        return {"status": "UNVERIFIED", "reason": str(error)}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("phase", choices=("before", "after"))
    parser.add_argument("--directory", required=True, type=Path)
    parser.add_argument("--run", required=True, type=int)
    parser.add_argument("--attempt", required=True, type=int)
    parser.add_argument("--pending", type=Path)
    parser.add_argument("--pull", type=int)
    args = parser.parse_args()
    repository = Path(__file__).resolve().parents[1]
    if args.phase == "before":
        if args.pending is None or args.pull is None:
            parser.error("before capture requires --pending and --pull")
        capture_before(repository, args.directory.resolve(), args.pending, args.pull, args.run, args.attempt)
    else:
        capture_after(repository, args.directory.resolve(), args.run, args.attempt)
