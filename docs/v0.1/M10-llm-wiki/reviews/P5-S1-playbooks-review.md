# M10/P5 S1（playbook 与说明）：审查记录

| 项 | 内容 |
|---|---|
| 对象 | 分支 `m10-p5-s1`，base `94c335db`。三个产品提交 `7ce0396f`、`e9d9589a`、`dbca1230`，之后五个修复提交 |
| 方法 | 三位独立审查者，各在自己的 worktree 里，在真库上跑，用 overlay 改坏代码。A 管服务端的正确性，B 管契约、组合根、矩阵，并照 playbook 一步一步用真实工具走，C 管测试、变异、e2e、每个提交是否单独为绿、CI 与量的数 |
| 结论 | 没有产品缺陷。Important 四条，都是抓不到错的测试，最值得看的是 B-I1、C-I1：矩阵把没有 `data.code` 的错误当成答复，读者取用失败时全绿。产品文字上一处 Minor（B-M3），ingest 有一句话会误导 agent。另外查出两件和 P5 无关的事：main 的 S1 冒烟有一个真实的启动窗口，第六份在 EPYC 7763 上过线 |

S1 建成了按入口的 Prompt 注册表和它的界、三份英文 playbook、两个入口共用的说明。P5 设计是设计，这份记录只写审查。

## 发现与处置

提交的简写：`3647fed1` A 席的测试缺口，`160d41e0` 分份，`a1892aba` fixture，`d9d78cac` B、C 两席的发现，`7efce659` 核对之后的一条。

