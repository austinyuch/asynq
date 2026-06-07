# Review Generation Guide — austinyuch/asynq

> `docs/review/index.html`(高階管理者 review 文件)的生成/再生筆記。配合 global `project-review-skill`;AGENTS.md 的 FORK GOVERNANCE & DOC MEMO 表有此檔入口。

## 受眾與語言

- 受眾:團隊管理層(內部 ROI/維運視角)+ 下游專案 owner(外部消費視角)
- 語言:zh-TW(團隊工作語言;README 雖為英文,review 受眾以中文溝通)

## Readiness Claim Cap(本專案的具體界線)

- 裁決權威 = `.agents/specs/**/review.md`;**首份**:`SPEC-008-gap-closeout/review.md`(2026-06-07,runtime-backed)。功能卡片的 Readiness State 沿用裁決;未列入裁決範圍 → `not_assessed`
- 裁決與 commit 範圍綁定;重大程式碼變動後應重跑裁決內的權威 gate 再引用
- `SPECS.md` 的 ✅ completed 是 **governance 完成度**(有 PR/tag/e2e 證據),不是 live-demo readiness verdict;review 文件引用時必須標明這個區別
- `NEXT_STEPS.md` / RTM 只能當 backlog/traceability hint,不可反推 readiness
- 可以陳述的「事實級」evidence:消費端 `go get`/`go install` e2e 輸出、CI run pass、全套測試綠、dry-run eval verdict — 引用時附 Source Ref

## Canonical Scenario(與 manual 共用,不另造資料)

Review 文件的所有數據引用 `docs/manual/assets/*.txt`(由 `docs/manual/demo/main.go` 在 governed Valkey allocation 產生)。**不得**在 review 流程另外造一份樣本資料 — 單一 seed 來源避免 evidence 漂移。

## 再生步驟

1. 依 `docs/MANUAL_GENERATION_GUIDE.md` 的命令序列刷新 `docs/manual/assets/`(governed allocation)
2. 更新本 repo 事實基準:`git log --oneline -20`、`git tag`、`gh run list --workflow=build --limit 3`、`.agents/specs/{SPECS,ISSUE_LOG,NEXT_STEPS}.md`
3. 重寫 `docs/review/index.html`:
   - 必含區塊順序:Hero(3 統計)→ Value Proposition(內/外雙視角)→ Amazon Backwards PR → FAQ(管理者/外部/使用者)→ Core Features(卡片 + evidence metadata)→ UX Flow(Mermaid)→ Gap Analysis(✅/⏳)→ Roadmap → Footer
   - 每張 feature card 必附:`Evidence Source` / `Coverage Tier` / `Readiness State` / `Source Ref`(非 live 時加 `Fallback Reason`)
4. Headless 渲染防呆(mermaid 渲染 + anchors + 截圖目視),命令同 manual guide
5. 更新本檔的 Gap 狀態節

## Gap 狀態(隨每次再生更新)

| Gap | 狀態 | Code |
|---|---|---|
| `asynq dash` TUI 視覺 | 文字 + 圖形層皆實擷(引用 manual assets txt/png) | resolved(SPEC-008) |
| `docs/assets/` upstream 素材 | 逐檔 disposition 完成(9 檔零引用 legacy、dash.gif upstream 示意) | documented |
| 正式 review.md 裁決 | **已建立**:`SPEC-008-gap-closeout/review.md` | 裁決機制生效 |
| Redis Cluster mode | 裁決明列 not_assessed | 殘餘(低) |

### Gaps resolved since last check(本次:2026-06-07 再生 #3 / gap closeout)

- ✅ review.md 裁決機制落地(SPEC-008);本文件 readiness 自此有權威來源,claim cap 解除至裁決範圍
- ✅ dash 卡片升級為文字+圖形層 live capture(含 PNG)
- ✅ IL-001 / IL-003 結案反映於 Gap Analysis

### 較早(2026-06-07 再生 #2)

- ✅ dash TUI feature card 從 `static_placeholder / DEMO_NOT_ASSESSED` 升級為 live TUI text capture(引用 `docs/manual/assets/cli-dash-*.txt`)
- ✅ review 引用的 manual assets 此前從未 commit(死連結)— 已隨 manual 再生 #2 補齊
- ✅ governance delta 反映 PR #9(govulncheck pre-push hook、`gh pr create --repo` 鐵則 / IL-R04)
- 首次生成基線:review 文件存在且全部 evidence 來自 fork 環境真實輸出;服務啟動走 registry 治理(Valkey 16381 + exporter 9876)
- ⏳ open:上表後兩項 + dash 圖形層
