# M2 工作区：收尾审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | `main` 的 `173ee86`（M2 的六个 Phase 全部合并、P6 文档与 M2→M3/M4/M7 的移交写好之后），M2 的改动是 `0eb3b15..173ee86`（372 个文件，+29822/−734）。对照[文档约定](../../../README.md)的"M 完成"、[M2 总设计](../00-M2-design.md)、六个 Phase 文档与审查记录、[M1 移交](../handoffs/M1-identity.md)与[总体设计](../../v0.1-design.md)中涉及 M2 的部分 |
| 审查方式 | 三位独立审查者并行（A 后端、B 前端与端到端、C 完成标准与文档），各在自己的 `git archive` 快照上做探针，仓库与别的 Docker 容器都没动过：<br>• **A**：`make lint-go`、`go test -race -count=1 -p 3 ./...`（38 个包，705 个测试、955 个子测试）、`server/tools` 的测试、`make gen-check`；workspace、access、identity、bootstrap、platform/jobs 另跑 `-race -count=3`（1344 次测试）；13 个交错另跑 `-race -count=10`（430 次运行）；deadcode（不带 `-test` 时 63 个不可达函数，都是测试辅助包、只给组合检查用的导出与 M0 预留的平台件；带 `-test` 时 0 个）。反向对照 52 项，探针 3 个（跨模块子行的清理、外键检查越过排队的独占等待者、sqlc 的模块范围）；<br>• **B**：`make lint-web knip`、vitest 679 个（另连跑 3 次）、`make build-web`、`make build`、`make e2e` 82 个；`stories/workspace` `--repeat-each 3`（102 个）、全部故事 `--workers 1`。反向对照 vitest 88 项、e2e 9 项（每项重新 `make build`），探针 2 个；<br>• **C**：文档与代码逐处核对约 960 处（路径、标识符、测试名、提交哈希、数字与行为说法），不符约 35 处；反向对照 6 项。<br>`make image-smoke` 会覆盖本地镜像标签，审查者都没跑（作者跑过） |
| 日期 | 2026-10-01 |
| 结论 | 处理完 A-I1 的文档、补上第 13 节之后可以收官：<br>• 门禁、全部故事、持续集成全绿；M2 的完成标准与 12.5 逐条满足（见下）；<br>• 规模：M2 新增 Go 生产代码约 4.8k 行、测试 8.9k 行，生成物 4.3k 行（含 TS 的类型），前端生产代码 2.3k 行（含文案约 200 行）、测试 2.3k 行，端到端 2.2k 行；<br>• 没有上帝文件：M2 新增的 Go 生产文件最大是 `workspace/app/ports.go`（225 行），TS 最大是 `invitations-section.tsx`（235 行），端到端最大是 `assert/workspace.ts`（233 行）；<br>• 加锁顺序与总设计第 8 节逐条一致，13 个交错都在、两种先后都跑、结果确定；没有发现过度防御或过度设计。<br>1 项 Important（A-I1，只改文档）、10 项 Minor、16 项 Nit 全部处理（Nit 中 A-N4 说明理由后保持现状）；收尾的事务（第 13 节、两处进度表、移交、本记录）一并完成 |

## 完成标准核对

