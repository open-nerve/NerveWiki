# M6/P4 链接改写：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M6/P4 链接改写 |
| 状态 | 设计已定 |
| 基线 | P3 合并之后的 main（`7a68254`）；本文提交之后开分支 `m6-p4` |
| 上级文档 | [M6 总设计](00-M6-design.md) 4.6（改写）、4.7（预算）、第 5 节（`locks`）、第 10 节（500 页的测量）、第 11 节第 1 项；[总体设计](../v0.1-design.md) 4.5、13.1 第 21 条；[M5 的锁移交](handoffs/M5-locks.md)第 1、3 项；[P3 文档](03-P3-index.md)第 2 节（解析规则）、3.2（索引表）；样例集 [README](../../../tools/md-fixtures/README.md) 的 `rename/` |

---

## 0. 梳理

作者先请两位 Opus 只读梳理（2026-10-05）：

- 一位对照总设计 4.6 读代码：写入单元与参与者、linking 的观察者与索引、编辑锁、预算、契约与前端。
- 一位读 Obsidian 1.12.7 的代码（`app.fileManager.renameFile` → `runAsyncLinkUpdate` → `fileToLinktext`），并在隔离的实例里（自己的用户数据目录与库）跑了 33 个改名、移动的情形，比对改写前后的文件。

下面的规则据此定稿。与总设计 4.6、4.7 的出入在第 9 节。

## 1. 范围

P4 做：

- linking 注册为写入单元的参与者：改名、移动之后，改写需要改写的链接（第 2、3、4 节）。
- 被锁时整个拒绝：409 `linking.pages_locked`，problem 带 `locks`（平台、契约、前端）。
- 预算：不排队的取（`TakeNow`）。
- `rename/` 样例扩展到树与路径，`obsidian-verified` 的与 Obsidian 核对。
- 前端：改名、移动（对话框与拖动）被拒时列出页面与编辑者。
- e2e：L3（改名之后链接跟着改，被锁时被拒）与 L1 留下的后半（`l1-links.spec.ts` 开头的说明）。

不做：

- 接口（P5）。P5 的 `link` 字段与 P7 的补全用本 Phase 的写法函数（3.1）。
- `update_links=false` 的入口（M9）：参与者照 `Options.UpdateLinks` 什么也不做，锁的预检也跳过，只有单元测试。
- 标题改了之后的锚点改写（Obsidian 改标题时改 `#h`；v0.1 不做）。

## 2. 改写哪些链接

**之前与之后**：

- 之前是单元开始之前的索引：`page_links` 的 `resolved_id` 与 `ambiguous`。
- 之后是在单元的事务里，按改完的树重新解析，与观察者同一个函数（`resolutions()`）。参与者在观察者之前运行，这时索引还是之前的。
- 候选的链接与观察者的范围相同：解析到这棵子树的（入链）；从子树出发的（出链）；目标的键是新的名称或别名的键的（可能被抢走的）。

**一条链接改写，当且仅当它之前解析到一个节点 N，并且**：

1. 之后解析不到、解析到别的节点，或者之后有歧义而之前没有；或者
2. 这次是只改大小写的改名（标题键不变）：链接按标题（不是别名）指向这一页，写的最后一段与新标题不逐字相同（`[[Old]]` → `[[old]]`，Obsidian 也改，样例 021）。

改写成仍指向 N、没有歧义的写法（第 3 节）。原则同总体设计 4.5：改名、移动不改变任何一条原来解析到的链接的指向。

**不改写**：

