# M6/P2 提取结果与平台的预算：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M6/P2 提取结果与平台的预算 |
| 状态 | 进行中 |
| 基线 | P1 合并之后的 main；本文提交之后开分支 `m6-p2` |
| 上级文档 | [M6 总设计](00-M6-design.md) 4.7、第 7、8 节；[M4/P3 给 M6 的移交](handoffs/M4-P3-markdown-extensions.md)第 9 项；[M12 的移交](../M12-release/handoffs/M4-performance.md)第 3 项 |

---

## 1. 基线

作者读代码（2026-10-05）：

- **写入路径带着语法树**：
  - `page/app.ContentParser` 在判定之后取解析预算、`Parse`，把 `app.Parsed`（`*markdown.Document`：原文、空白化之后的原文、语法树、frontmatter、各扩展的提取结果）连同 `release` 交给用例；
  - 用例 `defer release()`，所以预算一直持到单元结束，其间包括等笔记本行与页面正文行的锁；
  - `Parsed` 经 `ContentWrite`、`PageDraft` 进 `domain.Change.Parsed`，交给守卫、参与者与观察者。今天没有注册者读它，只有任务项的勾选在写之前读 `Tasks(parsed)`。
- **预算是 page 模块自己的**：`page/adapter/markdown.Budget`（按字节的信号量，先到先得，最多等 `page.parse_max_wait`，然后 `shared.ServerBusy`，带一条"饱和"的日志），由 `page.New` 按 `Deps.ParseBudgetBytes`、`Deps.ParseMaxWait` 建，交给 `ContentParser` 与 `GetPageView`。
- **阅读视图**：`GetPageView` 取预算，`Render(ctx, Parse(content), PageRef{NotebookID, PageID})`，渲染之后放回。`markdown.Page` 与 `app.PageRef` 没有版本。
- **钉住这些行为的测试**：
  - `page/app/content_test.go`：调用的次序（`Take` 在判定之后、单元之前，`Release` 在最后）、只有写者让服务端解析、503、panic 时放回；
  - `page/adapter/markdown/budget_test.go`、`markdown_test.go`；
  - `bootstrap/page_content_test.go` 的 `TestTheParseBudgetBoundsWritesAndViews`：A 的写在等页面的锁时占着 64 KiB 的预算，B 的阅读视图答 503；
  - `page/extension_test.go` 的 `TestAnExtensionReachesTheReadingView`。

## 2. 目标与范围

做（总设计第 7 节 P2、4.7）：

- 平台：`markdown.Facts`（frontmatter 与各扩展的提取结果，不含语法树与原文）；`markdown.Budget`（从 page 的适配器移来）；`markdown.Page.Revision`。
- page：写入路径只带 `Facts`，预算在提取之后即归还；`PageRef.Revision`；预算由组合根交给模块。
- 组合根：建一个预算交给 page（P3 起也交给 linking）。
- 总体设计 4.3、13.1 第 19 条、13.3 第 3 条与风险表随之修订；M12 移交第 3 项随之关闭。

不做：

- 参与者"一次取够、取不到立即 503"的取法：P4，与参与者一起（总设计 4.7）。
- `Fetch` 用 `Revision`：P3。
- 配置项改名：`page.parse_budget_bytes`、`page.parse_max_wait` 照旧，它们说的仍是页面正文的解析。

## 3. 设计

### 3.1 文件

| 文件 | 内容 |
|---|---|
| `platform/markdown/facts.go`（新） | `Facts`、`Document.Facts()` |
| `platform/markdown/markdowntest/facts.go`（新） | `CheckFacts`：树不随 `Facts` 存活 |
| `platform/markdown/budget.go`（新，移自 `page/adapter/markdown/budget.go`） | `Budget`、`NewBudget`、`Take`、`ErrBusy` |
| `platform/markdown/markdown.go` | `Page.Revision`；`Extension.Extract` 的说明加"结果不得留着语法树"；包说明加 `golang.org/x/sync` |
| `page/adapter/markdown/markdown.go` | `Facts(content)`、`Render(ctx, content, page)`、`Tasks(facts)` |
| `page/adapter/markdown/budget.go` | 只剩把平台的预算接到 `app.ParseBudget`：`ErrBusy` 换成 `shared.ServerBusy`（`Retry-After` 1 秒） |
| `page/app/ports.go`、`content.go`、`create_page.go`、`put_page_content.go`、`toggle_task.go`、`unit_*.go`、`get_page_view.go` | `Parsed` 改名 `Facts`；`ContentParser` 不再交出 `release`；`PageRef.Revision` |
| `page/domain/change.go` | `Change.Parsed` 改名 `Facts` |
| `page/module.go` | `Deps.Budget *markdown.Budget` 代替 `ParseBudgetBytes`、`ParseMaxWait` |
| `bootstrap/deps.go`（与 `app.go`） | 建一个预算交给 page |

### 3.2 平台：`Facts`

```go
// Facts is what a parse found that outlives it (M6 design 4.7): the
// frontmatter and what each extension took, neither the tree nor the
// content.
type Facts struct {
	frontmatter Frontmatter
	extracted   map[string]any
}

func (d *Document) Facts() Facts
func (f Facts) Frontmatter() Frontmatter
func (f Facts) Extracted(name string) any
```

- `Document` 的 `Frontmatter()`、`Extracted()` 照旧（阅读视图与测试用）。
- `Extension.Extract` 的约定加一句：它的结果比语法树活得久，不得留着树的节点（节点互相指着，留一个就留住整棵树）或 `Tree.Content`。tasks 与 obsidian 今天都满足：位置是整数，字符串是复制出来的。
- `Facts` 的内存与正文同阶、常数小：frontmatter 的属性与标量表（带转义的引号字符串，每个字节一个位置），提取出的链接与标签。

