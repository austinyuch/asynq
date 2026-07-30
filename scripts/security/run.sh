#!/usr/bin/env bash
# Local shift-left security CI for the asynq fork: SBOM + CVE + KEV.
#
# Pipeline (all three modules: root, x, tools):
#   1. SBOM      trivy  -> CycloneDX per module, with vulnerabilities inline
#   2. CVE       govulncheck -> Go-native findings with call-graph reachability
#   3. normalize GO-xxxx advisory IDs resolved to CVE aliases (closes the
#                correlation engine's documented alias recall boundary)
#   4. catalogs  CISA KEV feed + scoped CVE List V5 records for discovered CVEs
#   5. correlate kev-sbom-correlation skill's deterministic engine
#   6. gate      block/warn policy, verdict, and a reviewable fix plan
#
# Catalog acquisition happens here, never inside the deterministic correlation
# step. A catalog that cannot be refreshed is treated as unavailable evidence
# and fails the run; it is never downgraded to an empty catalog.
#
# Usage:
#   scripts/security/run.sh [--refresh] [--offline] [--quiet]
#
#   --refresh   force re-download of the CISA KEV catalog
#   --offline   never touch the network; require cached catalogs to exist
#   --quiet     suppress per-step progress (the summary is still printed)
#
# Exit codes: 0 pass, 1 policy block, 2 tooling/evidence failure.

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
OUT_DIR="${ASYNQ_SECURITY_DIR:-$REPO_ROOT/.security}"
SCRIPT_DIR="$REPO_ROOT/scripts/security"
KEV_URL="https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json"
CVE_API="https://cveawg.mitre.org/api/cve"
KEV_MAX_AGE_DAYS="${ASYNQ_KEV_MAX_AGE_DAYS:-7}"

REFRESH=0
OFFLINE=0
QUIET=0
for arg in "$@"; do
  case "$arg" in
    --refresh) REFRESH=1 ;;
    --offline) OFFLINE=1 ;;
    --quiet)   QUIET=1 ;;
    -h|--help) sed -n '2,26p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "run.sh: unknown argument: $arg" >&2; exit 2 ;;
  esac
done

log()  { [ "$QUIET" -eq 1 ] || printf '  %s\n' "$*"; }
fail() { printf 'security-ci: %s\n' "$*" >&2; exit 2; }

# GUI/IDE git clients and fresh shells may not have these on PATH.
export PATH="$PATH:$HOME/.local/bin:$(go env GOPATH 2>/dev/null || echo "$HOME/go")/bin"

need() { command -v "$1" >/dev/null 2>&1 || fail "$1 not found. Run: make security-tools"; }
need trivy
need govulncheck
need gosec
need python3
need jq

# ---------------------------------------------------------------------------
# Locate the kev-sbom-correlation skill bundle (builder + correlator).
# ---------------------------------------------------------------------------
find_skill_dir() {
  local candidates=(
    "${KEV_SBOM_SKILL_DIR:-}"
    "$REPO_ROOT/.agents/skills/kev-sbom-correlation"
    "$HOME/.claude/skills/kev-sbom-correlation"
    "$HOME/.kiro/skills/kev-sbom-correlation"
    "$HOME/.codex/skills/kev-sbom-correlation"
  )
  local dir
  for dir in "${candidates[@]}"; do
    [ -n "$dir" ] || continue
    if [ -f "$dir/scripts/security_cve_kev_correlate.py" ] &&
       [ -f "$dir/scripts/security_build_cve_catalog.py" ]; then
      printf '%s\n' "$dir"
      return 0
    fi
  done
  return 1
}

SKILL_DIR="$(find_skill_dir)" || fail "kev-sbom-correlation skill bundle not found.
Set KEV_SBOM_SKILL_DIR to the installed skill directory, or install the bundle
into .agents/skills/kev-sbom-correlation. Do not copy individual files."
CORRELATE="$SKILL_DIR/scripts/security_cve_kev_correlate.py"
BUILD_CATALOG="$SKILL_DIR/scripts/security_build_cve_catalog.py"

CI="$SCRIPT_DIR/security_local_ci.py"
[ -f "$CI" ] || fail "missing $CI"