- 之前解析不到的：之后解析到了也不改（Obsidian 一致）。
- 只有锚点的（`[[#h]]`、`[t](#h)`）。
- 之后仍解析到 N、没有新的歧义的。Obsidian 在候选列表变了时也改写：`C/y` 改名为 `C/x` 之后，`A/s` 里仍指向 `A/x` 的 `[[x]]` 被改成 `[[A/x]]`；移动文件夹时把仍能解析的 `[[F/sub/y]]` 规整为 `[[y]]`。不跟（nerve-defined，样例 020、022）：多改一页就多一分撞上锁的可能，也多一条修订。
- `aliases` 的值里的链接（`aliases: ["[[Old]]"]`）：它是这一页的别名，改写它会改变别的链接解析到哪里。Obsidian 照改（nerve-defined，样例 028）。
- 出发页在索引里没有，或索引过时（`indexed_pages` 的 `revision` 或 `extractor` 与当前不同：升级之后、`reindex` 之前）：没有"之前"，不改写，记一条日志。README 的升级一节写明升级之后先 `reindex`。

之前有歧义的链接，照之前解析到的那一个算（Obsidian 也保住它首先解析到的，样例 017）。

**在哪里的链接**：代码里的不是链接，不改；`%%` 注释里的照改（Obsidian 一致）；引用式链接改它的定义（样例 002；Obsidian 不索引定义，不改，nerve-defined）；嵌入同 wikilink；图片同 Markdown 链接；frontmatter 里的属性链接照改（3.3）。

## 3. 写法

### 3.1 wikilink 与嵌入（照 Obsidian 的 `fileToLinktext`）

- **名称或完整路径**：只凭名称就只有 N 一个（标题键在笔记本里唯一，或 N 在根下：P3 规则第 2 步先找根下的）时写名称，否则写从根起的完整路径，不带开头的 `/`（P3 规则第 2 步总能唯一解析到它）。
  - 与出发页在哪里无关：出发页之后移动也不坏。P5 的 `link` 与 P7 的补全用同一个函数。
  - 不再"依次试更长的路径后缀"（总设计 4.6 原文）。Obsidian 不写部分后缀（样例 015：`Docs/Old` 改名为 `Docs/Plan`、另有 `Work/Plan` 时写 `[[Docs/Plan]]`，哪怕从 `Docs/s` 出发 `[[Plan]]` 也能解析到）。
- **原来的写法不保留**：`[[./y]]`、`[[/Old]]`、`[[A/x]]`、`[[Old.md]]` 都写成上面的形式（Obsidian 一致，样例 005、010）。
- **核对**：写出之后用解析器从出发页核对一次。不成立时（以 `.md` 结尾的标题遇上"只读作一种"）写完整路径加 `.md`；仍不成立就不改这一条，记日志。模糊测试守住这一支不发生。
- **只换目标那一段**（`Range`）：锚点、`|`、`\|`、显示文字、目标两边的空格照原样。Obsidian 重写整个链接，去掉两边的空格（样例 001，nerve-defined）。
- **显示文字跟着改**（Obsidian 的规则，样例 006）：原来的目标带 `/`、显示文字去掉两边空白之后与目标的最后一段逐字相同时，显示文字换成 N 的标题。`[[F/Deep|Deep]]` → `[[Deeper|Deeper]]`；`[[Old|Old]]` 的目标不带 `/`，只换目标：`[[New|Old]]`。
- **经别名解析到的**改写时，没有显示文字的加上原来写的那个别名：`[[x]]`（`P` 的别名）在 `Q` 改名为 `x` 之后写成 `[[P|x]]`。Obsidian 不解析别名，不改（nerve-defined，样例 023）。

### 3.2 Markdown 链接与图片

- **不带 `./`、`../`、`/` 的**：同 wikilink 的写法，加 `.md`（`[t](Old)` → `[t](New.md)`，Obsidian 一致，样例 007、009）。P3 规则里它们与 wikilink 同样解析。
- **`./`、`../` 开头的**：保持相对，从出发文件夹重新算：N 在出发文件夹里或其下时以 `./` 开头，否则以 `../` 开头（样例 011）。Obsidian 改用它的设置，默认写成不带 `./` 的；不带 `./` 的在我们这里按名称与后缀解析，换了含义（nerve-defined）。
- **`/` 开头的**：`/` 加完整路径与 `.md`（nerve-defined，样例 008）。
- **编码**照样例 002：普通的写法把空格、`%`、`(`、`)` 写成百分号编码，其余字符（含中文）原样；尖括号的原样写。锚点、标题（`"title"`）、地址两边的空白照原样。Obsidian 不编码 `(`、`)`、`%`，改名为 `Close) 50%` 时链接坏了（nerve-defined，样例 026）。
- **链接文字跟着改**（Obsidian 的规则，样例 007）：文字是纯文本（原样的字节就是它的文字，没有转义与标记），去掉两边空白之后等于 N 原来的标题，或（带 `/` 时）等于 N 原来的完整路径时，换成 N 的新标题：`[Old](Old.md)` → `[New](New.md)`。引用式链接的文字不改。

