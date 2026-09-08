import argparse
import hashlib
import html
import json
import re
import subprocess
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import unquote, urlsplit

parser = argparse.ArgumentParser()
parser.add_argument('--starport-root', type=Path, required=True)
args = parser.parse_args()
revision = Path(__file__).resolve().parent
root = revision.parents[4]
starport = args.starport_root.resolve()
proof = revision.parent
manifest = json.loads((revision / 'input-manifest.json').read_text())
backup = Path(manifest['backup'])
plan = root / 'docs/plans/starport-production-catalog-plan.html'
text = plan.read_text()


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


class Page(HTMLParser):
    def __init__(self):
        super().__init__()
        self.ids = []
        self.links = []
        self.resources = []

    def handle_starttag(self, tag, attrs):
        attributes = dict(attrs)
        if 'id' in attributes:
            self.ids.append(attributes['id'])
        if tag == 'a' and 'href' in attributes:
            self.links.append(attributes['href'])
        if tag in ('img', 'script', 'iframe', 'link'):
            self.resources.append(attributes.get('src', attributes.get('href', '')))


page = Page()
page.feed(text)
assert len(page.ids) == len(set(page.ids))
assert not any(urlsplit(value).scheme or value.startswith('//') for value in page.resources)
rows = re.findall(r'<tr id="(CSP[\d.]+)" data-status="([^"]+)">(.*?)</tr>', text, re.S)
articles = re.findall(r'<article class="task" id="task-(CSP[\d.]+)">(.*?)</article>', text, re.S)
order = [task for task, status, body in rows]
assert len(rows) == 38 and all(status == 'todo' for task, status, body in rows)
assert order == [task for task, body in articles]
assert all('<code>todo</code>' in body for task, status, body in rows)
assert 'Record all 50 primary cases and 309 required subcases.' in text
oldplan = (backup / 'starmap' / plan.relative_to(root)).read_text()
oldorder = re.findall(r'<tr id="(CSP[\d.]+)"', oldplan)
assert [task for task in order if task in oldorder] == oldorder
assert order[-1] == 'CSP99'
for task, body in articles:
    for field in ['Problem:', 'Owning concept and paths:', 'Depends on:', 'Steps:', 'Acceptance:', 'Fail-before:', 'Verification:']:
        assert field in body, (task, field)
    dependencies = re.search(r'Depends on:</strong>(.*?)</p>', body, re.S).group(1)
    for dependency in re.findall(r'CSP\d+(?:\.\d+)?', dependencies):
        assert dependency in order and order.index(dependency) < order.index(task), (task, dependency)
    if task not in ('CSP0', 'CSP99'):
        assert '--task ' + task + '<' in body, task
sections = ['overview', 'outcome', 'progress', 'architecture', 'scope', 'promotion-gate', 'invariants', 'ledger', 'verification', 'tasks', 'goal', 'execution-log']
assert all(section in page.ids for section in sections)
assert [page.ids.index(section) for section in sections] == sorted(page.ids.index(section) for section in sections)
assert 'data-plan-status="proposed"' in text
ledger_line = text[:text.index('<section id="ledger">')].count('\n') + 1
assert ledger_line <= 120
assert len(text.splitlines()) <= 500
oldlog = oldplan.split('<section id="execution-log">')[1]
assert all(row in text for row in re.findall(r'<tr><td>2026-.*?</tr>', oldlog))

acceptance = json.loads((proof / 'acceptance-map.json').read_text())
previous = json.loads((backup / 'starmap/docs/plans/proof/starport-production-catalog/acceptance-map.json').read_text())
expected = {f'A{i:02}' for i in range(1, 51)}
assert acceptance['condition_count'] == 50
assert set(acceptance['primary_task']) == set(acceptance['required_subcases']) == expected
subcases = [case for roster in acceptance['required_subcases'].values() for case in roster]
assert len(subcases) == len(set(subcases)) == 309
assert all(subcase.startswith(case + '.') for case, roster in acceptance['required_subcases'].items() for subcase in roster)
oldsubcases = {subcase for roster in previous['required_subcases'].values() for subcase in roster}
assert oldsubcases <= set(subcases)
assert len(oldsubcases) == 241 and len(set(subcases) - oldsubcases) == 68
assert set(subcases) - oldsubcases <= set(acceptance['subcase_contracts'])
assert all(acceptance['subcase_contracts'][key] == value for key, value in previous['subcase_contracts'].items())
checks = set(subcases) | set(acceptance['early_checks']) | set(acceptance['rehearsal_checks'])
assert set(acceptance['task_checks']) == set(order) - {'CSP0', 'CSP99'}
for task, roster in acceptance['task_checks'].items():
    assert len(roster) == len(set(roster)) and set(roster) <= checks, task
