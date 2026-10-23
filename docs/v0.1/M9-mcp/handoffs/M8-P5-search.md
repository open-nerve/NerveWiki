```yaml
status: done
from: M8/P5
to: M9
created: 2026-10-16
```

# `grep` 用搜索的索引；agent 的写与搜索

M8/P5 加了工作区的全文搜索（`searchWorkspace`，[M8/P5 文档](../../M8-history-search/05-P5-search.md)，[M8 总设计](../../M8-history-search/00-M8-design.md) 4.12）：应用算的词存在侧表 `page_search` 里（`tsvector`，GIN），单元在事务里只把碰过的节点放进 `page_search_queue`，page 的后台任务在请求之外重建行；读时候选是索引的命中并上队列里准确的节点，对当前的名称与正文复查。M8 的 Codex 评审之后（2026-10-17）每个词带它的笔记本的前缀，候选只读读者的范围里的笔记本的键（R1：搜索的耗时曾说出看不到的笔记本里有没有某个词的大量页；[Codex 评审](../../M8-history-search/reviews/M8-codex-review.md)第 9 节）。M9 的 `grep` 用这个索引（总体设计 12.3）。留给 M9 的：

1. **`grep` 的键与候选**：
   - 索引里有什么（[M8/P5](../../M8-history-search/05-P5-search.md) 3.3）：每个活着的节点一行；标题是名称的 CJK 单字与二元组、字母与数字 3 个起的三元组、1–2 个的整词，正文是名称加正文的 CJK 二元组、三元组与整词（不收单字）；都按 `searchFold`（`shared.TitleKey` 加切罗基文的小写换成大写）折叠之后取。字母文字是 P5A 按实测定下的三元组：字母与数字的任意子串（3 个起）都取得出键，`api` 找得到 `rapid`。
   - 从正则的字面片段取键：用 `regexp/syntax` 解析，取每段必须出现的字面串（拼接里的字面量；选择、可选、可以重复零次的部分不取）：CJK 2 个字起给二元组，字母与数字 3 个起给三元组，1–2 个的只在两边必是边界时给整词；不区分大小写时折叠之后取，区分时也取折叠的键（键是超集），由正则复查。
   - 候选与读法照搜索的正文层（P5 3.5）：`page_search_hits(notebooks, body_keys, title_keys)`（迁移里的函数，只读 GIN 的位图；名称层是 `page_search_titles(notebooks, keys)`；键是不带前缀的数组，函数按范围里每本笔记本加前缀）的命中，范围是调用者看得到的笔记本，是必需的参数（为空时什么也不读）：`grep` 同样只读范围里的，不读别的笔记本的倒排（总体设计 13.1 第 4 条，由块数的测试核对，13.4 第 12 条），以同一快照里数出的命中数作 `LIMIT`，并上队列里准确的节点；只带笔记本的条件连 `nodes` 与 `page_contents`，别的检查放在 `OFFSET 0` 之上：照写的连接与规划器自己的读法在 30 万页上有 5–40 秒的整表扫描。之后在 SQL 里以正则复查（正文按它的 NFC 读），路径的 glob 再筛。正文是 PostgreSQL 的 NFC，Go 的 NFC 可以不同（30 个以上的组合记号时插 CGJ，两边的 Unicode 版本不同的记号）：搜索对 Go 的 NFC 改了的词另读 PostgreSQL 的 NFC 写正文的模式（`SearchNFC`，Codex 评审 R2），空隙记在 `nfcGaps`、由漂移测试核对；`grep` 的正则里的字面量同样要按 PostgreSQL 的 NFC 比正文。批量的节点（导入、后台的恢复与撤销、升级的补入、reindex 之后，建好之前）不当候选，`building` 时同样说明结果可能不全。
   - 取不出键的正则只能有界地扫：同 `MaxSearchWalk`（500 页的正文）的走查，答"继续"的游标。
   - 不进索引的部分（超过 64 个的 ASCII 字母与数字的连续串、长的 `data:` 串、一页超出约 1 MB 的词，每个词连同前缀算，约 0.8 MB 的中文）只在节点排队时找得到；`grep` 要找它们只能不经索引地扫。
   - PostgreSQL 的正则（ARE）与 Go 的 RE2 不同：搜索的模式由同一个构造器写成两种（大小写写成简单折叠的轨道的方括号，P5 3.3）；用户写的正则在 SQL 里以哪种语义执行、怎样拒绝两边不同的写法，M9 定。
