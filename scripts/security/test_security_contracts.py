"""Synthetic, offline contracts for security verdicts; no scanner or process runs."""
import argparse
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import random
import shlex
import tempfile
import unittest

spec = importlib.util.spec_from_file_location(
    'security_contract_subject', Path(__file__).with_name('security_local_ci.py'))
ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ci)


def finding(advisory='GO-contract', cves=None, reach='required-not-imported', fixed='', module='example.org/lib'):
    return {'advisory_id': advisory, 'cve_ids': cves or [], 'module': module,
            'version': 'v1.0.0', 'fixed_version': fixed, 'reachability': reach,
            'packages': ['example.org/lib'], 'symbols': [], 'summary': 'synthetic finding'}


class SecurityDecisionContracts(unittest.TestCase):
    def write(self, root, name, data):
        path = root / name
        path.write_text(json.dumps(data), encoding='utf-8')
        return str(path)

    def verdict(self, findings=(), listed=(), sboms=(), sast=(), cwes=()):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            correlation = {'nested': [{'cve_id': cve, 'kev_status': 'listed'} for cve in listed]}
            modules = []
            for index, (module_dir, items) in enumerate(findings):
                modules.append(self.write(root, f'findings-{index}.json',
                                          {'module_dir': module_dir, 'label': module_dir, 'findings': items}))
            sbom_paths = [self.write(root, label + '.cdx.json', data) for label, data in sboms]
            sast_paths = [self.write(root, f'sast-{index}.json', data) for index, data in enumerate(sast)]
            args = argparse.Namespace(correlation=self.write(root, 'correlation.json', correlation),
                                      findings=modules, sbom=sbom_paths, sast=sast_paths,
                                      kev_catalog=self.write(root, 'catalog.json', {'vulnerabilities': cwes}),
                                      out_verdict=str(root / 'verdict.json'), out_fix_plan=str(root / 'fix.sh'),
                                      out_summary=str(root / 'summary.txt'))
            with contextlib.redirect_stdout(io.StringIO()) as stream:
                code = ci.gate(args)
            return code, json.loads((root / 'verdict.json').read_text()), (root / 'fix.sh').read_text(), stream.getvalue()

    def test_independent_reachability_kev_fix_decision_table(self):
        cve = 'CVE-2026-10001'
        # An explicit policy truth table, independent of the implementation's branches.
        cases = [
            ('required-not-imported', '', False, 'pass', 'not-reachable (required-not-imported)'),
            ('required-not-imported', 'v2.0.0', False, 'pass', 'not-reachable (required-not-imported)'),
            ('imported-not-called', 'v2.0.0', False, 'pass', 'not-reachable (imported-not-called)'),
            ('called', '', False, 'pass', 'reachable-no-fix-available'),
            ('called', 'v2.0.0', False, 'block', 'reachable-with-fix'),
            ('required-not-imported', '', True, 'block', 'kev-listed'),
            ('imported-not-called', 'v2.0.0', True, 'block', 'kev-listed'),
            ('called', '', True, 'block', 'kev-listed'),
            ('called', 'v2.0.0', True, 'block', 'kev-listed'),
        ]
        for reach, fixed, is_listed, verdict, reason in cases:
            with self.subTest(reach=reach, fixed=fixed, listed=is_listed):
                code, data, plan, summary = self.verdict([('.', [finding(cves=[cve], reach=reach, fixed=fixed)])], [cve] if is_listed else [])
                self.assertEqual(data['schema'], 'asynq-security-verdict/v1')
                self.assertEqual(data['verdict'], verdict)
                self.assertEqual(code, 1 if verdict == 'block' else 0)
                collection = data['blocking'] if verdict == 'block' else data['warnings']
                self.assertEqual(len(collection), 1)
                self.assertEqual(collection[0]['reason'], reason)
                self.assertEqual(collection[0]['cve_ids'], [cve])
                self.assertEqual(data['fix_plan'], {'.': {'example.org/lib': fixed}} if fixed else {})
                self.assertIn('verdict: ' + verdict.upper(), summary)
                self.assertEqual(data['policy'], {
                    'block_on_kev_listed': True,
                    'block_on_reachable_with_fix': True,
                    'block_on_sast_high_severity': True,
                    'warn_on_reachable_without_fix': True,
                    'warn_on_not_reachable': True,
                    'warn_on_sast_medium_low_severity': True,
                })
                self.assertIn('supplied immutable snapshots', ' '.join(data['disclosure']))
        code, data, _, _ = self.verdict([('.', [finding(cves=[cve])])], ['CVE-2026-99999'])
        self.assertEqual(code, 0)
        self.assertEqual(data['kev_listed_cve_count'], 1)
        self.assertEqual(data['blocking'], [])

    def test_sast_high_blocks_but_cwe_membership_only_ranks(self):
        for severity, expected in [('HIGH', 1), ('MEDIUM', 0), ('LOW', 0), ('', 0)]:
            with self.subTest(severity=severity):
                record = {'rule_id': 'G-contract', 'cwe': 'CWE-79', 'severity': severity,
                          'confidence': 'HIGH', 'file': 'fixture.go', 'line': '17', 'details': 'fixture'}
                code, data, _, summary = self.verdict(sast=[{'label': 'root', 'findings': [record], 'scanned_files': 3, 'nosec_suppressions': 2}], cwes=[{'cwes': ['CWE-79']}, {'cwes': ['CWE-79']}])
                self.assertEqual(code, expected)
                item = (data['blocking'] if expected else data['warnings'])[0]
                self.assertEqual(item['kev_entries_for_cwe'], 2)
                self.assertEqual(item['location'], 'fixture.go:17')
                self.assertNotIn('cve_ids', item)
                self.assertEqual(data['sast']['finding_count'], 1)
                self.assertEqual(data['sast']['scanned_files'], 3)
                self.assertEqual(data['sast']['nosec_suppressions'], 2)
                self.assertIn('weakness-class prioritization, not exploitability', summary)
                self.assertEqual(data['sast']['cwe_kev_correlation'], [{'cwe': 'CWE-79', 'sast_findings': 1, 'kev_entries_citing_cwe': 2}])

    def test_sbom_mapping_and_sbom_only_gate(self):
        document = {'components': [None, {'bom-ref': 'pkg-one', 'name': 'example.org/lib', 'version': 'v1.0.0'}],
                    'vulnerabilities': [None, {'id': 'GO-not-a-CVE'}, {'id': ' cve-2026-10002 ',
                     'affects': [None, {'ref': 'missing'}, {'ref': 'pkg-one'}],
                     'ratings': [None, {'severity': 'high'}],
                     'recommendation': 'Upgrade example.org/lib to version 2.4.0; then rescan'}]}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = self.write(root, 'fixture-tools.cdx.json', document)
            mapped = ci.sbom_cve_map([str(root / 'missing.json'), self.write(root, 'non-object.json', []), path])
            self.assertEqual(mapped, {'CVE-2026-10002': {'cve_id': 'CVE-2026-10002', 'labels': ['fixture-tools'], 'module': 'example.org/lib', 'version': 'v1.0.0', 'fixed_version': 'v2.4.0', 'severity': 'high'}})
        for listed, exit_code in [([], 0), (['CVE-2026-10002'], 1)]:
            code, data, _, _ = self.verdict(listed=listed, sboms=[('fixture-tools', document)])
            self.assertEqual(code, exit_code)
            item = (data['blocking'] if exit_code else data['warnings'])[0]
            self.assertEqual(item['reachability'], 'not-assessed-by-govulncheck')
            self.assertEqual(item['reason'], 'kev-listed' if exit_code else 'sbom-only (reachability not assessed)')
            self.assertEqual(data['fix_plan'], {'tools': {'example.org/lib': 'v2.4.0'}})
        code, data, _, _ = self.verdict([('.', [finding(cves=['CVE-2026-10002'])])], sboms=[('fixture-tools', document)])
        self.assertEqual(code, 0)
        self.assertEqual(len(data['warnings']), 1)  # Same CVE must not acquire a second SBOM route.

    def test_cwe_catalog_mapper_and_sast_schema(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            catalog = self.write(root, 'catalog.json', {'vulnerabilities': [None, {}, {'cwes': [' cwe-79 ', None, '']}, {'cwes': ['CWE-79', 'CWE-89']}]})
            self.assertEqual(ci.kev_cwe_index(Path(catalog)), {'CWE-79': 2, 'CWE-89': 1})
            self.assertEqual(ci.kev_cwe_index(Path(self.write(root, 'empty.json', []))), {})
            issues = {'Issues': [None, {'rule_id': 'G-b', 'cwe': {'id': '79'}, 'severity': 'high', 'confidence': 'low', 'file': 'b.go', 'line': '2'}, {'rule_id': 'G-a', 'cwe': None, 'file': 'a.go', 'line': '1'}], 'Stats': {'files': 5, 'lines': 40, 'nosec': 3}}
            args = argparse.Namespace(gosec=self.write(root, 'gosec.json', issues), out_findings=str(root / 'sast.json'), label='synthetic-root', module_dir='.')
            self.assertEqual(ci.sast_normalize(args), 0)
            data = json.loads((root / 'sast.json').read_text())
            self.assertEqual(data['schema'], 'asynq-sast-findings/v1')
            self.assertEqual(data['finding_count'], 2)
            self.assertEqual(data['findings'][0]['rule_id'], 'G-a')
            self.assertEqual(data['findings'][1]['severity'], 'HIGH')
            self.assertEqual(data['findings'][1]['cwe'], 'CWE-79')
            for bad in [[], {'Issues': 'not-a-list'}]:
                args.gosec = self.write(root, 'bad.json', bad)
                with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as error:
                    ci.sast_normalize(args)
                self.assertEqual(error.exception.code, 2)

    def test_seeded_ordering_membership_and_highest_stable_fix_properties(self):
        rng = random.Random(20261002)
        for trial in range(250):
            versions = sorted(rng.sample(range(1, 100), 3))
            records = [finding(advisory=f'GO-{trial}-{i}', fixed=f'v{version}.0.0') for i, version in enumerate(versions)]
            rng.shuffle(records)
            code, data, _, _ = self.verdict([('.', records)])
            self.assertEqual(code, 0)
            self.assertEqual(data['fix_plan'], {'.': {'example.org/lib': f'v{versions[-1]}.0.0'}})
            rng.shuffle(records)
            _, reordered, _, _ = self.verdict([('.', records)])
            self.assertEqual(reordered['fix_plan'], data['fix_plan'])
            self.assertEqual({item['advisory_id'] for item in reordered['warnings']}, {item['advisory_id'] for item in data['warnings']})
            cve = f'CVE-2026-{10000 + trial}'
            record = finding(cves=[cve], reach=rng.choice(['called', 'imported-not-called', 'required-not-imported']))
            code, data, _, _ = self.verdict([('x', [record])], ['CVE-2025-99999'])
            self.assertEqual(code, 0)
            self.assertEqual(data['blocking'], [])
            code, data, _, _ = self.verdict([('x', [record])], ['CVE-2025-99999', cve])
            self.assertEqual(code, 1)
            self.assertEqual(data['blocking'][0]['reason'], 'kev-listed')

    def test_seeded_bounded_json_shape_generator(self):
        # Deterministic input generation, explicitly not a coverage-guided fuzzer.
        rng = random.Random(20261003)
        for index in range(1000):
            cve = f'CVE-2026-{10000 + index}'
            listed = rng.choice([True, False])
            record = {rng.choice(['cve_id', 'cveID', 'id']): ' ' + cve.lower() + ' ',
                      'kev_status': 'listed' if listed else 'not-listed',
                      'knownRansomwareCampaignUse': 'Known', 'project': 'fixture'}
            wrapped = record
            for depth in range(rng.randrange(5)):
                wrapped = rng.choice([{'nested': wrapped, 'noise': None}, [False, 17, wrapped]])
            result = ci.kev_listed_cves(wrapped)
            self.assertEqual(set(result), {cve} if listed else set())
            if listed:
                self.assertEqual(result[cve]['known_ransomware_campaign_use'], 'Known')
                self.assertEqual(result[cve]['projects'], 'fixture')
            self.assertEqual(ci.classify_trace([None, {'module': 'lib'}]), 'required-not-imported')
            self.assertEqual(ci.classify_trace([{}, {'package': 'lib'}]), 'imported-not-called')
            self.assertEqual(ci.classify_trace([{'package': 'lib'}, {'function': 'Invoke'}]), 'called')

    def test_runtime_fix_plan_stops_before_dependency_commands(self):
        code, data, plan, summary = self.verdict([('.', [finding(module='stdlib', reach='called', fixed='v1.26.6'), finding(module='example.org/lib', fixed='v2.0.0')])])
        self.assertEqual(code, 1)
        commands = [shlex.split(line) for line in plan.splitlines() if line and not line.startswith('#')]
        stops = [i for i, parts in enumerate(commands) if parts == ['exit', '2']]
        dependency_commands = [i for i, parts in enumerate(commands) if parts[:2] == ['go', 'get']]
        self.assertEqual(len(stops), 1)
        self.assertTrue(dependency_commands)
        self.assertLess(stops[0], min(dependency_commands))
        self.assertNotIn(['go', 'get', 'stdlib@v1.26.6'], commands)
        self.assertIn('select patched Go runtime v1.26.6', summary)
        self.assertEqual(data['fix_plan']['.']['stdlib'], 'v1.26.6')


if __name__ == '__main__':
    unittest.main()