SBOM_DIR="$OUT_DIR/sbom"
SCAN_DIR="$OUT_DIR/scan"
SAST_DIR="$OUT_DIR/sast"
EV_DIR="$OUT_DIR/evidence"
CVE_RECORDS="$EV_DIR/cve-records"
REPORT_DIR="$OUT_DIR/report"
mkdir -p "$SBOM_DIR" "$SCAN_DIR" "$SAST_DIR" "$EV_DIR" "$CVE_RECORDS" "$REPORT_DIR"

KEV_FILE="$EV_DIR/known_exploited_vulnerabilities.json"
CVE_CATALOG="$EV_DIR/cve-catalog.json"
PROVENANCE="$EV_DIR/provenance.json"

sha256() { sha256sum "$1" | cut -d' ' -f1; }
now_utc() { date -u +%Y-%m-%dT%H:%M:%SZ; }
today_utc() { date -u +%Y-%m-%d; }

# Modules: dir|label
MODULES=(".|asynq-root" "x|asynq-x" "tools|asynq-tools")

printf '\n  local security CI  (SBOM + CVE + KEV)\n'
printf '  skill bundle: %s\n\n' "$SKILL_DIR"

# ---------------------------------------------------------------------------
# 1 + 2 + 3. Per-module SBOM, CVE scan, evidence normalization.
# ---------------------------------------------------------------------------
SBOM_FILES=()
FINDING_FILES=()
SAST_FILES=()
EVIDENCE_ARGS=()

for spec in "${MODULES[@]}"; do
  dir="${spec%%|*}"
  label="${spec##*|}"
  mod_path="$REPO_ROOT/$dir"
  [ -f "$mod_path/go.mod" ] || fail "no go.mod in $dir"

  sbom="$SBOM_DIR/$label.cdx.json"
  gvc="$SCAN_DIR/$label.govulncheck.json"
  gvc_cdx="$EV_DIR/$label.govulncheck.cdx.json"
  findings="$REPORT_DIR/$label.findings.json"

  # trivy scans recursively, so exclude the sibling module directories from the
  # root scan to keep one SBOM per Go module.
  skip=()
  if [ "$dir" = "." ]; then
    skip=(--skip-dirs ./x --skip-dirs ./tools)
  fi

  log "[$label] SBOM (trivy)"
  ( cd "$mod_path" && trivy fs --quiet --scanners vuln --format cyclonedx \
      --skip-version-check "${skip[@]}" --output "$sbom" . ) \
    || fail "trivy failed for $label"

  # trivy omits the key entirely when it finds nothing. trivy exited 0, so this
  # is a completed scan with zero findings, not missing evidence -- make that
  # explicit so the correlation engine does not read it as unavailable.
  python3 - "$sbom" <<'PY' || fail "could not normalize $label SBOM"
import json, sys
from pathlib import Path
path = Path(sys.argv[1])
data = json.loads(path.read_text(encoding="utf-8"))
if "vulnerabilities" not in data:
    data["vulnerabilities"] = []
    path.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")
PY

  log "[$label] CVE scan (govulncheck)"
  # govulncheck exits 0 in JSON mode even when it reports findings; the gate
  # reads the findings, so only a genuine tool error matters here.
  ( cd "$mod_path" && govulncheck -format json ./... > "$gvc" ) \
    || fail "govulncheck failed for $label"

  log "[$label] normalize evidence (resolve GO -> CVE aliases)"
  python3 "$CI" normalize \
    --govulncheck "$gvc" \
    --label "$label" \
    --module-dir "$dir" \
    --out-cdx "$gvc_cdx" \
    --out-findings "$findings" || fail "normalize failed for $label"

  # SAST over first-party code. Generated files are excluded: asynq.pb.go is
  # regenerated by `make proto` and must never be hand-edited, so a finding
  # there is unactionable noise.
  sast_raw="$SAST_DIR/$label.gosec.json"
  sast_findings="$REPORT_DIR/$label.sast.json"
  log "[$label] SAST (gosec)"
  # gosec exits non-zero when it reports findings; the gate decides, so only a
  # genuine tool failure (no parseable JSON) is fatal here. Deliberately NOT
  # -quiet: that suppresses output entirely on a clean scan, which would throw
  # away the files/lines/nosec stats that make "0 findings" auditable.
  ( cd "$mod_path" && gosec -fmt=json -exclude-generated ./... > "$sast_raw" 2>/dev/null ) || true
  jq -e 'has("Issues") or has("Stats")' "$sast_raw" >/dev/null 2>&1 \
    || fail "gosec produced unparseable output for $label (see $sast_raw)"

  python3 "$CI" sast-normalize \
    --gosec "$sast_raw" \
    --label "$label" \
    --module-dir "$dir" \
    --out-findings "$sast_findings" || fail "sast-normalize failed for $label"

  SBOM_FILES+=("$sbom")
  FINDING_FILES+=("$findings")
  SAST_FILES+=("$sast_findings")
  EVIDENCE_ARGS+=("${label}-sbom=$sbom" "${label}-govulncheck=$gvc_cdx")