| 标准 | 结论 | 证据 |
|---|---|---|
| 本 M 的故事全部通过（本地与持续集成）；之前各 M 的故事仍通过，S2 与用到 `registerOnboarded` 的故事随首页与引导调整 | 满足 | 本地 82 个，另跑 `--repeat-each 3`、`--workers 1` 通过；持续集成的 e2e 任务为绿。S2 落在创建页并核对用户菜单的版本；A3、A4、A6、A9 随引导调整（A9 是 "Step 1 of 2"） |
| 故事表每一行、每个版本都有端到端测试，并做到表里写的每一句 | 满足（修复后） | W1–W12 共 12 个文件、34 个测试，B 逐句核对。W4 的页面版本原来删除的工作区只有管理员一人、没有邀请，没有调用 `expectInvitationsDeletedWith`，"之后所有成员都看不到它"对别的成员两个版本都没有断言；W1 没有发过不合格的 slug（B-M3），已补 |
| 对等验收：两个版本调用同一组断言；例外 | 满足（修复后） | `e2e/fixtures/assert/workspace.ts`，W4 的页面版本见上。W6、W7 的接口版本直接调公开的预览与注册接口；W2、W10 的命令行部分与 W12 只有一个版本；总体设计 10.1 的措辞已跟上（C-M5） |
| 本 M 建立的扩展点已建好，有测试证明注册者能挂上 | 满足 | 成员身份结束（移出、离开、停用三条路）、恢复（接受邀请、`reactivate-member`）、工作区删除：`workspace/extension_test.go` 与用例测试证明否决回滚并答出码、订阅者在事务内且失败即回滚、调用的时机；规则表与判定级别：`TestTheRuleTableIsTheModulesActions`；清理注册表：`bootstrap/purge_test.go` 两条与 `TestPurgeWorkspacesWaitsForTheirChildren`。A 的反向对照 A1–A8、B1–B3、E1–E5 全部失败。"组合根把注册者交给了模块"只到静态可达（P4 审查 Q1），已写进 M3 的移交第 1 项 |
| M1 的三个扩展点已注册，有经接口与经命令行的行为测试（13.1 第 21 条） | 满足 | 停用：`bootstrap/deactivation_test.go`（接口答 409、会话保留；`users deactivate` 退出码 1、账户照旧可用），F3、F4（两侧不交注册者）让 5 个、10 个测试失败；引导：`onboarding/steps.ts` 与 e2e 的 `onboardingSteps`（E1：去掉之后 13 个故事失败）；注册策略：`TestSignupPolicy`、`TestSignupClosedOpensToAnInvitation` |
| 权限矩阵覆盖每个需要判定的操作 | 满足 | 13 个操作有矩阵行；`previewWorkspaceInvitation`（公开）、`acceptWorkspaceInvitation`（按所持凭据）、`/workspace-slugs/{slug}`（路径参数不指向某个资源）、identity 与 instance 两个模块豁免并写明理由；覆盖检查自身有反例（`TestMatrixViolationsCatchesEachGap`）。C1–C3、R3 让对应的检查失败 |
| 12.5：用 PAT 完整操作 | 满足 | `TestEveryOperationAcceptsAPersonalAccessToken` 覆盖 14 个需要令牌的操作；W1、W3–W5、W8–W11 的 PAT 版本；按设计只能由命令行做的是关闭创建时的 `workspaces create` 与 `reactivate-member`（接口上的等价做法是重新邀请） |
| 12.5：扩展点 | 满足 | 12.4 中"建立于 M2"的行都已建好；12.4 的可见性一行补上"工作区的加入与改角色在 M2 没有事件"（C-M1），清理注册表一行补上跨模块的外键（A-I1） |
| 12.5：先写描述，再写代码 | 满足 | 带接口的 Step 提交都同时改 `api`；M2 总设计第 5 节与 `api/modules/workspace.yaml` 逐项一致（78 处，0 处不符） |
| 12.5：架构测试、depguard、前端静态检查 | 满足（修复后） | 都为零；"services 不导入 `app/`、stores 不导入页面"没有门禁（B-M4），已加进 oxlint |
| 12.5：本 M 没有 `open` 的 handoff | 满足（收尾后） | M1→M2 的移交 10 项都已落实（第 10 项转给 M7），改为 `done`；M2 写出的移交都在目标 M 的目录里 |
| 文档约定的"M 完成" | 满足（收尾后） | 六个 Phase 各有审查记录；本记录、第 13 节、两处进度表与 M2 总设计的状态在收尾时完成 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| A-I1 | Important | **清理注册表的契约"父表的清理器跳过仍被子行引用的行"跨模块做不到，而 M3、M7 的移交是照它写的。** 它靠 `PurgeWorkspaces` 的 `NOT EXISTS`，只能写本模块的表：sqlc 按模块限定（13.1 第 24 条），探针在 workspace 的查询里引用 `auth_sessions`，`sqlc generate` 报 relation does not exist。M3 的笔记本→工作区、M4 的节点→笔记本、M7 的附件→节点都是跨模块外键。探针（模拟别的模块的子表）：外键是 CASCADE 时，子表的清理器跳过一行之后，工作区的清理器去等它的锁；客户端期限到了，服务端的语句并不取消（A-Q1），持有者一放手就提交，子行被级联删掉，不是由它自己的清理器删的，到 M7 就是附件文件留在卷上。外键是 RESTRICT 时，删除答 23001，子行留着 | 只改文档，M2 的代码不变：跨模块的外键用 `ON DELETE RESTRICT`，每一行只由它自己模块的清理器删除；父行仍被引用时这一批删除失败、清理停下，River 重试，子行清掉之后完成；子行被另一次清理持有时父表的外键检查会等它提交。写进总体设计 13.1 第 6 条与 12.4、M2 总设计第 8、10 节、P4 文档 3.4、`jobs.Purger` 的注释、M3 移交第 4 项（另加一条：第一条跨模块外键到来时，检查指向被清理表、来自别的模块迁移的外键不是 CASCADE）与 M7 附件的移交第 1 项。另一条路（子模块提供"仍被引用"的端口，父表的清理器经它排除）要为一个后台任务建跨模块的查询，不取 |
| A-M1 | Minor | `LockWorkspacesOf` 改成 `ORDER BY slug DESC`，全部测试通过：两个测试的夹具里，id 的先后恰好是 slug 的倒序 | `TestLockWorkspacesOf` 加第三个工作区（创建顺序 zeta、acme、mike），id 顺序与 slug 的正序、倒序都不同。`slug DESC`、`slug`、`id DESC` 三项反向对照都失败 |
| B-M1 | Minor | P6 第 7 节与 P6 审查 Q7 说别的操作答出 `field.email.*` 时"由页面经 `texts` 换说法"，代码里做不到：`ProblemTexts` 的键只是 problem 码，字段错误按全局的 `field.<字段>.<码>` 查找 | 13.2 第 11 条写明：要么改写文案让两处都适用，要么到那时给 `formErrors` 加按字段码换说法的选项，现在不加。P6 第 7 节与审查记录的 Q7 改正 |
| B-M2 | Minor | 读写交错只在每个 store 的一个写上有测试：`WorkspaceStore` 的接受、改名、删除与离开，`MemberStore` 的移出，`InvitationStore` 的邀请，绕过 `changed()` 直接改列表，679 个测试全部通过。现在的代码没有错误行为，但 M3 的 store 会照这些写法来 | 三个 store 的交错测试按写表格驱动（`test.each`）：创建、接受、改名、删除、离开；改角色、移出；邀请、撤回。八个写各自"不计数"的反向对照都只让它那一行失败 |
| B-M3 | Minor | 见完成标准：W4 的页面版本比接口版本弱，"之后所有成员都看不到它"没有经接口或页面断言；W1 没有发过不合格的 slug | W4 的页面版本先经接口加一位成员与一份待接受的邀请，删除之后断言成员行与邀请一并软删、那位成员的列表里没有它、`GET` 答 404；接口版本对成员与访客同样断言；W1 发 `Not A Slug`，答 422 `slug: invalid_format`。反向对照：删除工作区时不删邀请，W4 的页面版本失败（修复之前它通过） |
| B-M4 | Minor | 依赖方向没有门禁：在 service 里导入 `../app/landing` 与 `../stores/context`，lint 照样通过 | `.oxlintrc.json` 加两条按目录的 `no-restricted-imports`：services 不导入 `app/`、`stores/`、`pages/`、`components/`、`onboarding/`，stores 不导入 `app/`、`pages/`、`components/`、`onboarding/`；更具体的规则会替换同名规则的选项，stores 一条连同 api-client 的限制一起写。七项探针（含多层的 `../pages/workspace/…`、store 导入 api-client 的类型）都被拦下，写进 13.2 第 6 条 |
| C-M1 | Minor | M3 的移交与 12.4 漏了"加入"与"改角色"两种成员身份变化：12.4 的可见性变化事件由 M3 建立、M5 用来关闭事件流，触发源包括成员身份；M2 在接受邀请插入新行（`joinAdded`）与改角色上都没有订阅者，而降为访客会失去 `workspace_access` 给的默认角色 | M3 的移交加第 6 项；12.4 的可见性一行加注；M2 总设计第 10 节的风险写明。现在不建事件：没有使用者 |
| C-M2 | Minor | README 说公开的只有四个认证接口与 `GET /instance`，漏了预览邀请 | 补上 `POST /api/v0/workspace-invitations/{workspace_invitation_id}/preview` |
| C-M3 | Minor | 邀请 MAC 密钥的派生 info 没有测试守住：改成刷新令牌的 info，两把密钥成了同一把，全部测试通过；改 info 还会让所有待接受的邀请链接一起失效 | `TestTheInvitationKeyIsPinned`：一个固定的签名私钥派生出的邀请密钥等于常量（期望值用 Python 按 RFC 5869 独立算过），配合 `TestTokenKnownAnswer`，令牌整条钉住。R6 的反向对照现在失败。派生密钥的约定写进 13.1 第 25 条 |
| C-M4 | Minor | 四条跨 M 的约定只写在 Phase 文档或 M 总设计里：新的顶层路由与保留名单（P1 3.6）；全局的字段文案（P6 Q7）；"`KEY SHARE` 越过排队的独占等待者"（P4 审查 Q6）；"关联字段只给 id"（M2 总设计引用，总体设计没有） | 13.1 第 26 条与 13.2 第 10 条；13.2 第 11 条；13.1 第 5 条；总体设计 6.1 的"数据格式" |
| C-M5 | Minor | 总体设计几处没跟上 M2：2.3 的本地存储键缺 `nwiki.workspace`；8.2 的共享内核缺名称规则、`module.go` 的说法不合 access；8.3 的"锁模式在 M2–M4 的设计中确定"；9.4 没提按工作区的 store；10.1 的例外只说认证接口 | 逐处改，记在总体设计第 15 节 |
| A-N1 | Nit | 迁移 00007、00008 的注释停在 P4 审查 T1 之前（"清理连带删除成员、邀请""清理的级联按工作区查"） | 改成"清理先删子行，级联是模块内的兜底"；只是注释，迁移器不校验已执行迁移的内容，schema 不变 |
| A-N2 | Nit | `create_workspace.go` 的注释引用 13.1 第 19 条，那一条说的是哈希与随机令牌 | 去掉引用 |
| A-N3 | Nit | 日志里 `user_id` 的含义不一：经接口的是调用者，命令行与 identity 是被操作的账户 | 不改代码；13.1 第 10 条写明现行的含义 |
| A-N4 | Nit | 三个测试文件超过 400 行（`app/invitations_test.go` 459、`adapter/postgres/invitations_test.go` 435、`members_test.go` 427） | 保持：每个只有一个主题，拆开反而要在文件之间找同一个用例的测试（与 M1 收尾的 N10 相同） |
| B-N1 | Nit | `InvitationLink` 手写成 `{id, token}`，与生成的 `SignupInvitation` 相同（13.2 第 3 条） | 改为它的别名 |
| B-N2 | Nit | 关闭创建时的说明写"成员邀请你之后"，只有管理员能邀请 | "工作区的管理员邀请你之后"（两种语言） |
| B-N3 | Nit | M2 总设计第 10 节说邀请页"只改变会话，不自己跳转"，接受之后页面自己进入工作区 | 改写，与 13.2 第 10 条一致 |
| B-N4 | Nit | 角色菜单选中当前角色时不发请求，没有测试（Radix 的单选项选中已选的一项也会回调） | 改角色的测试加一步；去掉这个判断时它失败 |
| B-N5 | Nit | 离开工作区、接受邀请进入工作区之后焦点也落到 body，M3 的移交只写了删除 | M3 移交第 5 项、P5 第 7 节、13.2 第 17 条一并写明 |
| B-N6 | Nit | e2e 夹具的注释说"每个函数都期待成功"，`preview`、`tryAccept` 返回原始答复；"注册、完成引导、经邀请加入"在 W8 是本地函数，W4、W5、W9 各内联一遍 | 改正注释；`joinOnboarded` 挪进 `fixtures/invitations.ts`，四个故事共用 |
| B-N7 | Nit | 常规页的注释多一个空格 | 改正 |
| C-N1 | Nit | README：`GET /instance` 的字段、命令表没有 `workspaces …`、`reactivate-member` 的失败、撤回邀请的路径参数 | 改正 |
| C-N2 | Nit | M2 总设计：两个事件的值没写执行者；`NewDatabaseFrom` 在 P1 就有了；M1 移交第 7、8 项没写"已落实" | 改正 |
| C-N3 | Nit | M3 移交第 1 项读起来像把停用注册者本身换成 nil（那样两条行为测试会失败） | 补上"`Workspaces` 拿到的" |
| C-N4 | Nit | Phase 文档与计划的残留：P1 的 `identityDeps` 签名；P2 规则二"复用同一个计数"（P4 另写了 `ListStandings`）、`NewProfiles` 在 P3 改名、`WaitForLockWaitsOn` 的签名；P3 的"成员离开"（W9 离开的是访客）；P4 的 `ErrSoleAdminOf` 所在文件；P5 的夹具签名；P6 引号里的文案是转述；P1、P2 第 7 节没写合并提交；P2-S4 计划与 P2 3.10 的说法相反 | 正文改写或加指向；P2-S4 改正；其余计划是实施之前写的，不改 |
| C-N5 | Nit | 来源与移交之间缺链接：P5 审查 T11、P4 审查的 M7 移交、P2、P4、P5 第 7 节的"留给之后的" | 补上链接 |

