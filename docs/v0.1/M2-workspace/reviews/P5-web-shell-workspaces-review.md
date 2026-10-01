# M2/P5 前端外壳与工作区：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m2-p5-web-shell-workspaces`（`main(89dab6b)...748c2a6`，S1–S5，63 个文件，+2372/−139），对照 [05-P5-web-shell-workspaces.md](../05-P5-web-shell-workspaces.md)、各 Step 计划、[M2 总设计](../00-M2-design.md)第 1 节第 6 条与第 3、4、7、9 节、[M1 移交](../handoffs/M1-identity.md)第 5、6 项、总体设计 9.2、9.4、13.2、13.4，以及 M1/P5、P6 的前端写法（分代、守卫、表单、`ConfirmDialog`） |
| 审查方式 | 独立审查者在 `git archive` 的快照上实测，仓库与别的 Docker 容器都没动过：<br>• 门禁：`make lint knip`、vitest 546 个、`make build-web`、`make gen-check`、workspace 模块与 bootstrap 的 Go 测试（保留名单改过）、`make e2e`（72 个，快照 `git init` 过）；W1–W4、W11、S2、A3、A4、A6、A9 `--repeat-each 3`，87 次全部通过；S1–S4 各个提交的 Web 门禁也逐个跑过。`make image-smoke` 会覆盖本地镜像标签，没跑（作者跑过）；<br>• 反向对照：单元 53 项、e2e 13 项（汇总见下）；另有 7 个探针（vitest 3 个、e2e 4 个文件、Go/PG 1 个），复现 T1–T3、T9 与 `postData` 为 `null` |
| 日期 | 2026-10-01 |
| 结论 | 修好 T1 之后可以合并。<br>• 第 5 节的 9 项反向对照全部失败；分代、SWR 键、守卫、保留名单、表单与 `ConfirmDialog` 的写法符合 13.2。<br>• T1（Major）：从一个工作区的常规页直接到另一个工作区的常规页时，改名表单留着前一个工作区的名称，保存就把这个工作区改成那个名字，浏览器里复现。<br>1 项 Major、7 项 Minor 与 6 项 Nit 合并前处理（`13123b8`），见下；7 个疑问：Q1 照建议改，Q3、Q5 改文档或加断言，其余不改；文档偏差全部同步 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| T1 | Major | 按工作区持有的页面状态不随 slug 重置：`RenameForm` 用 `useState(workspace.name)` 初始化，`WorkspaceLayout` 的 `<Outlet />` 不带 key，React Router 在参数变化时复用组件实例。从 `/lab/settings/general` 到 `/acme/settings/general`（浏览器 `history.go(-2)`，或连按两次后退），名称框显示 "Lab"，保存发出 `PATCH acme Lab`。"已保存"、`ConfirmDialog` 的输入、P6 的成员页、M3 在 `/:slug` 下的每一页都属于同一类 | 布局在一处统一：`<Outlet key={workspace.id} />`，工作区的页面随工作区重新挂载。新测试"a page of one workspace starts anew in another"：在 Acme 的常规页输入名称，转到 Lab 的常规页，框里是 "Lab"。去掉 key 时它失败 |
| T2 | Minor | 删除之后先显示 404 页再转到落点：页面先 `await remove`，store 立即移出这一项，外壳（observer）随即渲染 404，之后的 `navigate("/")` 还要等落点页的懒加载块。审查者给块加 300 ms 延迟时，连续约 280 ms 画出 "Page not found" | 页面不再自己跳转：store 记下这一代删掉的 slug（`wasRemoved`），外壳找不到工作区时，对这些 slug `<Navigate replace to="/">`，其余仍是 404。删除测试用 `MutationObserver` 断言 "Page not found" 从未进入页面。外壳对删掉的 slug 显示 404（页面不跳转，测试与 W4 失败），以及先渲染 404 再跳转（只有 `MutationObserver` 的断言失败），两项反向对照都失败 |
| T3 | Minor | 创建与读取交错时列表出现重复项：读取在创建提交之后发出、早于创建的答复回来，它不会被丢弃（回来时还没有写的答复），创建的答复再追加一次，得到 `["acme","acme","beta"]`，切换器的 key 重复。M1 的 `ApiTokenStore.create` 写法相同 | 两个 store 的创建都先按 id 去掉旧的再加。各加一个交错测试（读取先于创建的答复回来、并已含新项）；改回直接追加时它们失败 |
| T4 | Minor | 已加载的列表再读一次会不会更新，没有测试：把 `load` 改成"列表已有就不覆盖"，vitest 与 27 个 e2e 全部通过 | 外壳测试"a workspace a later read no longer has is not found"：离开再回来、上一次读取已不算新（SWR 去重 2 s，用假计时器推进），列表里没有它就是 404。上面的改动让它失败；去掉推进计时器时它也失败，证明测试确实依赖第二次读取 |
| T5 | Minor | 已有工作区的账户在关闭创建的服务上，引导的一步应当自动继续，没有测试：先看"创建关闭"再看"已有工作区"，全部通过 | "已有工作区时自动继续、只记录一次"改为 `test.each` 覆盖创建打开与关闭；先看创建开关时，关闭的一例失败 |
| T6 | Minor | "成功之后进入新工作区"分不出转到 `/:slug` 与转到 `/`：测试的新工作区 "Acme" 按名称排在 "Lab" 前面，落点碰巧就是它 | 测试改建 "Zeta"（排在 Lab 之后，`/` 会落到 Lab）；测试替身的列表按名称排序，与服务端一致。改成转到 `/` 时它失败 |
| T7 | Minor | 加载失败与"重试"只在落点页测过：外壳、创建页、引导的一步把错误当成加载中，全部通过 | 三处各加一个"读不到时说明原因，重试之后加载"的测试；三项反向对照各自失败 |
| T8 | Minor | 删除答 404 没有测试（S3 计划写了"403、404 时对话框保持"） | 处置见 Q1：404 当作已删除。删除测试改为 `test.each`：答 204 与答 404 `workspace.not_found` 都落到另一个工作区；store 测试核对 404 移出、403 不移出并抛出。把 404 当失败时两个测试失败 |
| T9 | Nit | 排序的注释与 3.4 说"按码位比较"，JS 的 `<` 按 UTF-16 码元：PG（builtin C.UTF-8）是 `Ａ team` < `😀 team`，`byName` 相反。测试名写了 `lower(name), name, id`，去掉 id 的比较照样全部通过 | 注释与 3.4 改为按 UTF-16 码元、与码位只在 BMP 之外不同，下一次读取以服务端为准。加同名不同 id 的测试；颠倒 id 的比较时它失败 |
| T10 | Nit | 首页导航项去掉 `end` 全部通过；`main` 的 `has-[[data-shell]]:p-0` 只能看画面（Q5） | 常规页的第一个测试断言"首页"不带 `aria-current`；W3 断言外壳里 `main` 的内边距为 0、创建页不为 0。两项反向对照（去掉 `end`；去掉 `has-[[data-shell]]:p-0`，W3 失败）都失败 |
| T11 | Nit | 可访问性：<br>• 输入确认的对话框打开时焦点在"取消"上，输入框里按 Enter 不提交；<br>• 切换器的触发按钮只有工作区名称；<br>• 删除之后焦点落到 body；<br>• 左栏用 `aside` 承载主导航 | • 有 `typedConfirmation` 时打开就聚焦输入框，输入一致之后 Enter 确认；<br>• 触发按钮以 `aria-describedby` 带上"切换工作区"；<br>• 删除之后的焦点与 `aside` 留到 M3 加页面树时一起设计。<br>新测试：对话框打开时焦点在输入框，输入一半时 Enter 不发，输入完整之后 Enter 删除；切换器按名称与说明找到。不聚焦、Enter 不确认、Enter 不等输入一致、去掉说明，四项反向对照各自失败 |
| T12 | Nit | 文案：<br>• zh-CN 的 `createWorkspace.off` "由管理员创建"易读成工作区管理员；<br>• zh-CN 的 `onboarding.workspace.*` 插在 `onboarding.profile.title` 与 `.hint` 之间；<br>• en "Taking you on…" 别扭；<br>• 已占用有 "Another workspace has it." 与 "Another workspace already has this address." 两种说法 | 改为"服务器管理员"；调整键的顺序；"Continuing…"；统一为后者（zh-CN"这个地址已经被别的工作区使用。"），单元测试与 W1 随之改 |
| T13 | Nit | 改名成功之后，名称框保留带首尾空白的原文 | 保存之后框里是去空白的名称；改名测试断言它。去掉时测试失败 |
| T14 | Nit | 存储读得到、写不进（配额已满）时，`lastWorkspace()` 返回存储里的旧值，内存里的新值被忽略 | 写失败时记下，之后读内存。偏好测试：写抛 `QuotaExceededError` 时读到新值。改回先读存储时它失败 |

