#!/usr/bin/env python3
"""Local pinned native analyzers; never downloads tools or uploads source."""
import argparse
import ast
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess

BANDIT_VERSION = '1.9.4'
SHELLCHECK_VERSION = '0.11.0'

class EvidenceError(Exception):
    """Fixed diagnostic code; subprocess diagnostics remain data artifacts."""

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def write_json(path, value):
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + '\n', encoding='utf-8')

def extract_shell(text):
    """Accept literal Python heredocs only; never evaluate shell expansion."""
    lines = text.splitlines(keepends=True)
    bodies = []
    index = 0
    while index < len(lines):
        line = lines[index]
        if '<<' not in line:
            index += 1
            continue
        # Other heredocs are outside Python source; reject unknown/multiline forms.
        if not re.search(r'\bpython(?:3)?\b', line):
            raise EvidenceError('unsupported-heredoc-source')
        match = re.search(r"<<'([A-Za-z_][A-Za-z_0-9]*)'", line)
        if not match or line.count('<<') != 1 or '<<-' in line:
            raise EvidenceError('dynamic-python-heredoc')
        delimiter = match.group(1)
        first = index + 1
        last = first
        while last < len(lines) and lines[last].rstrip('\r\n') != delimiter:
            last += 1
        if last == len(lines):
            raise EvidenceError('unterminated-python-heredoc')
        body = ''.join(lines[first:last])
        try:
            ast.parse(body)
        except (SyntaxError, ValueError) as exc:
            raise EvidenceError('invalid-embedded-python') from exc
        bodies.append((body, first + 1, last))
        index = last + 1
    return bodies

def extract_fake(text):
    try:
        tree = ast.parse(text)
    except (SyntaxError, ValueError) as exc:
        raise EvidenceError('invalid-tracked-python') from exc
    assignments = [n for n in tree.body if isinstance(n, ast.Assign)
                   and any(isinstance(t, ast.Name) and t.id == 'FAKE' for t in n.targets)]
    if not assignments:
        return []
    if len(assignments) != 1:
        raise EvidenceError('ambiguous-generated-python')
    node = assignments[0]
    try:
        body = ast.literal_eval(node.value)
        if not isinstance(body, str):
            raise ValueError('non-string')
        ast.parse(body)
    except (ValueError, TypeError, SyntaxError) as exc:
        raise EvidenceError('dynamic-generated-python') from exc
    return [(body, node.lineno, node.end_lineno)]

def policy(bandit, shellcheck, bandit_exit, shellcheck_exit):
    if bandit_exit not in (0, 1) or shellcheck_exit not in (0, 1):
        raise EvidenceError('analyzer-operational-exit')
    if not isinstance(bandit, dict) or not isinstance(bandit.get('results'), list) or not isinstance(bandit.get('errors'), list):
        raise EvidenceError('invalid-bandit-report')
    if bandit['errors']:
        raise EvidenceError('bandit-scan-errors')
    if not isinstance(shellcheck, list):
        raise EvidenceError('invalid-shellcheck-report')
    findings = bandit['results']
    for row in findings:
        if not isinstance(row, dict) or row.get('issue_severity') not in ('LOW', 'MEDIUM', 'HIGH') or not isinstance(row.get('filename'), str) or type(row.get('line_number')) is not int:
            raise EvidenceError('invalid-bandit-finding')
    for row in shellcheck:
        if not isinstance(row, dict) or row.get('level') not in ('error', 'warning', 'info', 'style') or not isinstance(row.get('file'), str) or type(row.get('line')) is not int:
            raise EvidenceError('invalid-shellcheck-finding')
    # A nonzero finding exit must correspond to material findings, and vice versa.
    if bool(findings) != bool(bandit_exit) or bool(shellcheck) != bool(shellcheck_exit):
        raise EvidenceError('analyzer-report-exit-mismatch')
    blocking = sum(row['issue_severity'] == 'HIGH' for row in findings) + sum(row['level'] == 'error' for row in shellcheck)
    warnings = len(findings) + len(shellcheck) - blocking
    return {'exit': 1 if blocking else 0, 'blocking_count': blocking,
            'warning_count': warnings, 'raw_clean': not findings and not shellcheck,
            'review_state': 'non-author-review-required' if warnings else 'no-warning-findings',
            'policy': {'block_bandit': ['HIGH'], 'block_shellcheck': ['error'],
                       'retain_all_other_findings': True}}

def run_process(name, command, root, output, commands):
    try:
        environment = dict(os.environ, PYTHONDONTWRITEBYTECODE='1')
        result = subprocess.run([str(s) for s in command], cwd=root, env=environment, capture_output=True, timeout=120)
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise EvidenceError('analyzer-process-unavailable') from exc
    (output / (name + '.stdout')).write_bytes(result.stdout)
    (output / (name + '.stderr')).write_bytes(result.stderr)
    commands.append({'name': name, 'argv': [str(s) for s in command], 'exit': result.returncode})
    return result

def tracked_sources(root):
    try:
        result = subprocess.run(['git', 'ls-files', '-z', '*.py', '*.sh'], cwd=root, capture_output=True, timeout=30)
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise EvidenceError('git-source-inventory-unavailable') from exc
    if result.returncode:
        raise EvidenceError('git-source-inventory-unavailable')
    names = result.stdout.decode('utf-8').rstrip('\0').split('\0') if result.stdout else []
    sources = []
    for name in names:
        path = root / name
        if not name or path.is_symlink() or not path.is_file() or not path.resolve().is_relative_to(root):
            raise EvidenceError('unsupported-tracked-source')
        sources.append(path)
    if not sources:
        raise EvidenceError('empty-source-inventory')
    return sources