## 疑问与判断

| # | 疑问 | 判断 |
|---|---|---|
| A-Q1 | 连接池用 pgx 默认的 `DeadlineContextWatcherHandler`，没有 `lock_timeout`、`statement_timeout`：客户端期限到了，服务端在等锁的语句照样完成 | 平台（M0）的行为，M2 的锁等待都排在短事务之后、有上限，不阻塞收官。写进 M2 总设计第 10 节的风险：M5（长连接）或 M12（实测）时评估 `CancelRequestContextWatcherHandler`，或给服务的角色设 `lock_timeout` |
| A-Q2 | 锁后的 `deleted_at`、`ShareWorkspace` 的共享锁、删除工作区也删除已结束的成员行，只由仓储测试守住 | 不改：用例层的锁下重读会掩盖前两类，后几类都有专门的仓储测试。"删除已结束的成员行"那一条是承重的：回归之后，已结束的成员行会让 `NOT EXISTS` 永远留住这个工作区 |
| A-Q3 | 移出时邮箱不加锁读：与 `users set-email` 并发时，新地址名下一份管理员亲手发出的待接受邀请可能留下 | 不改：结果与"移出之后再邀请"相同 |
| A-Q4 | 跨模块外键的检查要不要现在加进 `purge_test.go` | 不加：现在没有这样的外键，检查什么也查不到；写进 M3 的移交 |
| B-Q1 | 邀请 id 被改成非 UUID 时，邀请页显示通用的错误加重试 | 不改：截断的链接先丢掉片段，页面已经说"链接不完整"；只有手改 id 才会走到这里 |
| B-Q2 | `pageWatch.apiRequests` 只记路径 | 现在只有 W6 需要完整地址；之后的 M 也有带秘密的请求时，让 `pageWatch` 记完整地址 |
| B-Q3 | e2e 漏掉 `workspace` 步骤时只有 13 个故事失败 | 足够：没有工作区的故事（S2、A9 等）守住了同步 |
| B-Q4 | 改名表单打开期间别处改了名，保存会改回去 | 不改：与一般的表单一致，最后答复的就是服务端的状态 |
| C-Q1 | 两个代码提交把 `docs` 列为范围（README 与移交随代码提交）；`6442c09` 不是审查修复，后缀只写到 Phase | 接受：随代码改的文档列入范围符合约定的写法；`6442c09` 是 P1 合并之前补的 image-smoke 断言 |
| C-Q2 | W3 的两个版本都不断言数据库 | 接受：W3 测的是读（列表、落点、404），其中的离开与删除是准备步骤，落库的结果由 W4、W9 断言 |
| C-Q3 | `access.New` 不能进命令行的组合 | 合理：服务器管理员的命令本来就不判定。写进 13.1 第 3、22 条 |
| C-Q4 | 起新的顶层名字时，同名的未删除工作区怎么处理 | 机制留给用到它的 M，义务写进 13.1 第 26 条 |
| C-Q5 | M9 的 `/mcp` 要不要单写移交 | 不必：加了路由而名单没跟上时，`reserved_test` 会失败（R4） |

