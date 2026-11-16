# M12/P4/S4 页面树的渲染：设计 & 实施规划

| 项 | 内容 |
|---|---|
| Step | M12/P4/S4 页面树的渲染（P4 的条件修复，P4 设计 3.9；一个 Step，分支 `m12-p4-s4`） |
| 日期 | 2026-11-16（初稿；同日第一次、第二次修订） |
| 状态 | 设计已审查（2026-11-16）：三份审查、两次修订、两轮修订核对，见 [审查记录](reviews/P4-S4-design-review.md)；决定 487–505 在 [00 号文档](00-M12-design.md) 第 11 节 |
| 上级文档 | [M12 总设计](00-M12-design.md) 4.7.2、4.7.5、第 7 节"次序与并行"、第 8 节、决定 329、378；[P4 设计](04-P4-performance.md) 3.9、4.3、第 8 节第 5 条、第 9 节（两种候选）、10.2（P4/S2 的结论）；[P2 设计](02-P2-web-polish.md) 4.1 的 12.3、16.1、28.5（S1）、4.6.6、4.6.7 的 12.12；[v0.1 总体设计](../v0.1-design.md) 13.2 第 17、25 条、第 14 节"大笔记本（上万页）"一行 |
| 代码位置的写法 | 同 P4 设计：前端写作 `W/pages/notebook/page-tree-item.tsx:120`（`W/` 是 `web/apps/web/src/`）；e2e 写作 `e2e/perf/tree.perf.ts`。行号是 `main` 的 `99def961` 的（到 `46f9b05c`，本文引的网页文件都没变）。P2/S1 的分支 `m12-p2-s1`（未合并，审查时头是 `14cc9769`）改到的同一处按内容另写明，不钉提交 |
| 工作笔记 | 设计做了四个探针（经 `heavy.sh`，临时 worktree 在 `~/.cache/m12-p4s4-design/`，用完删了；探针、读数与没进正文的考虑在 scratchpad 的 `m12/p4s4/draft-notes.md`）：(1) 原型：照第 3 节改左栏的行与首页的列表（没做 `through` 与菜单的 `aria-labelledby`），`tsc` 为零，`W/pages/notebook/`、`W/pages/page/`、`W/app/` 的 1,757 格 vitest 全过；(2) 用 P4/S2 的 `e2e/perf/tree.perf.ts`（`m12-p4-s2` 的 `1a612abd`）在同一台机器上量改前、只分批与原型的五种组合，平的形状各一轮，选定的组合三种形状两轮（1.3）；(3) 4.1 的三格守卫与 4.2 的一格写成 vitest，在原型与 `main` 的代码上各跑一次（6.2）；(4) 第一次修订：平的形状上开关一行的菜单、关一个对话框，比原型、初稿的写法与改后的写法（3.4）；(5) 第二次修订：在原型（套审查 A 核对时的变体）上核对菜单的触发怎样量、行在哪个 effect 里说自己离开，vitest，各有反向对照（3.4、4.2）。三位审查者另做的探针与复跑见审查记录 |

---

## 0. Step

一个 Step，`m12-p4-s4`，改网页，另给 `e2e/perf/` 加两个只记的量：

- **左栏的行变便宜**（3.2–3.5）：
  - 行是 `memo` 的组件，属性都是原始值，由列表（一个 `observer`）算好给它；
  - 链接是普通的 `<a href>`，点击由树交给路由；
  - 一棵树一个菜单，开在被按的那一行的按钮处；那一行"菜单开着"经行自己登记的句柄告诉它，只有它重渲染；它离开树时菜单关上；
  - 对话框留在行里，选了菜单项才挂上；
  - 拖放的注册照旧每行一个。
- **列表先画首屏，其余分批**（3.6）：
  - 每个页的列表（左栏的每一层、首页的根页、页的子页、标签页）第一次渲染 100 行，之后每帧画完再加 400 行，直到全部；全部行最终都在 DOM 里。
  - 画全之后一次多出超过一批的，照样分批。
  - 树要把焦点放到还没画出的一行（或它的后代）时，路上的列表立刻画到它。
- **首页的列表同样**（3.7）：`SubpageList` 的行是 `memo` 的普通链接，一个点击处理在列表上，同样分批。
- 左栏与首页**列出什么不变**：仍是整棵树、全部根下的页；不加虚拟列表、不加"更多"，不需要负责人确认（2.3 的决定 487）。
- **缩小规模的守卫**：vitest，1,000 页平的形状上的结构断言，不计时（4.1）：
  - 第一次提交的行数、每一批的行数有上限；
  - 别人改一次树只渲染新的一行，MobX 的 reaction 只跑几个；
  - 行里没有自己的菜单与路由的链接；
  - 画全之后一次多出一千行照样分批。
- **在原规模上再量**：P4/S2 的 `tree.perf.ts`，同样的终点、方法、种子与两轮，三种形状（6.3）；另记全部行进 DOM 的时刻与平的形状里开关一行的菜单。

**开工的时机**（2.3 的决定 497）：P2/S1 合并之后从 `main` 开。

- P2/S1 改了同一块：左栏的 `NavLink` 与树里两个 `Link` 带 `state={opened}`，`subpage-list.tsx` 的 `Link` 带 `state={arrived}`。
- S4 把它们换成自己的点击处理时带上同样的状态，P2/S1 的测试随之钉住 S4 的写法（5.2）。
- 再量用 `e2e/perf/`：P4/S2 那时还没合并的，代码的提交照做，`probe.ts` 的提交与再量等它合并。
- 占"同时在实施的 Step 不多于两个"的一个名额（00 号文档第 7 节）。

**工作量**（推断）：

- 原型的改动是 `page-tree-item.tsx` +308/−202 行、`page-tree.tsx` +14/−3、`subpage-list.tsx` +41/−8、分批的钩子 35 行（不含测试与量的脚本）。
- 正式的实施再加：
  - `through` 认后代、画全之后的增长分批；
  - 菜单的 `aria-labelledby`、行的"菜单开着"句柄、行离开时关菜单、右键点到外面时不还焦点；
  - 点击的 `replace`；
  - 4.1、4.2 的约二十五格测试；
  - `probe.ts` 的一个量。
- 一个实施者一到两个工作段（审查 C 估第一次修订多约半个工作段，不出这个范围）。
- 在原规模上再量约 15 分钟机器时间（两轮三种形状，原型里各 3.7 分钟，加构建；菜单一项在每轮平的形状里、T2 之后），经 `heavy.sh`。

**不在本 Step**：

- 数据一侧（树的读、比较、索引）：分层加载是 v0.2 的事，P4 设计第 9 节。
- 5 万、10 万页：P4/S2 的 4.9 量、由它的文档提交写进 P4 设计 10.2 与总体设计第 14 节那一行；若进已知限制的清单，那一条归 P4/S3。
- "移到……"对话框：深的形状 1,258 ms，没有界线，P2/S6 的 12.12 与 4.6.6 的附带观察。打开它的路随 S4 改（树的菜单经行的句柄挂上对话框，3.5），对话框本身不改。
- 树的 JSON 压缩：部署文档，P5/S3。

## 1. 基线

### 1.1 P4/S2 的结论（`main` 的 `99def961`，P4 设计 10.2）

| 形状 | T1 ≤ 1,000 ms | T2 ≤ 1,500 ms | T2 的最长长任务 ≤ 200 ms | T3 ≤ 500 ms |
|---|---|---|---|---|
| 平（1 万页都在根下） | **3,015**，超 | **1,609**，超 | **1,429**，超 | — |
| 深（11,111 页） | 198 | 61 | 0 | 212 |
| 带附件 | 210 | 66 | 0 | 211 |

两轮的较大中位数（P4/S2 的表；T3 是它修过之后重量的，见下）；平的三格复核两轮照样超。

**T3 是重量的**（P4/S2 代码审查 B 的 M1）：P4/S2 原来量 T3 时，`000 big` 已被 T2 建的五页挤到第 6 行，20 次 T3 视口里都没有它的孩子，量的不是"孩子画出"（那时是 231、236 ms）。P4/S2 的修复把 T3 挪到 T2 之前、断言视口里有孩子（`5a76031a`），深与带附件各重量两轮：深 212.4、206.4，带附件 211.3、209.6 ms（每次视口里 6 个孩子；P4/S2 的 `tree-report.md`"T3 重量"），结论不变。上表与本文的 T3"改前"用的是它们。原型的 T3（1.3 的约 30 ms）是在旧的条件下量的，作废，随改后的 `tree.perf.ts` 再量（6.3）。

另记：平的第五次 T1 之后 JS 堆 604 MB（深 17.4 MB），T1 之内最长的长任务 2,254 ms，文档里 20,003 个 `li`（左栏 10,000、首页正文 10,000）。

### 1.2 原因：渲染一侧，每一行太贵，而且一次画两份一万行

**证据**：

- 树的答复约 150 ms 读完（`responseEnd` 145–166 ms），之后一个约 2.25 秒的长任务，画出在 3.0 秒（P4/S2）。数据一侧（读、`sameNodes`、`indexTree`）不在剖面的前列。
- 一次 T1 的 CPU 剖面（P4/S2 的 `diag/flat-profile-top.txt`，100 µs 采样，带 source map）。**限定**：这是 P4/S2 作废的旧探针那一次（`tree-diag`，13:09–13:10，在 `1a612abd` 之前），P4/S2 的报告注明了。按包分（审查 B 重分过）：
  - react-dom 约 828 ms；
  - react-router 加 `URL` 约 221 ms（每行的 `NavLink`：`useResolvedPath`、`createBrowserURLImpl`、`URL`）；
  - Radix 约 415 ms（`react-context` 147、`react-presence` 73、`react-slot` 46……；每行的 `DropdownMenu` 与四个对话框）；
  - GC 约 321 ms；
  - DOM 的原生调用约 233 ms（其中 `addEventListener` 128 ms）。
  - 剖面里的"排版约 390 ms"（自身 391.6 ms）：旧探针的 `filled` 把这次排版强迫进了提交的微任务；正式的探针里它落在提交之后的那一帧，仍在 T1 之内（改前那一轮 `results/base`：提交 `met` 2,470–2,572 ms，T1 3,077–3,184 ms，提交之后到画出 600–620 ms，就是两万行的样式、排版与绘制；审查 B 的核对）。P4/S2 的报告 `tree-report.md:46`：换探针之后平的 T1 没变，变的只是这次排版落在哪里。
- `addEventListener` 是谁（读源码，审查 B 核对过）：
  - Radix 的 `Menu` 根每挂一个，就在 `document` 上加一个捕获的 `keydown` 监听者（`@radix-ui/react-menu@2.1.24` 的 `dist/index.mjs:58-71`）。一万行就是一万个，之后页面上每按一个键都要跑一万个。
  - 拖放的注册不加 DOM 的监听者：`draggable` 与 `dropTargetForElements` 只进登记表、设 `draggable` 属性，文档级的监听者全局只有一份（`@atlaskit/pragmatic-drag-and-drop@4.0.0` 的 `element-adapter.js:25-35`、`:252-287`）。
  - P4/S2 的报告推断是拖放，以源码为准改正。
- 每一行今天挂的（`W/pages/notebook/page-tree-item.tsx:80-170`、`:250-375`）：
  - 一个 `observer` 的 `PageItem`（`useWorkspace`、`useT`、`useId`、`useState`、一个拖放的 `useEffect`）；
  - 一个 `NavLink`（订阅路由的位置，每次导航一万个都重渲染）；
  - 一个 `observer` 的 `PageMenu`（`useParams`、`useStore`、`useState`，每次渲染算一次 `subtreeOf`，`:274`）；
  - Radix 的 `DropdownMenu`（根、触发、内容的 `Portal` 与 `Presence`）；
  - 一直挂着的四个对话框（`SourceAddDialog`、`RenamePageDialog`、`MovePageDialog`、`ConfirmDialog`，各一个 Radix `Dialog` 根）。
  - 首页正文的 `SubpageList` 再列一万个 `Link`（`W/pages/notebook/notebook-home.tsx:66`、`W/pages/page/subpage-list.tsx:24-31`）。
