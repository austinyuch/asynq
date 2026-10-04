"""Offline ingress contracts using real files, JSON decoders and CLI handlers."""
import argparse
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import random
import sys
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('security_ingress_subject', Path(__file__).with_name('security_local_ci.py'))
ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ci)


class SecurityIngressContracts(unittest.TestCase):
    def write(self, root, name, value):
        path = root / name
        path.write_text(json.dumps(value, ensure_ascii=False), encoding='utf-8')
        return path

    def command(self, args):
        output, error = io.StringIO(), io.StringIO()
        with patch.object(sys, 'argv', ['security_local_ci.py', *map(str, args)]), contextlib.redirect_stdout(output), contextlib.redirect_stderr(error):
            code = ci.main()
        return code, output.getvalue(), error.getvalue()

    def assert_exit_two(self, call, path, message):
        error = io.StringIO()
        with contextlib.redirect_stderr(error), self.assertRaises(SystemExit) as caught:
            call()
        self.assertEqual(caught.exception.code, 2)
        self.assertIn('security-local-ci:', error.getvalue())
        self.assertIn(str(path), error.getvalue())
        self.assertIn(message, error.getvalue())

    def test_read_json_and_stream_error_boundaries(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            values = [None, False, 17, [], {'nested': ['繁體', {'x': 3}]}]
            for index, value in enumerate(values):
                path = self.write(root, str(index), value)
                self.assertEqual(ci.read_json(path), value)
            absent = root / 'absent.json'
            self.assert_exit_two(lambda: ci.read_json(absent), absent, 'cannot read')
            self.assert_exit_two(lambda: list(ci.iter_json_stream(absent)), absent, 'cannot read')
            malformed = root / 'malformed.json'
            malformed.write_text('{"x":', encoding='utf-8')
            self.assert_exit_two(lambda: ci.read_json(malformed), malformed, 'not valid JSON')
            self.assert_exit_two(lambda: list(ci.iter_json_stream(malformed)), malformed, 'not a valid govulncheck JSON stream')
            trailing = root / 'trailing.json'
            trailing.write_text('{"first":1} trailing', encoding='utf-8')
            stream = ci.iter_json_stream(trailing)
            self.assertEqual(next(stream), {'first': 1})
            self.assert_exit_two(lambda: next(stream), trailing, 'not a valid govulncheck JSON stream')
            for text in ['', ' \t\r\n', '{}{}', '\n{}\t{"a":null} \n']:
                path = root / 'stream.json'
                path.write_text(text, encoding='utf-8')
                expected = [] if not text.strip() else ([{}, {}] if text == '{}{}' else [{}, {'a': None}])
                self.assertEqual(list(ci.iter_json_stream(path)), expected)

    def test_invalid_utf8_fails_closed_with_decode_diagnostic(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'invalid-utf8.json'
            path.write_bytes(b'\xff')
            for reader in [ci.read_json, lambda p: list(ci.iter_json_stream(p))]:
                with self.subTest(reader=reader):
                    self.assert_exit_two(lambda: reader(path), path, 'UTF-8')

    def test_seeded_bounded_stream_shape_generator(self):
        # Bounded deterministic input generation, not coverage-guided fuzzing.
        rng = random.Random(20261011)
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'generated.json'
            for trial in range(500):
                values = []
                dictionaries = []
                for index in range(rng.randrange(25)):
                    value = rng.choice([None, rng.choice([True, False]), rng.randrange(-100, 100), 'text)括號\n', [index, None], {'trial': trial, 'index': index, 'nested': [False, '值']}])
                    values.append(value)
                    if isinstance(value, dict):
                        dictionaries.append(value)
                chunks = [rng.choice(['', '\n', ' \t', '\r\n']) + json.dumps(value, ensure_ascii=False) for value in values]
                path.write_text(''.join(chunks) + ' \n', encoding='utf-8')
                self.assertEqual(list(ci.iter_json_stream(path)), dictionaries)

    def test_cve_identity_aliases_exact_stdout(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            first = self.write(root, 'first.json', {'vulnerabilities': [None, {'id': ' cve-2026-10002 ', 'references': [None, {'id': 'CVE-2025-9999'}, {'id': ' cVe-2026-10002 '}, {'id': 'GO-1234'}]}, {'id': 'prefix CVE-2026-10003 suffix'}, {'id': 'CVE-2026-123'}, {'id': 'CVE-26-10000'}, {'id': 'CVE-2026-' + '1' * 20}, {'id': 17}, {'id': None}, {'id': 'CVE-٢٠٢٦-١٢٣٤'}, {'id': 'CVE-２０２６-１２３４'}]})
            second = self.write(root, 'second.json', {'vulnerabilities': [{'id': 'CVE-2026-10002'}, {'id': 'GO-5678', 'references': [{'id': 'CVE-2027-' + '9' * 19}]}]})
            nonobject = self.write(root, 'array.json', [])
            output = io.StringIO()
            with contextlib.redirect_stdout(output):
                code = ci.cve_ids(argparse.Namespace(inputs=[str(first), str(second), str(nonobject), str(root / 'missing')]))
            self.assertEqual(code, 0)
            self.assertEqual(output.getvalue(), 'CVE-2025-9999\nCVE-2026-10002\nCVE-2027-' + '9' * 19 + '\n')
            malformed = root / 'bad.json'
            malformed.write_text('[', encoding='utf-8')
            self.assert_exit_two(lambda: ci.cve_ids(argparse.Namespace(inputs=[str(malformed)])), malformed, 'not valid JSON')

    def test_seeded_cve_union_ordering_property(self):
        rng = random.Random(20261012)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for trial in range(250):
                expected = {f'CVE-{rng.randrange(2000, 2030)}-{rng.randrange(1000, 999999)}' for _ in range(rng.randrange(12))}
                records = [{'id': 'GO-ignored', 'references': [{'id': ' \t' + cve.lower() + '\n'}]} for cve in expected]
                records += [{'id': cve} for cve in expected]
                records += [None, {'id': 'prefix-CVE-2026-9999'}, {'id': 'CVE-2026-9999-suffix'}]
                rng.shuffle(records)
                mid = len(records) // 2
                paths = [self.write(root, 'a.json', {'vulnerabilities': records[:mid]}), self.write(root, 'b.json', {'vulnerabilities': records[mid:]})]
                rng.shuffle(paths)
                code, output, error = self.command(['cve-ids', *paths])
                self.assertEqual(code, 0)
                self.assertEqual(error, '')
                self.assertEqual(output, ''.join(cve + '\n' for cve in sorted(expected)))

    def test_main_cli_real_handler_dispatch(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            stream = root / 'govulncheck.json'
            stream.write_text(json.dumps({'osv': {'id': 'GO-ingress', 'aliases': ['CVE-2026-10001'], 'summary': 'contract'}}) + '\n' + json.dumps({'finding': {'osv': 'GO-ingress', 'trace': [{'module': 'example.org/fixture', 'version': 'v1.0.0', 'package': 'example.org/fixture', 'function': 'Invoke'}], 'fixed_version': 'v1.1.0'}}), encoding='utf-8')
            cdx, findings = root / 'nested/cdx.json', root / 'nested/findings.json'
            code, stdout, stderr = self.command(['normalize', '--govulncheck', stream, '--label', 'root', '--module-dir', '.', '--out-cdx', cdx, '--out-findings', findings])
            self.assertEqual((code, stdout, stderr), (0, '', ''))
            self.assertTrue(cdx.is_file() and findings.is_file(), 'normalize must emit both real documents')
            data = json.loads(findings.read_text())
            self.assertEqual(data['schema'], 'asynq-govulncheck-findings/v1')
            self.assertEqual(data['findings'][0]['reachability'], 'called')
            self.assertEqual(json.loads(cdx.read_text())['vulnerabilities'][0]['id'], 'CVE-2026-10001')
            self.assertEqual(self.command(['cve-ids', cdx]), (0, 'CVE-2026-10001\n', ''))
            gosec = self.write(root, 'gosec.json', {'Issues': [{'rule_id': 'G-ingress', 'cwe': {'id': '79'}, 'severity': 'high', 'file': 'fixture.go', 'line': '3'}], 'Stats': {'files': 1, 'lines': 3, 'nosec': 0}})
            sast = root / 'sast/findings.json'
            self.assertEqual(self.command(['sast-normalize', '--gosec', gosec, '--label', 'root', '--module-dir', '.', '--out-findings', sast]), (0, '', ''))
            self.assertTrue(sast.is_file(), 'sast handler must emit receipt')
            self.assertEqual(json.loads(sast.read_text())['findings'][0]['severity'], 'HIGH')
            correlation = self.write(root, 'correlation.json', {})
            verdict, summary, plan = root / 'gate/verdict.json', root / 'gate/summary.txt', root / 'gate/fix.sh'
            args = ['gate', '--correlation', correlation, '--findings', findings, '--sast', sast, '--out-verdict', verdict, '--out-summary', summary, '--out-fix-plan', plan]
            code, stdout, stderr = self.command(args)
            self.assertEqual(code, 1)
            self.assertEqual(stderr, '')
            self.assertTrue(verdict.is_file() and summary.is_file() and plan.is_file(), 'gate must emit all artifacts')
            self.assertEqual(json.loads(verdict.read_text())['verdict'], 'block')
            self.assertEqual(stdout, summary.read_text())
            self.assertIn('example.org/fixture@v1.1.0', plan.read_text())
            self.assertEqual(plan.stat().st_mode & 0o777, 0o755)

    def test_main_cli_parser_rejects_before_outputs(self):
        for args in [[], ['unknown'], ['normalize'], ['cve-ids'], ['gate', '--correlation', 'missing']]:
            with self.subTest(args=args):
                output = io.StringIO()
                with patch.object(sys, 'argv', ['security_local_ci.py', *args]), contextlib.redirect_stderr(output), self.assertRaises(SystemExit) as caught:
                    ci.main()
                self.assertEqual(caught.exception.code, 2)
                self.assertIn('usage: security_local_ci.py', output.getvalue())
                self.assertIn('error:', output.getvalue())


if __name__ == '__main__':
    unittest.main()