## 疑问与判断

| # | 疑问 | 判断 |
|---|---|---|
| Q1 | 删除答 403、404 之后怎么办：3.6 是保持对话框、显示原因；M1/P6 撤销令牌把 404 当成功 | 404 当作已删除：两种可能（已被删除、已被移出）都意味着这个账户已经没有它，结果就是用户要的。403（被降级）保持对话框并说明原因，不另读列表：下一次获得焦点的读取会更新控件 |
| Q2 | 退出时不清 `nwiki.workspace`：共用设备上，下一个账户能看到上一个账户最后访问的 slug | 接受：只是 slug，不是内容；换了账户时它不在新列表里，落点不受影响（3.3） |
| Q3 | 切换器是单选菜单项加 `navigate`，不是 3.2 写的链接：不能中键或在新标签页打开 | 保留单选项：读屏读得出当前选中的是哪个，菜单的键盘操作是 Radix 现成的。文档 3.2 改写；在新标签页打开别的工作区，用地址栏 |
| Q4 | 引导中创建答 403 `workspace.creation_disabled`（服务器刚关闭创建）之后，这一步仍是表单，要刷新才看到说明 | 不改：很少发生，表单上方已说明原因，刷新即可 |
| Q5 | `has-[[data-shell]]:p-0` 没有自动化测试 | W3 断言 `main` 的计算内边距（T10） |
| Q6 | S1–S4 的提交上 e2e 是红的（S1 删掉首页标题，S4 加了步骤而 e2e 的 `onboardingSteps` 到 S5 才改） | 接受并记下：计划只要求每个 Step `make check` 为绿（实测满足）；只影响用 e2e 做 bisect，合并提交是绿的 |
| Q7 | `field.name.too_long` 说的是 100 个字符 | 审查者核实：客户端发出 JS `trim()` 之后的名称，Go 的 `TrimSpace` 只会去得更多，码位数等于 rune 数，本地 80 的检查一定先拦下。名称的本地检查用自己的键 `field.workspace_name.too_long`，不改 |