for case, owner in acceptance['primary_task'].items():
    assert set(acceptance['required_subcases'][case]) <= set(acceptance['task_checks'][owner]), (case, owner)
qualification = acceptance['qualification']
candidate = set(qualification['candidate_required_primary_cases'])
published = set(qualification['requires_published_assets'])
assert len(candidate) == 43 and len(published) == 7
assert not candidate & published and candidate | published == expected
assert set(qualification['final_required_primary_cases']) == expected
assert set(acceptance['task_checks']['CSP22']) == {subcase for case in candidate for subcase in acceptance['required_subcases'][case]}
assert set(acceptance['task_checks']['CSP24']) == set(subcases)
assert set(acceptance['task_checks']['CSP21']) == set(acceptance['rehearsal_checks'])

prd = root / 'docs/design/catalog-lifecycle/PRD.md'
spec = root / 'docs/design/catalog-lifecycle/ENGINEERING_SPEC.md'
prd_ids = re.findall(r'^\| (P\d{2}) \|', prd.read_text(), re.M)
spec_cases = re.findall(r'^\| (A\d{2}) \| ([^|]+) \|', spec.read_text(), re.M)
assert len(prd_ids) == len(set(prd_ids)) == 38
oldprd = (backup / 'starmap' / prd.relative_to(root)).read_text()
assert all(line in prd.read_text() for line in oldprd.splitlines() if line.startswith('| D'))
assert len(spec_cases) == 50 and {case for case, requirements in spec_cases} == expected
assert set(prd_ids) == set(re.findall(r'P\d{2}', ' '.join(requirements for case, requirements in spec_cases)))
milestones = ['CSP7', 'CSP10.3', 'CSP15', 'CSP17', 'CSP21', 'CSP22', 'CSP23', 'CSP24']
phase_counts = {task: sum(order.index(owner) <= order.index(task) for owner in acceptance['primary_task'].values()) for task in milestones}
assert list(phase_counts.values()) == [5, 25, 35, 39, 41, 43, 45, 50]
assert acceptance['storage_review_mapping'] == previous['storage_review_mapping']
mapping = acceptance['latency_review_mapping']
assert set(mapping) == {f'LR{i:02}' for i in range(1, 8)}
resolution = (revision / 'REVIEW_RESOLUTION.md').read_text()
for finding, entry in mapping.items():
    assert set(entry['tasks']) <= set(order) and set(entry['cases']) <= expected
    assert '| ' + finding + ' |' in resolution
    for section in entry['spec']:
        assert re.search(r'^#{2,4} ' + re.escape(section) + r'\.?(?: |$)', spec.read_text(), re.M), (finding, section)

local_links = []
source_links = []
preexisting_missing_links = []
heads = {name: subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=directory, text=True).strip() for name, directory in [('starmap', root), ('starport', starport)]}
assert heads == manifest['heads']


def anchors(target):
    if target.suffix == '.html':
        parsed = Page()
        parsed.feed(target.read_text())
        return set(parsed.ids)
    prose = target.read_text()
    headings = {re.sub(r'[^\w\- ]', '', heading.lower()).replace(' ', '-') for heading in re.findall(r'^#+\s+(.+?)\s*#*$', prose, re.M)}
    explicit = set(re.findall(r'<(?:a|h[1-6])\s+[^>]*(?:id|name)=["\x27]([^"\x27]+)', prose))
    return headings | explicit


