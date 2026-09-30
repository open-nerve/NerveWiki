# M1 账户认证：总设计

| 项 | 内容 |
|---|---|
| 里程碑 | M1 账户认证（`M1-auth`） |
| 日期 | 2026-09-30 |
| 状态 | 进行中 |
| 依赖 | M0 |
| 上级文档 | [v0.1 总体设计](../v0.1-design.md) 第 6.1、6.2、7、9.4、12.2 节 |

---

## 1. 目标

M1 结束时：

1. **账户**：邮箱密码注册（prod 默认关闭）、登录、续期、退出、改密码、自助停用；服务器管理员的 `nervewiki users` 命令（`create`、`reset-password`、`set-email`、`deactivate`、`activate`）。
2. **凭证**：访问令牌（JWT，Ed25519）、刷新令牌（轮换与重复使用检测）、PAT。除了明确列出的公开操作，所有接口操作都要求 `Authorization: Bearer`；限流与认证之前的失败闸门。
3. **个人资料与偏好**：显示名；设备上的主题与语言（M0 已有）在个人设置页中集中呈现。
4. **新手引导**：步骤注册表（扩展点）与第一步"个人资料"。
5. **扩展点**（总体设计 12.4）：账户停用的否决者与事件，由 M2 注册；新手引导的步骤列表，由 M2、M3 注册。
6. **平台**：sqlc、River、客户端 IP、限流、认证接入、参数与请求体的整个程序测试；前端的令牌管理器与按会话分代的 stores。M0 移交给 M1 的 19 项全部落实或说明理由关闭（第 7 节）。

## 2. 范围

**做**：第 1 节各项，以及它们的接口描述、前端页面（登录、注册、新手引导、个人设置）与端到端故事。

**不做**：

| 事项 | 理由与去处 |
|---|---|
| 邮件（验证邮箱、找回密码） | v0.1 没有邮件通道（总体设计 6.2）：忘记密码由管理员 `users reset-password` |
| 服务端的偏好（主题、语言） | 属于设备，M0/P5 已定（`nwiki.theme`、`nwiki.locale`）；个人设置页只是集中呈现 |
| 时区 | v0.1 没有依赖服务端时区的功能，时间按浏览器的时区显示。不嵌入 `time/tzdata`，M0/P3 移交第 9 项因此关闭 |
| 会话列表、远程退出其他设备 | 改密码、停用、管理员重置密码会撤销其他会话；列表界面不在 v0.1 |
| 头像，`first_name` / `last_name` | Nerve 从 Plane 继承的字段，Nerve Wiki 只有显示名 |
| 按笔记本限定权限的 PAT | v0.2（总体设计第 14 节） |
| 长连接路由的认证 | M5 第一次用到，见 [M5 的移交](../M5-collab-editing/handoffs/M0-P1-sse-proxies.md)第 4 项 |
| 工作区、带邀请注册 | M2 |

## 3. 完成标准

- 本 M 的故事全部通过（本地 `make e2e` 与持续集成），M0 的冒烟故事仍然通过（S2、S4 随登录守卫调整，S3 随 `signup_enabled` 调整）：

  | 故事 | 内容 | 版本 |
  |---|---|---|
  | A1 注册 | 注册后进入新手引导；浏览器只保存 `login_id` 与刷新令牌；落库的账户、会话 | 页面、接口 |
  | A2 注册被拒 | 邮箱已占用 409、密码太弱 422、注册关闭 403 | 页面、接口 |
  | A3 登录 | 登录后回到 `next` 指向的页面，开放重定向被拒；错误的密码与不存在的邮箱答同样的错误 | 页面、接口 |
  | A4 续期 | 访问令牌到期后自动续期；两个标签页串行续期；没有 `navigator.locks` 时走 localStorage 租约 | 页面、接口 |
  | A5 重复使用检测 | 旧一代的刷新令牌被再次使用时撤销整个会话；伪造的旧代不能踢人下线；页面回到登录页 | 页面、接口 |
  | A6 退出 | 退出同步到所有标签页；退出后换另一个账户登录 | 页面、接口 |
  | A7 改密码 | 其他会话失效，当前会话保留；PAT 不受影响 | 页面、PAT 接口 |
  | A8 个人资料与偏好 | 改显示名；主题与语言在设置页切换并保持 | 页面、PAT 接口 |
  | A9 新手引导 | 注册后逐步完成引导，完成之后不再出现 | 页面、PAT 接口 |
  | A10 PAT | 创建要求当前密码、明文只显示一次、列表、`last_used_at`、撤销与过期之后答 401 | 页面、PAT 接口 |
  | A11 停用与启用 | 自助停用后所有会话失效、PAT 不能用；管理员 `users activate` 之后 PAT 恢复 | 页面、PAT 接口、命令行 |
  | A12 服务器管理员命令 | `users create`、`reset-password`（撤销全部会话与 PAT）、`set-email`；命令的输出与日志里没有密码 | 命令行 |
  | A13 会话清理 | 过期的会话由后台任务删除 | 后台任务 |
  | A14 登录限流 | 超出登录的桶答 429 与 `Retry-After`；认证之前的失败闸门 | 页面、接口 |

