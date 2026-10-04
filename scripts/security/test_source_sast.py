"""Native source scanner contracts; fixture analyzers are not scan evidence."""
import importlib.util
import json
import os
from pathlib import Path
import random
import subprocess
import tempfile
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location('source_sast', Path(os.environ.get('SOURCE_SAST_CANDIDATE', str(Path(__file__).with_name('source_sast.py')))))
SUBJECT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SUBJECT)

def bandit(severity=None):
    return {'errors': [], 'results': [] if severity is None else [{'issue_severity': severity, 'filename': 'fixture.py', 'line_number': 1}]}

def shell(level=None):
    return [] if level is None else [{'level': level, 'file': 'fixture.sh', 'line': 1}]

class Contracts(unittest.TestCase):
    def test_high_policy_and_raw_warning_retention(self):
        result = SUBJECT.policy(bandit('HIGH'), shell(), 1, 0)
        self.assertEqual(result['exit'], 1)
        self.assertEqual(result['blocking_count'], 1)
        result = SUBJECT.policy(bandit(), shell('error'), 0, 1)
        self.assertEqual(result['exit'], 1)
        for severity in ['LOW', 'MEDIUM']:
            result = SUBJECT.policy(bandit(severity), shell('warning'), 1, 1)
            self.assertEqual(result['exit'], 0)
            self.assertFalse(result['raw_clean'])
            self.assertEqual(result['warning_count'], 2)
            self.assertEqual(result['review_state'], 'non-author-review-required')

    def test_operational_and_malformed_reports_never_pass(self):
        cases = [(bandit('HIGH'), [], 2, 0), (bandit(), [], 2, 0), (bandit(), [], 0, 3), ({}, [], 0, 0),
                 ({'results': [], 'errors': ['failure']}, [], 0, 0),
                 (bandit('HIGH'), [], 0, 0), (bandit(), [], 1, 0),
                 (bandit(), shell('unknown'), 0, 1), (bandit('UNKNOWN'), [], 1, 0)]
        for args in cases:
            with self.subTest(args=args):
                with self.assertRaises(SUBJECT.EvidenceError):
                    SUBJECT.policy(*args)

    def test_literal_heredoc_source_mapping_property(self):
        rng = random.Random(20261002)
        for number in range(64):
            literal = ''.join(rng.choice('ab []?*測\\"\'') for _ in range(number % 19))
            body = 'value = ' + repr(literal) + '\n'
            source = '# header\npython3 - <<\'PY\'\n' + body + 'PY\n'
            with self.subTest(number=number):
                self.assertEqual(SUBJECT.extract_shell(source), [(body, 3, 3)])
                self.assertEqual(SUBJECT.extract_fake('FAKE = ' + repr(body)), [(body, 1, 1)])

    def test_dynamic_or_ambiguous_extraction_rejected(self):
        for source in ['python3 - <<PY\nvalue=1\nPY\n', "python3 - <<'PY'\nvalue=1\n", "cat <<'PY'\nvalue=1\nPY\n", "python3 - <<-'PY'\nvalue=1\nPY\n"]:
            with self.subTest(source=source):
                with self.assertRaises(SUBJECT.EvidenceError):
                    SUBJECT.extract_shell(source)
        for source in ['FAKE = build_source()', 'FAKE = 1', 'FAKE="x"\nFAKE="y"']:
            with self.subTest(source=source):
                with self.assertRaises(SUBJECT.EvidenceError):
                    SUBJECT.extract_fake(source)

    def test_cli_report_and_exit_policy_with_fixture_analyzers(self):
        for mode, expected in [('clean', 0), ('warning', 0), ('high', 1), ('malformed', 2), ('operational', 2)]:
            with self.subTest(mode=mode), tempfile.TemporaryDirectory(prefix='asynq-sast-cli-') as tmp:
                root=Path(tmp)
                subprocess.run(['git','init','-q',str(root)], check=True, capture_output=True)
                (root/'fixture.py').write_text('value=1\n')
                (root/'fixture.sh').write_text('#!/bin/bash\nprintf ok\n')
                subprocess.run(['git','-C',str(root),'add','fixture.py','fixture.sh'], check=True, capture_output=True)
                tools=root/'tools';tools.mkdir();python=tools/'python';check=tools/'shellcheck'
                python.write_text('fixture tool identity');check.write_text('fixture tool identity')
                output=root/'.security/result'
                def fake_process(name, command, source_root, destination, commands):
                    stdout=b'';exitcode=0
                    if name=='bandit-version':stdout=b'__main__.py 1.9.4\n'
                    elif name=='shellcheck-version':stdout=b'version: 0.11.0\n'
                    elif name=='bandit':
                        payload=bandit('HIGH' if mode=='high' else 'LOW' if mode=='warning' else None)
                        payload['metrics']={str(root/'fixture.py'): {}, '_totals': {}}
                        (destination/'bandit.json').write_text('{' if mode=='malformed' else json.dumps(payload))
                        exitcode=3 if mode=='operational' else int(bool(payload['results']))
                    elif name=='shellcheck':stdout=b'[]'
                    commands.append({'name':name,'exit':exitcode,'argv':[str(s) for s in command]})
                    return subprocess.CompletedProcess(command, exitcode, stdout, b'')
                with mock.patch.object(SUBJECT,'run_process',side_effect=fake_process):
                    actual=SUBJECT.main(['--source-root',str(root),'--out',str(output),'--bandit-python',str(python),'--shellcheck',str(check)])
                self.assertEqual(actual, expected)
                if expected==2:
                    self.assertTrue((output/'failure.json').is_file())
                    self.assertFalse((output/'verdict.json').exists())
                else:
                    verdict=json.loads((output/'verdict.json').read_text())
                    self.assertEqual(verdict['raw_clean'], mode=='clean')
                    self.assertEqual(verdict['warning_count'],int(mode=='warning'))
                    self.assertEqual(verdict['blocking_count'],int(mode=='high'))
                    self.assertTrue((output/'receipt.json').is_file())

    def test_missing_tool_cli_operational_exit(self):
        with tempfile.TemporaryDirectory(prefix='asynq-source-sast-') as tmp:
            root = Path(tmp)
            subprocess.run(['git', 'init', '-q', str(root)], check=True, capture_output=True)
            (root/'fixture.py').write_text('value=1\n')
            (root/'fixture.sh').write_text('#!/bin/bash\nprintf ok\n')
            subprocess.run(['git', '-C', str(root), 'add', 'fixture.py', 'fixture.sh'], check=True, capture_output=True)
            output=root/'.security/missing'
            self.assertEqual(SUBJECT.main(['--source-root', str(root), '--out', str(output)]), 2)
            self.assertEqual(json.loads((output/'failure.json').read_text())['error'], 'missing-analyzer')
            self.assertFalse((output/'verdict.json').exists())
            self.assertEqual(SUBJECT.main(['--source-root', str(root), '--out', str(output)]), 2)

