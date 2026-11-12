# M11/P3 S2（playbook 与文档，服务端）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m11-p3-s2`（实施的 3 个提交：prompt `upgrade` 与它的 playbook（`wiki/domain/playbook-upgrade.md`、`UpgradePlaybookText`、用例 `UpgradePlaybook`、笔记本入口的第四个 prompt、`api/mcp/prompts.json`、`upgrade_submit` 的说明、模块根的导出、单元测试三组、组合根的四处 prompt 测试、`Makefile` 的计数注释）`567f1201`；e2e 的 UP9 与 LW11 的帮手 `9d7e9004`；README 的"升级为 LLM Wiki"一节与 MCP 人工清单的 K `d2512eeb`；从 `main` 的 `d33b4544` 开始，审查时在 `d2512eeb`，22 个文件、加 897 行、减 83 行）；修复一轮在它之上加 3 个提交（只改测试与注释 `94db07e4`；playbook 的三处措辞 `8321734b`；README 与人工清单 `7daa3452`），头 `7daa3452`（7 个文件、加 96 行、减 69 行） |
| 审查方式 | 三位审查者（Opus）并行、只读，各在自己的分离的工作树里做实验（前几个 Step 的三个工作树挪到 `d2512eeb`），重的都经 `heavy.sh`，先读设计与代码、自己形成判断之后才看实施者的报告（报告里的数、变异表、8 条偏离与"自己加的"都当要核实的说法）：用例、登记与组合根的 prompt 测试（A：`UpgradePlaybook` 对照 `Playbooks.Execute` 与 `authorizeLintRead`、登记与模块根、契约的五处说明、组合根的四处测试；wiki 的单元，经 `heavy.sh` 以 `-v` 跑 `TestUpgradePromptMatrix`、`TestUpgradePrompt` 确认 217 个子测试真的跑了；`mut.py` 11 条与跨两个文件的 d3 一条，a1 在单元与矩阵上存活）；测试看不看得见、e2e 与分份（B：`-list` 以 `make -n` 展开的八条 `-run`/`-skip` 自己归份；1、2 号提交各自检出跑一轮；CI 的作业日志；自己的 60 条变异，其中 playbook 逐句删 30 句、保留片段只改建议的 20 条，另有跨文件的 d3 与 `make build` 之后对 UP9 的两条）；playbook、README、人工清单的 K 与文档稿（C：playbook 逐句对着代码与决定核对、与设计 3.17 的初稿逐字 `diff`；README 与 K 对着契约、配置、网页的文字与导入的测试；`tools.json` 两个入口逐项数工具与注解；`TestTheBuiltInInstructionsAreShort`、`TestUpgradePlaybookText` 以 `-v` 实跑；七条改 playbook 做法的变异全活；判实施报告第 7 节各条进不进 3.24） |
| 日期 | 2026-11-12（实施、审查、一轮修复与一轮修复核对） |
| 结论 | Important 0。Minor 2（去重之后）：playbook 的单元测试只钉每类的条件与码、不钉做法，00 号文档 4.13 清单里五项各自删掉照过，`page.upgrade_immutable` 一格被另一句代为满足，改成错的做法也照过（P3S2B-M1 = P3S2C-M1，B-N1 是它的另一面）；人工清单的 K 没写第二个客户端怎样做，第一个客户端的 K4 把升级送进审阅之后，第二个的 K2、K3 看不到要看的东西（P3S2C-M2）。Nit 10：单元缺"审阅中、有报告"一格（P3S2A-N1）、注释仍写"三个"（P3S2A-N2）、保留片段只改建议照过（P3S2B-N1）、README 三处与 K 两处的措辞（P3S2C-N1、N2、N5、N6）、playbook 闸门 4 与 `server_busy` 的两处说法（P3S2C-N3、N4）、实施报告第 10 节的稿（P3S2C-N7）。Q 2：`AGENTS` 的模板写"三个 prompt"（P3S2A-Q1 → 主持者的决定 299）；第三份是离 450 秒最近的一份（P3S2B-Q1，记进 10.2）。三位都没有查出 playbook、README 或 K 说错码的意思、指错路；实施者的 8 条偏离都判对（第 2 条不是偏离，第 8 条是过程）。修复一轮（3 个提交，头 `7daa3452`）照 `fix-1.md` 改了全部发现：playbook 的测试每句或分句一格、各恰好出现一次（47 格，合起来覆盖第一行之外的整段正文），33 条变异改前活、改后死，逐句删除改前 9 段照过、改后 30 段都失败；playbook 的三处措辞（闸门 4、`server_busy`、`forbidden`），4,979 字节；分支 CI 一次全绿；与修复说明不同之处 4 条，意思都一致（P3 设计 3.24）。修复核对一轮：三位把各自的发现都关上；新的 Nit 五条，P3S2B-N2（格数是 47，不是 46）与 P3S2C-N8–N10（文档稿：3.17 的块漏了 `forbidden` 那半句、3.20 的块还是初稿、决定 299 的一条理由不实）在本提交，P3S2B-N3（新加一句相反的话照过）不改；没有 Important、Minor 留下，不再有修复轮 |