- **T2**：树重读之后 `nodes` 整份替换（`W/stores/page-tree.store.ts:296-305`），`tree` 这个 computed 随之是新的。三条路都让每一行重跑：
  - 每个 `PageItem` 都经 `useDistinctName`（`:178-181`）、`childrenOf`、`isSource` 读它，一万个 `observer` 全部重跑，连同各自的菜单与对话框；
  - `PageList` 给每行新的 `node` 对象；
  - `PageTree` 读 `pages.tree`、每次渲染新建上下文（`page-tree.tsx:64`、`:116-124`），经属性交给每一行。
  - 首页的一万个 `Link` 也重渲染。一个 1.3–1.5 秒的长任务就是它（原型的 A/B：行便宜之后 T2 从 1,674 到 193–260 ms；守卫在 `main` 上数出 1,001 次行渲染）。
- **深与带附件为什么过**：根下 11 行，一屏之内；展开 1,000 个孩子（T3）约 206–212 ms（P4/S2 重量的，1.1 的注），同样的行，规模小一个量级。

### 1.3 探针：原型的几种组合（平的形状，同一台机器）

量法：

- 用 P4/S2 的 `tree.perf.ts` 原样量（同样的种子、形状、终点、五次取中位），只加两样只记的：
  - 页里的初始化脚本告诉原型用哪种组合；
  - T1 之后再等左栏与首页的列表都到一万行，记那一帧的时刻（"全部行进 DOM"，rAF 轮询看到行数够了的那一帧，还没画出）与 T1 之后到它之间最长的长任务。
- 每种组合平的形状一轮。改前在同一个下午在 `main` 的代码上再量一轮：各格比 P4/S2 的四轮略高 1–9%（T1 3,111 对 3,002–3,097，T2 1,674 对 1,532–1,637），量级相同；那一轮只一次，开始时负载 3.85（锁之前别人的）。
- 单位 ms，每格是五次的中位数（五次的数在 `draft-notes.md` 第 3 节）。
- 堆是全部行进 DOM 之后、GC 之后的 `JSHeapUsedSize`。

| 组合 | T1 ≤ 1,000 | T1 之内最长的长任务 | T2 ≤ 1,500 | T2 的最长长任务 ≤ 200 | 全部行进 DOM | 其间最长的长任务 | 堆 |
|---|---|---|---|---|---|---|---|
| 改前（`main` 的代码，14:06 再量） | 3,111，超 | 2,334 | 1,674，超 | 1,488，超 | 提交（`met`）约 2,490，画出即 T1 | — | 602 MB |
| 只分批（今天的行，首页的 `Link` 也分批） | 212 | 56 | 1,532，超 | 1,356，超 | 4,790 | 180 | 621 MB |
| 行便宜（左栏的行，3.2–3.5） | 1,245，超 | 601 | 260 | 109 | 1,304 | 0 | 91 MB |
| 行便宜，加首页的列表便宜（3.7） | 1,153，超 | 589 | 193 | 103 | 1,212 | 0 | 70 MB |
| 同上，加 `content-visibility: auto` | 905 | 417 | 241 | 156 | 940 | 0 | 70 MB |
| **同上（不加 `content-visibility`），加分批（3.6）** | **208** | **0** | **211** | **114** | 3,022 | 100 | 79 MB |
| 同上，再加 `content-visibility: auto` | 190 | 0 | 255 | 162 | 2,821 | 82 | 80 MB |
| 审查 B 复跑选定的组合（18:40，负载 0.50） | 187 | 0 | 190 | 101 | 2,645 | 72 | 79 MB |

读法：

- **只分批**：T1 过（首批便宜），T2 与它的长任务照样超。重读之后一万个 `observer` 的行连同菜单与对话框全部重跑，与分不分批无关。全部行进 DOM 要 4.8 秒，各批之间最长的任务 176–198 ms（中位 180），贴着 200 ms；堆照旧 621 MB。
- **行便宜**拿掉了 T2 的大头：别人改一次树，从 1,674 ms 到 193–260 ms，最长的长任务从 1,488 ms 到 103–109 ms；堆从 602 MB 到 70–91 MB。
- **T2 剩下的约 100 ms 长任务在提交之后**（审查 B 的读数，`review-b/t2-summary.txt`）：
  - 首页也便宜的各组共 35 次里，新行进 DOM（`met`，提交之后的微任务）多在答复之后 78–116 ms（33 次；另两次 250、308 ms，`all-home-batch-2`、`flat-home-batch-cv` 的第 1 次）；
  - 最长的长任务 63–173 ms，与"画出 − `met`"只差 2–8 ms，35 次没有例外；
  - 长任务只取开始在答复之后的，而 35 次里 31 次最长的长任务比答复到 `met` 的这一段还长，放不进提交之前：所以它落在提交与画出之间，那一帧的样式、排版与绘制。
  - 推断：两份一万行的列表最前插一行之后的排版（左栏的 `ul` 是 `flex flex-col gap-0.5`）。没有做带 trace 的剖面。
  - 对照：行照今天的两组，最长的长任务在 `met` 之前，就是重渲染。
- **只有行便宜，T1 过不了**：一次提交两份一万行，约 1.15–1.25 秒。`content-visibility` 让视口外的行不排版、不绘制，T1 到 905 ms，离界只有一成，换一台慢一点的机器就超；还让"画出 − `met`"与 T2 的长任务多了约 50 ms，在渲染一侧。
- **分批**把 T1 降到约 0.2 秒（首批 100 行），T1 之内没有长任务。代价是全部行进 DOM 从约 1.2 秒推迟到约 2.6–3.0 秒，各批之间最长的任务两轮的中位数 72、100 ms（五次 69–106）。今天约 3.0–3.1 秒画出（提交在约 2.5 秒）之前什么都看不到；分批之后 0.2 秒看到首屏，约 2.6–3.0 秒全部进 DOM，与今天画出的时刻相当。
- 分批之后再加 `content-visibility`，T1 只快 18 ms，T2 的长任务反而多了约 50 ms，不加（2.4 戊）。

**选定的组合，三种形状两轮**（行便宜、首页的列表便宜、分批；同样的量法，每格两轮的中位数与两轮的较大；单位 ms）：

| 形状 | T1 ≤ 1,000 | T2 ≤ 1,500 | T2 的最长长任务 ≤ 200 | T3 ≤ 500 | "移到……"对话框（只记） |
|---|---|---|---|---|---|
| 平 | 194（194、188），改前 3,015 | 206（206、206），改前 1,609 | 113（113、111），改前 1,429 | — | — |
| 深 | 198（190、198），改前 198 | 65（64、65），改前 61 | 0，改前 0 | **30**（28、30；旧条件，作废待再量），改前 212 | 1,331（1,331、1,246），改前 1,258 |
| 带附件 | 203（203、191），改前 210 | 66（66、61），改前 66 | 0，改前 0 | **31**（31、31；同上），改前 211 | — |

- 五次的数在 `draft-notes.md` 第 3 节。三条界线在三种形状上都过，平的每格离界至少 1.7 倍（T2 的长任务 113 对 200）。
- 深与带附件照 2.1 的判法都不算慢：
  - 深的 T2 65 对 P4/S2 的 61，多 4 ms，超过 P4/S2 自己两轮之差（3 ms），在 2.1 允许的 67 ms 之内；
  - 推断是日间的摆动：同一下午在 `main` 上再量的平的形状比 P4/S2 的四轮都高 1–9%。
  - T3：原型因首批只画 100 行，量到约 30 ms，但那是在 `000 big` 被挤出视口的旧条件下量的，作废（1.1 的注）。改前已由 P4/S2 重量（深 212、带附件 211 ms）。原型的 T3 随改后的 `tree.perf.ts` 再量，判法照 2.1。
- "移到……"对话框：较大中位数 1,331 对 1,258，多 73 ms；P4/S2 新旧探针的四轮在 1,188–1,258，原型两轮 1,331、1,246，差在两轮的摆动里。打开它的路随 S4 改，对话框本身不改。
- 这两轮不等全部行进 DOM，堆是第五次 T1 时（平的形状只画了约 900 行）的，不与上表比（6.3 改为全部行进 DOM 之后读）。

## 2. 目标与范围与取舍

### 2.1 目标

- 平的形状：T1 ≤ 1,000 ms、T2 ≤ 1,500 ms、T2 的最长长任务 ≤ 200 ms（00 号文档 4.7.2，同样的终点与判法）。
- 分批自己的界（平的形状，6.3 量）：全部行进 DOM ≤ 4 秒；T1 到全部行进 DOM 之间最长的长任务 ≤ 200 ms。原型是 2.6–3.0 秒与两轮的中位数 72、100 ms（五次 69–106）。
- **深与带附件不变慢**，逐格判（T1、T2、T2 的最长长任务、T3）。基准是 P4/S2 的数（T3 两行是它以修过的 `tree.perf.ts` 重量的两轮，1.1 的注）：
  - 改后两轮的较大中位数，不高于 P4/S2 两轮的较大中位数，加"P4/S2 两轮之差"与"它的 10%"两者中的较大。
  - 超了照 P4 设计 3.9 第 2 条复核两轮，再超就停下报主持者。
  - P4/S2 的两轮逐格（`p4s2/results/tree-round1`、`tree-round2`，即 P4/S2 表的轮 1、轮 2）：

| 格 | P4/S2 轮 1、轮 2 | 较大 | 两轮之差 | 10% | 改后不高于 | 原型（两轮的较大） |
|---|---|---|---|---|---|---|
| 深 T1 | 196、198 | 198 | 2 | 19.8 | 217.8 | 198，不慢 |
| 深 T2 | 61、58 | 61 | 3 | 6.1 | 67.1 | 65，不慢 |
| 深 T2 的最长长任务 | 0、0 | 0 | 0 | 0 | 0 | 0，不慢 |
| 深 T3 | 212.4、206.4（轮 5、6） | 212.4 | 6.0 | 21.2 | 233.6 | 30（旧条件，作废，6.3 再量） |
| 带附件 T1 | 204、210 | 210 | 6 | 21.0 | 231.0 | 203，不慢 |
| 带附件 T2 | 65、66 | 66 | 1 | 6.6 | 72.6 | 66，不慢 |
| 带附件 T2 的最长长任务 | 0、0 | 0 | 0 | 0 | 0 | 0，不慢 |
| 带附件 T3 | 211.3、209.6（轮 5、6） | 211.3 | 1.7 | 21.1 | 232.4 | 31（同上） |

  - 两格"最长长任务"的界是 0，判法同上：两轮的较大中位数过 0 才算慢，即一轮五次里三次以上有过 50 ms 的长任务。一两次偶然的长任务（例如 GC）只记，不复核。
- 页的行为不变（3.8 的清单）；P2/S1 的到达焦点照旧（5.2）。
- 不加依赖。

### 2.2 范围

改：

- `W/pages/notebook/page-tree.tsx`、`page-tree-item.tsx`：行、列表、菜单、对话框的挂法。
- `W/pages/notebook/page-drag.ts`：`useDrag` 移进来并导出（守卫的计数要包它）。
- `W/pages/page/subpage-list.tsx`；`W/pages/notebook/tag-page.tsx` 的一处 `key`。
- 新的 `W/app/shown-rows.ts`（分批）。
- `e2e/perf/probe.ts`、`tree.perf.ts`：两个只记的量，"全部行进 DOM"与平的形状里开关一行的菜单（6.3）；P4/S2 先合并，改的就是 `main` 上的这两个文件。
- 测试；文档（第 8 节）。

不改：`W/stores/page-tree*.ts`、`W/events/`、服务端、`e2e/stories/`。

### 2.3 本文的取舍（决定 487–505，主持者的决定，负责人可以改判）

正式编号 487–505 依次是审查时的临时号 P4S4-1 到 P4S4-19（括号里，审查记录的表用它对照），主持者在设计审查之后给的。

1. **487（P4S4-1）只改怎样画，不改列什么**：左栏仍列整棵树，首页仍列全部根下的页，全部行最终都在 DOM 里。原型不改列出的内容就过了三条界线，平的每格离界至少 1.7 倍（1.3），用不着"更多"或虚拟列表，不需要负责人。
2. **488（P4S4-2）每一行便宜**（3.2–3.5）：
   - T2 与它的长任务只要"别人改树时行不重渲染"：`memo`、原始值的属性、上下文随树的一生不变（3.2）。
   - 一棵树一个菜单、对话框选了才挂，T2 用不着，理由在 1.3 的"只分批"一行：
     - 只分批时各批 176–198 ms，贴着"每片短于 200 ms"；
     - 堆 621 MB；
     - 另有每按一个键跑一万个 `keydown` 监听者（1.2）。
     - 每行一个菜单与四个对话框，是这几样的来处。
