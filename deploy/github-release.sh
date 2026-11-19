#!/usr/bin/env bash
# GitHub 的 Release：先草稿、传附件，两种架构上核对过才公开（00-M12 4.10"Release"、M12/P5/S2，P5 设计 3.13）。
# release.yml 的 draft 与 release 两个作业在仓库根执行：
#   deploy/github-release.sh draft <tag> <说明文件> <附件目录>
#   deploy/github-release.sh publish <tag>
# 附件恰是三个，名字由 tag 算出：nervewiki-<版本>-image.spdx.json、nervewiki-<版本>-web.spdx.json、
# nervewiki-<版本>-licenses.tar.gz；rc 是预发布。
# draft：没有就建草稿（--verify-tag：tag 要已在），是草稿就改说明，之后传附件（--clobber）、核对附件；已公开（重跑）的
#   不再上传、不改说明，只核对附件。
# publish：草稿的附件齐了才公开，再读一次要是已公开；已公开、附件齐的不动；没有就失败（草稿该已由 draft 建好）。
# gh release view 的失败只有"release not found"当作没有：gh 的任何失败都以 1 退出，一时读不到就当作没有，会再建一个草稿
# （P5 设计 P5-12）。GitHub 的 immutable releases 打开时公开之后附件与 tag 都不能改，先草稿后公开在两种设置下都能用。
# 要 gh（GH_TOKEN 要 contents: write，GH_REPO 是 owner/repo）。server/internal/platform/buildinfo/release_test.go 用假的 gh
# 核对每一支。
set -uo pipefail

fail() {
  echo "github-release: $*" >&2
  exit 1
}

usage="用法：deploy/github-release.sh draft <tag> <说明文件> <附件目录>，或 deploy/github-release.sh publish <tag>"
[ $# -ge 2 ] || fail "$usage"
action=$1 tag=$2
case $action in
draft) [ $# -eq 4 ] || fail "$usage" ;;
publish) [ $# -eq 2 ] || fail "$usage" ;;
*) fail "$usage" ;;
esac
[[ $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.[1-9][0-9]*)?$ ]] ||
  fail "tag \"$tag\" 不是 vX.Y.Z 或 vX.Y.Z-rc.N"
[ -n "${GH_REPO:-}" ] || fail "要 GH_REPO（owner/repo）：不靠检出的 remote 去猜"
version=${tag#v}
assets=("nervewiki-$version-image.spdx.json" "nervewiki-$version-web.spdx.json" "nervewiki-$version-licenses.tar.gz")
want=$(printf '%s\n' "${assets[@]}" | sort)
prerelease=()
if [[ $version == *-rc.* ]]; then
  prerelease=(--prerelease)
fi
err=$(mktemp) || fail "mktemp"
trap 'rm -f "$err"' EXIT

# 这个 tag 的 Release：absent、draft 或 published
state() {
  local draft
  if draft=$(gh release view "$tag" --json isDraft --jq .isDraft 2>"$err"); then
    case $draft in
    true) echo draft ;;
    false) echo published ;;
    *) fail "读不懂 $tag 的 isDraft：$draft" ;;
    esac
  else
    grep -q 'release not found' "$err" || fail "读不到 Release $tag：$(cat "$err")"
    echo absent
  fi
}

# Release 的附件恰是那三个
assets_are_three() {
  local have
  have=$(gh release view "$tag" --json assets --jq '.assets[].name' 2>"$err") ||
    fail "读不到 $tag 的附件：$(cat "$err")"
  have=$(sort <<<"$have")
  [ "$have" = "$want" ] || {
    echo "github-release: $tag 的附件是 [$(paste -sd' ' <<<"$have")]，要恰是 [$(paste -sd' ' <<<"$want")]" >&2
    return 1
  }
}

case $action in
draft)
  notes=$3 dir=$4
  [ -s "$notes" ] || fail "说明 $notes 不在或是空的"
  have=$(cd "$dir" 2>/dev/null && ls -A | sort) || fail "附件目录 $dir 读不了"
  [ "$have" = "$want" ] || fail "附件目录 $dir 要恰是 [$(paste -sd' ' <<<"$want")]，有 [$(paste -sd' ' <<<"$have")]"
  s=$(state) || exit 1
  case $s in
  published)
    assets_are_three || fail "$tag 已公开，附件不对：已公开的不再上传（immutable releases）"
    echo "$tag 已公开，附件齐，不动"
    exit 0
    ;;
  absent)
    gh release create "$tag" --draft --verify-tag --title "Nerve Wiki $tag" --notes-file "$notes" "${prerelease[@]}" ||
      fail "建不出 $tag 的草稿"
    ;;
  draft)
    gh release edit "$tag" --notes-file "$notes" "${prerelease[@]}" || fail "改不了 $tag 的草稿"
    ;;
  esac
  gh release upload "$tag" "${assets[@]/#/$dir/}" --clobber || fail "传不上 $tag 的附件"
  assets_are_three || fail "传过之后 $tag 的附件不对"
  echo "$tag 的草稿与三个附件就绪"
  ;;
publish)
  s=$(state) || exit 1
  case $s in
  absent) fail "没有 $tag 的 Release：草稿该已由 draft 作业建好" ;;
  published)
    assets_are_three || fail "$tag 已公开，附件不对"
    echo "$tag 已公开，附件齐，不动"
    exit 0
    ;;
  esac
  assets_are_three || fail "$tag 的草稿附件不齐，不公开"
  gh release edit "$tag" --draft=false || fail "公开不了 $tag"
  s=$(state) || exit 1
  [ "$s" = published ] || fail "公开之后 $tag 是 $s"
  echo "$tag 已公开"
  ;;
esac