## 审查的发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| 编号 | — | （编号照审查者的报告：P3S2A-N1、N2、Q1，P3S2B-M1、N1、Q1，P3S2C-M1、M2、N1–N7。同一个口子合为一行：B-M1 = C-M1（B-N1 是它的另一面，另列一行）。修复一轮照主持者的修复说明（`p3s2/review/fix-1.md`，提交 1 只改测试与注释、提交 2 改 playbook 的两处措辞、提交 3 改 README 与人工清单）在 `d2512eeb` 之上加新提交，普通推送、不改写已推的历史，每个单独为绿。审查时的代码位置是 `d2512eeb` 的；服务端的模块在 `server/internal/modules/` 之下，`bootstrap/` 在 `server/internal/` 之下） | — |
| P3S2B-M1 = P3S2C-M1 | 中（测试） | `wiki/domain/playbook_test.go:27-73`：对每个片段只判 `strings.Contains`，片段多止于条件或码（"is not valid YAML""has no closing --- line""Keep an existing AGENTS page whose body is not empty""page.upgrade_protected"），后面接的做法不在断言里。B 逐句删 30 句，9 句照过，其中属于 4.13 清单的五项："You may change an original page's frontmatter only …, or the write is page.upgrade_immutable."（整个升级最要紧的一条护栏；这个码在"Splitting the batch … it is still page.upgrade_immutable"里还出现一次，删掉前一句之后 `:47` 的一格由后一句满足；实施者的 s16 把每一处都换掉，所以看起来抓到了）、"Move each source … under the source root."、"Sort the original pages …"、"Start with upgrade_verify without a rule."、"When upgrade_verify reports every gate passed, call upgrade_submit."；另单删"It checks again and puts the upgrade in review, where nothing can be written."也照过。C 把做法换成错的、条件留着，七条全活（`p3s2/review/c/spec.json`）：c1 把"type and summary; a page that rests on sources also lists them in sources"改回"type, summary and sources"（设计审查 P3C-M6 第 2 处修掉的那一种）、c2 删"or the proposed source root"、c3、c4 把两种无效 frontmatter 的改法换成错的、c5 把"or rename it and write a new AGENTS"改成"and give an empty one a body"（决定 172 的反面）、c6 把"through MCP … only while the upgrade is upgrading"改成"through MCP or REST, in review too"、c7 把根下保留标题的"rename"改成"delete"。UP9 与 `TestUpgradePrompt` 拿嵌入的同一份文字比，也看不见 | 修复一轮改（提交 1，`94db07e4`）：每个片段延长到做法那一半（C 的清单与 B 的 pb01–pb20 那几句）；补 4.13 清单里没有片段的六项（只改 frontmatter 的那一句，`page.upgrade_immutable` 一格改钉它；来源移进来源根；归类；先 `upgrade_verify`；全过再 `upgrade_submit`；审阅中冻结）；每个片段断言恰好出现一次（`strings.Count(text, s) == 1`）；C 的 c1–c7、B 的逐句删除里照过的与 pb01–pb20 改前活、改后都要死，还有照过的写明为什么等价或再补片段。实施者走到底：每格是一整句，一句里有两件事时是一个分句，46 格；提交 2 把 `notebook.upgrading` 那一句拆成两格，47 格（实施报告写 46，核对一轮 P3S2B-N2），合起来覆盖第一行之外的整段正文。c1–c7、pb01–pb20、逐句删除里照过的五段与 A 的 a1 共 33 条改前活、改后死，逐句删除 30 段改后都失败（"反向对照"）；设计 3.17 的"必有的句子"随之写（本提交） |
| P3S2C-M2 | 中（文档） | `docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md:312-322`：清单的头写 Claude Code 与 Codex 各一遍、"每一步每个客户端"，J 末写"逐客户端记"；K 的准备只备一本升级中的笔记本。第一个客户端做到 K4，提交通过、升级进入审阅、对所有人冻结；第二个客户端照表做：K2 取 `upgrade` 答 `wiki.upgrade_state`（只在 `upgrading` 答文字），K3 改正文答 `notebook.upgrading`，K4 答 `wiki.upgrade_state`。负责人照表会在第二个客户端记"不通过"，而那不是客户端的事 | 修复一轮改（提交 3，`7daa3452`）：K 的准备写两个客户端各用自己的一本（同一位管理员各发起一次升级），K5 共用那本普通笔记本；照 J 加"逐客户端记" |
| P3S2A-N1 | 细节（测试） | `wiki/app/upgrade_playbook_test.go:90` 的 `reviewing` 一格不带报告：判断改成"审阅中而有报告也答文字"（a1），单元与矩阵都照过（矩阵种的审阅中也没有报告），只有 `TestUpgradePrompt` 的真提交抓到；反过来"审阅中没有报告也答"（a2）只有矩阵抓到。两种审阅中的状态各只有一个组合根的测试看得见；也说明偏离 5 用真提交不是可有可无 | 修复一轮改（提交 1）：单元表加"审阅中、有报告"一格（A 给的写法）；a1 要死 |
| P3S2A-N2 | 细节（注释） | `wiki/adapter/mcp/prompts.go:30` 的"Prompts are the LLM Wiki's three prompts, in the notebook entry alone …"（函数现在交四个）；`bootstrap/mcp_prompts_test.go:141` 的"The three prompts of the composition root reach their handlers"（组合根现在有四个，这个测试的表照设计仍是三个） | 修复一轮改（提交 1）：照 A 的写法改两处 |
| P3S2B-N1 | 细节（测试） | `playbook_test.go:29`、`:30`、`:47-49`、`:51`、`:62`：保留片段、只把后面的建议改成相反的意思，20 条都照过（`b/pb.out`）：例如"is not valid YAML: add a new valid block before it"（正是原句末尾警告的那种错）、"Never delete …"改成"You may delete an original node, but not the proposed source root"、"the upgrade is upgrading again, so verify, fix and submit again"改成"the upgrade stays in review, so wait for an admin"。服务端的护栏照样拒绝错误的建议引出的写，影响只是 agent 白费一轮 | 随 P3S2B-M1 改（提交 1） |
| P3S2C-N1 | 细节（文档） | `README.md:254` 第 2 步"给其余的页补上 frontmatter（`type`、`summary`、`sources`）"读来每页都要 `sources`（设计审查 P3C-M6 第 2 处在 playbook 里修掉的那一种）；闸门只要 `type`、`summary`，`sources` 只对依据来源的页，`AGENTS` 与来源根本身不要 `type`、`summary`。设计 3.19 的原文就是这样 | 修复一轮改（提交 3）：写"给来源区之外的 wiki 页补上 `type`、`summary`（依据来源的页另列 `sources`）"；设计 3.19 随之改（本提交） |
| P3S2C-N2 | 细节（文档） | `README.md:220`"只有升级中的笔记本用得上"：`upgrade_verify` 在审阅中也答，两个工具也只给笔记本的管理员 | 修复一轮改（提交 3）：写"笔记本的管理员在升级中用得上（`upgrade_verify` 审阅中也可）" |
| P3S2C-N3 | 细节（playbook） | `playbook-upgrade.md:5`"every page outside the source root has valid frontmatter with type and summary"比代码宽：`invalid_frontmatter` 管来源区外的每一页（含来源根与根下 `AGENTS`），`missing_type`、`missing_summary` 不管这两页。agent 照它会给 `AGENTS` 加 `type`、`summary`，无害 | 修复一轮改（提交 2，`8321734b`）：写"every page outside the source root has valid frontmatter, and every one but the source root and AGENTS has type and summary"，测试的那一格随之改；设计 3.17 随之改（本提交） |
| P3S2C-N4 | 细节（playbook） | `playbook-upgrade.md:15` 只在提交一段说 `server_busy`："server_busy means wait retry_after_seconds, then submit again"；`upgrade_verify` 也答 `server_busy`（lint 的名额），也答 `forbidden`（用户已不是管理员） | 修复一轮改（提交 2）：写"server_busy from either tool means wait retry_after_seconds and call it again"，`forbidden` 并进"your user is no longer an admin"那一句（实施者写"and forbidden from upgrade_verify or upgrade_submit means your user is not an admin of it"：读者的 agent 调这两个工具也答 `forbidden`，这一段还没讲到 `upgrade_submit`；P3 设计 3.24）；测试随之改；playbook 4,979 字节；设计 3.17 随之改（本提交） |
| P3S2C-N5 | 细节（文档） | K 的准备（`P5-mcp-clients-checklist.md:314`）没有 J 那一句"沿用上面隔离的客户端环境，记下……版本和日期，不碰自己的配置"；K4（`:321`）写"提交通过之后再写答 `notebook.upgrading` 怎样显示"，没写提交不过（`wiki.upgrade_not_ready`）时记什么、要不要补齐再交 | 修复一轮改（提交 3）：照 J 加那一句；K4 写明提交不过时记什么、补齐再交 |
| P3S2C-N6 | 细节（文档） | `README.md:261`"根下的 `index`、`log`、`index.md`、`log.md` 要由 agent 改名，闸门才会过"：本段举的例子是从另一个实例导出的 LLM Wiki，它的 `index.md`、`log.md` 是导出加的文件，导入时跳过；句子不错，读来像一定要改名 | 修复一轮改（提交 3）：写"根下若有……"；设计 3.19 随之改（本提交） |
| P3S2C-N7 | 细节（文档） | 实施报告第 10 节的稿：`:299`、`:319` 是 `d33b4544` 的 README 行号，合并之后是 `:314`、`:334`，要写明是哪一版的；"它让之不实的几处（`:214` 的字节数……）"：`:214` 的数从 P2 起就过时，不是 S2 让它不实，写"一并改的"；第 7 节第 8 条（标题与范围）是过程，进 10.2、不进 3.24 | 本提交：10.2 照改 |
| P3S2A-Q1 | 问题（给主持者） | `wiki/domain/template-en-agents.md:33` 写"Three prompts are served there:"，`template-zh-CN-agents.md:33` 写"那里有三个 prompt："。发起升级时补写的 `AGENTS` 也用这份模板（`wiki/app/template.go:112`），普通笔记本的握手说明含 `AGENTS` 的正文，所以升级中的 agent 一握手读到"有三个 prompt：ingest、query、lint"，这时它们都答 `notebook.not_llm_wiki`，真正答文字的 `upgrade` 没提。照 LLM Wiki 读，英文的"served"仍可算对，中文按字面已不实。A 倾向不在 S2 改：改模板要动 LW1 等读模板的断言（核对一轮 C 查明没有断言钉这一句，P3S2C-N10） | 主持者的决定 299（下面）：S2 不改，进 M12 的打磨移交（本提交） |
| P3S2B-Q1 | 问题（给主持者） | 第三份 S2 没有改动它的时长（`TestThePromptReadsLLMWikiMatrix` 格数同前、契约的 `api/dist` 没变），但它在 EPYC 7763 上的样本 411（S1 分支）、401（S1 修复）、393（S1 合并之后的 `main`）、412（S2 分支，余 38 秒），是八份里最贴线的；10.2 要不要记下 | 主持者：记进 10.2（另有 S1 记合并号的 `main` 的 419 秒），合并之后看两次 `main`，现在不挪 |

