# M2/P1 权限框架与创建工作区：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M2/P1 权限框架与创建工作区 |
| 状态 | 进行中 |
| 基线 | `0904ed9`（M2 总设计提交之后的 main） |
| 上级文档 | [M2 总设计](00-M2-design.md) 第 4、5、7、8 节；[M1 移交](handoffs/M1-identity.md)第 3、7、8 项；[总体设计](../v0.1-design.md) 3.2、6.1、7、13 |

---

## 1. 基线

M1 留下的：
- identity 模块：账户、会话、PAT、停用的扩展点（没有注册者）、`ShareActiveAccount`。
- 平台：sqlc、River、限流、认证接入。
- 整个程序的测试：路由正好是契约的操作；公开操作；每个需要令牌的操作都接受 PAT；参数与请求体的破坏答 400。
- 组合根 `bootstrap/app.go`：332 行，组装与生命周期在一个文件里。
- 测试里各自构建 `httpserver.NewAPI` 的有三处。

本 Phase 接手的 M1 移交：
- 第 3 项：`ShareActiveAccount` 的事务检查。锁下的邮箱随 P3 的接受邀请再做。
- 第 7 项：接口测试的构造辅助。
- 第 8 项：组合根的拆分。

## 2. 目标与范围

**目标**：工作区的第一批能力（创建、列表、读取、检查 slug）跑在权限框架之上。规则表、判定、权限矩阵与覆盖检查一次建好，后面每个 Phase 只需加行。

**做**：
- 组合根拆分；`apitest.NewAPI`；`postgres.InTx` 与 `ShareActiveAccount` 的事务检查；`CheckName` 移到 `shared`。
- `shared` 的权限端口；access 模块。
- 迁移 `workspaces`、`workspace_members`；slug、名称的规则与保留名单；保留名单与前后端路由的一致性测试。
- `listWorkspaces`、`createWorkspace`、`getWorkspace`、`checkWorkspaceSlug`。
- `workspace.creation_enabled` 与 `GET /instance` 的 `workspace_creation_enabled`。
- 权限矩阵的框架、覆盖检查、本 Phase 四个操作的行；操作名与规则表的一致性测试。
- e2e：W1 与 W2 的接口版本（W2 只含创建开关）。

**不做**：
- 改名、删除、成员的操作，两个扩展点（P2）。
- 邀请、锁下的邮箱、注册策略（P3）。
- `nervewiki workspaces` 命令、停用的注册者、清理（P4）。
- 页面（P5、P6）。

## 3. 设计

### 3.1 文件

```
server/
  migrations/sql/00006_workspace_workspaces.sql、00007_workspace_workspace_members.sql
  migrations/schema_test.go                       新的约束与索引名、CHECK 的反例
  sqlc.yaml                                       workspace 一条
  internal/shared/authorize.go                    WorkspaceRole、Action、Target、Grant、Authorizer、ErrNotVisible
  internal/shared/name.go                         CheckName（从 identity/domain 移来）
  internal/platform/postgres/tx.go                InTx
  internal/platform/httpserver/apitest/api.go     NewAPI：测试用的接口与宽松的桶
  internal/platform/config/                       workspace 一节
  internal/modules/access/
    module.go                                     New(Deps) shared.Authorizer；RuleKeys()
    domain/rules.go、decide.go                    规则表与判定（纯函数）
    app/ports.go、authorizer.go                   事实端口 WorkspaceMemberships；Authorizer
  internal/modules/workspace/
    module.go                                     New(Deps)、Register、Actions()
    memberships.go                                NewMemberships(pool)：access 的事实端口
    domain/workspace.go、slug.go、reserved.go、reserved_slugs.txt、actions.go、errors.go
    app/ports.go、create_workspace.go、get_workspace.go、list_workspaces.go、check_slug.go
    adapter/postgres/（store、queries/workspaces.sql、gen）
    adapter/http/（handler、gen、main_test.go）
  internal/modules/identity/adapter/postgres/users.go   ShareAccount 在事务之外报错
  internal/modules/instance/                      workspace_creation_enabled
  internal/bootstrap/
    app.go                                        生命周期：run、startJobs、close
    wire.go                                       newApp：组装；失败时一处清理
    deps.go                                       各模块 Deps 的构建函数
    actions_test.go                               各模块操作名的并集 = 规则表的键
    reserved_test.go                              保留名单的 [server] 段 = 根路由器的顶层路径
    permission_matrix_test.go、_coverage_test.go、_seeded_test.go、_workspace_test.go
api/modules/workspace.yaml、api/openapi.yaml、api/modules/instance.yaml
deploy/runtime-grants.sql
web/apps/web/src/app/reserved-slugs.test.ts       保留名单的 [app] 段 = routes.tsx 的顶层静态段
e2e/fixtures/assert/workspace.ts；e2e/stories/workspace/w1-create-workspace.spec.ts、w2-creation-switch.spec.ts
```