- **对等验收的例外**（修订总体设计 10.1、12.5）：发生在取得令牌之前的故事（A1–A6、A14），接口版本直接调用认证接口，不用 PAT；只能由命令行或后台任务完成的（A12、A13）只有那一个版本。
- 本 M 建立的两个扩展点已建好，并有测试证明注册者能挂上去（第 8 节）。
- 总体设计 12.5 的其余各项；M0 移交的 19 项全部处理（第 7 节）。

## 4. 关键决定

| 决定 | 选择 | 放弃的方案及原因 |
|---|---|---|
| identity 的表 | `users`（账户与资料：邮箱、密码哈希、显示名、是否可用、已完成的引导步骤）、`auth_sessions`、`api_tokens`。修订总体设计 7.2 | Nerve 的 `profiles`：里面是 Plane 的偏好（主题、语言、每周第一天）与写死的四个引导键，Nerve Wiki 的偏好在设备上，引导另有设计 |
| 新手引导的存储 | `users.onboarding_steps`：已完成步骤的 id 集合（`text[]`，id 的格式与个数由数据库检查）。前端的注册表决定有哪些步骤、顺序与界面，一步完成时经接口记录它的 id | Nerve 的四个固定布尔键加 CHECK：M2、M3 每加一步都要改 M1 的表与契约。按实际状态推导：M3 的"一次性创建第一个个人笔记本"要记住做过，删掉笔记本之后不能再建 |
| 密码规则 | 8–128 个字符；拒绝常见密码名单中的（NCSC 常用密码前 10 万中 8 位以上的，数据取自 SecLists 的固定提交，附校验和），以及主干（去掉两端的非字母）是名单条目主干的（`Summer2024!`）；拒绝主干等于邮箱 @ 之前部分的主干；拒绝同一字符的重复与只有空白的；不要求字符组合。哈希的是密码的 NFKC 形式（NIST SP 800-63B 5.1.1.2） | Nerve 的"大小写、数字、特殊字符各一"：为配合 Plane 的强度提示，NIST 800-63B 不建议，而且把用户推向可预测的变形 |
| PAT | 名称必填；**创建时要求当前密码**；列表不分页（总体设计 6.1 的小集合）；字段 `name`、`expires_at`、`last_used_at`。改密码不撤销 PAT；管理员重置密码撤销全部 PAT | 不要求密码：泄露的刷新令牌可以换成永不过期的 PAT，而 PAT 要交给 agent 使用、拥有账户的全部权限。改密码时撤销 PAT：会让正在工作的 agent 突然失效，PAT 在设置页逐个列出，由用户决定 |
| 停用的扩展点 | 否决者（账户行锁之后、写入之前，同步调用，返回 `*shared.Error` 即整体回滚并答出它的码）加事务内事件"账户已停用"（写入之后，订阅者在同一事务内处理）。两者只凭连接池构造，HTTP 与命令行共用。增长路径（M2 的加入工作区等）经 identity 提供的端口对账户行取 `FOR SHARE`，在锁下确认账户可用 | Nerve M3 规划的单一同步端口：只能接一个接收方；没有 `FOR SHARE` 的约定时，并发的加入与停用会留下没有可用管理员的工作区（Nerve 的实验证明过） |
| 令牌与浏览器的键 | `nwk_rt_`、`nwk_pat_`（总体设计 2.3）；刷新令牌的 MAC 密钥由签名私钥经 HKDF 派生（info `nervewiki refresh-token mac v1`）；浏览器 `nwiki.auth`，续期锁 `nwiki.auth.refresh`，租约 `nwiki.auth.lease` | — |
| 签名私钥 | `auth.jwt.private_key_file`（PKCS#8 PEM 的 Ed25519）。prod 必须提供；dev、test 为空时生成临时密钥并告警（重启后访问令牌失效）；日志只记是否设置。换钥即重启，不做 kid 与 JWKS | — |
| 前端的会话 | `src/session/` 是唯一创建 API 客户端的模块：令牌管理器、续期锁、认证中间件从 Nerve 拷贝（纯逻辑，与框架无关），会话装配全新编写。每次登录一代 `RootStore`，`<AppProviders key={loginId}>` 随之重新挂载 | Nerve 的模块级单例：导入即启动，原地重建唯一的 `RootStore` 时旧 store 会在 `await` 之后碰到新会话 |
| 前端路由 | `/sign-in?next=`、`/sign-up`、`/onboarding`、`/settings/{profile,security,tokens}`；未登录访问其他页面转到 `/sign-in?next=<原路径>`；引导未完成转到 `/onboarding` | — |

