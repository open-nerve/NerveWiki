# M1 账户认证：收尾审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | `main` 的 `e942906`（M1 的六个 Phase 全部合并、M1→M2 的移交写好之后），对照[文档约定](../../../README.md)的"M 完成"、[M1 总设计](../00-M1-design.md)与[总体设计](../../v0.1-design.md)中涉及 M1 的部分 |
| 审查方式 | 独立审查者在仓库的克隆上（后端、前端与端到端、文档三块另由三个分支并行核对，各在自己的克隆里做探针）：<br>• 跑 `make check`、`make gen-check`（干净工作区）、`make e2e`（另跑 `--repeat-each 3` 与 `--workers 1`）、`make image-smoke`，identity、bootstrap、platform、cmd 另跑 `go test -race -count=3`；查持续集成在 `2de91ff`（run 36781832994）与 `e942906`（run 36782059798）上的结果；<br>• 逐条核对 M1 的完成标准与总体设计 12.5；<br>• 跨 Phase 的接缝：配置 ↔ 组合根 ↔ 命令行，逐路由中间件 ↔ 认证 ↔ 限流 ↔ 模块，契约 ↔ 生成代码 ↔ TS 客户端 ↔ service ↔ store ↔ 页面，停用的扩展点 ↔ HTTP 与命令行，前端会话 ↔ 设置页，e2e 夹具；命名、错误模型、日志、加锁顺序、事务边界；<br>• 代码质量：文件规模、职责、防御性代码、死代码（deadcode）、依赖方向；<br>• README、M1 总设计、各 Phase 的第 3、7 节、总体设计与代码逐处核对（约 130 个路径、测试名与数字）；<br>• 全部 handoff 对照代码；<br>• 对照代码核实 M1 总设计第 8 节，起草总体设计第 13 节的条目；<br>• 用探针核实发现（临时测试、改动代码的反向对照），全部还原 |
| 日期 | 2026-10-01 |
| 结论 | 修掉 1 项 Important 之后可以收官：<br>• 门禁、全部故事、镜像、持续集成全绿：Go 测试 523 个（子测试 588 个，`-race`），vitest 406 个，e2e 47 个；<br>• 规模：M1 新增 Go 生产代码约 5.9k 行、测试 13.3k 行，前端生产代码 3.1k 行、测试 4.0k 行，端到端 2.5k 行；<br>• 没有上帝文件：Go 生产文件最大 381 行（M0 的 `bodyshape.go`），M1 最大的是 `bootstrap/app.go`（332 行）；TS 最大的是拷来的 `token-manager.ts`（377 行）；<br>• 死代码只有预留的扩展点与平台件；防御性代码都有真实的依据。<br>1 项 Important、9 项 Minor、13 项 Nit：Important 与 Minor 全部处理；Nit 处理 9 项，其余 4 项说明理由后保持现状。收尾的事务（第 13 节、两处进度表、移交、本记录）一并完成 |

## 完成标准核对