### 3.2 组合根的拆分（M1 移交第 8 项）

- `app.go` 只留生命周期：`app` 结构、`run`、`startJobs`、`close`、`awaitDatabase`、`warnIfExposed`。
- `wire.go` 的 `newApp` 依次做：读私钥、建连接池与迁移器、装配模块、建接口与路由器。连接池与迁移器建好之后，失败的清理由一个 `defer` 统一负责（返回值是命名的错误），不再在三个地方各写一遍"关闭迁移器、关闭连接池"。
- `deps.go` 是各模块 `Deps` 的构建函数，例如 `identityDeps(cfg, pool, logger, limiter, registrants)`。新加一个模块，就加一个构建函数与 `newApp` 里的两三行。
- 纯粹的移动与提取，行为不变，现有测试原样通过。

### 3.3 测试的构造辅助（M1 移交第 7 项）

- `apitest.NewAPI(t, apitest.APIOptions{Authenticator, PublicOperations, RequestTimeouts, MaxBodyBytes})` 返回 `*httpserver.API`。
  - 日志丢弃；限流的三个平台桶用一个用不完的桶；请求期限 5 秒。
  - `MaxBodyBytes` 为零时取 1 MiB。
- `apitest` 属于 httpserver 这个平台包，导入 httpserver 不违反"平台包互不导入"。
- 三处现有构造（identity 的 `module_test.go` 与 `adapter/http/handler_test.go`、instance 的 `adapter/http/handler_test.go`）与 workspace 的新测试都改用它。`httpserver` 自己的 `api_test.go` 测的就是 `NewAPI` 的参数，保持原样。

### 3.4 权限端口与 access 模块

`shared/authorize.go`（只依赖标准库）：

- `WorkspaceRole`：字符串类型，取值 `admin`、`member`、`guest`；`WorkspaceRoles()` 列出这三个。M3 另加 `NotebookRole`，不与工作区的角色共用一个类型：两套词汇里都有 `admin`。
- `Action string`：操作名，如 `workspace.read`。各模块在 `domain/actions.go` 声明常量与 `Actions()`。
- `Target{WorkspaceID}`；`Grant{WorkspaceRole}`。M3 各加笔记本的字段。
- `Authorizer.Authorize(ctx, actor, action, target) (Grant, error)`。每次调用都在 `ctx` 携带的事务里读事实，不缓存。
- 两种拒绝：
  - `ErrNotVisible`：`KindNotFound`，没有码。用例以 `errors.Is` 认出它，答自己资源的 404 码。
  - `shared.Forbidden()`：403 `forbidden`。
- 没有规则的操作是内部错误（500）：什么都不允许。

access 模块：

- **`domain/rules.go`**：
  - `Level` 只有 `LevelWorkspace`；`Rule{Level, Workspace []shared.WorkspaceRole}`。
  - 规则表由函数返回，不是包级变量（`gochecknoglobals`）；`RuleFor(action)`、`RuleKeys()`。
  - 本 Phase 只有一行：`workspace.read`，三种角色。
