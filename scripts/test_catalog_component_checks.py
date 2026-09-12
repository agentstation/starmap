import io
import json
import sys
import unittest
from contextlib import redirect_stdout
from unittest.mock import patch

import catalog_product_verify as verifier


class ComponentCheckBoundaryTests(unittest.TestCase):
    def setUp(self):
        self.roster = verifier.read_json(verifier.ROSTER)
        self.identity = 'A22.inference_separate_egress'
        self.producer = {'kind': 'go_test', 'repository': 'starmap', 'package': './pkg/catalogs/config',
                         'test': 'TestOfflineCatalogLeavesCallerInferenceTransportAvailable'}
        self.consumer = {'kind': 'go_test', 'repository': 'starport', 'package': './internal/catalog',
                         'test': 'TestConsumerBoundaryFixture'}
        self.registry = {'schema_version': 1, 'checks': {self.identity: self.consumer},
                         'task_component_checks': {'CSP5': {self.identity: self.producer}}}

    def invoke(self, *arguments):
        def load(path):
            return self.registry if path == verifier.REGISTRY else self.roster

        def run(identity, entry, roots):
            if entry is None:
                return {'status': 'UNVERIFIED'}
            return {'status': 'PASS', 'repository': entry['repository']}

        output = io.StringIO()
        with patch.object(verifier, 'read_json', side_effect=load), patch.object(verifier, 'run_check', side_effect=run), \
                patch.object(sys, 'argv', ['verifier', *arguments, '--json']), redirect_stdout(output):
            code = verifier.main()
        return code, json.loads(output.getvalue())

    def test_producer_task_uses_its_component_without_qualifying_parent(self):
        _, report = self.invoke('--task', 'CSP5')
        result = report['subcases'][self.identity]
        self.assertEqual(result['repository'], 'starmap')
        self.assertEqual(result['evidence_scope'], 'producer_component')
        self.assertEqual(next(case for case in report['cases'] if case['id'] == 'A22')['status'], 'UNVERIFIED')

    def test_consumer_and_qualification_commands_use_consumer_evidence(self):
        for arguments in [('--task', 'CSP8'), ('--case', 'A22'), ('--gate', 'candidate'),
                          ('--gate', 'final'), ('--all',), ('--task', 'CSP5', '--released-assets'),
                          ('--task', 'CSP5', '--recipes'), ('--task', 'CSP5', '--backends', 'primary')]:
            with self.subTest(arguments=arguments):
                _, report = self.invoke(*arguments)
                result = report['subcases'][self.identity]
                self.assertEqual(result['repository'], 'starport')
                self.assertNotEqual(result.get('evidence_scope'), 'producer_component')

    def test_missing_consumer_never_uses_producer_fallback(self):
        self.registry['checks'] = {}
        for arguments in [('--task', 'CSP8'), ('--case', 'A22'), ('--gate', 'candidate'), ('--gate', 'final')]:
            with self.subTest(arguments=arguments):
                code, report = self.invoke(*arguments)
                self.assertEqual(code, 1)
                self.assertEqual(report['subcases'][self.identity]['status'], 'UNVERIFIED')

    def test_component_registry_refuses_unapproved_task_or_case(self):
        for task, identity in [('CSP8', self.identity), ('CSP22', self.identity), ('CSP5', 'A19.pin_restart'),
                               ('CSP5', 'A99.unknown')]:
            with self.subTest(task=task, identity=identity):
                self.registry['task_component_checks'] = {task: {identity: self.producer}}
                code, report = self.invoke('--task', 'CSP5')
                self.assertEqual(code, 2)
                self.assertEqual(report['gate_status'], 'FAIL')

    def test_component_results_cannot_make_primary_case_pass(self):
        selected = self.roster['required_subcases']['A22']
        results = {identity: {'status': 'PASS', 'evidence_scope': 'producer_component'} for identity in selected}
        report = verifier.aggregate(self.roster, selected, results, False)
        self.assertEqual(report['gate_status'], 'PASS')
        self.assertEqual(next(case for case in report['cases'] if case['id'] == 'A22')['status'], 'UNVERIFIED')


if __name__ == '__main__':
    unittest.main()