## 主持者的决定

负责人可以改判（299 在 S2 的第一轮修复说明里定下）：

299. **`AGENTS` 的两份模板照旧写"三个 prompt"，S2 不改**（P3S2A-Q1，2026-11-12）：模板写的是 LLM Wiki 建成之后的样子，S2 不改，进 M12 的打磨。中文的"那里有三个 prompt"在升级中按字面不实，进 M12 的打磨移交（本提交加一条，第 40 条）。修复说明原来另写了一条理由"改它要动 LW1 等读模板的断言"，核对一轮查明不实（P3S2C-N10：没有断言钉这一句），去掉。

另两项不编号的处置：P3S2B-Q1 记进 10.2，合并之后看两次 `main`，现在不挪测试；P3S2C-N7 由本提交改 10.2 的稿。实施者没有要编号的产品决定（实施报告第 9 节第 0 条：第 7、8 节都是测试、契约说明与文档的做法，产品行为照设计 3.16 与第 286 条）。

修复一轮的处置照 `fix-1.md`：代码与文档的发现都改，在 `d2512eeb` 之上加三个提交（只改测试与注释；playbook 的两处措辞；README 与人工清单），每个单独为绿，普通推送；变异先在 `d2512eeb` 上跑、要活（C 的 c1–c7、B 的逐句删除里照过的与 pb01–pb20、A 的 a1），改后要死；逐个提交跑 `make lint-go`、`make gen-check`、`make test-go-short`，提交 1、2 另跑 `wiki/...` 的单元与经 `heavy.sh` 的 `TestUpgradePrompt`，提交 2 另跑 UP9；头上 `make lint gen-check knip build`，经 `heavy.sh` 跑 `make test-go-short`、第三与第七份、`make e2e`（或只跑 UP9 与 LW11），不必整轮 `make test`；e2e 卡在 apt 镜像或红在 L1 时照第 277 条只重跑 e2e。P3S2C-N7、决定 299 的移交与 C 对第 7 节进不进 3.24 的判断在本提交。

## 接受与推后的

