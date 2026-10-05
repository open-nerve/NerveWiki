# M6/P3 A 部分（索引与解析）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m6-p3a`（`91a9f97..c455784`，合并 `b802987`：S1 `299c883` + `02784b9`、S2 `95d2a02`、S3 `31cb5c1` + `6177e46`、S4 `6d52e3c`、S5 `c455784`），对照 [03-P3-index.md](../03-P3-index.md) 第 0–5、7 节，[M6 总设计](../00-M6-design.md) 4.3–4.5、4.8、4.10 |
| 审查方式 | 三位审查者（Opus）并行、只读，各自在临时目录复制仓库做实验：解析规则、样例与提取结果（r1，从 Obsidian 1.12.7 的安装包里只读地取出 `MetadataCache.getLinkpathDest`，在 Node 里原样运行，与全部样例一致之后再造分歧的树）；增量维护、存储与并发（r2，3,000 个种子的替身上的随机多写入单元、40 个种子 × 80 步的整个程序的性质测试、`EXPLAIN ANALYZE`）；reindex、测试与文档（r3，一万页、二十万条链接的笔记本，10 个变体检验性质测试，交错的变体）。修复之后由 Opus 核对，直到一轮没有行为上的发现 |
| 日期 | 2026-10-05 |
| 结论 | High 3（其中一条三位都发现）、Medium 6、Low 13，都已处理（两条记为不改，写明理由）。修复的核对两轮：第一轮 Low 7（其一改了行为：文件夹自己的页算在子树里），第二轮没有行为上的发现，Low 3（文字）。都已处理 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| H1 | High | **U+0000 让保存答 500，reindex 停在这一页**（三位都发现）：Markdown 链接的 `%00` 按 `decodeURI` 解成 U+0000，YAML 双引号字符串的 `\0`、`\x00`、`\u0000` 同样；PostgreSQL 的 `text` 不收 0x00，`jsonb` 不收 `\u0000`。这样的正文在 P3 之前能保存；升级之后的 `nervewiki reindex` 遇到它整个停下，后面的笔记本都不重建 | 已修：`PageFacts` 把每个事实（链接的目标、锚点、显示文字、属性路径，标签，别名，属性的键与值，嵌套的键）里的 U+0000 记作 U+FFFD。reindex 遇到失败的笔记本记下原因、继续其余的，最后以 1 退出。nul-kept、nul-json-kept、nul-json-key-kept、nul-prop-key-kept、nul-link-display、nul-link-anchor、nul-link-property、nul-kept-db、reindex-stops 失败 |
| H2 | High | **过长的键让保存答 500**：约 2,700 字节以上的链接目标、别名或标签，标题键超过 B-tree 一项的上限（`page_links_notebook_id_target_key_idx`、`page_aliases_pkey`、`page_tags_pkey`） | 已修：`domain.MaxKey`（1024 字节）。255 字节的标题，键至多 508 字节（逐个字符量过，`TestNoTitlesKeyComesNearMaxKey`），所以更长的键不是任何页的：这样的链接没有键、解析不到，这样的别名、标签不记。maxkey-links、maxkey-links-db、maxkey-tags、maxkey-aliases、maxkey-small 失败 |
| H3 | High | **两条标为与 Obsidian 核对过的规则其实不是 Obsidian 的**（r1 H2）：Obsidian 的后缀匹配按导出路径的字符串长度（`path.length`）排序，不按层数；以 `.md` 结尾的目标在整个库里只读作一种（有那个文件名的文件就是它，没有才加 `.md`），不是每一步两种都试。原来的样例里层数与长度恰好一致、两种写法从不相争，`verify-resolve.mjs` 核不出来 | 已改为 Obsidian 的：第 3 步并列时选路径短的（UTF-16 码元，`é` 一个、`😀` 两个，与层数无关）；`.md` 先定一种写法。样例 007（改名为"then the shortest path"）、010 加了能分辨两种规则的链接，与真实的 Obsidian 1.12.7 再次核对一致。路径长度看名称，所以改名到同一个键而长度不同的（`Straße` → `STRASSE`）也算改名，它的子树重新解析。levels-not-length、bytes-not-utf16、form-per-step、form-never-stem、rename-by-key 失败 |
| M1 | Medium | `.md` 按大小写匹配：`[[Note.MD]]` 解析不到，Obsidian 把整个链接转成小写 | 已修：不分大小写。md-case 失败 |
| M2 | Medium | 另有三处 Obsidian 按字符串比较：出发文件夹的子树用 `startsWith`（`YZ/Q/dup` 算在 `Y` 里），后缀用 `endsWith`（`XA/note` 以 `A/note` 结尾），相对路径找不到时再按后缀找 | 保留按整段的规则，写进新样例 015（`nerve-defined`），说明这些是字符串比较的副作用，Obsidian 生成的链接不依赖它们 |
| M3 | Medium | **两组都多于 20 时不发 `links` 事件**（r2 M1）：先截断成 `null`，再按"都空"判断。改名一个有 21 页链进去的文件夹，客户端收不到任何事件 | 已修：按截断之前的两组判断。event-after-truncation 失败 |
| M4 | Medium | **只写正文的单元重新解析这一页的全部反链**（r2 M2）：`Affected` 把写了正文的页的键与 id 都算进去，每次自动保存都在索引的锁里重解析；5,000 条反链约 20 ms（缩小之后 2–3 ms），500 个 README 页时 110 ms | 已修：不移动任何节点的单元只到它写的页自己的链接，与它增删的别名的键（对称差）；有移动的单元照旧全部。content-full-reach、content-no-sources、aliases-all-keys、aliases-no-new 失败 |
| M5 | Medium | **reindex 的锁顺序没有测试**（r3 M1）：把"笔记本行、再索引的锁"反过来，全部测试照过，实际会死锁（审查者写了交错测试，反过来时 `40P01`） | 已加交错测试：正文写持笔记本行的 `FOR SHARE`、等在正文行上，reindex 排在它后面。rebuild-lock-order 失败（死锁） |
| M6 | Medium | **reindex 慢，期间的写入会 500**（r3 M2）：一万页、二十万条链接要 16–17 秒，其中每页 5 条在 `DeleteNotebooks` 之后什么也不删的 DELETE；期间的保存在请求期限后答 500；README 只说"在等待" | 已修：重建只插入（`Store.AddPage`，审查者实测 17.4 → 11.0 秒）；README 写明在没人写的时候、最好在 `migrate up` 之后启动服务之前执行，大笔记本等不到的保存答 500，并给出 Docker 的写法 |
| L1 | Low | 别名一步把两种写法混在一起 | 已修：先按去掉 `.md` 的别名，再按原样的。alias-written-first 失败 |
| L2 | Low | frontmatter 的标签按正文标签的规则 9 判断，文档却说"照标签面板"；面板的规则不同（去掉结尾的 `/`，拒绝空白、通用与补充标点和大部分 ASCII 标点、全是数字的），`a😀`、`a→b` 面板计入 | 已改为标签面板的（`obsidian.CountedTag`，从 Obsidian 的代码读出），正文的标签也照它去掉结尾的 `/`。tag-slash-kept、tag-no-punct-blocks、tag-nel-space、tag-bom-not-space、tag-ascii-punct、body-tags-raw 失败 |
| L3 | Low | `check.mjs` 对 `resolve/` 不严：多余或拼错的键、只差大小写的兄弟、服务端不收的标题都通过 | 已修：逐个核对键，拒绝撞键的兄弟（近似标题键）与不合法的标题；探针确认 |
| L4 | Low | `verify-resolve.mjs` 对 `to: null` 的链接可能空过：只等缓存、固定 1 秒，没提取到的链接也读作"解析不到"；`prepare` 会清空任何给它的目录 | 已修：等每个文件都有了 `resolvedLinks` 与 `unresolvedLinks`，每个文件要恰好一条链接；只清空它自己准备过的目录或空目录 |
| L5 | Low | `SetNameKeys` 一条语句改多行，兄弟之间一个键接过另一个的旧键时，唯一索引逐行检查会撞上（r2 L1、r3 L7） | 已修：两条语句，先改成各自独有的临时键（`chr(1) || id`），再改成新键。rekey-one-step 失败 |
| L6 | Low | YAML 键为空串的属性链接，`property_key` 与正文链接一样为空（r2 L2） | 不改：P4 的改写按链接的范围是否在 frontmatter 里区分（它本来就在单元里重新解析要改写的页），写进 P3 文档 3.2 |
| L7 | Low | `--notebook ""`、`--notebook=` 静默重建全部 | 已修：给了这个选项就要是 id。notebook-empty-ok 失败 |
| L8 | Low | 性质测试偏弱（r3 用 10 个变体：3 个种子时 5 个存活） | 已改：6 个种子，生成器更多根下的页、加了只差长度的标题；另收入 r2 写的 app 层随机多写入单元的测试（300 个种子 × 40 步，约 1 秒），它抓住 rename-by-key-random、aliases-no-new-random、aliaskeys-dropped-random |
| L9 | Low | 命令的撞键测试是空的（那个笔记本从未建过索引） | 已改：先建索引，再撞键并制造一个失败的笔记本；核对两者原样、不发事件 |
| L10 | Low | 事件契约的 `null` 没写 reindex 的情形 | 已改 `events.yaml`（"多于 20 或全部"） |
| L11 | Low（文字） | P3 文档第 5 节与代码不符（交错的等待函数、`checkLinks` 在哪里跑、它核对什么）；交错测试的说明言过其实；3.7 的样例位置与示例 | 已改 |
| L12 | Low | 递归的路径查询穿过已删的祖先 | 已改：遇到已删的节点就停，到不了根是错误。cte-through-deleted 失败 |
| L13 | Low | `--notebook` 之外，一个笔记本失败就停下整个命令（r1、r2、r3 都提到） | 已修，见 H1 |
| Q | 质量 | 解析样例的测试只用方言的扩展；`treeOf` 对没列出的父页、不存在的 `to` 不报错；`PageFacts` 在没有 obsidian 的提取结果时静默；`Resolve` 每个候选分配一次；`linkTargets` 的方法分在两处；"1 pages"；`checkLinks` 不核对提取规则的版本 | 已改。no-extraction-ok、names-not-carried 失败 |
| Q | 质量（不改） | 按笔记本限定删除语句（ids 都来自单元自己的笔记本）；孤立的行让写入失败而不是忽略（我们宁可大声失败）；`LinksOf(notebook)` 代替全部 id；`NewAdmin` 与 `NewIndex` 相似、`Pages` 与 `Contents` 是同一个值（两个端口，一个适配器） | 不改，理由如左 |
| — | 移交 | 链接很多的页每次自动保存都换掉、重解析它的全部行（5,000 条约 70 ms，留下死元组）；嵌套属性的键序在 `jsonb` 里丢失 | 写进 M12 的性能移交与 P5（属性接口）的注意事项 |