## 5. 接口

新增 12 个操作（M1 共 13 个，加上 M0 的 `GET /instance`），都在 `api/modules/identity.yaml`：

| 方法与路径 | 认证 | 成功响应 |
|---|---|---|
| `POST /api/v0/auth/register` | 公开 | 201 `AuthTokens` |
| `POST /api/v0/auth/login` | 公开 | 200 `AuthTokens` |
| `POST /api/v0/auth/refresh` | 公开 | 200 `AuthTokens` |
| `POST /api/v0/auth/logout` | 公开 | 204 |
| `GET /api/v0/me` | Bearer | 200 `User` |
| `PATCH /api/v0/me` | Bearer | 200 `User` |
| `POST /api/v0/me/onboarding-steps` | Bearer | 200 `User`（记录一个完成的步骤，幂等） |
| `POST /api/v0/me/change-password` | Bearer | 204 |
| `POST /api/v0/me/deactivate` | Bearer | 204 |
| `GET /api/v0/me/api-tokens` | Bearer | 200 `{data: ApiToken[]}` |
| `POST /api/v0/me/api-tokens` | Bearer | 201 `ApiTokenCreated`（含一次性的明文） |
| `DELETE /api/v0/api-tokens/{token_id}` | Bearer | 204 |

- `GET /instance` 增加 `signup_enabled`，登录页据此显示注册入口。
- 认证类接口不返回完整资源（总体设计 6.1 的例外，照 Nerve）：注册、登录、续期返回令牌，退出、改密码、停用、撤销返回 204。
- 模块码：`identity.signup_disabled`、`identity.email_taken`、`identity.invalid_credentials`、`identity.account_deactivated`、`identity.refresh_token_invalid`、`identity.current_password_incorrect`、`identity.api_token_not_found`；命令行另有 `identity.account_not_found`、`identity.email_unchanged`。`deactivateMe` 的 `x-problem-codes` 由停用的否决者追加（第 8 节）。

## 6. 从 Nerve 拷贝的范围

原则同 M0：只拷贝本 M 用得到的；拷贝即接管，逐个文件审查、改名、删掉用不到的分支与指向 Nerve 文档的引用。

