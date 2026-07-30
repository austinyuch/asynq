#!/usr/bin/env python3
"""Evidence normalization, gating, and fix planning for the local security CI.

This script owns the *interpretation* half of the pipeline. Acquisition (trivy,
govulncheck, CISA KEV, CVE List V5 records) lives in run.sh, and CVE/KEV catalog
correlation is delegated to the kev-sbom-correlation skill's deterministic
engine. Nothing here fetches over the network or executes a scanner.

Subcommands:
  normalize   govulncheck JSON stream -> CycloneDX evidence + findings receipt.
              Resolves GO-xxxx-xxxx advisory IDs to their CVE aliases, which the
              correlation engine deliberately does not do (its documented recall
              boundary), and classifies each finding's reachability.
  cve-ids     Print the union of CVE IDs across CycloneDX documents, so run.sh
              knows which CVE List V5 records to fetch for the scoped catalog.
  gate        Apply the block/warn policy and emit verdict, summary, fix plan.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any, Iterator

CVE_RE = re.compile(r"\bCVE-\d{4}-\d{4,19}\b", re.IGNORECASE)
TRIVY_FIX_RE = re.compile(
    r"[Uu]pgrade\s+(?P<module>\S+)\s+to\s+version\s+v?(?P<version>[0-9][^\s,;]*)"
)

# Reachability, most to least severe. govulncheck emits a separate finding per
# precision level for the same advisory, so we keep the strongest one.
REACH_SYMBOL = "called"
REACH_PACKAGE = "imported-not-called"
REACH_MODULE = "required-not-imported"
REACH_RANK = {REACH_MODULE: 0, REACH_PACKAGE: 1, REACH_SYMBOL: 2}


def die(message: str) -> None:
    print(f"security-local-ci: {message}", file=sys.stderr)
    raise SystemExit(2)


def read_json(path: Path) -> Any:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except OSError as exc:
        die(f"cannot read {path}: {exc}")
    except json.JSONDecodeError as exc:
        die(f"{path} is not valid JSON: {exc}")


def iter_json_stream(path: Path) -> Iterator[dict[str, Any]]:
    """Yield each object from govulncheck's concatenated JSON output."""
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as exc:
        die(f"cannot read {path}: {exc}")
    decoder = json.JSONDecoder()
    index = 0
    length = len(text)
    while index < length:
        while index < length and text[index].isspace():
            index += 1
        if index >= length:
            return
        try:
            value, index = decoder.raw_decode(text, index)
        except json.JSONDecodeError as exc:
            die(f"{path} is not a valid govulncheck JSON stream: {exc}")
        if isinstance(value, dict):
            yield value


