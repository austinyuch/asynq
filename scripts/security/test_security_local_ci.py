"""Execute generated fix plans with fake tools; never mutate real modules."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

spec = importlib.util.spec_from_file_location(
    'security_local_ci', Path(__file__).with_name('security_local_ci.py'))
ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ci)


class FixPlanTests(unittest.TestCase):
    def execute(self, fixes):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            bindir = root / 'bin'
            bindir.mkdir()
            calls = root / 'calls.jsonl'
            for directory in fixes:
                (root / directory).mkdir(parents=True, exist_ok=True)
            git = bindir / 'git'
            git.write_text('#!/bin/sh\nprintf "%s\\n" "$TEST_ROOT"\n')
            git.chmod(0o755)
            go = bindir / 'go'
            go.write_text(
                '#!/usr/bin/env python3\nimport os,json,sys\n'
                'with open(os.environ["TEST_CALLS"],"a") as f:\n'
                ' f.write(json.dumps({"args":sys.argv[1:],"cwd":os.getcwd()})+"\\n")\n')
            go.chmod(0o755)
            plan = root / 'fix.sh'
            ci.write_fix_plan(plan, fixes)
            env = dict(os.environ, PATH=str(bindir) + os.pathsep + os.environ['PATH'],
                       TEST_ROOT=str(root), TEST_CALLS=str(calls))
            result = subprocess.run(['bash', str(plan)], env=env, cwd=root,
                                    capture_output=True, text=True)
            recorded = [json.loads(line) for line in calls.read_text().splitlines()] if calls.exists() else []
            return result, recorded, plan.read_text(), (root / 'INJECTED').exists()

    def test_empty_plan_is_successful_noop(self):
        for fixes in ({}, {'.': {}}):
            result, calls, _, _ = self.execute(fixes)
            self.assertEqual(result.returncode, 0)
            self.assertEqual(calls, [])
            self.assertIn('nothing to bump', result.stdout)

    def test_runtime_advisories_block_dependency_mutation(self):
        for runtime in ('stdlib', 'toolchain'):
            with self.subTest(runtime=runtime):
                result, calls, script, _ = self.execute(
                    {'.': {runtime: 'v1.26.6', 'example.com/dependency': 'v2.0.0'}})
                self.assertEqual(result.returncode, 2)
                self.assertEqual(calls, [])
                self.assertIn('patched Go runtime at v1.26.6', result.stderr)
                self.assertIn('rerun scripts/security/run.sh', result.stderr)
                self.assertNotIn('go get ' + runtime, script)
                self.assertIn('go get example.com/dependency@v2.0.0', script)

    def test_regular_dependencies_remain_executable_per_module(self):
        result, calls, _, _ = self.execute(
            {'.': {'example.com/a': 'v1.2.3'}, 'tools': {'example.com/b': 'v2.0.0'}})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([c['args'] for c in calls], [
            ['get', 'example.com/a@v1.2.3'], ['mod', 'tidy'],
            ['get', 'example.com/b@v2.0.0'], ['mod', 'tidy']])
        self.assertTrue(calls[2]['cwd'].endswith('/tools'))

    def test_untrusted_paths_and_versions_are_literal_shell_arguments(self):
        directory = 'tools $(touch INJECTED)'
        module = 'example.com/a;touch INJECTED;#'
        version = "v1.0.0'$(touch INJECTED)"
        result, calls, _, injected = self.execute({directory: {module: version}})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(injected)
        self.assertEqual(calls[0]['args'], ['get', module + '@' + version])
        self.assertTrue(calls[0]['cwd'].endswith('/' + directory))

    def test_runtime_guidance_is_not_interpreted_as_shell_code(self):
        result, calls, _, injected = self.execute(
            {'.': {'stdlib': 'v1.26.6$(touch INJECTED)'}})
        self.assertEqual(result.returncode, 2)
        self.assertEqual(calls, [])
        self.assertFalse(injected)
        self.assertIn('$(touch INJECTED)', result.stderr)

    def test_summary_distinguishes_runtime_from_dependency_fixes(self):
        summary = ci.render_summary({'kev_listed_cve_count': 0, 'blocking': [],
            'warnings': [], 'verdict': 'block', 'fix_plan': {
                '.': {'stdlib': 'v1.26.6', 'toolchain': 'v1.26.6',
                      'example.com/a': 'v1.2.3'}}})
        self.assertNotIn('go get stdlib', summary)
        self.assertNotIn('go get toolchain', summary)
        self.assertIn('select patched Go runtime v1.26.6', summary)
        self.assertIn('go get example.com/a@v1.2.3', summary)


if __name__ == '__main__':
    unittest.main()