| 标准 | 结论 | 证据 |
|---|---|---|
| 本 M 的故事全部通过（本地与持续集成）；M0 冒烟 S1–S4 随守卫与 `signup_enabled` 调整之后仍通过 | 满足 | 本地 47 个，另跑 `--repeat-each 3`、`--workers 1` 通过；持续集成两次运行的 e2e 任务为绿 |
| 故事表每一行、每个版本都有端到端测试 | 满足（修复后） | A1–A6、A14 页面与接口；A7–A10 页面与 PAT 接口；A11 另有命令行一段；A12 只有命令行；A13 只有后台任务。A1、A3 的页面版本原来没调用落库断言（M3）、A10 的接口版本撤销用的是会话的令牌（N1），已补 |
| 对等验收的例外 | 满足 | A1–A6、A14 的接口版本直接调认证接口；A12、A13 各只有一个版本 |
| 两个扩展点已建好，有测试证明注册者能挂上 | 满足 | 停用：`identity/deactivate_test.go` 5 个测试（否决回滚并答出码、订阅者在事务内且失败即回滚、`FOR SHARE` 两个方向的交错），`admin_test.go` 覆盖管理员一路。引导：注入两步注册表的页面测试、步骤 id 的测试。组合根把注册者交给 `New`、`NewAdmin` 只有静态可达检查（探针：两处置空，测试全部通过），第一个注册者的行为测试写进 M1→M2 移交第 1 项 |
| M0 移交的 19 项全部处理 | 满足 | 逐项对照代码，见下文 handoff 一节 |
| 12.5：用 PAT 完整操作 | 满足 | `TestEveryOperationAcceptsAPersonalAccessToken`；A7–A11 的 PAT 版本；重新启用按设计只能由命令行完成 |
| 12.5：扩展点 | 满足 | 12.4 中"建立于 M1"的行都已建好；注册策略原来漏在表外（M5），已补；M1 没有要注册的扩展点 |
| 12.5：先写描述，再写代码 | 满足 | 每个 Step 把 `identity.yaml` 与实现放在同一个提交里；handler 实现由描述生成的 `StrictServerInterface`，各 Phase 文档 3.8 在实现之前定下了接口 |
| 12.5：架构测试、depguard、前端静态检查 | 满足（修复后） | 都为零；"组合根只导入模块根"没有规则（M4），已补 |
| 12.5：本 M 没有 `open` 的 handoff | 满足（收尾后） | M0-P3、M0-P4 两份移交各项都已落实，改为 `done`；M0-P5 原已 `done`；M1→M2 的移交在 M2 的目录里 |
| 文档约定的"M 完成" | 满足（收尾后） | 六个 Phase 各有审查记录；本记录、第 13 节、两处进度表与 M1 总设计的状态在收尾时完成 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| I1 | Important | 密码规则检查原文，哈希的却是 NFKC 形式：`ｐａｓｓｗｏｒｄ１２３` 通过规则，它的哈希能用 `password123` 校验成功；`ａｌｉｃｅ２０２６` 绕过邮箱主干规则；分解的 é×4 是 8 个 UTF-16 单位，通过长度规则，NFKC 之后只有 4 个字符。常见密码名单、邮箱主干、最短长度三条规则都被兼容字符绕过（审查者用临时测试证实） | `domain.CanonicalPassword`（NFKC）是唯一的定义：`PasswordRules.Check` 判断它，argon2 哈希它，注册、改密码、重置、`CreateUser` 四处因此都覆盖。domain 需要 `golang.org/x/text/unicode/norm`：archtest 让纯的层只多用 Unicode 规范化（与它导入的 `transform`），其余第三方照旧禁止，规则的用例同时断言 `x/text/language` 仍被拒绝。前端的本地长度检查同样数 NFKC 形式。测试：全角常见密码、全角邮箱主干答 `common_password`，分解的 é×4 答 `too_short`，全角的强密码通过；前端同样的用例。<br>反向对照：去掉规范化，域的测试失败；去掉 archtest 的例外，纯度与规则测试失败；前端去掉 `normalize`，测试失败 |
| M1 | Minor | 邮箱校验放过域名中的格式字符：Django 的域名字符范围包含 U+202E、U+200B、U+2066–2069，`alice@exa‮mple.com` 与 `alice@exa​mple.com` 都是合法地址。M2 会把邮箱展示给成员，邀请按地址比对 | `ValidEmail` 另拒绝格式字符（Cf：双向控制、零宽字符、软连字符），六个新的拒绝用例。现在没有生产数据，改起来最便宜。反向对照：去掉这个判断，测试失败 |
| M2 | Minor | 前端文案表的契约核对只查手写清单里的 11 个操作，新操作不会被查到（探针：契约加一个 `createWorkspace`，测试全部通过）；README 的说法比实际宽 | 改为取契约中的全部 `operationId`，只列出页面不显示其错误的 `refreshTokens`、`logout`，并断言这两个仍在契约中。README 与 13.2 第 11 条相应改写。反向对照：契约加一个带新码的操作，测试失败 |
| M3 | Minor | A1、A3 的页面版本没有调用与接口版本相同的落库断言（故事表写明"落库的账户、会话"），与 10.1、13.4 第 3 条、M1 总设计第 9 节的"同一组断言"不符 | A1 页面版本补 `expectNewSession`（UA 取 `navigator.userAgent`，IP 为 127.0.0.1）；A3 页面版本在被拒的登录之后 `expectNothingAdded`，成功之后 `expectNewSession` |
| M4 | Minor | archtest 没守住"组合根只导入模块根"：bootstrap 导入 `identity/adapter/postgres` 照样通过；同一模块的适配器互相导入也不报 | 加两条规则："bootstrap 只导入模块根"、"同一模块的适配器互不导入"（放在规则表末尾，已有规则的编号不变），各有放行与拒绝的用例。反向对照：bootstrap 导入 `identity/adapter/postgres`、`adapter/authn` 导入 `adapter/argon2`，仓库的规则测试失败 |
| M5 | Minor | 注册策略是扩展点表漏掉的一行，M1→M2 移交的说法不成立：端口 `AllowSignup(ctx) (bool, error)` 拿不到邀请与邮箱，注册的请求体也没有邀请（Nerve 是 `AllowSignup(ctx, email, *SignupInvitation)`）；`signup_enabled` 接了两条线（instance 直接取配置，identity 经 `signupSwitch`） | 总体设计 12.4 加一行"注册策略 \| M1 \| M2"，写明 M2 扩展端口的参数与请求体、是本表的例外；移交第 4 项改写，写明两条线要一起改。不在 M1 预先扩展端口：邀请的形状属于 M2 |
| M6 | Minor | M1→M2 移交的其他错漏：否决者的码只能在 **identity** 的 `adapter/http` 测试中答出（`apitest.Main` 按模块的描述核对）；漏了 `ShareActiveAccount` 答出的两个跨模块的码与前端缺的文案；漏了 `registerOnboarded` 只记录 `profile`、A9 的 "Step 1 of 1"；`deactivationRegistrants()` 没有参数 | 移交逐项改写（第 1–5 项）。`registerOnboarded` 改为记录 `onboardingSteps` 列出的全部步骤，注释指向前端的注册表，M2 加一步只改一行 |
| M7 | Minor | PAT 的撤销被称作"软删除"（迁移注释、P3 文档 3.2），与 7.1 的软删除（`deleted_at`、60 天物理清除）和 13.1 第 6 条（向清理任务登记）不一致 | 写明撤销凭证不是软删除：会话与 PAT 记下撤销时间，行保留，不登记清理（13.1 第 6 条、6.1 的 DELETE、迁移注释、P3 文档 3.2）。撤销要求密码、有限流，行的规模很小 |
| M8 | Minor | 长连接路由的移交与约定没跟上 M1：M1 之后逐路由链上还有请求信息（`LongLived` 上 `RequestMetaFrom` 是零值）、失败闸门与按凭证的限流，都在未导出的中间件里；`httpserver.Authenticator` 同时接受访问令牌与 PAT，而 6.3 说 MCP 只收 PAT | 13.1 第 14 条改写：写明中间件的完整顺序，第一个挂载长连接路由的 M 由平台导出复用这些部件的入口，不在模块里重写。M5、M9 的移交第 4 项补上这三项；M9 另写明是否拒绝访问令牌由它决定 |
| M9 | Minor | 总体设计与 M1 文档中多处没跟上实现：9.4（`oneAtATime` 的范围、"await 之后核对会话"、没写 `AppStores`）；13.1、13.2、13.4 的若干条；8.2（`idgen` 不存在、共享内核漏了邮箱规则、模块根的文件）；12.6 与 M1 总设计的状态；第 15 节 | 与第 13 节一起修订，写进总体设计第 15 节与 M1 总设计的变更记录。明细见"文档与代码的不一致" |
| N1 | Nit | A10 的接口版本中撤销、错误密码、过期三段用会话的访问令牌，e2e 里没有"用 PAT 撤销" | 撤销与最后的列表改用第二个 PAT |
| N2 | Nit | 13.4 第 3 条说"每个页面自动受监视"，实际只有 `page` fixture；另开的标签页由故事自己调用 | 13.4 第 3 条改写 |
| N3 | Nit | store 的加载方法不统一：`InstanceStore.fetch`，另外两个是 `load`；README 与 13.2 第 7 条写 `fetch` | 统一为 `load`，README 与 13.2 同步 |
| N4 | Nit | README 与 `root.store.ts` 说 `RootStore` 是唯一装配的地方，但同一文件的 `AppStores` 也装配 | 改为"`root.store.ts`（`AppStores` 与每代的 `RootStore`）" |
| N5 | Nit | 只为测试存在的接缝：`Session.dispose()`、`formatDate` 的 `timeZone` 参数、`completeOnboarding` 的 `step` 参数 | `step` 随 M6 的处置去掉。其余保持：`dispose` 让测试结束时不再跟随别的标签页（取消 storage 事件的订阅）；`timeZone` 让格式化的测试不依赖进程的时区设置，代价只是一个可选参数 |
| N6 | Nit | 创建令牌的 `<select>` 手写了 `FormField` 的接线 | 保持：出现第二个下拉框时再合并 |
| N7 | Nit | "认证之后账户不见了"答 401 的写法不一：`GetMe` 用 `shared.Unauthenticated()`，其余用 `unauthenticated(errUserUnknown)` | 统一为后者（带原因，进调试日志） |
| N8 | Nit | `adapter/http` 的 `RateLimiter` 接口只有一个实现，测试都用真实的 limiter | 保持：接口让 HTTP 适配器只依赖它用到的一个方法，改成具体类型的收益很小 |
| N9 | Nit | `bootstrap/app.go` 332 行，每加一个模块约多 20 行，出错时的清理重复三次 | 写进 M1→M2 移交第 8 项：M2 加模块之前把组装与生命周期分开 |
| N10 | Nit | 超过约 400 行的测试文件有 6 个（总体设计 8.1 要求说明理由或拆分） | `httpserver/api_test.go`（489 行）的认证测试移到 `api_auth_test.go`（与 `api_ratelimit_test.go` 对应），剩 397 行。其余保持：`interleavings_test.go`（500 行）把确定性的交错放在一起，对照着读正是它的用处；`load_test.go`、`app_test.go`、`apierrors_test.go`、`rules_test.go` 刚过 400 行，各自只有一个主题 |
| N11 | Nit | `httpserver/api.go` 说凭证键是 `session:<id>`，P3 起还有 `pat:<id>` | 改注释 |
| N12 | Nit | 续期的 8 秒前后端各一份，没有跨文件的测试（探针：两边各改为 12 秒，各自的测试失败） | 保持：两边注释互指，各自的测试钉住 8 秒；写进 13.1 第 15 条 |
| N13 | Nit | M1→M2 移交里有几项是所有后续 M 都要遵守的约定（注册者不叫 `New`、一处组合；扩展点约定的重复；守卫规则；组合检查的起点） | 移进第 13 节（13.1 第 21、22 条，13.2 第 10 条），移交只留 M2 要做的事 |