- **接受**：
  - 等价的变异：e1（只给 `PlaybookText` 的 `switch` 加 `upgrade` 一支，没有调用者；设计审查 P3B-N3），对照是 d3（同一处加上调用者，矩阵与 `TestUpgradePrompt` 都抓到），实施者、A、B 各自核过。d2（不判 `lint.read`）在矩阵上照过是照设计：看不到的、删掉的笔记本在 HTTP 的边上由 `Gate`（`node.list`）答 404，到不了用例，单元抓到。B 的 e2b（`reviewing` 单独答 `ErrUpgradeSubmitting`，同码不同 `detail`）在矩阵与 `TestUpgradePrompt` 上照过：两者只比码，单元的 `detail` 断言看得见，不算漏。A 指出用例里 `!found ||` 一支是等价的（没找到时 `Current` 交回零值，`State == ""` 照样被拒）。
  - 未核实的怀疑：A：`TestUpgradePromptMatrix` 先复制准备好的库、再起应用，要求同一份里前面的测试在清理时关掉了连着模板的池子（`t.Cleanup` 是同步的，`CREATE DATABASE … TEMPLATE` 对其他会话另有约 5 秒的等待，S1、P2 的矩阵同样写法），只单独跑过这个测试，分支 CI 的第七份第一次就绿。B：UP9 以 JS 的 `.trim()` 去首尾空白、Go 是 `strings.TrimSpace`，两者对 U+FEFF 不同，今天的文件是 ASCII，文件将来以 BOM 开头时 `TestUpgradePlaybookText` 的第一行一格同样失败，不会漏；第三、七份本机两轮没有重量，用的是实施者的日志；LW11 在 1 号提交上没有单独跑（服务端与头上逐字相同）。C：发起补写的 `AGENTS` 取 LLM Wiki 的模板（含 `sources` 的说法），一个模型照模板给原有的笔记编来源的可能没有排除，是 K 该记的，没有跑真的 agent。
  - 核对过、没有问题的（三位各自的第 4 节）：`UpgradePlaybook` 的次序、码与"不读模式"，与 `Playbooks.Execute` 只共用 `authorizeLintRead`、状态判断两处各有测试；登记、列表里没有参数的 prompt 不带 `arguments`、模块根的导出有 `TemplatePages` 的先例；`upgrade_submit` 的说明五处一致、`TestTheToolListsAreTheContract` 与 `TestLintToolObjects` 看得见任一处不一致（A 的 a4、a5，B 的 c1、c2）；组合根四处测试的期望表、计数与副本的自我核对（B 的 t1–t4）；16 KiB 的界与去空白（B 的 d1′、实施者的 o13、o12）；UP9 的值来自答复、u1、u2 在 `make build` 之后都抓到；分份（553 个，145/50/120/120/61/15/30/12，每个恰在一份，与 S1 末比只多第七份的两个）；playbook 逐句与代码、决定一致、与设计 3.17 的初稿逐字相同；README 的新一节、九个路径、网页的文字、`request_timeout`、"导入不加序号"、工具数与注解（笔记本入口 18、账户入口 20，读的十二个）；K 的次序照设计审查 P3C-M9；提交的范围与说明（C）。
  - 过度设计：A、B 都指出 `TestThePromptReadsLLMWikiMatrix`（第三份）的 78 格是 `TestUpgradePromptMatrix`（第七份）同样 78 格的子集，后者整值比文字与说明；这是设计 3.18 要留着的，不记为发现，第三份要腾地方时可以只留它对 `prompts.json` 的名单断言。C 没有。
  - 实施者的 8 条偏离（实施报告第 7 节）：都判对；第 2 条不是偏离（设计 3.16 写的就是 `wiki/domain/playbook_test.go`，括号里引的是照着写的那几格，A）；第 8 条（标题与范围）是过程，只进 10.2（C）；其余进 3.24。
  - README 先写了 S3 才有的三个审阅按钮：照第 207 条与 P2/S2 的同类处理接受，措辞与 S3 的设计 4.3、决定 291 一致（C 核过）。playbook 末段的"an admin reviews the upgrade in the web app"与 `upgrade_submit` 说明的"in the web, or sends it back"同样先于网页，属于同一类，同样接受（核对一轮的 C）。
  - 核对一轮的 P3S2B-N3（`TestUpgradePlaybookText` 不看有没有多出来的句子，新加一句相反的话照过；B 建议把"去掉 47 个片段之后只剩空白与标点"写进测试）：不改。这个测试钉的是"该有的句子在"，防的是改写时漏掉一类；新加的文字由审查看 diff。
  - 流程（照实写下）：没有改写过历史；1、2 号的标题照 brief 写行为的改变（设计 6.2 是名词短语），1 号范围带 `build`（`Makefile` 的注释）；说明写进文件再 `git commit -F`；每次提交之前看过 `git status`；没有推 `main`，没有 force-push。
- **推后**：
  - `AGENTS` 模板的"三个 prompt"：第 299 条，M12 的打磨。
  - 第三份贴线：合并之后两次 `main` 为准（设计第 8 节第 1 条）。
  - S3 的部分：网页的审阅一节、批准之后的跳转、UP7 的页面版本，与 7.3 里它们的各行。

## 偶发

实施、审查、修复、修复核对与两次分支 CI 里都没有。两次分支 CI 的 e2e 都一次过，L1 没有出，不用照第 277 条重跑。

## 反向对照

建成时以 `mut.py`（`NW_SERVER` 指实施者的工作树的 `server`）与 `mut/multi.py`（跨文件与"每处都删"的变异）跑，经 `heavy.sh`，在头 `d2512eeb` 的树上；先在没改的树上把 11 个命令各跑一次（`mut/base.txt`，都 rc=0）；组合根的命令不带 `-race`（变异只看抓不抓得到，每条约 2 秒）；从备份复原，没有用 `git checkout --`，改坏时没有跑 `make gen*`；跑完 `git status` 干净、没有 `*.mutbak`。合计 `mut.py` 37 条、`multi.py` 38 条（d3、e1、s01–s36）。7.3 的 S2 三行（d1–d3）、实施者自己的 17 条（o1–o15、x1、x2）与 playbook 要的 36 句（s01–s36）都抓到；d2 在矩阵上照过是照设计，e1 是等价、有对照 d3。

**7.3 的 S2 各行与实施者自己的**：

