# 安全

## 报告漏洞

请经 GitHub 的私密漏洞报告提交：在仓库的 Security 页点 "Report a vulnerability"（[直达](https://github.com/open-nerve/NerveWiki/security/advisories/new)）。**不要开公开的 issue**，也不要在讨论区、PR 里写漏洞的细节。

写明受影响的版本（`/api/v0/instance` 的 `version` 或 `nervewiki version` 的输出）、怎样复现、影响是什么。我们收到之后确认、修复，在修复的版本发布时公开这份报告。

## 支持的版本

| 版本  | 支持                     |
| ----- | ------------------------ |
| 0.1.x | 只支持最新的那个补丁版本 |

安全修复发在下一个补丁版本里（例如 0.1.1），不回到更早的补丁版本。升级照 [deploy/README.md](deploy/README.md) 的"升级与回退"。

## 校验镜像

镜像 `ghcr.io/open-nerve/nervewiki` 带 GitHub 的来源证明、两份 SBOM 的证明与 Sigstore 的签名（keyless）。校验的命令在每个版本的 Release 的说明里，也在 [deploy/README.md](deploy/README.md) 的"安装"。只用校验过的镜像；镜像只有确切的版本号，没有 `latest`。