3. **489（P4S4-3）先画首屏，其余分批**（3.6）：首批 100 行，之后每帧 400 行；T1 靠它（1.3：只让行便宜，T1 仍 1.15–1.25 秒，2.4 己）。
4. **490（P4S4-4）首页与页的列表同样**（3.7）：首页的列表是另一万行，便宜了 T1 少约 90 ms、T2 少约 70 ms、堆少约 20 MB（1.3 的表里"行便宜"与"加首页的列表便宜"两行）。
5. **491（P4S4-5）不用虚拟列表**（2.4 甲）。
6. **492（P4S4-6）不取"首页只列前若干个加更多"**（2.4 乙）。
7. **493（P4S4-7）不取悬停、聚焦时才建菜单**（2.4 丙）。
8. **494（P4S4-8）拖放照旧每行注册**（2.4 丁）。
9. **495（P4S4-9）守卫是结构的断言，不计时**（4.1、2.4 辛）。
10. **496（P4S4-10）不加 `content-visibility`**（2.4 戊）。
11. **497（P4S4-11）P2/S1 合并之后开**（第 0 节、5.2）。
12. **498（P4S4-12）"菜单开着"与"画到哪一行"不经列表读的盒子**（3.4、3.6）：
    - 行的"菜单开着"由树经那一行登记的句柄告诉它，只有那一行重渲染；
    - 列表的 `through` 只在这个列表还没画全时读（画全之后不订阅）。
    - 初稿让每个列表读一个可观察的盒子，开关一次菜单、移一次焦点，一万行的列表都重算一遍属性。3.4 的探针：这一步比原型多约 6–37 ms 的长任务、41–79 ms 的用时；三种写法的长任务都在 0.4–0.56 秒，过了主持者定的"约 50 ms"，所以取不让整列重算的写法。大头不在列表，是 Radix 菜单的模态（证实），见 3.4 的"模态的菜单与今天的基线"。
13. **499（P4S4-13）菜单开着时它那一行离开树，菜单关上**（3.4）：照 P2/S1 的 16.1，焦点在菜单里时交给 `focusActions`。
14. **500（P4S4-14）"删的是不是正显示的页"每次渲染算**（3.5），同今天。
15. **501（P4S4-15）列表的写法记进总体设计 13.2 的新一条（第 31 条；若 S4 先于 P2/S3 合并，取第 30 条，P2/S3 照改），不放进第 25 条**（第 8 节第 1 条）：
    - 第 25 条的规则是"写者的内容决定数量的地方设上限"，页的列表不设上限，放进去与那一条的规则相反；
    - 新的一条写明它是第 25 条的例外与理由（读者要能走到每一页），代价靠什么兜住。
16. **502（P4S4-16）树的菜单保持模态**（3.4）：平的一万页里开一行的菜单约 0.5 秒的长任务，大头是模态（证实）；S4 不比今天慢（开 542 对 702 ms），打开菜单不在界线里，不扩范围；"不模态"作 v0.2 的候选（第 9 节）。
17. **503（P4S4-17）菜单开着时树变了，触发重新定位，不关菜单**（3.4，审查 A 的 C1-M1）：别人的改动不该把用户正在用的菜单关掉；重新定位只在菜单开着时、每次提交之后量一个按钮。
18. **504（P4S4-18）笔记本没了、编辑没存时，左栏照最后找到的那一本**（3.2，审查 A 的 C1-M2）：同今天。
19. **505（P4S4-19）总体设计 13.2 的新条按合并的先后取号**（第 8 节第 1 条，审查 C 的 C1-I1）：P2/S3 与 S4 都在 13.2 加一条；先合并的取第 30 条，后合并的取第 31 条，后合并的那一个在自己的文档提交里改自己的设计、决定与 00 号文档第 7 节里的号。

### 2.4 比较过、不取的

- **甲、虚拟列表**（只渲染视口里的行，P4 设计第 9 节候选 1）：
  - 左栏不自己滚动，随整页滚（`W/pages/workspace/workspace-layout.tsx:111-113`），要按窗口虚拟化（例如 `@tanstack/react-virtual` 的 `useWindowVirtualizer`，MIT，新依赖）。
  - 打开的文件夹要压成一张平的行表（每行带深度），嵌套的 `ul` 与展开按钮的 `aria-controls` 指向的那个 `ul` 就没了：要么改成 ARIA 的 tree/treegrid（`aria-level`、`aria-posinset`、`aria-setsize`、方向键），要么丢掉分组的语义。今天的设计是"没有 ARIA tree，行是 Tab 走过的链接"（`page-tree.tsx:29-34`）。
  - 看不见的行不在 DOM 里：浏览器的页内查找找不到它们，读屏的链接列表只有视口里的几十个，Tab 走到视口边就出了列表；拖放拖到视口边要自动滚动，放到没画的行上要先算位置。
  - 树要聚焦的那一行（`page-tree.tsx:73-79` 的 `[data-actions-of]`）可能没画：先滚过去。P2/S1 的 28.5"焦点在树的链接上的，后退时不动"：那个链接滚出视口就被卸下，焦点掉到 `body`。
  - jsdom 没有排版，虚拟列表在 vitest 里一行都不画或只画缓冲的几行，按名字找行的测试（`page-tree.test.tsx`、`page-tree-writes.test.tsx`、`page-tree-drop.test.tsx`、`source-uploads.test.tsx` 等）要另给尺寸；e2e 里不在视口的行要先滚。
  - 好处是 5 万、10 万页时 DOM 不随页数长；那时数据一侧（2.4 MB 一万页的 JSON）本来也要分层加载，留给 v0.2 一起做。
- **乙、首页只列前若干个加"更多"**（P4 设计第 9 节候选 2）：改的是首页列什么，是产品的事，要负责人；单靠它左栏仍有一万行。原型里首页的列表照样列全部也过了界线，不需要。
- **丙、悬停或聚焦时才把这一行的按钮换成 Radix 的触发**：
  - 换的那一刻按钮是另一个 DOM 元素（组件类型变了，React 重挂），Shift+Tab 从下一行退到这个按钮时焦点随之丢掉，要再放回。
  - 触屏的 `pointerenter` 与 `pointerdown` 在同一个任务里派发，换之前按下就落在旧按钮上，仍要一份自己的打开逻辑（推断，没在浏览器里试）。
  - 一棵树一个菜单没有这两件事。
- **丁、拖放委托到列表上**：pragmatic-drag-and-drop 按元素找放下的目标（从事件的目标往上找登记过的元素），委托要在列表上自己按坐标算是哪一行、哪一种放法，重写 `dropOperations` 的接法。每行的注册不加 DOM 的监听者（1.2），代价是一万个 effect 与登记，分批之后摊在各批里。不值。
- **戊、`content-visibility: auto`**（行与首页的 `li`，CSS，不改 DOM，页内查找与读屏照样看得到视口外的内容）：不分批时 T1 到 905 ms，离界一成；分批之后只快 18 ms，T2 的长任务多约 50 ms（1.3）。不取。
- **己、只让行便宜、不分批**：T2 过，T1 1,153–1,245 ms 不过（1.3）。一次提交两份一万行，DOM 的创建、样式与排版本身就过了 1 秒。
- **庚、只分批、行照今天**：T1 212 ms 过，T2 1,532 ms、它的长任务 1,356 ms 超（1.3）；全部行进 DOM 4.8 秒，各批的任务 176–198 ms，堆 621 MB。T2 的原因（重读时每一行重跑）分批碰不到。
- **辛、计时的守卫**：
  - jsdom 没有排版与绘制，计时量不到 1.2 的大头；
  - e2e 里一万页要先导入（P4/S2 每种形状 24–28 秒），CI 的机器上 1 秒的界摆动大。
  - 结构的断言抓得住两个原因（一次画全部、重读时全部重渲染，后者连同行里的 `observer`，4.1 第 2 格的 reaction 计数）与一个已知的贵东西（每行一个菜单）回来；原规模的计时在 6.3 照 P4/S2 的方法手跑。
- **壬、React 的 transition 与时间切片**（`startTransition` 包住列表的渲染）：React 让出主线程的单位是组件，一次提交插一万行，提交本身切不开；之后浏览器的样式与排版更切不到。证据（审查 B 从 1.3"行便宜，加首页的列表便宜"那一组 `flat-home` 的五次算）：读完树的答复到提交（`met − nodesEnd`）393–411 ms；提交之后到画出 570–607 ms，与 T1 之内最长的长任务 555–593 ms 只差 13–15 ms。所以那一组的 589 ms 是提交之后那一帧的样式与排版，不是 React 的提交，transition 只能把 React 的渲染切成小段，碰不到它。不取。

## 3. 网页

### 3.1 文件

| 文件 | 改什么 |
|---|---|
| `W/pages/notebook/page-tree.tsx` | 树的上下文只放随树的一生不变的东西（3.2）；一个菜单与它离开时的关闭（3.4）；跟随链接（3.3）；`focusing` 照旧，列表的 `through` 由各自的 computed 给（3.6） |
| `W/pages/notebook/page-tree-item.tsx` | `PageList` 算每行的属性；`PageItem` 换成 `memo` 的 `PageRow`，登记打开对话框与"菜单开着"的句柄；`PageMenu` 拆成树的菜单与行的对话框（3.4、3.5） |
| `W/pages/notebook/page-drag.ts` | `useDrag` 移进来并导出（4.1 第 2 格要包它计数） |
| `W/pages/page/subpage-list.tsx` | 行是 `memo` 的普通链接，列表上一个点击处理，分批（3.7） |
| `W/app/shown-rows.ts`（新） | `useShownRows`（3.6） |
| `e2e/perf/probe.ts` | `coldOpen` 加只记的"全部行进 DOM"（6.3） |
| `W/pages/notebook/tag-page.tsx` | `SubpageList` 加 `key={tag}`，换标签就重挂、从首批开始（3.7） |
| `e2e/perf/tree.perf.ts` | T2 之前先等全部行进 DOM（6.3）；平的形状在 T2 之后加只记的菜单一项：开一行的菜单、`Escape`、关它的对话框（3.4 的量法，6.3） |
| 测试 | 4.1、4.2 |

### 3.2 行与列表

- `PageList`（`observer`）读 store，为每个孩子算好行要的东西，交给 `PageRow`：
  - `id`、`name`（`node.name`）、`label`（`distinctName(...) ?? node.name`，按钮的可访问名用）、`href`（`${home}/pages/${id}`）、`depth`；
  - `hasChildren`、`open`（`hasChildren && pages.isOpen(id)`）、`active`（与 `NavLink` 相同：这一页是地址里的页，3.3）；
  - `writer`、`agents`、`sourceRoot`、`source`、`canDrag`（`canManageNode`）。
  - 不给 `node` 对象：重读之后每个节点都是新对象（`nodes` 整份替换），给了就每行都重渲染。
  - 不给"菜单开着"：它不经列表（3.4）。
- `PageRow` 是 `memo`（浅比较）的普通组件，不是 `observer`，不读 store：别人改一次树，只有属性变了的行重渲染（新的一行；在它前后的行属性不变）。
- **树的上下文**（`TreeContext`，`page-tree.tsx:116-124`）今天每次渲染新建一个对象，交给每一行，会让 `memo` 失效。改为只放随树的一生不变的东西，`useMemo` 只依 `pages` 与 `home`（换了笔记本才重建）：
  - `pages`、`heading`（ref）、`home`；
  - `lastNotebook`（ref）：`PageTree` 的属性 `notebook`，`PageTree` 每次渲染时写；只作下表笔记本一行的后备（不叫 `held`：那是对话框的持有，3.5 的 `held.onClosed`）；
  - 几个 setter：`fail`、`focusActions`、`openMenu`、`go`（`useNavigate` 在数据路由里是稳定的）；
  - 句柄的登记表：`rows`（每行登记"打开哪个对话框""菜单开着"两个函数，3.4、3.5）。
- **渲染时要读、读了要跟着变的值，不放进上下文**，各走可观察的路或作属性（审查 A 的 M2）：

