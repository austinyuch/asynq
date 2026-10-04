# Shell and browser measurement qualification — 2026-10-03

Authority: delegated technical measurement choices under `scripts/coverage/POLICY.md`.
Production source is unchanged from dev c735657c998e5aaa700e88529eee21af083437ae.
This follow-up does not amend the formal SPEC-008 readiness verdict. Project
executable LINE>=95%, main promotion, release and cleanup remain OPEN.

## Adopted attribution and bounded Shell evidence

Source-operation attribution uses original AST token positions, not complete
node spans. Function registration belongs to its name; loop/if/case decisions
belong to headers; dynamic children and bodies remain candidates. Static case
labels, structural footers and plain continuation literals are nonexecutable.
Source/eval command sites remain candidates; dynamic execution provenance is a
separate unadmitted context. Missing observations never remove a candidate.

The native mvdan.cc/sh/v3 v3.14.1 classifier inventories 292 run.sh, 30 pre-push
and 18 upstream-sync candidate lines. These 340 lines are experimental, not a
qualified whole-Shell denominator. Nine native contracts, 100 offset properties
and two viable omission mutants pass independent assertions. A 10-second native
fuzz run exits 0:33311 raw engine executions;55543 accepted and21168 invalid-parse
callbacks include replay/minimization and cannot form an execution acceptance ratio.
Contracts SHA256:621fb9fca6aa31891b57245d0ab739b691534669e90b77fb9b1fba2ce187808c.
Fuzz SHA256:7307392507001d576a65538633d3014719de7f90eba555e9889eb5589434b131.

Actual kcov v43 omits executable multiline command/parameter expansion sites.
Bash PS4 also attributes inner expansion to the outer line. A DEBUG trap creates
an actual false hit under the earlier mapping guard. Neither producer is admitted
as a general original-line observer. The successor uses explicit AST/context
allowlists and rejects trap/eval/source/alias and source drift, preserving
UNRESOLVED candidates. Eight fixtures,16 owned sessions and a caught guard-removal
mutant are bounded adapter evidence only. Receipt SHA256:
c5a8c436e1c668f6df31c70200577ea8acb8172987a91feef02a303712a5e599.

## Current owned browser class

Independent HTMLParser inventory finds one identical inline module in each of
manual/en, manual/zh-tw and review index.html; no handlers, JavaScript URLs or
owned external JS. Each module has two straight-line executable original lines,
8 and9. Real pinned Chromium151 with CDP V8 precise coverage records all six
owned original lines with retained exact-source UTF16 maps. Real CDN imports and
one rendered Mermaid SVG per document succeed. Normal/traced result observables
match; this is not complete DOM byte equivalence, pixel acceptance or generic JS
qualification. Separate native contracts retain nested zero-count overrides and
an unrequested module as unmeasured, preventing positive-parent span filling.

Six browser exits are0; six adopted helper exits are1 and are retained. Owned
sessions clean up without output truncation. The floating mermaid@11 CDN has
point-in-time source hashes, not a future version pin. The 181 native materials
and current three HTML sources pass independent readback. Browser receipt SHA256:
7cd45b1768597a877eb6766cdf1eebdfd7175499a7800799db7d762187150a6d.
Independent bounded review SHA256:
8fb9c0828e51885b97b78ae8efeeed7d899aafcef95c7728b1f8c1ede9475520.

## Other observer and runtime prerequisites

Go line grouping disables an instruction probe only after all mapped original
sites are covered. Selected base/log/timeutil native hits match the predecessor;
probe events fall from4334/976/265 to303/90/19. Independent 1002 multi-site
properties and two viable mutants protect this optimization. It does not supply
full Go profiles or Windows runtime. Comparison SHA256:
02d953af42355bdf0056d008a86db5688556c0ddb51ce6840c4144aacedd045b.

The published registry client has a fixture-proven lost-update defect; current
owner source already fixes it. Exact owner SHA256
721be9947e38003ad57472c7d4ed339f1ac0f0f5dd9865710b777da1529ffe0e
passes concurrent preservation and fail-closed lock checks and is selected for
future owned asynq runtime claims. No owner patch/global publication or service
start is inferred. Redis7.0.15 and Lua5.1 were acquired task-locally with metadata
hash verification. Native Go AST extracts all50 embedded redis.NewScript literals
(25 rdb,24 inspect,1 semaphore), retaining decoded bytes and original embedding
maps; all50 compile. Compiler lineinfo is not a qualified Lua denominator.
Lua inventory SHA256:48d915d30a317c8afc2022714155125c56ec9122bdece2eaf971fad2e7760cde.

Next: register an owned Redis fixture with the verified owner client; qualify
Lua original-line/runtime mapping and full exact-source Go runtime profiles.
Ignored `.security/` receipts remain local experimental custody; this report is
not a portable full evidence archive or an exact delivery gate receipt.
