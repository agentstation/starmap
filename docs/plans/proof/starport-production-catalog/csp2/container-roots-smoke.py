#!/usr/bin/env python3
"""Verify isolated container roots using an existing local binary and cached images."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import time
import uuid

parser = argparse.ArgumentParser()
parser.add_argument('--binary', required=True)
parser.add_argument('--output', required=True)
args = parser.parse_args()
binary = str(Path(args.binary).resolve(strict=True))
base = 'cgr.dev/chainguard/static@sha256:60582b2ae6074f641094af0f370d4ab241aab271858a66223dcde7eee9f51638'
probe = 'sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc'
volume = 'csp2-roots-' + uuid.uuid4().hex
containers = []
events = []
checks = {}


def run(command, check=True):
    result = subprocess.run(command, text=True, capture_output=True, timeout=60)
    events.append(dict(command=command, exit_code=result.returncode, stdout=result.stdout, stderr=result.stderr))
    if check and result.returncode:
        raise RuntimeError('command failed: ' + command[0] + ' ' + command[1])
    return result


def helper(script, writable=False):
    mount = 'type=volume,src=' + volume + ',dst=/data' + ('' if writable else ',readonly')
    return run(['docker', 'run', '--rm', '--pull=never', '--network', 'none', '--read-only', '--user', '65532:65532', '--cap-drop=ALL', '--mount', mount, probe, 'sh', '-ec', script]).stdout


def start(mounts, settings):
    command = ['docker', 'run', '-d', '--pull=never', '--network', 'none', '--read-only', '--user', '65532:65532', '--cap-drop=ALL', '--security-opt=no-new-privileges', '--tmpfs', '/tmp', '--mount', 'type=bind,src=' + binary + ',dst=/ko-app/starmap,readonly']
    command += mounts
    values = {'HOME': '/unwritable-home', 'STARMAP_CATALOG_SOURCE': 'embedded', 'STARMAP_CATALOG_ACQUISITION_ENABLED': 'false', 'STARMAP_CATALOG_SOURCE_POLL_INTERVAL': '0s', **settings}
    for name, value in values.items():
        command += ['--env', name + '=' + value]
    command += ['--entrypoint', '/ko-app/starmap', base, 'serve', '--host', '0.0.0.0']
    container = run(command).stdout.strip()
    containers.append(container)
    deadline = time.monotonic() + 45
    while time.monotonic() < deadline:
        result = run(['docker', 'run', '--rm', '--pull=never', '--network', 'container:' + container, '--read-only', '--cap-drop=ALL', probe, 'wget', '-qO-', '-T', '2', 'http://127.0.0.1:8080/health'], check=False)
        if result.returncode == 0:
            break
        if run(['docker', 'inspect', '--format', '{{.State.Running}}', container]).stdout.strip() != 'true':
            run(['docker', 'logs', container], check=False)
            raise RuntimeError('server exited before liveness')
        time.sleep(0.5)
    else:
        raise RuntimeError('server did not become live within 45 seconds')
    run(['docker', 'run', '--rm', '--pull=never', '--network', 'container:' + container, '--read-only', '--cap-drop=ALL', probe, 'wget', '-qO-', '-T', '5', 'http://127.0.0.1:8080/api/v1/ready'])
    report = run(['docker', 'exec', container, '/ko-app/starmap', 'config', 'paths', '--inspect', '--output', 'json']).stdout
    parsed = json.loads(report)
    selectors = {'config': 'STARMAP_CONFIG_DIR', 'data': 'STARMAP_DATA_DIR', 'state': 'STARMAP_STATE_ROOT', 'cache': 'STARMAP_CACHE_DIR'}
    for role, selector in selectors.items():
        expected = values.get(selector, values['STARMAP_HOME'] + '/' + role)
        actual = parsed['roots'][role]
        if actual['path'] != expected or actual['origin'] != 'environment':
            raise RuntimeError('container root depends on ambient user directories: ' + role)
    if 'STARMAP_CONFIG_DIR' in settings:
        configuration = next(item for item in parsed['files'] if item['id'] == 'configuration')
        if configuration['location'] != {'path': settings['STARMAP_CONFIG_DIR'] + '/config.yaml', 'origin': 'selected-file'}:
            raise RuntimeError('read-only configuration was not selected')
    return container, parsed


def stop(container):
    run(['docker', 'stop', '--time', '10', container])
    run(['docker', 'rm', container])
    containers.remove(container)


try:
    run(['docker', 'volume', 'create', '--label', 'purpose=csp2-isolated-verification', volume])
    first, report = start(['--mount', 'type=volume,src=' + volume + ',dst=/home/nonroot'], {'STARMAP_HOME': '/home/nonroot/starmap'})
    checks['durable_cold_start'] = True
    stop(first)
    snapshot = 'find /data/starmap/data/catalog/baseline -type f -exec sha256sum {} +; sha256sum /data/starmap/state/catalog/runtime/default/instance-seed /data/starmap/state/catalog/runtime/default/owner.json'
    before = helper(snapshot)
    helper("umask 077; mkdir -p /data/starmap/config; printf '%s\\n' 'catalog_source: embedded' 'catalog_acquisition_enabled: false' 'catalog_source_poll_interval: 0s' > /data/starmap/config/config.yaml", writable=True)
    mounts = ['--mount', 'type=volume,src=' + volume + ',dst=/home/nonroot', '--mount', 'type=volume,src=' + volume + ',dst=/etc/starmap,volume-subpath=starmap/config,readonly']
    second, second_report = start(mounts, {'STARMAP_HOME': '/home/nonroot/starmap', 'STARMAP_CONFIG_DIR': '/etc/starmap'})
    checks['readonly_configuration_restart'] = True
    stop(second)
    after = helper(snapshot)
    if sorted(before.splitlines()) != sorted(after.splitlines()):
        raise RuntimeError('baseline or runtime identity changed across container replacement')
    checks['baseline_and_identity_survive_replacement'] = True
    ephemeral, ephemeral_report = start(['--tmpfs', '/var/lib/starmap:uid=65532,gid=65532,mode=0700'], {'STARMAP_HOME': '/var/lib/starmap'})
    checks['ephemeral_cold_start'] = True
    stop(ephemeral)
except Exception as error:
    checks['error'] = str(error)
finally:
    for container in list(containers):
        run(['docker', 'logs', container], check=False)
        run(['docker', 'rm', '-f', container], check=False)
    cleanup = run(['docker', 'volume', 'rm', volume], check=False)
    checks['test_volume_removed'] = cleanup.returncode == 0
    output = {'binary_sha256': hashlib.sha256(Path(binary).read_bytes()).hexdigest(), 'base_image': base, 'probe_image': probe, 'checks': checks, 'events': events}
    Path(args.output).write_text(json.dumps(output, indent=2) + '\n')
print(json.dumps(checks))
raise SystemExit(1 if 'error' in checks or not checks['test_volume_removed'] else 0)
