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
- page：写入路径只带 `Facts`，提取之后只留下 `Facts` 那一份预算（约为正文字节的十分之一），到单元结束；`PageRef.Revision`；预算由组合根交给模块。
- 组合根：建一个预算交给 page（P3 起也交给 linking）。
- 总体设计 4.3、13.1 第 19 条、13.3 第 3 条与风险表随之修订；M12 移交第 3 项随之改写（缓解，没有关闭）。

不做：

- 参与者"一次取够、取不到立即 503"的取法：P4，与参与者一起（总设计 4.7）。
- `Fetch` 用 `Revision`：P3。
- 配置项改名：`page.parse_budget_bytes`、`page.parse_max_wait` 照旧，它们说的仍是页面正文的解析。

## 3. 设计

### 3.1 文件

| 文件 | 内容 |
|---|---|
| `platform/markdown/facts.go`（新） | `Facts`、`Document.Facts()` |
| `platform/markdown/markdowntest/facts.go`（新）、`costs.go` | `CheckFacts`：树不随 `Facts` 存活；`CheckCosts` 加 `Facts` 留下的堆不超过 `Facts.Limit` |
| `platform/markdown/budget.go`（新，移自 `page/adapter/markdown/budget.go`） | `Budget`、`NewBudget`、`Take` 答 `Hold`（`KeepFacts(facts)`、`Release`）、`ErrBusy`、`FactsRatio`；`facts.go` 的 `Facts.Limit` |
| `platform/markdown/markdown.go` | `Page.Revision`；`Extension.Extract` 的说明加"结果不得留着语法树"；包说明加 `golang.org/x/sync` |
| `page/adapter/markdown/markdown.go` | `Facts(content)`、`Render(ctx, content, page)`、`Tasks(facts)` |
| `page/adapter/markdown/budget.go` | 只剩把平台的预算接到 `app.ParseBudget`：`ErrBusy` 换成 `shared.ServerBusy`（`Retry-After` 1 秒） |
| `page/app/ports.go`、`content.go`、`create_page.go`、`put_page_content.go`、`toggle_task.go`、`unit_*.go`、`get_page_view.go` | `Parsed` 改名 `Facts`；`ContentParser` 交出 `Facts` 与放回 `Facts` 那一份的 `release`；`PageRef.Revision` |
| `page/domain/change.go` | `Change.Parsed` 改名 `Facts` |
| `page/module.go` | `Deps.Budget *markdown.Budget` 代替 `ParseBudgetBytes`、`ParseMaxWait` |
| `bootstrap/deps.go`、`wire.go` | `parsing`：建一个 Markdown 与一个预算（配置的大小与等待），预算交给 page |

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
- `Facts` 的内存与正文同阶，但常数不小（审查 M1、修复核对 M-1 实测）：普通的正文约 1.1 倍；满页的 `[[a]]` 约 19–21 倍（每条链接一个约 96 字节的 `obsidian.Link`；第三轮核对 M1 之前，追加留下的两倍容量让 2 KB 以下的页到 38 倍，现在提取结果的切片按长度复制一份），`[a](b)` 约 15 倍，`#a` 约 10–13 倍，只有一个转义的引号字符串约 10 倍（标量表每个字节一个位置）；frontmatter 的每个值另有约 130–300 字节（属性、标量、属性链接），在 YAML 的值数上限（一万）之下，很密的小 frontmatter 可达正文的 70 倍。
  - 每个字符串标量还带它的路径（上面的键逐层以 `.` 连起来），长键压在长列表之上时每一项都复制一遍键（第二轮修复核对 C1：80 KB 的正文曾让解析分配 567 MB，1 MiB 的显式键压在一千项之上约 1 GB）。所以 YAML 的读取给路径的总字节数也设了上限：1 000 000 字节（值数上限的每个值一条约 100 字节的路径）与 YAML 字节数的两倍中较大者，超出 frontmatter 无效，与别名重复的上限同一种做法；两倍是因为别名重复的键在它下面各值的路径里还要再算一次。第二轮修复时下限是 200 000，第三轮核对 L3 指出它让 42 KB、两千多项、路径约 96 字节的映射无效，改为现在的下限。但路径也不超过 YAML 字节数的 64 倍（两字节写的每个值约 128 字节的路径）：第四轮核对 L1 指出，没有这一条时 4 KB 的 frontmatter 能带出约 1 MB 的路径，超过它取的 4 KiB 所记的 1.2 MB。合理的大 frontmatter 的路径约是 YAML 的 2–5 倍，不受影响。`Limit` 按实际的路径字节数计。
  - 小正文的切片按倍数增长，另有约 4 KB 的常数（第二轮核对 L2：1 KB 的满页 `[[a]]` 超出原来的上限 8%）。
  - 所以上限 `Facts.Limit(n)` 是 4 KiB，加 `FactsRatio`（30）倍正文，加 frontmatter 每个值 400 字节，加各字符串路径的字节数。`CheckCosts` 核对它：
    - 每个病态输入在 512 KB、16 KB 与 1 KB；
    - 放大类输入在 16 KB：新增长键压在列表之上的三种（超出路径的上限，只防爆炸；去掉上限由 YAML 的单元测试抓到），与一个路径约 185 KB、在上限之内的；
    - 最密的几种输入（`dense`：满页的 `[[a]]`、`![[a]]`、`[a](b)`、`#a`，转义的字符串，很密的 frontmatter 列表）从 256 字节到 8 KB，每步大 5%（第三轮核对 M1：只量一个点看不到容量翻倍之后的那一段）；
    - 每次读数是提取结果活着时的堆减去丢掉之后的堆，前后各两次 GC（`sync.Pool` 的对象不算进去），取三次的中位数（第三轮核对 L2：取最少的会往低偏）；余量 1 KB。`kept` 有自己的正向对照：一个每次解析留下 64 KB 的扩展读出 64 KB（第四轮核对 L3：提取结果在两次读数之间被误留活着时，读数都约为 0，检查会静默通过）。
    - 实测：正文类输入至多为上限的 0.74（1715 字节的满页 `[[a]]`），16 KB 与 512 KB 约 0.65；路径约 185 KB 的约 0.78。逐点记录的里最紧的一处还余约 13.8 KB，扫描的最小尺寸约余 6 KB。

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
- **`Facts` 仍在预算之内，只占一小份**（审查 M1、修复核对 M-1）：解析约占正文的 300 倍，`Facts` 至多 `Facts.Limit`，所以解析完之后只留 `Limit` 的三百分之一（向上取整）：正文字节的十分之一，加 frontmatter 每个值约 1.3 字节与路径字节数的三百分之一，再加 14 字节。
  - 但至多留下解析时取的那么多（`keep` 只放回）。所以**每次至少取 4 KiB**（`minTake`，按 300 倍约 1.2 MB，第三轮核对 L1）：别名把两三百字节的 frontmatter 展开到值数上限时，提取结果约 400 KB（至多约 480 KB），解析的峰值也不过约 455 KB，都在其内；路径至多是 YAML 的 64 倍，在按 300 倍记的之内。第二轮修复只把这一点写成每个请求的常数，第三轮核对指出 236 字节就能在等锁时占住约 400 KB，8 MiB 的预算能容纳三万多个这样的写，所以改为至少取。代价：8 MiB 的预算同时最多约 2000 次解析，阅读视图解析完即放回，写入在解析之后只留自己那一份。
  - 平台的 `Budget.Take` 答 `*Hold`：`KeepFacts(facts)` 放回解析多占的部分，`Release()` 放回全部；page 的端口 `ParseBudget.Take` 答 `BudgetHold`（同样两个方法，`facts` 是不透明的 `app.Facts`，适配器转换）。
  - `ContentParser.Parse`、`Decided` 答 `(Facts, release, error)`：取预算、`Facts`、`KeepFacts`，用例 `defer release()` 到单元结束；解析 panic 时全部放回。勾选任务项的第一次解析只为找任务项，用完即放回。