| Nerve 中的位置 | 处理 | Phase |
|---|---|---|
| `platform/httpserver` 的 `clientip.go`、认证与限流的接入（`api.go`、`limit.go`）、401 的 `WWW-Authenticate` | 拷贝，并入 M0 的 `api.go`，保留 M0 的改进（`errors.Join` 的参数校验、`LongLived`） | P1、P2 |
| `httpserver/apitest/operations.go` 的其余部分、两条写法规则 | 拷贝 | P1 |
| `platform/ratelimit` | 拷贝 | P2 |
| `platform/jobs`（River） | 拷贝；去掉启动失败的重试循环（M0 启动前已确认数据库可用） | P4 |
| `platform/clock/clocktest` | 拷贝，加锁（并发安全），列入架构测试规则 8 | P2 |
| `platform/postgres/pgtest/lockwait.go` | 拷贝（确定性的交错测试）；改为计数版 | P2（从 P3 提前） |
| `platform/config` 的列表解码、`netip.Prefix`、auth / ratelimit / jobs 三节 | 拷贝、裁剪 | P1、P2、P4 |
| `shared` 的 `Actor`、邮箱规则 | 拷贝；不拷贝 `url.go`、`timezone.go`、`cursor.go` | P1 |
| `modules/identity` | 拷贝、裁剪：去掉 `profiles`、时区、Plane 字段、PAT 分页、M3 加入的 `provide.go` 与 `accounts.go`；密码规则按第 4 节改写；停用改为扩展点 | P1–P4 |
| `migrations/sql` 的 identity 与 River 迁移、`schema_test.go` 的三个测试 | 按裁剪后的列重写 identity 的迁移；River 的迁移原样导出 | P1、P3、P4 |
| `sqlc.yaml`；`archtest` 的 `sqlc_test`、`rawsql_test`、`composition_test` | 拷贝；sqlc 进 `server/tools` | P1、P4 |
| `cmd/nerve/users.go`、`bootstrap/users.go`、命令的错误格式 | 拷贝、改名 | P4 |
| `deploy/runtime-grants.sql` 与它的测试 | 拷贝（第一批业务表与 River） | P4 |
| `tools/password-blocklist` | 拷贝，按第 4 节的规则重新生成名单 | P1 |
| 前端 `core/lib/auth/{token-manager,refresh-lock,auth-middleware}.ts`、`one-at-a-time.ts` 与它们的测试、测试替身 | 原样拷贝，改名；页面、会话装配全新编写 | P5 |
| 前端 `next` 的校验规则（`packages/utils` 的 `isValidNextPath`）与它的用例；problem 码的文案映射与它的契约测试（`authentication.helper`） | 按 Nerve Wiki 的路由与文案重写，用例拷贝 | P5 |
| `e2e/fixtures/{auth,users}.ts`、`settings-pages.ts` 的通用部分、`assert/identity.ts`；`signedInPage` 与记录的辅助函数（`recordOf`、`writeRecord`） | 拷贝、改写 | P1–P6 |

## 7. Phase 划分

每个 Phase 按[文档约定](../../README.md)执行：写 Phase 文档 → 实现 → 测试 → 审查 → 修复 → 合并。平台约定与会话安全分在两个 Phase，互不稀释审查；"默认要求令牌"的整个程序测试与第一个需要令牌的操作一起进入。