| 值 | 谁读 | 怎样得来 |
|---|---|---|
| 笔记本（`writer`、`sourceRoot`、`distinctName` 用） | `PageList`、`PageDialogs`、`TreeMenu`（都是 `observer`） | `pages.notebook ?? lastNotebook.current`。`pages.notebook`（`W/stores/page-tree.store.ts:185`，从工作区的列表按 id 找，可观察）：列表重读或收到一本之后是新对象，读它的组件随之重渲染。它没有时（笔记本被删或读不到，而这个标签页正编辑它的一页没存：`NotebookLayout` 照旧用最后找到的那一本留着外壳与左栏，`W/pages/notebook/notebook-layout.tsx:63`，M5/P4 3.9），`pages.notebook` 变成 `undefined` 这件事本身让它们重渲染，读到后备 `lastNotebook.current`，即 `NotebookLayout` 留着的那一本。同今天：今天的树用的就是这个属性（审查 A 的 C1-M2，决定 504） |
| 删除的确认里的期限 | `PageDialogs` | 自己从 `useStore()` 的 `instance.info?.trash_retention_seconds` 与 `preferences.locale` 算（都可观察）；`useSWR("instance")` 照旧在 `PageTree` |
| 新建页在途（`sending`） | `TreeMenu` | 作属性，`PageTree` 给 |
| `pageId`（`active`） | `PageList` | `useParams()`，同今天 |

- **用到时再读的**（只在事件处理与 effect 里读，不在渲染时读）：`pages` 的方法（`toggle`、`move`、`remove` 等）、`heading.current`、`fail`、`focusActions`、`go`、`rows`。
- 行的 DOM 与今天相同（`li > div.group[ref=row] > 展开按钮或占位 + 链接 + 菜单按钮 + 放下的指示`，打开时再一个嵌套的 `ul`），P4/S2 的探针（`li > :first-child`）与 e2e 的 `treeRow`（链接的父元素，`e2e/fixtures/wiki-pages.ts`）照用。
- **导航**：列表读 `useParams`，导航时为已画的行重算属性，`memo` 之后只有新旧两个当前行重渲染。重算本身多少没有量（推断与 3.4 的探针同量级：一万行的列表重算一遍属性，比原型多 6–37 ms 的长任务、41–79 ms 的用时；`tree.perf.ts` 不量导航）。今天点一页要重渲染一万个 `NavLink`，不会更慢。

### 3.3 链接

- 普通的 `<a href={href}>`，`aria-current={active ? "page" : undefined}`，类名照 `NavLink` 的两种（`page-tree-item.tsx:124-130`），`draggable={writer ? false : undefined}` 照旧。
- 点击：列表上一个 `onClick`，按 `closest("a")` 认行；照 react-router 的 `useLinkClickHandler`：
  - 只管主键、没有修饰键（Ctrl、⌘、Shift、Alt）、`target` 空或 `_self`、事件没被阻止的点击；其余（新标签页打开、中键）交给浏览器；
  - `preventDefault()`，`go(href, { state: opened, replace })`（`go` 就是 `navigate`）：`opened` 是 P2/S1 的 28.5 加的（`W/app/arrival.ts`）；
  - `replace` 在当前地址的路径、查询与 hash 连起来等于 `href` 时为真（`createPath(location) === href`），与 `Link` 的缺省相同（`react-router@8.4.0` 的 `dist/development/lib/dom/lib.js:615`）。所以在 `pages/Notes` 上再点 Notes 换掉这一条历史；在 `pages/Notes#h`、`pages/Notes?x=1` 上点是新的一条，后退回得到锚点（审查 A 的 R6）。
  - **当前地址在点击处理里算**（审查 A 的 C1-M3）：列表的 `onClick` 是列表这次渲染建的，用列表这次渲染的 `useLocation()`。列表本来就随每次导航重渲染（`useParams`，3.2 的"导航"），所以它总是当下的地址，多读 `useLocation()` 不多一次渲染。不在上下文的 `go` 里算：上下文的 `useMemo` 只依 `pages`、`home`，里面的 `location` 是建上下文那一刻的。
- `active`：`NavLink` 在地址的路径等于或以链接的路径加 `/` 开头时为当前（`end` 缺省为假，不分大小写）。路由里带 `pageId` 的只有 `pages/:pageId` 与 `pages/:pageId/versions/:revision`（`W/app/routes.tsx:158`、`:166`），所以等于"`useParams().pageId` 等于这一行的 id"（页的 id 是小写的 UUID）。
- 不用 `NavLink`、`Link`：每个都订阅路由的位置、解析一次路径、建一个 `URL`（1.2 的约 221 ms），每次导航一万个都重渲染。
- 树里另外两个 `Link`（目录与历史，`page-tree.tsx:150`、`:154`）只有两个，照旧。

### 3.4 一棵树一个菜单

- **行里的按钮**是普通的 `<button>`：
  - `data-actions-of={id}`（树的焦点照它找，`page-tree.tsx:76`）、`aria-label={t("page.actions", { name: label })}`、`aria-haspopup="menu"`；
  - `aria-expanded={expanded}`，开着时 `aria-controls` 是菜单的 id、`id` 给菜单的 `aria-labelledby` 用；
  - 类名照今天，`data-[state=open]:opacity-100` 换成 `aria-expanded:opacity-100`。
- **打开**与 Radix 的触发相同：主键的 `pointerdown`（没有 Ctrl）、`Enter`、空格、`ArrowDown` 的 `keydown`；打开时树记下是哪一行。
- **树的菜单**（`TreeMenu`，`PageTree` 里一个，只给编辑者与管理员）：
  - 一个受控的 Radix `DropdownMenu`，触发是一个看不见的、定位在那个按钮上的元素（`position: fixed`；左栏的祖先没有 `transform`、`filter`、`contain`，固定定位相对视口）。
  - **触发的位置**（审查 A 的 C1-M1，决定 503）：
    - 打开的那一刻在事件处理里量：`openMenu(id, button, keyboard)` 取按钮的 `getBoundingClientRect()`，与 id 一起存进树的菜单状态。不在渲染时量：渲染时这次提交的 DOM 还没变，量到的是旧位置。
    - 菜单开着时树变了（别人在上面加了页、删了页，按钮上下移）：`TreeMenu` 每次提交之后（`useLayoutEffect`，不给依赖数组）从 `[data-actions-of=id]` 当前的那个按钮再量一次，位置变了就更新菜单状态里的位置（画之前同步再渲染一次）。`TreeMenu` 开着时读树（算 `subtreeOf`、`depthOf`，3.5），树一变它与列表在同一次提交里重渲染，这一步就在新行插进 DOM 之后。
    - 取重新定位，不取关上：别人的改动不该把用户正在用的菜单关掉（菜单是模态的，关上焦点要回按钮，用户要重来）；代价只是菜单开着时每次提交量一个按钮。那一行离开了树（删、挪到别处、所在的列表收起），照下面的"菜单开着时它那一行离开树"关上。
    - 菜单经 Radix 的 Popper 跟着触发：触发的 `style` 变了，floating-ui 的 `autoUpdate` 跟上（推断：它的 `layoutShift` 以 IntersectionObserver 看参照元素移动；jsdom 没有排版，量不到）。4.2 的那一格量的是触发的位置等于按钮的位置。
    - 原型核对（第二次修订，`rev2/`，vitest 探针，jsdom 补了 4.2 那一格的 `getBoundingClientRect`）：开 Notes 的菜单，按钮与触发都在 132 px；别处在根下最前加 Atlas、焦点重读之后，菜单仍开，按钮 164、触发 164 px。反向对照（在渲染时量，即第一次修订的写法与审查 A 的核对）：触发留在 132 px，那一格红。
    - 菜单是模态的，开着时页面不滚，不必随滚动重新定位。
    - 跟不上的三种（推断，罕见，只记）：窗口大小变了、左栏变宽变窄；分批还在进行时，排在这一行之前的列表又画了一批，按钮下移；菜单开着时用浏览器的后退、前进（Alt+←、鼠标的侧键，不经菜单）到这本里的另一页，`PageTree` 的 `openTo` 打开新页的祖先（`page-tree.tsx:67-71`），排在这一行之前的文件夹展开、按钮下移（审查 A 的 C2-N2）。这三种 `TreeMenu` 都不重渲染（它不读 `isOpen`，属性也没变），触发留在原处，菜单在那一刻离按钮有偏差，关上再开就对。要碰到第二种，得在冷打开的约 3 秒之内开一行的菜单，而它排在一个还在画的、孩子过百的列表之后。
  - 内容 `align="end"` 照旧，`aria-labelledby` 给那一行的按钮（不给的话菜单的名字是看不见的触发的，空）。菜单项与今天相同（`page-tree-item.tsx:298-318`）。
  - "New subpage"的 `disabled` 读 `sending` 属性（3.2）。
- **"菜单开着"告诉那一行**（决定 498）：
  - 每行在一个自己的 effect 里登记句柄：打开哪个对话框（3.5）、`expanded(open: boolean)`（设自己的 `useState`）、离开（下面的"菜单开着时它那一行离开树"）。
  - 这个 effect 依赖只有上下文、`id` 与 `writer`，与拖放的 effect 分开（审查 A 的 C1-M4）。拖放的 effect 随 `open`、`canDrag` 重跑（今天 `page-tree-item.tsx:200-231`）：登记放在那里，清理就会在那一行展开、收起时跑，把开着的菜单关掉。例如开着 Install 的菜单，别处删了它唯一的孩子，它随之收起，菜单关了。原型核对（`rev2/`）：登记在自己的 effect 里，菜单仍开，`Escape` 之后焦点回到 Install 的按钮；反向对照（离开的清理放进随 `open`、`canDrag` 重跑的 effect）：菜单关了，那一格红。
  - `TreeMenu` 在菜单开、关时调那一行的它：只有那一行重渲染。列表不读菜单的状态。
  - 初稿让列表读一个可观察的盒子算 `menuOpen`，开关一次菜单，一万行的列表都重算一遍属性。探针（第一次修订，平的一万行，全部行进 DOM 之后，五次中位）：

| 写法 | 按下到菜单画出 | 其间最长的长任务 | `Escape` 到焦点回到按钮 | 其间最长的长任务 | 对话框 `Escape` 到焦点回到按钮（`focusActions`） | 其间最长的长任务 |
|---|---|---|---|---|---|---|
| 原型（直接改按钮的属性） | 664.9 | 523 | 516.0 | 429 | 529.3 | 426 |
| 初稿（列表读盒子） | 706.1 | 560 | 594.6 | 443 | 572.6 | 432 |
| 改后（行的句柄） | 688.6 | 542 | 530.3 | 441 | 530.7 | 435 |

  单位 ms。量法：`menu.perf.ts`（设计的探针，不合并；`draft-notes.md` 第 7 节），P4/S2 的准备与长任务观察者，平的形状，同一次构建的三个运行时变体，各一轮。读法：
  - 列表读盒子比原型多：open 的长任务约 37 ms，`Escape` 的用时约 79 ms，对话框的约 43 ms。这就是一万行的列表重算属性的代价（推断：同一构建、同一轮之差），比初稿推断的约百毫秒小。
  - 改后与原型相差在 20 ms 之内，一轮之内的摆动量级。
  - 三种都有约 0.42–0.56 秒的长任务，大头不在列表：见下一条。
- **模态的菜单与今天的基线**（只记；主持者定保持模态，决定 502）：
  - 同一个平的一万页、同样的量法，另量了两轮（`draft-notes.md` 第 7.2 节）：

| 写法 | 按下到菜单画出 | 其间最长的长任务 | `Escape` 到焦点回到按钮 | 其间最长的长任务 | 对话框 `Escape` 到焦点回到按钮 | 其间最长的长任务 |
|---|---|---|---|---|---|---|
| 今天（`main` 的代码：每行一个模态的 Radix 菜单，一次画全） | 824.0 | 702 | 1,089.3 | 619 | 3,535.3 | 1,454 |
| 改后（行的句柄，模态） | 688.6 | 542 | 530.3 | 441 | 530.7 | 435 |
| 改后，菜单不模态（`modal={false}`，只为归因） | 98.5 | 83 | 164.5 | 78 | 506.0 | 406 |

  - **证实**：大头是模态。Radix 的 `DropdownMenu` 缺省是模态的，开着时它：`hideOthers` 给内容之外的兄弟们设 `aria-hidden`；`RemoveScroll` 锁住页面的滚动（给 `body` 加样式）；`DismissableLayer` 设 `body` 的 `pointer-events: none`。关上时撤销（`@radix-ui/react-menu@2.1.24` 的 `dist/index.mjs:137`、`:205`）。同一构建只把菜单改成不模态，开的长任务从 542 降到 83 ms，`Escape` 从 441 降到 78 ms。
  - **推断**：这几样里具体是哪一样让两万个 `li` 的整页重算样式、重排，没有剖。对话框（Radix 的 `Dialog`，也是模态的）关上的约 0.4 秒推断是同一回事，菜单不模态时它不变。
  - **S4 不比今天慢**：开菜单的长任务 542 对 702 ms，`Escape` 441 对 619 ms，关对话框 435 对 1,454 ms（今天关上时 `setFocusing` 让树新建上下文，一万行全重渲染）。所以保持模态：打开菜单不在 4.7.2 的三条界线里，照"超出设计预算才修"不扩范围。
  - "不模态"作 v0.2 的候选（第 9 节），代价写在那里。

