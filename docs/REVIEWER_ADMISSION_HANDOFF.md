# Managed reviewer promotion capability handoff

Owner: aclab-code-review-private native xreview lifecycle. Consumer: asynq.
Proposed owner CR: ACP staged-pointer promotion coverage support.

## Problem and scope

The user authorized the fixed published main57 to dev25, 154-path payload and
configured Grok/ACP destination on 2026-10-03, then delegated handling of the
reviewer admission evidence. Native v0.90.0 promotion preflight exited3 with
`scope-evidence-unavailable` / `promotion-route-without-coverage-proof` and
`evidence_started=false`. No review or promotion approval occurred.

Current owner route registry lists Grok ACP as settled. The newest owner live
contract receipt, 20260928T210322Z, records a reviewed ACP PASS at
v0.88.4-156-g4a502521. That general live contract does not prove strict promotion
pointer coverage. Native `grok_connector_diagnostics_test.go` explicitly expects
`acp-pointer-route-unsupported`; `sealed_scope.go` refuses promotion manifests.
Grok exec remains unroutable after an exact-dev live review missed the planted
finding. Do not bypass quarantine or treat discovery as review completion.

## Owner implementation and acceptance

1. Implement an admitted ACP carrier for staged pointers under the native
   least-privilege contract, with exact candidate/base/tree/plan identities.
2. Have the native coverage validator verify every scoped path, completed units
   and cross-topic seams. Reviewer-supplied digests alone cannot establish reads.
3. Add meaningful contract regressions for incomplete reads, wrong source or
   candidate identity, missing checkpoints, early exit and replay. Preserve
   failed evidence and refuse promotion on any incomplete result.
4. Run the existing Grok-only settled-route regression and actual strict
   promotion live proof on the supported route. A small smoke or same-family
   verdict cannot substitute for the authorized complete scope.
5. Rebuild and publish through the owner's governed managed-skill workflow;
   verify installed identity and rerun consumer exact-candidate promotion review.
   No local consumer edit of managed binaries, route registry or global policy.

Read-only owner entrypoint, cwd
`/home/ac/projects/aclab/aclab-code-review-private`:
`python3 scripts/xreview_live_contract_receipt.py show`.
Supported Grok-only regression, cwd
`/home/ac/projects/aclab/aclab-code-review-private/go-review-service`:
`go test -tags live -count=1 -p 1 -run '^TestLiveSmokeACPGrok$' -v -timeout 15m ./internal/xreview/run/`.
The regression was not executed in this handoff. The all-route receipt runner
would dispatch additional providers and is outside the fixed Grok-only scope.

## Evidence and consumer holds

Consumer original payload manifest:
`.security/remaining-objective-audit/review-approval-packet-20261002/manifest.json`
SHA256 `445db3df9511b468a80d906c7cb6ba3d5bfe5feeda216d6fa5e337b7a6d5ea61`;
payload SHA256 `9cd4beb6c9fe157be7551bb1fad4f21ac535dfaea5245ed1feb51c05e519ccfe`.
Actual preflight record:
`.security/xreview-approved-20261003/records/repo-1a23e58a2758b5a9002964b3.jsonl`.
The managed native binary SHA256 was
`2472f75d07a6e147d4a0d35b3de92229d8dee1a3163aaeb15e06c2604f5a0d25`.
Independent current route audit SHA256:
`763baeefb3667e5f9bd1702cedfa620981aba28536b6d5003418258bb7aa9a50`.

The fixed original approval is not evidence of a review of later changed
candidates. Preserve the original scope and failed receipt; bind a future review
to its actual candidate. Main PR22, release tagging/publication and the overall
goal remain held until their own gates pass. This handoff is a prepared owner
change request, not an externally sent message, owner implementation or PASS.
