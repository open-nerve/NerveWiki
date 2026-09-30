```yaml
status: open
from: M0/P6
to: M12
created: 2026-09-30
```

# 镜像的发布

M0/P6 只在本地与持续集成中构建镜像并做冒烟检查（`make image`、`make image-smoke`，持续集成的 `image` 任务），不推送到任何仓库（[P6 文档](../../M0-foundation/06-P6-e2e-delivery.md) 第 2 节"不做"）。M12 的发布流程要补上：

1. **推送**：发布时构建带正式版本号的镜像（`VERSION=0.1.0`），推送到镜像仓库。持续集成需要写权限，只在发布的触发条件下授予。
2. **多架构**：基础镜像的摘要都是多架构索引，`CGO_ENABLED=0` 的构建可以交叉编译。用 `docker buildx` 同时构建 amd64 与 arm64，每个架构都跑一次 image-smoke。
3. **签名与来源证明**：给镜像签名，附上 SBOM 与构建来源证明（provenance）；说明中写明如何校验。
4. **依赖、工具链与基础镜像的更新**：版本都精确固定（总体设计 13.5），升级全靠人来改。发布前逐项核对是否过时：Go 的 `toolchain`、Node 的大版本（Node 26 成为 LTS 之后评估升级；Node 25 起不再自带 corepack，`corepack enable` 这一步与 README 要随之调整）、pnpm、npm 与 Go 的依赖、actions 的 SHA、基础镜像的摘要。是否引入 Dependabot 之类的自动更新，由负责人决定（M0/P2 审查留下的问题）。