| # | 变异 | 命令（抓它的测试） | 结果 |
|---|---|---|---|
| d1 | `UpgradePlaybook` 在 `reviewing` 也答文字（7.3） | `TestUpgradePromptMatrix` | 抓到：`upgrade/reviewing/*` 各列 |
| | | 单元 `TestUpgradePlaybookAuthorizesAndRequiresUpgrading` | 抓到：`reviewing` 一格 |
| | | `TestUpgradePrompt` | 抓到：`in review` 一格 |
| d2 | `UpgradePlaybook` 不判 `lint.read`（7.3） | 单元 | 抓到：调用的次序与"看不到"等格 |
| | | `TestUpgradePromptMatrix`（对照，设计说它看不见） | 照过，照设计：看不到的、删掉的笔记本在 HTTP 的边上由 `Gate`（`node.list`）答 404，到不了用例（设计审查 P3B-M3 第 2 条 = P3C-N6）；看得到的列都有 `lint.read` |
| d3 | `upgrade` 的 prompt 经 `Playbooks.Execute`，`PlaybookText` 收进它（7.3；跨两个文件，`multi.py`） | `TestUpgradePromptMatrix` | 抓到：`upgrade/llm_wiki/*` 答文字、`upgrade/upgrading/*` 答 `notebook.not_llm_wiki` |
| e1 | 只给 `PlaybookText` 的 `switch` 加 `upgrade` 一支，没有调用者（设计审查 P3B-N3 说的等价变异） | `TestUpgradePromptMatrix` | 照过：等价。对照是 d3（同一组格、同一处 `switch` 加上调用者就抓到），照 05-lessons"等价变异的对照" |
| o1 | 没有升级也答文字 | 单元 / 矩阵 / `TestUpgradePrompt` | 都抓到（`no upgrade`；`upgrade/llm_wiki/*`、`upgrade/standard/*`；`no upgrade`、`an LLM Wiki`） |
| o2 | `rolling_back` 答文字（判断改成只拒 `reviewing`） | 单元 / `TestUpgradePrompt` | 都抓到（`rolling back` 与"别的状态"；`rolling back`） |
| o3 | 读升级出错当成"不在状态" | 单元 | 抓到："a read of the upgrade that fails" |
| o4 | 先读升级、后判定 | 单元 | 抓到：调用的次序 |
| o5 | 答 `wiki.upgrade_not_found` | 单元 / 矩阵 | 都抓到 |
| o6 | 答 `ErrUpgradeSubmitting`（同码 `wiki.upgrade_state`，`detail` 是"still being checked"） | 单元 / `TestUpgradePrompt` | 都抓到（单元的 `detail` 一项；整个程序里 message 整值比） |
| o7 | `Get` 交零值的笔记本 | 适配器 / 矩阵 | 都抓到 |
| o8 | `Get` 不交 `description` | 适配器 / `TestUpgradePrompt` / 矩阵 | 都抓到 |
| o9 | `Prompts()` 漏了 `upgrade` | 适配器 / `TestThePromptListsAreTheContract` | 都抓到 |
| o10 | `upgrade` 登记在两个入口 | 适配器 / `TestThePromptListsAreTheContract` | 都抓到 |
| o11 | `Playbooks.Execute` 放过普通笔记本（另一处复写的判断，brief"两处复写的同一判断各自有测试"） | `TestPlaybooksAuthorizeAndRequireMode` / 矩阵 | 都抓到（`standard`；`ingest/standard/*` 等） |
| o12 | `UpgradePlaybookText` 不去首尾空白 | `TestUpgradePlaybookText` | 抓到（测试另断言文件本身带空白，这一格才有意义） |
| o13 | `MaxPlaybookBytes` 缩到 4 KiB（对照：界真的被量了） | `TestUpgradePlaybookText` | 抓到 |
| o14 | `upgrade_submit` 的说明只改了 `tools.go`、契约没改 | `TestTheToolListsAreTheContract` / `TestLintToolObjects` | 都抓到 |
| o15 | `prompts.json` 多一个没有期望的 prompt | `TestThePromptReadsLLMWikiMatrix` / `TestUpgradePromptMatrix` | 都抓到 |
| x1 | 用例答 lint 的 playbook（"答文字的格整值断言文字"） | 单元 / 矩阵 / `TestUpgradePrompt` | 都抓到 |
| x2 | 第一行改掉 | `TestUpgradePlaybookText` | 抓到 |
| s01–s36 | `TestUpgradePlaybookText` 要的 36 句，每句在 md 里每一处都换成 `[removed]`（`multi.py`；`page.upgrade_immutable` 出现两次，两处都换） | `TestUpgradePlaybookText` | 36 条都抓到，各由那一句自己的子测试点名 |

**审查者的变异**（照各自的命名；"存活"的都成了一条发现，等价的除外；规格与输出在 `p3s2/review/a/`、`b/`、`c/`；每条都先在没改的树上跑过同样的命令）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | a1（审阅中而有报告也答文字） | 单元与矩阵存活，`TestUpgradePrompt` 抓到：P3S2A-N1 |
| A | a2（审阅中而没有报告也答文字） | 矩阵抓到，`TestUpgradePrompt` 照过（它的审阅中有报告） |
| A | a3（只给升级的发起人答文字） | 单元、矩阵、`TestUpgradePrompt` 都抓到 |
| A | o6、d3（复跑） | 抓到 |
| A | a4（契约账户入口的 `upgrade_submit` 说明改回旧句）、a5（`testdata/tools.json` 账户入口同一项） | 抓到（`TestTheToolListsAreTheContract`；`TestLintToolObjects/account`） |
| B | d1–d3、e1（7.3 的三行与等价的一支，自己重做） | d1、d3 抓到；d2 单元抓到、矩阵照过（照设计）；e1 照过，等价，对照 d3 |
| B | e2、e2b（`reviewing` 单独答 `ErrUpgradeSubmitting`） | 单元抓到；矩阵、`TestUpgradePrompt` 照过（只比码，单元看得见） |
| B | c1–c4（说明与标题各改一处）、d1′（playbook 加长过 16 KiB） | 抓到 |
| B | u1、u2（没有升级也答文字；`Get` 不交 `description`，`make build` 之后跑 UP9） | 抓到 |
| B | t1–t4（矩阵的期望表与副本的建法写错） | 对没改的代码都失败（期望表与副本的自我核对是活的） |
| B | pb01–pb20（保留片段、建议改成相反） | 20 条都存活：P3S2B-N1 |
| B | 逐句删 30 句 | 21 句抓到，9 句存活（其中 4.13 清单的五项见 P3S2B-M1）；另单删"It checks again and puts the upgrade in review …"也存活 |
| C | c1–c7（playbook 的做法换成错的，条件留着） | 7 条都存活：P3S2C-M1 |

