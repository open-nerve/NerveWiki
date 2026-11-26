```yaml
status: open
from: M12
to: v0.2
created: 2026-11-22
```

# 发布、依赖与部署：v0.1 没有做的

M12 只做与安全相关的依赖更新、单实例的部署与 tag 触发的发布流水线（[00-M12](../../v0.1/M12-release/00-M12-design.md) 4.3、4.8、4.10）。其余写在这里（00-M12 4.13；[P1 设计](../../v0.1/M12-release/01-P1-foundation.md)与 [P5 设计](../../v0.1/M12-release/05-P5-deploy-release.md)的第 9 节）。

1. **Node 26 与功能性的大版本依赖**（00-M12 4.3；决定 353）：Node 26 评估过，v0.1.0 仍用 24。River 0.51、katex 0.18/0.19（连同我们对它的补丁重做，与 `pnpm-workspace.yaml` 的 `audit.ignore` 里的 GHSA-238p-pmpm-9mq7，low，修在 0.18.2）、`@lezer/markdown` 1.8、mermaid 12、pgx、goose、`x/text`。
2. **Dependabot 的版本更新 PR：不开**（00-M12 第 11 节"需要负责人确认"第 6 项；移交 M0-P6-image-release 第 4 条）：负责人 2026-11-22 第 14 问：不开；发布之前在 runner 上跑 `make outdated`（00-M12 4.3）。v0.2 若要改，再问负责人。Dependabot alerts 只管 Go 与 actions：GitHub 的依赖图读不出 pnpm 12 的锁文件，npm 一侧靠 `make vuln` 的 `pnpm audit --prod`。
3. **多实例**（00-M12 4.8；移交 M5-performance 第 1、2 条）：v0.1 只支持单实例：附件在本地磁盘，限流、大请求体的名额与正在上传的导入只在进程内计数，T4 的启动收拾假定单实例（deploy/README"要求"）。事件流每个凭证 4 条、每个账户 64 条（常量，决定 315）与多实例、配置项一起重看（决定 696）。
4. **构建与发布的工具**（P5 第 9 节）：独立的二进制附件；构建的缓存（P5-7）；actionlint 查 `run:` 里的脚本（P5-6）；`outdated.sh` 不列 actionlint、syft、cosign、BuildKit。
5. **第三方许可证里没有文本的包**（决定 439；P5 第 9 节）：fastdom 1.0.12、react-remove-scroll-bar 2.3.8、strictdom 1.0.1（`package.json` 都写 MIT），镜像的 `/licenses` 照实列为"没有文本"。发布说明的"第三方许可证"一条。
6. **GHCR 里留下的 `sha-` tag 与没有 tag 的摘要清不清**（P5 设计 3.23 末、第 9 节）：rc 与 `v0.1.0` 的发布留下的，负责人定。
7. **续期途中刷新会被登出**（00-M12 4.4；设计审查 C 的 N6）：访问令牌续期的答复还没收到时刷新或离开，旧令牌再用算重复使用，会话结束（M1 第 10 节接受的后果）。e2e 由帮手 `e2e/fixtures/navigation.ts` 避开它（M12/P1/S1）。重看 M1 第 10 节的取舍；续期答错误之后被登出是同一处的另一面（[M12-polish.md](M12-polish.md) 第 13 项）。发布说明的"用户在续期途中刷新页面会被登出"一条。
8. **从 v0.1.0 之前的试用库升上来**（00-M12 4.13"逐项核对去处"；P3 第 9 节）：标题里有切罗基文的，升级之后要跑一次 `nervewiki reindex`，已有只差切罗基文大小写的兄弟页时它报撞键、这本整个不重建，先改名再跑；标题里有自动链接或原始 HTML 的，锚点 id 变了。负责人 2026-11-17 第 8 问说没有这样的实例；发布说明写了。v0.2 若再改标题键或 id，照同样写明。
9. **`close` 在 `run` 已经停过 lint 时不再等**（决定 694）：`run` 停服务时与 `close` 关闭之前各停一次 lint 的工作者，各至多 `database.commit_timeout` 加 5 秒；停机因此至多 52 秒（20 + 2 + 11 + 7 + 7 + 5），宽限 60 秒（`deploy/compose.yaml` 的 `stop_grace_period`）。让 `close` 在 `run` 已停过时不再等，就省下其中一个 7 秒；改了之后 deploy/README、`config.yaml` 与 `compose.yaml` 的数随之。
10. **Release 页上的标题层级倒过来**（决定 706）：`CHANGELOG.md` 一节的 `###` 在前、`release-notes.sh` 接的 `## 镜像` 等在后，只是外观；要改得同时改 `release-notes.sh` 与 `release_test.go` 的期望。
11. **以后要跨文档后退的 e2e 故事**（P1 第 9 节）：在帮手里加 `goBackAcross`（先 `settle`，再 `page.goBack()`，断言真的换了文档），照 A15 加一格对照，门禁照旧。
12. **发布脚本的测试在 macOS 的 bash 3.2 下失败**（M13 收尾的全量门禁，负责人 2026-11-27 答"接受，靠 CI"）：`deploy/release-version-tag.sh`、`release-gate.sh`、`release-notes.sh` 里变量后面紧跟全角标点（如 `$have，`、`$sha，`、`$base，`），macOS 自带的 `/bin/bash` 3.2 把那几个字节读进变量名，`set -u` 下报 `unbound variable`，`LC_ALL=C.UTF-8` 与 `en_US.UTF-8` 都一样；`server/internal/platform/buildinfo` 的 `TestTheReleaseScripts`、`TestTheReleaseGate`、`TestTheReleaseNotes` 随之失败。CI 是 Linux 的 bash 5，不受影响；在 Mac 上跑 `make test` 或这几个脚本的人会碰到。候选的改法：变量写成 `${have}` 这样带花括号，或 README 的"需要安装"写 bash 4 以上、`needTools` 查版本。

## 处理进展

- M12 收尾（2026-11-22）：建这一份，收 00-M12 4.13 的 `M12-release.md` 一行、P1、P5 第 9 节交来的、决定 694、696、706 与"需要负责人确认"第 6 项（负责人 2026-11-22 第 14 问；收尾记录 `docs/v0.1/M12-release/reviews/M12-closeout-review.md`）。v0.2 的总体设计开工时先处理它。
- M13 收尾（2026-11-27）：加第 12 项（`main` 合并之后的全量门禁里看到的；收尾记录 `docs/v0.1/M13-ui-refresh/reviews/M13-closeout-review.md`"全量门禁"）。