import sys
SOURCE = Path(SPEC.origin).resolve()
FAKE = """#!/usr/bin/env python3
import json,sys
from pathlib import Path
args=sys.argv[1:]
if '--version' in args:
 print(BANNER)
elif '-o' in args:
 index=args.index('-o');path=args[index+1];sources=args[index+2:]
 payload={'errors':[],'results':[],'metrics':dict.fromkeys(sources,{})}
 if REPORT_MODE == 'metrics-missing':payload['metrics'].pop(sources[0])
 if REPORT_MODE == 'metrics-extra':payload['metrics'][str(Path.cwd()/'unscanned.py')]={}
 if REPORT_MODE == 'source-drift':Path(sources[0]).write_text('value=2\\n')
 with open(path, 'w') as stream:
  stream.write(json.dumps(payload))
else:
 payload={'shape': []} if REPORT_MODE=='shell-object' else None if REPORT_MODE=='shell-null' else 7 if REPORT_MODE=='shell-number' else 'empty' if REPORT_MODE=='shell-string' else []
 print(json.dumps(payload))
"""

class GuardedInputs(unittest.TestCase):
    def case(self, kind, expected, bandit='__main__.py 1.9.4', shell='version: 0.11.0', report_mode='valid'):
        with tempfile.TemporaryDirectory(prefix='asynq-guarded-') as tmp:
            root = Path(tmp)
            subprocess.run(['git', 'init', '-q', str(root)], check=True,
                           capture_output=True, timeout=15)
            py, sh = b'value=1\n', b'#!/bin/bash\nprintf ok\n'
            python_cases = {'invalid-python': b'broken = (\n',
                            'dynamic-fake': b"FAKE = str('value=1')\n",
                            'invalid-utf8': b'\xff\n'}
            py = python_cases.get(kind, py)
            if kind == 'invalid-heredoc':
                sh = b"python3 - <<'PY'\nbroken = (\nPY\n"
            if kind != 'empty':
                if kind != 'only-shell':
                    (root / 'fixture.py').write_bytes(py)
                if kind != 'only-python':
                    (root / 'fixture.sh').write_bytes(sh)
                if kind == 'symlink':
                    (root / 'fixture.py').unlink()
                    (root / 'target').write_text('value=1\n')
                    (root / 'fixture.py').symlink_to('target')
                subprocess.run(['git', '-C', str(root), 'add', '.'], check=True,
                               capture_output=True, timeout=15)
            tools = root / 'tools'
            tools.mkdir()
            for name, banner in [('python', bandit), ('shellcheck', shell)]:
                path = tools / name
                path.write_text(FAKE.replace('BANNER', repr(banner)).replace('REPORT_MODE', repr(report_mode)))
                path.chmod(0o700)
            output = root / '.security/result'
            result = subprocess.run([
                sys.executable, str(SOURCE), '--source-root', str(root),
                '--out', str(output), '--bandit-python', str(tools / 'python'),
                '--shellcheck', str(tools / 'shellcheck')],
                capture_output=True, text=True, timeout=15)
            self.assertEqual(result.stderr, '', (kind, report_mode, result.stderr))
            if expected is None:
                self.assertEqual(result.returncode, 0, (kind, report_mode, result.stdout))
                verdict = json.loads((output / 'verdict.json').read_text())
                self.assertTrue(verdict['raw_clean'])
                self.assertEqual(verdict['blocking_count'], 0)
                self.assertEqual(verdict['warning_count'], 0)
                self.assertTrue((output / 'receipt.json').is_file())
                self.assertFalse((output / 'failure.json').exists())
                self.assertEqual(json.loads((output / 'source-before.json').read_text()),
                                 json.loads((output / 'source-after.json').read_text()))
            else:
                self.assertEqual(result.returncode, 2, (kind, report_mode, result.stdout))
                self.assertEqual(json.loads(result.stdout)['error'], expected)
                self.assertEqual(json.loads((output / 'failure.json').read_text())['error'], expected)
                self.assertFalse((output / 'receipt.json').exists())
                self.assertFalse((output / 'verdict.json').exists())

    def test_empty_tracked_inventory_diagnostic(self):
        self.case('empty', 'empty-source-inventory')

    def test_real_tracked_source_guards(self):
        cases = [('symlink', 'unsupported-tracked-source'),
                 ('only-python', 'unsupported-empty-language-inventory'),
                 ('only-shell', 'unsupported-empty-language-inventory'),
                 ('invalid-python', 'invalid-tracked-python'),
                 ('dynamic-fake', 'dynamic-generated-python'),
                 ('invalid-utf8', 'source-or-output-unavailable'),
                 ('invalid-heredoc', 'invalid-embedded-python')]
        for kind, error in cases:
            with self.subTest(kind=kind):
                self.case(kind, error)

    def test_version_admission_boundaries(self):
        for version in ['1.9.3', '1.9.5', '2.0.0', '1.9.40', '1.9.4evil']:
            with self.subTest(bandit=version):
                self.case('normal', 'bandit-version-mismatch', bandit='__main__.py ' + version)
        for version in ['0.10.0', '0.11.1', '1.0.0', '0.11.00', '0.11.0evil']:
            with self.subTest(shell=version):
                self.case('normal', 'shellcheck-version-mismatch', shell='version: ' + version)


    def test_real_report_shapes_and_valid_control(self):
        self.case('normal', None)
        for mode in ['shell-object', 'shell-null', 'shell-number', 'shell-string']:
            with self.subTest(report_mode=mode):
                self.case('normal', 'invalid-shellcheck-report', report_mode=mode)

    def test_real_bandit_inventory_identity(self):
        self.case('normal', None)
        for mode in ['metrics-missing', 'metrics-extra']:
            with self.subTest(report_mode=mode):
                self.case('normal', 'bandit-source-inventory-mismatch', report_mode=mode)

    def test_real_source_drift_and_unchanged_control(self):
        self.case('normal', None)
        self.case('normal', 'source-changed-during-scan', report_mode='source-drift')

if __name__ == '__main__':
    unittest.main(verbosity=2)
