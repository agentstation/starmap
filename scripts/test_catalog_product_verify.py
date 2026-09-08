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


if __name__ == '__main__':
    unittest.main(verbosity=2)