| 编号 | 级别 | 发现 | 处置 | 提交 |
|---|---|---|---|---|
| B-I1、C-I1 | Important | `promptCellOf` 把没有 `data.code` 的 JSON-RPC 错误读成空格子，等于答复。读者每次取用都失败时 26 格全绿，组合根交空时 LLM Wiki 的 13 格也全绿 | 没有码的错误判失败，答复要有这个 prompt 的第一行 | `d9d78cac` |
| C-I2 | Important | adapter 的参数处理没有测试：空的 source 丢掉、带空白的 source 被裁、空键的值进正文，三个变异都活 | 加 `TestThePromptsPassTheirArgument` 与组合根的三个子测试 | `3647fed1`、`d9d78cac` |
| C-I3 | Important | `Read` 没有 AGENTS 时提前返回，握手就说 "pages under `/`"，全绿 | 单元与组合根都加清空 AGENTS 的一步 | `d9d78cac` |
| C-I4、A-M1、A-M2 | Important、Minor | 设计 §7.2 的三种并发结果没有钉住；握手答内置说明、`open_notebook` 吞掉错误、`methodError` 丢掉码都活。C 定 Important，A 定 Minor，取高的 | `TestTheInstructionsShareNoTransaction`、`TestOpenNotebookKeepsTheInstructionsError`、`TestAProblemOfTheInstructionsIsTheHandshakesError` | `3647fed1`、`d9d78cac` |
| B-M1 | Minor | 只测 `lint`，`query` 绕过用例时全绿，矩阵也不跟着 `prompts.json` 走 | 矩阵遍历 `prompts.json`，78 格 | `d9d78cac` |
| B-M2、A-M3、C-M1 | Minor | `logged` 子测试读到的是更早的日志行 | 数这次取用前后的行 | `3647fed1` |
| B-M3 | Minor | ingest 说 batch 成功而没记标记时 lint 会说明原因，索引落后时两条规则都是 0，唯一的信号是 `stale_pages`，agent 会去报告一个修不了的失败 | ingest 写明 `stale_pages`，写明来源离开 `pending_sources` 就算消化过 | `d9d78cac` |
| B-M4 | Minor | README 写"有排队或运行的任务时拒绝"，导出进行时降级其实放行 | 改成写入任务 | `d9d78cac` |
| A-M4、C-M2 | Minor | 客户端、入口、`request_id` 在 `Call` 与日志里没有断言，账户入口从没走到处理函数 | `TestPromptCallKnowsItsCaller` 补全，两个入口都走 | `3647fed1`、`d9d78cac` |
| C-M3 | Minor | 预算一行的 BOM、frontmatter 没有和系统段一起测，答复用 `TextBytes` 判 | 组合根写带 BOM 与 frontmatter 的 AGENTS，答复用 SDK 量 | `3647fed1`、`d9d78cac` |
| C-M4 | Minor | "512 是 raw" 没有测试，路径里有 `&` 时按转义算也全绿 | `path_at_bound` 加 `&` | `d9d78cac` |
| C-M5 | Minor | `WorkspaceOf` 的失败变成 `notebook.not_found` 全绿 | 加这一例 | `d9d78cac` |
| C2-M1 | Minor | 核对时发现：两个调用处各只测一种错误，把 `ErrNotFound` 当没有说明的写法抓不到 | 两种错误都走两边 | `7efce659` |
| A-N1 | Nit | 代码逐个参数查，设计写先查 Required 再查字节数 | 改设计的话 | 文档 |
| A-N2、A-N3、A-N4、C-N4 | Nit | 答复用 `TextBytes` 判；500 没有测；空键没有测 | 补 | `3647fed1` |
| A-N5、C-N1、C-N2 | Nit | 注释过时；测试里是裸的 U+2028、U+2029 | 改注释，改成 `\u` 转义 | `3647fed1`、`d9d78cac` |
| B-N1 | Nit | 组合根交空时比较器那一段 panic，后面的测试不报 | 先判空 | `d9d78cac` |
| B-N2 | Nit | README 没写路径过长时改写标题 | 补一句 | `d9d78cac` |
| B-N4 | Nit | `log` 的变更只有名字，lint 那句只说读全用 `changeset` | 写明路径也要 `changeset` | `d9d78cac` |
| B-N5、C2-N1 | Nit | M9 清单的工具数过时，比 P5 早 | 改成 16、18 | `d9d78cac`、文档 |
| C-N3 | Nit | `listPrompts` 的 `?? []` 让 null 也过 | 不是数组就抛错 | `d9d78cac` |
| C-N6 | Nit | `notebook.yaml` 写降级之后没有 prompt，其实列着三个 | 改写，`make gen` | `d9d78cac` |
| C-N7 | Nit | 清单 J7 与设计 §8.2 不一致；过界的日志多了 `request_id` | 设计照实改 | 文档 |
| C2-N2 | Nit | "降级"一例是两个固定的状态 | 一个 entry，状态在两次握手之间变 | `7efce659` |
| B-N3 | Nit | 来源根的标题带反引号时断开代码段 | 接受，文字还读得懂 | — |
| B-N6 | Nit | 清空 AGENTS 之后，系统段后面跟的内置说明写着 "It has no AGENTS page" | 接受，这是 M9 的句子，页确实没有正文 | — |
| C-N5 | Nit | `at_bound` 写 `==`，§7.1 写握手用 `≤` | 接受，这里量的是文字工具，输入也没有 `<>&` | — |

## 两件和 P5 无关的事