**修复一轮的**（实施者以 `mut.py` 经 `heavy.sh` 跑 `go test -count=1 -run '^TestUpgradePlaybookText$' ./internal/modules/wiki/domain/`，a1 跑单元表那个测试；逐句删除用 B 的 `b/sentdel.py`（30 段，按". "后接大写字母切）；先在没改的树上跑，从备份复原，每轮之后工作目录 0 处改动；规格与结果在 `p3s2/fix1/`）：

| 变异 | 改前（`d2512eeb`） | 提交 1 之后（`94db07e4`） | 提交 2 之后（`8321734b`） |
|---|---|---|---|
| C 的 c1–c7 | 7 条活 | 7 条死 | 7 条死 |
| B 的 pb01–pb20 | 20 条活 | 20 条死 | 20 条死（pb02、pb16 的原文随提交 2 改写，在新原文上做同一种改法） |
| B 的逐句删除里照过的 sd05、sd10、sd13、sd26、sd27a | 5 条活 | 5 条死 | 5 条死 |
| A 的 a1（单元） | 活 | 死 | 死 |
| 盖新措辞的五条：闸门 4 改回"with type and summary"、去掉"and AGENTS"、`server_busy` 只说 `upgrade_submit`、去掉 `forbidden` 的分句、`forbidden` 改成"wait and call it again" | — | — | 5 条死 |
| 合计（`spec.json` 33 条、`spec2.json` 38 条） | 33 条活 | 33 条死 | 38 条死 |
| 逐句删除（30 段） | 9 段照过 | 30 段都失败 | 30 段都失败 |

第 1 段（第一行）删掉时由第一行的判断抓到，没有具名的格。

**修复核对一轮的**（规格与结果在 `p3s2/review/a/check1/`、`b/check1/`、`c/check1/`）：A 在头上重跑 a1，单元失败于"reviewing, reported"，`TestUpgradePrompt` 照旧失败于审阅中（矩阵照过，同第一轮：矩阵种的审阅中没有报告）。B 在头上重跑 pb01–pb20（pb02、pb16 换成新原文）都失败，逐句删除 30 段与按". "、"; "切的逐分句删除 52 个都失败；另加 n1（"Pages you created during the upgrade may be deleted."之后加一句"An original node that is empty may be deleted too."）照过（P3S2B-N3）、n2（一句重复一次）失败、n3（闸门 4 去掉"and AGENTS"）失败、n4（"全过再提交"一句挪到开头）照过（带着条件，意思没变，不算问题）、n5（`forbidden` 一句改成"wait and call it again"）失败。C 在头上重跑 c1–c7，都由自己那一格点名失败。

## 核对

| 轮 | 对象 | 发现 | 处置 |
|---|---|---|---|
| 1 | `7daa3452`（修复一轮 3 个提交），三位各在自己的工作树里；C 另核文档稿（`try.diff`，在 `d2512eeb` 的副本上起草，打到 `7daa3452` 的 `docs/v0.1` 上核） | A：P3S2A-N1、N2 关；a1 在 `d2512eeb` 上单元照过、在头上单元失败；闸门 4、`forbidden`、`server_busy` 的新措辞与 `linking/app/lint_scope.go:218-238`、两个工具的码表（`permission_matrix_mcp_test.go:138-141`）、`mcpserver/errors.go:204-206` 的 `retry_after_seconds` 一致；`94db07e4`、`8321734b` 各自检出 build、vet、wiki 单元都过；没有新发现。B：P3S2B-M1、N1 关（逐句删除 30 段、逐分句 52 个、pb01–pb20 在头上都失败；47 格各恰好一次；4.13 的六项各有一格）；`94db07e4`、`8321734b` 各自检出 build、vet、wiki 三个包的单元、`make lint-go` 都过，头上组合根五个（竞态）与 `make build` 之后 UP9、LW11 都过；新的 P3S2B-N2（Nit：实施报告写 46 格，`8321734b` 与头上是 47）、P3S2B-N3（Nit：测试不看有没有多出来的句子，新加一句相反的话照过）。C：P3S2C-M1、M2、N1–N6 关，N7 在文档稿里关；c1–c7 改前活、改后死；47 格合起来覆盖第一行之外的整段正文；三个提交逐个为绿；文档稿的 3.24 取舍、10.1 的第二次 `main`、第三份的五个样本（S1 修复一轮的 401 秒原来没记 CPU，从作业日志查明是 EPYC 7763）、决定 299 的移交、两份移交与总体设计的句子核过；新的 P3S2C-N8（3.17 的块漏了 `forbidden` 那半句，字节数与片段数没改）、N9（3.20 的块还是初稿，3.17、3.19 都已改在原处）、N10（决定 299 与 M12 第 40 条的理由"改模板要动 LW1 等读模板的断言"不实：LW1 只比 `AGENTS` 的第一行，`TestTheTemplatesCorrespond` 比两种语言的结构，没有断言钉这一句），都是 Nit，都在文档稿 | P3S2B-N2、P3S2C-N8–N10 在本提交：3.17 的块与 `8321734b` 的 playbook 逐字相同（4,979 字节），"必有的句子"写 47 格、覆盖第一行之外的全文；3.20 的块照 `7daa3452` 的 K 改在原处；决定 299 与第 40 条的理由只写模板是建成之后的样子。P3S2B-N3 不改：这个测试钉的是"该有的句子在"，新加的文字由审查看 diff。没有 Important、Minor 留下，核对到此为止 |

## 实测

