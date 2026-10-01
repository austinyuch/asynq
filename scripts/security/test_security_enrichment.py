"""Exercise real security CLI enrichment; inspect plans without executing them."""
import hashlib
import json
import os
from pathlib import Path
import random
import subprocess
import sys
import tempfile
import unittest

SUBJECT = Path(os.environ.get(
    "ASYNQ_SECURITY_SUBJECT", str(Path(__file__).with_name("security_local_ci.py"))
))


def run_case(subject: Path, seed: int, known_first: bool = False) -> dict:
    """Each seed independently determines a nonconflicting valid input domain."""
    rng = random.Random(seed)
    route, label = ((".", "application"), ("x", "application-x"),
                    ("tools", "application-tools"))[seed % 3]
    cve = f"CVE-2026-{10000 + seed}"
    module = f"example.com/dependency{seed}"
    version = f"v{rng.randrange(1, 9)}.{rng.randrange(10)}.{rng.randrange(10)}"
    fixed = "v9.0.0"
    ref = f"pkg:golang/{module}@{version}"
    with tempfile.TemporaryDirectory() as temporary:
        directory = Path(temporary)
        correlation = directory / "correlation.json"
        sparse = directory / "sparse.json"
        enriched = directory / f"{label}.json"
        correlation.write_text(json.dumps({"cve_id": cve, "kev_status": "listed"}),
                               encoding="utf-8")
        sparse_payload = {"vulnerabilities": [{"id": cve}]}
        if known_first:
            sparse_payload = {"components": [{"bom-ref": ref, "name": module, "version": ""}],
                              "vulnerabilities": [{"id": cve, "affects": [{"ref": ref}]}]}
        sparse.write_text(json.dumps(sparse_payload),
                          encoding="utf-8")
        enriched.write_text(json.dumps({
            "components": [{"bom-ref": ref, "name": module, "version": version}],
            "vulnerabilities": [{
                "id": cve,
                "affects": [{"ref": ref}],
                "ratings": [{"severity": "high"}],
                "recommendation": f"Upgrade {module} to version {fixed}",
            }],
        }), encoding="utf-8")
        verdict_path = directory / "verdict.json"
        summary_path = directory / "summary.txt"
        plan_path = directory / "fix.sh"
        command = [
            sys.executable, str(subject), "gate", "--correlation", str(correlation),
            "--sbom", str(sparse), str(enriched), "--out-verdict", str(verdict_path),
            "--out-summary", str(summary_path), "--out-fix-plan", str(plan_path),
        ]
        result = subprocess.run(command, capture_output=True, timeout=10)
        assert result.returncode == 1 and result.stderr == b"", "gate disposition"
        verdict = json.loads(verdict_path.read_text(encoding="utf-8"))
        assert verdict["verdict"] == "block", "KEV verdict"
        assert len(verdict["blocking"]) == 1 and verdict["warnings"] == [], "KEV partition"
        item = verdict["blocking"][0]
        assert item["reason"] == "kev-listed" and item["cve_ids"] == [cve], "KEV identity"
        actual = (item["package_module"], item["version"], item["fixed_version"],
                  item["severity"], item["module_dir"])
        assert actual == (module, version, fixed, "high", route), "metadata enrichment/route"
        assert verdict["fix_plan"] == {route: {module: fixed}}, "fix plan ownership"
        plan = plan_path.read_text(encoding="utf-8")
        assert f"go get {module}@{fixed}" in plan, "plan dependency/version"
        assert f'cd -- "$repo_root"/{route}\n' in plan, "plan directory"
        assert plan.count("go get ") == 1 and plan.count("go mod tidy") == 1, "plan cardinality"
        assert result.stdout == summary_path.read_bytes(), "stdout/summary parity"
        return {
            "seed": seed, "route": route, "exit": result.returncode,
            "command": command,
            "input_sha256": {
                path.name: hashlib.sha256(path.read_bytes()).hexdigest()
                for path in (correlation, sparse, enriched)
            },
            "plan_content_asserted_not_executed": True,
        }


class MetadataEnrichmentContracts(unittest.TestCase):
    def test_generated_sparse_enrichment_and_kev_routes(self):
        for seed in range(64):
            with self.subTest(seed=seed):
                run_case(SUBJECT, seed)


    def test_known_module_missing_version_enrichment(self):
        for seed in range(64):
            with self.subTest(seed=seed):
                run_case(SUBJECT, seed, known_first=True)

    def test_cross_module_version_must_not_be_borrowed(self):
        for seed in range(64):
            with self.subTest(seed=seed), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                first = root / "application.json"
                second = root / "application-later.json"
                cve = f"CVE-2026-{20000+seed}"
                module_a, module_b = f"example.com/known{seed}", f"example.com/other{seed}"
                for path, module, version in [(first, module_a, ""), (second, module_b, "v2.0.0")]:
                    path.write_text(json.dumps({"components": [{"bom-ref": "ref", "name": module, "version": version}],
                        "vulnerabilities": [{"id": cve, "affects": [{"ref": "ref"}]}]}), encoding="utf-8")
                # Direct public module function in a separate interpreter, not a reimplementation.
                code = "import runpy,json,sys; n=runpy.run_path(sys.argv[1]); print(json.dumps(n['sbom_cve_map'](sys.argv[2:])))"
                result = subprocess.run([sys.executable, "-c", code, str(SUBJECT), str(first), str(second)],
                                        capture_output=True, timeout=10)
                self.assertEqual(result.returncode, 0, "direct map disposition")
                self.assertEqual(result.stderr, b"", "direct map diagnostics")
                entry = json.loads(result.stdout)[cve]
                self.assertEqual((entry["module"], entry["version"]), (module_a, ""),
                                 "cross-module version identity assertion")


    def test_existing_version_is_preserved(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            paths = [directory / "first.json", directory / "later.json"]
            cve, module = "CVE-2026-30001", "example.com/existing"
            for path, version in zip(paths, ("v1.2.3", "v2.0.0")):
                path.write_text(json.dumps({
                    "components": [{"bom-ref": "ref", "name": module,
                                    "version": version}],
                    "vulnerabilities": [{"id": cve, "affects": [{"ref": "ref"}]}],
                }), encoding="utf-8")
            code = ("import runpy,json,sys; n=runpy.run_path(sys.argv[1]); "
                    "print(json.dumps(n['sbom_cve_map'](sys.argv[2:])))")
            result = subprocess.run(
                [sys.executable, "-c", code, str(SUBJECT), *(str(p) for p in paths)],
                capture_output=True, timeout=10,
            )
            self.assertEqual(result.returncode, 0, "direct map disposition")
            self.assertEqual(result.stderr, b"", "direct map diagnostics")
            entry = json.loads(result.stdout)[cve]
            self.assertEqual((entry["module"], entry["version"]),
                             (module, "v1.2.3"), "existing version identity assertion")


if __name__ == "__main__":
    unittest.main()