def scan(root, output, bandit_python, shellcheck):
    sources = tracked_sources(root)
    before = {str(path.relative_to(root)): sha(path) for path in sources}
    write_json(output / 'source-before.json', before)
    commands = []
    for tool in (bandit_python, shellcheck):
        if not tool.is_file():
            raise EvidenceError('missing-analyzer')
    version = run_process('bandit-version', [bandit_python, '-m', 'bandit', '--version'], root, output, commands)
    if version.returncode or not re.search(rb'^(?:bandit|__main__\.py) 1\.9\.4(?:\s|$)', version.stdout):
        raise EvidenceError('bandit-version-mismatch')
    version = run_process('shellcheck-version', [shellcheck, '--version'], root, output, commands)
    if version.returncode or not re.search(rb'^version: 0\.11\.0$', version.stdout, re.M):
        raise EvidenceError('shellcheck-version-mismatch')
    embedded = output / 'embedded-python'
    embedded.mkdir()
    mapping = []
    for source in sources:
        text = source.read_text(encoding='utf-8')
        bodies = extract_shell(text) if source.suffix == '.sh' else extract_fake(text)
        for body, first, last in bodies:
            target = embedded / ('source-%04d.py' % len(mapping))
            target.write_text(body, encoding='utf-8')
            mapping.append({'generated': str(target), 'source': str(source.relative_to(root)),
                            'source_sha256': before[str(source.relative_to(root))],
                            'first_source_line': first, 'last_source_line': last,
                            'mapping_kind': 'heredoc-offset' if source.suffix == '.sh' else 'literal-assignment',
                            'sha256': sha(target)})
    write_json(output / 'embedded-source-map.json', mapping)
    python_sources = [p for p in sources if p.suffix == '.py'] + sorted(embedded.glob('*.py'))
    shell_sources = [p for p in sources if p.suffix == '.sh']
    if not python_sources or not shell_sources:
        raise EvidenceError('unsupported-empty-language-inventory')
    bandit_run = run_process('bandit', [bandit_python, '-m', 'bandit', '--ignore-nosec', '-f', 'json', '-o', output / 'bandit.json', *python_sources], root, output, commands)
    shell_run = run_process('shellcheck', [shellcheck, '--norc', '--shell=bash', '--format=json', *shell_sources], root, output, commands)
    (output / 'shellcheck.json').write_bytes(shell_run.stdout)
    try:
        bandit = json.loads((output / 'bandit.json').read_text())
        shell = json.loads(shell_run.stdout)
    except (ValueError, OSError) as exc:
        raise EvidenceError('analyzer-report-unreadable') from exc
    verdict = policy(bandit, shell, bandit_run.returncode, shell_run.returncode)
    metrics = bandit.get('metrics')
    if not isinstance(metrics, dict) or set(metrics) - {'_totals'} != {str(p) for p in python_sources}:
        raise EvidenceError('bandit-source-inventory-mismatch')
    after = {str(path.relative_to(root)): sha(path) for path in sources}
    if after != before:
        raise EvidenceError('source-changed-during-scan')
    write_json(output / 'source-after.json', after)
    write_json(output / 'verdict.json', verdict)
    receipt = {'schema': 'asynq-native-source-sast/v1', 'generated_at': datetime.now(timezone.utc).isoformat(),
               'sources_sha256': before, 'tools': {'bandit_version': BANDIT_VERSION,
               'shellcheck_version': SHELLCHECK_VERSION, 'bandit_python_sha256': sha(bandit_python),
               'shellcheck_sha256': sha(shellcheck),
               'python_environment_files_sha256': {str(p): sha(p) for p in sorted((bandit_python.parent.parent / 'lib').rglob('*')) if p.is_file() and '__pycache__' not in p.parts}}, 'commands': commands, 'verdict': verdict,
               'runner_sha256': sha(Path(__file__)), 'source_before_after_equal': True,
               'materials_sha256': {str(p): sha(p) for p in sorted(output.rglob('*')) if p.is_file()},
               'scope': 'Native analyzers; findings retained. No global absence, coverage, hosted upload or cross-family review claim.'}
    write_json(output / 'receipt.json', receipt)
    return verdict['exit']

def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source-root', type=Path, default=Path(__file__).resolve().parents[2])
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--bandit-python', type=Path)
    parser.add_argument('--shellcheck', type=Path)
    args = parser.parse_args(argv)
    root = args.source_root.resolve()
    output = args.out.resolve()
    if not output.is_relative_to(root / '.security') or output.exists():
        print(json.dumps({'error': 'output-must-be-new-workspace-security-directory', 'exit': 2}))
        return 2
    output.mkdir(parents=True)
    try:
        return scan(root, output, args.bandit_python or root / '.security/analyzers/bandit-1.9.4/venv/bin/python', args.shellcheck or root / '.security/analyzers/shellcheck-v0.11.0/shellcheck')
    except (OSError, UnicodeError) as exc:
        write_json(output / 'failure.json', {'error': 'source-or-output-unavailable', 'exit': 2})
        print(json.dumps({'error': 'source-or-output-unavailable', 'exit': 2}))
        return 2
    except EvidenceError as exc:
        write_json(output / 'failure.json', {'error': str(exc), 'exit': 2})
        print(json.dumps({'error': str(exc), 'exit': 2}))
        return 2

if __name__ == '__main__':
    raise SystemExit(main())
