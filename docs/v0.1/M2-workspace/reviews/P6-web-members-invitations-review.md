# M2/P6 前端成员与邀请：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m2-p6-web-members-invitations`（`main(a74df3b)..f51b526`，S1–S5，56 个文件，+2713/−128），对照 [06-P6-web-members-invitations.md](../06-P6-web-members-invitations.md)、各 Step 计划、[M2 总设计](../00-M2-design.md)第 1 节第 2、3、6 条与第 3、4、5、7、9 节、总体设计 9.2、9.4、13.2、13.4、[P5 文档](../05-P5-web-shell-workspaces.md)与 [P5 审查记录](P5-web-shell-workspaces-review.md)，以及 M1/P5、P6 的前端写法（分代、守卫、表单、`ConfirmDialog`） |
| 审查方式 | 独立审查者在 `git archive` 的快照上实测，仓库与别的 Docker 容器都没动过：<br>• 门禁：`make lint knip`、vitest 664 个、`make build-web`、`make gen-check`、workspace 模块与 bootstrap 的 Go 测试（保留名单改过）、`make e2e`（82 个）；W5–W10 的页面版本 `--repeat-each 3`，30 次全部通过；P6 改过的 vitest 文件连跑 5 遍；S1–S4 各个提交的 Web 门禁也逐个跑过。`make image-smoke` 会覆盖本地镜像标签，没跑（作者跑过）；<br>• 反向对照：单元 46 项、Go 2 项、e2e 6 项（汇总见下）；另有探针：vitest 4 组、e2e 2 项、Chromium 脚本 2 个，复现 T1–T3、T6 与 Q8 |
| 日期 | 2026-10-01 |
| 结论 | 处理 T1–T3、补上 T4–T10 的测试之后可以合并；没有 Major。<br>• 第 5 节的 11 项反向对照全部失败，只有"令牌放进查询参数"不让 W6 失败（D2）；分代、SWR 键、读写交错、去重、守卫的例外、令牌只在片段里，符合 13.2 与 M2 总设计第 4 节。<br>• P5 审查出过的两类缺口又出现了：已读过的列表再读一次会不会更新（P5 T4）、加载失败与"重试"（P5 T7），见 T4、T5。<br>11 项 Minor 与 7 项 Nit 合并前处理（`7f999e3`、`6733b53`），见下；12 个疑问：Q4、Q8、Q9 随 T3、T13 改，Q2、Q3、Q5 改文档或 README，其余不改；文档偏差全部同步 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| T1 | Minor | 移出成员之后，同一页的邀请列表仍列着服务端已一并撤回的、对他邮箱的邀请，管理员复制到的是失效的链接。要成员的邮箱改成一个有待接受邀请的地址（`users set-email`，W9 正是这个场景）才会出现。vitest 探针与 e2e 探针（W9）都复现 | 移出成功之后重新读取邀请（SWR 的 `["invitations", id]`）。移出的测试（答 204 与 404 两例）断言那份邀请随之不在列表上；W9 的页面版本先断言那份邀请在列表上，移出之后列表变成"没有待接受的邀请"。去掉重新读取时，单元测试与 W9 都失败 |
| T2 | Minor | 改角色失败的提示永远不清除：先 403、再成功，alert 仍在，读屏用户会以为第二次也失败了 | 每次改角色开始时清掉。测试"one change of role goes out at a time; a refused one leaves the role, and says why until the next"断言下一次成功之后提示消失；不清时它失败 |
| T3 | Minor（可访问性） | 改角色时焦点丢失：发送期间 `<select disabled>`，Chromium 把焦点移到 body，重新启用之后不还回来。e2e 探针（W8）：之后 `document.activeElement` 是 `BODY` | 角色改成菜单（Radix `DropdownMenu` 的单选项），触发按钮显示当前角色：选中就发，菜单关闭时焦点回到按钮，发送中按钮 `aria-busy` 并且不接受新的选择（Q4），方向键在菜单里移动不发请求（Q9）。改角色的测试断言选中之后焦点回到按钮；W8 断言改完之后焦点在按钮上。关闭时不还焦点（`onCloseAutoFocus` 里 `preventDefault`），单元测试与 W8 都失败 |
| T4 | Minor（测试缺口，同 P5 T4） | 列表一旦读过就不再更新，没有测试发现（`MemberStore.load`、`InvitationStore.load` 改成"已有列表就不覆盖"，vitest 与 W5–W10 都通过）；"改角色被拒"的测试只核对又发了一次 GET | 新测试"the lists read again show what changed elsewhere"：外面有成员离开、邀请被撤回，离开再回来（假计时器推进 2 s）之后两份列表都更新；改角色被拒的测试核对读回的角色被采用。两项反向对照各自失败；e2e 层邀请一半现在让 W9 失败（T1 的重新读取） |
| T5 | Minor（测试缺口，同 P5 T7） | 加载失败与"重试"没有测试：成员列表、邀请列表、邀请页读账户把错误当成加载中，预览的"重试"什么也不做，全部通过 | 四处各加"读不到时说明原因，重试之后加载"的测试；四项反向对照各自失败 |
| T6 | Minor（测试缺口） | SWR 键里的工作区 id 没有守住：键改成固定的，全部通过；探针在 2 s 内从 Lab 的成员页转到 Acme 的，Acme 一直停在加载中 | 新测试"each workspace's members page lists its own members and invitations"：从 Lab 的成员页直接到 Acme 的，列出 Acme 的成员，邀请一节没有 Lab 的那份。成员的键、邀请的键（连同移出之后 `reload` 的键）改成固定的，两项反向对照各自失败（`6733b53`：起初只靠 `reload` 的键对不上才让移出的测试失败） |
| T7 | Minor（测试缺口） | "没有剪贴板"（非安全上下文）没有测到，只模拟了写入被拒绝；改成 `navigator.clipboard?.writeText` 时显示"已复制"，实际什么也没复制 | 新测试以 `vi.spyOn(navigator, "clipboard", "get")` 让剪贴板为 `undefined`，断言显示链接框、不说已复制；反向对照失败 |
| T8 | Minor（测试缺口，同 P5 T6） | 接受之后转到 `/` 分不出来：被邀请人只有这一个工作区，落点碰巧就是它；只有引导的单元测试失败 | 单元测试与 W6 的被邀请人先有一个按名称排在前面的工作区（"Aardvark"）。改成转到 `/` 时，单元测试与 W6 的页面版本都失败 |
| T9 | Minor（测试缺口） | 角色控件的两个性质没有测试：非受控（被拒绝时仍显示被拒的角色）；发送中不禁用（设计 3.3 的要求，也是依次发出的唯一保证） | 随 T3 换成菜单：显示 store 里的角色（store 测试"a role change refused leaves the role as it was"），发送中的选择不接受（成员页测试"one change goes out at a time"）。先显示所选的角色、发送中照样发，两项反向对照各自失败 |
| T10 | Minor（测试缺口） | 邀请页还有三处没测到：会话不可用时显示表单、页内注册用登录的本地检查、接受答 404 | 各补一个测试；三项反向对照各自失败 |
| T11 | Nit（测试缺口） | 邀请表单不把邮箱算作显示的字段（422 时上方多出通用错误）、默认角色改成访客、邀请页的"退出"不发 logout、复制成功之后不收起链接框，都没有测试发现 | 四处各加断言；四项反向对照各自失败 |
| T12 | Nit（可访问性） | "复制链接"的可见文字不在可访问名称里（WCAG 2.5.3），语音控制说"点击复制链接"找不到它 | 名称改为"复制链接：{email}"（en "Copy link: {email}"），以可见文字开头 |
| T13 | Nit | 邀请表单的注释"e-mail 字段的值不带首尾空白"位置不对，也只对 ASCII 空白成立（Q8） | 恢复 `trim()`（JS 的 `trim` 也去全角空格），注释说明为何浏览器去空白之后还要 trim；新测试用全角空格。去掉 `trim()` 时它失败 |
| T14 | Nit | 一节的"已复制……"从不清除：撤回了那份邀请，或别的一行复制失败之后，仍显示 | 撤回、复制失败时清掉；两项测试与反向对照 |
| T15 | Nit | `useWorkspaces` 的注释与错误消息说只用于 `SignedIn` 之下，邀请页的 `Accept` 在守卫之外合法地用它；`ProblemTexts` 的键是 `string`，码写错也能编译 | 注释改为"已登录的一代"，消息改为 "useWorkspaces is used signed out"；键收窄到已有文案的 problem 码 |
| T16 | Nit | en 的 `invitation_email_mismatch` 写 "email"，全站其余地方写 "e-mail" | 统一为 "e-mail" |
| T17 | Nit | e2e 的 `inviteWith` 把任何答复都当成 `WorkspaceInvitation`，422 时其实是 Problem | 答 201 时才有 `invitation`，否则 `undefined` |
| T18 | Nit | 两位成员同名时，"Bob 的角色""移出 Bob"分不开 | 管理员看得到邮箱：控件的名称带上邮箱（"Bob (bob@…)"）；成员页测试与 e2e 夹具 `who(name, email)` 照此查找。只用名称时反向对照失败 |

