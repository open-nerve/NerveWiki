```yaml
status: open
from: M4/P1
to: M9
created: 2026-10-02
```

# 客户端名称的检查与区域设置

M4/P1 的变更集记下写入的来源（[P1 文档](../../M4-pages/01-P1-page-module-pipeline.md) 3.4、3.6）：`web`、`api`、`cli`，或 `mcp:` 加 1–128 个非控制字符。同一条规则写了两处：应用里的 `domain.Client.Valid`（Go 的 `unicode.IsControl`），数据库里的 `changesets_client_check`（`client ~ '^mcp:[^[:cntrl:]]{1,128}$'`）。

`[:cntrl:]` 随数据库的 `LC_CTYPE` 而变，`Client.Valid` 不随。P1 审查在测试库（`datctype = C.UTF-8`）上核对过：U+0085、U+009F、U+007F 两边都拒，U+2028、U+200B、U+00AD 两边都收，一致（[P1 审查](../../M4-pages/reviews/P1-page-module-pipeline-review.md) Q4）。M4 只写前三种，碰不到它。

M9 加 `mcp:<名>` 时：

1. 名称从 MCP 客户端的握手里来，先经 `Client.Valid`，不合规的按 M9 的规则换掉或拒绝，不能让数据库的检查答 500。
2. 在部署的镜像所用的区域设置上，用上面六个字符再核对一次两边一致；不一致时改成不依赖区域设置的写法（例如在检查里列出控制字符的范围）。
