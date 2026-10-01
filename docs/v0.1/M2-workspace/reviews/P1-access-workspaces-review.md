# M2/P1 权限框架与创建工作区：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m2-p1-access-workspaces`（`main...6442c09`，S1–S6 与 image-smoke 的断言），对照 [01-P1-access-workspaces.md](../01-P1-access-workspaces.md)、各 Step 计划、[M2 总设计](../00-M2-design.md)与 [M1 移交](../handoffs/M1-identity.md) |
| 审查方式 | 独立审查者在 `git archive` 的快照上实测，仓库不动：<br>• 门禁：`make check`（vitest 429 个）、`make gen-check`、`make e2e`（49 个通过；S3 因快照没有 `.git` 答 `commit: "unknown"`，与代码无关），W1、W2 另跑 `--repeat-each=3`，6 次全部通过；`make image-smoke` 会覆盖本地镜像标签，没跑（作者在修复之后跑过）；<br>• 在接好线的应用与真实数据库上探测 `%00`、`%FF`、`a%00b` 的 slug；8 个账户并发创建同一 slug（1 个 201、7 个 409）；<br>• 反向对照：改生成的 SQL 去掉 `deleted_at`、`ended_at` 的过滤，只跑矩阵（见 N2） |
| 日期 | 2026-10-01 |
| 结论 | 修复后合并。规则表、纯函数的判定、经端口读事实、矩阵与覆盖检查的框架符合设计；模块隔离、端口在消费方、组合根的拆分与一处清理都成立；创建的顺序（开关 → 422 → 事务：共享账户行 → 工作区 → 成员）与全局加锁顺序一致，与停用不会死锁。<br>1 项 Major、3 项 Minor、9 项 Nit、14 处文档偏差：全部在合并前处理。4 个疑问：Q1–Q3 采纳建议，Q4 不改 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| M1 | Major | `getWorkspace` 遇到含 NUL 或不是 UTF-8 的 slug 答 500：路径参数经 `PathValue` 反转义，用例不判断格式就查库，PostgreSQL 报 22021。任何登录的账户都能稳定触发 500 与 ERROR 日志；P2–P3 按 `{slug}` 寻址的操作会原样继承；现有的参数测试只覆盖有类型的参数。设计 3.7 写的是 404 | `slugPattern` 导出为 `domain.ValidSlug`，`GetWorkspace` 先判断，不合格式答 `workspace.not_found`、不查库。app 测试加 `"\x00"`、`"\xff"`，断言 store 没被调用。`apitest.Operation` 加 `TextCases`（每个绑定任意字符串的路径或查询参数，取 NUL 与 `0xFF` 各一次）与 `ExampleBody`（结构检查接受的请求体）；整个程序的测试 `TestFreeTextParametersDoNotAnswer5xx` 带令牌与请求体逐个发送，不许 5xx，之后的操作自动纳入。反向对照：去掉判断，app 测试与整个程序的测试都失败（`%00`、`%FF` 答 500）；`TextCases` 不排除有格式的参数、`ExampleBody` 不判空，各自的单元测试失败 |
| N1 | Minor | `app.Store` 是五个方法的胖端口，三个读用例各只用一两个；identity 按用例拆成窄端口；P2–P4 还要往里加，每个替身都得全部实现 | 拆成 `WorkspaceCreator{CreateWorkspace, AddMember}`、`WorkspaceFinder`、`MembershipLister`、`SlugChecker`，postgres 的 `Store` 实现全部；`CreateWorkspaceDeps.Store` 改名 `Workspaces` |
| N2 | Minor | 设计 3.4 说矩阵的"已删除"一列守住 `RoleOf` 不连表，但种子用 SQL 同时软删工作区与成员行：`FindWorkspaceBySlug` 或 `RoleOf` 去掉 `deleted_at` 的过滤，矩阵都还是绿的，只有仓储测试失败。"删除工作区同一事务软删成员"的不变量在 P1 没有任何测试证明 | 3.4 改写：这一列在本 Phase 只证明列表与读取看不到已删除的工作区，`RoleOf` 不连表靠仓储测试；P2 的义务写进 3.4 与第 7 节：`deleteWorkspace` 有了之后种子改经接口，加"同一事务软删成员行"的集成测试。列表另加 `w.deleted_at IS NULL`（Q2） |
| N3 | Minor | 覆盖检查的"公开豁免又有一行"分支没有反例（现有反例命中前一个分支）；`{…_id}` 的"不是 uuid""不是种子行"也没有 | `TestMatrixViolationsCatchesEachGap` 加三个反例。反向对照：删掉这个分支、让 `{…_id}` 的判断恒为假，对应的反例失败 |
| T1 | Nit | `ReservedSlugs.App` 的注释还写着"and public/'s top-level directories"，与名单文件和实现矛盾 | 删去 |
| T2 | Nit | 配置、契约与错误的描述都说关闭创建之后由管理员用 `nervewiki workspaces create` 创建，这条命令 P4 才有 | 不改描述（它们按 M2 完成之后的状态写）；P1 第 7 节记下：P4 之前关闭开关没有创建途径，v0.1 发布在 M2 之后 |
| T3 | Nit | `checkWorkspaceSlug` 的描述说 reserved 是"a path of the site"，`[reserved]` 段的名字现在还不是 | 改为 "a path of the site, or held for one" |
| T4 | Nit | `ShareAccount` 事务之外的测试后半段（之后还能锁账户行）是空断言：事务外的 `FOR SHARE` 本来就随语句释放，去掉检查它也通过 | 删去后半段；前半段证明检查存在 |
| T5 | Nit | `webui.AssetsDir` 的注释出现"no workspace may take"，平台包里的业务概念 | 删去；理由在名单文件与 `reserved_test.go` |
| T6 | Nit | web 的保留名单测试：以 `/` 开头的顶层路由得到 `""` 被悄悄丢掉；`path: ""` 的子路由不遍历 | 先去掉开头的 `/`；没有路径、路径为空或 `/` 的路由都遍历子路由。反向对照：`sign-in` 的路由改成 `/sign-in`，测试读出 `sign-in` |
| T7 | Nit | README 只写了 `auth.signup_enabled`，没写 `workspace.creation_enabled` | README 加"工作区"一节：创建的开关与 slug 的规则 |
| T8 | Nit | `seeded` 的注释说"行的请求用它们"，P1 没有行用到 | 改为"P2 起" |
| T9 | Nit | 创建的测试注释说"先共享、后写"，两份记录没有断言先后 | 注释指向证明先后的子测试（账户已停用时 store 一次都没被调用） |