- **键盘打开时第一项取得焦点**：Radix 靠菜单的根在 `document` 上的捕获 `keydown` 监听者判断是键盘（1.2 引的那一段），树的菜单的根一直挂着，行上的 `keydown` 先经过它。原型的守卫那一格核对过：在按钮上按 `Enter`，菜单开、第一项（New subpage）取得焦点、`aria-expanded` 为真，`Escape` 之后焦点回到按钮。同一格也看到原型漏了的一件：菜单的 `aria-labelledby` 指着看不见的触发（`radix-…`），行的按钮没有 id，菜单没有名字；所以上一条写明要给。
- **关上**：`onCloseAutoFocus` 里 `preventDefault()`，选的不是开对话框的项时 `focusActions(id)`，焦点回到那一行的按钮（它可能已因移动换了地方，照今天的 `focusActions`）。
  - 例外，同 Radix（`@radix-ui/react-dropdown-menu@2.1.24` 的 `dist/index.mjs:114-124`）：右键（或 Ctrl 加左键）点到菜单外面关上的，不还焦点，焦点留给用户点的地方。`onInteractOutside` 记下是不是这一种。
- **菜单开着时它那一行离开树**（决定 499，审查 A 的 M3）：
  - 那一行卸下时（被删、被挪到别的父页下，或它所在的列表收起），行的登记 effect（上一条）的清理告诉树：菜单开在这一行就关上，清掉菜单的状态。这个清理只在卸下时跑（或 `writer` 变时，那时 `TreeMenu` 本来就随之没有）。
  - 用 `useEffect` 就够，不用 layout effect：菜单在 `TreeMenu` 里，不随行卸下，焦点由关上时的 `focusActions` 给，清理早一步晚一步，落点相同。P2/S1 的 16.1 要 layout 的清理，是因为附件的菜单 portal 随行卸下，这里不是（审查 A 的 C1-M4）。原型核对（`rev2/`，两种登记都一样）：开着 Notes 的菜单，别处删了 Notes，菜单关上，焦点到树的标题 `h2`"Plans"，`nav` 没有 `aria-hidden`。
  - 菜单关上，`onCloseAutoFocus` 照上一条 `focusActions(id)`：页挪走了，新父页开着时焦点到它新的那一行的按钮，收着时到树的标题；页删了，到树的标题（`page-tree.tsx:77` 的后备）。这与 P2/S1 16.1"行离开、焦点在它或它的菜单里，交给这一节的标题"一致，挪走的情形更进一步回到那一页。
  - 所以之后别处恢复同一个 id 的页，菜单不会自己再开（审查 A 的 R3 在原型上红：模态的菜单自己打开，整页 `aria-hidden`）。
  - 今天的 `main`：行卸下带走菜单，焦点落在 `body`。
- **不用 React 的 context 放"开着的是哪一行"**：每一行都读它，就每一行都重渲染。

### 3.5 对话框留在行里，选了才挂

- 每行的四个对话框不再一直挂着。选了"重命名""移到……""删除""粘贴 Markdown""上传文件"之后，树的菜单经 `context.rows.get(id).open(which)` 告诉那一行，行把自己的 `dialog` 状态设上，才挂上那一个对话框；关上之后 `held.onClosed` 照今天移焦点，再卸下。
- `rows` 是树的一个 `Map`：行在 3.4 的登记 effect 里登记自己的句柄（与拖放的 effect 分开）、卸下时删（只删自己登记的那个）。
  - 挪到别的父页下的行是新的一行，状态是新的：**对话框随它原来的那一行卸下**，与今天相同。
  - 旧行的清理先于新行的登记，不会删掉新行的（审查 A 核对过）。
  - `W/pages/notebook/source-uploads.test.tsx:633`（"Reading the moved tree unmounts the old tree row and its dialog"）钉住这一条：原型第一版把对话框放在树上（不随行卸下），这一格红了。
- **"删的是不是正显示的页"**（`page-tree-item.tsx:277`，`shown`）同今天：在 `PageDialogs` 每次渲染时算，关上时用最后一次渲染的值（决定 500）。
  - 为什么成立：对话框在行里，页离开树时行与对话框一起卸下，最后一次渲染时子树还在。
  - 原型第一版"关上时算"错在对话框放在树上，不随行卸下，关上时页已不在树里，子树是空的（`page-tree-writes.test.tsx:396` 那一格红了）。
  - 初稿改成"按确认时记"也错：确认之后答复之前读者去了别处（后退到不在被删子树里的页），按确认时的值仍为真，关上时什么也不做，外壳也不带读者去父页，焦点落在 `body`（审查 A 的 R5 在原型上实测；13.2 第 17 条要控件带着焦点离开时不让它落到页首）。今天的写法用最后一次渲染的值，为假，焦点到树的标题。
  - 审查 A 在原型上改回今天的写法：删除的各格（7 格，含 `:392` 起那一格）与 R5 都过，三个文件 75 格全过。
- `subtreeOf`、`depthOf`（`page-tree-item.tsx:274-275`）不再每行每次渲染算：只在菜单开着（`TreeMenu`）、对话框挂着（`PageDialogs`）时为那一页算。

### 3.6 先画首屏，其余分批：`useShownRows`

```ts
/** How many of a list's total rows to render: first at once, then step more after each paint, until all. While
 * not all are rendered, through is asked how many must be, and those are rendered at once, and stay. Grown by more
 * than step since all were rendered, the rest come a step at a time again from those rendered. */
export function useShownRows(total: number, through?: () => number): number
```

- 首批 `first = 100`：1280 × 720 下左栏的视口约 6 行（P4/S2：树的第一行 top 524 px），100 行盖住任何视口与几屏的滚动；每批 `step = 400`。原型里各批之间最长的任务两轮的中位数 72、100 ms（五次 69–106；1.3，左栏与首页两份列表各一批），离 200 ms 两倍；首批 100 行时 T1 之内没有长任务。
- 推断越往后的批越贵，与 T2 剩下的长任务是同一回事：增长的 flex 列表的排版（1.3）。探针只记 T1 到全部行进 DOM 之间的最大值，没量它落在哪一批；也没有剖。6.3 再量时全部行进 DOM 过 4 秒或其间的长任务过 200 ms（第 7 节的界），先做一次带 trace 的剖面再定改法，不预先设计。
- 一批画完（提交、绘制）之后再加下一批：`requestAnimationFrame` 里 `setTimeout(0)`，下一批在那一帧绘制之后的任务里；卸下时取消。标签页在后台时 `requestAnimationFrame` 停着，回到前台再接着画。
- vitest 5.0.2 的 jsdom 环境缺省 `pretendToBeVisual = true`，`requestAnimationFrame` 在（`vitest/dist/chunks/index.hTNFpC24.js:1354`，项目用的缺省池走的 `setup`）。用假计时器的测试若也假了它（缺省连它一起假），推进时间才走下一批；今天的树的测试都在首批之内（4.3）。
- **画全之后**：
  - 多出的不超过一批（400）：照常一次画（T2 的新行）。
  - 多出超过一批：从上次画到的行数接着分批（审查 C 的 M4）。会碰到的：导入一个平的 zip（导入导出页在笔记本的外壳里，左栏开着），任务结束、事件让树重读，根下多出一万行。
  - 多出的排在已画的行之后时（导入排在已有的页之后：TB2 的 `importBig` 注释"at its root after its pages"），已画的行不动。排在前面时（例如一次 `batch` 在根下最前建四百页以上，`after_id: null`），钩子按 `slice(0, shown)` 取行，已画的行被挤出前 `shown` 行、暂时卸下，下一批再挂上；焦点在那里的会丢到 `body`（审查 C 的 C1-N1；推断，罕见，见 3.9）。
  - 列表重新挂上（收起再展开、换了页）时重新从首批开始。
- **`through`**：树要把焦点放到这个列表里还没画的一行，或它的后代时，列表立刻画到路上的那个孩子（审查 C 的 M1）。
  - 每个列表一个 computed：`PageTree` 的 `focusing`（`page-tree.tsx:62`，改放进一个可观察的盒子）有值时，看它与它的祖先（`pages.ancestorsOf`）里有没有这个列表的孩子，有就答那个孩子的序号加一，否则答 0。
  - 列表只在自己还没画全时读这个 computed（决定 498，审查 C 的 M5）：画全之后不订阅，移一次焦点不让一万行的列表重算属性。"画全没有"在钩子的状态里，所以列表把读它的那一步作函数交给钩子（`useShownRows(total, () => through.get())`），钩子只在没画全时调它（审查 C 的 C1-N3）。钩子在列表的渲染里跑，调它就是列表的 `observer` 订阅了它；不调就不订阅。
  - 抬高的行数留在钩子的状态里，不降回去：`focusing` 在焦点给出之后立刻清掉（`page-tree.tsx:75`），降回去那一行就被卸下，焦点掉到 `body`。
  - 一次提交就够：路上的孩子画出来，它的列表随它挂上（`openTo` 已打开全部祖先，`move-page-dialog.tsx:149`、`page-tree.store.ts:647-651`），新挂的列表第一次渲染就读到同一个 `focusing`、画到下一层，`PageTree` 的聚焦 effect 在这次提交之后找得到按钮。
  - 会碰到的：用"移到……"把一页挪进一个收着的、孩子过百的文件夹的末尾，或挪到那个文件夹排在百名之后的孩子之下。没有这一条，焦点落到树的标题上（`page-tree.tsx:77` 的后备）。原型没有做这一条，4.2 的两格在原型上红（焦点在 `h2` "Plans" 上），在 `main` 的代码上过；只认自己孩子的写法，嵌套的那一格照样红（审查 C 实测）。
- 当前页（地址里的页）不另画到它：今天没有"滚到当前页"，`aria-current` 的那一行在它那一批画出时才有。

### 3.7 首页与页的列表：`SubpageList`

- 用在笔记本首页的根页（`notebook-home.tsx:66`）、页的子页（`W/pages/page/page-layout.tsx:341`）、标签页（`W/pages/notebook/tag-page.tsx:54`）。三处都在有数据之后才挂。
- 行是 `memo` 的 `<li><a href>`，属性是 `href` 与名字；列表的 `ul` 上一个 `onClick`，按 `closest("a")` 认链接，判断同 3.3，`navigate(href, { state: arrived })`。`arrived` 在 `main` 上就有（`W/app/arrival.ts`，`notebook-layout.tsx` 在用），P2/S1 的 12.3 把它加到这里的 `Link` 上。
- 同样 `useShownRows(pages.length)`；页的子页随 `PageShell key={page.id}` 换页重挂，从首批开始。标签页给 `SubpageList` 加 `key={tag}`（`W/pages/notebook/tag-page.tsx:54`），换标签就重挂、从首批开始（审查 C 的 C1-N2）：增长分批按行数算，从画全了的 5,000 页的标签换到 SWR 缓存着的 4,000 页的另一个，行数没有增长，不加 `key` 就是 4,000 个新行一次画出。

### 3.8 行为不变的清单（出处与钉住它的测试）

