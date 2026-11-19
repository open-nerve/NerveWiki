#!/usr/bin/env bash
# 版本号的 tag（00-M12 4.10"次序与重跑"第 4 步、M12/P5/S2）。release.yml 的 publish 作业在证明与签名之后、最后一步执行：
#   deploy/release-version-tag.sh <镜像> <版本> <索引的摘要>
# 已在：指着同一个摘要就不动；指着别的就失败、不改指：整次运行重跑时构建得到新的摘要，0.1.0 不能悄悄换成它（P5 设计 P5-12）。
# 不在：只认 registry 答的 not found，从索引的摘要原样复制出这个 tag（单个来源，摘要不变）；别的错误（一时读不到、没有
# 权限）不当作不在。最后再读一次，要恰是这个摘要。
# 要 docker（buildx 的 imagetools）与 jq；登录由作业做。server/internal/platform/buildinfo/release_test.go 用假的 docker 核对每一支。
set -uo pipefail

fail() {
  echo "release-version-tag: $*" >&2
  exit 1
}

[ $# -eq 3 ] || fail "用法：deploy/release-version-tag.sh <镜像> <版本> <索引的摘要>"
image=$1 version=$2 digest=$3
[[ $version =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.[1-9][0-9]*)?$ ]] || fail "版本号 \"$version\" 不对"
[[ $digest =~ ^sha256:[0-9a-f]{64}$ ]] || fail "摘要 \"$digest\" 不对"
err=$(mktemp) || fail "mktemp"
trap 'rm -f "$err"' EXIT

# 已在：指着同一个摘要就不动，指着别的就失败、不改指。不在：只认 registry 答的 not found；别的错误不当作不在
if have=$(docker buildx imagetools inspect "$image:$version" --format '{{json .Manifest}}' 2>"$err"); then
  have=$(jq -r .digest <<<"$have") || fail "读不懂 $image:$version 的清单"
  [ "$have" = "$digest" ] || fail "$image:$version 已指向 $have，不改指到 $digest"
  echo "$image:$version 已指向 $digest，不动"
else
  grep -qiE 'not found|manifest unknown' "$err" || fail "读不到 $image:$version：$(cat "$err")"
  docker buildx imagetools create -t "$image:$version" "$image@$digest" || fail "复制不出 $image:$version"
fi
got=$(docker buildx imagetools inspect "$image:$version" --format '{{json .Manifest}}' | jq -r .digest) ||
  fail "复制之后读不到 $image:$version"
[ "$got" = "$digest" ] || fail "$image:$version 是 $got，要 $digest"
echo "$image:$version → $digest"
