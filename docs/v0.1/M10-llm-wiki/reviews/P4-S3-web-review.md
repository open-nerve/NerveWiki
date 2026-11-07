# M10/P4 S3（网页）：审查记录

| 项 | 内容 |
|---|---|
| 对象 | 分支 `m10-p4-s3`，base `78a093e9`。前 3 个提交（`9ce12f44`、`c4b2baea`、`ec6569cb`）由 Claude 从 Codex 没有编译过的补丁做成，之后 10 个是审查的修复 |
| 方法 | 三位独立审查者，各在自己的 worktree 里，从分支构建真实服务，在 Chromium、Firefox、WebKit 里探查，改坏代码。A 管网页的正确性，B 管契约与目标语法，C 管测试、变异、完整度与工程。修复之后由原审查者核对 |
| 结论 | 数据与竞态没有问题，缺陷集中在焦点、滚动和窄屏。发现两处真实的产品缺陷（A-I1，目录的行打开页面后主栏一片空白；B-M1，落点提议守卫会拒绝的根下保留页）和一处随机的 CI 失败（C-I3）。全部处置 |

S3 建成了四件事：目录页面与普通笔记本的入口，树上的两条虚拟行，读者的虚拟链接，历史的 ingest 标记与筛选。P4 §4 是设计，这份记录只写审查。

## 发现与处置

提交的简写：`2f45b60d` 落点，`d3e1eb75` 目录的焦点与滚动，`e71e9c2c` 变量改名，`fe2af458` 落点的测试，`bcd5144d` 落点的文案，`d7cafe08` Reopen 的焦点，`b40ce609` Clear 与补的测试，`4d711984` e2e，`fb966353` Clear 的测试，`9bdf58c9` 两处整洁。

