"""Validate captured GitHub evidence for one checked catalog publication."""

from datetime import datetime
import re

from catalog_publication import REQUIRED_CHECKS, validate_pending


REPOSITORY = "agentstation/starmap"
BOT = "starmap-catalog-publisher[bot]"
ACTIONS_APP = 15368


def timestamp(value):
    if not isinstance(value, str):
        raise ValueError("Publication evidence has no timestamp.")
    result = datetime.fromisoformat(value.replace("Z", "+00:00"))
    if result.utcoffset() is None:
        raise ValueError("Publication evidence has an unqualified timezone.")
    return result


def validate_promotion(pull, protection, check_pages):
    """Require successful checks on the bot head before its protected merge."""
    if (pull["user"]["login"] != BOT or pull["user"]["type"] != "Bot"
            or pull["base"]["repo"]["full_name"] != REPOSITORY or pull["base"]["ref"] != "main"
            or pull["head"]["repo"]["full_name"] != REPOSITORY
            or pull["merged"] is not True or pull["state"] != "closed"):
        raise ValueError("Publication evidence does not identify a merged catalog bot pull request.")
    head, merged = pull["head"]["sha"], pull["merge_commit_sha"]
    if not all(isinstance(value, str) and re.fullmatch(r"[0-9a-f]{40}", value) for value in (head, merged)):
        raise ValueError("Publication evidence has an invalid head or merge commit.")
    merged_at = timestamp(pull["merged_at"])
    rules = protection["required_status_checks"]
    required = {(item["context"], item["app_id"]) for item in rules["checks"]}
    if rules["strict"] is not True or not {(name, ACTIONS_APP) for name in REQUIRED_CHECKS[:2]} <= required:
        raise ValueError("Promotion evidence lacks strict checks from GitHub Actions.")
    latest = {}
    for page in check_pages:
        for check in page["check_runs"]:
            if check["name"] not in REQUIRED_CHECKS or check["head_sha"] != head or check["app"]["id"] != ACTIONS_APP:
                continue
            if type(check["id"]) is not int or check["id"] <= 0:
                raise ValueError("A promotion check has an invalid identity.")
            # A post-merge rerun cannot qualify the decision to merge.
            if timestamp(check["started_at"]) > merged_at:
                continue
            prior = latest.get(check["name"])
            if prior is None or check["id"] > prior["id"]:
                latest[check["name"]] = check
    for name in REQUIRED_CHECKS:
        check = latest.get(name)
        if (check is None or check["status"] != "completed" or check["conclusion"] != "success"
                or timestamp(check["completed_at"]) > merged_at):
            raise ValueError("A required promotion check did not pass before the merge: " + name)
    return {"head": head, "merge": merged, "merged_at": pull["merged_at"], "checks": {name: latest[name]["id"] for name in REQUIRED_CHECKS}}


def validate_workflow(run, expected_id):
    """Require a completed main-branch run of the repository publisher."""
    if (type(expected_id) is not int or expected_id <= 0 or run["id"] != expected_id
            or run["repository"]["full_name"] != REPOSITORY or run["head_branch"] != "main"
            or run["path"] != ".github/workflows/catalog-generation.yaml"
            or run["event"] not in ("workflow_dispatch", "schedule", "workflow_run")
            or run["status"] != "completed" or run["conclusion"] != "success"
            or not re.fullmatch(r"[0-9a-f]{40}", run["head_sha"])):
        raise ValueError("Publication evidence has an unsuccessful or unrelated publisher run.")
    if (type(run["run_attempt"]) is not int or run["run_attempt"] <= 0
            or not timestamp(run["created_at"]) <= timestamp(run["run_started_at"]) <= timestamp(run["updated_at"])):
        raise ValueError("Publisher run attempt or timestamps are invalid.")
    jobs = [job for job in run["jobs"] if job["name"] == "generate"]
    if (len(jobs) != 1 or jobs[0]["run_id"] != expected_id or jobs[0]["run_attempt"] != run["run_attempt"]
            or jobs[0]["status"] != "completed" or jobs[0]["conclusion"] != "success"):
        raise ValueError("The publisher job did not complete in the selected attempt.")
    steps = {step["name"]: step for step in jobs[0]["steps"]}
    for name in ("Restore accepted or pending publication", "Publish and verify immutable public inputs",
                 "Promote exact input through checked pull request", "Stage channels after verified merge",
                 "Attest both discovery channels", "Publish and verify both discovery channels"):
        if name not in steps or steps[name]["status"] != "completed" or steps[name]["conclusion"] != "success":
            raise ValueError("The publisher did not complete a required publication step: " + name)