## 疑问与判断

| # | 疑问 | 判断 |
|---|---|---|
| Q1 | 预览答 404 时用 `NotLoaded`，带"重试" | 接受：与其他加载失败一致；4xx 重试也不会不同，但多一个按钮无害 |
| Q2 | 改角色失败会重新读取工作区，移出、撤回、邀请答 403 时却不读 | 接受，与 P5 Q1 一致：移出、撤回的失败留在对话框，下一次获得焦点时的读取更新控件。3.3 写明两种处理 |
| Q3 | 改角色失败一律重新读取成员与工作区，不按码分支 | 合理：404 `member_not_found` 时那一行随之消失，5xx 多读一次无害。3.3 照实现改写（D1） |
| Q4 | 13.2 第 1 条要求答复整个资源的写依次发出，改角色靠禁用控件保证 | 菜单在发送中不接受新的选择，一行一次一个；不同成员的改角色本就是不同资源，可以同时发。T9 的测试守住 |
| Q5 | 链接用 `window.location.origin` 拼出，经内网地址访问时复制的是内网链接 | 接受：服务端没有配置公开地址。README 的前端一节写明，对外发链接请从公开地址打开 |
| Q6 | 停用对话框不列出是哪些工作区（第 2 节"不做"） | 同意 |
| Q7 | `field.email.not_allowed`、`field.email.duplicate` 是全局的字段键 | 现在只有邀请会答出这两个码，所以没问题。以后的 M 有别的操作对 `email` 答出它们而意思不同时，要么改写文案让两处都适用，要么给 `formErrors` 加按字段码换说法的选项：`texts` 只换 problem 码（M2 收尾审查 B-M1 改正了这里原先"由页面经 `texts` 换说法"的写法） |
| Q8 | `type=email` 由浏览器去掉首尾空白，所以去掉了 `trim()`：前提成立吗 | 按 WHATWG 的值清理只去 ASCII 空白，全角空格保留（Chromium 与 jsdom 实测）。恢复 `trim()`（T13） |
| Q9 | 原生 select 选了就发：Windows、Linux 的 Chrome 在收起的 select 上按方向键会立即触发 change，每按一次发一个中间角色 | 随 T3 换成菜单，方向键只移动，选中（Enter、点击）才发 |
| Q10 | `Accept` 在调用 hooks 之前 throw | 接受：与守卫的 `Account` 一致，throw 中止这次渲染，完成的渲染里 hooks 顺序不变 |
| Q11 | `membersOf`、`invitationsOf` 在渲染中创建 store | 接受：`once` 幂等，按代缓存，严格模式渲染两遍也只建一个 |
| Q12 | `wasRemoved` 在离开之后不会因为接受邀请而清除 | 忽略：只影响同一代里离开、再经邀请加入、又失去它之后，转到 `/` 而不是 404 |