| 行为 | 今天在哪里 | S4 之后 | 钉住它的测试 |
|---|---|---|---|
| 行是链接，Tab 依次走过；没有 ARIA 的 tree | `page-tree.tsx:29-34` | 同（分批画完之前 Tab 走不到没画的行，3.9） | `page-tree.test.tsx` |
| 展开按钮的 `aria-expanded`、`aria-controls` 指向嵌套的 `ul` | `page-tree-item.tsx:106-116`、`:167` | 同 | `page-tree.test.tsx`；P4/S2 的 T3 的条件也用它 |
| 当前页 `aria-current="page"`，版本页上也是 | `NavLink` | 列表算（3.3） | `page-tree-writes.test.tsx:75`；4.2 新格（版本页） |
| 点开页，焦点留在树上；P2/S1 的 `opened` | `NavLink` | 点击处理带 `state: opened` | P2/S1 的 `arrival.test.tsx`（28.5 的各格） |
| 修饰键、中键交给浏览器 | `NavLink` | 同（3.3） | 4.2 新格 |
| 再点当前页换掉这一条历史；带 hash、查询的是新的一条 | `NavLink` | `createPath(location) === href`（3.3） | 4.2 新格 |
| 菜单的名字、项、键盘打开第一项取得焦点、关上焦点回按钮 | Radix 每行的触发 | 树的菜单（3.4） | `page-tree-writes.test.tsx` 的 `choose`；4.2 新格 |
| 右键点到菜单外面关上，焦点不回按钮 | Radix 每行的菜单 | 同（3.4） | 4.2 新格 |
| 菜单开着时它那一行离开，菜单关上、不再自己打开 | 行卸下带走菜单（焦点 `body`） | 树关上菜单，焦点经 `focusActions`（3.4） | 4.2 新格（R3、R4） |
| 菜单开着时别人改了树，菜单留在那一行的按钮处 | 每行的触发就是按钮 | 打开时量，每次提交之后重量（3.4） | 4.2 新格（审查 A 的 C1） |
| 菜单开着时那一行展开或收起（别人删了它唯一的孩子），菜单不关 | 行的菜单不随展开重挂 | 登记与拖放的 effect 分开（3.4） | 4.2 新格 |
| 笔记本被删或读不到、这个标签页正编辑它的一页没存，外壳与左栏留着到离开编辑 | `notebook-layout.tsx:63`，树用 `PageTree` 的属性 | `pages.notebook ?? lastNotebook.current`（3.2） | `W/pages/page/page-lock.test.tsx` 的两格"a notebook deleted / out of the account's reach while this tab edits a page of it unsaved stays until the edit is left" |
| 编辑者变读者、读者变编辑者，行的"Actions"随之没有、有 | `PageItem` 读笔记本 | 列表读 `pages.notebook`（3.2） | 4.2 新格（R2） |
| 删除的确认里的期限，实例的读晚到也换上 | 上下文每次新建 | `PageDialogs` 自己算（3.2） | 4.2 新格（R1） |
| 对话框随行卸下 | 每行挂着 | 选了才挂，在行里（3.5） | `source-uploads.test.tsx:633` |
| 删了正显示的页，焦点到父页的标题；删除在途时读者离开，焦点到树的标题 | `PageMenu` 的 `shown` | 同，每次渲染算（3.5） | `page-tree-writes.test.tsx:396`；4.2 新格（R5） |
| 拖放：拖、放的三种位置、被挡的显示、移动 | `useDrag`、`page-tree.tsx:80-111` | 同 | `page-tree-drop.test.tsx`；e2e PG3、L3、LW5 的 `dragPage` |
| 移动之后焦点回到那一行的按钮，在收着的大文件夹里也是 | `focusActions` | 同，加 `through`（3.6） | 4.2 两格（一层、嵌套） |

### 3.9 代价

- **页内查找（Ctrl+F）与读屏的链接列表**：分批画完之前，没画的行找不到。平的一万页在冷打开之后约 2.6–3.0 秒全部在 DOM 里（1.3；今天约 3.0–3.1 秒画出之前什么都看不到）；深的形状根下 11 行，没有差别；展开 1,000 个孩子，首批 100 行画出（T3；原型的约 30 ms 要随改后的脚本再量，1.1 的注），其余两三批，推断再一两百毫秒，没有量。之后与今天相同。
- **键盘**：同上，画完之前 Tab 走到已画的最后一行之后就出了列表。读屏用户的虚拟光标在列表增长时看到新行，读屏念到的项数是已画的行数（推断，没有用读屏核对）。
- **拖放、展开**：不变；展开的列表首批 100 行，其余分批（T3 只会快）。
- **滚到当前页**：今天没有；不加。回到首页时列表从 100 行起重挂，原生的同文档滚动恢复今天也未必恢复（应用没有 `ScrollRestoration`），没量。
- **`aria-controls`**：指向的 `ul` 照旧在，分批只是它的 `li` 晚到。
- **菜单**：开关一次只有那一行与 `TreeMenu` 重渲染。平的一万页里开一次菜单约 0.69 秒，其间长任务约 0.54 秒；`Escape` 关上约 0.53 秒，长任务约 0.44 秒；今天分别是 0.82、0.70 与 1.09、0.62 秒（3.4 的表）。大头在模态，不模态是 v0.2 的候选（第 9 节）。
- **画全之后一次在列表最前多出超过一批**（3.6）：已画的行暂时卸下、下一批再挂上，焦点在那里的丢到 `body`。只有一次 API 的 `batch` 在根下最前建四百页以上会碰到（推断，罕见）；今天一次画全，没有这一件。
- **分批进行中有人改树**：头约 3 秒里重读，列表为已画的行重算属性，下一批又排着；这一段的长任务没有量（4.7.2 的 T2 在标签页静下来之后量，不在界线的范围里）。
- **导入一个平的 zip 之后**：根下一次多出一万行，照 3.6 分批，不再是一次约 0.6 秒的长任务（推断：1.3 一次两份一万行 1.15–1.25 秒）。
- **P2/S6 的各项**（照 [P2 设计](02-P2-web-polish.md) 4.6.6 的表，"改成只渲染看得见的行时"一列都是"不受影响"；S4 比那一列改得更少：不改 store、不改树的读）：
  - **12.12**（`subtreeOf`、`heightOf` 的参数上限，`W/stores/page-tree.ts:115-125`）：纯函数不变，改法与测试照旧。它的"现状"里"树的每一行的菜单渲染调 `subtreeOf`（`page-tree-item.tsx:274`）"在 S4 之后改成"菜单或删除的确认开着时为那一页调"，行号随 S4 变；`remove`、`canHold` 两处照旧。
  - **27.2**（过时的树先显示外壳）：改 `PageTreeStore` 与 `PageLayout`、`VersionPage`，S4 不碰，不受影响。
  - **22.14、23.10**（在途的恢复与撤销放进 store）：`restore()`、`revert()` 不变，不受影响。
  - **24.8**（`links` 事件等树的读）：事件与树的读不变，不受影响。
  - **"2（附带）"**（4.6.6 表里的那一行，与 4.6.7 的"2"）：`removedTo` 不变；S6 那两格经树的菜单删除，走 S4 的 `TreeMenu` 与行里的 `PageDialogs`，S4 不碍它。删除在途时读者去了别处，S4 另有一格（4.2 的 R5）钉住焦点到树的标题。
  - 4.6.6 末的附带观察（"移到……"对话框对每页调 `canHold`）：S4 不改对话框，1,258 ms 照旧，交 P2/S6（第 9 节）。
- **深的形状**：T1 的 11 行、T2、T3 走同样的代码，行更便宜，推断不会慢；判法与结果在 2.1、6.3。
- **依赖**：不加。分批 40 行左右的钩子，菜单仍是 Radix 的 `DropdownMenu`。
- **代码的代价**：树的菜单与行之间多了一张句柄的登记表（`rows`：打开对话框、菜单开着），列表与树之间多了一个 `focusing` 的盒子；3.4、3.6 写明为什么不用 context。

## 4. e2e 与测试

### 4.1 缩小规模的守卫（vitest，新文件 `W/pages/notebook/page-tree-scale.test.tsx`）

规模：平的 1,000 页（十倍于首批，CI 的 `web` 作业里跑得快），编辑者的笔记本首页（左栏与正文两份列表）。页由测试自己造（`pageNode` 的 id 到 899 为止同长，1,000 页另写一个按序号补零的造法），经 `pageServer({ nodes })` 答。

1. `"a notebook of 1,000 pages at its root shows the first 100 of each list at once and the rest at most 400 a commit, all of them in the end"`：
   - `renderApp` 之前在 `document.body` 上挂一个 `MutationObserver`：它的回调在一次提交之后的微任务里跑，一次提交一次回调（下一批在另一个任务里，不会并进同一次）。
   - 第一次看到左栏的 `nav` 里有 `li` 时，记下左栏的行数与首页列表（`ul[aria-label="Pages"] > li`）的行数：这就是第一次提交画了几行，不靠轮询的时机。断言两者都 ≤ 100。
   - 之后每次回调记两份列表各多了几行，断言都 ≤ 400（审查 C 的 M3）。
   - 再 `waitFor` 两者都到 1,000，`p1000` 的链接在左栏与正文里都有。
   - **撤回修复**（`useShownRows` 答 `total`，或列表不用它）：第一次提交就是 1,000，红。一批改成 100,000（第二批画完全部）：第二次回调多 900，红。
2. `"another's change to a tree of 1,000 pages renders only the row it adds, and runs a few reactions"`：
   - 数行的渲染：`vi.mock` 包住行用的拖放钩子（`useDrag` 移进 `page-drag.ts` 并导出，模拟时包一层计数），每次行渲染调它一次；编辑者与读者的每一行都调它。
   - 数 MobX 的 reaction（审查 C 的 M2）：`spy` 记重读期间 `type === "reaction"` 的次数（`observer` 每次渲染报一次；mobx 7.0.5 在 vitest 里是开发版，`spy` 报事件）。
   - 全部画出之后清零；`server` 在根下最前加一页，像 refresher 那样让树重读（`mutate(["pages", id])` 或发一个 `pages` 事件，照 `page-tree.test.tsx` 已有的写法）；等新行出现。
   - 断言行的计数 ≤ 2（新的一行；留一行的余量，原型里是 1，34 ms）；reaction ≤ 10（原型是 4：`PageTree`、`PageList`、`TreeMenu`、`NotebookHomePage` 各一，审查 C 实测）。
   - **撤回修复**：
     - 行是读 `pages.tree` 的 `observer`，或行的属性里给 `node` 对象：行的计数 1,000 或 1,001，红。
     - 对话框一直挂在行里（撤回 3.5）：行的计数 1，reaction 4,008，红（审查 C 的变体 dialogs）。
     - 行里另放一个读树的 `observer`（例如标题）：行的计数 1，reaction 1,005，红（审查 C 的变体 label）。
3. `"a row of the tree holds no menu of its own and no router link, until one is opened and after"`：
   - 1,000 行全部画出之后，左栏的 `li` 里没有带 `data-state` 或 `id^="radix-"` 的元素（Radix 的触发都带这两样）；打开一行的菜单再关上，照样没有。
   - 左栏与正文列表的链接都没有 `data-discover`（react-router 的 `Link`、`NavLink` 带它，`react-router@8.4.0` 的 `lib/dom/lib.js`）；同一页里外壳的路由链接有它，作阳性的对照（版本升级后它不再带这个属性时，这一格先红在对照上，不会悄悄失效）。
   - **撤回修复**：每行一个 `DropdownMenu`：1,000 个 `data-state="closed"`，红；行的链接换回 `NavLink`：1,000 个 `data-discover`，红。
4. `"a list of pages all shown, grown by 1,000 at once, shows them at most 400 a commit"`（审查 C 的 M4）：
   - 根下 10 页的笔记本，全部画出之后，`server` 多出 1,000 页，让树重读；同第 1 格的 `MutationObserver`，每次回调两份列表各多的行 ≤ 400；最后都到 1,010。
   - **撤回修复**（画全之后不再分批）：一次回调多 1,000，红（审查 C 的探针 growth：`main` 与原型都是一次加 1,000 行）。
5. `"following a page of a tree of 1,000 renders the two rows whose mark changes"`：全部画出之后点 `p500` 的链接，行的计数 ≤ 2（新旧两个当前行）。**撤回修复**（每行给 `pageId`，让行自己比）：1,000，红。

**用时与时限**：

- 原型的三格（第 1–3 格的前身）本机约 2.5 秒（审查 C 复跑；初稿写的"测试本身约 5.9 秒"含红着的"moved to the end"一格的 3.3 秒等待，那一格按 4.2 放进 `page-tree-writes.test.tsx`）。第 4、5 格推断各不到 1 秒。
- 每格的时限写进测试：`waitFor` 20 秒、每格 30 秒（原型给的）；缺省是 `asyncUtilTimeout` 3 秒（`W/test/setup.ts:9`）与 vitest 的 5 秒，一千行分三批在 CI 上可能不够。
- CI 的 `web` 作业（`make test-web`）多多少：实施者从 `web` 作业日志里 vitest 的用时与本机的直接比（设计时没取：不往外取），推断多 5–10 秒。不计时，所以不分本机与 CI 的界。

### 4.2 行为的新格（照 3.8 的清单补没有测试的几条）