done

# ---------------------------------------------------------------------------
# 4a. CISA KEV catalog. Unavailable refresh != empty catalog.
# ---------------------------------------------------------------------------
kev_is_fresh() {
  [ -f "$KEV_FILE" ] || return 1
  local age_days
  age_days=$(( ( $(date -u +%s) - $(stat -c %Y "$KEV_FILE") ) / 86400 ))
  [ "$age_days" -le "$KEV_MAX_AGE_DAYS" ]
}

KEV_RETRIEVED="cached"
if [ "$OFFLINE" -eq 1 ]; then
  [ -f "$KEV_FILE" ] || fail "--offline requested but no cached KEV catalog at $KEV_FILE.
Run once with network access: scripts/security/run.sh --refresh"
  log "[kev] using cached catalog (offline)"
elif [ "$REFRESH" -eq 1 ] || ! kev_is_fresh; then
  log "[kev] downloading CISA Known Exploited Vulnerabilities catalog"
  tmp_kev="$(mktemp "$EV_DIR/.kev.XXXXXX")"
  if curl -fsS --max-time 60 "$KEV_URL" -o "$tmp_kev" && jq -e '.count' "$tmp_kev" >/dev/null 2>&1; then
    mv "$tmp_kev" "$KEV_FILE"
    KEV_RETRIEVED="$(now_utc)"
  else
    rm -f "$tmp_kev"
    if [ -f "$KEV_FILE" ]; then
      log "[kev] refresh failed; falling back to cached catalog"
    else
      fail "KEV catalog refresh failed and no cached catalog exists.
The correlation engine treats an unavailable catalog as unavailable evidence,
not as an empty one, so this run cannot produce a KEV verdict."
    fi
  fi
else
  log "[kev] cached catalog is fresh (<= ${KEV_MAX_AGE_DAYS}d)"
fi

KEV_VERSION="$(jq -r '.catalogVersion' "$KEV_FILE")"
KEV_RELEASED="$(jq -r '.dateReleased' "$KEV_FILE")"
KEV_COUNT="$(jq -r '.count' "$KEV_FILE")"
KEV_SHA="$(sha256 "$KEV_FILE")"
log "[kev] version=$KEV_VERSION count=$KEV_COUNT"