## 第 13 节

M2 总设计第 8 节"横切约定"列出的约定（权限与矩阵、加锁顺序、扩展点的注册、公开页面的例外、清理；前端的按工作区的 store、外壳、`texts`），经三位审查者对照代码核实，连同审查中发现只写在 Phase 文档里的约定，补进[总体设计](../../v0.1-design.md)第 13 节：

- 13.1：第 3 条（权限：规则表、操作名与 `Actions()` 的并集、不经规则表的三类、`Grant`、新的判定级别）、第 4 条（`ErrNotVisible` 换成自己的 404、码的次序）、第 5 条（加锁顺序：第一条语句、父行的锁模式、锁后的 `deleted_at`、停用的交替、外键检查与改邮箱，落实 P3 审查 Q1 与 P4 审查 Q6）、第 6 条（软删除与清理，含 A-I1）改写；第 9、10、11、18、22、23 条补充；新增第 25 条（派生的密钥与令牌）、第 26 条（顶层路径与保留名单）。
- 13.2：第 1 条（写的依次发出、计数、去重、删除类的 404）、第 6 条（依赖方向）、第 7 条（加载失败与重试的测试）、第 10 条（守卫、外壳与公开页面）、第 11 条（字段码的文案）、第 12 条（`texts`、`ConfirmDialog`）改写或补充；新增第 15 条（按工作区的数据）、第 16 条（工作区的外壳）、第 17 条（列表与行内控件）。9.4 同步补上按工作区的 store。
- 13.4：第 3 条（命令行的夹具、`workspace.ts`、页面版本的数据、故事之间不共用数据、页面操作的夹具）、第 4 条（交错测试的写法）、第 5 条（自由文本参数）补充；新增第 6 条（权限矩阵）。