## 文档与代码的不一致

作者已知的 9 处，审查者逐条核实，都合理：按 problem 码换说法的机制叫 `texts`（`ProblemTexts`），贯通 `errorText`、`formErrors`、`useForm({ onField, texts })`、`ConfirmDialog`、`CredentialsForm`，`useForm` 的四个调用方都已改成选项对象；`signInProblems`、`signUpProblems` 挪进 `credentials-form.tsx`，`useSession` 导出；`InvitationLink` 类型在 services；`role-options.tsx` 与 `once`；预览答 404 时用 `NotLoaded`（Q1）；邀请成功之后显示服务端保存的小写邮箱；成员列表带 `aria-label`；`watchFor` 挪到 `src/test/watch.ts`；测试文件与 e2e 测试的拆分。

另外的：

- D1：3.3 说"答 403 时重新读取工作区列表"，实现是任何失败都重新读取成员与工作区（Q3）。
- D2：第 5 节"令牌放进查询参数 → W6 页面版本失败"不成立：W6 打开的是夹具 `linkTo` 拼出的链接；只改 `invitationLink` 时 W5（复制的链接与列表里的邀请对照）失败，W6 通过。
- D3：3.7 的夹具名：`membersTable` 实际是 `membersList`、`membersListed`、`roleOf`；`copiedLink` 是 `copyLinkWith`；`invitationPage(page, link)` 是 `linkTo(invitation, baseURL)`，只返回地址。
- D4：3.7 的 W8 写"把成员改成访客"，测试是把访客改成成员。
- D5：3.7 的 W9 写"成员离开，落到 `/`"，测试里离开的是访客；"待接受邀请一并删除"只核对了数据库（T1）。
- D6：3.6 写"没有剪贴板时显示链接框"，测试只覆盖写入被拒绝（T7）。
- D7：3.4 的表中"会话不可用""接受答 404"两行没有测试（T10）。
- D8：README 的"表单"一节、总体设计 13.2 第 12 条都还没写页面自己的 `texts`。
- D9：`context.tsx` 的注释（T15）、邀请表单的注释（T13）与代码不符。