另有几处审查者没能验证：读密码的提示在 Linux 终端上的表现；README 中 River 残留 `_ccnew` 索引、每天记 WARN 的说法（P4 审查时按 River 源码写）；局域网 HTTP 与两个账户的浏览器实测（P5、P6 做过，这次没重做）。

## 第 13 节

M1 总设计第 8 节"暂定"的约定经审查者对照代码逐条核实（加锁顺序的第一条锁、`FOR NO KEY UPDATE` 与锁下复核、哈希在事务之外、日志只记 id、公开操作只由模块清单声明），连同第 8 节漏掉、M1 实际建立的横切约定，补进[总体设计](../../v0.1-design.md)第 13 节：
- 13.1：第 5 条（全局加锁顺序）、第 6 条（撤销凭证不是软删除）、第 10 条（日志）、第 11 条（模块入口）、第 12 条（纯的层的唯一例外）、第 14 条（逐路由中间件与长连接路由）改写，第 15、16 条补充（test 配置与前后端共有的量；运行时角色的授权与 River 的迁移）；新增第 17–24 条：默认拒绝与两种凭证、账户行锁、事务里只有锁与写、限流、扩展点的注册者、管理命令、后台任务、SQL 与约束。
- 13.2：第 1 条（会话与分代）、第 6 条（接口客户端）、第 7 条（加载与重试）、第 9 条（控制台）改写；新增第 10–14 条：路由与守卫、错误的文案、表单、只显示一次的秘密、日期。9.4 同步改写。
- 13.4：第 3 条（端到端）改写；新增第 4 条（并发与交错）、第 5 条（整个程序的测试）。

