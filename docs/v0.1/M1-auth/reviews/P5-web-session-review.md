# M1/P5 前端会话、登录与引导：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m1-p5-web-session`（`main...6446141`，S1–S6），对照 [05-P5-web-session.md](../05-P5-web-session.md)、各 Step 计划、[M1 总设计](../00-M1-design.md)与总体设计 9.4 |
| 审查方式 | 独立审查者在工作树上实测：<br>• 门禁：`make check`（vitest 299 个）、`make e2e`（41 个）；页面版本的 13 个故事与 S2、S4 另跑 `--repeat-each 6`，78 次全部通过；<br>• 与 Nerve 逐文件 diff：令牌管理器、续期锁、认证中间件、`one-at-a-time` 与 6 个测试、3 个替身，除改名、注释与版权头之外行为代码逐字相同；会话的 108 个测试在 jsdom 下通过；<br>• 反向对照：`safeNextPath` 只留"以 `/` 开头"，A3 失败（React Router 8.4 的 `validateNavigationTarget` 另外拒绝了外部跳转，是第二道防线）；Web Locks 换成空锁，A4 失败；`AppProviders` 去掉 key，`session-root` 的测试失败；另有两处没被抓住（M3）；<br>• 用 Chromium 在真实构建上探测带点段的 `next`（`/..//evil.example`、`/.//evil.example`、`/%2e%2e//evil.example`、`/a/../..//evil.example`）：都停在同源的 `/evil.example`（404），没有报错；<br>• 用临时的 vitest 复现 M1（已删除） |
| 日期 | 2026-10-01 |
| 结论 | 修复后合并。拷贝忠实；会话装配靠依赖注入，导入没有副作用；分代模型简单而正确；守卫是唯一决定去处的地方；文案表与契约逐码核对；端到端扎实（A4 以续期代数证明串行，A1 核对令牌的去处，控制台逐条声明）。没有 Critical 与 Important。<br>4 项 Minor、7 项 Nit：全部在合并前处理，N7 的后半不做（理由见下） |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| M1 | Minor | `AccountStore.load()` 不排队，读的答复无条件写进 `me`。SWR 聚焦时重新读：切回窗口、直接点"继续"，GET `/me` 与写请求几乎同时出去，读到写之前的行又比写后答复，`me` 就回到旧值（引导的 `onboarding_steps` 回退，`Onboarded` 把页面送回 `/onboarding`；P6 的设置页显示名回弹）。审查者用临时测试复现 | 记下已答复的写的次数；读出去之后有写答复了，丢弃读的答复，保留写的。加测试（读挂起 → 记录一步答复 → 读答复旧值，`me` 仍是写的答复；没有写时读照常生效）。反向对照：去掉这个判断，测试失败 |
| M2 | Minor | 会话暂不可用（`starting` 之后续期失败、`/me` 取不到）时页面只有"重试"，用户菜单要等账户加载才出现：共用电脑上服务器宕机时没有办法退出 | "会话暂不可用"加"退出"：令牌管理器的退出尽力通知服务器（8 秒超时），无论如何删除记录。加测试：续期与退出都答 503，点"退出"，记录被删，到登录页 |
| M3 | Minor | 两处行为没有测试守住：`GuestOnly` 的 `starting`（加载中）与 `unavailable`（去 `next`）改成显示表单，全部测试通过（带记录的标签页打开登录页会闪出表单，用户可能再建一个会话）；删掉 `providers.tsx` 的 `onErrorRetry`（SWR 退回默认策略，4xx 也重试），全部测试通过：移交第 3 项只有纯函数的判定表守着 | 加三个守卫测试：`starting` 的登录页显示加载中、没有表单；`unavailable` 的登录页去 `next`，显示会话暂不可用；`/me` 先答 429、`Retry-After: 1`，页面自己在 2 秒内再取一次并显示（SWR 的默认策略至少等 2.5 秒）。反向对照：两个分支改成显示表单、去掉 `onErrorRetry`，对应的测试失败 |
| M4 | Minor | 字段下方的错误不在 live region 里，提交之后焦点也不动：本地检查失败或 422 时读屏用户听不到反馈（WCAG 4.1.3）。表单上方的错误有 `role=alert`，没有问题 | `useFocusOnInvalid(failures)`：每次提交失败之后焦点移到表单中第一个 `aria-invalid` 的字段，读屏读出它与它的错误（`aria-describedby`）；登录、注册与个人资料一步都用它。测试断言空字段、422 的字段得到焦点。反向对照：去掉移动焦点，测试失败 |
| N1 | Nit | `onErrorRetry` 以 `revalidate({ retryCount })` 重试，丢掉了 SWR 传入的 `dedupe: true`：重试赶上进行中的请求时会并行再发一次 | 原样传入 SWR 的选项 |
| N2 | Nit | 守卫为了 `pendingSteps` 导入注册表，注册表静态导入每一步的组件，整个引导 UI 进了入口的包；M2、M3 的步骤也会跟着进去 | 注册表中步骤的组件用 `React.lazy`，引导页用 `Suspense` 包着它。构建之后 `ProfileStep` 在自己的 chunk 里，入口不再含它 |
| N3 | Nit | `.oxlintrc.json` 对 `root.store.ts` 的放行已经没有用处（它不再导入 api-client），只是放宽了规则 | 删掉 |
| N4 | Nit | 明文切换按钮的 `aria-label` 随状态变（显示、隐藏），同时又有 `aria-pressed`，读屏读出两重状态 | 标签固定为"显示密码"，状态只在 `aria-pressed`；测试断言 `aria-pressed` |
| N5 | Nit | 拷贝时 nerve 改成 server 留下缺冠词的句子（`token-manager.ts` 两处，三个测试文件各一两处）；`auth-middleware.test.ts` 的请求体还是 Nerve 的 `first_name`（openapi-fetch 的泛型不做多余属性检查，类型检查没抓到） | 改为 "the server"；请求体改为 `display_name` |
| N6 | Nit | 通用的 `Loading` 放在 `session-unavailable.tsx` 里，注册页从那里导入；它也不是状态区域 | 移到 `components/loading.tsx`，是 `<output>`（隐含 `role=status`）；首页的加载中也用它，文案键合并为 `status.loading` |
| N7 | Nit | ① 422 的问题全在字段上时不显示上方的错误，但表单没有的字段会被静默吞掉（例如个人资料一步的记录答 422 `step`）；② 任何 `TypeError` 都说成"连不上服务器"，提交路径上的代码缺陷会被当成网络问题，也不留日志 | ① `formErrors(error, t, shown)`：只有问题全在表单显示的字段上时才不显示上方的错误，否则显示"有些内容不符合要求"；加用例。② 不做：浏览器里 `fetch` 的网络失败就是 `TypeError`，消息因浏览器而异，分不出代码缺陷；代码缺陷由单元测试与端到端的控制台检查（未处理的异常、控制台错误）去抓 |

