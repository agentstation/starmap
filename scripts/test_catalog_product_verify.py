import argparse
import io
import sys
from contextlib import redirect_stdout
import json
import hashlib
import subprocess
import tempfile
import unittest
from copy import deepcopy
from pathlib import Path
from unittest.mock import patch

import catalog_product_verify as verifier
import constructor_network
import cold_server
import native_catalog


class CatalogVerifierTests(unittest.TestCase):
    def setUp(self):
        self.roster = verifier.read_json(verifier.ROSTER)

    def test_verifier_supports_isolated_document_import(self):
        script = ("import importlib.util, sys; "
                  "spec = importlib.util.spec_from_file_location('catalog_verifier', sys.argv[1]); "
                  "module = importlib.util.module_from_spec(spec); spec.loader.exec_module(module); "
                  "module.validate_roster(module.read_json(module.ROSTER))")
        result = subprocess.run([sys.executable, "-I", "-c", script, str(Path(verifier.__file__).resolve())],
                                capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_complete_red_report(self):
        read_json = verifier.read_json
        def without_registered_evidence(path):
            return {"schema_version": 1, "checks": {}} if path == verifier.REGISTRY else read_json(path)
        output = io.StringIO()
        with patch.object(verifier, 'read_json', side_effect=without_registered_evidence), patch.object(sys, 'argv', ['verifier', '--all', '--json']), redirect_stdout(output):
            self.assertEqual(verifier.main(), 1)
        report = json.loads(output.getvalue())
        self.assertEqual(report['summary'], 'Summary: 0 passed, 50 failed')
        self.assertEqual(report['unverified_cases'], 50)
        self.assertEqual(report['selected_subcases'], 324)

    def test_unknown_case_refuses(self):
        args = argparse.Namespace(task=None, gate=None, case=['A99'])
        with self.assertRaises(ValueError):
            verifier.select_checks(args, self.roster)

    def test_duplicate_subcase_refuses(self):
        self.roster['required_subcases']['A01'].append(self.roster['required_subcases']['A01'][0])
        with self.assertRaises(ValueError):
            verifier.validate_roster(self.roster)

    def test_missing_primary_refuses(self):
        del self.roster['required_subcases']['A50']
        with self.assertRaises(ValueError):
            verifier.validate_roster(self.roster)

    def test_empty_task_cannot_pass(self):
        self.roster['task_checks']['CSP0.1'] = []
        with self.assertRaises(ValueError):
            verifier.validate_roster(self.roster)

    def test_final_roster_cannot_omit_a_case(self):
        self.roster['qualification']['final_required_primary_cases'].remove('A50')
        with self.assertRaises(ValueError):
            verifier.validate_roster(self.roster)

    def test_missing_candidate_local_check_refuses(self):
        self.roster['task_checks']['CSP22'].remove('A29.recovery_without_auth')
        with self.assertRaises(ValueError):
            verifier.validate_roster(self.roster)

    def test_partial_parent_never_passes(self):
        local = self.roster['qualification']['candidate_additional_subcases']
        report = verifier.aggregate(self.roster, local, {item: {'status': 'PASS'} for item in local}, False)
        self.assertTrue(all(case['status'] == 'UNVERIFIED' for case in report['cases']))

    def test_qualification_cannot_use_component_passes(self):
        selected = self.roster['task_checks']['CSP22']
        report = verifier.aggregate(self.roster, selected, {item: {'status': 'PASS'} for item in selected}, True)
        self.assertEqual(report['summary'], 'Summary: 43 passed, 7 failed')
        self.assertEqual(report['gate_status'], 'FAIL')
        self.assertEqual(report['qualification'], 'UNVERIFIED')

    def test_missing_registration_is_unverified(self):
        self.assertEqual(verifier.run_check('A01.test', None, {})['status'], 'UNVERIFIED')

    def test_real_named_go_test_executes(self):
        entry = {'kind': 'go_test', 'repository': 'starmap', 'package': './pkg/errors', 'test': 'TestConflictError'}
        result = verifier.run_check('runner-fixture', entry, {'starmap': verifier.ROOT})
        self.assertEqual(result['status'], 'PASS', result)

    def test_real_go_missing_match_is_unverified(self):
        entry = {'kind': 'go_test', 'repository': 'starmap', 'package': './pkg/errors', 'test': 'TestCSPVerifierMissingMatch'}
        result = verifier.run_check('runner-fixture', entry, {'starmap': verifier.ROOT})
        self.assertEqual(result['status'], 'UNVERIFIED', result)

    def test_skipped_child_cannot_pass_parent(self):
        events = [{'Test': 'TestBudget/subcase', 'Action': 'skip'}, {'Test': 'TestBudget', 'Action': 'pass'}]
        output = subprocess.CompletedProcess([], 0, '\n'.join(map(json.dumps, events)), '')
        entry = {'kind': 'go_test', 'repository': 'starmap', 'package': './pkg/errors', 'test': 'TestBudget'}
        with patch.object(verifier.subprocess, 'run', return_value=output):
            result = verifier.run_check('runner-fixture', entry, {'starmap': verifier.ROOT})
        self.assertEqual(result['status'], 'UNVERIFIED')

    def test_command_failure_cannot_pass(self):
        output = subprocess.CompletedProcess([], 1, json.dumps({'Test': 'TestBudget', 'Action': 'pass'}), 'failure')
        entry = {'kind': 'go_test', 'repository': 'starmap', 'package': './pkg/errors', 'test': 'TestBudget'}
        with patch.object(verifier.subprocess, 'run', return_value=output):
            result = verifier.run_check('runner-fixture', entry, {'starmap': verifier.ROOT})
        self.assertEqual(result['status'], 'FAIL')

    def test_combined_check_needs_every_result(self):
        self.assertEqual(verifier.run_check('E01', {'kind': 'all', 'checks': []}, {})['status'], 'FAIL')
        self.assertEqual(verifier.run_check('E01', {'kind': 'all', 'checks': [None]}, {})['status'], 'UNVERIFIED')

    def test_browser_review_requires_current_inputs_and_captures(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'source.tsx').write_text('current source')
            (root / 'capture.png').write_bytes(b'review fixture')
            digest = lambda path: hashlib.sha256((root / path).read_bytes()).hexdigest()
            review = {'schema_version': 1, 'verdict': 'PASS', 'observations': {'reflow_320': True},
                      'inputs': {'source.tsx': digest('source.tsx')},
                      'captures': {'capture.png': digest('capture.png')}}
            (root / 'review.json').write_text(json.dumps(review))
            entry = {'kind': 'reviewed_ui', 'proof': 'review.json', 'repository': 'starport',
                     'required_inputs': ['source.tsx'], 'observations': ['reflow_320']}
            with patch.object(verifier, 'ROOT', root):
                self.assertEqual(verifier.run_check('E01', entry, {'starport': root})['status'], 'PASS')
                (root / 'source.tsx').write_text('changed source')
                self.assertEqual(verifier.run_check('E01', entry, {'starport': root})['status'], 'UNVERIFIED')
                (root / 'source.tsx').write_text('current source')
                (root / 'capture.png').unlink()
                self.assertEqual(verifier.run_check('E01', entry, {'starport': root})['status'], 'UNVERIFIED')


    def performance_profile(self):
        return verifier.read_json(verifier.ROOT / 'docs/plans/proof/starport-production-catalog/csp0.4/numeric-profile.json')

    def performance_baseline(self):
        return verifier.read_json(verifier.ROOT / 'docs/plans/proof/starport-production-catalog/csp0.4/baseline-run-1.json')

    def test_retained_full_http_baseline_satisfies_measurement_contract(self):
        result = verifier.validate_performance_baseline(self.performance_baseline(), 100)
        self.assertEqual(result['warm_pairs'], 200)
        self.assertEqual(result['initial_pairs'], 2)

    def test_baseline_refuses_missing_variant_or_initial_evidence(self):
        for field in ('warm_pairs', 'initial_pairs'):
            report = self.performance_baseline()
            report[field].pop()
            with self.subTest(field=field), self.assertRaises(ValueError):
                verifier.validate_performance_baseline(report, 100)

    def test_baseline_refuses_partial_or_fabricated_timing(self):
        for field, value in [('elapsed_ns', -1), ('first_byte_ns', 0), ('gateway_handler_ns', 0),
                             ('controlled_wait_ns', 0), ('wait_adjusted_elapsed_ns', 0),
                             ('request_bytes', 0), ('client_connection_reused', 'yes')]:
            report = self.performance_baseline()
            report['warm_pairs'][0]['proxied'][field] = value
            with self.subTest(field=field), self.assertRaises(ValueError):
                verifier.validate_performance_baseline(report, 100)

    def test_baseline_keeps_negative_paired_differences(self):
        report = self.performance_baseline()
        pair = report['warm_pairs'][0]
        # Add client delay to the direct sample. A valid negative pair must survive.
        extra = pair['proxied']['elapsed_ns'] + 1_000_000
        pair['direct']['elapsed_ns'] += extra
        pair['direct']['wait_adjusted_elapsed_ns'] += extra
        pair['paired_adjusted_delta_ns'] = pair['proxied']['wait_adjusted_elapsed_ns'] - pair['direct']['wait_adjusted_elapsed_ns']
        self.assertLess(pair['paired_adjusted_delta_ns'], 0)
        verifier.validate_performance_baseline(report, 100)
        pair['paired_adjusted_delta_ns'] = 0
        with self.assertRaises(ValueError):
            verifier.validate_performance_baseline(report, 100)

    def test_baseline_refuses_lost_stream_events_and_wrong_milestones(self):
        for field, value in [('event_forwarding_ns', []), ('first_token_ns', 0)]:
            report = self.performance_baseline()
            pair = next(pair for pair in report['warm_pairs'] if pair['stream'])
            pair['proxied'][field] = value
            with self.subTest(field=field), self.assertRaises(ValueError):
                verifier.validate_performance_baseline(report, 100)

    def test_baseline_cannot_claim_release_qualification(self):
        report = self.performance_baseline()
        report['qualification'] = 'PASS'
        with self.assertRaises(ValueError):
            verifier.validate_performance_baseline(report, 100)

    def test_numeric_profile_is_complete_but_unqualified(self):
        verifier.validate_performance_profile(self.performance_profile())

    def test_numeric_profile_refuses_invalid_limits(self):
        for value in (0, -1, True, float('nan'), float('inf')):
            profile = self.performance_profile()
            profile['resources']['request_allocated_bytes'] = value
            with self.subTest(value=value), self.assertRaises(ValueError):
                verifier.validate_performance_profile(profile)

    def test_numeric_profile_cannot_relax_correctness_for_latency(self):
        for field, value in [('permission_validity_seconds', 600), ('maximum_clock_uncertainty_seconds', 60),
                             ('unknown_required_budget', 'allow'), ('admission_mode', 'unbounded-local'),
                             ('authority_activation_failure', 'use-old-policy'), ('controlled_backend_recovery', False)]:
            profile = self.performance_profile()
            profile['correctness'][field] = value
            with self.subTest(field=field), self.assertRaises(ValueError):
                verifier.validate_performance_profile(profile)

    def test_numeric_profile_cannot_hide_workload_or_uncertainty(self):
        mutations = [('workload', 'credential_sources', ['environment']),
                     ('workload', 'authority_modes', ['public']), ('observability', 'usage_capture', 'off'),
                     ('evidence', 'minimum_samples_per_variant', 100), ('evidence', 'negative_deltas', 'clamp'),
                     ('evidence', 'provider_wait_method', 'whole-connector'), ('evidence', 'final_artifact_binding', False)]
        for group, field, value in mutations:
            profile = self.performance_profile()
            profile[group][field] = value
            with self.subTest(group=group, field=field), self.assertRaises(ValueError):
                verifier.validate_performance_profile(profile)
        profile = self.performance_profile()
        profile['qualification_exercises'] = ['cold-start'] * 13
        with self.assertRaises(ValueError):
            verifier.validate_performance_profile(profile)

    def test_numeric_profile_refuses_contradictory_resources(self):
        profile = self.performance_profile()
        profile['resources']['replica_live_heap_bytes'] = profile['resources']['replica_rss_bytes'] + 1
        with self.assertRaises(ValueError):
            verifier.validate_performance_profile(profile)

    def test_performance_review_requires_current_targets_and_retained_evidence(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'profile.json').write_text(json.dumps(self.performance_profile()))
            (root / 'baseline.json').write_text(json.dumps(self.performance_baseline()))
            digest = lambda name: hashlib.sha256((root / name).read_bytes()).hexdigest()
            names = ('timing_boundaries', 'latency_and_workload', 'resources_and_deadlines', 'correctness',
                     'qualification_method', 'baseline_costs', 'no_current_release_claim')
            review = {'schema_version': 1, 'verdict': 'PASS', 'profile_sha256': digest('profile.json'),
                      'observations': dict.fromkeys(names, True), 'assessment': dict.fromkeys(names, 'Unit-test fixture assessment.'),
                      'evidence': {'baseline.json': digest('baseline.json')}}
            (root / 'review.json').write_text(json.dumps(review))
            entry = {'kind': 'performance_profile', 'repository': 'starport', 'profile': 'profile.json', 'proof': 'review.json'}
            with patch.object(verifier, 'ROOT', root):
                self.assertEqual(verifier.run_check('profile', entry, {'starport': root})['status'], 'PASS')
                original = (root / 'profile.json').read_bytes()
                (root / 'profile.json').write_bytes(original + b' ')
                self.assertEqual(verifier.run_check('profile', entry, {'starport': root})['status'], 'UNVERIFIED')
                (root / 'profile.json').write_bytes(original)
                (root / 'baseline.json').unlink()
                self.assertEqual(verifier.run_check('profile', entry, {'starport': root})['status'], 'UNVERIFIED')

    def test_evidence_paths_cannot_escape_repository(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ('../outside', str(root.parent / 'outside')):
                with self.subTest(name=name), self.assertRaises(ValueError):
                    verifier.contained_path(root, name)

    def test_fresh_baseline_adapter_cannot_pass_skipped_test(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = root / 'internal/app/performance_test.go'
            path.parent.mkdir(parents=True)
            path.touch()
            events = [{'Test': 'TestFullPathMeasurementBaseline', 'Action': 'skip'}]
            result = subprocess.CompletedProcess([], 0, '\n'.join(map(json.dumps, events)), '')
            with patch.object(verifier.subprocess, 'run', return_value=result):
                checked = verifier.run_performance_baseline({'repository': 'starport'}, {'starport': root})
            self.assertEqual(checked['status'], 'UNVERIFIED')


class FirstUseReviewTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        (self.root / 'README.md').write_text('reviewed README')
        (self.root / 'native.json').write_text('{"exit_code": 0}')
        self.inference = {'verdict': 'PASS', 'release': 'v1.2.0', 'response_status': 200,
                          'content_type': 'text/event-stream', 'request': {'stream': True},
                          'stream': 'data: {"choices":[{"delta":{"content":"Hello"}}]}\n\ndata: [DONE]\n\n'}
        (self.root / 'inference.json').write_text(json.dumps(self.inference))
        self.review = {'schema_version': 1, 'verdict': 'PASS', 'release': 'v1.2.0',
                       'observations': {'catalog_before_keys': True},
                       'inputs': {'README.md': self.digest('README.md')},
                       'captures': {name: self.digest(name) for name in ['native.json', 'inference.json']},
                       'inference_capture': 'inference.json',
                       'methods': {'archive': {'verdict': 'PASS', 'native': True, 'platform': 'linux/arm64',
                                              'artifact_kind': 'release', 'release': 'v1.2.0', 'captures': ['native.json']}}}
        self.entry = {'kind': 'reviewed_first_use', 'proof': 'review.json', 'repository': 'starport',
                      'required_inputs': ['README.md'], 'observations': ['catalog_before_keys'], 'methods': ['archive']}

    def digest(self, name):
        return hashlib.sha256((self.root / name).read_bytes()).hexdigest()

    def check(self):
        (self.root / 'review.json').write_text(json.dumps(self.review))
        with patch.object(verifier, 'ROOT', self.root):
            return verifier.run_check('E02', self.entry, {'starport': self.root})['status']

    def test_complete_review_passes(self):
        self.assertEqual(self.check(), 'PASS')

    def test_changed_readme_requires_review(self):
        (self.root / 'README.md').write_text('new installer')
        self.assertEqual(self.check(), 'UNVERIFIED')

    def test_missing_observation_refuses(self):
        self.review['observations'].clear()
        self.assertEqual(self.check(), 'UNVERIFIED')

    def test_unqualified_or_missing_method_refuses(self):
        method = self.review['methods']['archive']
        for field, bad in [('verdict', 'FAIL'), ('native', False), ('platform', ''),
                           ('release', 'v1.1.0'), ('captures', []), ('captures', ['missing.json']),
                           ('artifact_kind', 'unknown')]:
            before = method[field]
            method[field] = bad
            with self.subTest(field=field, bad=bad):
                self.assertEqual(self.check(), 'UNVERIFIED')
            method[field] = before
        self.review['methods'].clear()
        self.assertEqual(self.check(), 'UNVERIFIED')

    def test_source_build_needs_exact_commit(self):
        method = self.review['methods']['archive']
        method['artifact_kind'] = 'source'
        self.assertEqual(self.check(), 'UNVERIFIED')
        method['source_commit'] = 'a' * 40
        self.assertEqual(self.check(), 'PASS')

    def test_changed_or_missing_capture_refuses(self):
        (self.root / 'native.json').write_text('{"exit_code": 1}')
        self.assertEqual(self.check(), 'UNVERIFIED')
        (self.root / 'native.json').unlink()
        self.assertEqual(self.check(), 'UNVERIFIED')

    def test_incomplete_inference_refuses_even_with_matching_capture_digest(self):
        for stream in ['', 'data: [DONE]\n', 'data: {"choices":[]}\n\ndata: [DONE]\n',
                       'data: {"choices":[{"delta":{"content":"Hello"}}]}\n']:
            self.inference['stream'] = stream
            (self.root / 'inference.json').write_text(json.dumps(self.inference))
            self.review['captures']['inference.json'] = self.digest('inference.json')
            with self.subTest(stream=stream):
                self.assertEqual(self.check(), 'UNVERIFIED')

    def test_evidence_cannot_escape_its_root(self):
        self.review['inputs'] = {'../README.md': 'a' * 64, 'README.md': self.digest('README.md')}
        self.assertEqual(self.check(), 'UNVERIFIED')


class DemoReviewTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        (self.root / 'README.md').write_text('fixture')
        (self.root / 'assets').mkdir()
        # The fixture covers identity and header checks. Human review owns visual quality.
        data = b'GIF89a' + (1280).to_bytes(2, 'little') + (800).to_bytes(2, 'little')
        outputs = {}
        for name in ['first-use.gif', 'first-use-uncut.gif']:
            (self.root / 'assets' / name).write_bytes(data)
            outputs[name] = {'sha256': hashlib.sha256(data).hexdigest(), 'bytes': len(data)}
        self.capture = {'verdict': 'PASS', 'release': 'v1.2.0', 'response_status': 200,
                        'stream_events': [{'data': '{"choices":[{"delta":{"content":"Hello"}}]}'}, {'data': '[DONE]'}],
                        'events': [{'seconds': 0}, {'seconds': 4}, {'seconds': 7}],
                        'persistent_selectors_present': [], 'remaining_home_files': [],
                        'catalog_environment_has_provider_key': False, 'shutdown_exit_code': 0, 'scratch_removed': True,
                        'inference_start_seconds': 5, 'inference_end_seconds': 7}
        self.render = {'width': 1280, 'height': 800, 'effective_font_at_900px': 18,
                       'edits': [{'before_event': 1, 'original_gap_seconds': 4, 'edited_gap_seconds': 2}], 'outputs': outputs}
        self.entry = {'kind': 'reviewed_demo', 'repository': 'starport', 'proof': 'review.json',
                      'asset_directory': 'assets', 'required_inputs': ['README.md'], 'observations': ['readable']}

    def check(self):
        (self.root / 'capture.json').write_text(json.dumps(self.capture))
        digest = lambda name: hashlib.sha256((self.root / name).read_bytes()).hexdigest()
        self.render['capture_sha256'] = digest('capture.json')
        (self.root / 'render.json').write_text(json.dumps(self.render))
        review = {'schema_version': 1, 'verdict': 'PASS', 'release': 'v1.2.0', 'capture': 'capture.json',
                  'render': 'render.json', 'observations': {'readable': True},
                  'inputs': {'README.md': digest('README.md')},
                  'captures': {name: digest(name) for name in ['capture.json', 'render.json']}}
        (self.root / 'review.json').write_text(json.dumps(review))
        with patch.object(verifier, 'ROOT', self.root):
            return verifier.run_check('E03', self.entry, {'starport': self.root})['status']

    def test_coherent_review_passes(self):
        self.assertEqual(self.check(), 'PASS')

    def test_edits_cannot_intersect_inference(self):
        self.render['edits'] = [{'before_event': 2, 'original_gap_seconds': 3, 'edited_gap_seconds': 1}]
        self.assertEqual(self.check(), 'UNVERIFIED')

    def test_false_gap_or_negative_duration_refuses(self):
        self.render['edits'][0]['original_gap_seconds'] = 10
        self.assertEqual(self.check(), 'UNVERIFIED')
        self.render['edits'][0]['original_gap_seconds'] = 4
        self.render['edits'][0]['edited_gap_seconds'] = -1
        self.assertEqual(self.check(), 'UNVERIFIED')

    def test_incomplete_stream_or_cleanup_refuses(self):
        self.capture['stream_events'] = []
        self.assertEqual(self.check(), 'UNVERIFIED')
        self.capture['stream_events'] = [{'data': '[DONE]'}]
        self.assertEqual(self.check(), 'UNVERIFIED')
        self.capture['stream_events'] = [{'data': '{"choices":[{"delta":{"content":"Hello"}}]}'}, {'data': '[DONE]'}]
        self.capture['remaining_home_files'] = ['retained.db']
        self.assertEqual(self.check(), 'UNVERIFIED')

    def test_unreadable_dimensions_or_changed_gif_refuses(self):
        self.render['width'] = 500
        self.assertEqual(self.check(), 'UNVERIFIED')
        self.render['width'] = 1280
        self.render['height'] = 900
        self.assertEqual(self.check(), 'UNVERIFIED')
        self.render['height'] = 800
        (self.root / 'assets/first-use.gif').write_bytes(b'changed')
        self.assertEqual(self.check(), 'UNVERIFIED')


class ConstructorNetworkTests(unittest.TestCase):
    def setUp(self):
        self.payload = {"constructors": constructor_network.CONSTRUCTORS, "os": "linux", "arch": "arm64",
                        "files_after": 0, "generation_id": "embedded-generation", "payload_checksum": "sha256:" + "a" * 64}
        self.read_only = {"exit_code": 0, "stdout": json.dumps(self.payload),
                          "state": {"ExitCode": 0, "OOMKilled": False, "Error": ""}}
        self.attempted = {"exit_code": 159, "stdout": '{"phase":"network-attempt"}',
                          "state": {"ExitCode": 159, "OOMKilled": False, "Error": ""}}

    def check(self):
        return constructor_network.classify(self.read_only, self.attempted, "arm64")[0]

    def test_both_controls_are_required(self):
        self.assertEqual(self.check(), "PASS")
        for code in [0, 1, 137]:
            with self.subTest(code=code):
                self.attempted["exit_code"] = code
                self.attempted["state"]["ExitCode"] = code
                self.assertEqual(self.check(), "UNVERIFIED")

    def test_control_must_reach_its_socket_attempt(self):
        for output in ["", "{}", "null", '{"phase":"before-main"}']:
            with self.subTest(output=output):
                self.attempted["stdout"] = output
                self.assertEqual(self.check(), "UNVERIFIED")

    def test_resource_failure_cannot_prove_network_enforcement(self):
        for name in ["read_only", "attempted"]:
            for field, value in [("OOMKilled", True), ("Error", "container failed")]:
                with self.subTest(name=name, field=field):
                    original = deepcopy(getattr(self, name))
                    getattr(self, name)["state"][field] = value
                    self.assertEqual(self.check(), "UNVERIFIED")
                    setattr(self, name, original)

    def test_constructor_socket_attempt_is_a_failure(self):
        self.read_only.update(exit_code=159, stdout="")
        self.read_only["state"]["ExitCode"] = 159
        self.assertEqual(self.check(), "FAIL")

    def test_missing_constructor_evidence_refuses(self):
        for output in ["", "[]", "null", "invalid"]:
            with self.subTest(output=output):
                self.read_only["stdout"] = output
                self.assertEqual(self.check(), "UNVERIFIED")

    def test_incomplete_baseline_or_changed_platform_refuses(self):
        for field, value in [("constructors", ["New"]), ("files_after", 1), ("files_after", False),
                             ("generation_id", ""), ("generation_id", True), ("payload_checksum", "missing"),
                             ("os", "darwin"), ("arch", "amd64")]:
            with self.subTest(field=field, value=value):
                payload = dict(self.payload, **{field: value})
                self.read_only["stdout"] = json.dumps(payload)
                self.assertEqual(self.check(), "FAIL")

    def test_missing_container_state_refuses(self):
        self.attempted.pop("state")
        self.assertEqual(self.check(), "UNVERIFIED")

    def test_missing_docker_is_unverified(self):
        with patch.object(constructor_network.subprocess, "run", side_effect=FileNotFoundError("docker")):
            result = constructor_network.verify(Path(__file__).resolve().parents[1])
        self.assertEqual(result["status"], "UNVERIFIED")
        self.assertTrue(result["cleanup_complete"])


class ColdServerTests(unittest.TestCase):
    def setUp(self):
        manifest = {"generation_id": "baseline", "payload": {"checksum": "sha256:" + "a" * 64, "size_bytes": 1024}}
        item = {"phase": "cold", "isolation": {"network_mode": "none", "user": "65532:65532", "readonly_rootfs": True,
                "privileged": False, "environment_names": ["HOME", "STARMAP_HOME", "PATH"], "volume": "private"},
                "network": {"denied": True}, "exit_state": {"ExitCode": 0, "Running": False},
                "ready": {"status_code": 200, "body": {"data": {"runtime": {"usable": True, "source_kind": "public",
                    "fallback": True, "generation_id": "baseline", "instance_identity": "instance"}}}},
                "manifest": {"status_code": 200, "manifest_validated": True, "body": manifest, "generation_header": "baseline"},
                "baseline_manifest": {"manifest_validated": True, "body": deepcopy(manifest), "mode": "0600"},
                "payload": {"status_code": 200, "checksum": manifest["payload"]["checksum"], "bytes": 1024, "generation_header": "baseline"},
                "baseline_payload": {"checksum": manifest["payload"]["checksum"], "bytes": 1024, "mode": "0600"}}
        self.observations = [item, deepcopy(item)]
        self.observations[1]["phase"] = "restart"

    def test_complete_offline_restart_passes(self):
        self.assertEqual(cold_server.classify(self.observations)[0], "PASS")

    def test_both_observations_and_schema_validation_are_required(self):
        for observations in [[], self.observations[:1], [None, None]]:
            with self.subTest(observations=observations):
                self.assertEqual(cold_server.classify(observations)[0], "UNVERIFIED")
        self.observations[1]["baseline_manifest"].pop("manifest_validated")
        self.assertEqual(cold_server.classify(self.observations)[0], "UNVERIFIED")

    def test_external_network_or_credentials_cannot_qualify(self):
        for field, value in [("network_mode", "bridge"), ("user", "root"), ("privileged", True),
                             ("readonly_rootfs", False), ("environment_names", ["HOME", "STARMAP_HOME", "OPENAI_API_KEY"])]:
            with self.subTest(field=field):
                observations = deepcopy(self.observations)
                observations[0]["isolation"][field] = value
                self.assertEqual(cold_server.classify(observations)[0], "UNVERIFIED")

    def test_failed_endpoints_and_invalid_bytes_fail(self):
        cases = [("ready", "status_code", 503), ("payload", "bytes", 1), ("payload", "bytes", True),
                 ("payload", "checksum", "sha256:" + "b" * 64), ("payload", "generation_header", "other"),
                 ("baseline_manifest", "mode", "0644"), ("baseline_payload", "mode", "0644")]
        for name, field, value in cases:
            with self.subTest(name=name, field=field):
                observations = deepcopy(self.observations)
                observations[0][name][field] = value
                self.assertEqual(cold_server.classify(observations)[0], "FAIL")

    def test_restart_identity_and_manifest_changes_fail(self):
        for variant in ["identity", "volume", "manifest"]:
            with self.subTest(variant=variant):
                observations = deepcopy(self.observations)
                if variant == "identity":
                    observations[1]["ready"]["body"]["data"]["runtime"]["instance_identity"] = "other"
                elif variant == "volume":
                    observations[1]["isolation"]["volume"] = "other"
                else:
                    observations[1]["baseline_manifest"]["body"]["generation_id"] = "other"
                self.assertEqual(cold_server.classify(observations)[0], "FAIL")

    def test_missing_instance_identity_cannot_qualify(self):
        for value in ["", None, True]:
            with self.subTest(value=value):
                observations = deepcopy(self.observations)
                for item in observations:
                    item["ready"]["body"]["data"]["runtime"]["instance_identity"] = value
                self.assertEqual(cold_server.classify(observations)[0], "UNVERIFIED")

    def test_abnormal_exit_and_resource_failure_are_distinct(self):
        self.observations[1]["exit_state"]["ExitCode"] = 137
        self.assertEqual(cold_server.classify(self.observations)[0], "FAIL")
        self.observations[1]["exit_state"]["OOMKilled"] = True
        self.assertEqual(cold_server.classify(self.observations)[0], "UNVERIFIED")

    def test_missing_docker_is_unverified(self):
        with patch.object(cold_server.subprocess, "run", side_effect=FileNotFoundError("docker")):
            result = cold_server.verify(Path(__file__).resolve().parents[1])
        self.assertEqual(result["status"], "UNVERIFIED")
        self.assertTrue(result["cleanup_complete"])

    def test_build_failure_attempts_cleanup_and_cleanup_failure_fails(self):
        for cleanup_code in [0, 1]:
            calls = []
            def run(args, **kwargs):
                calls.append(args)
                output, code = "", 0
                if args[:2] == ["docker", "version"]:
                    output = json.dumps({"Os": "linux", "Arch": "arm64"})
                elif args[:2] == ["go", "build"]:
                    Path(args[args.index("-o") + 1]).write_bytes(b"fixture binary")
                elif args[:2] == ["docker", "build"]:
                    code = 1
                elif args[:3] == ["docker", "image", "rm"]:
                    code = cleanup_code
                return subprocess.CompletedProcess(args, code, output, "")
            with self.subTest(cleanup_code=cleanup_code), patch.object(cold_server.subprocess, "run", side_effect=run):
                result = cold_server.verify(Path(__file__).resolve().parents[1])
            self.assertEqual(result["status"], "FAIL" if cleanup_code else "UNVERIFIED")
            self.assertEqual(result["cleanup_complete"], cleanup_code == 0)
            self.assertTrue(any(args[:3] == ["docker", "image", "rm"] for args in calls))

    def test_registered_adapter_requires_probe_and_calls_verifier(self):
        entry = {"kind": "cold_server", "repository": "starmap"}
        self.assertEqual(verifier.run_check("A01.starmap_cold_offline", entry, {})["status"], "UNVERIFIED")
        class Adapter:
            @staticmethod
            def verify(root):
                return {"status": "PASS", "root": str(root)}
        root = Path(__file__).resolve().parents[1]
        with patch.object(verifier.importlib.util, "module_from_spec", return_value=Adapter), patch("importlib.machinery.SourceFileLoader.exec_module"):
            self.assertEqual(verifier.run_check("A01.starmap_cold_offline", entry, {"starmap": root})["status"], "PASS")


class NativeCatalogTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.test = {"package": "github.com/agentstation/starmap/runtime", "test": "TestNativeContract"}
        self.proof = {"run": {"databaseId": 123, "url": "https://github.com/agentstation/starmap/actions/runs/123",
                              "status": "completed", "conclusion": "success", "headSha": "a" * 40, "jobs": []}, "sha256": {}}
        for arch, runner in native_catalog.RUNNERS["windows"].items():
            self.proof["run"]["jobs"].append({"databaseId": len(self.proof["run"]["jobs"]) + 1,
                                            "name": f"Runtime {runner}", "status": "completed", "conclusion": "success"})
            prefix = f"native-runtime-{runner}/"
            self.write(prefix + "toolchain.txt", f"go version go1.25.12 windows/{arch}\nwindows\n{arch}\nwindows\n{arch}\n0\n")
            events = [{"Package": self.test["package"], "Test": self.test["test"], "Action": action} for action in ("run", "pass")]
            events.append({"Package": self.test["package"], "Action": "pass"})
            self.write(prefix + "tests.jsonl", "\n".join(map(json.dumps, events)))

    def write(self, name, data):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(data)
        self.proof["sha256"][name] = hashlib.sha256(path.read_bytes()).hexdigest()

    def validate(self):
        return native_catalog.validate_platform(self.root, self.proof, "windows", [self.test])

    def test_both_native_architectures_are_required(self):
        self.assertEqual({item["architecture"] for item in self.validate()}, {"amd64", "arm64"})
        self.proof["run"]["jobs"].pop()
        with self.assertRaises(ValueError):
            self.validate()

    def test_duplicate_native_jobs_refuse(self):
        self.proof["run"]["jobs"].append(deepcopy(self.proof["run"]["jobs"][0]))
        with self.assertRaises(ValueError):
            self.validate()

    def test_failed_or_incomplete_workflow_refuses(self):
        for status, conclusion in [("in_progress", ""), ("completed", "failure"), ("completed", "cancelled")]:
            with self.subTest(status=status, conclusion=conclusion):
                self.proof["run"].update(status=status, conclusion=conclusion)
                with self.assertRaises(ValueError):
                    self.validate()

    def test_other_repository_refuses(self):
        self.proof["run"]["url"] = "https://github.com/another/repository/actions/runs/123"
        with self.assertRaises(ValueError):
            self.validate()

    def test_cross_compiled_or_changed_toolchain_refuses(self):
        name = "native-runtime-windows-2025/toolchain.txt"
        original = (self.root / name).read_text()
        for changed in [original.replace("1.25.12", "1.26.6"), original.replace("\nwindows\namd64\n0", "\nlinux\namd64\n0")]:
            self.write(name, changed)
            with self.assertRaises(ValueError):
                self.validate()

    def test_changed_evidence_bytes_refuse(self):
        path = self.root / "native-runtime-windows-2025/tests.jsonl"
        path.write_text(path.read_text() + "\n")
        with self.assertRaises(ValueError):
            self.validate()

    def test_missing_test_or_package_completion_refuses(self):
        name = "native-runtime-windows-2025/tests.jsonl"
        original = (self.root / name).read_text()
        events = list(map(json.loads, original.splitlines()))
        for changed in [events[1:], events[:-1], [events[-1]], events + [events[1]]]:
            self.write(name, "\n".join(map(json.dumps, changed)))
            with self.assertRaises(ValueError):
                self.validate()

    def test_skipped_or_failed_child_refuses(self):
        name = "native-runtime-windows-2025/tests.jsonl"
        original = (self.root / name).read_text()
        for action in ["skip", "fail"]:
            child = {"Package": self.test["package"], "Test": self.test["test"] + "/child", "Action": action}
            self.write(name, original + "\n" + json.dumps(child))
            with self.assertRaises(ValueError):
                self.validate()

    def test_absent_capture_is_unverified(self):
        self.assertEqual(native_catalog.verify(self.root, {"platform": "windows", "tests": [self.test]})["status"], "UNVERIFIED")

    def test_linux_requires_actual_administrator_owned_read(self):
        events = (self.root / "native-runtime-windows-2025/tests.jsonl").read_text()
        events += "\n" + json.dumps({"Package": "github.com/agentstation/starmap/internal/privatefiles", "Test": "TestServiceConfigurationAdministratorOwnedRead", "Action": "skip"})
        owner = "=== RUN   TestServiceConfigurationAdministratorOwnedRead\n--- PASS: TestServiceConfigurationAdministratorOwnedRead (0.00s)\nPASS\n"
        for arch, runner in native_catalog.RUNNERS["linux"].items():
            self.proof["run"]["jobs"].append({"databaseId": len(self.proof["run"]["jobs"]) + 1,
                                            "name": f"Runtime {runner}", "status": "completed", "conclusion": "success"})
            prefix = f"native-runtime-{runner}/"
            self.write(prefix + "toolchain.txt", f"go version go1.25.12 linux/{arch}\nlinux\n{arch}\nlinux\n{arch}\n0\n")
            self.write(prefix + "tests.jsonl", events)
            self.write(prefix + "service-owner.txt", owner)
        self.assertEqual(len(native_catalog.validate_platform(self.root, self.proof, "linux", [self.test])), 2)
        for changed in ["PASS\n", owner.replace("--- PASS:", "--- SKIP:")]:
            self.write("native-runtime-ubuntu-24.04/service-owner.txt", changed)
            with self.assertRaises(ValueError):
                native_catalog.validate_platform(self.root, self.proof, "linux", [self.test])

    def test_source_and_test_changes_invalidate_evidence(self):
        repository = self.root / "repository"
        repository.mkdir()
        native_catalog.command(["git", "init", "-q"], repository)
        source = repository / "main.go"
        source.write_text("package main\n")
        native_catalog.command(["git", "add", "main.go"], repository)
        native_catalog.command(["git", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
                                "-c", "commit.gpgsign=false", "commit", "-qm", "fixture"], repository)
        revision = native_catalog.command(["git", "rev-parse", "HEAD"], repository).strip()
        native_catalog.unchanged_source(repository, revision)
        proof = repository / "docs/plans/evidence.json"
        proof.parent.mkdir(parents=True)
        proof.write_text("{}")
        native_catalog.unchanged_source(repository, revision)
        source.write_text("package changed\n")
        with self.assertRaises(ValueError):
            native_catalog.unchanged_source(repository, revision)
        source.write_text("package main\n")
        (repository / "new_test.go").write_text("package main\n")
        with self.assertRaises(ValueError):
            native_catalog.unchanged_source(repository, revision)


if __name__ == '__main__':
    unittest.main(verbosity=2)