审查者核过并认为成立的：解析的次序、`.md` 之外的各步、子树按 id、对输入次序无关（18 万次打乱）、附件与已删的页不会被解析到；候选完整性的论证（r2 在替身上 3,000 个种子 × 40 步、每单元至多 3 个写，1,500 × 80、至多 5 个写，整个程序上 40 个种子 × 80 步，都没有找到反例）；观察者的各步在锁之后、单元自己的写之后读；锁总是单元最后取的，与 reindex、笔记本删除、`NOTIFY` 都不成环，`(int4, int4)` 的锁与 goose、River 的不冲突；查询计划（`LinksReached` 用上四个索引）；迁移、授权与不带外键的理由；`PageFacts` 对 frontmatter 的读法与 Obsidian 的代码一致；reindex 的事务、计数、跳过列出之后被删的笔记本、运行用户的权限；`WaitForAdvisoryLockWaits` 对负的键也对；三个交错测试各跑 25 次没有失败。

反向对照：修复（`573ee6f`）有 43 个变体，全部让测试失败（其中 4 个第一次写成了编译不过、1 个起初存活：属性路径里的 U+0000 没有测试，补了之后失败）。

## 修复的核对

修复（`573ee6f`）之后由一位 Opus 只读核对：约 13,000 棵随机的树、96,000 条链接（含 `x.md`、`X.MD`、`x.md.md`、`.md`、emoji、组合字符、`İ`），nerve 的 `Resolve` 与 Obsidian 自己的 `getLinkpathDest`（三处字符串比较换成按整段）逐条一致；`CountedTag` 与 Obsidian 标签面板的判断在四种上下文里对每个码点一致（440 万次）；只写正文的单元缩小之后的范围是全的，两个性质测试加大到 3,000 个种子 × 60 步（替身）、40 个种子（整个程序）都没有找到反例；U+0000 在到达 PostgreSQL 的每个字符串里都替换了，键在替换之后算，不会撞主键；`chr(1) || id` 的临时键不会撞、满足 `name_key <> ''`、出错时随事务回滚；reindex 继续、退出码与锁顺序测试（15 次）都对。没有 Critical、High、Medium，也没有修复带来的行为问题；Low：

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C1-L1 | Low | **出发页的父页本身不算"出发文件夹的子树"**（修复之前就如此）：从 `Docs/API/Overview` 写 `[[API]]`，有 `Docs/API/v2/API` 时解析到它，Obsidian 选 `Docs/API`；从 `P/x/x` 写 `[[x]]` 解析到它自己。Obsidian 的 `startsWith` 把 `p/a.md` 算在 `p/a` 里。文档只列了三处字符串比较的差异，这是第四处；没有测试 | 已改为文件夹自己的页也算：导出时页 `A` 是 `A.md`，与它的文件夹 `A/` 并排，在我们的模型里父页就是那个文件夹，这样既合直觉，也与 Obsidian 一致。样例 006 加了两条，与真实的 Obsidian 1.12.7 核对一致。folder-page-outside 失败 |
| C1-L2 | Low | "任何正文都能保存"的测试用 `strings.Repeat("x", 2800)`，PostgreSQL 把它压缩进了 B-tree 的一项，去掉键的上限也能保存，只靠行的断言抓住 | 已改为随机字母。去掉上限时保存答 500（maxkey-off-db-status） |
| C1-L3 | Low | 路径穿过已删的页只经 `ByKeys` 测了，`ByIDs` 的那条去掉也通过 | 已补。byids-through-deleted 失败 |
| C1-L4 | Low | `TestNoTitlesKeyComesNearMaxKey` 只扫到 U+20000、按原字符算长度，跳过了 NFC 会变长的字符（结论成立：逐个码点扫过，最坏正好两倍） | 已改：扫全部码点，先 NFC |
| C1-L5 | Low（文字） | P3 文档 3.1 仍写 `IsTag`；3.3 的接口注释没写名称；发现的编号用的是各审查者自己的；README 列的与 Obsidian 的不同不全（漏了大小写折叠、按 id 的并列），标签规则写得比代码宽 | 已改 |
| C1-L6 | Low | `check.mjs` 的标题检查没拒绝行、段分隔符与双向控制字符 | 已改 |
| C1-L7 | Low | reindex 期间数据库断了，余下每个笔记本各打一行失败（可能各等一次连接超时）；取消则立刻停 | 不改：少见，每行都写明原因，Ctrl-C 立刻停 |