## 对有意决定的判断

| 决定 | 判断 |
|---|---|
| 拷贝令牌管理器、续期锁、认证中间件与测试，只改名、改注释 | 同意；逐文件 diff 证实行为未改 |
| 会话装配不在导入时启动，`main.tsx` 显式 `start()`；只有 `src/session/` 创建客户端，oxlint 守着 | 同意 |
| 每个 `loginId` 一代 `RootStore`，`AppProviders` 带 key 重新挂载，SWR 的键不带 `loginId` | 同意；同一 `loginId` 的 starting、unavailable、signed-in 不换代，有测试证明 |
| 守卫是唯一决定去处的地方，页面不自己跳转 | 同意 |
| `next` 去掉同源解析，只留三条规则 | 同意；推理成立：三条规则保证第二个字符既不是 `/` 也不是 `\`，URL 解析器只能进入路径；会被解析器删掉的制表符、换行属于 C0，已整体拒绝；`trim()` 去掉的范围是解析器剥离范围的超集；百分号编码与全角斜杠在路径里不是特殊字符。守卫都用 `replace`，不会走 `push` 失败之后的 `location.assign` |
| 文案表与契约逐码核对 | 同意 |
| 声明式的控制台检查（`expectConsole`、`failedToLoad`） | 同意 |
| A4 以"续期代数 0..n 各一次"证明串行，有无 `navigator.locks` 各一遍 | 同意 |

## 文档与代码的不一致

审查者在已知的偏差之外列出 8 处，全部处理（P5 文档第 7 节第 2–12 项，正文已同步）：

1. `SessionDeps` 的 `onStorage` 收键名、`locks` 只要 `request`、`baseUrl` 换成 `client`：3.2 已改。
2. `GuestOnly` 的 `starting` 与 `unavailable`：3.5 的路由树已改。
3. `Loading` 不在 3.1 的文件表里：随 N6 移到 `components/loading.tsx`，3.1 已列。
4. `OnboardingStep.title` 收窄为 `` `onboarding.${string}.title` ``：3.7 已改。
5. `AuthStore.signIn(email, password)`：3.6 已改。
6. `signedInPage(tokens, baseURL?)` 由故事注册账户，另有几个辅助函数：3.9 已改。
7. A9 的 422 用 101 个字符：3.9 已改。
8. 第 7 节与浏览器实测没写：审查之后补写。

## 没有失败的反向对照

- `GuestOnly` 的 `starting`、`unavailable` 改成显示表单；删掉 `onErrorRetry`（M3，已补）。