def link(origin, href):
    href = html.unescape(href)
    parsed = urlsplit(href)
    if parsed.scheme in ('http', 'https'):
        match = re.match(r'https://github.com/agentstation/(starmap|starport)/(blob|tree)/([0-9a-f]{40})/([^#]+)(?:#L(\d+)(?:-L(\d+))?)?$', href)
        if match:
            product, kind, commit, path, start, end = match.groups()
            directory = root if product == 'starmap' else starport
            data = subprocess.check_output(['git', 'show', f'{commit}:{unquote(path)}'], cwd=directory)
            if start:
                assert 1 <= int(start) <= int(end or start) <= len(data.splitlines()), href
            source_links.append(href)
        return
    if parsed.scheme:
        return
    target = (origin.parent / unquote(parsed.path)).resolve() if parsed.path else origin
    if not target.exists() and target != revision / 'verification.json':
        baseline = backup / 'starmap' / origin.relative_to(root)
        assert baseline.exists() and href in baseline.read_text(), (origin, href)
        preexisting_missing_links.append((str(origin.relative_to(root)), href))
        return
    if parsed.fragment and target.suffix in ('.md', '.html'):
        assert unquote(parsed.fragment) in anchors(target), (origin, href)
    local_links.append((str(origin.relative_to(root)), href))


for href in page.links:
    link(plan, href)
html_count = len(local_links)
documents = [prd, spec, root / 'docs/design/catalog-lifecycle/REPOSITORY_FINDINGS.md', root / 'docs/design/catalog-lifecycle/LATENCY_REVIEW.md', revision / 'REVIEW_RESOLUTION.md', proof / 'readme-demo-brief.md', root / 'docs/plans/README.md', root / 'docs/README.md']
for document in documents:
    prose = document.read_text()
    for href in re.findall(r'!?\[[^\]]*\]\(([^\s)]+)(?:\s+"[^"]*")?\)', prose):
        link(document, href)
    for href in re.findall(r'^\[[^\]]+\]:\s+(\S+)', prose, re.M):
        link(document, href)

for entry in manifest['historical_evidence']:
    assert digest(root / entry['path']) == entry['sha256'], entry['path']
for entry in manifest['inputs']:
    assert digest(backup / entry['repository'] / entry['path']) == entry['sha256'], entry['path']
findings = root / 'docs/design/catalog-lifecycle/REPOSITORY_FINDINGS.md'
assert findings.read_text().startswith((backup / 'starmap' / findings.relative_to(root)).read_text())
source_manifest = json.loads((root / 'docs/design/catalog-lifecycle/evidence/latency-review-2026-09-05/manifest.json').read_text())
for path, sha256 in source_manifest['source_sha256'].items():
    assert digest(starport / path) == sha256, path
json_files = list(proof.rglob('*.json')) + list((root / 'docs/design/catalog-lifecycle/evidence').rglob('*.json'))
for path in json_files:
    json.loads(path.read_text())
for directory in [root, starport]:
    subprocess.run(['git', 'diff', '--check'], cwd=directory, check=True)
    changed = subprocess.check_output(['git', 'diff', '--name-only'], cwd=directory, text=True).splitlines()
    assert all(path.startswith('docs/') for path in changed), changed

result = {
    'verdict': 'PASS', 'scope': 'Document structure and evidence validation. No product acceptance credit.',
    'task_count': len(order), 'todo_count': len(rows), 'plan_lines': len(text.splitlines()), 'ledger_line': ledger_line,
    'prd_count': len(prd_ids), 'acceptance_count': len(spec_cases), 'required_subcase_count': len(subcases),
    'new_subcase_count': len(set(subcases) - oldsubcases), 'latency_findings_mapped': len(mapping), 'storage_findings_preserved': len(acceptance['storage_review_mapping']),
    'candidate_primary_cases': len(candidate), 'publication_dependent_cases': len(published),
    'phase_primary_counts': phase_counts, 'task_checklists': len(acceptance['task_checks']),
    'html_local_links_checked': html_count, 'markdown_local_links_checked': len(local_links) - html_count,
    'pinned_source_references_checked': len(source_links), 'historical_files_unchanged': len(manifest['historical_evidence']),
    'preexisting_missing_links': preexisting_missing_links, 'json_files_parsed': len(json_files),
    'original_tasks_preserved': True, 'execution_log_preserved': True, 'old_subcases_preserved': True,
    'no_product_source_changes': True, 'source_hashes_checked': len(source_manifest['source_sha256']),
    'document_sha256': {str(path.relative_to(root)): digest(path) for path in [plan, proof / 'acceptance-map.json', Path(__file__).resolve(), *documents]},
    'starport_index_sha256': digest(starport / 'docs/TASKS.md'), 'implementation_cases_run': 0, 'plan_status': 'proposed'
}
(revision / 'verification.json').write_text(json.dumps(result, indent=2) + '\n')
print(json.dumps(result, indent=2))