2. **agent 写完即搜**：经单元的写（含 MCP 的）以准确入队，读时当作候选、对当前的状态复查，写完立即搜得到。agent 以 `batch` 大量新建时（导入与后台的恢复、撤销以外的单元是准确的），准确的积压大过 `MaxSearchWalk` 会让罕见词的第一页为空而带游标，直到任务追上（从空建约 300 页/秒，按笔记本分键之后满的索引上约 190–250 页/秒；准确的积压 1,000 页时 92 个第一页里 17 个为空，平时 7 个）。看任务的吞吐跟不跟得上，必要时让大的 `batch` 的单元以批量入队（`UnitSpec.Background`，之后带 `building`）。
3. **MCP 的 `search`**（若加）：调用同一个 `Search` 用例（动作 `page.search`、看得到的笔记本、两层、约数、`building`）。agent 要读 `count` 的种类（`exact`、`about`、`at_least`）与 `next_cursor`：第一页可能少于 `limit` 个、甚至一个也没有而带游标（走查的界线）；被隔开的单字的词（`中·国`）在没有正文键的查询里只找名称。
4. **上限**：`q` 1–256 字节、至多 8 个词、至多 64 个连续的 ASCII 字母与数字；`grep` 的正则另定上限并在上限处测试（总体设计 13.1 第 31 条）。

## 处理进展

- M9/P3 的 S1、P4 的 S1（2026-10-20、22，合并 `f19d0b6f`、`7e7e414d`）（[M9 总设计](../00-M9-design.md) 4.11、4.13）：
  - 第 1 项（P4 的 S1）：`GrepKeys`（`page/domain/grep_keys.go`）：CJK 的二元组，字母与数字 3 个起的三元组；1–2 个字符的整词只在模式本身给出两边的边界时取（`\b`、`\B` 从不算）；危险的开头与结尾的原子去掉（96 个结尾，`grep_tables.go`）；至多 32 个键。`GrepCandidates` 一条语句三支（这本笔记本的键、准确的队列、`!cut`），不连 `nodes` 与 `page_contents`，树与 glob 在 Go 里取交，所以这一份说的两个陷阱都碰不到。分词的版本 5、`partial`、迁移 00046。复查不在 SQL 里以 ARE 做，在 Go 里以 RE2 按行匹配，两边都是 Go 的 NFC，PostgreSQL 从不编译用户的模式；取不出键的模式有界地走查、给游标；批量的节点答 `building`。测试：`TestGrepKeys`、`TestGrepKeysAreASuperset`、`TestGrepKeysOverTheRiskyTails`、`TestGrepCandidates`、`TestGrepPlans`、`TestHiddenNotebooksLeaveTheSearchsReadsAlone`、`TestTheMigrationQueuesEveryPartialPage`、`TestTheMCPGrepFindsWhatItsKeysFind`、`TestTheMCPGrepWalksEveryPartialPage`。
  - 第 2 项（P3 的 S1）：batch 批量入队（`UnitSpec.BulkSearch`，只由 `apply_batch.go` 设），`Outcome.Building` 与 batch、`grep`、`search` 的答复里的 `building`（M9 总设计第 11 节主持者的决定第 44 条）；单个的 MCP 写照旧准确入队。
  - 第 3 项（P4 的 S1）：`search` 经 `Search.InNotebook`（看不到的一律 `notebook.not_found`）、`SearchHit.Tier`、`Cursor()`；`TestTheMCPSearchIsTheNotebooksSearch`、MC6。
  - 第 4 项：`MaxGrepPatternBytes` 1,024、`MaxGrepInsts` 2,000、`MaxGrepWork` 2 × 10⁸、`MaxGrepLineWork` 0.75 × 10⁸、3 秒的软期限的余地与并发的名额；`TestTheMCPGrepStopsAtItsBounds` 与三个时间守卫。
- M9 收尾（2026-10-23）：核对过各项的落实（[M9 收尾审查](../reviews/M9-closeout-review.md)"handoff"，C 逐项对照代码），关闭。
