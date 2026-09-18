#!/usr/bin/env python3
"""Coordinate public catalog preparation, promotion, and channel recovery."""

import argparse
from datetime import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile


ARCHIVE = "starmap-catalog.tar.gz"
RECEIPT = "starmap-catalog-run.json"
CHECKPOINT = "starmap-catalog-state.json"
ASSETS = (ARCHIVE, ARCHIVE + ".sha256", "starmap-catalog.intoto.json")
WORKFLOW = ".github/workflows/catalog-generation.yaml"
PENDING_BRANCH = "catalog/publication"
PROFILE = ".github/catalog-publication.yaml"
HEX = re.compile(r"[0-9a-f]{64}\Z")
COMMIT = re.compile(r"[0-9a-f]{40}\Z")
# Retain both models.dev transports at the catalog limit of 10,000 records each.
MAX_ACQUISITION_CORRECTIONS = 20000
MAX_CORRECTION_ID_LENGTH = 4096
REQUIRED_CHECKS = (
    "Security & Reliability", "Verification Gate", "Runtime ubuntu-24.04",
    "Runtime ubuntu-24.04-arm", "Runtime macos-15", "Runtime macos-15-intel",
    "Runtime windows-2025", "Runtime windows-11-arm",
)


class PublicationError(Exception):
    """Report a publication operation that did not complete."""


def command(args, *, cwd=None, env=None, input=None, check=True, timeout=3600):
    result = subprocess.run(
        [str(arg) for arg in args], cwd=cwd, env=env, input=input,
        text=True, capture_output=True, timeout=timeout, check=False,
    )
    if check and result.returncode:
        raise PublicationError(f"{args[0]} {args[1]} failed with exit status {result.returncode}")
    return result


def canonical(value):
    return (json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n").encode()


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(descriptor, "wb") as stream:
        stream.write(canonical(value))


def read_json(path, limit=1 << 20):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > limit:
        raise PublicationError("publication record must be a bounded regular file")
    with path.open("rb") as stream:
        data = stream.read(limit + 1)
    if len(data) > limit:
        raise PublicationError("publication record exceeds its byte limit")
    try:
        return json.loads(data, object_pairs_hook=unique_object)
    except (ValueError, UnicodeError) as error:
        raise PublicationError("publication record contains invalid JSON") from error


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise PublicationError("publication record contains a duplicate field")
        result[key] = value
    return result


def checksum(path, limit=256 << 20):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > limit:
        raise PublicationError("publication input must be a bounded regular file")
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1 << 20), b""):
            digest.update(block)
    return "sha256:" + digest.hexdigest()


def digest_hex(value):
    if not isinstance(value, str) or not value.startswith("sha256:") or not HEX.fullmatch(value[7:]):
        raise PublicationError("publication digest is invalid")
    return value[7:]


def validate_pending(record):
    fields = {
        "schema_version", "run_id", "workflow_run_id", "preparation_commit", "profile_checksum",
        "receipt_checksum", "checkpoint_checksum", "archive_checksum", "catalog_checksum",
        "generation_id", "artifact_tag", "receipt_tag",
    }
    if not isinstance(record, dict) or set(record) != fields or type(record["schema_version"]) is not int or record["schema_version"] != 1:
        raise PublicationError("pending publication has an unsupported schema")
    run = record["workflow_run_id"]
    if type(run) is not int or run < 1 or record["run_id"] != f"github-{run}":
        raise PublicationError("pending publication has an invalid run identity")
    if not isinstance(record["preparation_commit"], str) or not COMMIT.fullmatch(record["preparation_commit"]):
        raise PublicationError("pending publication has an invalid source commit")
    for key in ("profile_checksum", "receipt_checksum", "checkpoint_checksum", "archive_checksum", "catalog_checksum"):
        digest_hex(record[key])
    if record["artifact_tag"] != "catalog-" + digest_hex(record["catalog_checksum"]):
        raise PublicationError("pending artifact tag does not match its digest")
    if record["receipt_tag"] != "catalog-run-" + digest_hex(record["receipt_checksum"]):
        raise PublicationError("pending receipt tag does not match its digest")
    if not isinstance(record["generation_id"], str) or not record["generation_id"] or len(record["generation_id"]) > 4096:
        raise PublicationError("pending generation identity is invalid")
    return record


def completed(record, channels):
    modern = channels.get("catalog/v2", {}).get("document") or {}
    legacy = channels.get("catalog/v1", {}).get("document") or {}
    publication = modern.get("publication") or {}
    return (
        modern.get("tag") == legacy.get("tag") == record["artifact_tag"]
        and publication.get("receipt_tag") == record["receipt_tag"]
        and publication.get("receipt", {}).get("checksum") == record["receipt_checksum"]
        and publication.get("checkpoint", {}).get("checksum") == record["checkpoint_checksum"]
    )


