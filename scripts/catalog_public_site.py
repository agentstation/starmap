"""Verify that the public Starport site serves the released documentation and that a redeploy restores an earlier site."""

import hashlib
import http.client
import io
import json
import re
import subprocess
import tarfile
import urllib.parse
import urllib.request
import zlib
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path, PurePosixPath


ROOT = Path(__file__).resolve().parents[1]
REPOSITORY = "agentstation/starport"
ARCHIVE_URL = "https://github.com/" + REPOSITORY + "/releases/download/{tag}/starport-docs-{tag}.tar.gz"
MANIFEST_PATH = "/docs/manifest.json"
MANIFEST_KEYS = ("starport_release", "starmap_module_version", "content_revision", "generated_at")
OBSERVED_KEYS = ("starport_release", "content_revision", "generated_at")
SITE_URL = re.compile(r"https://[a-z0-9.-]+")
COMMIT = re.compile(r"[0-9a-f]{40}")
FETCH_TIMEOUT_SECONDS = 30
FETCH_WORKERS = 8
MISMATCH_LIMIT = 10
USER_AGENT = "starmap-catalog-verify"
ROLLBACK_SCOPE = "Recorded rollback exercise plus the live manifest. This invocation did not deploy."
# These errors prevent an observation. They do not show that the site is wrong.
ENVIRONMENT_ERRORS = (OSError, ValueError, KeyError, TypeError, EOFError, zlib.error, http.client.HTTPException,
                      tarfile.TarError, subprocess.SubprocessError)


def fetch_bytes(url, timeout):
    """Return the body of one public GET request. The opener follows redirects."""
    request = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return response.read()


def unverified(error):
    return {"status": "UNVERIFIED", "reason": str(error) or type(error).__name__}


def site_url(entry):
    """Return the registered site root, or None when the registry entry does not name one."""
    url = entry.get("url")
    return url if isinstance(url, str) and SITE_URL.fullmatch(url) else None


def public_manifest(url):
    """Return the manifest that the site serves after it has every required key and a file list."""
    manifest = json.loads(fetch_bytes(url + MANIFEST_PATH, FETCH_TIMEOUT_SECONDS))
    if not isinstance(manifest, dict) or any(key not in manifest for key in (*MANIFEST_KEYS, "files")):
        raise ValueError("The public manifest lacks a required key.")
    files = manifest["files"]
    if (any(not isinstance(manifest[key], str) for key in MANIFEST_KEYS) or not isinstance(files, dict) or not files
            or any(not isinstance(digest, str) for digest in files.values())):
        raise ValueError("The public manifest has an invalid value.")
    return manifest


def served_path(url, name):
    """Map a manifest path to its address. The host serves x.html at /x and dir/index.html at /dir."""
    parts = name.split("/")
    if any(part in ("", ".", "..") for part in parts):
        raise ValueError(f"The public manifest lists an invalid path: {name}")
    if parts[-1] == "index.html":
        parts.pop()
    elif parts[-1].endswith(".html"):
        parts[-1] = parts[-1][:-len(".html")]
    return url + "/" + urllib.parse.quote("/".join(parts))


def archive_manifest(tag):
    """Return the manifest of the released documentation archive and the sha256 of the archive bytes."""
    data = fetch_bytes(ARCHIVE_URL.format(tag=tag), FETCH_TIMEOUT_SECONDS)
    with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as archive:
        # The manifest is at the archive root or under one top-level directory.
        members = [member for member in archive.getmembers() if member.isfile()
                   and PurePosixPath(member.name).name == "manifest.json"
                   and len(PurePosixPath(member.name).parts) <= 2 and ".." not in PurePosixPath(member.name).parts]
        if len(members) != 1:
            raise ValueError("The documentation archive does not contain exactly one manifest.json.")
        manifest = json.loads(archive.extractfile(members[0]).read())
    if not isinstance(manifest, dict) or any(key not in manifest for key in ("content_revision", "starmap_module_version")):
        raise ValueError("The documentation archive manifest lacks a required key.")
    return manifest, hashlib.sha256(data).hexdigest()


def served_digest(address):
    return hashlib.sha256(fetch_bytes(address, FETCH_TIMEOUT_SECONDS)).hexdigest()


def served_mismatches(url, files):
    """Return up to MISMATCH_LIMIT paths whose served bytes differ from the manifest. The first transport error stops the scan."""
    addresses = {name: served_path(url, name) for name in files}
    differing = []
    with ThreadPoolExecutor(max_workers=FETCH_WORKERS) as pool:
        futures = {pool.submit(served_digest, address): name for name, address in addresses.items()}
        try:
            for future in as_completed(futures):
                if future.result() != files[futures[future]]:
                    differing.append(futures[future])
        except Exception:
            pool.shutdown(cancel_futures=True)
            raise
    return sorted(differing)[:MISMATCH_LIMIT]