- 于是写入在单元里等锁时只占约十分之一：一页 5 MiB 的写等锁时占约 512 KiB。默认 8 MiB 的预算里，一个 5 MiB 页面的阅读视图要 5 MiB 空闲，7 个这样的写同时等锁就会让它 503；小页面的阅读视图要十几个。M4/P4 第 7 节与[M12 移交](../M12-release/handoffs/M4-performance.md)第 3 项的那条风险随之缓解，移交第 3 项改写为剩下的部分。
- `Parsed` 改名 `Facts`（`app.Facts`、`ContentWrite.Facts`、`PageDraft.Facts`、`domain.Change.Facts`），在 page 里仍不透明。P3 的观察者经组合根拿到的是 `markdown.Facts`。
- 观察者调用之后不得留着 `Facts` 的约定（总体设计 13.3 第 3 条）照旧：单元结束时预算放回，留着的 `Facts` 就不在预算之内了。

### 3.5 page：`Revision`

`markdown.Page` 与 `app.PageRef` 加 `Revision`；`GetPageView` 交出它读到的版本。P3 的 `Fetch` 用它判断索引是不是这个版本的（总设计 4.7"阅读视图的一致性"）。

### 3.6 组合根

- `bootstrap` 按配置建一个 `markdown.NewBudget(cfg.Page.ParseBudgetBytes, cfg.Page.ParseMaxWait, logger)`，交给 `page.Deps.Budget`；P3 起同一个交给 linking。
- `page.New` 用交来的预算，不自己建。

## 4. 实施步骤

| 步 | 内容 | 提交 |
|---|---|---|
| S1 | 平台：`Facts`、`Budget`（移来，`ErrBusy`）、`Page.Revision`；树不随 `Facts` 存活的测试 | `markdown: a parse's facts, the parse budget, a page's revision (M6/P2/S1)` |
| S2 | page 与组合根：`Facts` 代替 `Parsed`，提取之后放回解析多占的预算，`PageRef.Revision`，预算由组合根交来；总体设计与 M12 移交的修订 | `page, bootstrap: a write keeps its content's facts, the budget back at once (M6/P2/S2)` |