- `page-tree.test.tsx`：
  - `"a link of the tree opened with a modifier or the middle button is the browser's: the app does not follow it"`：Ctrl、⌘、Shift 加点击与中键各一次，事件没有被阻止，路由的地址不变。jsdom 里没被阻止的 `<a href>` 点击会走 jsdom 自己的导航（它没实现），可能经虚拟控制台进 `console.error`，被 `setup.ts` 的检查判失败：先跑一次看，有就在这一格 `mockImplementation` 声明（13.2 第 9 条）。
  - `"the link of the page shown, followed again, replaces its entry; with a hash or a query it pushes one"`：在 `pages/Notes` 上点 Notes，`router.state.historyAction` 是 `REPLACE`；在 `pages/Notes#h`、`pages/Notes?x=1` 上点是 `PUSH`（审查 A 的 R6）。再一种（审查 A 的 C1-M3）：从首页起，经树点 Guide，再点两次 Notes，第一次 `PUSH`、第二次 `REPLACE`：这一种在树建上下文之后才走到当前地址，抓得住"用建上下文时的地址"。
  - `"on a version of a page, the tree marks that page's link as the page shown"`：在 `pages/Notes/versions/3` 上，树里 Notes 的链接 `aria-current="page"`。
  - `"an editor made a reader while the tree is shown loses each row's Actions; made an editor again, gets them back"`：改 `server.notebook.role`、焦点重读（审查 A 的 R2）。
- `page-tree-writes.test.tsx`：
  - `"the tree's menu is named by its row's button, says it is open there, opened by the keyboard on its first item, and gives the focus back on Escape"`：按钮 `aria-haspopup="menu"`、`aria-expanded` 由假到真、`aria-controls` 等于菜单的 id；菜单的名字是"Actions for Notes"；`Enter` 打开时第一项取得焦点；`Escape` 之后焦点回到按钮，`aria-expanded` 回到假。
  - `"the tree's menu closed by a right click outside it leaves the focus where the click was"`：`pointerdown`（`button: 2`）在页的正文上，菜单关，焦点不在按钮上。
  - `"a row's menu open as another deletes its page closes, and does not open again as the page is restored"`：开着 Notes 的菜单，别处删了它，菜单关、焦点到树的标题；别处恢复它（同一个 id），菜单不开，`nav` 没有 `aria-hidden`（审查 A 的 R3）。
  - `"a row's menu open as another moves its page under another closes, and the focus goes to the page's button where it went"`：在 Install 的页上开始（`pagePath(install.id)`，Guide 随之开着），开 Notes 的菜单，别处把 Notes 挪到 Guide 下：菜单关，焦点到 Guide 下 Notes 的按钮（审查 A 的 R4；起点见审查 A 的 C1-N1：Guide 收着时焦点到树的标题）。
  - `"a row's menu open as another adds a page above it stays at its row's button"`（审查 A 的 C1）：给 jsdom 补一个按文档次序算的 `getBoundingClientRect`（每个"Actions"按钮比前一个低 32 px）。开 Notes 的菜单，看不见的触发的 `top` 等于按钮的；别处在根下最前加 Atlas、焦点重读之后，菜单仍开，触发的 `top` 等于按钮新的 `top`（从 132 到 164 px）。
  - `"a row's menu stays open as another's change collapses that row"`（审查 A 的 C1-M4）：展开 Guide、Install，开 Install 的菜单；别处删了 Linux（Install 唯一的孩子），焦点重读，Install 的展开按钮没了；菜单仍开，`Escape` 之后焦点回到 Install 的按钮。
  - `"the deletion's confirmation says the trash's period once the instance answers, open as it is"`：扣住 `GET /api/v0/instance`，开删除的确认框，放，框里是带期限的那句（审查 A 的 R1）。
  - `"a deletion of the page shown's ancestor, the reader gone back meanwhile to a page out of it, gives the focus to the tree's heading"`：在 Linux 上删 Install（Linux 在它下面），答复扣住时后退到 Notes，再放（审查 A 的 R5）。
  - `"a page moved to the end of a folder of 150 opened by the move keeps the focus on its menu's button"`：一个 150 个孩子的文件夹（收着），用"移到……"把一页挪进去（排在最后），焦点落在它的按钮上。**没有 `through` 时**：焦点在树的标题上，红（原型上实测，6.2 第 5 条）。
  - `"a page moved under the 150th child of a folder of 150 closed keeps the focus on its menu's button"`：Big（收着）有 150 个孩子，用"移到……"把 Mover 挪到"Big / c150"之下（审查 C 的探针 nested）。**`through` 只认自己的孩子时**：焦点在树的标题上，红。
- `notebook-home` 的测试：修饰键的点击交给浏览器（jsdom 的导航同上）；普通点击带 `arrived`（P2/S1 的 12.3 那一格已钉住，S4 照它跑）。
- `tag-page` 的测试（审查 C 的 C1-N2）：`"a tag's list, changed for another tag's of as many pages, renders its first rows again at once"`：两个标签各 500 页，第一个画全之后换到第二个（SWR 里已有），同 4.1 第 1 格的 `MutationObserver`，换过之后第一次提交 ≤ 100 行。

### 4.3 已有的测试

- 原型（不分批时）跑过 `W/pages/notebook/`、`W/pages/page/`、`W/app/` 共 84 个文件 1,757 格，全过（前两版各红一格，见 3.5，改后全过）。
- 分批开着时：审查 C 在原型上让 `setup.ts` 对每个测试都开分批，跑 `W/` 的全部 vitest，193 个文件 4,537 格全过（`review-c/out/vitest-suite-batched.log`，63 秒）。超过 100 行的只有 `W/pages/notebook/changeset-page.test.tsx:301`（101 个根下的页），它不看左栏。实施时照样全量跑一次。
- e2e 不改：行的 DOM、名字与拖放照旧（3.2、3.8）；碰左栏与列表的 e2e（`treeRow`、`dragPage`、PG11 的首页列表、TB2 的 Big）都用自动等待的定位器，行晚几帧到不碍（读出来的，`make e2e` 与三种浏览器照常跑）。

## 5. 实施的次序与提交

### 5.1 提交

1. `web: the page tree's rows render cheap: one menu for the tree, dialogs once chosen, plain links (M12/P4/S4)`：3.2–3.5、3.8，4.1 的第 2、3、5 格，4.2 除 `tag-page` 一格之外的各格。
2. `web: lists of pages render their first rows at once and the rest a chunk after each paint (M12/P4/S4)`：3.6、3.7（含 `tag-page.tsx` 的 `key`），4.1 的第 1、4 格，4.2 的 `tag-page` 一格。
3. `e2e: the page tree's perf waits for every row in the document, records when, and times a row's menu (M12/P4/S4)`：`e2e/perf/probe.ts`、`tree.perf.ts` 的两个只记的量（6.3）；P4/S2 合并之后。
4. `docs: the page tree's rendering, measured again on its three shapes (M12/P4/S4)`：6.3 的表写进本文第 10 节与 P4 设计 10.2，第 8 节的文字。

每个提交单独为绿（`make lint-web`、`make test-web`、`make knip`；第 3 个另加 e2e 的 `check:types` 与 oxlint）；第 4 个提交之前在原规模上再量（6.3）。

### 5.2 与 P2 的关系

- **P2/S1 先合并**（决定 497，预期）：S4 从合并之后的 `main` 开，那时左栏的 `NavLink`、树里两个 `Link` 带 `state={opened}`，`SubpageList` 的 `Link` 带 `state={arrived}`。S4 换成自己的点击处理时带上同样的状态（3.3、3.7）；P2/S1 的 `arrival.test.tsx`（28.5 的各格）与 `page-layout.test.tsx`（12.3）随之钉住 S4 的写法，不必改。
  - 审查 A 实测：在 `m12-p2-s1` 的 `14cc9769` 上放原型的四个文件，树的 `navigate` 带 `{ state: opened }`、`SubpageList` 带 `arrived`，`src/app`、`src/pages/page`、`src/pages/notebook` 85 个文件 1,795 格全过；树的 `navigate` 去掉 `opened`，只红 28.5 的第一格。
  - P2/S1 改了 `ConfirmDialog`（`busy`、`aria-disabled`、拒绝之后焦点回确认）：属性的形状没变，`PageDialogs` 照用；合并时只是代码的合并。
  - P2/S1 的 16.1（附件的行离开时焦点给这一节的标题）：S4 的 3.4 照它的清理时机与落点。
- **万一 S4 先合并**：P2/S1 合并前 `git merge main`，把 `state` 加在 S4 的 `navigate` 上，而不是 `NavLink`、`Link` 的属性上；它的测试不变。
- **P2/S2**（9.5 改 `distinctName` 的文字）：S4 在列表里调它算 `label`，不改它；S4 的新格里用的名字（"Actions for Notes"等）不带位置。谁先合并都不冲突。
- **P2/S3**（焦点环与菜单项的高亮）：普通的 `<a>` 与 `NavLink` 一样没有 `outline-none`，照它改的基础样式画环；树的菜单仍用 `W/components/ui/dropdown-menu.tsx`，它改的高亮照样到。不冲突。
- **P2/S6**：排在 S4 之后（决定 378）；会碰到的见 3.9，交待见第 9 节。

## 6. 测试与验证

### 6.1 变异表

| 变异 | 抓住它的测试 |
|---|---|
| `useShownRows` 总答 `total`（不分批） | 4.1 第 1 格（第一次提交 1,000 行） |
| 首批改成 1,000 | 4.1 第 1 格 |
| 一批改成远大于 400 | 4.1 第 1 格（每次回调 ≤ 400；审查 C 的变体 step 在初稿的守卫上过） |
| 画完一批之后不再加（`step` 为 0 或不再排下一批） | 4.1 第 1 格的"到 1,000" |
| 画全之后多出多少都一次画 | 4.1 第 4 格 |
| 不管 `through` | 4.2 "moved to the end of a folder of 150" |
| `through` 只认自己的孩子 | 4.2 "moved under the 150th child"（审查 C 的变体 through） |
| `through` 抬高的行数不留在状态里 | 4.2 两格（焦点给出、`focusing` 清掉之后那一行卸下，焦点在 `body`） |
| `SubpageList` 不分批 | 4.1 第 1 格（正文的列表） |
| `SubpageList` 的点击处理不看修饰键、不带 `arrived` | 4.2 首页那一格；P2/S1 的 12.3 那一格 |
| `SubpageList` 的行不 `memo` | 不守：正文的行是普通的 `li > a`，重渲染只多 JS，1.3 里约 70 ms，在 T2 的余量里，6.3 的 T2 量得到；为它数正文行的渲染要在行里加一个只为测试的钩子，不值 |
| 行改回读 `pages.tree` 的 `observer` | 4.1 第 2 格（行的计数） |
| 行的属性里给 `node` 对象 | 4.1 第 2 格（行的计数） |
| 树的上下文每次渲染新建 | 4.1 第 2 格（审查 C 的变体 ctx：1,001） |
| 对话框一直挂在行里 | 4.1 第 2 格（reaction 4,008） |
| 行里放一个读树的 `observer` | 4.1 第 2 格（reaction 1,005） |
| 每行一个 `DropdownMenu` | 4.1 第 3 格 |
| 行的链接换回 `NavLink` 或 `Link` | 4.1 第 3 格（`data-discover`） |
| 每行给 `pageId`，行自己比 `active` | 4.1 第 5 格 |
| 点击处理不看修饰键 | 4.2 "with a modifier or the middle button" |
| 点击处理不带 `opened` | P2/S1 的 `arrival.test.tsx` 28.5 第 1 格 |
| 再点当前页不 `replace` | 4.2 "replaces its entry" |
| `replace` 只比路径 | 4.2 同一格（带 `#h`、`?x=1` 的两种） |
| `active` 用别的比较（例如只在 `pages/:pageId` 上为真） | 4.2 "on a version of a page" |
| 菜单不给 `aria-labelledby` | 4.2 菜单那一格（名字） |
| `aria-expanded` 不随开关变 | 4.2 菜单那一格 |
| 关上不 `focusActions` | 4.2 菜单那一格（Escape）；`page-tree-writes.test.tsx` 已有的焦点格 |
| 右键点到外面也还焦点 | 4.2 "closed by a right click outside" |
| 行离开时不关菜单 | 4.2 R3、R4 两格 |
| 触发在渲染时量（不在打开时与提交之后） | 4.2 "adds a page above it"（原型实测，3.4） |
| 行的登记放在拖放的 effect 里（随 `open`、`canDrag` 重跑） | 4.2 "collapses that row"（原型实测，3.4） |
| 笔记本只读 `pages.notebook`，没有后备 | `page-lock.test.tsx` 的两格（审查 A 在原型上实测：`TypeError`，两格红） |
| `replace` 用建上下文时的地址 | 4.2 "replaces its entry"的第四种（先点 Guide，再点两次 Notes） |
| 标签页不给 `SubpageList` 加 `key` | 4.2 `tag-page` 那一格 |
| 钩子在画全之后仍调 `through` | 不守：只多订阅，行为不变，代价是 3.4 那种重算（6–37 ms），3.4 的探针量得到 |
| `period` 从上下文读 | 4.2 R1 那一格 |
| 笔记本从上下文读（不经 `pages.notebook`） | 4.2 R2 那一格（推断：审查 A 指出它在原型上靠巧合过，实施时以这一格核对） |
| 对话框放在树上（不随行卸下） | `source-uploads.test.tsx:633`（原型实测） |
| `shown` 在关上时算（对话框在树上） | `page-tree-writes.test.tsx:396`（原型实测） |
| `shown` 在按确认时记 | 4.2 R5 那一格（审查 A 在原型上实测） |

