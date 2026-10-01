"""Offline normalization contracts; fixtures are data, never scanner invocations."""
import argparse
import contextlib
import copy
import importlib.util
import io
import json
from pathlib import Path
import random
import tempfile
import unittest

spec = importlib.util.spec_from_file_location(
    'security_normalize_subject', Path(__file__).with_name('security_local_ci.py'))
ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ci)


class NormalizeContracts(unittest.TestCase):
    def normalized(self, entries, separator='\n', prefix='', suffix=''):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / 'govuln.json'
            source.write_text(prefix + separator.join(json.dumps(entry) for entry in entries) + suffix, encoding='utf-8')
            args = argparse.Namespace(govulncheck=str(source), label='fixture-root', module_dir='.',
                                      out_cdx=str(root / 'out' / 'evidence.cdx.json'),
                                      out_findings=str(root / 'out' / 'findings.json'))
            code = ci.normalize(args)
            return code, json.loads(Path(args.out_cdx).read_text()), json.loads(Path(args.out_findings).read_text())

    def assert_schema(self, cdx, findings):
        self.assertEqual(cdx['bomFormat'], 'CycloneDX')
        self.assertEqual(cdx['specVersion'], '1.6')
        self.assertEqual(cdx['version'], 1)
        self.assertEqual(cdx['metadata'], {'component': {'type': 'application', 'name': 'fixture-root'},
                                         'properties': [{'name': 'asynq:evidence-source', 'value': 'govulncheck'},
                                                        {'name': 'asynq:alias-resolved', 'value': 'true'}]})
        self.assertEqual(findings['schema'], 'asynq-govulncheck-findings/v1')
        self.assertEqual(findings['label'], 'fixture-root')
        self.assertEqual(findings['module_dir'], '.')
        refs = {component['bom-ref'] for component in cdx['components']}
        self.assertEqual(len(refs), len(cdx['components']))
        for vulnerability in cdx['vulnerabilities']:
            for affect in vulnerability['affects']:
                self.assertIn(affect['ref'], refs)

    def test_independent_alias_reachability_union_and_wire_schema(self):
        entries = [
            {'osv': {'id': 'GO-A', 'aliases': ['cve-2026-10001', 'CVE-2026-10002', 'CVE-2026-10001', 'GHSA-not-cve', None], 'summary': 'alias fixture'}},
            {'osv': {'id': 'GO-B'}},
            {'finding': {'osv': 'GO-A', 'trace': [{'module': 'example.org/a', 'version': 'v1.0.0', 'package': 'pkg/z'}]}},
            {'finding': {'osv': 'GO-A', 'fixed_version': 'v1.1.0', 'trace': [None, {'module': 'example.org/a', 'version': 'v1.0.0', 'package': 'pkg/a', 'function': 'Invoke'}, {'package': 'pkg/shared', 'function': 'Retry'}]}},
            {'finding': {'osv': 'GO-A', 'trace': [{'module': 'example.org/b', 'version': 'v2.0.0'}]}},
            {'finding': {'osv': 'GO-B', 'trace': [{'module': 'example.org/a', 'version': 'v1.0.0', 'package': 'pkg/b'}]}},
        ]
        code, cdx, receipt = self.normalized(entries, separator='')
        self.assertEqual(code, 0)
        self.assert_schema(cdx, receipt)
        self.assertEqual(receipt['advisory_count'], 2)
        self.assertEqual(receipt['findings'], [
            {'advisory_id': 'GO-A', 'cve_ids': ['CVE-2026-10001', 'CVE-2026-10002'], 'summary': 'alias fixture', 'module': 'example.org/a', 'version': 'v1.0.0', 'fixed_version': 'v1.1.0', 'reachability': 'called', 'packages': ['pkg/a', 'pkg/shared', 'pkg/z'], 'symbols': ['Invoke', 'Retry']},
            {'advisory_id': 'GO-B', 'cve_ids': [], 'summary': '', 'module': 'example.org/a', 'version': 'v1.0.0', 'fixed_version': '', 'reachability': 'imported-not-called', 'packages': ['pkg/b'], 'symbols': []},
            {'advisory_id': 'GO-A', 'cve_ids': ['CVE-2026-10001', 'CVE-2026-10002'], 'summary': 'alias fixture', 'module': 'example.org/b', 'version': 'v2.0.0', 'fixed_version': '', 'reachability': 'required-not-imported', 'packages': [], 'symbols': []},
        ])
        self.assertEqual(cdx['components'], [
            {'bom-ref': 'pkg:golang/example.org/a@v1.0.0', 'type': 'library', 'name': 'example.org/a', 'version': 'v1.0.0', 'purl': 'pkg:golang/example.org/a@v1.0.0'},
            {'bom-ref': 'pkg:golang/example.org/b@v2.0.0', 'type': 'library', 'name': 'example.org/b', 'version': 'v2.0.0', 'purl': 'pkg:golang/example.org/b@v2.0.0'},
        ])
        # Explicit expected routes: aliases retain GO traceability, and the same
        # advisory in two modules must keep both component references.
        expected = [
            ('CVE-2026-10001', 'pkg:golang/example.org/a@v1.0.0', 'v1.0.0', ['GO-A', 'CVE-2026-10002']),
            ('CVE-2026-10002', 'pkg:golang/example.org/a@v1.0.0', 'v1.0.0', ['GO-A', 'CVE-2026-10001']),
            ('GO-B', 'pkg:golang/example.org/a@v1.0.0', 'v1.0.0', ['GO-B']),
            ('CVE-2026-10001', 'pkg:golang/example.org/b@v2.0.0', 'v2.0.0', ['GO-A', 'CVE-2026-10002']),
            ('CVE-2026-10002', 'pkg:golang/example.org/b@v2.0.0', 'v2.0.0', ['GO-A', 'CVE-2026-10001']),
        ]
        self.assertEqual(len(cdx['vulnerabilities']), len(expected))
        for actual, (identifier, ref, version, references) in zip(cdx['vulnerabilities'], expected):
            self.assertEqual(actual['id'], identifier)
            self.assertEqual(actual['affects'], [{'ref': ref, 'versions': [{'version': version, 'status': 'affected'}]}])
            self.assertEqual(actual['references'], [{'id': value} for value in references])
            self.assertEqual(actual['description'], '' if identifier == 'GO-B' else 'alias fixture')

    def test_invalid_metadata_findings_and_empty_stream_shape(self):
        invalid = [False, [], {'osv': []}, {'osv': {'id': None}}, {'osv': {'id': 17}}, {'osv': {'id': '  '}},
                   {'finding': None}, {'finding': {'osv': None}}, {'finding': {'osv': 17}},
                   {'finding': {'osv': ' '}}, {'finding': {'osv': 'GO-no-trace', 'trace': 'not-list'}},
                   {'finding': {'osv': 'GO-no-module', 'trace': [None, {'package': 'pkg'}]}}]
        for entries in [[], invalid]:
            code, cdx, receipt = self.normalized(entries, separator=' \n\t ', prefix='\n', suffix=' \n')
            self.assertEqual(code, 0)
            self.assert_schema(cdx, receipt)
            self.assertEqual(receipt['advisory_count'], 0)
            self.assertEqual(receipt['findings'], [])
            self.assertEqual(cdx['components'], [])
            self.assertEqual(cdx['vulnerabilities'], [])
        # A valid finding without metadata keeps its advisory identity, rather
        # than inventing CVE aliases or dropping the unknown advisory.
        _, cdx, receipt = self.normalized([{'finding': {'osv': ' GO-unknown ', 'trace': [{'module': 'lib'}]}}])
        self.assertEqual(receipt['findings'][0]['advisory_id'], 'GO-unknown')
        self.assertEqual(receipt['findings'][0]['version'], '')
        self.assertEqual(cdx['vulnerabilities'][0]['id'], 'GO-unknown')

    def test_bad_stream_is_rejected_before_outputs(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / 'bad.json'
            args = argparse.Namespace(govulncheck=str(source), label='fixture-root', module_dir='.', out_cdx=str(root / 'out.cdx'), out_findings=str(root / 'findings.json'))
            for text in ['{"osv":', '{} trailing-junk']:
                source.write_text(text)
                with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as error:
                    ci.normalize(args)
                self.assertEqual(error.exception.code, 2)
                self.assertFalse(Path(args.out_cdx).exists())
                self.assertFalse(Path(args.out_findings).exists())
            source.unlink()
            with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as error:
                ci.normalize(args)
            self.assertEqual(error.exception.code, 2)

    def test_seeded_nonconflicting_permutation_property(self):
        rng = random.Random(20261004)
        meta = {'osv': {'id': 'GO-order', 'aliases': ['CVE-2026-12345'], 'summary': 'stable metadata'}}
        records = [
            {'finding': {'osv': 'GO-order', 'trace': [{'module': 'lib', 'version': 'v1.0.0'}]}},
            {'finding': {'osv': 'GO-order', 'trace': [{'module': 'lib', 'version': 'v1.0.0', 'package': 'pkg/b'}]}},
            {'finding': {'osv': 'GO-order', 'fixed_version': 'v1.1.0', 'trace': [{'module': 'lib', 'version': 'v1.0.0', 'package': 'pkg/a', 'function': 'Called'}]}},
        ]
        # There is one consistent module version and one unique nonempty fix.
        # No order invariance for conflicting versions or metadata is assumed.
        for trial in range(250):
            entries = copy.deepcopy([meta] + records)
            rng.shuffle(entries)
            _, cdx, data = self.normalized(entries, separator=rng.choice(['', '\n', ' \t ']))
            self.assert_schema(cdx, data)
            self.assertEqual(len(data['findings']), 1)
            item = data['findings'][0]
            self.assertEqual((item['advisory_id'], item['module'], item['version'], item['fixed_version'], item['reachability']), ('GO-order', 'lib', 'v1.0.0', 'v1.1.0', 'called'))
            self.assertEqual(item['packages'], ['pkg/a', 'pkg/b'])
            self.assertEqual(item['symbols'], ['Called'])
            self.assertEqual(item['cve_ids'], ['CVE-2026-12345'])

    def test_seeded_bounded_generated_streams(self):
        # Stdlib generated input, not coverage-guided fuzzing.
        rng = random.Random(20261005)
        for trial in range(400):
            count = rng.randrange(1, 5)
            aliases = ['CVE-2026-12345'] if rng.choice([True, False]) else []
            advisory = {'osv': {'id': 'GO-generated', 'aliases': aliases}}
            records = []
            for index in range(count):
                trace = [{'module': f'fixture/lib-{index}', 'version': 'v1.0.0'}]
                records.append({'finding': {'osv': 'GO-generated', 'trace': trace}})
            entries = [None, 17, advisory] + records
            rng.shuffle(entries)
            _, cdx, data = self.normalized(entries, separator=rng.choice(['', '\n\n', '\t ']))
            self.assert_schema(cdx, data)
            self.assertEqual(len(data['findings']), count)
            self.assertEqual(len(cdx['components']), count)
            self.assertEqual(len(cdx['vulnerabilities']), count)
            self.assertEqual({item['module'] for item in data['findings']}, {f'fixture/lib-{index}' for index in range(count)})
            for item in data['findings']:
                self.assertEqual(item['reachability'], 'required-not-imported')
                self.assertEqual(item['cve_ids'], aliases)
            self.assertEqual({item['id'] for item in cdx['vulnerabilities']}, set(aliases) if aliases else {'GO-generated'})


if __name__ == '__main__':
    unittest.main()