- **`go test -race -list`**（头上，经 `heavy.sh`；`m11/tools/shards8-p3.py`，S2 没改模式）：553 个顶层，八份 145、50、120、120、61、15、30、12，每个恰在一份；与 S1 末的 551 个比只多 `TestUpgradePrompt`、`TestUpgradePromptMatrix`，都在第七份。`make -n test-go` rc=0。审查 B 以 `make -n test-go-bootstrap` 展开的八条命令自己归份，相同。
- **第三、七份本机两轮**（头 `d2512eeb`，`shards/measure.sh`：每份每轮一次经 `heavy.sh` 的 `make -j1 test-go-bootstrap-<n> GO_TEST_RACE="go test -race -count=1 -timeout 15m -v"`，交替 3、7、3、7，取包时间；每轮之前 `df` 46G，负载 0.9–1.5）：

  | 份 | 第 1 轮 | 第 2 轮 | 均值 | 之前的本机数 | 折到 EPYC 7763 的 Test 一步 |
  |---|---|---|---|---|---|
  | 第三份（120 个） | 236.92 | 231.74 | 234.33 | P2 末 238.0、233.7（S1 没量第三份） | 约 400 秒（P2 的"7763 的 Test 一步 / 本机的包"之比 1.71） |
  | 第七份（30 个） | 191.82 | 190.39 | 191.10 | S1 头 182.8、185.9，均值 184.35 | 约 331 秒（S1 的包对包之比 1.566 再加约 32 秒的竞态构建；照 S1 的 Test 一步对本机之比约 334 秒） |

  第七份比 S1 多 6.8 秒：两个新测试 `TestUpgradePromptMatrix` 2.59、2.58 秒，`TestUpgradePrompt` 1.05、0.94 秒，合 3.6 秒（设计 7.5 估约 6 秒），其余约 3 秒是起伏。第三份里改写的 `TestThePromptReadsLLMWikiMatrix` 1.94、1.92 秒（78 格同前），`TestThePromptListsAreTheContract` 0.83、0.84 秒，S2 对第三份几乎没有增减。
- **playbook 与新测试**：playbook 去掉首尾空白 4,827 字节（`MaxPlaybookBytes` 16,384），同设计 3.17。竞态下（1 号的核对）`TestUpgradePromptMatrix` 2.50 秒（208 格加两份副本与三个应用；设计估约 4 秒），`TestUpgradePrompt` 0.93 秒（估约 2 秒）。内置说明照旧（S2 不改它）：笔记本入口 raw 3,225、SDK 3,276 字节，账户入口 3,294、3,346，距 4,096 余 820、750（C 实跑相同）。
- **e2e**：`make e2e` 529 个（S1 末 528，多 UP9）：UP9 55 ms、LW11 64 ms。
- **修复一轮**（头 `7daa3452`）：playbook 去掉首尾空白 4,979 字节（改前 4,827，界 16,384），`TestUpgradePlaybookText` 47 格（`94db07e4` 上 46 格）；组合根四个（竞态 `-v`：`TestUpgradePrompt`、`TestUpgradePromptMatrix`、`TestTheWikiPromptsAreReached`、`TestThePromptListsAreTheContract`）包 7.8、7.7 秒（1、2 号）；第三份 236 秒（包 235.3 秒，120 个顶层）、第七份 189 秒（包 188.3 秒，30 个顶层），测试集合与 S2 末相同，时长同建成时的两轮；`make e2e` 529 个、133 秒。核对时 B 在头上数得 47 格、各恰好一次，C 用脚本把 47 个片段从正文里挖掉，剩下的只有第一行与句间的标点、空白。

## 门禁与 CI

本机（PostgreSQL 18 在 Docker 里，要 Docker 的与计时的都经 `heavy.sh`）：

- **逐个提交**（每个提交在它自己的树上经 `heavy.sh` 跑一个脚本，逐步记 rc、用时与之前的 `df`，都是 47G 左右；日志 `logs/c1/`、`logs/c2/`、`logs/c3/` 的 `summary.txt`）：

  | 提交 | build+vet | `make lint-go` | `make gen-check` | `make test-go-short` | `lint-web`、`knip`、`test-web` | 它自己的 |
  |---|---|---|---|---|---|---|
  | `567f1201` | 0 | 0 | 0 | 0（40 秒） | 0、0、0（57 秒） | wiki 各包、`page/adapter/mcp`、`mcpserver` 的测试；组合根（竞态）16 个顶层（四处 prompt 测试、读工具表与契约的、覆盖的两个与 `TestTheWikiPromptsAreReached`），16.9 秒；`make build` 之后 MC1、LW8（三种浏览器）、LW11 共 74 个 |
  | `9d7e9004` | 0 | 0 | 0 | 0（40 秒） | 0、0、0（56 秒） | `make build` 之后 UP9、LW11、MC1 |
  | `d2512eeb` | 0 | 0 | 0 | 0（40 秒） | 不碰 | `check_links.py origin/main`（没有新坏的链接） |

  审查 B 在 1、2 号上各自检出另跑一轮：1 号 `go build ./...`、`go vet`、wiki 单元与 `archtest`、`make gen-check`、`make lint-go`、`pnpm run check:format`、`make test-go-short`、组合根竞态 11 个顶层；2 号 `make lint-web`（含 e2e 的 `tsc`）、`make knip`、`make build` 之后 UP9 与 LW11；都 rc=0。3 号只改 README 与人工清单，没有测试读它们。
- **建成时的头**（`d2512eeb`，`gates.sh`，日志 `logs/gates/`，一项接一项，每项之前 `df` 46G），一轮全绿：

  | 门禁 | 结果 | 用时 |
  |---|---|---|
  | `make lint gen-check knip build`（经 `heavy.sh`） | rc=0 | 9 秒 |
  | `heavy.sh make test` | rc=0（vitest 193 个文件、4,447 个测试；Go 各包全过；批准的守卫 117.5 秒，六格中位数 1,081–1,196 ms，界 3.9 s；发起的守卫中位数 1.10–1.13 s；没有偶发） | 3,491 秒 |
  | `heavy.sh make test-go-short` | rc=0 | 41 秒 |
  | `heavy.sh make e2e` | rc=0，529 个 | 136 秒 |
- **修复一轮**（`fix1/verify.sh`，各检出、各经 `heavy.sh`，日志 `fix1/logs/`；每步之前判 `df`，都是 45G）：`94db07e4`、`8321734b` 各自 `make lint-go`、`make gen-check`、`make test-go-short`（40 秒）、`wiki/...` 的单元、组合根四个（竞态 `-v`）都 rc=0，提交之前先跑过 `make lint-go`（0 issues）；`8321734b` 另 `make build` 之后跑 UP9、LW11（2 个过）；`7daa3452` 只改 README 与人工清单，在头上的门禁里核。头 `7daa3452` 的门禁（`fix1/gates.sh`，日志 `fix1/logs/gates/`）：`make lint gen-check knip build` rc=0（8 秒）；经 `heavy.sh`：`make test-go-short` rc=0（41 秒），第三份（`make -j1 test-go-bootstrap-3`，`-race -v`）rc=0（236 秒），第七份 rc=0（189 秒），`make e2e` rc=0、529 个（133 秒）；照修复说明不跑整轮 `make test`（只改了测试、注释、playbook 的文字与文档）。整轮之前之后工作目录 0 处改动。

