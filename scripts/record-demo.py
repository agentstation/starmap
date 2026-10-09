#!/usr/bin/env python3
"""Record the Starmap catalog server from an isolated source build."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import socket
import subprocess
import tempfile
import xml.etree.ElementTree as ET


ROOT = Path(__file__).resolve().parents[1]
TAPE = ROOT / 'scripts/demo.tape'
FONT = ROOT / 'scripts/demo-font/GeistMono-Regular.woff2'
MODEL_ID = 'openai/gpt-6.1-sol'
MODEL_NAME = 'GPT-6.1 Sol'


def run(command, **kwargs):
    return subprocess.run(command, check=True, timeout=180, **kwargs)


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def require(condition, message):
    if not condition:
        raise ValueError(message)



def catalog_inputs(root=ROOT):
    embedded = root / 'internal/embedded'
    paths = sorted(path for directory in [embedded / 'catalog', embedded / 'sources']
                   for path in directory.rglob('*') if path.is_file())
    files = {str(path.relative_to(root)): {'sha256': sha256(path), 'bytes': path.stat().st_size} for path in paths}
    digest = hashlib.sha256(json.dumps(files, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
    changed = run(['git', 'diff', 'HEAD', '--name-only', '--', 'internal/embedded'], cwd=root, capture_output=True, text=True).stdout.splitlines()
    changed += run(['git', 'ls-files', '--others', '--exclude-standard', '--', 'internal/embedded'],
                   cwd=root, capture_output=True, text=True).stdout.splitlines()
    catalog = 'internal/embedded/catalog/'
    required = {catalog + name for name in ['generation.json', 'generation-manifest.json', 'generation-payload.json.gz',
                                          'membership-scopes.yaml', 'provenance.yaml', 'endpoints.yaml',
                                          'authors/openai/models/gpt-6.1-sol.yaml', 'providers/openai/models/gpt-6.1-sol.yaml']}
    manifest = json.loads((root / catalog / 'generation-manifest.json').read_text())
    bootstrap = json.loads((root / catalog / 'generation.json').read_text())
    return {'embedded_tree_sha256': digest, 'embedded_file_count': len(files),
            'working_tree_changes': sorted(set(changed)),
            'input_files': {path: files[path] for path in sorted(required | set(changed)) if path in files},
            'bootstrap': bootstrap,
            'committed_generation': {key: manifest[key] for key in ['generation_id', 'schema_version', 'generated_at', 'payload']},
            'model_definition': {'id': MODEL_ID, 'name': MODEL_NAME, 'origin': 'Reviewed working-tree author record.'},
            'provider_observations': [record for record in manifest['source_observations'] if record['source'] == 'providers'],
            'review_candidate_count': len(manifest.get('review_candidates', []))}


def verify_catalog_inputs(before, after):
    require(before == after, 'Embedded catalog inputs changed during recording.')


def verify_generation_snapshot(result, catalog):
    expected = catalog['bootstrap']
    require(result['generation_id'] == expected['generation_id'], 'Served generation differs from the reviewed catalog.')
    require('sha256:' + result['payload_sha256'] == expected['payload']['checksum'], 'Served payload differs from the reviewed catalog.')
    require(result['payload_size_bytes'] == expected['payload']['size_bytes'], 'Served payload size differs from the reviewed catalog.')


def verify(directory):
    ready = json.loads((directory / 'ready.json').read_text())
    model = json.loads((directory / 'model.json').read_text())
    manifest = json.loads((directory / 'manifest.json').read_text())
    payload = directory / 'catalog.json'
    require(ready['data']['status'] == 'ready', 'Server readiness did not pass.')
    require(model['data']['id'] == MODEL_ID, 'Wrong model definition.')
    require(model['data']['name'] == MODEL_NAME, 'Wrong model display name.')
    require(model['data']['author_ids'] == ['openai'], 'Wrong model author.')
    require(manifest['validation']['status'] == 'passed', 'Generation validation failed.')
    require(manifest['payload']['checksum'] == 'sha256:' + sha256(payload), 'Payload digest mismatch.')
    require(manifest['payload']['size_bytes'] == payload.stat().st_size, 'Payload size mismatch.')
    headers = (directory / 'payload.headers').read_text().lower()
    require('x-starmap-generation-id: ' + manifest['generation_id'].lower() in headers, 'Generation header mismatch.')
    outcome = json.loads((directory / 'outcome.json').read_text())
    require(outcome == {'model_id': model['data']['id'], 'generation_id': manifest['generation_id'], 'checksum': 'verified'},
            'Ending does not identify the observed model and verified generation.')
    require((directory / 'complete').is_file(), 'Tape did not finish.')
    return {'checks': 10, 'outcome': outcome, 'generation_id': manifest['generation_id'], 'schema_version': manifest['schema_version'],
            'payload_sha256': sha256(payload), 'payload_size_bytes': payload.stat().st_size}



def verify_story(svg_root, outcome):
    rows = [''.join(row.itertext()) for row in svg_root.findall('.//{http://www.w3.org/2000/svg}text')]
    titles = ['Starmap / Serve a model catalog', '1 / Start the catalog server',
              '2 / Read a model over HTTP', '3 / Fetch and verify a generation',
              'Result / Model read and payload verified']
    positions = []
    for title in titles:
        require(title in rows, 'Recording omits a story segment: ' + title)
        positions.append(rows.index(title))
    require(positions == sorted(positions), 'Recording story segments are out of order.')
    ending = rows[positions[-1]:]
    generation = outcome['generation_id']
    expected = ['Model: ' + outcome['model_id'], 'Generation: ' + generation[:24] + '...' + generation[-12:],
                'Payload checksum: OK', 'Next: configure a catalog subscriber.']
    for line in expected:
        require(line in ending, 'Recording ending omits the verified result: ' + line)
    require(not any(row.startswith('$ touch complete') or row.startswith('$ printf') for row in rows),
            'Recording exposes an internal capture command.')
    return {'checks': 11, 'segments': titles, 'ending_matches_observed_data': True}

def prepare(directory, commit):
    home = directory / 'home'
    home.mkdir()
    binary = directory / 'starmap-source'
    flags = f'-X main.version=demo -X main.commit={commit} -X main.builtBy=record-demo.py'
    run(['go', 'build', '-trimpath', '-ldflags', flags, '-o', str(binary), './cmd/starmap'], cwd=ROOT)
    wrapper = directory / 'bin'
    wrapper.mkdir()
    environment = {'PATH': os.environ['PATH'], 'HOME': str(home), 'LANG': 'en_US.UTF-8', 'TERM': 'xterm-256color',
                   'STARMAP_HOME': str(home / 'starmap'), 'STARMAP_CATALOG_SOURCE': 'embedded',
                   'STARMAP_CATALOG_ACQUISITION_ENABLED': 'false', 'STARMAP_CATALOG_SOURCE_POLL_INTERVAL': '0s',
                   'STARMAP_CATALOG_NETWORK_MODE': 'offline'}
    env_args = ' '.join(shlex.quote(f'{key}={value}') for key, value in environment.items())
    (wrapper / 'starmap').write_text(f'#!/bin/sh\nexec env -i {env_args} {shlex.quote(str(binary))} --quiet "$@"\n')
    (wrapper / 'starmap').chmod(0o755)
    (directory / 'setup.sh').write_text(
        f'export PATH={shlex.quote(str(wrapper))}:"$PATH"\n'
        "export PS1='$ '\nunset PROMPT_COMMAND\nset +m\nset -euo pipefail\n"
        "unset http_proxy https_proxy all_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY\nexport NO_PROXY=127.0.0.1\n"
        "trap 'for demo_pid in $(jobs -pr); do kill \"$demo_pid\" 2>/dev/null || true; done; wait || true' EXIT\n")
    return binary


def preflight(directory, tape):
    for line in tape.splitlines():
        if line.startswith('Type ') and not (line.startswith('Type `') and line.endswith('`')):
            raise ValueError('The demo runner requires backtick strings for Type commands.')
    commands = [line[6:-1] for line in tape.splitlines() if line.startswith('Type `')]
    script = []
    for command in commands:
        script.append("printf '%s\\n' " + shlex.quote('$ ' + command))
        script.append(command)
    transcript = directory / 'transcript.txt'
    try:
        with transcript.open('w') as output:
            run(['bash', '--noprofile', '--norc', '-c', '\n'.join(script)], cwd=directory, stdout=output, stderr=subprocess.STDOUT)
    except subprocess.CalledProcessError:
        print(transcript.read_text())
        raise
    return verify(directory)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true', help='Build and verify the tape without recording media.')
    parser.add_argument('--vhs', default=os.environ.get('VHS', 'vhs'), help='VHS executable with SVG output support.')
    parser.add_argument('--browser-path', default=os.environ.get('VHS_BROWSER_PATH'), help='Capture browser executable.')
    args = parser.parse_args()
    for program in ['go', 'bash', 'curl', 'jq', 'shasum', 'ffprobe']:
        if not shutil.which(program):
            parser.error(f'Missing required program: {program}')
    commit = run(['git', 'rev-parse', 'HEAD'], cwd=ROOT, capture_output=True, text=True).stdout.strip()
    tape = TAPE.read_text()
    catalog = catalog_inputs()
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 18991))
    with tempfile.TemporaryDirectory(prefix='starmap-catalog-demo-') as temporary:
        directory = Path(temporary)
        binary = prepare(directory, commit)
        checked = preflight(directory, tape)
        verify_generation_snapshot(checked, catalog)
        verify_catalog_inputs(catalog, catalog_inputs())
        if args.check:
            print(json.dumps({'status': 'PASS', 'source_commit': commit, 'catalog_inputs': catalog, **checked}, indent=2))
            return
        vhs = shutil.which(args.vhs)
        if not vhs:
            parser.error('VHS is absent. Use the agentstation/vhs fork for SVG output.')
        capture_tool = directory / 'vhs-capture'
        shutil.copy2(vhs, capture_tool)
        # Each recording starts with new product state and repeats the same commands.
        shutil.rmtree(directory / 'home/starmap')
        (directory / 'complete').unlink()
        output = directory / 'scripts'
        output.mkdir()
        (directory / 'demo.tape').write_text(tape)
        capture = [str(capture_tool), 'demo.tape', '--svg-font-file', str(FONT)]
        if args.browser_path:
            capture += ['--browser-path', args.browser_path]
        run(capture, cwd=directory)
        recorded = verify(directory)
        verify_generation_snapshot(recorded, catalog)
        svg = output / 'demo.svg'
        story = verify_story(ET.parse(svg).getroot(), recorded['outcome'])
        require('@font-face' in svg.read_text(), 'SVG does not contain its capture font.')
        duration = re.search(r'animation: slide ([0-9.]+)s', svg.read_text())
        require(duration, 'SVG has no slide animation.')
        svg_seconds = float(duration.group(1))
        probe = run(['ffprobe', '-v', 'error', '-show_entries', 'format=duration:stream=width,height', '-of', 'json', str(output / 'demo.gif')], capture_output=True, text=True)
        media = json.loads(probe.stdout)
        gif_seconds = float(media['format']['duration'])
        require(abs(svg_seconds - gif_seconds) < 1, 'SVG and GIF duration mismatch.')
        require(30 <= svg_seconds <= 60, 'Unexpected demo duration.')
        require(svg.stat().st_size <= 5_000_000, 'SVG exceeds the README asset budget.')
        verify_catalog_inputs(catalog, catalog_inputs())
        destination = ROOT / 'scripts'
        for name in ['demo.svg', 'demo.gif']:
            shutil.copyfile(output / name, destination / name)
        transcript = destination / 'demo-transcript.md'
        plain_output = re.sub(r'\x1b\[[0-?]*[ -/]*[@-~]', '', (directory / 'transcript.txt').read_text())
        transcript.write_text('# Catalog server preflight transcript\n\nThis transcript contains the preflight commands and output.\nThe transcript omits terminal color and screen controls.\nThe recording adds chapter titles, typing, prompt waits, and pauses.\nTitles precede each action and remain above the commands.\nThe runner waits for server readiness before the client reads a model.\nThe ending identifies the observed model and verified generation.\n\nThe catalog combines retained provider observations with a reviewed GPT-6.1 Sol definition.\nThe recording evidence binds the exact working-tree catalog inputs and generation metadata.\n\nThe checksum check verifies payload integrity.\nConsumer configuration, authentication, authority checks, and subscriber activation require separate setup.\nSee [Go consumer configuration](../remote/README.md#Config) or the [Starport central-server guide](https://github.com/agentstation/starport/blob/main/docs/site/operate-starmap/central-server.md#connect-replicas-to-a-central-server).\n\n```text\n' + plain_output + '```\n')
        evidence = {'source_commit': commit, 'version': 'source build (demo)', 'catalog_inputs': catalog, 'tape_sha256': sha256(TAPE),
                    'typing_speed_ms': int(re.search(r'^Set TypingSpeed (\d+)ms$', tape, re.MULTILINE).group(1)),
                    'framerate': int(re.search(r'^Set Framerate (\d+)$', tape, re.MULTILINE).group(1)),
                    'direction': {'chapters': re.findall(r'^# Chapter: (.+)$', tape, re.MULTILINE),
                                  'chapter_holds_seconds': {'start_server': 2.5, 'read_model': 1.8, 'fetch_generation': 1.8},
                                  'title_position': 'above commands',
                                  'opening': 'Serve a model catalog for applications and gateways', 'opening_hold_seconds': 2.5,
                                  'output_holds_seconds': {'readiness': 2, 'model_definition': 3, 'manifest': 2, 'checksum': 3},
                                  'ending': 'Model read and generation payload verified', 'ending_hold_seconds': 5,
                                  'next_step': 'Configure a catalog consumer. Setup and activation are separate steps.',
                                  'readiness': 'Hidden GET /api/v1/ready retries with a 10 second retry budget and 2 second request timeout.'},
                    'runner_sha256': sha256(Path(__file__)),
                    'transcript_sha256': sha256(transcript), 'transcript_kind': 'preflight',
                    'font_sha256': sha256(FONT),
                    'binary_sha256': sha256(binary), 'vhs_sha256': sha256(capture_tool), 'browser_path': args.browser_path,
                    'configuration': {'source': 'embedded', 'acquisition_enabled': False, 'source_poll_interval': '0s', 'network_mode': 'offline'},
                    'preflight': checked, 'recording': recorded, 'story': story, 'svg_duration_seconds': svg_seconds,
                    'gif_duration_seconds': gif_seconds, 'width': media['streams'][0]['width'], 'height': media['streams'][0]['height'],
                    'assets': {name: {'sha256': sha256(destination / name), 'bytes': (destination / name).stat().st_size} for name in ['demo.svg', 'demo.gif']}}
        (destination / 'demo-record.json').write_text(json.dumps(evidence, indent=2) + '\n')
        print(json.dumps({'status': 'PASS', **evidence}, indent=2))


if __name__ == '__main__':
    main()