| 编号 | 级别 | 发现 | 处置 | 提交 |
|---|---|---|---|---|
| A-I1 | Important | 目录的行没带 `arrived`，长目录下方点一行，地址换了，滚动没有归位，焦点落到 body，视口里只有左栏 | 行带 `arrived`；`takeArrivalFocus` 在标题还在视口上方时补一次滚动，因为 WebKit 不滚到靠近文档顶部的标题，裸页面上也这样 | `d3e1eb75` |
| A-M1 | Minor | 375 宽下长的类型、标签、来源名把整页撑到 960–1437 | 下拉外层与标签、来源 `li` 加 `min-w-0` 和 `[overflow-wrap:anywhere]`，LW13 加 375 的故事 | `d3e1eb75` |
| A-M2 | Minor | More 读到最后一页时，无类型的行插进上方的组，焦点落到屏外（top -1168） | 落到条目时不再 `preventScroll`（决定 157） | `d3e1eb75` |
| A-M3 | Minor | More 失败之后自动重试读到最后一页，焦点落到屏外的标题 | 已有第一页时不自动重试（决定 156） | `d3e1eb75` |
| A-M4 | Minor | "清除筛选"按下后变成 disabled，焦点掉到 body | 改 `aria-disabled` | `d3e1eb75` |
| A-M5 | Minor | "重新打开目录"随列表离开，焦点掉到 body。核对时发现带 `arrived` 进入的条目修不到：effect 只依赖 `[arrival]`，true 到 true 不跑 | 链接在点击时把焦点交给标题，两种入口走同一条路 | `d3e1eb75`、`d7cafe08` |
| A-M6 | Minor | 换类型后下拉在读取期间收缩成两项，键盘连按跳不到 | 类目用上一次读到的，它与筛选无关 | `d3e1eb75`、`d7cafe08` |
| A-M7、C-I2 | Minor、Important | W08 的滚动断言在 jsdom 里比 0 和 0，不可能失败，"不抢滚动"这一腿没有任何测试能判。A 定 Minor，C 定 Important，取高的 | 删掉这句，LW13 加"读者滚动时 More 读完"的一腿 | `d3e1eb75`、`4d711984` |
| A-N1、A-N3 | Nit | 空笔记本也说"没有符合筛选的页"；树没读到时说"父页不可用" | 分一句文案；未读到时显示"…" | `d3e1eb75` |
| A-N6、B-N1 | Nit | `index-server.ts` 的注释与交叉类型过时；`decision116`、`decision 120` 查不到出处 | 删除，出处改成 M10 design 4.9。`changeset-page.tsx:212` 的判断同时管前面的空格，不改 | `2f45b60d`、`d3e1eb75` |
| A-N2、A-N4、A-N5、A-Q2、A-Q3 | Nit、Q | 未解析链接的 `aria-haspopup`、英文单数、过长 `type` 的重试、上方新增的行没有可见反馈、普通笔记本的 Ingestion 筛选 | 写进 M12 [`M5-polish.md`](../../M12-release/handoffs/M5-polish.md) 第 35 条 | 文档提交 |
| A-Q1 | Q | 后续页失败要不要自动重试 | 不要，见决定 156 | — |
| B-M1 | Minor | 落点提议根下的保留页，写者确认后守卫答 409，与 00 §4.9 矛盾 | LLM 本里答 `not_resolvable`，只有根下的保留标题才读模式（决定 158） | `2f45b60d` |
| B-M2 | Minor | Go 表的 `virtual` 列没有断言 | 再以标准本跑一遍，互相核对 | `2f45b60d` |
| B-M3 | Minor | 共用样例缺保留键作父页的行 | 加 `index/child`、`../index/child`；核对时发现 B 建议的行抓不住 `up > parentDepth`，再加 `../index/child` 在根下的一行 | `2f45b60d`、`fe2af458` |
| B-N2 | Nit | 全空白的长类型每行多一个空的 `<p>` | 行上也判 `trim() !== ""` | `d3e1eb75` |
| B-Q1 | Q | 00 §4.9 的落点这半句这个 Step 做不做 | 做，见 B-M1 | — |
| B2-M1 | Minor | 核对时发现：没有标题的落点不读模式，这一点没有测试 | 落点的表加两行，整个程序的测试加 `AGENTS` | `fe2af458` |
| B2-M2 | Minor | 核对时发现：`not_resolvable` 的文字对新原因不成立 | 两种语言各加一个分句，注释同步 | `fe2af458`、`bcd5144d` |
| A2-M1 | Minor | 核对时发现：Clear 的判断里只应用了类型这一支没有测试 | `P4/clear_focus` 改成两腿 | `fb966353` |
| C-I1 | Important | LW13 的"读者"断言在页面加载之前就过了 | 先等标题和树里的行，LW10 同样改 | `4d711984` |
| C-I3 | Important | WebKit 的 preload 警告不在页面监视的口径里，LW4 随机器快慢失败，CI 不重试。分支 CI [42337052922](https://github.com/open-nerve/NerveWiki/actions/runs/42337052922) 真的红在这里 | 只对 WebKit、`/assets/`、这句原文放行 | `4d711984` |
| C-M1 | Minor | 两条测试读目录比目录出现早一拍 | 先等目录 | `b40ce609` |
| C-M2 | Minor | 共用样例 `rooted-up` 钉不住根位置 | `parents` 改成 `["a"]` | `b40ce609` |
| C-M3 | Minor | 筛选组合有三个分支无人判 | 加 `P4/filters_together`、`P4/tag_follows`。写这条时发现只有未应用的文字时按 Clear 也换地址，目录重读，More 读到的页全部收起，已修 | `b40ce609` |
| C-M4 | Minor | 模式改变后的出路没测 | `P4/reopen` 断言地址、筛选和新读，加 `P4/crossed_mode` | `b40ce609` |
| C-M5 | Minor | 读屏文字、空结果、树读失败没有断言 | `P4/route` 断言文字，加 `P4/empty`、`P4/tree_failure` | `b40ce609` |
| C-M6 | Minor | "焦点落到第一个新项"没被钉住 | `P4/focus` 末页加两行 | `b40ce609` |
| C-M7 | Minor | 目录 label 截断没测 | 加 `P4/long_label` | `b40ce609` |
| C-M8 | Minor | 历史里选 ingest 后的播报没测 | W05 断言 `Listed: 1.` | `b40ce609` |
| C-M9 | Minor | Make 与 README 的说明过时 | 改写 | `4d711984` |
| C 的 Nit | Nit | LW10 的恒真断言、LW4 多余的 focus、`browser.ts` 的 `new URL` 位置、不在码表里的 `problem(403)`、lifecycle 的测试名 | 都改了 | `b40ce609`、`4d711984` |
| C 的 Nit | Nit | W05l（`replace`）、W05p（`ingest=0`）、W09k（`Source unavailable`，fake 走不到）没有测试 | 不补，都是小分支 | — |
| C 的 Nit | Nit | 提交范围词：`9ce12f44`、`c4b2baea` 含 server、api，题目只写 `web:`；`ec6569cb` 含 CI 与文档 | 已推送，不改写，记在这里 | — |
| C-Q1 | Q | 决定 155 在 git 的文档里搜不到 | 写进 00 第 11 节，§10.3 记成偏离，矩阵第 23 行已改 | 文档提交 |
| C-Q2 | Q | §8.3 写五片用时 | 按六份写，见下文 | 文档提交 |
| C-Q3 | Q | `type_key` 是否算偏离 | 是，Codex 的决定 116，记在 §10.3 | 文档提交 |

修 A-M5 和 C-I1 时各走了一段弯路，留个记录。A-M5 第一次只加了 `state={arrived}`，原席核对时才发现另一种入口没修到。C-I1 的控制第一次没有失败，追下去确认产品没问题（`guest` 加笔记本 `editor` 能看到 Edit），是我的变异改到了同一文件里 API 故事的那一行。这条写进了 [05-lessons](../../claude_to_codex/05-lessons.md) 第 6 节。

## 修复核对

三位原审查者都核对了两轮，第一轮在 `e71e9c2c`，第二轮在 `4d711984`。

- **A**：两轮都从分支构建真实服务，在三种引擎上用原来的探针重跑。第一轮 M-5 未关闭：带 `arrived` 进入的条目焦点仍掉到 body。第二轮两种入口乘键盘、鼠标，读取延迟 700 ms，分三个时点看焦点，都在标题上。`takeArrivalFocus` 的副作用实测了搜索命中、从树的低处删页、Back 回长页，都没有问题；Back 恢复滚动位置，`scrollIntoView` 没有抢走阅读位置。新发现一条 Minor：Clear 的判断里只应用了类型这一支没有测试，`fb966353` 补上。
- **B**：两轮都有变异和真库矩阵。第一轮 930 行差分，变化的 64 行全是 LLM 本里根下保留标题的落点，普通本 0 行，`AGENTS`、`raw` 的落点不变。新发现两条 Minor（B2-M1、B2-M2），B-M3 差一行。第二轮全部关闭，Go 与 TS 的差分 1,943,708 行 0 处不一致。B 另判断决定 158 的端口没有破坏决定 120 要防的东西：依赖方向、权限和码的次序都没变，唯一新增的是一次按主键读笔记本，只有根下保留标题的落点才读。
- **C**：在 `4d711984` 上重跑了全部变异，3 条 Important、9 条 Minor 全部关闭，没有新的 Important 或 Minor。新提 3 条 Nit：两处整洁已修（`9bdf58c9`），`load-more` 落到标题时的 `preventScroll` 与来源路径的换行没有测试，留着。C 另逐提交跑了 8 个修复提交，结果见下文。

## 主持者的决定（负责人可以改判）

**决定 154，不装证据脚手架；决定 155，全部故事只跑 Chromium，S3 相关的跑三种引擎。** 见 [00 第 11 节](../00-M10-design.md#11-负责人确认的决定)。全量跑过一次三种引擎的结果在 M12 的 `M5-polish.md` 第 33 条。

**决定 156，目录的后续页失败不自动重试；决定 157，More 落到条目时随焦点滚动。** 见 [00 第 11 节](../00-M10-design.md#11-负责人确认的决定)。

**决定 158，落点读模式，复用 `not_resolvable`。** 这条改了 Codex 的决定 120 的后半句，请负责人留意。修 B-M1 时我不知道有这条：它只在本机的证据目录里，写的是"不添 `target_is_virtual` 或 GetLinkLanding mode port"，直到写这份记录时才查到。查到之后对照 00 §4.9，那里写的是"落点也用它，不再提议建"，而 B 在真库上证实了不改的后果，所以保留修复，理由与改判的做法写在 [00 第 11 节](../00-M10-design.md#11-负责人确认的决定)。

**Codex 的决定 116、120、123。** S3 的补丁沿用了这几条，负责人都可以改判。116 给目录的每项补必填可空的 `type_key`，取自第一次排序的键。120 的前半句是读者点虚拟链接时由网页先导航，后半句见决定 158。123 是目录与历史的到达焦点随各自的补丁安装。原文在本机的证据目录，不在 git 里。

## 变异

C 定义了 129 个变异：vitest 108 个，Playwright 21 个，每个 Playwright 变异单独构建，三种引擎都跑。

初审时 vitest 杀 71、存活 37，Playwright 杀 14、存活 7。存活的分两类。等价的，比如 W03c（去掉 NFC，差分 41 万个目标 0 处不一致）、W04j、W06b、W06c、W07b、W05m，留着。真缺口都进了上面的表。

修复时我对每条新测试做了控制，把修复改回去，确认它失败：

| 改回 | 失败的测试 |
|---|---|
| 行不带 `arrived` | `P4/open_row`，LW13 |
| 去掉 `takeArrivalFocus` 的补滚动 | LW13，只在 WebKit（Chromium、Firefox 本来就滚） |
| Reopen 只用 `state` | `P4/reopen` 的 `arrived=true` 一腿 |
| 已有第一页时照常重试 | `P4/later_retry` |
| 类目用当前这次的 | `P4/catalogue_stays` |
| 条目也 `preventScroll` | LW13（-1248） |
| 去掉下拉外层、标签的换行 | LW13 的 375 故事 |
| `disabled` 代替 `aria-disabled`、去掉判断 | `P4/clear_focus`，LW13 |
| 空笔记本、空白类型、父页未读 | `P4/empty`、`P4/blank_type`、`P4/parent_pending` |
| 落点的模式判断永不成立、没有标题也读模式 | `TestALandingLeavesTheGeneratedFilesTitlesAlone` |
| Clear 总是换地址 | `P4/tag_follows` |
| W05h、W05i、W05j、W07e、W07f、W07g、W08f、W05o2、G12、G13、W10g | 对应的新断言 |
| EW16（More 后 `scrollTo(0,0)`） | LW13，三种引擎 |
| EW19（页面故事的读者改成 editor） | 旧写法 9 次全过，新写法 9 次全败 |

C 在 `4d711984` 上重跑：vitest 124 个，杀 94、存活 30；Playwright 27 个，杀 23、存活 4；Go 5 个全杀。存活的里面 16 个是等价的，10 个由另一层判住（比如 jsdom 判不了的滚动由 EW16 判），剩下 7 个没有测试，都接受：

| 编号 | 改坏 | 为什么留着 |
|---|---|---|
| W05l、W05p、W09k | 筛选不用 `replace`、`ingest=0` 也当筛选、`Source unavailable` 说成已删 | 小分支，不补 |
| W04k、W04l | 登录移出键、`current` 不比登录 | 换登录时 `AppProviders` 重新挂载；测试已改名，不再声称这一点 |
| N12、EW17b | 落到标题时去掉 `preventScroll` | 旧代码，只有末页为空才走到 |
| N16 | 来源路径不换行 | 修复本身有效（A 的探针），要量链接本身才判得到，页面宽度被旧的超宽盖住 |

## 门禁与 CI

本机在 `4d711984` 上：`make lint gen-check knip build test-web test-go-short` 都过，`make e2e` 494 条全过。这次 Firefox 与 WebKit 进了 CI，e2e 任务从 5:27 到 8:12，限额 20 分钟。

| 提交 | CI | 结果 |
|---|---|---|
| `ec6569cb` | [42333903353](https://github.com/open-nerve/NerveWiki/actions/runs/42333903353) | 17 个作业全绿，这一次 LW4 碰巧过了 |
| `d3e1eb75` | 42336999073 | 取消，被下一次推送顶掉 |
| `e71e9c2c` | [42337052922](https://github.com/open-nerve/NerveWiki/actions/runs/42337052922) | e2e 红在 LW4 WebKit 的 preload 警告（C-I3），其余全绿 |
| `4d711984` | [42339911909](https://github.com/open-nerve/NerveWiki/actions/runs/42339911909) | 17 个作业全绿 |

I-3 的修法按 CI 的形状（`taskset` 4 核、`--workers=2`、`\(page\)` 重复 3 次）验证：修前 C 量到 2/3 失败，修后我跑两轮、C 跑一轮，都是 105/105；C 摘掉放行之后又失败。

C 逐提交跑了 build、vet、lint-web、lint-go、knip、test-go-short、gen-check 和 test-web：

| 提交 | 结果 |
|---|---|
| `9ce12f44`、`c4b2baea`、`ec6569cb` | 全过（初审） |
| `2f45b60d`、`e71e9c2c`、`fe2af458`、`bcd5144d`、`d7cafe08`、`b40ce609`、`4d711984` | 全过 |
| `d3e1eb75` | lint-web 不过，只差 `no-shadow` |


`d3e1eb75` 那一条：第一次改名也撞了名，那一版已经推上去。我试过用 `--force-with-lease` 改写分支，被权限拦下，于是留着这个提交，加了一个普通的提交 `e71e9c2c`。`fb966353` 只改一个测试文件，`9bdf58c9` 在门禁里，这两个没有逐个跑。

## 分份的数

`4d711984` 这一次分支 CI 的 Test 为 461/361/400/421/344/345 秒。第一份三次都在最慢的 EPYC 7763 上：423、438、461，单次分支样本不算线，规定的两次 main 见 [00 第 12 节](../00-M10-design.md#12-phase-进度表)。

S2 记录留下的两次 main：[42331529264](https://github.com/open-nerve/NerveWiki/actions/runs/42331529264) 的 Test 为 421/427/329/405/354/439 秒，[42333629450](https://github.com/open-nerve/NerveWiki/actions/runs/42333629450) 为 424/359/265/415/307/270 秒，均值 422.5/393/297/410/330.5/354.5，都低于 450。

## 接受与推后的

- **Safari 的两处待人工确认**：375 宽的搜索页宽到 704（SE1），第二个标签页登出之后另一个标签页的事件流数没有归零（C8）。都在 M12 的 `M5-polish.md` 第 33 条。
- **过度设计的提醒**：`current()` 与 `active()` 的重复检查、`more.failure` 这一行、`preventScrollReset`、`limit` 属性、`virtual-target.ts` 里的几处等价检查，都留着。`more.failure` 与其他列表同形，其余无害。`owner.unload` 删了。
- **375 宽下树展开把标题推出视口**：旧机制，搜索命中也这样，写进 `M5-polish.md` 第 35 条。
- **WebKit 的 preload 警告为什么出现**：只知道消息点名入口模块，空闲单 worker 不复现，触发条件没有查清。放行的条件很窄，在 210 次 WebKit 页面故事里没有再失败。
- **没有用真实读屏软件。**
- **`useLoadMore` 的其余五处列表**（决定 157）只沿代码走过，实测了目录、历史、回收站。全量的 494 条故事都过了。
- **`load-more` 与 `arrival.ts` 的改动波及别的页面**：C 的 Playwright 变异只盖 LW8、LW9、LW10、LW12、LW13，其余靠全量故事。
- **Reopen 的 `preventScroll`**：读者滚过标题再按时，标题在视口上方约 116 像素。A 判定不必动。