全部处理：P6 文档 3.1–3.7、第 5、7 节，S1、S2、S4、S5 计划，M2 总设计第 8、11、12 节，README 的前端一节（D8 的表单一节与 Q5）；13.2 第 12 条的 `texts` 在 M2 收尾审查时补。

## 反向对照

会失败的（审查者与作者分别做过）：

- 第 5 节的 11 项：`MemberStore.load` 不管之后答复的写（store 的交错测试）；自己的一行有控件（成员页、W8）；访客显示邮箱列（成员页、W8 访客）；非管理员也去读邀请（5 个测试、W5 成员、W8 访客）；离开不记下 slug（离开测试、store 测试）；令牌放进查询参数（`invitationLink` 3 个、复制 2 个、W5，见 D2）；注册不带邀请（邀请页、W7）；邀请页挂到 `SignedIn` 之下（邀请页 7 个，W6 两个、W7：地址变成 `/sign-in?next=…%23nwk_inv_…`，令牌进了 `next`）；接受之后不放进列表（store 测试）；停用对话框不换说法（安全页、W10）；`[app]` 漏掉 `invitations`（保留名单）。Go 一侧：从名单删掉 `invitations`，`TestSlugProblem` 失败；挪回 `[reserved]` 时 Go 测试通过，`[app]` 与路由的一致由 vitest 守住。
- 审查者另做的：改角色被拒却不说；"你"；`ConfirmDialog`、`CredentialsForm`、`useForm` 各自丢掉 `texts`；`useForm` 与三个调用方丢掉 `onField`；邀请成功之后显示输入的原文；成员列表按 slug 缓存；片段不解码；片段为空也发预览；撤回答 404 当失败；新邀请放到最后；预览键不带令牌；标题不可聚焦。
- 作者在各 Step 做的单元 39 项（S1 11、S2 14、S3 12、S4 2）与 e2e 6 项。
- 作者在审查修复中做的单元 26 项与 e2e 4 项：T1、T2、T3、T4 两项、T5 四项、T6 三项（邀请的键先单改、再连同 `reload` 一起改）、T7、T8、T9 两项、T10 两项（会话不可用时显示表单、注册用登录的检查；接受答 404 是新增的情形）、T11 四项、T13、T14 两项、T18；e2e：T1（W9）、T3（W8）、T8（W6）、邀请列表不再读取（W9）。

审查时改了却没有测试失败的 17 项单元与 3 项 e2e，处理之后：单元 17 项都已补上测试，现在都会失败（原生 select 已不在，"非受控"一项换成菜单显示 store 的角色）；e2e 的"接受之后转到 `/`"（W6）、"列表都不再读取"（邀请一半，W9）现在会失败；"令牌只放进查询参数"仍只让 W5 失败，W6 打开夹具拼出的链接，这一项由 W5 与单元测试守住（D2）。

## 没能验证的风险

- 审查者没跑 `make image-smoke`，也没看持续集成；作者跑过 image-smoke，持续集成为绿（`f51b526`、`6733b53`）。修复之后作者在本地重跑门禁（vitest 679 个）、`make gen-check` 与 `make e2e`（82 个）。
- 只用 macOS 上的 headless Chromium：Firefox、Safari 的 `type=email` 去空白只按规范判断；Safari 在 `onFocus` 里 `select()` 会不会被 mouseup 取消没有验证；读屏的实际朗读没有验证。
- 非安全上下文（HTTP 局域网地址）下没有剪贴板，只由单元测试模拟，没有实机打开。
- "令牌不进 Referer"依据规范；W6 只核对请求的 URL。
- T1 的触发依赖 `users set-email`；有没有别的路径让有效成员的邮箱上挂着待接受邀请，没有穷举。