## 文档与代码的不一致

作者已知的 10 处，审查者逐条核实，都合理：service 的改名方法叫 `rename` 而不是 `update`；新增 `app/not-loaded.tsx`（加载中或失败与重试）与 `components/nav-item.tsx`（三处导航共用，M1 设置页的样式不变）；创建页没有"回到 /"的链接（顶栏的 Nerve Wiki 就是）；创建页的骨架与 `create-workspace` 移进 `[app]` 在 S1，表单在 S2；名称的本地检查用自己的键；slug 改了之后不再显示提交时的错误；测试替身 `byRoute` 支持 `/prefix/*`；W4 的页面版本拆成管理员与成员两个测试；A4、A6 落在引导的第二步（它们的账户没有完成引导），A3 用 `expectCreatePage`；e2e 的 `stepRecorded` 按答复匹配（页面发出的 POST 在 Playwright 里 `postData()` 为 `null`）。

另外的：

- D1：3.2 的切换器"每项是链接"，实现是单选菜单项（Q3）。
- D2：3.5 的 slug 生成少写了三点：先做 NFKD 并去掉附加符号（Café → cafe）；`-` 本身也当分隔符折叠（`a--b` → `a-b`）；先截到 48，再去掉末尾的 `-`。
- D3：3.4 与 store 的注释"按码位比较"（T9）。
- D4：3.6 的"204 之后从列表删除并转到 /"导致 T2，随 T2 改写。
- D5：S1 计划的"加载失败与重试"（路由）、S3 计划的"403、404 时对话框保持"只部分落实（T7、T8）。
- D6：README 的前端一节只写了外壳与落点：创建页（关闭时的说明）、`/:slug/settings/general`、引导的工作区一步、用户菜单的版本都没写。
- D7：合并时要更新 P5 第 1 节"状态"与第 7 节、M2 总设计第 7 节（M1 移交第 5、6 项）与进度表。