class Publisher:
    def __init__(self, root, repository, run_id, *, source=None):
        self.root = Path(root).resolve()
        self.source = Path(source or Path(__file__).resolve().parents[1]).resolve()
        self.repository = repository
        self.run_id = int(run_id)
        self.root.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.stage = self.root / "stage"
        self.assets = self.root / "verified-assets"
        self.control = self.root / "control.json"
        self.publish_tool = self.root / "bin" / "publish"
        self.release_tool = self.root / "bin" / "release"

    def git(self, *args, **options):
        return command(["git", *args], cwd=self.source, **options)

    def gh(self, *args, **options):
        return command(["gh", *args, "--repo", self.repository], **options)

    def api(self, endpoint, *, missing=False, method="GET"):
        result = command(["gh", "api", "--method", method, f"repos/{self.repository}/{endpoint}"], check=False, timeout=120)
        if missing and result.returncode and "(HTTP 404)" in result.stderr:
            return None
        if result.returncode:
            raise PublicationError("GitHub API operation failed")
        return json.loads(result.stdout) if result.stdout.strip() else None

    def graphql(self, query, variables):
        args = ["gh", "api", "graphql", "-f", f"query={query}"]
        for name, value in variables.items():
            args.extend(("-f", f"{name}={value}"))
        result = command(args, check=False, timeout=120)
        if result.returncode:
            raise PublicationError("GitHub GraphQL operation failed")
        try:
            document = json.loads(result.stdout, object_pairs_hook=unique_object)
        except (ValueError, UnicodeError) as error:
            raise PublicationError("GitHub GraphQL response is invalid") from error
        if not isinstance(document, dict) or document.get("errors") or not isinstance(document.get("data"), dict):
            raise PublicationError("GitHub GraphQL response has no usable data")
        return document["data"]

    def find_release(self, tag):
        release = self.api(f"releases/tags/{tag}", missing=True)
        if release is not None:
            return release
        owner, name = self.repository.split("/", 1)
        data = self.graphql("""query($owner: String!, $name: String!, $tag: String!) {
            repository(owner: $owner, name: $name) {
                release(tagName: $tag) { databaseId tagName }
            }
        }""", {"owner": owner, "name": name, "tag": tag})
        repository = data.get("repository")
        if not isinstance(repository, dict) or "release" not in repository:
            raise PublicationError("release lookup did not identify the repository")
        selected = repository["release"]
        if selected is None:
            return None
        if (not isinstance(selected, dict) or type(selected.get("databaseId")) is not int
                or selected["databaseId"] < 1 or selected.get("tagName") != tag):
            raise PublicationError("release lookup returned an invalid identity")
        release = self.api(f"releases/{selected['databaseId']}")
        if (not isinstance(release, dict) or type(release.get("id")) is not int
                or release["id"] != selected["databaseId"] or release.get("tag_name") != tag):
            raise PublicationError("release response differs from its selected identity")
        return release

    def attest(self, path):
        self.gh("attestation", "verify", path, "--signer-workflow", f"{self.repository}/{WORKFLOW}",
                "--source-ref", "refs/heads/main", "--deny-self-hosted-runners")

    def outputs(self, **values):
        output = os.environ.get("GITHUB_OUTPUT")
        if not output:
            return
        with open(output, "a", encoding="utf-8") as stream:
            for key, value in values.items():
                value = str(value).lower() if isinstance(value, bool) else str(value)
                if "\n" in value or "\r" in value:
                    raise PublicationError("workflow output contains a line break")
                stream.write(f"{key}={value}\n")

    def build(self):
        self.publish_tool.parent.mkdir(exist_ok=True, mode=0o700)
        for package, target in (("starmap-catalog-publish", self.publish_tool), ("starmap-catalog-release", self.release_tool)):
            command(["go", "build", "-o", target, f"./cmd/{package}"], cwd=self.source)

    def read_branch(self, branch, filename):
        probe = self.git("ls-remote", "--exit-code", "origin", f"refs/heads/{branch}", check=False)
        if probe.returncode == 2:
            return {"commit": "", "document": None, "path": ""}
        if probe.returncode:
            raise PublicationError("cannot read the publication branch")
        self.git("fetch", "--no-tags", "--depth=1", "origin", f"+refs/heads/{branch}:refs/remotes/origin/{branch}")
        commit = self.git("rev-parse", f"refs/remotes/origin/{branch}").stdout.strip()
        path = self.root / "heads" / branch.replace("/", "-") / filename
        path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        result = self.git("show", f"{commit}:{filename}")
        path.write_text(result.stdout, encoding="utf-8")
        self.attest(path)
        return {"commit": commit, "document": read_json(path), "path": str(path)}

    def download(self, tag, files, directory):
        directory.mkdir(parents=True, exist_ok=True, mode=0o700)
        for filename in files:
            self.gh("release", "download", tag, "--pattern", filename, "--dir", directory, "--clobber")

    def verify_stage(self, record):
        validate_pending(record)
        for name, key in ((ARCHIVE, "archive_checksum"), (RECEIPT, "receipt_checksum"), (CHECKPOINT, "checkpoint_checksum")):
            path = self.stage / name
            if checksum(path) != record[key]:
                raise PublicationError("saved publication does not match its pending record")
            self.attest(path)
        self.restore_stage(record)

    def restore_stage(self, record):
        validate_pending(record)
        for name, key in ((ARCHIVE, "archive_checksum"), (RECEIPT, "receipt_checksum"), (CHECKPOINT, "checkpoint_checksum")):
            if checksum(self.stage / name) != record[key]:
                raise PublicationError("saved publication does not match its pending record")
        report = json.loads(command([self.publish_tool, "-restore-receipt", self.stage / RECEIPT,
            "-restore-receipt-checksum", record["receipt_checksum"], "-state", self.stage / CHECKPOINT,
            "-state-checksum", record["checkpoint_checksum"], "-publisher-id", "starmap-public",
            "-run-id", record["run_id"], "-output-dir", self.root / "restored"]).stdout)
        if report["generation_id"] != record["generation_id"] or report["archive_checksum"] != record["archive_checksum"]:
            raise PublicationError("restored publication does not match its artifact")
        self.assets.mkdir(exist_ok=True, mode=0o700)
        for name in ASSETS:
            original = self.stage / name
            if checksum(original) != checksum(Path(report["artifact_directory"]) / name):
                raise PublicationError("saved release asset differs from the restored publication")
            shutil.copyfile(original, self.assets / name)
        verified = json.loads(command([self.release_tool, "--verify-dir", self.assets]).stdout)
        if verified["semantic_checksum"] != record["catalog_checksum"]:
            raise PublicationError("restored catalog identity does not match its record")

    def recover(self, record):
        validate_pending(record)
        release = self.api(f"releases/tags/{record['receipt_tag']}", missing=True)
        available = {asset["name"] for asset in (release or {}).get("assets", []) if asset.get("state") == "uploaded"}
        if {RECEIPT, CHECKPOINT}.issubset(available):
            self.download(record["receipt_tag"], (RECEIPT, CHECKPOINT), self.stage)
            for filename, key in ((RECEIPT, "receipt_checksum"), (CHECKPOINT, "checkpoint_checksum")):
                if checksum(self.stage / filename) != record[key]:
                    raise PublicationError("published recovery input has the wrong digest")
                self.attest(self.stage / filename)
            restored = json.loads(command([self.publish_tool, "-restore-receipt", self.stage / RECEIPT,
                "-restore-receipt-checksum", record["receipt_checksum"], "-state", self.stage / CHECKPOINT,
                "-state-checksum", record["checkpoint_checksum"], "-publisher-id", "starmap-public",
                "-run-id", record["run_id"], "-output-dir", self.root / "restored"]).stdout)
            for filename in ASSETS:
                shutil.copyfile(Path(restored["artifact_directory"]) / filename, self.stage / filename)
        else:
            self.gh("run", "download", str(record["workflow_run_id"]), "--name",
                    f"catalog-publication-{record['workflow_run_id']}", "--dir", self.stage)
            if read_json(self.stage / "pending.json") != record:
                raise PublicationError("workflow recovery selected a different pending publication")
        self.verify_stage(record)

    def rejected(self, record, channels):
        path = self.source / ".github/catalog-rejections.json"
        if not path.exists():
            return False
        entries = read_json(path)
        if not isinstance(entries, list):
            raise PublicationError("catalog rejection registry must be a list")
        selected = None
        seen = set()
        for entry in entries:
            if not isinstance(entry, dict) or set(entry) != {"pending", "pull_request", "reason"}:
                raise PublicationError("catalog rejection has an unsupported schema")
            pending = validate_pending(entry["pending"])
            if type(entry["pull_request"]) is not int or entry["pull_request"] < 1:
                raise PublicationError("catalog rejection requires a pull request")
            if not isinstance(entry["reason"], str) or not entry["reason"].strip() or len(entry["reason"]) > 4096:
                raise PublicationError("catalog rejection requires a bounded reason")
            digest = pending["receipt_checksum"]
            if digest in seen:
                raise PublicationError("duplicate catalog rejection")
            seen.add(digest)
            if digest == record["receipt_checksum"]:
                selected = entry
        if selected is None:
            return False
        if selected["pending"] != record:
            raise PublicationError("rejected candidate differs from its reviewed record")
        if any((state["document"] or {}).get("tag") == record["artifact_tag"] for state in channels.values()):
            raise PublicationError("cannot reject a candidate already accepted by a channel")
        retained = self.root / "rejected"
        self.download(record["artifact_tag"], (ARCHIVE,), retained)
        self.download(record["receipt_tag"], (RECEIPT, CHECKPOINT), retained)
        for filename, key in ((ARCHIVE, "archive_checksum"), (RECEIPT, "receipt_checksum"), (CHECKPOINT, "checkpoint_checksum")):
            path = retained / filename
            if checksum(path) != record[key]:
                raise PublicationError("rejected evidence has the wrong digest")
            self.attest(path)
        self.outputs(rejected_receipt=record["receipt_checksum"], rejected_pull_request=selected["pull_request"])
        return True

    def inspect(self):
        self.build()
        channels = {name: self.read_branch(name, "channel.json") for name in ("catalog/v1", "catalog/v2")}
        for name, state in channels.items():
            if state["document"] and state["document"].get("channel") != name:
                raise PublicationError("channel branch contains a different channel")
        pending = self.read_branch(PENDING_BRANCH, "pending.json")
        record = validate_pending(pending["document"]) if pending["document"] else None
        active = record is not None and not completed(record, channels)
        if active and self.rejected(record, channels):
            active = False
        event = os.environ.get("GITHUB_EVENT_NAME", "workflow_dispatch")
        acquire = not active and event != "workflow_run"
        if not active and acquire:
            artifacts = self.api(f"actions/runs/{self.run_id}/artifacts?per_page=100")
            matches = [item for item in artifacts["artifacts"] if item["name"] == f"catalog-publication-{self.run_id}"]
            if len(matches) > 1:
                raise PublicationError("run has multiple retained publication inputs")
            if matches:
                if matches[0]["expired"]:
                    raise PublicationError("retained preparation expired before publication")
                self.gh("run", "download", str(self.run_id), "--name", f"catalog-publication-{self.run_id}", "--dir", self.stage)
                self.attest(self.stage / "pending.json")
                record = validate_pending(read_json(self.stage / "pending.json"))
                if record["workflow_run_id"] != self.run_id:
                    raise PublicationError("retained preparation belongs to another run")
                active, acquire = True, False
        if active:
            if (self.stage / "pending.json").exists():
                self.verify_stage(record)
            else:
                self.recover(record)
        control = {"channels": channels, "pending_head": pending["commit"], "pending": record,
                   "active": active or acquire, "acquire": acquire}
        write_json(self.control, control)
        self.outputs(active=control["active"], acquire=acquire, directory=self.stage)

    def prepare(self):
        control = read_json(self.control)
        if not control["acquire"]:
            return
        args = [self.publish_tool, "-baseline-embedded", "-profile", self.source / PROFILE,
                "-publisher-id", "starmap-public", "-run-id", f"github-{self.run_id}",
                "-output-dir", self.root / "prepared"]
        modern = control["channels"]["catalog/v2"]["document"]
        if modern:
            publication = modern["publication"]
            tag = publication["receipt_tag"]
            if tag != "catalog-run-" + digest_hex(publication["receipt"]["checksum"]):
                raise PublicationError("accepted channel has an invalid receipt tag")
            accepted = self.root / "accepted"
            self.download(tag, (CHECKPOINT,), accepted)
            self.attest(accepted / CHECKPOINT)
            if checksum(accepted / CHECKPOINT) != publication["checkpoint"]["checksum"]:
                raise PublicationError("accepted checkpoint does not match the channel")
            args += ["-state", accepted / CHECKPOINT, "-state-checksum", publication["checkpoint"]["checksum"]]
        try:
            acquired = command(args, timeout=75 * 60, check=False)
        except subprocess.TimeoutExpired as error:
            self.retain_acquisition_corrections(error.stderr, "timed_out")
            raise PublicationError("catalog acquisition exceeded its time limit") from error
        self.retain_acquisition_corrections(acquired.stderr, "failed" if acquired.returncode else "succeeded")
        if acquired.returncode:
            raise PublicationError(f"catalog acquisition failed with exit status {acquired.returncode}")
        report = json.loads(acquired.stdout)
        self.stage.mkdir(exist_ok=True, mode=0o700)
        for filename in ASSETS:
            shutil.copyfile(Path(report["artifact_directory"]) / filename, self.stage / filename)
        shutil.copyfile(report["receipt_path"], self.stage / RECEIPT)
        self.retain_source_status()
        shutil.copyfile(report["state_path"], self.stage / CHECKPOINT)
        verified = json.loads(command([self.release_tool, "--verify-dir", report["artifact_directory"]]).stdout)
        record = {"schema_version": 1, "run_id": f"github-{self.run_id}", "workflow_run_id": self.run_id,
            "preparation_commit": self.git("rev-parse", "HEAD").stdout.strip(), "profile_checksum": checksum(self.source / PROFILE),
            "receipt_checksum": report["receipt_checksum"], "checkpoint_checksum": report["state_checksum"],
            "archive_checksum": report["archive_checksum"], "catalog_checksum": verified["semantic_checksum"],
            "generation_id": report["generation_id"], "artifact_tag": "catalog-" + digest_hex(verified["semantic_checksum"]),
            "receipt_tag": "catalog-run-" + digest_hex(report["receipt_checksum"])}
        write_json(self.stage / "pending.json", validate_pending(record))
        control["pending"] = record
        write_json(self.control, control)

    def retain_source_status(self):
        receipt = read_json(self.stage / RECEIPT)
        completed = datetime.fromisoformat(receipt["completed_at"])
        statuses = []
        for source in receipt["sources"]:
            binding = source["policy"].get("binding") or {}
            observation = source.get("observation")
            observed = observation["observed_at"] if observation else None
            statuses.append({"source": source["policy"]["source"],
                "binding_id": binding.get("id"), "provider_id": binding.get("provider_id"), "attempt": source["attempt"],
                "evidence_kind": source["evidence_kind"], "observed_at": observed,
                "age_seconds": (completed - datetime.fromisoformat(observed)).total_seconds() if observed else None,
                "stale": source["evidence_kind"] == "stale_retained"})
        write_json(self.root / "source-status.log", {"schema_version": 1, "run_id": f"github-{self.run_id}", "sources": statuses})
        summary = os.environ.get("GITHUB_STEP_SUMMARY")
        if summary:
            stale = [status for status in statuses if status["stale"]]
            oldest = max((status["age_seconds"] for status in stale), default=0)
            with open(summary, "a", encoding="utf-8") as stream:
                stream.write(f"### Catalog source quality\n\nStale sources: {len(stale)}. Oldest stale evidence: {oldest:g} seconds.\n\n"
                             "The catalog-validation artifact contains source identities, outcomes, and ages in `source-status.log`.\n\n")

    def retain_acquisition_corrections(self, stderr, process_status):
        if isinstance(stderr, bytes):
            stderr = stderr.decode("utf-8", errors="replace")
        corrections = []
        count, invalid = 0, 0
        for line in (stderr or "").splitlines():
            try:
                event = json.loads(line, object_pairs_hook=unique_object)
            except (ValueError, PublicationError):
                continue
            if not isinstance(event, dict) or event.get("code") != "display_name_whitespace_trimmed":
                continue
            if event.get("source") not in ("models_dev_http", "models_dev_git"):
                continue
            identities = (event.get("provider_id"), event.get("model_id"))
            if any(not isinstance(value, str) or not value or len(value) > MAX_CORRECTION_ID_LENGTH
                   or not value.isprintable() for value in identities):
                invalid += 1
                continue
            count += 1
            if len(corrections) < MAX_ACQUISITION_CORRECTIONS:
                corrections.append({key: event[key] for key in ("source", "provider_id", "model_id", "code")})
        write_json(self.root / "acquisition-corrections.log", {
            "schema_version": 1, "run_id": f"github-{self.run_id}", "process_status": process_status,
            "correction_count": count, "omitted_count": count - len(corrections),
            "invalid_count": invalid, "corrections": corrections,
        })
        summary = os.environ.get("GITHUB_STEP_SUMMARY")
        if summary:
            with open(summary, "a", encoding="utf-8") as stream:
                stream.write(f"### Catalog acquisition\n\nProcess: {process_status}. Correction events: {count}. "
                             f"Omitted events: {count - len(corrections)}. Invalid events: {invalid}.\n\n"
                             "The catalog-validation artifact contains `acquisition-corrections.log`.\n\n")

    def stage_promotion(self, staged):
        args = [self.release_tool, "--stage-promotion-dir", staged, "--promotion-release-dir", self.assets]
        log = self.root / "promotion-staging.log"
        try:
            result = command(args, check=False)
        except subprocess.TimeoutExpired as error:
            output = []
            for value in (error.stdout, error.stderr):
                output.append(value.decode("utf-8", errors="replace") if isinstance(value, bytes) else value or "")
            log.write_text("".join(output), encoding="utf-8")
            raise PublicationError("promotion staging exceeded its time limit; inspect its retained validation log") from error
        log.write_text(result.stdout + result.stderr, encoding="utf-8")
        if result.returncode:
            raise PublicationError(f"promotion staging failed with exit status {result.returncode}; inspect its retained validation log")

    def validate(self):
        record = validate_pending(read_json(self.control)["pending"])
        if record["preparation_commit"] != self.git("rev-parse", "HEAD").stdout.strip():
            raise PublicationError("new preparation does not match the trusted workflow source")
        self.restore_stage(record)
        checkout = self.promotion_checkout(record["preparation_commit"], "candidate-validation")
        staged = self.root / "validation-catalog"
        self.stage_promotion(staged)
        target = checkout / "internal/embedded/catalog"
        shutil.rmtree(target)
        shutil.copytree(staged, target)
        for target in ("catalog-generation-check", "embedded-catalog-budget-check"):
            result = command(["make", target], cwd=checkout, check=False, timeout=30 * 60)
            log = self.root / (target + ".log")
            log.write_text(result.stdout + result.stderr, encoding="utf-8")
            if result.returncode:
                raise PublicationError(f"candidate {target} failed; inspect its retained validation log")

    def push_document(self, branch, filename, document, parent):
        self.attest(document)
        with tempfile.TemporaryDirectory(prefix="catalog-index-", dir=self.root) as temporary:
            env = dict(os.environ, GIT_INDEX_FILE=str(Path(temporary) / "index"),
                       GIT_AUTHOR_NAME="github-actions[bot]", GIT_COMMITTER_NAME="github-actions[bot]",
                       GIT_AUTHOR_EMAIL="41898282+github-actions[bot]@users.noreply.github.com",
                       GIT_COMMITTER_EMAIL="41898282+github-actions[bot]@users.noreply.github.com")
            self.git("read-tree", parent or "--empty", env=env)
            blob = self.git("hash-object", "-w", document).stdout.strip()
            self.git("update-index", "--add", "--cacheinfo", f"100644,{blob},{filename}", env=env)
            tree = self.git("write-tree", env=env).stdout.strip()
            args = ["commit-tree", tree, "-m", f"catalog: update {branch}"]
            if parent:
                args += ["-p", parent]
            commit = self.git(*args, env=env).stdout.strip()
            self.git("-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential",
                     "push", "origin", f"{commit}:refs/heads/{branch}")
            return commit

    def begin(self):
        control = read_json(self.control)
        record = validate_pending(control["pending"])
        self.verify_stage(record)
        path = self.stage / "pending.json"
        if not path.exists():
            write_json(path, record)
        if read_json(path) != record:
            raise PublicationError("pending preparation changed before persistence")
        current = self.read_branch(PENDING_BRANCH, "pending.json")
        if current["document"] == record:
            return
        if current["commit"] != control["pending_head"]:
            raise PublicationError("another publication changed the pending branch")
        self.push_document(PENDING_BRANCH, "pending.json", path, current["commit"])

    def publish_release(self, tag, filenames, record):
        release = self.find_release(tag)
        if release is None:
            notes = self.root / "release-notes.md"
            notes.write_text(f"Public catalog publication {record['run_id']}.\nCatalog digest: {record['catalog_checksum']}.\n", encoding="utf-8")
            self.gh("release", "create", tag, "--draft", "--prerelease", "--target", record["preparation_commit"],
                    "--title", tag, "--notes-file", notes)
            release = self.find_release(tag)
            if release is None:
                raise PublicationError("created draft release is not yet visible")
        existing = {asset["name"] for asset in release["assets"] if asset.get("state") == "uploaded"}
        for asset in release["assets"]:
            if release["draft"] and asset["name"] in filenames and asset.get("state") == "starter":
                self.api(f"releases/assets/{int(asset['id'])}", method="DELETE")
        for filename in filenames:
            if filename not in existing:
                if not release["draft"]:
                    raise PublicationError("published immutable release is missing an asset")
                self.gh("release", "upload", tag, self.stage / filename)
        downloaded = self.root / "public" / tag
        self.download(tag, filenames, downloaded)
        for filename in filenames:
            if checksum(downloaded / filename) != checksum(self.stage / filename):
                raise PublicationError("downloaded release differs from the admitted input")
        if release["draft"]:
            self.gh("release", "edit", tag, "--draft=false")
        self.download(tag, filenames, downloaded)
        for filename in filenames:
            if checksum(downloaded / filename) != checksum(self.stage / filename):
                raise PublicationError("public release differs from verified staged input")
        return self.api(f"releases/tags/{tag}")

    def publish(self):
        record = validate_pending(read_json(self.control)["pending"])
        self.verify_stage(record)
        self.publish_release(record["receipt_tag"], (RECEIPT, CHECKPOINT), record)
        release = self.publish_release(record["artifact_tag"], ASSETS, record)
        self.attest(self.root / "public" / record["artifact_tag"] / ARCHIVE)
        self.outputs(published_at=release["published_at"])

    def promotion_checkout(self, commit, name):
        if not COMMIT.fullmatch(commit):
            raise PublicationError("promotion source commit is invalid")
        path = self.root / name
        if path.exists():
            head = command(["git", "rev-parse", "HEAD"], cwd=path).stdout.strip()
            dirty = command(["git", "status", "--porcelain"], cwd=path).stdout
            if head != commit or dirty:
                raise PublicationError("existing promotion checkout does not match its verified commit")
        else:
            self.git("worktree", "add", "--detach", path, commit)
        return path

    def verify_promotion_head(self, head, main, name):
        common = self.git("merge-base", main, head).stdout.strip()
        changed = self.git("diff", "--name-only", "-z", common, head).stdout.split("\0")
        if any(path and not path.startswith("internal/embedded/catalog/") for path in changed):
            raise PublicationError("promotion branch changes files outside the embedded catalog")
        checkout = self.promotion_checkout(head, name)
        if not self.verify_embedding(checkout):
            raise PublicationError("promotion head differs from its admitted artifact")
        return checkout

    def checks_pass(self, head):
        result = command(["gh", "api", "--paginate", "--slurp",
                          f"repos/{self.repository}/commits/{head}/check-runs?per_page=100"], timeout=120)
        checks = {}
        for page in json.loads(result.stdout):
            for item in page["check_runs"]:
                if item["head_sha"] == head and item["app"]["id"] == 15368:
                    prior = checks.get(item["name"])
                    if prior is None or item["id"] > prior["id"]:
                        checks[item["name"]] = item
        return all(checks.get(name, {}).get("conclusion") == "success"
                   and checks[name]["status"] == "completed" for name in REQUIRED_CHECKS)

    def verify_embedding(self, checkout):
        return command([self.release_tool, "--verify-promotion-dir", checkout / "internal/embedded/catalog",
                        "--promotion-release-dir", self.assets], check=False).returncode == 0

    def promotion_rules(self):
        rules = self.api("branches/main/protection")
        required = rules.get("required_status_checks") or {}
        checks = {(entry["context"], entry["app_id"]) for entry in required.get("checks", [])}
        if not required.get("strict") or not {(name, 15368) for name in REQUIRED_CHECKS[:2]}.issubset(checks):
            raise PublicationError("promotion requires strict checks from the GitHub Actions app on main")

    def promote(self):
        control = read_json(self.control)
        record = validate_pending(control["pending"])
        self.verify_stage(record)
        self.promotion_rules()
        self.git("fetch", "--no-tags", "origin", "main")
        main = self.git("rev-parse", "FETCH_HEAD").stdout.strip()
        checkout = self.promotion_checkout(main, "promotion")
        branch = "catalog/promotion/" + digest_hex(record["receipt_checksum"])
        fields = "number,state,headRefOid,mergeCommit,reviewDecision,url"
        pulls = json.loads(self.gh("pr", "list", "--state", "all", "--head", branch, "--limit", "100", "--json", fields).stdout)
        if len(pulls) > 1:
            raise PublicationError("promotion branch has multiple pull requests")
        if not pulls:
            if self.verify_embedding(checkout):
                self.outputs(ready=True, source_commit=main, checkout=checkout)
                return
            changed = self.git("diff", "--name-only", record["preparation_commit"], main, "--", "internal/embedded/catalog", PROFILE).stdout
            if changed:
                raise PublicationError("authored catalog inputs changed after preparation")
            probe = self.git("ls-remote", "--exit-code", "origin", f"refs/heads/{branch}", check=False)
            if probe.returncode == 0:
                self.git("fetch", "--no-tags", "origin", f"refs/heads/{branch}")
                head = self.git("rev-parse", "FETCH_HEAD").stdout.strip()
                self.verify_promotion_head(head, main, "retained-promotion")
            elif probe.returncode == 2:
                staged = self.root / "promotion-catalog"
                self.stage_promotion(staged)
                target = checkout / "internal/embedded/catalog"
                shutil.rmtree(target)
                shutil.copytree(staged, target)
                command(["git", "add", "--", "internal/embedded/catalog"], cwd=checkout)
                command(["git", "-c", f"user.name={os.environ['CATALOG_BOT_NAME']}", "-c", f"user.email={os.environ['CATALOG_BOT_EMAIL']}",
                         "commit", "-m", f"catalog: embed {record['generation_id']}"], cwd=checkout)
                head = command(["git", "rev-parse", "HEAD"], cwd=checkout).stdout.strip()
                self.git("-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential", "push", "origin", f"{head}:refs/heads/{branch}")
            else:
                raise PublicationError("cannot inspect the retained promotion branch")
            body = self.root / "promotion-body.md"
            body.write_text(f"Embed verified public catalog `{record['catalog_checksum']}`.\n\nPublication run: `{record['run_id']}`.\nReceipt: `{record['receipt_checksum']}`.\n\nThe publisher advances discovery only after the merged catalog matches this artifact.\n", encoding="utf-8")
            created = self.gh("pr", "create", "--base", "main", "--head", branch, "--title", f"catalog: embed {record['generation_id']}", "--body-file", body)
            self.outputs(ready=False, status="awaiting_checks", pull_request=created.stdout.strip())
            return
        pull = pulls[0]
        details = self.api(f"pulls/{pull['number']}")
        if details["user"]["login"] != os.environ["CATALOG_BOT_NAME"]:
            raise PublicationError("promotion pull request belongs to a different author")
        if pull["state"] == "CLOSED":
            raise PublicationError("an operator closed the pending promotion")
        if pull["state"] == "MERGED":
            merged = pull["mergeCommit"]["oid"]
            self.git("fetch", "--no-tags", "origin", merged)
            if self.git("merge-base", "--is-ancestor", merged, main, check=False).returncode:
                raise PublicationError("promotion merge is not on the default branch")
            checkout = self.promotion_checkout(merged, "merged-promotion")
            if not self.verify_embedding(checkout):
                raise PublicationError("merged promotion differs from its admitted artifact")
            self.outputs(ready=True, source_commit=merged, checkout=checkout)
            return
        self.git("fetch", "--no-tags", "origin", f"refs/heads/{branch}")
        head = self.git("rev-parse", "FETCH_HEAD").stdout.strip()
        if head != pull["headRefOid"]:
            raise PublicationError("promotion branch changed during inspection")
        proposed = self.verify_promotion_head(head, main, "proposed-promotion")
        if self.git("merge-base", "--is-ancestor", main, head, check=False).returncode:
            command(["git", "-c", f"user.name={os.environ['CATALOG_BOT_NAME']}", "-c", f"user.email={os.environ['CATALOG_BOT_EMAIL']}",
                     "merge", "--no-edit", main], cwd=proposed)
            if not self.verify_embedding(proposed):
                raise PublicationError("updated base changes the selected catalog")
            updated = command(["git", "rev-parse", "HEAD"], cwd=proposed).stdout.strip()
            self.git("-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential", "push", "origin", f"{updated}:refs/heads/{branch}")
            self.outputs(ready=False, status="awaiting_updated_checks", pull_request=pull["url"])
            return
        if not self.checks_pass(head):
            self.outputs(ready=False, status="awaiting_checks", pull_request=pull["url"])
            return
        if pull.get("reviewDecision") in ("REVIEW_REQUIRED", "CHANGES_REQUESTED"):
            self.outputs(ready=False, status="awaiting_review", pull_request=pull["url"])
            return
        self.gh("pr", "merge", str(pull["number"]), "--merge", "--match-head-commit", head)
        result = json.loads(self.gh("pr", "view", str(pull["number"]), "--json", "state,mergeCommit").stdout)
        if result["state"] != "MERGED":
            raise PublicationError("promotion merge remains incomplete")
        merged = result["mergeCommit"]["oid"]
        self.git("fetch", "--no-tags", "origin", "main")
        if self.git("merge-base", "--is-ancestor", merged, "FETCH_HEAD", check=False).returncode:
            raise PublicationError("reported merge is absent from the default branch")
        checkout = self.promotion_checkout(merged, "merged-promotion")
        if not self.verify_embedding(checkout):
            raise PublicationError("merged promotion differs from its admitted artifact")
        self.outputs(ready=True, source_commit=merged, checkout=checkout)

    def channels(self):
        control = read_json(self.control)
        record = validate_pending(control["pending"])
        self.verify_stage(record)
        directory = self.root / "channels"
        directory.mkdir(exist_ok=True, mode=0o700)
        release = self.api(f"releases/tags/{record['artifact_tag']}")
        for name in ("catalog/v2", "catalog/v1"):
            previous = control["channels"][name]
            args = [self.release_tool, "--channel-release-dir", self.assets, "--channel-tag", record["artifact_tag"],
                    "--channel-published-at", release["published_at"], "--channel-out", directory / (name.replace("/", "-") + ".json"),
                    "--channel-attestation-verified"]
            if previous["document"]:
                old_tag = previous["document"]["tag"]
                if not old_tag.startswith("catalog-") or not HEX.fullmatch(old_tag[8:]):
                    raise PublicationError("previous channel has an invalid artifact tag")
                old = self.root / "previous" / name.replace("/", "-")
                self.download(old_tag, ASSETS, old)
                self.attest(old / ARCHIVE)
                args += ["--channel-current", previous["path"], "--previous-release-dir", old]
            if name == "catalog/v2":
                args += ["--channel-receipt", self.stage / RECEIPT, "--channel-receipt-checksum", record["receipt_checksum"],
                    "--channel-checkpoint", self.stage / CHECKPOINT, "--channel-checkpoint-checksum", record["checkpoint_checksum"],
                    "--channel-source-commit", os.environ["CATALOG_PROMOTED_COMMIT"],
                    "--channel-promoted-repository", os.environ["CATALOG_PROMOTED_CHECKOUT"],
                    "--channel-receipt-attestation-verified", "--channel-checkpoint-attestation-verified"]
            command(args)
        self.outputs(directory=directory)

    def finish(self):
        control = read_json(self.control)
        for name in ("catalog/v2", "catalog/v1"):
            document = self.root / "channels" / (name.replace("/", "-") + ".json")
            self.push_document(name, "channel.json", document, control["channels"][name]["commit"])
            confirmed = self.read_branch(name, "channel.json")
            if checksum(Path(confirmed["path"])) != checksum(document):
                raise PublicationError("public channel does not match its staged document")
        self.outputs(status="published", generation_id=control["pending"]["generation_id"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("inspect", "prepare", "validate", "begin", "publish", "promote", "channels", "finish"))
    parser.add_argument("--directory", required=True)
    args = parser.parse_args()
    if os.environ.get("GITHUB_ACTIONS") != "true" or os.environ.get("GITHUB_REPOSITORY") != "agentstation/starmap":
        raise PublicationError("publication operations require the configured repository workflow")
    publisher = Publisher(args.directory, os.environ["GITHUB_REPOSITORY"], os.environ["GITHUB_RUN_ID"])
    getattr(publisher, args.operation)()


if __name__ == "__main__":
    try:
        main()
    except (PublicationError, subprocess.TimeoutExpired, OSError, ValueError, KeyError) as error:
        print(f"catalog publication stopped: {error}", file=sys.stderr)
        sys.exit(1)