# ---------------------------------------------------------------------------
# 4b. Scoped CVE catalog: fetch only the CVE IDs our scanners discovered.
# ---------------------------------------------------------------------------
# Collect via a temp file, not a pipe: mapfile would mask a non-zero exit from
# the extractor and an empty list would then read as "no CVEs found", which is
# exactly the false-clean result this pipeline must never produce.
cve_id_list="$(mktemp)"
trap 'rm -f "$cve_id_list"' EXIT
python3 "$CI" cve-ids "${SBOM_FILES[@]}" "$EV_DIR"/*.govulncheck.cdx.json \
  > "$cve_id_list" || fail "CVE ID extraction failed; refusing to report a clean run"
mapfile -t DISCOVERED_CVES < "$cve_id_list"

CVE_CATALOG_MODE="scoped"
if [ "${#DISCOVERED_CVES[@]}" -eq 0 ]; then
  log "[cve] no CVE candidates in evidence; correlation is vacuous"
else
  log "[cve] ${#DISCOVERED_CVES[@]} CVE candidate(s) discovered"
  fetched=0
  for cve in "${DISCOVERED_CVES[@]}"; do
    record="$CVE_RECORDS/$cve.json"
    [ -f "$record" ] && continue
    if [ "$OFFLINE" -eq 1 ]; then
      fail "--offline requested but CVE record $cve is not cached.
Run once with network access: scripts/security/run.sh --refresh"
    fi
    tmp_rec="$(mktemp "$CVE_RECORDS/.$cve.XXXXXX")"
    if curl -fsS --max-time 30 "$CVE_API/$cve" -o "$tmp_rec" &&
       jq -e --arg id "$cve" '.cveMetadata.cveId == $id' "$tmp_rec" >/dev/null 2>&1; then
      mv "$tmp_rec" "$record"
      fetched=$((fetched + 1))
    else
      rm -f "$tmp_rec"
      fail "could not retrieve CVE List V5 record for $cve.
A missing record is unavailable evidence, not an absent CVE."
    fi
  done
  log "[cve] fetched $fetched new record(s), $(ls -1 "$CVE_RECORDS" | wc -l) cached total"

  log "[cve] building cve-catalog-snapshot/v1 (skill builder)"
  python3 "$BUILD_CATALOG" "$CVE_RECORDS" \
    --catalog-version "scoped-cve-services-$(today_utc)" \
    --date-released "$(today_utc)" \
    --output "$CVE_CATALOG" || fail "CVE catalog build failed"
fi

# ---------------------------------------------------------------------------
# 5. Deterministic CVE/KEV correlation (skill engine).
# ---------------------------------------------------------------------------
CORRELATION="$REPORT_DIR/correlation.json"
CVE_SHA=""
if [ "${#DISCOVERED_CVES[@]}" -eq 0 ]; then
  # Nothing to correlate. Record that explicitly instead of fabricating a
  # clean correlation receipt from catalogs that were never consulted.
  python3 - "$CORRELATION" "$KEV_VERSION" "$KEV_RELEASED" "$KEV_COUNT" "$KEV_SHA" <<'PY'
import json, sys
from pathlib import Path
out, version, released, count, sha = sys.argv[1:6]
Path(out).write_text(json.dumps({
    "schema": "asynq-correlation-skipped/v1",
    "reason": "no CVE candidates present in the supplied evidence",
    "routes": [],
    "kev_catalog": {
        "catalog_version": version,
        "date_released": released,
        "count": int(count),
        "sha256": sha,
        "completeness": "not-asserted-by-source-schema",
    },
    "disclosure": "Zero CVE candidates does not prove there is no KEV exposure.",
}, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
  log "[correlate] skipped (no CVE candidates)"
else
  CVE_SHA="$(sha256 "$CVE_CATALOG")"
  log "[correlate] kev-sbom-correlation engine"
  python3 "$CORRELATE" "${EVIDENCE_ARGS[@]}" \
    --cve-catalog "$CVE_CATALOG" \
    --kev-catalog "$KEV_FILE" \
    --expect-cve-sha256 "$CVE_SHA" \
    --expect-kev-sha256 "$KEV_SHA" \
    --format json > "$CORRELATION" || fail "correlation engine failed (see $CORRELATION)"
fi

# ---------------------------------------------------------------------------
# Operator-owned catalog provenance.
# ---------------------------------------------------------------------------
python3 - "$PROVENANCE" <<PY
import json
from pathlib import Path
Path("$PROVENANCE").write_text(json.dumps({
    "schema": "asynq-catalog-provenance/v1",
    "generated_at": "$(now_utc)",
    "kev_catalog": {
        "source_uri": "$KEV_URL",
        "retrieved_at": "$KEV_RETRIEVED",
        "sha256": "$KEV_SHA",
        "catalog_version": "$KEV_VERSION",
        "date_released": "$KEV_RELEASED",
        "count": $KEV_COUNT,
        "completeness": "not-asserted-by-source-schema",
    },
    "cve_catalog": {
        "source_uri": "$CVE_API/<CVE-ID>",
        "mode": "$CVE_CATALOG_MODE",
        "sha256": "$CVE_SHA",
        "note": "Scoped to the CVE IDs discovered by the local scanners. "
                "'not_present_in_supplied_snapshot' is therefore uninformative "
                "and never a global absence claim.",
    },
}, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

# ---------------------------------------------------------------------------
# 6. Policy gate + fix plan.
# ---------------------------------------------------------------------------
set +e
python3 "$CI" gate \
  --correlation "$CORRELATION" \
  --findings "${FINDING_FILES[@]}" \
  --sbom "${SBOM_FILES[@]}" \
  --sast "${SAST_FILES[@]}" \
  --kev-catalog "$KEV_FILE" \
  --out-verdict "$REPORT_DIR/verdict.json" \
  --out-summary "$REPORT_DIR/summary.txt" \
  --out-fix-plan "$OUT_DIR/fix-plan.sh"
gate_status=$?
set -e

printf '  evidence: %s\n\n' "${OUT_DIR#$REPO_ROOT/}"
exit "$gate_status"