第一轮核对的修复是 `9f35fb5`。

## 第二轮修复的核对

第一轮的修复（`9f35fb5`）之后由一位 Opus 只读核对：约 43 万条随机链接（偏向同名的深链、与父页或祖先同名的子页、改了大小写的同名、相对与从根起的目标、`.md`/`.MD`、Markdown 链接），把三处按整段的规则换成 Obsidian 的字符串比较之后，与 `getLinkpathDest` 没有未记录的差异；文件夹自己的页决定了其中每批一千多条，把 `>=` 改回 `>` 正好多出这么多差异。文件夹自己的页不会与子树里的别的候选并列（它们都更长）；`under` 只读路径上的 id，所以增量维护不需要多到达什么：替身上的随机测试里文件夹自己的页赢了 4,860 次（3,194 次有别的候选），一个专门的测试（文件夹页改名、改回、同键改名、移到根下，出发页移深、移浅、改成与文件夹同名，别名，删除）每步都等于重建。新的或改了的测试各自在去掉修复时失败，快且确定。没有行为上的发现；Low：

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C2-L1 | Low（文字） | P3 文档的"也不是出发页自己"挂在了 `Docs/API/Overview` 的例子上，它属于 `P/q/q` 的例子 | 已改（`b0516ff`） |
| C2-L2 | Low（文字） | M6 总设计与总体设计 4.4 还是"同一父节点、层数少" | 随合并修订（本次文档提交） |
| C2-L3 | Low | 没有正文、只有子页的页导出时只有文件夹、没有 `.md`（总体设计 3.5）：Obsidian 里没有它这个文件，解析到它的链接（第 2 步与文件夹自己的页）在 Obsidian 里解析到别处 | 写进 P3 文档第 2 节；导出（M7）是否给被链接的这种页写空的 `.md`，经 M6 收尾移交 M7 |