### 3.3 frontmatter 里的属性链接

- 只换目标那一段（与正文同样的写法）；显示文字跟着改时同样。
- 值在单引号里时 `'` 写成 `''`（样例 004）。双引号里不用转义：标题里没有 `"` 与 `\`，显示文字与锚点不动。不带引号的标量不会是链接（`[` 开头是 YAML 的序列）。
- 其余字节都不动：注释、流式列表、`010`、CRLF。Obsidian 重写整个 frontmatter：去掉注释、改引号、`010` 变 `10`、行内的 CRLF 变 LF（nerve-defined，样例 003、027）。
- 属性链接与正文链接按范围是否在 frontmatter 里区分（P3 文档 3.2：键为空串的属性链接在索引里与正文的一样记为空）。

### 3.4 字节

- 同一页的几处从后往前替换，合成一次追加的写。其余字节逐字节不变：CRLF、缩进、行尾空白。
- 替换之后的正文重新解析，提取结果随追加的写交给守卫与观察者。

## 4. 参与者（`linking`）

### 4.1 流程

`Participate(ctx, s, u)`：

1. 只看改名与移动，且 `s.Options.UpdateLinks`；同一父节点里的重排什么也不做。
2. **范围**与观察者相同：把 `app/index.go` 里算范围的那段抽出来共用。
3. **之前**：一条语句读出范围里的链接行（出发页、范围、解析到的、有无歧义），与各出发页的 `indexed_pages`（`revision`、`extractor`）。
4. **之后**：在事务里照 `resolutions()` 重新解析这些链接。
5. 照第 2 节算出要改写的链接与它们的页；没有就结束。
6. **锁的预检**：一条语句查出这些页上活着的编辑会话（`AliveSessionsOf`，本人的也算）。有就答 409 `linking.pages_locked`，`locks` 列出每一页与编辑者（按页 id 排序）。
7. **逐页**（按页 id 升序，`unit_content.go` 的约定）：
   1. `TakeNow(len(正文))`：取不到答 503 `server_busy`，不排队（4.2）。
   2. 读正文，解析、提取，按范围的起点对上索引行（`revision` 与 `extractor` 相同，范围相同）。
   3. 算出新的文字（第 3 节），从后往前替换。
   4. 放掉这次的取，为新的正文再 `TakeNow` 一次，解析、提取，`KeepFacts`。
   5. `u.WriteContent`：以当前的 `revision` 为 base，带新的提取结果。
   6. 这一页的取在单元结束时放掉（`Appender.Defer`，第 5 节）：观察者要用提取结果。
8. **守卫在追加的写上答 `page.locked`**：预检之后，一个读时钟早于会话到期、提交晚于预检的心跳，会把预检看作已过期的会话续活（总设计 4.6 第 4 项）。组合根的适配器把它转成 linking 的哨兵错误，参与者重查一次会话，答 `linking.pages_locked`。

### 4.2 次序与锁

- 参与者在改名、移动的操作之后、同一事务里运行，持着笔记本行的 `FOR NO KEY UPDATE`：别的正文写、开启、接管、强制解锁都排在后面。预检与追加的写之间只剩心跳的窗口（4.1 第 8 步）。
- 心跳在追加的写之后提交时，那位编辑者的会话续活在改写过的正文上，下一次保存答 409 `page.revision_mismatch`。这与 M5 里普通的写的窗口相同，接受，写在代码的注释里。
- 不取索引的咨询锁：笔记本行已经挡住了这个笔记本的其他单元。先取咨询锁会与内容行的加锁次序倒置（13.1 第 5 条）；观察者在单元结束时照旧取。
- 持着笔记本行不排队等预算：正文写先取预算、再等笔记本行，改名持着笔记本行再等预算，就倒置了（总设计 4.7）。

### 4.3 错误的次序

改名、移动自己的 422、409 在前（操作里），然后是参与者：409 `linking.pages_locked`（预检）在 503 `server_busy`（预算）之前。`renameNode`、`moveNode` 声明这两个码。

### 4.4 修订与事件

追加的写与普通的正文写一样：每页一条修订，作者是改名、移动的人，客户端字段同这个请求；事件流照常（正文变了的页、`links`）。被改写的页正开着阅读视图的，经事件重读（P3）。

### 4.5 一个单元里多个改名、移动

之前是单元开始之前的索引：第二个改名、移动看到的之前不准（第一个追加的写还没进索引）。v0.1 没有这样的单元，网页一次一个操作。移交 M9 的批量（总设计第 7 节"给 M9"已写"批量里的改名、移动要求单元开始前的索引是新的"，这里加上这一条）。

## 5. 平台、页面模块与契约

**平台**：

- `markdown.Budget.TakeNow(n)`：可用就取，不可用立即 `ErrBusy`，不排队；`minTake` 照旧。
  - 总设计 4.7 原定一次取够：最大一页，加各页提取结果那一份的上限。那个上限按 YAML 的最大值算，每页至少约 13.7 KB，默认 8 MiB 时六百来页的改名永远取不到。
  - 逐页取，解析之后照 `KeepFacts` 只留实际的那一份（没有 frontmatter 的页约是正文的十分之一），同样不排队。平台不再需要给那个上限，总设计 4.7 随之修订。
- `shared.Error` 加 `Locks []LockHolder{PageID, UserID, DisplayName}`；平台的可选接口 `problemLocks` 与 `Problem.Locks`；契约测试照 `lock` 的三处（`shared`、`httpserver` 的 `apierrors` 与 `contract`）。

**linking**：

- 领域错误 `PagesLocked`（码 `linking.pages_locked`，带 `Locks`）。
- 预算的适配器把 `ErrBusy` 转成 `server_busy`，照页面的 `adapter/markdown/budget.go`。现在 linking 的 `Parser` 返回原样的 `ErrBusy`（reindex 用，命令行打印它），参与者的这一支另走转换。

**页面模块**：

- `Appender` 加 `Defer(func())`：单元结束、观察者之后调用，成功、失败与回滚都调用。
- 模块根公开 `ContentWrite` 的别名与编辑者的查询：`page.NewLockHolders(pool, names)`，按 `AliveSessionsOf` 与显示名。
- 守卫的 `page.locked` 由组合根的适配器转成 linking 的哨兵（linking 不认页面的码）。

**组合**：

- 参与者要 Markdown 实例与预算，不只凭连接池：13.1 第 21 条的又一个例外。
- `pageRegistrants(pool)` 分成两个：只凭连接池的会话注册者（笔记本删除也从这里取，同一组订阅者经两处接线的约定不变），与写入的注册者 `pageWriteRegistrants(pool, md, budget)`（守卫、观察者、参与者）。13.1 第 21 条随之修订。

**契约**：

- `api/common.yaml` 的 `Problem` 加 `locks`。
- `api/modules/page.yaml` 的 `renameNode`、`moveNode` 加 `linking.pages_locked` 与 `server_busy`，描述写明会改写别的页、被锁时整个拒绝。
- `make gen`。页面的 HTTP 测试答出这两个码（`apitest` 按描述核对）。前端的文案表同一个提交加上（vitest 按契约核对每个操作的码）。

## 6. 前端

- 改名、移动的对话框与拖动的提示：409 `linking.pages_locked` 时列出页面（标题取自树）与编辑者（本人写"你"），说明改名需要改写这些页里的链接：等他们结束编辑，或由管理员强制解锁。
- `useForm` 的横幅加 `explain`（React 节点），照删除对话框的 `lockedText`。
- 503 `server_busy` 照通用的文案。
- 成功之后不需要新的处理：被改写的页经正文事件与 `links` 事件重读（P3）。

## 7. 样例

### 7.1 格式

`rename/NNN-名.json`：

```json
{
  "description": "…",
  "source": "obsidian-verified",
  "pages": ["top", "A/x", "A/y", "s", "B/C"],
  "page": "s",
  "from": "A/x",
  "to": "B/C/x"
}
```

- `pages` 照 `resolve/` 的写法；没有时只有 `from` 与 `page`，都在根下（001–004 照旧）。
- `page` 是 `.md` 所在的页，默认 `src`。一个样例一页正文；同一棵树的另一页另起一个样例。
- `from`、`to` 是路径：父节点相同是改名，名称相同是移动，两者都变的不合，`check.mjs` 拒绝。
- `.md` 是之前，`.out.md` 是之后。
- `check.mjs` 核对格式；`verify.mjs` 对 `obsidian-verified` 的在隔离的 Obsidian 里改名或移动并比对。文件夹页的改名在 Obsidian 里是两次改名：`F.md` 与 `F/`。
- Go 测试：`markdowntest` 加读 `rename/` 的加载器，逐个经 linking 的改写核对。

### 7.2 新加的样例

`V` 是 Obsidian 1.12.7 里跑过的（照原样核对），`N` 是 nerve-defined（Obsidian 的结果写在样例的说明里）。

| # | 名 | 树与操作 | 要点 | 来源 |
|---|---|---|---|---|
| 005 | wikilink-forms-2 | `Old` → `New` | `[[Old.md]]`、`[[/Old]]`、`[[./Old]]`、`[[Old#^b]]`、`![[Old#h\|300]]`、`[[Old\|Old]]`、`[[old]]`、表格里的 `[[Old\|t]]` | V |
| 006 | wikilink-display-sync | `F/Deep` → `F/Deeper` | `[[F/Deep\|Deep]]` → `[[Deeper\|Deeper]]`；`\|other` 不改；带锚点的不改；`[[f/deep\|deep]]` | V |
| 007 | markdown-forms | `Old` → `New` | `[Old](Old.md)`、`[t](Old)`、尖括号、标题、地址两边的空白、`#My%20H`、图片 | V |
| 008 | markdown-rooted-relative | `Old` → `New` | `[t](./Old.md)`、`[t](/Old.md)` 保持 | N |
| 009 | move-inbound | `A/x` → `B/C/x`，页 `s` | `[[A/x]]`、`[t](A/x.md)`、`[[A/x\|x]]`、`[t](<A/x.md> "ti")` → 名称 | V |
| 010 | move-outbound-wikilinks | 同上，页 `A/x` | `[[./y]]` → `[[y]]`、`[[../top]]` → `[[top]]`；`[[A/y]]`、`[[x#h]]`、`[[#h]]` 不改 | V |
| 011 | move-markdown-relative | 同上，页 `A/x` 与 `A/s2` | `[t](./y.md)` → `[t](../../A/y.md)`；`[t](/top.md)` 不改；`A/s2` 的 `[t](./x.md)` → `[t](../B/C/x.md)` | N |
| 012 | steal-shorter-path | `Long/x`、`C/y`；`C/y` → `C/x` | `[[x]]` → `[[Long/x]]` | V |
| 013 | steal-root-exact | `A/x`、`y`；`y` → `x` | `[[x]]` → `[[A/x]]` | V |
| 014 | steal-subtree-folder-move | `B/xx/x`、`Z/x`、`A/s`；`B/xx` → `A/xx` | `A/s` 的 `[[x]]` → `[[Z/x]]` | V |
| 015 | form-not-unique | `Docs/Old`、`Work/Plan`、`Docs/s`；`Docs/Old` → `Docs/Plan` | `[[Old]]` → `[[Docs/Plan]]`（另一个样例：`[[Deep/Old]]` → `[[Long/Deep/Plan]]`） | V |
| 016 | root-bare-name | `Old`、`A/Plan`；`Old` → `Plan` | `[[Old]]` → `[[Plan]]` | V |
| 017 | ambiguous-original-kept | `A/x`、`B/x`；`A/x` → `A/z` | 之前有歧义的 `[[x]]` 照之前解析到的 | V |
| 018 | move-duplicate | `A/x`、`B/x`、`C`；`A/x` → `C/x` | `[[A/x]]` → `[[C/x]]`；`[[B/x]]` 不改 | V |
| 019 | folder-page-inbound | `F`（有正文）、`F/x`、`F/sub/y`；`F` → `G` | `[[F/x]]` → `[[x]]`、`[[F]]` → `[[G]]`、`[t](F/sub/y.md)` → `[t](y.md)` | V（两次改名） |
| 020 | folder-move-kept | `F` → `X/F` | 仍能解析的 `[[F/sub/y]]` 不改（Obsidian 改成 `[[y]]`） | N |
| 021 | case-only | `Old` → `old` | `[[Old]]`、`[[OLD\|t]]`、`[t](Old.md)` → `old` | V |
| 022 | ambiguity-without-steal | `A/x`、`A/s`、`C/y`；`C/y` → `C/x` | `A/s` 的 `[[x]]` 不改（Obsidian 改成 `[[A/x]]`） | N |
| 023 | alias-stolen | `P`（别名 `x`）、`Q`；`Q` → `x` | `[[x]]` → `[[P\|x]]` | N |
| 024 | folder-only-page | `F`（没有正文）、`F/x`；`F` → `G` | `[[F]]` → `[[G]]`（Obsidian 里没有 `F.md`，不改） | N |
| 025 | whole-segment | `A/B/item`、`C/y`；`C/y` → `XB/item` | `[[B/item]]` 不改（Obsidian 按字符串比较，改成 `[[A/B/item]]`） | N |
| 026 | markdown-encoding | `Old` → `Close) 50%` | `[t](Close%29%2050%25.md)`；跨两行的链接文字保留 | N |
| 027 | frontmatter-forms | `Old` → `New` | 流式与块式列表、注释、`n: 010`、单引号、属性里的 Markdown 链接、CRLF：只换范围 | N |
| 028 | aliases-value | `Old` → `New` | `aliases: ["[[Old]]"]` 不改；`related: "[[Old]]"` 改 | N |

001–004 照旧是 N，说明里补上 Obsidian 的结果（001 去掉目标两边的空格；002 不编码 `(`、`)`，不改引用定义；003、004 重写 frontmatter）。

## 8. 测试

**领域**（`linking/domain`）：

- 写法的表格测试：名称与完整路径、`.md` 的核对、显示文字的两条规则、Markdown 的三种写法与编码、YAML 的单引号。
- `rename/` 样例的 Go 测试。
- 改写的模糊测试：随机的多层树（重名、同名不同层、别名）、随机的链接（各种写法、属性里的）、随机的改名或移动。不变量：
  - 之前解析到的每一条，之后仍解析到同一个节点，没有新的歧义（第 2 节不改写的几类除外）；
  - 之前解析不到的，逐字节不变；
  - 范围之外的字节不变。

**平台**：`TakeNow`（可用、不可用、`minTake`）；`Problem.Locks` 的契约。

**页面模块**：`Defer` 在成功、失败、回滚时都调用，在观察者之后；参与者的次序测试照旧。

**整个程序**（`bootstrap`）：

- 改名、移动之后链接跟着改：入链、出链、被抢走的。每个测试结束时 `checkLinks`，"增量等于重建"也覆盖改写。
- 引用它的页被别人锁着、被本人锁着：409，`locks` 列全，什么都没改（树也没改）。
- 移动自己正在编辑、带相对链接的页：409；不带需要改写的链接时照常（`c4-guard` 的情形）。改名的那一页自己正被编辑、正文不需要改写：照常。
- 预算不够：503，什么都没改。
- 最后一跳：组合根交空的参与者时，这些测试失败。

**交错**（照 `interleaveWith` 等），每个结束时 `checkLinks`：

- 改名与正文写：正文写先拿到笔记本行，或后拿到；
- 两次改名；
- 改名与开启编辑；
- 改名与心跳：要两个时钟，照 M5 的第 49 号（页面模块里），或在 `bootstrap` 里建两份页面模块。

**测量**：一次改名改写 500 页（每页 10 KB、各一条链接）的耗时与 Go 堆的峰值，写进第 12 节（总设计第 10 节）。

**前端**（vitest）：对话框与拖动的 `locks` 列表（别人、本人）、503。

**e2e**：

- L3：改名之后，另一页的链接跟着改（阅读视图里点它到新页）；另一个会话编辑引用它的页时改名被拒，列出那一页与编辑者。
- L1 的后半：`l1-links.spec.ts` 开头留给 P4 的那一步。

## 9. 与总设计的出入

（本文定稿，总设计与总体设计随 P4 的合并修订。）

- **4.6 第 1 项**：判定写准（第 2 节）：之后解析到别处、解析不到，或新有歧义；加上只改大小写的改名；`aliases` 的值与索引过时的出发页不改写。
- **4.6 第 2 项**：
  - wikilink 照 Obsidian 写名称或完整路径，不再依次试后缀，原来的写法不保留；
  - Markdown 链接，普通的同 wikilink，`./`、`../`、`/` 的保持；
  - 显示文字与链接文字的跟随规则；经别名的加显示文字。
- **4.7**：预算逐页不排队地取，不再一次取够；平台不需要给"提取结果那一份的上限"。
- **总体设计 4.5 第 1 条**："最短的、没有歧义的形式"写明为 3.1 的规则。
- **总体设计 13.1 第 21 条**：参与者的例外与 `pageRegistrants` 的拆分。
- **P3 文档 3.1**：linking 模块根的 `New` 留给"P4 的接口"，接口在 P5；P4 的 409 由页面的适配器答出。

**负责人可以改判**（作者的判断）：

- 只在指向变了或新有歧义时改写，不跟 Obsidian 规整仍能解析的（第 2 节）。
- wikilink 的写法跟 Obsidian；Markdown 链接的相对与根路径保持（3.1、3.2）。
- `aliases` 的值不改写；索引过时的出发页跳过（第 2 节）。
- 本人的锁同样拒绝（总设计第 11 节已写）。
- 预算取不到立即 503：超过整个预算的改名一直 503。默认 8 MiB 能容纳多少页，第 12 节的测量写明。

## 10. 实施步骤

| 步 | 内容 |
|---|---|
| S1 | 样例：格式扩展、005–028、`check.mjs`、`verify.mjs` 的改名（与 Obsidian 核对）、README |
| S2 | `linking/domain`：写法、改写的判定、替换；表格测试、样例的 Go 测试、模糊测试 |
| S3 | 平台与契约：`TakeNow`；`Error.Locks`、`Problem.locks`；`linking.pages_locked`；`page.yaml` 的码；`make gen`；页面的 HTTP 测试；前端的文案表（同一个提交） |
| S4 | 页面模块：`Appender.Defer`；`ContentWrite`、编辑者的查询；组合的拆分 |
| S5 | 参与者：共用的范围、之前与之后、锁的预检、预算、解析、替换、追加；`page.locked` 的转换；接线与最后一跳；整个程序的测试 |
| S6 | 交错；"增量等于重建"加上改写；500 页的测量 |
| S7 | 前端与 e2e |

## 11. 完成标准

- 第 8 节的测试通过；`rename/` 的 `obsidian-verified` 样例与 Obsidian 1.12.7 一致。
- `make check`、`make gen-check`、e2e 全量通过。
- 审查的发现处理完，修复经 Opus 核对，直到一轮没有行为上的发现；`make image-smoke` 通过。

## 12. 结果

（合并之后填写。）
