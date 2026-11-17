#!/usr/bin/env bash
# 发布说明（00-M12 4.10、M12/P5/S1）。release.yml 建 Release 的草稿时在仓库根执行，说明写到标准输出：
#   deploy/release-notes.sh <tag> <索引的摘要>
# 取 CHANGELOG.md 里这个版本的一节（rc 取去掉 -rc.N 的那一节，前面加一句"预发布"），后面接镜像、摘要、两条校验的
# 命令与附件。tag 的写法不对、摘要不是 sha256:<64 位十六进制>、没有这一节或它是空的，就以 1 退出。闸门也调它一次。
# 要 IMAGE（镜像名，不带 tag）与 GITHUB_REPOSITORY。server/internal/platform/buildinfo/release_test.go 对着假的 CHANGELOG.md 核对。
set -uo pipefail

fail() {
  echo "release-notes: $*" >&2
  exit 1
}

[ $# -eq 2 ] || fail "用法：deploy/release-notes.sh <tag> <索引的摘要>"
tag=$1
digest=$2
[[ $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.[1-9][0-9]*)?$ ]] ||
  fail "tag \"$tag\" 不是 vX.Y.Z 或 vX.Y.Z-rc.N"
[[ $digest =~ ^sha256:[0-9a-f]{64}$ ]] || fail "摘要 \"$digest\" 不是 sha256:<64 位十六进制>"
image=${IMAGE:-}
[ -n "$image" ] || fail "要 IMAGE（镜像名，不带 tag）"
repo=${GITHUB_REPOSITORY:-}
[ -n "$repo" ] || fail "要 GITHUB_REPOSITORY（owner/repo）"
version=${tag#v}
base=${version%%-rc.*}

# 这一节：以"## <base>（"起头的那一行之后，到下一个"## "之前，去掉开头与结尾的空行。index 按字面比字节：版本号里的 .
# 不当通配，"## 0.1.0（" 不匹配 "## 0.1.00（"；"### " 不是 "## "。exit 之后照样走 END。"下一个"先判：同一个版本的
# 第二个标题（例如写坏了的"## 0.1.0（草稿"）也结束这一节，不另起一节接着读，与闸门的"恰好一个标题"一致
[ -f CHANGELOG.md ] || fail "没有 CHANGELOG.md"
awk -v h="## $base（" 'index($0, h) == 1 { found = 1 } END { exit !found }' CHANGELOG.md ||
  fail "CHANGELOG.md 里没有\"## $base（…）\"的一节"
section=$(awk -v h="## $base（" '
  on && /^## / { exit }
  index($0, h) == 1 { on = 1; next }
  on { lines[++n] = $0 }
  END {
    first = 1
    while (first <= n && lines[first] ~ /^[ \t\r]*$/) first++
    last = n
    while (last >= first && lines[last] ~ /^[ \t\r]*$/) last--
    for (i = first; i <= last; i++) print lines[i]
  }' CHANGELOG.md) || fail "读不了 CHANGELOG.md"
[ -n "$section" ] || fail "CHANGELOG.md 里\"## $base（…）\"的一节是空的"

if [ "$version" != "$base" ]; then
  printf '这是预发布版本 %s，供核对与试用；正式版本是 v%s。\n\n' "$tag" "$base"
fi
printf '%s\n\n' "$section"
# 两条校验的身份逐字相同，都钉住这个 tag 上的 release.yml：只带 -R 时仓库里任何工作流、任何 ref 做的证明都算数。
# release.yml 的 verify 作业用同一个身份
identity="https://github.com/$repo/.github/workflows/release.yml@refs/tags/$tag"
cat <<EOF
---

## 镜像

\`linux/amd64\` 与 \`linux/arm64\` 两种架构，只有确切的版本号，没有 \`latest\`：

\`\`\`bash
docker pull $image:$version
\`\`\`

索引的摘要：\`$digest\`。部署、升级与备份照 [deploy/README.md](https://github.com/$repo/blob/$tag/deploy/README.md)。

## 校验

来源证明（GitHub 的 artifact attestation，\`gh\` 要先 \`gh auth login\`）与签名（Sigstore，keyless，用 cosign v3；流水线用 v3.1.4）：

\`\`\`bash
gh attestation verify oci://$image:$version -R $repo \\
  --cert-identity $identity
cosign verify $image:$version \\
  --certificate-identity=$identity \\
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
\`\`\`

两份 SBOM 的证明：第一条加 \`--predicate-type https://spdx.dev/Document/v2.3\`。

## 附件

- \`nervewiki-$version-image.spdx.json\`：镜像的 SBOM（SPDX 2.3，syft 从 amd64 的镜像生成，含二进制里的 Go 模块）；
- \`nervewiki-$version-web.spdx.json\`：内嵌的前端的生产依赖（SPDX 2.3）；
- \`nervewiki-$version-licenses.tar.gz\`：第三方许可证，与镜像里的 \`/licenses\` 相同。
EOF
