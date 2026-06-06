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

# 4) README 的可執行引用(go get / go install / import 範例)
sed -i -e 's|^go get -u github.com/hibiken/asynq$|go get -u github.com/austinyuch/asynq|' \
       -e 's|"github.com/hibiken/asynq"|"github.com/austinyuch/asynq"|g' \
       -e 's|go install github.com/hibiken/asynq/tools/asynq@latest|go install github.com/austinyuch/asynq/tools/asynq@latest|' \
       README.md tools/asynq/README.md

remaining=$({ grep -rn '"github.com/hibiken/asynq' --include='*.go' . 2>/dev/null || true; } | wc -l)
echo "remaining hibiken imports in .go files: ${remaining}"
[ "${remaining}" -eq 0 ] || { echo "ERROR: rename incomplete" >&2; exit 1; }
