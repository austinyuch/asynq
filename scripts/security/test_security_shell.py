"""Offline shell process contracts; fixture scanners do not establish scan results.

Default unittest runs clean their own directories. Set ASYNQ_SHELL_TEST_WORK
only for a campaign that intentionally retains all generated evidence.
"""
import hashlib, json, os, pathlib, random, subprocess, tempfile, unittest
RUNS = []

def repository_root():
    configured = os.environ.get('ASYNQ_SHELL_TEST_ROOT')
    if configured:
        return pathlib.Path(configured)
    return pathlib.Path(__file__).resolve().parents[2]
FAKE = '#!/usr/bin/python3\nimport json,os,pathlib,sys\nname=pathlib.Path(sys.argv[0]).name;mode=os.environ.get("FIXTURE_MODE", "clean")\nif name=="go":\n print(os.environ["FIXTURE_GOPATH"]);sys.exit(0)\nif name=="curl": sys.exit(91)\nif name=="trivy":\n if mode=="trivy-fail":sys.exit(3)\n target=pathlib.Path(sys.argv[sys.argv.index("--output")+1])\n with (target.parent.parent / "trivy-invocations.jsonl").open("a", encoding="utf-8") as log:log.write(json.dumps({"cwd":str(pathlib.Path.cwd()),"argv":sys.argv[1:]})+"\\n")\n target.write_text("[]" if mode=="trivy-malformed" else json.dumps({"bomFormat":"CycloneDX","specVersion":"1.5","version":1,"components":[]}));sys.exit(0)\nif name=="govulncheck":\n if mode=="govuln-fail":sys.exit(3)\n print("{" if mode=="govuln-malformed" else "");sys.exit(0)\nif name=="gosec":\n with (pathlib.Path(os.environ["ASYNQ_SECURITY_DIR"]) / "gosec-invocations.jsonl").open("a", encoding="utf-8") as log:log.write(json.dumps({"cwd":str(pathlib.Path.cwd()),"argv":sys.argv[1:]})+"\\n")\n if mode=="gosec-malformed":print("not-json");sys.exit(3)\n issues=[]\n if mode=="gosec-high":issues=[{"rule_id":"G101","severity":"HIGH","confidence":"HIGH","file":"fixture.go","line":"1","details":"synthetic fixture","cwe":{"id":"798"}}]\n print(json.dumps({"Issues":issues,"Stats":{"files":1,"lines":1,"nosec":0}}));sys.exit(1 if issues else 0)\n'