| P | 名称 | 交付 | 验证 |
|---|---|---|---|
| P1 | 身份基础与默认拒绝 | 迁移 `users`、`auth_sessions`；sqlc 与它的架构测试（`sqlc_test`、`rawsql_test`）、约束与索引名的数据库测试；`shared.Actor` 与邮箱规则；auth 配置节、签名私钥、`warnIfExposed`；JWT 与刷新令牌的 MAC、argon2 与并发名额、密码规则与名单；httpserver 的 `Authenticator`、公开操作清单、401 与 `WWW-Authenticate`、客户端 IP 与 `trusted_proxies`；apitest 的参数与请求体用例、两条写法规则；`register`、`getMe`；`instance` 的 `signup_enabled` | 整个程序测试：公开操作正好是契约中 `security: []` 的；没有令牌答 401；参数与请求体的破坏答 400；每个 problem 响应声明两个头。prod 默认关闭注册、缺私钥拒绝启动。e2e：A1、A2 的接口版本 |
| P2 | 会话与限流 | `login`（等时的不存在邮箱、快照重试）、`refresh`（重复使用检测、条件轮换、服务端期限）、`logout`；`platform/ratelimit`；认证之前的失败闸门；各个桶与 IPv6 前缀；`clocktest` | 续期的判定表（伪造旧代、真实旧代、换钥）、并发续期；登录耗时与邮箱是否存在无关；并发下失败闸门不超额；经可信代理的客户端 IP。e2e：A3–A6、A14 的接口版本 |
| P3 | 账户、PAT 与停用 | 迁移 `api_tokens`；`updateMe`、`onboarding-steps`、`changePassword`、`deactivateMe` 与停用的扩展点、`ShareActiveAccount`；PAT 的创建（要求密码）、列出、撤销，PAT 认证与 `last_used_at`；账户行锁协议；`lockwait`；oapi-codegen runtime 的例外 | 每个需要登录的操作都有 PAT 的测试；账户行锁的确定性交错测试；用测试替身证明否决会整体回滚并答出它的码、事件在事务内、`FOR SHARE` 挡住并发的停用。e2e：A7–A11 的 PAT 接口版本 |
| P4 | 管理命令与后台任务 | `nervewiki users` 五个命令、只凭连接池的管理组合、`composition_test`、密码的读取（终端不回显）；River 迁移、`platform/jobs`、停机顺序 HTTP → 后台任务 → 迁移器 → 连接池、会话清理任务；`runtime-grants.sql` 与测试 | 停机顺序的测试；命令行的组合到不了 HTTP 与 River；命令的输出与各级日志里查不到密码（含十六进制、base64）。e2e：A11 的命令行部分、A12、A13 |
| P5 | 前端会话、登录与引导 | `src/session/`（拷贝令牌管理器、续期锁、认证中间件，编写会话装配）；`RootStore` 分代与 `AppProviders` 的 key；oxlint 的放行移到会话模块；204 与 429（按 `Retry-After` 重试）；路由守卫；登录、注册、会话暂不可用三个页面；新手引导的步骤注册表与资料一步；顶栏的用户菜单（显示名与退出）；e2e 的 `signedInPage`、预期控制台输出的声明、冒烟故事的调整。令牌管理器的续期超时是 8 秒，与服务端配置校验的 `webRefreshTimeout` 相同（P2 审查） | vitest：拷来的认证测试（先确认运行环境）、会话装配、分代隔离。浏览器实测：局域网 HTTP（走租约）、两个账户两个标签页。e2e：A1–A6、A9、A14 的页面版本 |
| P6 | 前端个人设置 | 设置页布局；资料与偏好；安全（改密码、停用）；PAT 管理（一次性显示、撤销）；用户菜单中设置的入口 | e2e：A7、A8、A10、A11 的页面版本；本 M 与 M0 的全部故事通过 |

### M0 移交的落实

| 移交 | 项 | Phase |
|---|---|---|
| [P3 平台层](handoffs/M0-P3-platform.md) | 1 客户端 IP 与 `trusted_proxies`（列表解码）；3 认证的接入；5 `warnIfExposed`；7 sqlc 相关的架构测试；10 `*_file` 机密键的日志 | P1 |
| | 2 限流；8 测试用的固定时钟（加锁） | P2 |
| | 4 后台任务与停机顺序；6 命令行组合的检查 | P4 |
| | 9 时区数据库 | 关闭：M1 不做时区（第 2 节） |
| [P4 接口契约](handoffs/M0-P4-api-contract.md) | 1 401 与 `WWW-Authenticate`；2 公开操作清单；3 参数与请求体的整个程序测试；5 逐路由中间件的扩充（认证） | P1 |
| | 5 逐路由中间件的扩充（限流） | P2 |
| | 4 oapi-codegen runtime 的例外（第一个带路径参数的操作） | P3 |
| [P5 前端外壳](handoffs/M0-P5-web-shell.md) | 1 每次登录一代 `RootStore`；2 SWR 的缓存跟着一代走；3 429 的重试；4 客户端只在会话装配模块创建 | P5 |