## handoff

- **M0→M1 三份**：逐项对照代码都已落实（客户端 IP 与列表解码、限流、认证的接入、后台任务与停机顺序、`warnIfExposed`、命令行组合的检查、sqlc 的架构测试、`clocktest` 加锁、私钥只记是否设置；401 与 `WWW-Authenticate`、公开操作清单、契约推导的三个整个程序测试、runtime 的例外、中间件的顺序；P5 的四项；时区一项按设计关闭）。[P3 平台层](../handoffs/M0-P3-platform.md)与[P4 接口契约](../handoffs/M0-P4-api-contract.md)改为 `done`（P3 的进展里"由 M5 决定"改为 M5、M9），[P5 前端外壳](../handoffs/M0-P5-web-shell.md)原已 `done`。
- **M1→M2**：[M2 的移交](../../M2-workspace/handoffs/M1-identity.md)按审查意见改写：第 1–5 项改正与补充（M6、M5），长期约定移进第 13 节（N13），加上组合根的拆分（N9）；邮箱的格式字符（M1）、archtest 的模块根（M4）、前端契约核对的操作清单（M2）已在 M1 修掉，不再移交。
- **M5、M9**：长连接路由的第 4 项按 M1 的中间件补充（M8）。

`M1-auth/handoffs/` 没有 `open` 的事项。

