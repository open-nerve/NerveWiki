#!/usr/bin/env bash
# 镜像里的第三方许可证，Go 一侧（M12/P5/S2，P5 设计 3.11）。deploy/Dockerfile 的 server 阶段在 go build 之后、
# 在 server/ 下执行：
#   tools/licenses/go.sh <输出目录>
# 链接进 nervewiki 的每个模块（与构建同一个 GOOS/GOARCH、同一个模块缓存，CGO_ENABLED=0 同 go build 那一行）：
# 复制模块目录里名字像许可证的文件，一个也没有就失败、写明模块（P5 设计 P5-10）；Go 本身（标准库与运行时随二进制
# 发出）的 LICENSE、PATENTS 放进 go@<Go 的版本>/。INDEX.tsv 每行"<模块>@<版本> TAB <文件名，逗号分隔>"，排好序：
# 同一份 go.sum 得到同样的目录。image-smoke.sh 核对结果。
set -euo pipefail
export LC_ALL=C

fail() {
  echo "licenses/go.sh: $*" >&2
  exit 1
}

[ $# -eq 1 ] || fail "用法：tools/licenses/go.sh <输出目录>"
out=$1
# 许可证文件的名字，不分大小写：LICENSE、LICENCE、COPYING、NOTICE、PATENTS，可带 .txt、.md、-MIT 这样的后缀。
# 只看模块的根目录：子目录里的是第三方代码自己带的，随它的模块另算
pattern='^(licen[cs]e|copying|notice|patents)([.-].*)?$'
shopt -s nocasematch

mkdir -p "$out"
rows=()

# take <模块>@<版本> <目录>：复制目录里名字匹配的文件，记一行；一个也没有就失败
take() {
  local id=$1 dir=$2 f name names=()
  for f in "$dir"/*; do
    name=${f##*/}
    [[ -f $f && $name =~ $pattern ]] || continue
    mkdir -p "$out/$id"
    install -m 0644 "$f" "$out/$id/$name"
    names+=("$name")
  done
  [ ${#names[@]} -gt 0 ] || fail "模块 $id 没有许可证文件（$dir）"
  rows+=("$id"$'\t'"$(IFS=,; echo "${names[*]}")")
}

modules=$(CGO_ENABLED=0 go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}}@{{.Version}}{{"\t"}}{{.Dir}}{{end}}{{end}}' ./cmd/nervewiki | sort -u)
[ -n "$modules" ] || fail "go list 没有列出模块"
while IFS=$'\t' read -r id dir; do
  [ -n "$dir" ] || fail "模块 $id 不在模块缓存里：先 go mod download"
  take "$id" "$dir"
done <<<"$modules"

# Go 本身：两份都要在
goroot=$(go env GOROOT)
gover=$(go env GOVERSION)
for f in LICENSE PATENTS; do
  [ -s "$goroot/$f" ] || fail "Go 本身没有 $f（$goroot）"
done
take "go@$gover" "$goroot"

printf '%s\n' "${rows[@]}" | sort >"$out/INDEX.tsv"
echo "licenses/go.sh：${#rows[@]} 行写进 $out/INDEX.tsv"
