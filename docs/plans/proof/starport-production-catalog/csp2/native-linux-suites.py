#!/usr/bin/env python3
"""Run the prepared native package roster in isolated Linux ARM64 containers."""
import argparse
from concurrent.futures import ThreadPoolExecutor, as_completed
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import uuid

parser = argparse.ArgumentParser()
parser.add_argument('--output', required=True)
parser.add_argument('--image', default='cgr.dev/chainguard/static@sha256:60582b2ae6074f641094af0f370d4ab241aab271858a66223dcde7eee9f51638')
parser.add_argument('--workspace-ownership', action='store_true')
parser.add_argument('--packages', nargs='*', default=['runtime', 'internal/bootstrap', 'internal/filepublish', 'internal/privatefiles', 'internal/runtimeacl/windows', 'internal/sources/github', 'internal/cli/app', 'pkg/productpaths', 'pkg/catalogs/storage', 'internal/catalog/workspace'])
args = parser.parse_args()
if args.workspace_ownership:
    args.packages = ['internal/catalog/workspace']
root = Path.cwd()
output = Path(args.output).resolve()
output.mkdir(parents=True, exist_ok=True)
base = args.image


def run(command, **kwargs):
    result = subprocess.run(command, capture_output=True, text=True, **kwargs)
    return dict(command=command, exit_code=result.returncode, stdout=result.stdout, stderr=result.stderr)


engine = run(['docker', 'info', '--format', '{{.OSType}} {{.Architecture}} {{.ServerVersion}} {{.KernelVersion}}'])
if engine['exit_code'] or not engine['stdout'].startswith('linux aarch64 '):
    raise SystemExit('requires the native Linux ARM64 Docker engine')
(output / 'engine.json').write_text(json.dumps(engine, indent=2) + '\n')
image = run(['docker', 'image', 'inspect', base, '--format', '{{.Os}} {{.Architecture}} {{.Id}}'])
if image['exit_code'] or not image['stdout'].startswith('linux arm64 sha256:'):
    raise SystemExit('requires a cached Linux ARM64 image')
(output / 'image.json').write_text(json.dumps(image, indent=2) + '\n')
base = image['stdout'].split()[2]


def verify(package):
    evidence = {'package': package, 'race': False, 'toolchain': 'go1.25.12', 'commands': [], 'test_events': []}
    name = 'csp2-native-' + uuid.uuid4().hex
    volume_created = False
    container = None
    with tempfile.TemporaryDirectory(prefix='csp2-native-build-') as temporary:
        binary = Path(temporary) / 'test'
        try:
            tags = ['-tags=starmap_ownership_test'] if args.workspace_ownership else []
            build = run(['env', 'GOTOOLCHAIN=go1.25.12', 'GOOS=linux', 'GOARCH=arm64', 'CGO_ENABLED=0', 'go', 'test', '-c', *tags, '-o', str(binary), './' + package])
            evidence['commands'].append(build)
            if build['exit_code']:
                raise RuntimeError('test binary build failed')
            evidence['binary_sha256'] = hashlib.sha256(binary.read_bytes()).hexdigest()
            create_volume = run(['docker', 'volume', 'create', '--label', 'purpose=csp2-native-package-verification', name])
            evidence['commands'].append(create_volume)
            if create_volume['exit_code']:
                raise RuntimeError('volume creation failed')
            volume_created = True
            actor = '0:0' if args.workspace_ownership else '65532:65532'
            capabilities = ['--cap-add=' + cap for cap in ['CHOWN', 'FOWNER', 'DAC_OVERRIDE', 'SETUID', 'SETGID']] if args.workspace_ownership else []
            fixture = ['--env', 'STARMAP_OWNERSHIP_FIXTURE=1'] if args.workspace_ownership else []
            selection = ['-test.run=^TestWorkspaceForeignOwnership$'] if args.workspace_ownership else []
            create = run(['docker', 'create', '--pull=never', '--network', 'none', '--read-only', '--user', actor, '--cap-drop=ALL', *capabilities, '--security-opt=no-new-privileges', *fixture, '--mount', 'type=bind,src=' + str(binary) + ',dst=/native-test,readonly', '--mount', 'type=bind,src=' + str(root) + ',dst=' + str(root) + ',readonly', '--mount', 'type=volume,src=' + name + ',dst=/home/nonroot', '--env', 'TMPDIR=/home/nonroot', '--env', 'GOMAXPROCS=4', '--workdir', str(root / package), '--entrypoint', '/native-test', base, *selection, '-test.v=test2json', '-test.count=1', '-test.parallel=4', '-test.timeout=10m'])
            evidence['commands'].append(create)
            if create['exit_code']:
                raise RuntimeError('container creation failed')
            container = create['stdout'].strip()
            execution = run(['docker', 'start', '--attach', container])
            evidence['commands'].append(execution)
            state = run(['docker', 'inspect', '--format', '{{json .State}}', container])
            evidence['commands'].append(state)
            evidence['state'] = json.loads(state['stdout'])
            package_name = 'github.com/agentstation/starmap' + ('' if package == '.' else '/' + package)
            converted = run(['env', 'GOTOOLCHAIN=go1.25.12', 'go', 'tool', 'test2json', '-t', '-p', package_name], input=execution['stdout'] + execution['stderr'])
            evidence['conversion_exit_code'] = converted['exit_code']
            evidence['test_events'] = [json.loads(line) for line in converted['stdout'].splitlines()]
            evidence['counts'] = {action: sum(event.get('Action') == action and 'Test' in event for event in evidence['test_events']) for action in ['pass', 'fail', 'skip']}
            evidence['passed'] = execution['exit_code'] == 0 and evidence['state']['ExitCode'] == 0 and converted['exit_code'] == 0 and evidence['counts']['pass'] > 0 and evidence['counts']['fail'] == 0 and not evidence['state']['OOMKilled']
        except Exception as error:
            evidence['error'] = str(error)
            evidence['passed'] = False
        finally:
            if container:
                cleanup = run(['docker', 'rm', '-f', container])
                evidence['commands'].append(cleanup)
                evidence['container_removed'] = cleanup['exit_code'] == 0
            if volume_created:
                cleanup = run(['docker', 'volume', 'rm', name])
                evidence['commands'].append(cleanup)
                evidence['volume_removed'] = cleanup['exit_code'] == 0
            evidence['passed'] = evidence.get('passed', False) and evidence.get('container_removed', False) and evidence.get('volume_removed', False)
            record_name = 'root' if package == '.' else package.replace('/', '-')
            (output / (record_name + '.json')).write_text(json.dumps(evidence, indent=2) + '\n')
            print(json.dumps({key: evidence.get(key) for key in ['package', 'passed', 'counts', 'error']}), flush=True)
    return evidence


results = []
with ThreadPoolExecutor(max_workers=2) as pool:
    for future in as_completed([pool.submit(verify, package) for package in args.packages]):
        results.append(future.result())
summary = {'packages': len(results), 'passed_packages': sum(item['passed'] for item in results), 'test_counts': {action: sum(item.get('counts', {}).get(action, 0) for item in results) for action in ['pass', 'fail', 'skip']}, 'scope': 'Native Linux ARM64 containers, CGO disabled, no race instrumentation, no external network. Named volumes hold temporary test files. No power-loss or released-pair qualification.'}
(output / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
print(json.dumps(summary), flush=True)
raise SystemExit(0 if summary['passed_packages'] == summary['packages'] else 1)