class Contracts(unittest.TestCase):

    def run_case(self, mode='clean', name='plain', cache=True, args=('--offline',), script=None, version='fixture'):
        root = repository_root()
        artifact_root = os.environ.get('ASYNQ_SHELL_TEST_WORK')
        if artifact_root:
            pathlib.Path(artifact_root).mkdir(parents=True, exist_ok=True)
            case = pathlib.Path(tempfile.mkdtemp(prefix='case-', dir=artifact_root))
        else:
            temporary = tempfile.TemporaryDirectory(prefix='asynq-shell-contracts-')
            self.addCleanup(temporary.cleanup)
            case = pathlib.Path(temporary.name)
        script = pathlib.Path(script or os.environ.get('SHELL_CANDIDATE', str(root / 'scripts/security/run.sh')))
        tools = case / 'bin'
        tools.mkdir()
        dispatcher = tools / 'dispatcher'
        dispatcher.write_text(FAKE)
        dispatcher.chmod(493)
        for tool in ('go', 'curl', 'trivy', 'govulncheck', 'gosec'):
            (tools / tool).symlink_to(dispatcher)
        skill = case / 'fixture-skill' / 'scripts'
        skill.mkdir(parents=True)
        for leaf in ('security_cve_kev_correlate.py', 'security_build_cve_catalog.py'):
            (skill / leaf).write_text('raise SystemExit(93)\n')
        out = case / name
        if cache:
            ev = out / 'evidence'
            ev.mkdir(parents=True)
            (ev / 'known_exploited_vulnerabilities.json').write_text(json.dumps({'catalogVersion': version, 'dateReleased': '2026-10-01', 'count': 0, 'vulnerabilities': []}))
        env = os.environ.copy()
        env.update(PATH=str(tools) + ':' + env['PATH'], ASYNQ_SECURITY_DIR=str(out), KEV_SBOM_SKILL_DIR=str(skill.parent), ACLAB_SECURITY_DATA_REGISTRY=str(case / 'absent-registry'), FIXTURE_MODE=mode, FIXTURE_GOPATH=str(case / 'gopath'))
        if os.environ.get('ASYNQ_SHELL_DEBUG_ENV'):
            env.update(BASH_ENV=os.environ['ASYNQ_SHELL_DEBUG_ENV'], ASYNQ_SHELL_DEBUG_LOG=str(case / 'line-entries.txt'))
        p = subprocess.run(['/bin/bash', str(script), *args], cwd=root, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30)
        (case / 'stdout.log').write_bytes(p.stdout)
        (case / 'stderr.log').write_bytes(p.stderr)
        RUNS.append({'mode': mode, 'name': name, 'exit': p.returncode, 'stdout_sha256': hashlib.sha256(p.stdout).hexdigest(), 'stderr_sha256': hashlib.sha256(p.stderr).hexdigest(), 'out': str(out), 'entries': str(case / 'line-entries.txt')})
        return (p, out)

    def test_clean_and_quiet(self):
        for args in (('--offline',), ('--offline', '--quiet')):
            p, out = self.run_case(args=args)
            self.assertEqual(p.returncode, 0, p.stderr.decode())
            self.assert_provenance(out, 'fixture')
            self.assertTrue((out / 'report/verdict.json').is_file())
            if '--quiet' in args:
                self.assertNotIn(b'[asynq-root] SBOM', p.stdout)

    def test_controlled_failures(self):
        for mode in ('trivy-fail', 'trivy-malformed', 'govuln-fail', 'govuln-malformed', 'gosec-malformed'):
            with self.subTest(mode=mode):
                p, out = self.run_case(mode)
                self.assertEqual(p.returncode, 2)
                self.assertFalse((out / 'report/verdict.json').exists())

    def test_sast_policy(self):
        p, out = self.run_case('gosec-high')
        self.assertEqual(p.returncode, 1, p.stderr.decode())
        verdict = json.loads((out / 'report/verdict.json').read_text())
        self.assertEqual(verdict['sast']['finding_count'], 3)
        self.assertEqual(len(verdict['blocking']), 3)
        self.assertTrue(all((row['severity'] == 'HIGH' for row in verdict['blocking'])))

    def test_missing_cache(self):
        p, out = self.run_case(cache=False)
        self.assertEqual(p.returncode, 2)
        self.assertIn(b'no cached KEV', p.stderr)
        self.assertFalse((out / 'report/verdict.json').exists())

    def test_unknown_args(self):
        p, out = self.run_case(args=('--unknown-fixture',))
        self.assertEqual(p.returncode, 2)
        self.assertIn(b'unknown argument', p.stderr)
        self.assertFalse((out / 'report/verdict.json').exists())

    def test_literal_paths(self):
        rng = random.Random(20261001)
        names = ['double"quote', "single'quote", 'space path', 'unicode-測試', 'back\\slash'] + ['path-' + ''.join((rng.choice('ab _-"\'測') for _ in range(10))) for i in range(12)]
        for name in names:
            with self.subTest(name=name):
                version = 'catalog-' + name
                p, out = self.run_case(name=name, version=version)
                self.assertEqual(p.returncode, 0, p.stderr.decode())
                self.assert_provenance(out, version)

    def test_sbom_evidence_directory_exclusion(self):
        """Check the actual three production scanner invocations, not a clone."""
        process, out = self.run_case()
        self.assertEqual(process.returncode, 0, process.stderr)
        rows = [json.loads(line) for line in
                (out / 'trivy-invocations.jsonl').read_text().splitlines()]
        self.assertEqual(len(rows), 3)
        root = repository_root().resolve()
        expected = {str(root): {'./.security', './x', './tools'},
                    str(root / 'x'): {'./.security'},
                    str(root / 'tools'): {'./.security'}}
        self.assertEqual({row['cwd'] for row in rows}, set(expected))
        for row in rows:
            with self.subTest(module=row['cwd']):
                args = row['argv']
                exclusions = {args[index + 1] for index, arg in enumerate(args)
                              if arg == '--skip-dirs'}
                self.assertEqual(exclusions, expected[row['cwd']])
                self.assertEqual(args[-1], '.')
                self.assertEqual(args[args.index('--scanners') + 1], 'vuln')

    def test_sast_evidence_directory_exclusion(self):
        process, out = self.run_case()
        self.assertEqual(process.returncode, 0, process.stderr)
        rows = [json.loads(line) for line in
                (out / 'gosec-invocations.jsonl').read_text().splitlines()]
        root = repository_root().resolve()
        self.assertEqual(len(rows), 3)
        self.assertEqual({row['cwd'] for row in rows},
                         {str(root), str(root / 'x'), str(root / 'tools')})
        for row in rows:
            self.assertEqual(row['argv'], ['-fmt=json', '-exclude-generated',
                                          '-exclude-dir', r'(^|/)\.security(/|$)', './...'])

    def test_literal_repository_prefix_display(self):
        """Exercise the actual Bash diagnostic with literal path data."""
        subject = pathlib.Path(os.environ.get(
            'SHELL_CANDIDATE', str(repository_root() / 'scripts/security/run.sh')))
        statements = [line for line in subject.read_text().splitlines()
                      if line.startswith("printf '  evidence:")]
        self.assertEqual(len(statements), 1, 'production diagnostic statement')
        rng = random.Random(20261002)
        roots = ['/tmp/normal', '/tmp/path[ab]', '/tmp/path*star',
                 '/tmp/path?mark', '/tmp/quote"測試', '/tmp/back\\slash']
        roots += ['/tmp/' + ''.join(rng.choice('ab []?*測\\"')
                                   for _ in range(10)) for _ in range(32)]
        for root in roots:
            with self.subTest(root=root):
                env = dict(os.environ, REPO_ROOT=root,
                           OUT_DIR=root + '/.security/evidence')
                result = subprocess.run(['/bin/bash', '-c', statements[0]],
                                        env=env, capture_output=True, timeout=10)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout, b'  evidence: .security/evidence\n\n')
                self.assertEqual(result.stderr, b'')

    def assert_provenance(self, out, version):
        provenance = json.loads((out / 'evidence/provenance.json').read_text())
        timestamp = provenance.pop('generated_at')
        self.assertRegex(timestamp, '^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}Z$')
        from datetime import datetime
        datetime.strptime(timestamp, '%Y-%m-%dT%H:%M:%SZ')
        self.assertEqual(provenance, {'schema': 'asynq-catalog-provenance/v1', 'kev_catalog': {'source_uri': 'https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json', 'acquisition': 'consumer-fallback', 'provider_mode': 'absent', 'retrieved_at': 'cached', 'sha256': hashlib.sha256((out / 'evidence/known_exploited_vulnerabilities.json').read_bytes()).hexdigest(), 'catalog_version': version, 'date_released': '2026-10-01', 'count': 0, 'completeness': 'not-asserted-by-source-schema'}, 'cve_catalog': {'source_uri': 'https://cveawg.mitre.org/api/cve/<CVE-ID>', 'acquisition': 'consumer-fallback', 'mode': 'scoped', 'sha256': '', 'note': "Scoped to the CVE IDs discovered by the local scanners. 'not_present_in_supplied_snapshot' is therefore uninformative and never a global absence claim."}})
if __name__ == '__main__':
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(Contracts))
    artifact_root = os.environ.get('ASYNQ_SHELL_TEST_WORK')
    if artifact_root:
        pathlib.Path(artifact_root, 'runs.json').write_text(json.dumps({'runs': RUNS, 'success': result.wasSuccessful()}, indent=2))
    raise SystemExit(not result.wasSuccessful())