每项在所在 Phase 合并时核对；M1 收尾时三份移交改为 `done`。

## 8. 本 M 建立的扩展点与约定

**账户停用：否决者与事件**（总体设计 12.4，M2 注册）：

- 注册方式：组合根把注册者传进 identity 的 `Deps` 与管理用的 `AdminDeps`；注册者只凭连接池构造。
- 否决者与订阅者收到同一个值 `Deactivation{UserID, Email, At}`，邮箱取自锁下读到的行。
- 否决者：`VetoDeactivation(ctx, d) error`，在账户行锁之后、任何写入之前按顺序调用；返回 `*shared.Error` 时整个停用回滚，接口答出它的码，命令行打印原因、退出码 1。注册者把自己的码追加到 `deactivateMe` 的 `x-problem-codes`。
- 订阅者：`AccountDeactivated(ctx, d) error`，在停用的写入之后、同一事务内逐个调用；返回错误同样整体回滚。
- 增长路径：任何让账户获得新的访问（加入工作区、接受邀请）的写事务，先经 identity 提供的 `ShareActiveAccount(ctx, userID)` 对账户行取 `FOR SHARE` 并确认账户可用；于是它与停用串行，否决者总能看到已提交的成员关系。
- M1 用测试替身证明：否决时整体回滚并答出码；事件在事务内、订阅者失败即回滚；持有 `FOR SHARE` 的事务让并发的停用等待，提交之后停用的否决者看到它的结果（自助停用与管理员的 `users deactivate` 两路都有）。

**新手引导的步骤列表**（总体设计 12.4，M2、M3 注册）：

- 前端的注册表（`web/apps/web/src/onboarding/steps.ts`）是一个有序列表，每一步 `{id, title, Component}`；注册者往列表里加一项，不改已有的步骤。组件用 `React.lazy`：守卫在每个页面都读注册表，只需要 id。
- 一步完成时调用 `POST /me/onboarding-steps` 记录它的 id；所有注册的步骤都记录过，引导即完成。以后新加的步骤会让已有账户再进入一次引导，只显示新步骤。
- id 的格式（小写字母、数字、下划线，最长 32）与个数上限由领域与数据库共同检查，服务端不认识具体的步骤。

**M1 建立、在收尾时补进总体设计第 13 节的约定**（暂定）：全局加锁顺序 `users → auth_sessions → api_tokens`；账户行锁用 `FOR NO KEY UPDATE`，凭证在锁外校验、锁下复核快照；哈希计算放在事务之外；日志只记账户、会话、令牌的 id，从不记邮箱与凭证；公开操作只由模块的清单声明；前端的会话规则（总体设计 9.4）按实现修订措辞。

## 9. 测试策略

| 层次 | M1 覆盖 |
|---|---|
| 单元 | 领域规则（邮箱、密码、显示名、PAT、引导步骤 id、续期判定表）；令牌的格式与解析；限流的桶；客户端 IP；表格驱动 |
| 集成 | 每个用例在真实 PostgreSQL 上的事务与并发：账户行锁的确定性交错、重复使用检测、并发续期、失败闸门；约束与索引名、CHECK 的反例；停机顺序 |
| 契约 | 每个操作的响应与声明的错误码；整个程序的测试：公开操作清单、默认要求令牌、参数与请求体的破坏、problem 响应的头 |
| 架构 | sqlc 的模块范围、原生 SQL 的禁用、命令行组合、测试辅助包只被测试导入 |
| 前端 | vitest：令牌管理器（单标签页、多标签页、会话变化）、续期锁、认证中间件、会话装配与分代、各页面的表单与错误显示 |
| 端到端 | A1–A14，页面版本、接口版本与命令行，调用 `e2e/fixtures/assert/identity.ts` 中按表组织的同一组断言（M1 建立这个目录） |

## 10. 风险