## handoff

- **M1→M2**：[10 项](../handoffs/M1-identity.md)逐项对照代码都已落实：停用的第一个注册者与两条行为测试、否决者的码、`ShareActiveAccount`（事务之外报错、锁下的邮箱）、注册策略、引导的新步骤、首页、接口测试的构造辅助、组合根的拆分、加锁顺序与停用的交错 9–13（条文在收尾补进 13.1 第 5 条）；第 10 项已转给 M7。改为 `done`。
- **M2→M3**：[M3 的移交](../../M3-notebook/handoffs/M2-workspace.md)按审查意见改写：第 1 项的措辞（C-N3）；第 4 项加上跨模块的外键与它的检查（A-I1）；第 5 项的焦点包括离开与接受（B-N5）；新增第 6 项，加入与改角色没有事件（C-M1）。
- **M2→M4**：[页面树的清理顺序](../../M4-pages/handoffs/M2-P4-purge-page-tree.md)，核对无误。
- **M2→M7**：[只投递的客户端](../../M7-assets-transfer/handoffs/M2-P4-insert-only-client.md)核对无误；[附件的清理](../../M7-assets-transfer/handoffs/M2-P4-attachment-purge.md)第 1 项改为附件的外键用 RESTRICT（A-I1）。

`M2-workspace/handoffs/` 没有 `open` 的事项。