### 3.3 平台：`Budget`

从 `page/adapter/markdown/budget.go` 原样移来，只改两处：

- 取不到时答平台的 `ErrBusy`，不答 `shared.ServerBusy`：平台不能导入 `shared`（archtest 的 `platformIsBusinessFree`）。"饱和"的日志照旧在平台里写。
- 包说明：`platform/markdown` 多导入 `golang.org/x/sync/semaphore`（不是 Markdown 的库，archtest 不必改）。

适配器各自把 `ErrBusy` 换成 503：page 的在 `page/adapter/markdown`，P3 起 linking 的在它自己的适配器。

### 3.4 page：写入路径只带 `Facts`

- `app.Markdown` 端口：
  - `Facts(content string) Facts`：解析、提取，丢掉语法树；
  - `Render(ctx, content string, page PageRef) (string, error)`：阅读视图自己解析再渲染，语法树只活在这一次调用里；
  - `Tasks(facts Facts) []Task`。
- `ContentParser.Parse`、`Decided` 答 `(Facts, error)`：取预算、`Facts`、放回（`defer`，解析 panic 时同样放回），再交给用例。用例不再 `defer release()`。
- 于是写入在单元里等锁时不占预算。M4/P4 第 7 节与[M12 移交](../M12-release/handoffs/M4-performance.md)第 3 项的那条风险（一页的锁争用让别处的大页面阅读视图 503）随之消失，移交第 3 项关闭。
- `Parsed` 改名 `Facts`（`app.Facts`、`ContentWrite.Facts`、`PageDraft.Facts`、`domain.Change.Facts`），在 page 里仍不透明。P3 的观察者经组合根拿到的是 `markdown.Facts`。
- 观察者调用之后不得留着 `Facts` 的约定（总体设计 13.3 第 3 条）去掉：`Facts` 不在预算之内，它的大小与正文同阶，留着也不过与正文本身相当。

### 3.5 page：`Revision`

`markdown.Page` 与 `app.PageRef` 加 `Revision`；`GetPageView` 交出它读到的版本。P3 的 `Fetch` 用它判断索引是不是这个版本的（总设计 4.7"阅读视图的一致性"）。

### 3.6 组合根

- `bootstrap` 按配置建一个 `markdown.NewBudget(cfg.Page.ParseBudgetBytes, cfg.Page.ParseMaxWait, logger)`，交给 `page.Deps.Budget`；P3 起同一个交给 linking。
- `page.New` 用交来的预算，不自己建。

## 4. 实施步骤

| 步 | 内容 | 提交 |
|---|---|---|
| S1 | 平台：`Facts`、`Budget`（移来，`ErrBusy`）、`Page.Revision`；树不随 `Facts` 存活的测试 | `markdown: a parse's facts, the parse budget, a page's revision (M6/P2/S1)` |
| S2 | page 与组合根：`Facts` 代替 `Parsed`，提取之后归还预算，`PageRef.Revision`，预算由组合根交来；总体设计与 M12 移交的修订 | `page, bootstrap: a write keeps its content's facts, the budget back at once (M6/P2/S2)` |

## 5. 测试与验证

- **平台**：
  - 树不随 `Facts` 存活（`markdowntest.CheckFacts`）：一个探针扩展在 `Extract` 里弱引用语法树的根（`weak.Pointer`），解析普通正文、只留 `Facts`，`runtime.GC()` 之后根已被回收。平台不带扩展与带测试扩展各跑一次，组合根以应用注册的扩展跑一次；
  - `Facts` 与 `Document` 的 frontmatter、提取结果相同；
  - 预算的测试随代码移来（取、放、等待、饱和、`ErrBusy`、零与负的拒绝）。
- **page**：
  - 调用的次序：`Take n`、`Facts`、`Release n` 都在单元之前（写、建、勾选）；
  - 预算取不到时 503 在解析与单元之前；解析 panic 时放回；只有写者让服务端解析（照旧）；
  - 适配器：`ErrBusy` 答 503 `server_busy`，`Retry-After` 1 秒；其他错误照旧；
  - 阅读视图把读到的版本交给渲染（`PageRef.Revision`），扩展的 `Fetch` 收到它（`TestAnExtensionReachesTheReadingView`）；
  - 模块用交来的预算：测试先占满一个预算交给 `page.New`，写与阅读视图都答 503。
- **整个程序**（`TestTheParseBudgetBoundsWritesAndViews` 改写为 `TestAWriteWaitingForItsLockHoldsNoBudget`）：A 的写在等页面的锁时，B 的阅读视图答 200，A 的写在锁放开之后答 200。改写之前它答 503，这一条钉住"提取之后即归还"。
- **反向对照**：`Facts` 留着 `Document`；`ContentParser` 照旧持到单元结束；`GetPageView` 不交版本；适配器不换 `ErrBusy`；`page.New` 自己建预算。每个都要有测试失败。
- `make check`、`make gen-check`、e2e 全量。

## 6. 完成标准

- 写入路径里没有语法树：`Change` 带的是 `Facts`，平台的测试证明树不随它存活。
- 预算在提取之后即归还，由组合根建一个、交给 page。
- `markdown.Page`、`app.PageRef` 带 `Revision`。
- M4、M5 的写入路径测试照旧通过（只改调用次序与名称）。
- 总体设计与 M12 移交的修订落档。

## 7. 结果

（完成后填写）
