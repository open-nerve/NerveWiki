# Nerve Wiki

单个 Go 程序加 PostgreSQL 与 React 网页的 wiki，带 MCP 服务端。文档是权威，这里只是入口。

## 先读

1. [docs/README.md](docs/README.md)：文档约定、完成标准、handoff 规则、Git 的写法。
2. [docs/v0.1/claude_to_codex/02-workflow.md](docs/v0.1/claude_to_codex/02-workflow.md)：从一个 M 到一个 Step 的推进方式，审查、变异、文档、提交、合并与 CI。目录名是历史，内容现行。
3. 同目录的 [05-lessons.md](docs/v0.1/claude_to_codex/05-lessons.md)（审查一再发现的问题）与 [06-environment.md](docs/v0.1/claude_to_codex/06-environment.md)（命令与规矩）。
4. [docs/v0.1/v0.1-design.md](docs/v0.1/v0.1-design.md)：第 12 节的里程碑，第 13 节的长期约定，每一条都约束实现。
5. 当前的 M 的总设计：第 11 节是决定，第 12 节是进度。

## 规矩

- 一个 M 之内自己推进，不必停下来问；开新的 M 要负责人批准。
- 小的产品决定取推荐的做法，记成"主持者的决定（负责人可以改判）"。
- 不 force-push `main`；只把完成且验证过的状态推上 `main`。
- 不碰真实的 MCP 客户端配置（`~/.claude.json`、`~/.claude/`、`~/.codex/`、已有的 `.mcp.json`）。
- 兄弟项目 Nerve 只读；不读负责人的笔记。
- 不用 `pkill`、`killall`，不重启 Docker。
- 重的运行（bootstrap 测试、`make test`、`make e2e`、量时间）经 `docs/v0.1/claude_to_codex/tools/heavy.sh`。