- **`domain/decide.go`**：`Decide(rule, Facts)` 是纯函数。
  - 不是有效成员 → `ErrNotVisible`；
  - 角色不在规则的集合里 → 403；
  - 否则返回 `Grant`。
  - 数据库的 CHECK 之外的角色值什么都不允许：集合里没有它。
- **`app/ports.go`**：`WorkspaceMemberships.RoleOf(ctx, workspaceID, userID) (role, ok, err)`。
- **`app/authorizer.go`**：先查规则，再经端口读一次事实，然后 `Decide`。
- **`module.go`**：`New(Deps{Memberships}) shared.Authorizer`；`RuleKeys()`。
- **事实端口的实现**：`workspace.NewMemberships(pool)`，只凭连接池构造（总体设计 13.1 第 11 条）。
  - 查询经 `postgres.DB(ctx, pool)` 进入调用方的事务。
  - 读的是有效、未删除的成员行。工作区删除时成员行随之软删除（P2），所以"已删除的工作区"不必再连表判断；矩阵的"已删除"一列守住这一点。
- **装配**：组合根先建 `access.New(...)`，再把它交给 `workspace.New(Deps{Authorizer})`。运行时互相调用，导入上没有环。
- **一致性测试**：`bootstrap/actions_test.go` 核对各模块 `Actions()` 的并集等于 `access.RuleKeys()`。少了规则、多了规则，测试都会失败。

### 3.5 数据

**`workspaces`**：

| 列 | 类型与约束 |
|---|---|
| `id` | `uuid PRIMARY KEY` |
| `slug` | `varchar(48) NOT NULL`，`workspaces_slug_check CHECK (slug ~ '^[a-z0-9_-]{1,48}$')` |
| `name` | `varchar(80) NOT NULL`，`workspaces_name_check CHECK (name <> '')`；其余规则在领域 |
| `created_by_id`、`updated_by_id` | `uuid NOT NULL REFERENCES users` |
| `created_at`、`updated_at` | `timestamptz NOT NULL` |
| `deleted_at` | `timestamptz` |

- 部分唯一索引 `workspaces_slug_key ON (slug) WHERE deleted_at IS NULL`：删除之后 slug 立即可以再用。
- 删除者记在 `updated_by_id`（P2）。

**`workspace_members`**：

| 列 | 类型与约束 |
|---|---|
| `id` | `uuid PRIMARY KEY` |
| `workspace_id` | `uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE`（物理清除时连带，P4） |
| `user_id` | `uuid NOT NULL REFERENCES users`（v0.1 不物理删除账户） |
| `role` | `text NOT NULL`，`workspace_members_role_check CHECK (role IN ('admin', 'member', 'guest'))` |
| `ended_at` | `timestamptz`：移出、离开、停用的时刻；恢复时清空 |
| `created_by_id`、`updated_by_id` | `uuid NOT NULL REFERENCES users` |
| `created_at`、`updated_at` | `timestamptz NOT NULL` |
| `deleted_at` | `timestamptz`：只随工作区删除 |

- 部分唯一索引 `workspace_members_workspace_id_user_id_key ON (workspace_id, user_id) WHERE deleted_at IS NULL`：每对（工作区，账户）只有一行，恢复改这一行。
- `workspace_members_user_id_idx ON (user_id) WHERE deleted_at IS NULL AND ended_at IS NULL`：我的工作区，以及停用时（P4）的列举。
- `workspace_members_workspace_id_idx ON (workspace_id)`：物理清除时，外键的连带查找用不上部分索引。

**其他**：
- 指向 `users` 的外键不建索引：v0.1 不物理删除账户，这些外键检查不会走反向查找。
- `sqlc.yaml` 加 workspace 一条，只列 00006、00007。
- `runtime-grants.sql` 给两张表 DML。
- `schema_test` 列出新的约束与索引名，CHECK 的反例：大写的 slug、49 个字符、空名称、第四种角色。