**main 的红。** 设计的第二个提交 `94c335db` 只改文档，main CI [42357198372](https://github.com/open-nerve/NerveWiki/actions/runs/42357198372) 却红在 S1 冒烟：`nervewiki did not exit cleanly: code 1`。我推上去之后没有看这次运行，是 C 查出来的。根因在服务的日志里：`/readyz` 答 200 之后 River 的 `LISTEN` 还没做完，故事删了数据库，`start the jobs` 失败，服务退出。C 把 River 的启动压到数据库没了之后，确定性地复现了同样的两个错误。"任务起不来就停服务"是写明的行为，所以修 fixture，不改产品（决定 167）。

**第六份。** C 问第六份为什么 491 秒。它只有 `TestIngest`，在 EPYC 7763 上 483–493 秒，别的 CPU 上 301–373 秒，两次 main 都落在 7763 时过线。决定 166 移走两个测试。效果要看后面的 main，分支上 C 看到的单次样本还说明不了什么。

## 修复核对

A 的发现都只涉及测试与注释，照 02 §3.4 由主持者读 diff、看变异核对。

B、C 原席核对 `d9d78cac`。B：全部关闭，只剩 M9 清单一处工具数和附录 B 要同步，都在文档提交里。B 重跑了自己的变异：M7 现在 9 格失败，M3 7 格失败，新的 ingest 句子对得上 `stale_pages` 的探针。C：4 条 Important、5 条 Minor 全部关闭，重跑的 X、D 变异都被杀死，另提一条 Minor、两条 Nit，都在 `7efce659` 与文档里。这一条 Minor 只改测试，由主持者核对。

设计提交本身有一处失误，写在[设计审查](P5-design-review.md)里。

## 主持者的决定（负责人可以改判）

- **166**：第六份的 `TestIngestCandidates`、`TestIngestCoverage` 移到第五份。
- **167**：`/readyz` 不等后台任务，e2e 的 fixture 等 `jobs started`。

两条都在 [00](../00-M10-design.md) 第 11 节。

## 变异

每条都先在没改过的树上跑同样的命令，全部通过，再改坏。

`dbca1230`：设计 §7.3 的 P01–P27，P23 分单元与组合根两条，28 条全部被抓到。P19 与设计说的一样，`rename` 抓不到，`move` 抓到。

`d9d78cac`：同样 28 条，加审查者留下的 15 条（设计表的 P28–P41，P35 分单元与组合根两条），43 条全部被抓到。

## 门禁与 CI

`dbca1230` 本机：`make lint gen-check knip build test-go-short` 通过；`make e2e` 495 个通过。`make test` 第一次被我设的 1500 秒时限在 `test-go-costs-2` 中途掐断，race 包与组合根六份已经过了；剩下的 `test-go-costs-2`、`test-go-costs-3`、`test-go-costs-lint`、`test-go-guards`、`test-web` 分开跑完，都通过。修复之后的提交没有重跑整个 `make test`，靠分支 CI；最后的树上本机重跑了 `make lint gen-check knip build test-go-short test-web` 与 `make e2e`，都通过，e2e 495 个。

分支 CI：`dbca1230` [42359985832](https://github.com/open-nerve/NerveWiki/actions/runs/42359985832) 全绿，495 个 e2e。最后的 `f57a6d44` [42365781770](https://github.com/open-nerve/NerveWiki/actions/runs/42365781770) 全绿。合并 `ba1fd4c9`，main CI [42367580440](https://github.com/open-nerve/NerveWiki/actions/runs/42367580440) 全绿。

每个提交是否单独为绿：C 逐个跑了 build、vet、lint、短测试，第 2、3 个加 `check:types`、`oxfmt`、`knip`，都过。

## 分份的数

| 运行 | 提交 | 1 | 2 | 3 | 4 | 5 | 6 |
|---|---|---|---|---|---|---|---|
| 42357162761（main） | `3b0d55f2` | 429 | 381 | 369 | 415 | 344 | 483 |
| 42357198372（main） | `94c335db` | 352 | 336 | 324 | 408 | 359 | 493 |
| 42359985832（分支） | `dbca1230` | 433 | 288 | 409 | 317 | 351 | 491 |
| 42364701652（分支，已移两个测试；这次运行被下一次推送取消，组合根六份已跑完） | `d9d78cac` | 356 | 332 | 392 | 353 | 366 | 378 |
| 42365781770（分支） | `f57a6d44` | 401 | 426 | 344 | 301 | 292 | 368 |
| 42367580440（main，合并） | `ba1fd4c9` | 393 | 338 | 277 | 300 | 436 | 398 |

新测试在本机竞态检测下 1.2–4.1 秒，四个落第三份，降级落第四份，`TestNotebookInstructionsShareTheirBudget` 在 `test-go-race`。全部 568 个顶层测试各落一份：167/58/123/136/63/21。

## 接受与推后

- 没有接真实客户端。人工清单的 J 部分留给负责人，在 v0.1 的最后执行。
- 客户端怎样截断说明只有文档上的说法（决定 161），J6 那一步才看得到。
