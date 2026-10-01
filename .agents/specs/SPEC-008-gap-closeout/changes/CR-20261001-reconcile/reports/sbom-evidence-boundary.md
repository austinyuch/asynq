# SBOM application/evidence boundary

The root application SBOM also inventoried ignored `.security` analyzer dependency inputs. The prior frozen5eb scan21-component root SBOM therefore described mixed inputs, not application ownership of tooling dependencies. Historical scanner bytes remain unchanged; the tooling SBOM remains a separate subject.

The candidate adds `.security` exclusion to each root/x/tools Trivy application scan while retaining existing sibling-module exclusions. It does not suppress tracked application dependency inputs. Candidate runner SHA `14ce3d6088c595b22675395082a6c27a12fcb1f31529c112bcd95567a977682e` is bound by `.security/sbom-boundary/candidate-review-ready.json`.

Thirteen actual native Trivy scans execute source-bound production command fragments against controlled copied-module fixtures across four seeded benign root paths. Visible application requests input remains present; evidence-only packages disappear; root/x/tools sibling separation is retained. Removing the exclusion is a viable mutant: scanner exits0 and the ordinary assertion fails. `.security/sbom-boundary/receipt.json` SHA `ac954b46a123ce9af45fbbbf072b6ed2c5bebe85e6425fce410daeeb1503d463` and oracle receipt SHA `1d32d31e262b97299fc47bcf8d66d1e573e03303ccbb6c5eb996c68fa9bad151` retain94 witness materials and actual oracles.

The maintained fixture-scanner pipeline contract reproduces three original assertion failures and passes on the candidate. It is separate from native scanner command-fragment evidence; neither constitutes a fresh full-repository security pipeline, global line coverage or dependency/KEV absence proof. Full maintained test count and non-author closeout are not claimed here before their terminal receipts.

Candidate evidence does not establish exact full-pipeline delivery. Successor delivery state is authoritative only when `.security/final-sbom-boundary/gate-ledger.json` exists with DEV_DELIVERED_MAIN_HELD state and verified head/tree/material/remote/hosted bindings; this report does not freeze that future state as pending forever. Main, release, cross-family approval and global line95 remain held; SPEC-008 verdict is unchanged.