def verify_manifest(entry, roots, release_version):
    """Prove that the public site serves the released documentation archive and the bytes that its manifest lists."""
    url, root = site_url(entry), roots.get(entry.get("repository"))
    if url is None:
        return {"status": "FAIL", "reason": "Invalid public site check."}
    if root is None:
        return {"status": "UNVERIFIED", "reason": "The consumer repository is unavailable."}
    try:
        version = release_version(root)
    except ENVIRONMENT_ERRORS as error:
        return unverified(error)
    if version is None:
        return {"status": "UNVERIFIED", "reason": "No stable release tag is an ancestor of the consumer checkout."}
    result = {"version": version, "url": url}
    try:
        manifest = public_manifest(url)
    except ENVIRONMENT_ERRORS as error:
        return result | unverified(error)
    result |= {"content_revision": manifest["content_revision"], "starmap_module_version": manifest["starmap_module_version"]}
    if manifest["starport_release"] != version:
        return result | {"status": "FAIL", "reason": "The public site serves a different Starport release.",
                         "served_release": manifest["starport_release"]}
    try:
        archived, result["archive_sha256"] = archive_manifest(version)
    except ENVIRONMENT_ERRORS as error:
        return result | unverified(error)
    for key in ("content_revision", "starmap_module_version"):
        if archived[key] != manifest[key]:
            return result | {"status": "FAIL",
                             "reason": f"The public site content differs from the released documentation archive in {key}."}
    try:
        mismatches = served_mismatches(url, manifest["files"])
    except ENVIRONMENT_ERRORS as error:
        return result | unverified(error)
    if mismatches:
        return result | {"status": "FAIL", "reason": "The public site serves bytes that its manifest does not list.",
                         "mismatches": mismatches}
    return result | {"status": "PASS", "files": len(manifest["files"])}


def valid_deploy(deploy):
    if not isinstance(deploy, dict):
        return False
    if deploy.get("method") == "wrangler":
        return isinstance(deploy.get("version_id"), str) and deploy["version_id"] != ""
    run_id = deploy.get("run_id")
    return deploy.get("method") == "workflow_dispatch" and (
        (type(run_id) is int and run_id > 0) or (isinstance(run_id, str) and re.fullmatch(r"[0-9]+", run_id) is not None))


def rollback_steps(record, url):
    """Return the recorded steps after the record matches schema version 1 and the registered site."""
    if not isinstance(record, dict) or record.get("schema_version") != 1:
        raise ValueError("The rollback record needs schema version 1.")
    if record.get("url") != url:
        raise ValueError("The rollback record names a different site.")
    steps = record.get("steps")
    if not isinstance(steps, list) or len(steps) < 3:
        raise ValueError("The rollback record needs at least three steps.")
    for step in steps:
        if (not isinstance(step, dict) or not isinstance(step.get("ref"), str) or not step["ref"]
                or not isinstance(step.get("commit"), str) or not isinstance(step.get("capture"), str)
                or not isinstance(step.get("observed"), dict)
                or any(not isinstance(step["observed"].get(key), str) for key in OBSERVED_KEYS)
                or not valid_deploy(step.get("deploy"))):
            raise ValueError("The rollback record has an invalid step.")
    return steps


def check_step(step, root, proof, checked_output):
    """Require the recorded commit in the consumer checkout and a capture that matches the recorded observation."""
    lacking = ValueError("The rollback record names a commit that the consumer checkout lacks.")
    if not COMMIT.fullmatch(step["commit"]):
        raise lacking
    try:
        checked_output(root, ["git", "-C", str(root), "cat-file", "-e", step["commit"] + "^{commit}"])
    except subprocess.CalledProcessError:
        raise lacking from None
    differs = ValueError("A rollback capture differs from its recorded observation.")
    try:
        path = (proof.parent / step["capture"]).resolve()
        if not path.is_relative_to(proof.parent.resolve()):
            raise differs
        capture = json.loads(path.read_bytes())
    except (OSError, ValueError):
        raise differs from None
    if not isinstance(capture, dict) or any(capture.get(key) != step["observed"][key] for key in OBSERVED_KEYS):
        raise differs


def dispatched_run(step, root, checked_output):
    """Return whether the recorded Site workflow run succeeded for the step commit."""
    run = json.loads(checked_output(root, ["gh", "run", "view", str(step["deploy"]["run_id"]), "-R", REPOSITORY,
                                           "--json", "conclusion,event,workflowName,headSha"]))
    if not isinstance(run, dict):
        raise ValueError("The workflow run record is not an object.")
    return ((run.get("conclusion"), run.get("event"), run.get("workflowName"), run.get("headSha"))
            == ("success", "workflow_dispatch", "Site", step["commit"]))


def verify_rollback(entry, roots, checked_output):
    """Prove from a recorded exercise and the live manifest that a redeploy of an earlier reference restores the earlier site."""
    url, root = site_url(entry), roots.get(entry.get("repository"))
    if url is None:
        return {"status": "FAIL", "reason": "Invalid public site check."}
    if root is None:
        return {"status": "UNVERIFIED", "reason": "The consumer repository is unavailable."}
    try:
        proof = ROOT / entry["proof"]
        data = proof.read_bytes()
        steps = rollback_steps(json.loads(data), url)
        for step in steps:
            check_step(step, root, proof, checked_output)
        dispatched = [dispatched_run(step, root, checked_output)
                      for step in steps if step["deploy"]["method"] == "workflow_dispatch"]
    except ENVIRONMENT_ERRORS as error:
        return unverified(error)
    if not all(dispatched):
        return {"status": "FAIL", "reason": "A recorded rollback deploy did not succeed as recorded."}
    observed = [step["observed"] for step in steps]
    served = [(item["content_revision"], item["starport_release"]) for item in observed]
    # Each deploy is a new build. The rollback changes the served content, and the roll forward restores it.
    if (any(left["generated_at"] == right["generated_at"] for left, right in zip(observed, observed[1:]))
            or served[0] == served[1] or served[0] != served[2]):
        return {"status": "FAIL", "reason": "The rollback record does not show a restored earlier site."}
    try:
        live = public_manifest(url)
    except ENVIRONMENT_ERRORS as error:
        return unverified(error)
    if any(live[key] != observed[-1][key] for key in OBSERVED_KEYS):
        return {"status": "FAIL", "reason": "The public site no longer matches the last recorded deploy."}
    return {"status": "PASS", "proof": str(proof), "proof_sha256": hashlib.sha256(data).hexdigest(),
            "steps": len(steps), "scope": ROLLBACK_SCOPE}
