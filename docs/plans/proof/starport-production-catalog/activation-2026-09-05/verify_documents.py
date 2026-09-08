import hashlib
import html
import importlib.util
import json
import re
import subprocess
from pathlib import Path
from urllib.parse import unquote, urlsplit

revision = Path(__file__).resolve().parent
root = revision.parents[4]
proof = revision.parent
sp = root.parent / 'starport-catalog-lifecycle'
module = importlib.util.spec_from_file_location('catalog_product_verify', root / 'scripts/catalog_product_verify.py')
verifier = importlib.util.module_from_spec(module)
module.loader.exec_module(verifier)
a = verifier.read_json(verifier.ROSTER)
verifier.validate_roster(a)
plan = root / 'docs/plans/starport-production-catalog-plan.html'
text = plan.read_text()
rows = re.findall(r'<tr id="(CSP[\d.]+)" data-status="([^"]+)"', text)
articles = re.findall(r'<article class="task" id="task-(CSP[\d.]+)">(.*?)</article>', text, re.S)
order = [task for task, state in rows]
assert len(order) == len(set(order)) == 38
assert order == [task for task, body in articles]
assert len([state for task, state in rows if state == 'in_progress']) == 1
assert 'data-plan-status="active">active' in text
assert text[:text.index('<section id="ledger">')].count('\n') + 1 <= 120
assert len(text.splitlines()) <= 500
assert len(re.findall(r'^\| P\d{2} \|', (root / 'docs/design/catalog-lifecycle/PRD.md').read_text(), re.M)) == 38
assert sum(map(len, a['required_subcases'].values())) == 324
assert len(a['task_checks']['CSP22']) == 305
assert len(a['qualification']['candidate_additional_subcases']) == 8
assert set(a['audit_review_mapping']) == {f'CA{i:02}' for i in range(1, 8)}
graph = {}
for task, body in articles:
    dep = html.unescape(re.sub('<[^>]+>', ' ', re.search(r'Depends on:</strong>(.*?)</p>', body, re.S).group(1)))
    deps = set(re.findall(r'CSP\d+(?:\.\d+)?', dep))
    for start, end in re.findall(r'(CSP\d+(?:\.\d+)?) through (CSP\d+(?:\.\d+)?)', dep):
        deps.update(order[order.index(start):order.index(end)+1])
    assert all(order.index(d) < order.index(task) for d in deps)
    graph[task] = sorted(deps, key=order.index)
assert 'CSP12.2' in graph['CSP13']
previous = json.loads((proof / 'final-audit-2026-09-05/input-manifest.json').read_text())
allowed = set(json.loads((revision / 'input-manifest.json').read_text())['files'])
unchanged = 0
for path, digest in previous['files'].items():
    if path in allowed:
        continue
    assert hashlib.sha256(Path(path).read_bytes()).hexdigest() == digest, path
    unchanged += 1
historical = json.loads((proof / 'latency-revision-2026-09-05/input-manifest.json').read_text())['historical_evidence']
for entry in historical:
    assert hashlib.sha256((root / entry['path']).read_bytes()).hexdigest() == entry['sha256']
local, pinned = [], []
paths = [plan, root / 'docs/plans/README.md', root / 'docs/design/catalog-lifecycle/PRD.md', root / 'docs/design/catalog-lifecycle/ENGINEERING_SPEC.md', root / 'docs/design/catalog-lifecycle/REPOSITORY_FINDINGS.md', revision / 'RESOLUTION.md']
for p in paths:
    contents = p.read_text()
    refs = re.findall(r'(?:href="([^"]+)"|!?\[[^\]]*\]\(([^\s)]+)\))', contents)
    for pair in refs:
        href = html.unescape(next(value for value in pair if value))
        u = urlsplit(href)
        if u.scheme:
            m = re.match(r'https://github.com/agentstation/(starmap|starport)/(?:blob|tree)/([a-f0-9]{40})/([^#]+)(?:#L(\d+)(?:-L(\d+))?)?$', href)
            if m:
                product, commit, source, start, end = m.groups()
                payload = subprocess.check_output(['git', 'show', f'{commit}:{unquote(source)}'], cwd=root if product == 'starmap' else sp)
                if start:
                    assert 1 <= int(start) <= int(end or start) <= len(payload.splitlines())
                pinned.append(href)
            continue
        target = (p.parent / unquote(u.path)).resolve() if u.path else p
        assert target.exists(), (p, href)
        if u.fragment:
            data = target.read_text()
            anchors = set(re.findall(r'\bid="([^"]+)"', data))
            anchors |= {re.sub(r'[^\w\- ]', '', h.lower()).replace(' ', '-') for h in re.findall(r'^#+\s+(.+?)\s*#*$', data, re.M)}
            assert unquote(u.fragment) in anchors, (p, href)
        local.append(href)
for directory in [root, sp]:
    subprocess.run(['git', 'diff', '--check'], cwd=directory, check=True)
result = {'verdict': 'PASS', 'scope': 'Document contracts only. No product acceptance credit.', 'tasks': 38, 'primary_cases': 50, 'required_subcases': 324, 'candidate_subcases': 305, 'candidate_extra_subcases': 8, 'dependency_graph': graph, 'CA05_dependency_present': True, 'prior_audit_inputs_preserved': unchanged, 'historical_files_preserved': len(historical), 'local_links_checked': len(local), 'pinned_links_checked': len(pinned), 'plan_lines': len(text.splitlines()), 'artifact_sha256': {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest() for p in paths}}
print(json.dumps(result, indent=2))