| 风险 | 应对 |
|---|---|
| 规模：Nerve 同类 M 的测试代码约为生产代码的 2 倍，平台部分约 3 倍 | 六个 Phase，每个都能独立验证；Phase 文档写明规模估计，超出时再拆 |
| 前端认证测试在 jsdom 下运行：`AbortSignal` 与 Node 的 `Request` 混用可能抛出，被当作网络失败吞掉 | P5 拷贝之后先在 jsdom 与 node 两种环境下跑通，再决定这批测试的运行环境。已解除：原样在 jsdom 下通过（P5 第 7 节） |
| 部署在反向代理之后却没配 `trusted_proxies`：所有人共用代理的 IP，一个人就能让所有人被限流 | 启动时告警一次；README 的部署一节写明 |
| localStorage 租约不是原子的（非安全上下文下的续期锁） | 沿用 Nerve 的分析：最坏情况是两个标签页各续期一次，其中一个触发重复使用检测、需要重新登录；安全上下文下用 `navigator.locks` |
| 常见密码名单的数据源（NCSC 原文件已下线） | 取自 SecLists 的固定提交，脚本与校验和进仓库，可复现 |
| 调高 argon2 参数后，登录耗时能区分停用账户与不存在的邮箱 | 沿用 Nerve 的登记：不存在的邮箱对同参数的哑哈希校验一次；参数只在配置中调整 |

## 11. Phase 进度表

| P | 名称 | 状态 | Phase 文档 | 审查 |
|---|---|---|---|---|
| P1 | 身份基础与默认拒绝 | 已完成 | [01-P1-identity-foundation.md](01-P1-identity-foundation.md) | [P1 审查](reviews/P1-identity-foundation-review.md) |
| P2 | 会话与限流 | 已完成 | [02-P2-sessions-ratelimit.md](02-P2-sessions-ratelimit.md) | [P2 审查](reviews/P2-sessions-ratelimit-review.md) |
| P3 | 账户、PAT 与停用 | 已完成 | [03-P3-accounts-tokens.md](03-P3-accounts-tokens.md) | [P3 审查](reviews/P3-accounts-tokens-review.md) |
| P4 | 管理命令与后台任务 | 已完成 | [04-P4-admin-jobs.md](04-P4-admin-jobs.md) | [P4 审查](reviews/P4-admin-jobs-review.md) |
| P5 | 前端会话、登录与引导 | 已完成 | [05-P5-web-session.md](05-P5-web-session.md) | [P5 审查](reviews/P5-web-session-review.md) |
| P6 | 前端个人设置 | 进行中 | [06-P6-settings.md](06-P6-settings.md) | — |
| — | M1 收尾审查 | 未开始 | — | — |

## 12. 变更记录

| 日期 | 修订 | 原因 |
|---|---|---|
| 2026-09-30 | 初版 | M1 启动 |
| 2026-09-30 | 第 4 节密码规则：名单另收条目的主干，拒绝同一字符的重复与只有空白的密码，哈希前做 NFKC 规范化 | P1 审查 M1、M4、N3：只收 8 位以上的条目时主干规则放过 `Qwerty123!` 一类；NFC 与 NFD 的同一密码哈希不同 |
| 2026-09-30 | 第 6 节 `lockwait` 从 P3 提前到 P2；第 7 节 P5 的续期超时写明 8 秒 | P2 的并发续期需要确定性的交错；P2 审查：前端的 8 秒要与服务端的配置校验一致 |
| 2026-10-01 | 第 8 节停用扩展点的签名按实现：否决者与订阅者收到同一个 `Deactivation` 值 | P3 审查：总设计写的事件名与否决者参数与代码不同 |
| 2026-10-01 | 第 3 节 A5 加页面版本；第 6 节拷贝清单补上 `next` 的校验、文案映射与 e2e 的 `signedInPage`；第 7 节用户菜单（显示名与退出）从 P6 提前到 P5 | [P5 文档](05-P5-web-session.md) 3.10：A5 的页面版本证明前端把会话被吊销变成回到登录页；A6 的页面版本需要退出 |
| 2026-10-01 | 第 8 节写明引导注册表的位置，步骤组件按需加载；第 10 节 jsdom 的风险解除 | P5 审查 N2：守卫导入注册表，静态导入的步骤组件会进入口的包；P5 第 7 节：拷来的测试原样在 jsdom 下通过 |