## 文档与代码的不一致

审查者列出的全部处理：
- **总体设计**：2.3、6.1、8.2、8.3、9.4、10.1（C-M4、C-M5）；12.4 的可见性一行与清理注册表一行；12.6 M2 已完成；第 13 节见上；第 15 节补记。
- **M2 总设计**：状态；第 4、8 节事件的值带执行者；第 6 节 `NewDatabaseFrom`；第 7 节 M1 移交全部落实；第 8 节清理与横切约定；第 10 节清理、邀请页的措辞、客户端期限的风险；第 11、12 节。
- **Phase 文档、计划与审查记录**：C-N4、C-N5 的各处；P4 文档 3.4（A-I1）；P6 第 7 节与审查记录的 Q7（B-M1）；P5 第 7 节（B-N5）。
- **README**：C-M2、C-N1。
- **代码注释**：`jobs.Purger`（A-I1）、迁移 00007 与 00008（A-N1）、`create_workspace.go`（A-N2）、`general-page.tsx`（B-N7）、e2e 的 `fixtures/invitations.ts`（B-N6）。

## 反向对照

会失败的（审查者与作者分别做过）：
- **A**：扩展点的否决、订阅的时机与失败（A1–A8、D12）；规则表的并集（B1–B3）；矩阵（C1–C3、G4、G5）；加锁（D1–D4、D6–D11、D13–D16、G6、G7，多数同时让相应的交错失败；D3、D4、D7、D15 只有仓储测试失败，见 A-Q2）；清理（E1–E5、G3）；组合与命令行（F1–F4、F6）；别的 SQL（G1、G2）。
- **B**：外壳的 key 与转走、`wasRemoved`、各 store 的读写交错与去重、按工作区缓存、邀请页在守卫之外与令牌只在片段里、`texts` 的三处入口、保留名单、引导的步骤，vitest 82 项；e2e 8 项（E1–E4、E6–E9）。
- **C**：规则表并集的两个方向、矩阵少一行、`[server]` 多一段、增长路径的第一条语句（R1–R5）。
- **作者在修复中做的**：C-M3（info 改成刷新令牌的）；B-M4 七项探针；B-M2 八项（每个写不计数）；B-N4；B-M3（删除工作区时不删邀请，W4 的页面版本，e2e）；A-M1 三项（`slug DESC`、`slug`、`id DESC`）。

