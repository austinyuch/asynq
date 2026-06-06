---
name: upstream-sync
description: 同步 upstream hibiken/asynq 到這個 team fork(module path 已改名為 github.com/austinyuch/asynq)。當使用者提到「sync upstream」「同步上游」「upstream 有沒有更新」「merge hibiken」「更新 fork」「拉 upstream 的新 commit / release / tag」,或詢問 fork 與 upstream 的差異與衝突處理時使用。即使使用者只說「看一下上游有什麼新東西」也應觸發。
---

# Upstream Sync(hibiken/asynq → austinyuch/asynq)

這個 repo 是 team-maintained fork,branch 模型:
- module path 已改名:`github.com/hibiken/asynq` → `github.com/austinyuch/asynq`(root、`x/`、`tools/` 三個 module)
- **`main`**(default):下游專案直接消費、團隊 PR 的目標分支。**只能 PR merge,永遠不可 rebase 或 force-push**
- **`master`**:upstream 的**純鏡像**,只能 `--ff-only` 從 `upstream/master` 更新,永不放團隊 commit——這條線是與 upstream 的對照基準
- 與 upstream 的差異清單記錄在 `FORK.md`,每次 sync 後必須更新

## Pipeline(前一步未通過不可進入下一步)

### 1. 前置檢查

```bash
git status --porcelain        # 必須是 clean worktree
git checkout main && git pull --ff-only origin main
git remote get-url upstream || git remote add upstream https://github.com/hibiken/asynq.git
```

`--ff-only` 是必要的:預設 pull 在 divergence 時會報錯或(若使用者設了 `pull.rebase=true`)默默 rebase——後者直接違反本 skill 的「永不 rebase main」。

### 2. 偵測 upstream 更新,並更新鏡像分支

```bash
git fetch upstream --tags
git rev-list --count --first-parent master..upstream/master
```

(`--first-parent` 讓計數符合人類直覺;不加的話 merge commits 會展開,例如 10 個 PR merge 報 31。寫進 PR body / FORK.md 的數字以 first-parent 為準。)

- 為 `0` → 回報「已與 upstream 同步」,**流程到此結束**
- 大於 0 → 先更新鏡像,再列出這次 sync 的內容:

```bash
git checkout master && git merge --ff-only upstream/master && git push origin master
git log --oneline main..master        # 這次要合入 main 的 upstream commits
```

`--ff-only` 失敗代表 master 被污染(混入了非 upstream commit)——停下來回報,不可硬 merge。

### 3. 建立 sync branch 並 merge

```bash
SYNC_BRANCH=sync/upstream-$(date +%Y%m%d)   # 只算一次,後續步驟都用變數(跨午夜不會分岔)
git checkout -b "$SYNC_BRANCH" main
git merge master
```

只用 merge,不用 rebase——保留 fork commit 的 hash,衝突一次解完,歷史可追溯。

### 4. 解衝突(已知 divergence hotspots)

| 檔案 | 解法 |
|---|---|
| `go.mod` / `x/go.mod` / `tools/go.mod` | **保留 fork 的** `module github.com/austinyuch/asynq...` 行、`replace => ../` 區塊、較新的 Go/toolchain 版本;**採用 upstream 的**新增依賴,之後跑 `go mod tidy` 收斂 `go.sum` |
| `README.md` / `tools/asynq/README.md` | **保留 fork 的** import path(austinyuch)、Valkey 說明;**採用 upstream 的**新內容段落 |
| `.github/workflows/*.yml` | **保留 fork 的** `valkey/valkey` image、Go 版本、最小化 trigger;**採用 upstream 的**新 job/step |
| `server_test.go` | **保留 fork 的** goleak ignore 清單(go-redis 新背景 goroutine);**採用 upstream 的**新測試 |
| 任何 `*_test.go` / `*.go` 的 import 區塊 | 直接**採 upstream 側**(hibiken path 也沒關係)——step 5 的 rename script 會兜底改成 austinyuch,這是最機械、最不易出錯的解法 |

不在表內的衝突:優先理解 upstream 意圖,必要時對照 `FORK.md` 判斷哪邊是 intentional divergence。

衝突解完後 `git commit`(完成 merge commit)再進下一步。

### 5. 重新套用 module-path rename

upstream 新增的檔案 import 的是 `github.com/hibiken/asynq`,merge 後必須重套 rename。執行 idempotent script:

```bash
bash .claude/skills/upstream-sync/scripts/reapply-module-path.sh
```

script 結束時會輸出殘留的 hibiken import 數量,必須為 0 才能繼續。若 script 有改動檔案,連同 step 6 `go mod tidy` 產生的變更一起 `git commit -m "Re-apply module path rename after upstream sync"`——不可留 uncommitted 變更進 step 9,否則 push 出去的是不完整的 branch。

### 6. 建置驗證(三個 module 全過才算過)

```bash
go mod tidy && go build ./... && go vet ./...
(cd x     && go mod tidy && go build ./... && go vet ./...)
(cd tools && go mod tidy && go build ./... && go vet ./...)
```

### 7. 完整測試(Valkey-backed)

需要真實 Valkey instance——**必須走 `local-infra-registry-governance` skill 的流程**(registry query → env request/reuse → 跑測試 → release),不可直接 `docker run` 繞過 registry。

instance 就緒後(假設 port 為 `$PORT`):

```bash
go test -count=1 ./... -redis_addr=localhost:$PORT            # root(約 3-4 分鐘)
go test -count=1 ./internal/rdb/ -redis_addr=localhost:$PORT
go test -count=1 $(go list ./internal/... | grep -v /rdb)      # 其餘 internal 不收 redis flag
(cd x && go test -count=1 ./... -redis_addr=localhost:$PORT)
(cd tools && go test -count=1 ./...)
```

注意:`-redis_addr` 只有 root、`internal/rdb`、`x/rate` 有註冊,對其他 package 傳入會得到 `flag provided but not defined` 的假 FAIL。

### 8. 更新 FORK.md

在 sync log 加一列:日期、合入的 upstream commit(short hash)、解掉的衝突、測試結果,然後 `git commit`。若這次 sync 改變了 divergence 清單(fork-only 修改被 upstream 收編、或新增了 fork-only 改動),同步更新差異表。

### 9. PR → main

確認 `git status --porcelain` 乾淨(所有步驟的變更都已 commit)後:

```bash
git push -u origin "$(git branch --show-current)"
gh pr create --repo austinyuch/asynq --base main --title "Sync upstream $(date +%Y%m%d)" --body "<upstream commits 摘要 + 測試結果>"
```

(`gh` 在 fork 上預設指向 upstream repo,`--repo` 不可省。)

CI(Valkey-backed build.yml)綠了才 merge。merge 後視需要打 tag:

```bash
git tag v<upstream-version>-team.<N>        # root module
git tag x/v<x-version>-team.<N>             # x module(multi-module repo 的 tag 要帶目錄前綴)
git tag tools/v<tools-version>-team.<N>
git push origin --tags
```

## 絕對不做

- force-push 或 rebase `main`(下游 go.sum 記著 commit hash,改寫歷史會弄壞所有消費者)
- 把團隊 commit 放進 `master`(它是 upstream 鏡像;污染後 `--ff-only` 會永久失敗)
- 在測試未全綠時 merge sync PR
- 繞過 local-infra registry 直接起容器