## 5. 测试与验证

- **平台**：
  - 树不随 `Facts` 存活（`markdowntest.CheckFacts`）：一个探针扩展在 `Extract` 里弱引用语法树的根（`weak.Pointer`），解析带属性链接的 frontmatter 加普通正文（以及调用方给的正文）、只留 `Facts`，`runtime.GC()` 之后根已被回收；Markdown 本身保持存活（扩展的状态留着树也算）；某个扩展的提取结果与空正文的相同（`reflect.DeepEqual`）时检查失败：它什么都没取到，检查就看不到它。`FactsError` 有自己的测试：结果里留着树、空的非 nil 切片、扩展的状态留着树、干净的扩展。平台不带扩展与带测试扩展各跑一次，obsidian、tasks 各在自己的包里跑一次，组合根以应用注册的扩展跑一次；
  - `Facts` 的大小：`CheckCosts` 对每个病态输入（加了满页的 `[[a]]`、`[a](b)`、`#a`、转义的引号字符串、很密的 frontmatter 列表）在 512 KB、16 KB 与 1 KB，对放大类输入（加了长键压在列表之上的）在 16 KB，对最密的几种从 256 字节到 8 KB 每步 5%，量 `Facts` 留下的堆（活着与丢掉之差，三次的中位数），不超过 `Facts.Limit` 加 1 KB；`Limit` 数 frontmatter 的值与路径的字节数；
  - YAML 字符串路径的上限：到上限有效，多一项无效；短的 YAML 至多 64 倍；YAML 更长时上限随之变大；不收的值（数字、别名重复的值）不计；值数上限之内、路径约 96 字节的映射有效；
  - `Facts` 与 `Document` 的 frontmatter、提取结果相同；
  - 预算的测试随代码移来（取、放、等待、饱和、`ErrBusy`、零与负的拒绝）；`KeepFacts` 留下 `Limit` 的三百分之一（向上取整），带 frontmatter 时多留它的值与路径的那一份；但至多留下取的那么多：比预算大的正文至多整个预算，取了 4 KiB 而那一份更大的（别名展开的 frontmatter）留下 4 KiB；`math.MaxInt` 字节的正文留下整个预算（`Limit` 饱和，不回绕，第二轮核对 L5）；别名的份额大于 4 KiB 时留下取的 4 KiB；负的大小什么都不取、不留、不放回；
  - 短正文至少取 4 KiB：三个一字节的正文占满 12 KiB 的预算，第四个答 `ErrBusy`；比 4 KiB 长的取自己的大小；
- **page**：
  - 调用的次序：`Take n`、`Facts`、`KeepFacts` 在单元之前，`Release` 在最后（写、建、勾选；勾选的第一次解析用完即放回）；
  - 预算取不到时 503 在解析与单元之前；解析 panic 时放回；只有写者让服务端解析（照旧）；
  - 适配器：`ErrBusy` 答 503 `server_busy`，`Retry-After` 1 秒；其他错误照旧；没有预算时组装即失败（审查 L2）；
  - 阅读视图把读到的版本交给渲染（`PageRef.Revision`），扩展的 `Fetch` 收到它（`TestAnExtensionReachesTheReadingView`）；
  - 模块用交来的预算：测试先占满一个预算交给 `page.New`，写与阅读视图都答 503，带 `Retry-After: 1`。
- **整个程序**（`TestTheParseBudgetBoundsWritesAndViews` 改写为 `TestAWriteWaitingForItsLockKeepsItsFactsShare`，预算 64 KiB）：A 的写在等页面的锁时，正文与预算一样大的，B 的阅读视图答 200（M4/P4 时答 503）；正文是预算十倍的，它留下的那一份占满预算，B 的阅读视图答 503 `server_busy`、`Retry-After: 1`，经契约核对；A 的写在锁放开之后都答 200。它同时钉住组合根用的是配置的预算。
- **反向对照**：`Facts` 留着 `Document`；扩展的状态留着树；`ContentParser` 整份持到单元结束，或不留 `Facts` 那一份；`GetPageView` 不交版本；适配器不换 `ErrBusy`；`page.New` 自己建预算；组合根交给 page 的不是配置的预算。每个都要有测试失败。
- `make check`、`make gen-check`、e2e 全量。

## 6. 完成标准

- 写入路径里没有语法树：`Change` 带的是 `Facts`，平台的测试证明树不随它存活。
- 预算在提取之后只留 `Facts` 那一份，由组合根建一个、交给 page；`Facts` 至多 `Facts.Limit`，由 `CheckCosts` 核对。
- `markdown.Page`、`app.PageRef` 带 `Revision`。
- M4、M5 的写入路径测试照旧通过（只改调用次序与名称）。
- 总体设计与 M12 移交的修订落档。

## 7. 结果

（完成后填写）