审查时改了却没有测试失败的，处理之后：
- A 的 D5（`ORDER BY slug DESC`）现在失败（A-M1）；F5、F7、F8（把交给模块的注册者换成空）现在没有注册者，测不了，已写进 M3 的移交第 1 项。
- B 的 U10、U11、U12、U16、U20（五个写不计数）与 U37（选当前角色也发请求）现在都失败；E5（去掉外壳的 key）在 e2e 层仍不失败，没有故事从一个工作区的子页直接转到另一个，由 vitest 守住。
- C 的 R6（邀请的 info）现在失败（C-M3）。

## 核实修复

- 修复在分支 `m2-closeout`（`c36533a`），合并 `c1f7ee3`。本地 `make check`（vitest 685 个）、`make gen-check`、`make e2e`（82 个；改过的 W1、W4、W5、W8、W9 另跑 `--repeat-each 3`，69 个）为绿，上文的反向对照都按预期失败，改动全部还原；合并提交上 `make e2e` 与 `make image-smoke` 为绿（工作区里有未跟踪的 `.claude/`，image-smoke 报 `modified=true`）。
- 修复后的持续集成（run 36860051240）四个任务为绿。

## 没能验证的风险

- 只用 macOS 上的 headless Chromium：Firefox、Safari 的行为，窄屏布局，读屏的实际朗读没有验证。
- A-I1 的影响在模拟的子表上探的；M3、M4、M7 的真实表形状（附件的外键指向谁、删文件时怎么持锁）要到那时复核。RESTRICT 下"失败即停"会让同一次运行里排在后面的清理器推迟，没有在多张表的情形下实测。
- A-Q1 的 pgx 行为只在清理语句上观察到；HTTP 请求期限到了之后锁还被持有多久，没有测量。
- 大数据量下的清理（`deleted_at` 没有索引，每批顺序扫描）、多实例下 River 定时任务的重叠，都没有验证（与 P4 审查相同，见 M3 的移交第 4 项）。
- 交错的确定性只在本机、Docker Desktop 下以 `-count=10` 验证；读写交错只在 store 层用确定的顺序证明，没有在浏览器里复现时间窗口。
