#!/usr/bin/env bash
# 镜像里的第三方许可证，Go 一侧（M12/P5/S2，P5 设计 3.11）。deploy/Dockerfile 的 server 阶段在 go build 之后、
# 在 server/ 下执行：
#   tools/licenses/go.sh <构建出的程序> <输出目录>
# 链接进 nervewiki 的每个模块（与构建同一个 GOOS/GOARCH、同一个模块缓存，CGO_ENABLED=0 同 go build 那一行）：
# 复制模块目录里名字像许可证的文件，一个也没有就失败、写明模块（P5 设计 P5-10）；Go 本身（标准库与运行时随二进制
# 发出）的 LICENSE、PATENTS 放进 go@<Go 的版本>/。收完之后与程序自己记的模块（go version -m 的 dep 行）和 Go 的
# 版本比，不同就失败：清单恰是链接进去的。INDEX.tsv 每行"<模块>@<版本> TAB <文件名，逗号分隔>"，排好序：同一份
# go.sum 得到同样的目录。tools/licenses/check.mjs 用假的 go 核对，image-smoke.sh 核对镜像里的结果。
set -euo pipefail
export LC_ALL=C

fail() {
  echo "licenses/go.sh: $*" >&2
  exit 1
}

[ $# -eq 2 ] || fail "用法：tools/licenses/go.sh <构建出的程序> <输出目录>"
program=$1 out=$2
# 许可证文件的名字，不分大小写：LICENSE、LICENCE、UNLICENSE、COPYING、NOTICE、PATENTS，可带 .txt、.md、-MIT 这样的
# 后缀；代码与数据的扩展名不算（js、cjs、mjs、jsx、ts、cts、mts、tsx、go、json、map、sh、py：license.go 不是许可证）。
# 与 npm.mjs 的是同一条。只看模块的根目录：今天链接进来的包所在的子目录里没有另外的许可证（M12/P5/S2 审查 A 核过）
pattern='^((un)?licen[cs]e|copying|notice|patents)([.-].*)?$'
code='\.([cm]?[jt]sx?|go|json|map|sh|py)$'
shopt -s nocasematch

mkdir -p "$out"
rows=()

# take <模块>@<版本> <目录>：复制目录里名字匹配的文件，记一行；一个也没有就失败
take() {
  local id=$1 dir=$2 f name names=()
  for f in "$dir"/*; do
    name=${f##*/}
    [[ -f $f && $name =~ $pattern && ! $name =~ $code ]] || continue
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

# 收的模块恰是程序里记的：go list 与构建对模块的看法不同（构建标签、CGO_ENABLED、GOOS 不一样）时在这里失败，
# 不发一份缺的清单
info=$(go version -m "$program") || fail "go version -m 读不了 $program"
[ "$(head -n 1 <<<"$info")" = "$program: $gover" ] || fail "$program 不是 $gover 构建的：$(head -n 1 <<<"$info")"
built=$(awk -F'\t' '$2 == "dep" { print $3 "@" $4 }' <<<"$info" | sort -u)
listed=$(cut -f1 <<<"$modules" | sort -u)
[ "$listed" = "$built" ] || fail "收的模块与 $program 记的不同：只在 go list 里的 [$(comm -23 <(echo "$listed") \
  <(echo "$built") | paste -sd' ' -)]，只在程序里的 [$(comm -13 <(echo "$listed") <(echo "$built") | paste -sd' ' -)]"

printf '%s\n' "${rows[@]}" | sort >"$out/INDEX.tsv"
echo "licenses/go.sh：${#rows[@]} 行写进 $out/INDEX.tsv，模块与 $program 记的相同"