def write_json(path: Path, payload: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(payload, indent=2, sort_keys=True, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )


# --------------------------------------------------------------------------
# normalize
# --------------------------------------------------------------------------


def classify_trace(trace: list[dict[str, Any]]) -> str:
    if any(entry.get("function") for entry in trace if isinstance(entry, dict)):
        return REACH_SYMBOL
    if any(entry.get("package") for entry in trace if isinstance(entry, dict)):
        return REACH_PACKAGE
    return REACH_MODULE


def normalize(args: argparse.Namespace) -> int:
    source = Path(args.govulncheck)
    advisories: dict[str, dict[str, Any]] = {}
    raw_findings: list[dict[str, Any]] = []

    for entry in iter_json_stream(source):
        if "osv" in entry and isinstance(entry["osv"], dict):
            osv = entry["osv"]
            osv_id = osv.get("id")
            if isinstance(osv_id, str) and osv_id.strip():
                aliases = [a for a in osv.get("aliases") or [] if isinstance(a, str)]
                advisories[osv_id.strip()] = {
                    "aliases": aliases,
                    "summary": osv.get("summary") or "",
                }
        if "finding" in entry and isinstance(entry["finding"], dict):
            raw_findings.append(entry["finding"])

    # Collapse to one record per (advisory, module), keeping strongest reachability.
    collapsed: dict[tuple[str, str], dict[str, Any]] = {}
    for finding in raw_findings:
        osv_id = finding.get("osv")
        if not isinstance(osv_id, str) or not osv_id.strip():
            continue
        osv_id = osv_id.strip()
        trace = finding.get("trace") if isinstance(finding.get("trace"), list) else []
        module = ""
        version = ""
        for entry in trace:
            if isinstance(entry, dict) and entry.get("module"):
                module = str(entry["module"])
                version = str(entry.get("version") or "")
                break
        if not module:
            continue
        reach = classify_trace(trace)
        packages = sorted(
            {
                str(entry["package"])
                for entry in trace
                if isinstance(entry, dict) and entry.get("package")
            }
        )
        symbols = sorted(
            {
                str(entry["function"])
                for entry in trace
                if isinstance(entry, dict) and entry.get("function")
            }
        )
        key = (osv_id, module)
        existing = collapsed.get(key)
        if existing is None:
            meta = advisories.get(osv_id, {})
            cves = sorted(
                {
                    alias.upper()
                    for alias in meta.get("aliases", [])
                    if CVE_RE.fullmatch(alias)
                }
            )
            collapsed[key] = {
                "advisory_id": osv_id,
                "cve_ids": cves,
                "summary": meta.get("summary", ""),
                "module": module,
                "version": version,
                "fixed_version": finding.get("fixed_version") or "",
                "reachability": reach,
                "packages": packages,
                "symbols": symbols,
            }
        else:
            if REACH_RANK[reach] > REACH_RANK[existing["reachability"]]:
                existing["reachability"] = reach
            existing["packages"] = sorted(set(existing["packages"]) | set(packages))
            existing["symbols"] = sorted(set(existing["symbols"]) | set(symbols))
            if not existing["fixed_version"] and finding.get("fixed_version"):
                existing["fixed_version"] = finding["fixed_version"]

    findings = sorted(
        collapsed.values(), key=lambda f: (f["module"], f["advisory_id"])
    )

    # CycloneDX evidence for the correlation engine. The CVE alias becomes the
    # primary id so the engine can match it; the GO id is retained as a
    # reference for traceability.
    components: dict[str, dict[str, Any]] = {}
    vulnerabilities: list[dict[str, Any]] = []
    for finding in findings:
        purl = f"pkg:golang/{finding['module']}@{finding['version']}"
        components.setdefault(
            purl,
            {
                "bom-ref": purl,
                "type": "library",
                "name": finding["module"],
                "version": finding["version"],
                "purl": purl,
            },
        )
        affects = [
            {
                "ref": purl,
                "versions": [{"version": finding["version"], "status": "affected"}],
            }
        ]
        primary_ids = finding["cve_ids"] or [finding["advisory_id"]]
        for primary in primary_ids:
            references = [{"id": finding["advisory_id"]}]
            references += [
                {"id": cve} for cve in finding["cve_ids"] if cve != primary
            ]
            vulnerabilities.append(
                {
                    "id": primary,
                    "affects": affects,
                    "references": references,
                    "description": finding["summary"],
                }
            )

    write_json(
        Path(args.out_cdx),
        {
            "bomFormat": "CycloneDX",
            "specVersion": "1.6",
            "version": 1,
            "metadata": {
                "component": {
                    "type": "application",
                    "name": args.label,
                },
                "properties": [
                    {"name": "asynq:evidence-source", "value": "govulncheck"},
                    {"name": "asynq:alias-resolved", "value": "true"},
                ],
            },
            "components": [components[key] for key in sorted(components)],
            "vulnerabilities": vulnerabilities,
        },
    )

    write_json(
        Path(args.out_findings),
        {
            "schema": "asynq-govulncheck-findings/v1",
            "label": args.label,
            "module_dir": args.module_dir,
            "advisory_count": len(advisories),
            "findings": findings,
        },
    )
    return 0


# --------------------------------------------------------------------------
# cve-ids
# --------------------------------------------------------------------------


def sast_normalize(args: argparse.Namespace) -> int:
    """gosec JSON -> a findings receipt keyed by rule and CWE."""
    data = read_json(Path(args.gosec))
    if not isinstance(data, dict):
        die(f"{args.gosec}: gosec output must be a JSON object")

    issues = data.get("Issues")
    if issues is None:
        # gosec emits no Issues key when it finds nothing. It exited cleanly, so
        # this is a completed scan with zero findings, not missing evidence.
        issues = []
    if not isinstance(issues, list):
        die(f"{args.gosec}: Issues must be a list")

    findings: list[dict[str, Any]] = []
    for issue in issues:
        if not isinstance(issue, dict):
            continue
        cwe = issue.get("cwe") or {}
        cwe_id = ""
        if isinstance(cwe, dict) and cwe.get("id"):
            cwe_id = f"CWE-{str(cwe['id']).strip()}"
        findings.append(
            {
                "rule_id": str(issue.get("rule_id") or ""),
                "cwe": cwe_id,
                "severity": str(issue.get("severity") or "").upper(),
                "confidence": str(issue.get("confidence") or "").upper(),
                "file": str(issue.get("file") or ""),
                "line": str(issue.get("line") or ""),
                "details": str(issue.get("details") or ""),
            }
        )
    findings.sort(key=lambda f: (f["file"], f["line"], f["rule_id"]))

    stats = data.get("Stats") if isinstance(data.get("Stats"), dict) else {}
    write_json(
        Path(args.out_findings),
        {
            "schema": "asynq-sast-findings/v1",
            "label": args.label,
            "module_dir": args.module_dir,
            "tool": "gosec",
            "scanned_files": stats.get("files"),
            "scanned_lines": stats.get("lines"),
            "nosec_suppressions": stats.get("nosec"),
            "finding_count": len(findings),
            "findings": findings,
        },
    )
    return 0


def kev_cwe_index(path: Path) -> dict[str, int]:
    """CWE -> number of CISA KEV entries citing it, from the supplied snapshot."""
    data = read_json(path)
    if not isinstance(data, dict):
        return {}
    counts: dict[str, int] = {}
    for vuln in data.get("vulnerabilities") or []:
        if not isinstance(vuln, dict):
            continue
        for cwe in vuln.get("cwes") or []:
            if isinstance(cwe, str) and cwe.strip():
                key = cwe.strip().upper()
                counts[key] = counts.get(key, 0) + 1
    return counts


def cve_ids(args: argparse.Namespace) -> int:
    found: set[str] = set()
    for raw in args.inputs:
        path = Path(raw)
        if not path.is_file():
            continue
        data = read_json(path)
        if not isinstance(data, dict):
            continue
        for vulnerability in data.get("vulnerabilities") or []:
            if not isinstance(vulnerability, dict):
                continue
            candidates = [vulnerability.get("id")]
            for reference in vulnerability.get("references") or []:
                if isinstance(reference, dict):
                    candidates.append(reference.get("id"))
            for candidate in candidates:
                if isinstance(candidate, str) and CVE_RE.fullmatch(candidate.strip()):
                    found.add(candidate.strip().upper())
    for cve in sorted(found):
        print(cve)
    return 0


# --------------------------------------------------------------------------
# gate
# --------------------------------------------------------------------------


def kev_listed_cves(correlation: dict[str, Any]) -> dict[str, dict[str, Any]]:
    """Extract CVE -> KEV detail for every kev_status=listed route."""
    listed: dict[str, dict[str, Any]] = {}

    def walk(node: Any) -> None:
        if isinstance(node, dict):
            cve = node.get("cve_id") or node.get("cveID") or node.get("id")
            if (
                isinstance(cve, str)
                and CVE_RE.fullmatch(cve.strip())
                and node.get("kev_status") == "listed"
            ):
                listed[cve.strip().upper()] = {
                    "cve_id": cve.strip().upper(),
                    "known_ransomware_campaign_use": node.get(
                        "known_ransomware_campaign_use"
                    )
                    or node.get("knownRansomwareCampaignUse")
                    or "",
                    "cve_catalog_status": node.get("cve_catalog_status") or "",
                    "projects": node.get("projects") or node.get("project") or "",
                }
            for value in node.values():
                walk(value)
        elif isinstance(node, list):
            for value in node:
                walk(value)

    walk(correlation)
    return listed


def sbom_cve_map(paths: list[str]) -> dict[str, dict[str, Any]]:
    """CVE -> {label, module, version, fixed} from trivy CycloneDX SBOMs."""
    result: dict[str, dict[str, Any]] = {}
    for raw in paths:
        path = Path(raw)
        if not path.is_file():
            continue
        data = read_json(path)
        if not isinstance(data, dict):
            continue
        label = path.name.split(".")[0]
        refs: dict[str, dict[str, Any]] = {}
        for component in data.get("components") or []:
            if isinstance(component, dict) and component.get("bom-ref"):
                refs[str(component["bom-ref"])] = component
        for vulnerability in data.get("vulnerabilities") or []:
            if not isinstance(vulnerability, dict):
                continue
            vid = vulnerability.get("id")
            if not isinstance(vid, str) or not CVE_RE.fullmatch(vid.strip()):
                continue
            vid = vid.strip().upper()
            severity = ""
            for rating in vulnerability.get("ratings") or []:
                if isinstance(rating, dict) and rating.get("severity"):
                    severity = str(rating["severity"])
                    break
            module = ""
            version = ""
            for affect in vulnerability.get("affects") or []:
                if not isinstance(affect, dict):
                    continue
                component = refs.get(str(affect.get("ref") or ""))
                if component:
                    module = str(component.get("name") or "")
                    version = str(component.get("version") or "")
                    break
            fixed = ""
            recommendation = vulnerability.get("recommendation")
            if isinstance(recommendation, str):
                match = TRIVY_FIX_RE.search(recommendation)
                if match:
                    fixed = "v" + match.group("version")
            entry = result.setdefault(
                vid,
                {
                    "cve_id": vid,
                    "labels": [],
                    "module": module,
                    "version": version,
                    "fixed_version": fixed,
                    "severity": severity,
                },
            )
            if label not in entry["labels"]:
                entry["labels"].append(label)
            if not entry["module"]:
                entry["module"] = module
                entry["version"] = version
            if not entry["fixed_version"]:
                entry["fixed_version"] = fixed
            if not entry["severity"]:
                entry["severity"] = severity
    return result


def gate(args: argparse.Namespace) -> int:
    correlation = read_json(Path(args.correlation))
    listed = kev_listed_cves(correlation)

    modules: list[dict[str, Any]] = []
    for raw in args.findings:
        path = Path(raw)
        if not path.is_file():
            continue
        data = read_json(path)
        if isinstance(data, dict):
            modules.append(data)

    sbom = sbom_cve_map(args.sbom or [])

    blocking: list[dict[str, Any]] = []
    warnings: list[dict[str, Any]] = []
    # module dir -> module path -> fixed version
    fixes: dict[str, dict[str, str]] = {}
    seen_cves: set[str] = set()

    def record_fix(module_dir: str, module: str, fixed: str) -> None:
        if not module_dir or not module or not fixed:
            return
        bucket = fixes.setdefault(module_dir, {})
        # Keep the highest requested version if several findings disagree.
        current = bucket.get(module)
        if current is None or version_key(fixed) > version_key(current):
            bucket[module] = fixed

    for entry in modules:
        module_dir = entry.get("module_dir") or "."
        label = entry.get("label") or module_dir
        for finding in entry.get("findings") or []:
            cves = finding.get("cve_ids") or []
            seen_cves.update(cves)
            kev_hits = [cve for cve in cves if cve in listed]
            fixed = finding.get("fixed_version") or ""
            reach = finding.get("reachability") or REACH_MODULE
            item = {
                "label": label,
                "module_dir": module_dir,
                "advisory_id": finding.get("advisory_id"),
                "cve_ids": cves,
                "summary": finding.get("summary") or "",
                "package_module": finding.get("module"),
                "version": finding.get("version"),
                "fixed_version": fixed,
                "reachability": reach,
                "packages": finding.get("packages") or [],
                "symbols": finding.get("symbols") or [],
                "kev_listed": kev_hits,
            }
            record_fix(module_dir, str(finding.get("module") or ""), fixed)
            if kev_hits:
                item["reason"] = "kev-listed"
                blocking.append(item)
            elif reach == REACH_SYMBOL and fixed:
                item["reason"] = "reachable-with-fix"
                blocking.append(item)
            elif reach == REACH_SYMBOL:
                item["reason"] = "reachable-no-fix-available"
                warnings.append(item)
            else:
                item["reason"] = f"not-reachable ({reach})"
                warnings.append(item)

    # CVEs only trivy saw (dependency present but govulncheck had no finding,
    # e.g. a non-Go component or an advisory absent from the Go vuln DB).
    for cve, entry in sorted(sbom.items()):
        if cve in seen_cves:
            continue
        item = {
            "label": ",".join(entry["labels"]),
            "module_dir": label_to_dir(entry["labels"]),
            "advisory_id": cve,
            "cve_ids": [cve],
            "summary": "",
            "package_module": entry["module"],
            "version": entry["version"],
            "fixed_version": entry["fixed_version"],
            "reachability": "not-assessed-by-govulncheck",
            "packages": [],
            "symbols": [],
            "kev_listed": [cve] if cve in listed else [],
            "severity": entry["severity"],
        }
        record_fix(item["module_dir"], entry["module"], entry["fixed_version"])
        if cve in listed:
            item["reason"] = "kev-listed"
            blocking.append(item)
        else:
            item["reason"] = "sbom-only (reachability not assessed)"
            warnings.append(item)

    # ---- SAST (first-party code) --------------------------------------------
    # SAST findings carry CWEs, never CVEs, so they cannot join the CVE->KEV
    # correlation. What they *can* join is CWE->KEV: how often this weakness
    # class shows up among actively exploited vulnerabilities. That is a
    # prioritization signal about the class, never evidence about this code.
    kev_cwes = kev_cwe_index(Path(args.kev_catalog)) if args.kev_catalog else {}
    sast_blocking: list[dict[str, Any]] = []
    sast_warnings: list[dict[str, Any]] = []
    sast_cwe_hist: dict[str, int] = {}
    sast_scanned = 0
    sast_suppressions = 0

    for raw in args.sast or []:
        path = Path(raw)
        if not path.is_file():
            continue
        entry = read_json(path)
        if not isinstance(entry, dict):
            continue
        sast_scanned += entry.get("scanned_files") or 0
        sast_suppressions += entry.get("nosec_suppressions") or 0
        for finding in entry.get("findings") or []:
            cwe = finding.get("cwe") or ""
            if cwe:
                sast_cwe_hist[cwe] = sast_cwe_hist.get(cwe, 0) + 1
            item = {
                "label": entry.get("label") or "",
                "module_dir": entry.get("module_dir") or ".",
                "rule_id": finding.get("rule_id"),
                "cwe": cwe,
                "severity": finding.get("severity"),
                "confidence": finding.get("confidence"),
                "location": f"{finding.get('file')}:{finding.get('line')}",
                "details": finding.get("details"),
                "kev_entries_for_cwe": kev_cwes.get(cwe, 0),
            }
            if item["severity"] == "HIGH":
                item["reason"] = "sast-high-severity"
                sast_blocking.append(item)
            else:
                item["reason"] = f"sast-{(item['severity'] or 'unknown').lower()}-severity"
                sast_warnings.append(item)

    blocking.extend(sast_blocking)
    warnings.extend(sast_warnings)

    sast_kev_correlation = sorted(
        (
            {
                "cwe": cwe,
                "sast_findings": count,
                "kev_entries_citing_cwe": kev_cwes.get(cwe, 0),
            }
            for cwe, count in sast_cwe_hist.items()
        ),
        key=lambda r: (-r["kev_entries_citing_cwe"], -r["sast_findings"], r["cwe"]),
    )

    verdict = "block" if blocking else "pass"
    payload = {
        "schema": "asynq-security-verdict/v1",
        "verdict": verdict,
        "policy": {
            "block_on_kev_listed": True,
            "block_on_reachable_with_fix": True,
            "block_on_sast_high_severity": True,
            "warn_on_reachable_without_fix": True,
            "warn_on_not_reachable": True,
            "warn_on_sast_medium_low_severity": True,
        },
        "kev_listed_cve_count": len(listed),
        "sast": {
            "tool": "gosec",
            "scanned_files": sast_scanned,
            "nosec_suppressions": sast_suppressions,
            "finding_count": len(sast_blocking) + len(sast_warnings),
            "kev_cwe_catalog_size": len(kev_cwes),
            "cwe_kev_correlation": sast_kev_correlation,
        },
        "blocking": blocking,
        "warnings": warnings,
        "fix_plan": {
            module_dir: dict(sorted(entries.items()))
            for module_dir, entries in sorted(fixes.items())
        },
        "disclosure": [
            "kev_status and cve_catalog_status come from the supplied immutable "
            "snapshots only; neither proves global presence or absence.",
            "A KEV-listed route is a prioritization candidate, not proof this "
            "target is affected. Confirm applicability with security-review "
            "before treating it as an incident.",
            "Reachability is govulncheck's static call-graph result. "
            "'not-reachable' is not a guarantee of safety.",
            "SAST findings carry CWEs, not CVEs, so they are never matched to "
            "KEV by identifier. kev_entries_for_cwe counts how often that "
            "weakness CLASS appears in the KEV snapshot: it ranks classes of "
            "risk and says nothing about whether this specific code is "
            "exploitable.",
            "gosec is a static analyzer: zero findings is not proof of "
            "absence, and nosec_suppressions are asserted by the code author, "
            "not verified here.",
        ],
    }
    write_json(Path(args.out_verdict), payload)
    write_fix_plan(Path(args.out_fix_plan), fixes)
    summary = render_summary(payload)
    Path(args.out_summary).parent.mkdir(parents=True, exist_ok=True)
    Path(args.out_summary).write_text(summary, encoding="utf-8")
    print(summary, end="")
    return 1 if verdict == "block" else 0


def label_to_dir(labels: list[str]) -> str:
    for label in labels:
        if label.endswith("-tools"):
            return "tools"
        if label.endswith("-x"):
            return "x"
    return "."


def version_key(value: str) -> tuple[int, ...]:
    cleaned = value.lstrip("v")
    parts: list[int] = []
    for chunk in re.split(r"[.\-+]", cleaned):
        if chunk.isdigit():
            parts.append(int(chunk))
        else:
            break
    return tuple(parts) or (0,)


def write_fix_plan(path: Path, fixes: dict[str, dict[str, str]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    lines = [
        "#!/usr/bin/env bash",
        "# Generated by scripts/security/run.sh -- do not edit by hand.",
        "# Minimum dependency bumps that clear the advisories found locally.",
        "# Review, then run:  make security-fix",
        "set -euo pipefail",
        "",
        'repo_root="$(git rev-parse --show-toplevel)"',
        "",
    ]
    if not fixes:
        lines += ['echo "security fix plan: nothing to bump"', ""]
    for module_dir in sorted(fixes):
        entries = fixes[module_dir]
        if not entries:
            continue
        lines.append(f'echo "==> {module_dir}"')
        lines.append(f'cd "$repo_root/{module_dir}"')
        for module in sorted(entries):
            lines.append(f"go get {module}@{entries[module]}")
        lines.append("go mod tidy")
        lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")
    path.chmod(0o755)


def render_summary(payload: dict[str, Any]) -> str:
    out: list[str] = []
    sast = payload.get("sast") or {}
    out.append("")
    out.append("  local security CI -- SBOM / CVE / KEV / SAST")
    out.append("  " + "-" * 56)
    kev_count = payload["kev_listed_cve_count"]
    out.append(f"  KEV-listed CVEs in this evidence : {kev_count}")
    out.append(f"  SAST findings ({sast.get('tool', 'sast')})            : "
               f"{sast.get('finding_count', 0)}"
               f"  [{sast.get('nosec_suppressions', 0)} documented nosec]")
    out.append(f"  blocking findings                : {len(payload['blocking'])}")
    out.append(f"  warnings                         : {len(payload['warnings'])}")
    out.append("")

    correlation = sast.get("cwe_kev_correlation") or []
    if correlation:
        out.append("  SAST CWE x KEV (weakness-class prioritization, not exploitability)")
        for row in correlation:
            out.append(
                f"    {row['cwe']:<10} {row['sast_findings']:>3} finding(s)"
                f"   {row['kev_entries_citing_cwe']:>4} KEV entries cite this CWE"
            )
        out.append("")

    if payload["blocking"]:
        out.append("  BLOCKING")
        for item in payload["blocking"]:
            out.append(f"    [{item['reason']}] {describe(item)}")
        out.append("")

    if payload["warnings"]:
        out.append("  WARNINGS (not blocking)")
        for item in payload["warnings"]:
            out.append(f"    [{item['reason']}] {describe(item)}")
        out.append("")

    if payload["fix_plan"]:
        out.append("  FIX PLAN (apply with: make security-fix)")
        for module_dir, entries in payload["fix_plan"].items():
            for module, version in entries.items():
                out.append(f"    ({module_dir}) go get {module}@{version}")
        out.append("")

    out.append(f"  verdict: {payload['verdict'].upper()}")
    out.append("")
    return "\n".join(out) + "\n"


def describe(item: dict[str, Any]) -> str:
    if item.get("rule_id"):
        # SAST finding: code location, not a dependency route.
        kev = item.get("kev_entries_for_cwe") or 0
        amp = f" [{kev} KEV entries cite {item['cwe']}]" if kev else ""
        return (
            f"{item['rule_id']} {item.get('cwe') or 'CWE-?'} "
            f"{item.get('severity')}/{item.get('confidence')} "
            f"{item.get('location')} [{item.get('module_dir')}]"
            f"{amp} :: {item.get('details')}"
        )
    ids = ", ".join(item["cve_ids"]) or str(item["advisory_id"])
    if item["cve_ids"] and item["advisory_id"] not in item["cve_ids"]:
        ids = f"{ids} ({item['advisory_id']})"
    where = f"{item['package_module']}@{item['version']}"
    fix = f" -> {item['fixed_version']}" if item["fixed_version"] else " -> no fix yet"
    summary = f" :: {item['summary']}" if item.get("summary") else ""
    return f"{ids} in {where}{fix} [{item['module_dir']}]{summary}"


def main() -> int:
    parser = argparse.ArgumentParser(prog="security_local_ci.py")
    sub = parser.add_subparsers(dest="command", required=True)

    p_norm = sub.add_parser("normalize", help="govulncheck JSON -> CycloneDX + findings")
    p_norm.add_argument("--govulncheck", required=True)
    p_norm.add_argument("--label", required=True)
    p_norm.add_argument("--module-dir", required=True)
    p_norm.add_argument("--out-cdx", required=True)
    p_norm.add_argument("--out-findings", required=True)
    p_norm.set_defaults(func=normalize)

    p_sast = sub.add_parser("sast-normalize", help="gosec JSON -> SAST findings receipt")
    p_sast.add_argument("--gosec", required=True)
    p_sast.add_argument("--label", required=True)
    p_sast.add_argument("--module-dir", required=True)
    p_sast.add_argument("--out-findings", required=True)
    p_sast.set_defaults(func=sast_normalize)

    p_ids = sub.add_parser("cve-ids", help="print CVE IDs found in CycloneDX documents")
    p_ids.add_argument("inputs", nargs="+")
    p_ids.set_defaults(func=cve_ids)

    p_gate = sub.add_parser("gate", help="apply policy, emit verdict and fix plan")
    p_gate.add_argument("--correlation", required=True)
    p_gate.add_argument("--findings", nargs="*", default=[])
    p_gate.add_argument("--sbom", nargs="*", default=[])
    p_gate.add_argument("--sast", nargs="*", default=[])
    p_gate.add_argument("--kev-catalog", help="KEV snapshot, for CWE-class correlation")
    p_gate.add_argument("--out-verdict", required=True)
    p_gate.add_argument("--out-summary", required=True)
    p_gate.add_argument("--out-fix-plan", required=True)
    p_gate.set_defaults(func=gate)

    args = parser.parse_args()
    return args.func(args)


if __name__ == "__main__":
    raise SystemExit(main())