## 文档与代码的不一致

审查者列出的全部处理：
- **总体设计**：6.1 的 DELETE 与 13.1 第 6 条（M7）；8.2 去掉 `idgen`（ID 由标准库 `uuid.NewV7()` 生成）、共享内核加邮箱规则、模块根的文件与 `ports_<主题>.go`；9.4 改写；12.4 加注册策略；12.6 M1 已完成；第 13 节见上；第 15 节补记。
- **M1 总设计**：状态与进度表；第 8 节引导步骤的个数只由 CHECK 裁决；第 10 节代理告警的措辞（每种配置错误在第一次遇到相应的请求时各告警一次）；第 8 节的约定指向第 13 节。
- **Phase 文档**：P3 3.1 扩展点的类型别名在 `accounts.go`；P3 3.2 的"软删除"；P4 3.1 补上 `app/admin.go`；P4 第 2 节"理由见 3.4"改为 3.5。
- **README**：`RootStore` 与 `AppStores`（N4）、`load`（N3）、契约核对的范围（M2）、新增模块接口时公开操作的声明；Makefile 的 `image-smoke` 帮助补上"注册关闭"。
- **代码注释**：`httpserver/api.go` 的凭证键（N11）、迁移的"软删除"（M7）、`root.store.ts`（N4）。

## 核实修复

- 修复在分支 `m1-closeout`（`f8a9086`）：本地 `make check`（vitest 415 个）、`make gen-check`、`make e2e`（47 个；另跑 `--repeat-each 3`，141 个）、`make image-smoke`（在克隆上，`modified=false`）为绿；上文的反向对照都按预期失败，改动全部还原。
- 修复后的持续集成（run 36786129512）四个任务为绿。