def release_assets(releases, record):
    """Bind both immutable releases and all assets to the selected publication."""
    expected = {
        record["artifact_tag"]: {"starmap-catalog.tar.gz", "starmap-catalog.tar.gz.sha256", "starmap-catalog.intoto.json"},
        record["receipt_tag"]: {"starmap-catalog-run.json", "starmap-catalog-state.json"},
    }
    if len(releases) != 2 or {release["tag_name"] for release in releases} != set(expected):
        raise ValueError("Publication evidence omits or duplicates an immutable release.")
    assets = {}
    identifiers = set()
    for release in releases:
        if release["draft"] is not False or release["prerelease"] is not False:
            raise ValueError("A catalog release is not publicly published.")
        tag = release["tag_name"]
        if len(release["assets"]) != len(expected[tag]) or {item["name"] for item in release["assets"]} != expected[tag]:
            raise ValueError("A catalog release omits or duplicates a required asset.")
        for asset in release["assets"]:
            identifier = asset["id"]
            if type(identifier) is not int or identifier <= 0 or identifier in identifiers:
                raise ValueError("A catalog asset has an invalid or duplicate identity.")
            identifiers.add(identifier)
            if (asset["state"] != "uploaded" or type(asset["size"]) is not int or asset["size"] <= 0
                    or not re.fullmatch(r"sha256:[0-9a-f]{64}", asset["digest"])):
                raise ValueError("A catalog asset has incomplete bytes or digest evidence.")
            assets[asset["name"]] = (identifier, asset["size"], asset["digest"], asset["created_at"])
    for name, field in (("starmap-catalog.tar.gz", "archive_checksum"),
                        ("starmap-catalog-run.json", "receipt_checksum"),
                        ("starmap-catalog-state.json", "checkpoint_checksum")):
        if assets[name][2] != record[field]:
            raise ValueError("Published asset bytes differ from the admitted publication.")
    return assets


def validate_same_bytes_retry(record, initial_releases, final_releases, completion, retry):
    """Require a later successful run without replacing any published asset."""
    validate_pending(record)
    validate_workflow(completion, completion["id"])
    validate_workflow(retry, retry["id"])
    if ((completion["id"], completion["run_attempt"]) == (retry["id"], retry["run_attempt"])
            or timestamp(retry["run_started_at"]) <= timestamp(completion["updated_at"])):
        raise ValueError("Retry evidence does not follow a completed publication run.")
    before = release_assets(initial_releases, record)
    after = release_assets(final_releases, record)
    if before != after:
        raise ValueError("Publication retry replaced or changed an immutable asset.")
    if any(timestamp(asset[3]) > timestamp(completion["updated_at"]) for asset in after.values()):
        raise ValueError("An immutable asset appeared only after the completed publication.")
    return {"completion_run": [completion["id"], completion["run_attempt"]],
            "retry_run": [retry["id"], retry["run_attempt"]], "asset_ids": sorted(asset[0] for asset in after.values())}


def validate_channels(record, channels, promotion):
    """Bind both discovery documents to the checked merge and immutable assets."""
    if set(channels) != {"catalog/v1", "catalog/v2"}:
        raise ValueError("Publication evidence requires both discovery channels.")
    for name, channel in channels.items():
        if (channel["channel"] != name or channel["tag"] != record["artifact_tag"]
                or channel["catalog_digest"] != record["catalog_checksum"]
                or channel["generation_id"] != record["generation_id"]
                or type(channel["sequence"]) is not int or channel["sequence"] <= 0):
            raise ValueError("A discovery channel selects another catalog publication.")
        archives = [asset for asset in channel["assets"] if asset["name"] == "starmap-catalog.tar.gz"]
        if len(archives) != 1 or archives[0]["checksum"] != record["archive_checksum"]:
            raise ValueError("A discovery channel selects different archive bytes.")
    modern = channels["catalog/v2"]["publication"]
    if (modern["source_commit"] != promotion["merge"] or modern["receipt_tag"] != record["receipt_tag"]
            or modern["receipt"]["checksum"] != record["receipt_checksum"]
            or modern["checkpoint"]["checksum"] != record["checkpoint_checksum"]):
        raise ValueError("Discovery does not bind the checked merge and original publication evidence.")


def validate_attestation(reports, digest, allowed_sources):
    """Bind a successful CLI verification to the expected source and run attempt."""
    for report in reports:
        result = report["verificationResult"]
        certificate = result["signature"]["certificate"]
        identity = (certificate["sourceRepositoryDigest"], certificate["runInvocationURI"])
        subjects = result["statement"]["subject"]
        if any(commit == identity[0] and re.fullmatch(invocation, identity[1]) for commit, invocation in allowed_sources):
            if any(subject.get("digest", {}).get("sha256") == digest.removeprefix("sha256:") for subject in subjects):
                return
    raise ValueError("Attestation does not bind these bytes to the expected publisher source and run.")