### 3.6 slug、名称与保留名单

- **slug**：`^[a-z0-9_-]{1,48}$`，不做规范化：大写或空白都是 `invalid_format`，不会悄悄改成小写。建好之后不能修改（总体设计 3.2）。
- **名称**：`shared.CheckName("name", s, 80)`，与显示名同一条规则：去掉两端空白之后 1–80 个字符，不含改变周围文字读法的字符。
  - `CheckName` 从 `identity/domain` 移到 `shared`：第二个使用者出现了，测试随之移动。
  - 不做 Nerve 的"不含网址"：v0.1 没有显示工作区名称的公开页面，能看到名称的只有成员。
- **保留名单**只有一份：`workspace/domain/reserved_slugs.txt`（`embed`），分三段：
  - `[app]`：前端的顶层静态路由段。由 web 的 vitest 核对：等于 `routes.tsx` 中顶层静态段的集合，加上 `public/` 的顶层目录（目前没有）。
  - `[server]`：服务端在前端页面之外自己回答的顶层路径。由 `bootstrap/reserved_test.go` 核对：等于接好线的根路由器上，除 `/` 之外各模式的第一段，加上 `webui` 的资源目录 `assets`。
  - `[reserved]`：以后可能用到的顶层段，与另两段不重复。
- **本 Phase 的名单**：
  - `[app]`：`onboarding`、`settings`、`sign-in`、`sign-up`；
  - `[server]`：`api`、`assets`、`healthz`、`readyz`；
  - `[reserved]`：`admin`、`create-workspace`、`docs`、`help`、`invitations`、`mcp`、`static`。
  - `create-workspace` 与 `invitations` 在 P5、P6 有了路由时，从 `[reserved]` 移到 `[app]`；`mcp` 在 M9 移到 `[server]`。
- **解析**：每次调用解析内嵌的文本（几十行，代价可以忽略），不放包级变量。名单的格式错误由领域的单元测试守住。
- **以后加顶层路由的 M**：优先用 `[reserved]` 里的名字；否则同一次改动把新段加进名单，并在启动时检查有没有同名的未删除工作区（同 Nerve 3.10）。

### 3.7 接口与用例

`api/modules/workspace.yaml` 的四个操作，都是 Bearer：

| 操作 | 成功 | `x-problem-codes` |
|---|---|---|
| `GET /api/v0/workspaces`（`listWorkspaces`） | 200 `{data: Workspace[]}` | — |
| `POST /api/v0/workspaces`（`createWorkspace`），`{name, slug}` | 201 `Workspace` | `validation_failed`、`workspace.creation_disabled`、`workspace.slug_taken`、`identity.account_deactivated` |
| `GET /api/v0/workspaces/{slug}`（`getWorkspace`） | 200 `Workspace` | `workspace.not_found` |
| `GET /api/v0/workspace-slugs/{slug}`（`checkWorkspaceSlug`） | 200 `SlugAvailability {available, reason?: taken \| reserved \| invalid}` | — |

- **结构**：`Workspace {id, slug, name, role, created_at, updated_at}`，`role` 是调用者的角色。
- **路径参数 `slug`**：不在契约里加 `pattern`。不合格式的 slug 在 `getWorkspace` 答 404，在 `checkWorkspaceSlug` 答 `invalid`，都不是 400：slug 是地址的一部分，地址错了就是"没有这个工作区"。
- **`createWorkspace`**：
  1. 开关关闭时，403 `workspace.creation_disabled`（在检查取值之前）。
  2. 名称与 slug 的规则一次列出全部问题（422），在事务之前。
  3. 一个事务：
     - `ShareActiveAccount(调用者)`：账户已停用答 403 `identity.account_deactivated`；
     - 插入工作区：`workspaces_slug_key` 冲突译成 409 `workspace.slug_taken`；
     - 插入成员行，角色为管理员。
  4. 日志记 `workspace_id`、`user_id`。
  5. 回答带 `role: admin`。