全部处理：P5 文档 3.1–3.6、3.9、3.10、第 4、7 节，S1、S2、S3、S5 计划，M2 总设计第 7、11、12 节，README 的前端一节。

## 反向对照

会失败的（审查者与作者分别做过）：

- 第 5 节的 9 项：落点不看最后访问（`landingPath`、落点页、W3）；布局不记下最后访问（外壳、W3）；`load` 不管之后答复的写（store 的交错测试）；不是成员的 slug 照样渲染外壳（外壳 8 个、W3）；创建关闭时切换器仍有入口（外壳、W2）；删除不要求输入 slug（常规页、W4）；已有工作区时引导的一步不自动继续（引导 3 个、W11 两个）；`[app]` 漏掉 `create-workspace`（保留名单）；e2e 的 `onboardingSteps` 漏掉 `workspace`（19 个故事）。
- 审查者另做 34 项单元与 4 项 e2e：排序的三种错法；最后访问没有内存退路、先读内存；slug 允许大写或 49 个字符；名称按 UTF-16 计数、去空白之前计数；发出未去空白的名称；可用性检查任何拼写都问、不去抖、文案对调；409 不放到 slug 下、改了之后旧错误仍在、改过之后仍随名称；不去重音、先去 `-` 再截断；关闭时仍有表单、删除之后不转走、没改的名称也发、改名不更新列表、成员看到表单、关闭对话框不清输入；引导的一步没有 once 守卫、创建之后再记录、关闭时仍给表单、"继续"或重试不记录；用户菜单不显示版本；`WorkspaceStore` 跨代共用；删除不移出；最后访问不在列表里仍用它；e2e 上版本（S2）、改名不更新列表（W4）、可用性不显示（W1）。
- 作者在各 Step 做的 27 项单元与 4 项 e2e。
- 作者在审查修复中做的 21 项单元与 2 项 e2e：T1、T2 两项（另有 e2e：W4）、T3 两项、T4（另核实测试依赖第二次读取）、T5、T6、T7 三项、T8、T9、T10 两项（其一在 e2e：W3）、T11 四项、T13、T14，见上表。

审查时改了却没有测试失败的 10 项单元与 3 项 e2e（M10、M30、M38、M40、M41–M43、M44、M45、M46，及 E8、E9、E12），都已补上测试，现在都会失败。

## 没能验证的风险

- 审查者没跑 `make image-smoke`，也没看持续集成；作者跑过 image-smoke，持续集成为绿（`748c2a6`、`13123b8`）。修复之后作者在本地重跑门禁、`make gen-check` 与受影响的 e2e（W1–W4、W11、S2、A3、A4、A6、A9 各 3 次，96 次全部通过）。
- 只用 Chromium（headless）：Safari、Firefox 的画面、窄屏布局、读屏的实际朗读没有验证。
- T1 只用 `history.go(-2)` 与 `router.navigate` 复现；T3 只在 store 层用确定的顺序证明，浏览器里的时间窗口太窄。
- JS 与 PG 的排序差别只验证了几个代表字符（全角、emoji、İ、ß）；差别只在下一次读取之前。