CI（作业，括号里是 Test 一步；e2e 是 E2E 一步）：

| 作业 | 43129936469（建成，`d2512eeb`） | 43138853267（修复一轮，`7daa3452`） |
|---|---|---|
| server (race) | 15:03（14:38） | 12:55（12:34） |
| server (bootstrap-1) | 6:22（6:03），Xeon 8370C | 4:54（4:34），EPYC 9V45 |
| server (bootstrap-2) | 4:10（3:50），EPYC 9V45 | 6:33（6:14），EPYC 7763 |
| server (bootstrap-3) | 7:19（6:52），EPYC 7763 | 5:05（4:42），EPYC 9V45 |
| server (bootstrap-4) | 5:13（4:43），EPYC 9V45 | 5:36（5:16），Xeon 6973P-C |
| server (bootstrap-5) | 4:32（4:14），EPYC 9V45 | 4:29（4:07），EPYC 9V45 |
| server (bootstrap-6) | 6:36（6:14），EPYC 7763 | 6:32（6:14），EPYC 7763 |
| server (bootstrap-7) | 5:44（5:25），EPYC 7763 | 4:02（3:43），EPYC 9V45 |
| server (bootstrap-8) | 4:48（4:27），EPYC 9V74 | 3:49（3:30），EPYC 9V45 |
| server (costs-1) | 15:31（12:43），EPYC 9V74 | 20:11（16:42），EPYC 9V74 |
| server (costs-2) | 24:50（24:30） | 25:00（24:33） |
| server (costs-3) | 17:12（16:52） | 13:32（13:12） |
| server (costs-lint) | 8:07（7:48） | 10:30（10:06） |
| server (guards-core) | 11:37（11:18） | 19:53（19:27） |
| server (guards-sources) | 16:20（16:01） | 11:50（11:29） |
| server (short) | 3:39（3:17） | 3:59（3:39） |
| web | 4:30（3:58） | 4:44（4:11） |
| e2e | 9:12（7:35） | 8:05（6:26） |
| image | 1:30 | 1:33 |

- 建成时一次全绿，19 项（`p3s2/ci.out`；CPU 照作业日志的"Runner CPU"一步，`ci/`），普通推送（新分支，`d2512eebec20ab02549103e47c738b09d0ed425b`，rc=0），`ci-wait.sh` 等到第 1 次 attempt。八份的 Test 一步 363、230、412、283、254、374、325、267 秒，包 331.3、209.2、378.2、261.0、233.1、340.9、291.6、240.4 秒，都在 450 秒之下。S2 碰的两份都在 EPYC 7763 上：第三份 412 秒（本机折的约 400 秒；S1 的分支与合并之后 411、393 秒），第七份 325 秒（本机折的约 331 秒；S1 的分支与合并之后 322、317 秒，多的 3–8 秒与本机多的 6.8 秒相合；审查 B 从作业日志核过起止）；最贴近的是第三份（余 38 秒，P3S2B-Q1）。costs-1 在 EPYC 9V74 上，批准的守卫六格中位数 1.64–1.80 s（界 3.9 s；第 298 条看 7763 上的数，这一次不在 7763 上）。e2e 一次过，"Install the Playwright browsers"照常装好，529 个，L1 没有出。
- 修复一轮：普通推送（`d2512eeb..7daa3452`，`fix1/push.txt`），`ci-wait.sh`：run 43138853267 的第 1 次 attempt 19 项一次全绿，没有重跑（`fix1/ci.out`；CPU 由主持者的文档稿从作业日志取）。八份的 Test 一步 274、374、282、316、247、374、223、210 秒，都在 450 秒之下；第三份这一次在 EPYC 9V45 上（282 秒，不是 7763 的样本），第七份在 9V45 上 223 秒；第二、六份在 7763 上，各 374 秒（包 340.2、340.9 秒）。costs-1 在 EPYC 9V74 上 20:11（Test 一步 16:42），批准的守卫六格中位数 2,209、2,262、2,410、2,258、2,197、2,139 ms（界 3.9 s；第 298 条看 7763 上的数，这一次不在 7763 上）。e2e 一次过，8:05（E2E 6:26），L1 没有出。

S2 开工之前 `main` 的两次 CI（详见 [P3 设计](../03-P3-review-approve-playbook.md) 10.1 的"合并之后"）：S1 合并的 `daf14dd1`（run 43112344486）与记合并号的 `d33b4544`（run 43117118697）都一次全绿；后一次 costs-1 在 EPYC 7763 上，批准的守卫六格中位数 2.23–2.37 s，都在 3.25 s 之下，第 298 条不触发、界不改。S2 从 `d33b4544` 开出。

## 环境

实施者每个提交的核对与门禁之前判 `df`（46–47G），没有修剪缓存；`GOTMPDIR`（`~/.cache/nw-gotmp-p3s2`）已删；这一 Step 没有手动起容器（库都由测试经 testcontainers 起、收）。三位审查者的重的运行都经 `heavy.sh`（A、B 每次之前看 `df`，45–46G；C 没有跑重的）；A 的 `GOTMPDIR`（`~/.cache/nw-gotmp-reva`）已删；三个工作树最后都回到 `d2512eeb`、`git status` 干净、没有 `*.mutbak`，B 没有留下容器。修复一轮：每步之前判 `df`（45G），`GOTMPDIR`（`~/.cache/nw-gotmp-p3s2`）用完删掉，变异与核对的每一轮之后工作目录 0 处改动，普通推送、没有改写已推的历史。修复核对一轮：三个工作树最后都在 `7daa3452`、`git status` 干净、没有 `*.mutbak`；B 核对时 `df` 44–46G；C 逐个提交检出之后回到头。

合并之后 `main` 的第一次 CI：（待填）。