- **`getWorkspace`**：
  1. 按 slug 读未删除的工作区（不加锁），没有就答 404；
  2. `Authorize(workspace.read)`：`ErrNotVisible` 答 404 `workspace.not_found`；
  3. 回答带 `Grant` 中的角色。
  - 读操作不开事务（总体设计 8.3）。
- **`listWorkspaces`**：调用者有效成员关系所在的、未删除的工作区，按名称再按 `id` 排序。
  - 只涉及调用者自己的成员关系，不经 `Authorizer`。矩阵仍有它的一行：每一列只看到自己的工作区。
- **`checkWorkspaceSlug`**：
  - 依次判断：格式不对 → `invalid`；在保留名单里 → `reserved`；有未删除的工作区占着 → `taken`；否则可用。
  - 任何登录的账户都可以问。它与创建时的 409 一样，透露 slug 是否被占用，这是取名所需。
  - 不经 `Authorizer`，矩阵把它的路径列为"不指向某个工作区"。
- **`GET /instance`**：加必填的 `workspace_creation_enabled`。前端的类型随 `make gen` 更新；S3 的断言随之调整。

### 3.8 配置

- 新的一节 `workspace`，只有 `creation_enabled`：布尔值，三种环境的默认都是 `true`，与 Plane 相同。
  - 关闭创建是部署者的选择：注册关闭时，账户由管理员创建或经邀请得到，让这些账户自己建工作区通常是想要的。
- 在 `LogValue` 中列出。组合根把它交给 workspace 的 `Deps` 与 instance 的 `Deps`，两处读同一个值。

### 3.9 `ShareActiveAccount` 的事务检查（M1 移交第 3 项）

- `postgres.InTx(ctx) bool` 报告 `ctx` 是否携带 `WithinTx` 的事务。
- identity 的仓储方法 `ShareAccount` 在事务之外返回故障（500，带说明）：没有事务，`FOR SHARE` 随语句结束，与停用不再串行。这是只靠注释提醒、测试未必能碰到的错误，所以变成一个必然的失败。
- 测试：事务之外调用得到错误；事务之内照旧。

### 3.10 权限矩阵

照 Nerve 的框架（M2 总设计第 6 节），按本仓库改写：

- **列**：管理员、成员、访客、从来不是成员、已结束的成员、已删除的工作区（调用者曾是它的管理员）。
- **准备数据**：
  - 账户经接口注册，取得令牌。
  - 工作区与成员行经 workspace 的仓储写入。
  - 本 Phase 还没有的写入（结束成员关系、删除工作区）暂由 SQL 代替，P2 有了用例之后换掉。
- **执行**：
  - 只读的格共用一份准备好的库。
  - 每个写的格各用一份副本：`pgtest.NewDatabaseFrom`，从准备好的库复制，从 Nerve 拷贝。
  - 每一格经 HTTP 断言状态码与 problem 码；行可以带 `check`，核对回答的内容。
- **覆盖检查**：
  - 契约中每个操作都要有行，除非它属于豁免的模块（identity、instance）或是公开的。
  - 路径参数必须指向本列的工作区，否则要列为"不是目标"并写明理由（本 Phase 是 `checkWorkspaceSlug` 的路径）。
  - 写方法的行必须标明 `write`。
  - 每种缺口各有一个反例测试。
- **本 Phase 的行**：
  - `listWorkspaces`：每列 200，`check` 核对只含该列的工作区。
  - `createWorkspace`：写，每列 201。
  - `getWorkspace`：前三列 200 并核对角色，后三列 404 `workspace.not_found`。
  - `checkWorkspaceSlug`：每列 200。

### 3.11 端到端