### 6.2 守卫的反向对照

实施时各做一次，写进第 10 节：在 S4 的头上只撤回对应的修复，跑 4.1 的那一格：

1. `useShownRows` 换成 `(total) => total`：第 1 格红（左栏与正文的第一次提交各 1,000 行）。
2. `PageRow` 换回读 store 的 `observer`（或给 `node`）：第 2 格红（行的计数约 1,000）。
3. 对话框改回一直挂在行里：第 2 格红（reaction 约 4,000）。
4. 行里放回一个 `DropdownMenu`：第 3 格红。
5. 画全之后不再分批：第 4 格红。
6. 在 `main`（S4 之前）上跑第 1–4 格：全红。设计时在 `main` 的代码上跑过原型的三格：第一次提交左栏 1,000 行（> 100）、重读渲染 1,001 行（> 2）、行里 1,000 个 Radix 的元素（≠ 0），三格都红；原型上三格都过（第一次提交左栏与首页各 100 行，1,000 行在 712 ms 之后都在；重读渲染 1 行）。审查 C 复跑一致；reaction 在 `main` 上 2,005、原型上 4。

### 6.3 在原规模上再量

- P4/S2 的 `e2e/perf/tree.perf.ts`、`probe.ts`、`setup.ts`、`shapes.ts`（合并时的，即 T3 挪到 T2 之前、断言视口里有孩子之后的），同样的终点、方法、种子（20261116）、视口与两轮，三种形状；T3 的改前用 P4/S2 重量的数；`make build` 在 S4 的头上；经 `heavy.sh`，在没有别的 Step 在量的时段。
- `probe.ts` 加的只记的量（第 3 个提交）：T1 之后等左栏与首页的列表都到根下的页数，记那一帧的时刻（"全部行进 DOM"）与其间最长的长任务。
- **量的时刻**（审查 B 的 M3、N8）：
  - 堆与首页、文档的 `li` 数在全部行进 DOM 之后读（P4/S2 是"第五次 T1 之后"：那时一万行都在，分批之后 T1 时只有约 900 行，照旧读会把堆少报三四倍）；P4 设计 10.2 的表照它填，写明与 P4/S2 的时刻不同、为什么。T1 时的 `li` 数另记，标明是首批。
  - T2 之前先等全部行进 DOM（同一个量），量的是 P4/S2 那个"一万行都在"的状态。今天 T2 之前 `changeOnce` 等 1 秒没有长任务，各批是长任务，所以碰巧在画完之后；批变短之后就不一定。
- 另记（只记）：平的形状里开关一行的菜单与关一个对话框（`tree.perf.ts` 的平的形状，T2 之后、全部行都在时；量法同 3.4 的探针 `menu.perf.ts`：派发 `pointerdown`、`Escape`，等到菜单画出、焦点回到按钮再两帧，记用时与其间最长的长任务，热身一次、五次取中位）。
- 写出改前（P4/S2 的表）与改后，每格两轮的中位数与五次的数、对照的界线。
- **判**：
  - 平的三格过界线（2.1）；全部行进 DOM ≤ 4 秒、其间最长的长任务 ≤ 200 ms。过了先做一次带 trace 的剖面再定改法（3.6）。
  - 深与带附件照 2.1 的表逐格判。
  - 没过的照 P4 设计 3.9 第 2 条复核两轮，再超就停下报主持者。

### 6.4 门禁

`make lint-web`、`make test-web`、`make knip`、`make build-web`；`make e2e`（Chromium 与 `threeBrowsers` 的三种）经 `heavy.sh`；CI 绿。S4 不改生成的代码，`gen-check-web` 不受影响。

## 7. 完成标准

- 平的形状 T1 ≤ 1,000 ms、T2 ≤ 1,500 ms、T2 的最长长任务 ≤ 200 ms（6.3，两轮的较大中位数）。
- 平的形状全部行进 DOM ≤ 4 秒，T1 到它之间最长的长任务 ≤ 200 ms（6.3，两轮的较大中位数）。
- 深与带附件照 2.1 的表逐格不慢于改前。
- 4.1 的五格与 4.2 的新格在；6.2 的反向对照做过、写进第 10 节；6.1 的变异逐条抓住（写明不守的那一条除外）。
- 3.8 的清单逐条有测试；`make test-web`、`make e2e` 全绿，CI 绿。
- 第 8 节的文字改了；本文第 10 节写了结果。

## 8. 文档的新文字

1. **v0.1 总体设计 13.2 新加第 31 条**（决定 501，不放进第 25 条）：
   - **编号**（决定 505，审查 C 的 C1-I1）：P2/S3 也在 13.2 加一条（决定 396、00 号文档第 7 节"总体设计的修订"第 21 条，P2 设计 8.2 的 S3，都写作第 30 条）。13.2 的新条按合并的先后取号：P2/S3 与 S4 先合并的取第 30 条，后合并的取第 31 条；后合并的那一个在自己的文档提交里改自己的设计、决定与 00 号文档第 7 节里的号。照预期 P2/S3 先合并，S4 取第 31 条，插在第 30 条末句之后；若 S4 的文档提交先于 P2/S3，S4 取第 30 条（插在第 29 条末句之后），P2/S3 照改成第 31 条。文档提交的脚本断言前一条的末句恰好出现一次、新号还没有。
   - 正文（照预期的号）：

   > 31. **页的列表列出全部，分批画**（M12/P4/S4；第 14 节）：左栏的树与首页、页、标签页的列表都列出全部，不设上限。页的数量由写者决定，这里是第 25 条的例外：读者要能走到每一页（页内查找、读屏的链接列表、Tab）。代价靠三样兜住：先画 100 行，之后每帧画完再加 400 行，画全之后一次多出超过一批的照样分批（`app/shown-rows.ts` 的 `useShownRows`），树要聚焦的行与它路上的列表立刻画到；行是 `memo` 的、属性是列表算好的原始值，链接是普通的 `<a>`；一棵树一个菜单，对话框选了才挂。行上要加东西时照此在列表里算成原始值，不在行里读树。守卫是结构的断言（第一次提交与每批的行数、别人改一次树只渲染新的一行与几个 reaction、行里没有自己的菜单与路由链接），见 `[P4/S4 设计](M12-release/04-P4-S4-page-tree.md)` 4.1。1 万页根下的一本，冷打开约 0.2 秒、别人改一次树约 0.2 秒（实施后照实测改）。

2. **v0.1 总体设计第 14 节**"大笔记本（上万页）时前端一次加载整棵页面树的性能"一行（P4 设计第 8 节第 5 条，审查 C 的 I1）：
   - 这一行由 P4/S2 的文档提交改写（结论在 S2，行末是 `（M12/P4 实测，[P4 设计](M12-release/04-P4-performance.md) 10.2）`，5 万、10 万页的数也是它写的）。S4 不整列重写、不去后缀。
   - S4 的文档提交在 S2 写下的、说平的形状在渲染上没过的那一句之后，接着补一句（照实测改）：
     > P4/S4 改为行便宜、先画首屏的分批渲染（`[P4/S4 设计](M12-release/04-P4-S4-page-tree.md)`），三种形状都过：1 万页根下的一本冷打开约 0.2 秒、别人改一次树约 0.2 秒，约 3 秒全部行进 DOM。
   - 锚点是"S2 的文档提交之后的那一句"：实施者在那时的 `main` 上取 S2 写下的那句原文，提交的脚本断言它恰好出现一次，插在它之后、行末的后缀之前。
   - S4 的文档提交在 S2 合并之后才做（6.3 的再量要 `e2e/perf/`），不会先于 S2 的文档提交。万一先了：S4 不碰这一行，把要补的那一句写进 P4 设计 10.2 的"S4 之后"，交 S2 的文档提交一起写。
3. **P4 设计 10.2** 页面树一节之后加"S4 之后"的表（6.3），指向本文。
4. **00 号文档**：
   - **第 8 节**（M12 改到的前面 M 的代码）加一项：`网页（M4）：页面树的行、菜单与对话框的挂法，首页、页与标签页的列表的分批（P4/S4）`。
   - **第 12 节**的 P4 一行写到 S4，"审查"一格加 S4 的记录（P4 设计第 8 节第 4 条"每个 Step 的文档提交"）。
   - **第 13 节**加一行：S4 的结论一句。
   - **第 7 节"总体设计的修订"**的第 26 条（审查 C 的 C1-M1）在设计提交时已加：13.2 的新条（号照决定 505）与第 14 节页面树一行补 S4 之后的一句，随 P4/S4 改。S4 的文档提交照它落进 `v0.1-design.md`；那时号若因别的 Step 先合并而变了，照决定 505 改。
5. **v0.1 总体设计第 15 节**加一行：`2026-11-xx：M12/P4/S4：13.2 新加第 31 条（页的列表列出全部，分批画；号照第 1 条）；第 14 节"大笔记本（上万页）"一行补 S4 之后的结果。依据 [P4/S4 设计](M12-release/04-P4-S4-page-tree.md) 10 与 [S4 记录](M12-release/reviews/P4-S4-page-tree-review.md)。`
6. **S4 的审查记录**：设计审查的是 `reviews/P4-S4-design-review.md`（主持者随设计提交）；实施的是 `reviews/P4-S4-page-tree-review.md`（P4 设计第 8 节第 4 条的写法）。
7. `page-tree.tsx` 的文档注释（`:28-46`）加一句：行是 `memo` 的，一棵树一个菜单，列表分批画（照 3.2–3.6）。

## 9. 交给后面的

- **P2/S6**：可以开（决定 378）。3.9 的各项：
  - 12.12 的"现状"里 `page-tree-item.tsx:274` 一处随 S4 移到菜单与对话框，改写那一句与行号；
  - 27.2、22.14、23.10、24.8 不受影响；
  - "2（附带）"与 4.6.7 的"2"：删除在途时读者去了别处，S4 的 4.2 有一格（R5）钉住焦点到树的标题，S6 改那里时照它跑；
  - "移到……"对话框在深的形状 1,258 ms（P4/S2，没有界线）：每页一次 `canHold`，在 12.12 的范围里顺手看，不在 S4。
- **P4/S3**：第 14 节那一行不归它（P4 设计第 8 节第 5 条）；若 5 万、10 万页进已知限制的清单，那一条归它。
- **v0.2**：
  - **树的菜单不模态**（决定 502 的候选）：平的一万页里开一行的菜单，长任务能从约 0.54 秒降到约 0.08 秒（3.4 的表）。代价：菜单开着时页面会滚、能点别处；看不见的触发是固定定位的，要随滚动重新定位（或放进那一行里让它跟着行走），或者滚动时关上菜单；`aria-hidden` 不再盖住其余内容，读屏的虚拟光标能走出菜单。对话框（模态的 `Dialog`）关上的约 0.4 秒不随它变，要另看。
  - 5 万、10 万页的树（P4/S2 的 4.9 记的数）要分层加载时，左栏多半也要虚拟化（2.4 甲的代价那时要一起付）；这里的分批与便宜的行照用。
  - 行上加东西时（例如移交 M5-polish 第 1 条的"某某正在编辑"标记），照 3.2 在列表里算成原始值，不在行里放读树的 `observer`；4.1 第 2 格的 reaction 计数会抓住。

## 10. 结果

（实施后补。）