## 疑问与判断

| # | 疑问 | 判断 |
|---|---|---|
| Q1 | 列表按码点排序（库是 builtin C.UTF-8），大写都在小写之前；W3 的落点"按名称排第一的"受它影响 | 采纳：`ORDER BY lower(w.name), w.name, w.id`，契约写"by name, case-insensitively"。仓储测试加小写开头的名称；反向对照：去掉 `lower`，测试失败。带重音的字母不另处理：不引入 ICU |
| Q2 | 列表已经连了 workspaces，加 `AND w.deleted_at IS NULL` 不花代价，矩阵也不再依赖种子 | 采纳。仓储测试加"工作区删除、成员行没删"的一行；反向对照：去掉这个条件，测试失败。`RoleOf` 保持不连表（N2） |
| Q3 | M1 修在平台（含 NUL 或非 UTF-8 的路径参数统一答 400）还是用例（404） | 用例。slug 是地址，地址错了就是"没有这个工作区"（设计 3.7）；平台不认识哪些字符串参数会进数据库。整个程序的测试兜住以后的参数 |
| Q4 | `ErrNotVisible` 没有码，用例忘了翻译会答出 `"code": ""` | 不改。用例必须答自己资源的 404 码；矩阵的不可见列逐格断言码，漏翻译以空码失败。给它带上平台码 `not_found`，漏翻译反而不会被发现 |

## 文档与代码的不一致

审查者列出 14 处，全部处理（P1 文档第 7 节第 1–9 项，正文已同步）：

1. 接口构造辅助在 `httpservertest`（D1）：审查者认为合理，`httpserver` 的内部测试导入 `apitest`，反过来会成环；另两条路（把内部测试改成外部、把辅助放进生产包）都更差。P1 文档 2、3.1、3.3、第 4 节，M2 总设计第 7 节已改。
2. 保留名单不收 `public/` 的顶层目录（D2）：3.6 已改。
3. 矩阵的数据由 SQL 写入（D3）、"已删除"一列守住什么（D4）：3.10、3.4 已改。
4. CHECK 的反例（D5）：3.5 已改。
5. 端到端的函数名（D6）、`deps.go` 的签名与 access 没有构建函数（D7）、矩阵多的一行（D8）、文件清单（D9）：已改。
6. M2 总设计把锁下的邮箱放在 P1（D10）：第 7 节已改为 P3。
7. `identity.account_not_found` 不声明的理由（D11）：P1 第 7 节第 7 项。
8. 总体设计 13.1 第 3 条与 M2 总设计第 1 节第 4 条（D12）：改为"需要判定的操作"，两份文档的变更记录各加一行。
9. M1 的修复（D13）、第 7 节的内容（D14）：已写。

## 没有失败的反向对照

- `FindWorkspaceBySlug`、`RoleOf` 去掉 `deleted_at` 的过滤：矩阵绿，仓储测试失败（N2；P2 的集成测试补上不变量）。
- `ShareAccount` 去掉事务检查：后半段的断言通过（T4，已删去）。

## 没能验证的风险

- 审查者没跑 `make image-smoke`；作者在修复之后跑过，通过。
- 本地 `make check` 在修复之后两次因 Docker Desktop 映射容器端口超时而失败（同一台机器上另有项目的 testcontainers 在频繁起容器），与代码无关；`go test -race -p 3` 与其余各步分别跑过，全部为绿。