- `e2e/fixtures/assert/workspace.ts`：按表组织的断言，`workspaceRows`、`memberRows`，读数据库，只比较业务列。
- **W1 的接口版本**：
  - PAT 调用 `checkWorkspaceSlug` 的四种答复；
  - 创建：201，落库的工作区与管理员成员行；
  - 再建同一个 slug 答 409，保留的 slug 答 422；
  - `getWorkspace` 与 `listWorkspaces` 的回答。
- **W2 的接口部分**：`nervewikiWith` 关闭创建，创建答 403，`GET /instance` 答 `false`，数据库不变。
- 页面版本在 P5，命令行部分在 P4，加进同一个故事文件。

## 4. 实施步骤

分支 `m2-p1-access-workspaces`。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 平台与组合根：拆分 `bootstrap`、`apitest.NewAPI`、`postgres.InTx` 与 `ShareAccount` 的检查、`CheckName` 移到 `shared` | [P1-S1](plans/P1-S1-platform.md) |
| S2 | 权限框架：`shared/authorize.go`、access 模块 | [P1-S2](plans/P1-S2-access.md) |
| S3 | 工作区的数据与规则：两条迁移、sqlc、授权、schema 测试；slug、名称、保留名单与两边的一致性测试；事实端口的实现 | [P1-S3](plans/P1-S3-workspace-data.md) |
| S4 | 接口与用例：`workspace.yaml`、四个用例、HTTP 适配器、模块根、组合根的接线、配置与 `GET /instance`、操作名的一致性测试 | [P1-S4](plans/P1-S4-workspace-api.md) |
| S5 | 权限矩阵：框架、覆盖检查、本 Phase 的行 | [P1-S5](plans/P1-S5-matrix.md) |
| S6 | 端到端：W1、W2 的接口版本 | [P1-S6](plans/P1-S6-e2e.md) |

每个 Step 结束时 `make check` 为绿；S4 之后 `make gen-check` 为绿；S6 之后 `make e2e` 为绿。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元 | slug 的格式（大小写、长度、字符）；名称（`CheckName` 移动后的原测试）；保留名单的解析（未知段、段外的行、不合 slug 的名字、重复）；判定的表格（三种角色 × 规则，非成员，未知角色）；`Authorizer`：没有规则、端口出错、每次都读 |
| 集成 | 仓储：插入与 `workspaces_slug_key` 的翻译、列表的顺序与过滤（已结束、已删除）、`RoleOf` 在事务内读；CHECK 的反例；`ShareAccount` 在事务之外报错 |
| 契约 | 四个操作的每个码在 workspace 的 `adapter/http` 测试中答出（`identity.account_deactivated` 经端口的替身）；整个程序的测试自动覆盖新操作，包括 PAT |
| 架构 | access 只导入 `shared`；workspace 的适配器互不导入；新模块只经模块根接入 |
| 一致性 | 操作名的并集 = 规则表的键；`[server]` 段 = 根路由器；`[app]` 段 = 前端路由 |
| 矩阵 | 3.10 |
| 端到端 | W1、W2 的接口版本 |

**反向对照**（13.4 第 1 条）：
- 把规则表的 `workspace.read` 去掉访客 → 矩阵失败；
- 删掉 `[server]` 里的 `readyz` → 一致性测试失败；
- `getWorkspace` 改为先读工作区、不经 `Authorize` → 矩阵的"从来不是成员"一列失败；
- 去掉 `InTx` 的检查 → 事务检查的测试失败；
- 矩阵漏一行 → 覆盖检查失败。

## 6. 完成标准

- 第 2 节"做"的各项完成；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿，持续集成四个任务为绿。
- M1 移交第 7、8 项与第 3 项的事务检查已落实，在 M2 总设计第 7 节的表中核对。
- 审查记录 `reviews/P1-access-workspaces-review.md`；发现的问题已修复或明确移交。
- 本文第 7 节、M2 总设计的进度表已更新。

## 7. 结果

（完成后补写）
