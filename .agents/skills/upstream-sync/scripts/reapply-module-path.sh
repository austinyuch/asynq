#!/usr/bin/env bash
# Idempotent:merge upstream 後重新套用 module-path rename(hibiken → austinyuch)。
# 只改「引用程式碼」的 path,不動指向 upstream 的文件連結(wiki / issues / license)。
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# 1) Go import paths(quoted form,不會誤傷註解中的 upstream issue 連結)
#    grep 無匹配時回傳 1(= 已是 idempotent 狀態),不可讓 pipefail 視為錯誤
{ grep -rl '"github.com/hibiken/asynq' --include='*.go' . 2>/dev/null || true; } \
  | xargs -r sed -i 's|"github.com/hibiken/asynq|"github.com/austinyuch/asynq|g'

# 2) go.mod module / require 行(replace 區塊由 hotspot 衝突解法保留,這裡只兜底)
sed -i 's|^module github.com/hibiken/asynq|module github.com/austinyuch/asynq|' go.mod x/go.mod tools/go.mod
sed -i 's|^	github.com/hibiken/asynq |	github.com/austinyuch/asynq |' x/go.mod tools/go.mod
sed -i 's|^	github.com/hibiken/asynq/x |	github.com/austinyuch/asynq/x |' tools/go.mod

# 3) proto 的 go_package(generated .pb.go 的 raw descriptor 不動——runtime 無影響,
#    重新 protoc 時會從 .proto 帶出正確值)
sed -i 's|go_package = "github.com/hibiken/asynq|go_package = "github.com/austinyuch/asynq|' internal/proto/*.proto

# 4) README 的可執行引用(go get / go install / import 範例 / build badge)
sed -i -e 's|^go get -u github.com/hibiken/asynq$|go get -u github.com/austinyuch/asynq|' \
       -e 's|"github.com/hibiken/asynq"|"github.com/austinyuch/asynq"|g' \
       -e 's|go install github.com/hibiken/asynq/tools/asynq@latest|go install github.com/austinyuch/asynq/tools/asynq@latest|' \
       -e 's|github.com/hibiken/asynq/workflows/build/badge.svg|github.com/austinyuch/asynq/workflows/build/badge.svg|' \
       README.md tools/asynq/README.md
sed -i 's|go install github.com/hibiken/asynq/tools/asynq@latest|go install github.com/austinyuch/asynq/tools/asynq@latest|' tools/AGENTS.md 2>/dev/null || true

# 5) Makefile 的 protoc 生成參數(漏掉的話下次 make proto 會生成 hibiken path)
sed -i 's|--go_opt=module=github.com/hibiken/asynq|--go_opt=module=github.com/austinyuch/asynq|' Makefile

remaining=$({ grep -rn '"github.com/hibiken/asynq' --include='*.go' . 2>/dev/null || true; } | wc -l)
proto_remaining=$({ grep -rn 'go_package = "github.com/hibiken' --include='*.proto' . 2>/dev/null || true; } | wc -l)
echo "remaining hibiken imports in .go files: ${remaining}"
echo "remaining hibiken go_package in .proto files: ${proto_remaining}"
[ "$((remaining + proto_remaining))" -eq 0 ] || { echo "ERROR: rename incomplete" >&2; exit 1; }

# 資訊性 audit(不擋流程):其他 tracked 檔案中的 hibiken 引用。
# README/CONTRIBUTING/FORK.md 指向 upstream wiki/issues/license 的連結與
# generated asynq.pb.go 的 raw descriptor 屬刻意保留;此清單供人工掃一眼
# upstream 是否新增了該改而沒改到的引用(新 workflow、新文件、新 go.mod 等)。
echo "--- informational audit (intentional upstream links expected) ---"
git grep -n 'github.com/hibiken/asynq' -- ':!*.go' ':!internal/proto/*.pb.go' ':!.claude' ':!CHANGELOG.md' || echo "(none)"
