# M1 账户认证：Codex 对抗性深度评审

日期：2026-10-01。评审基线：`f873f69f6f111d0bdd5e248302ded7b4778f77c5`；改动范围：`bd775ab..f873f69`。本文中的源码行号均指这一基线。

## 1. 结论

**常规认证与账户生命周期基本闭环，现有门禁全部通过；故障恢复、表单在途编辑和少数契约／端到端断言仍有缺口。没有证实新的认证绕过、跨账户越权、可利用的令牌伪造或数据损坏。**

新增正式发现 **6 项：Critical 0、Important 0、Minor 6、Nit 0**，全部有实证。其中 3 项是实现边界，3 项是测试缺口。另有 3 个经过故障注入核实的协议／设计边界，放在第 5 节，不把有意的严格令牌轮换策略、已承认的非原子租约或尚未明确的授权时点重复算作新漏洞。

对三个核心问题的回答：

- **闭环**：注册、登录、轮换、退出、改密、停用／启用、PAT 生命周期正常路径均成立。但 localStorage 可读不可写时，启动可能永远停在加载状态，没有应用内重试／退出入口（R1）；网络恢复也不等于会话恢复（D1）。不能给出“每个失败分支都有去处”的无条件结论。
- **BUG**：确认了存储异常逃出启动状态机、保存反馈与当前草稿不一致、改密码依赖的邮箱快照过时。契约安全语义及两处 E2E 的边界也有实际反向对照。
- **设计**：分层与事务主体合理，未发现需要推倒重做的架构。M2 可以接入停用和引导扩展，但“只注册、不改 M1”并不完全成立：邀请注册的端口／请求体、instance 注册含义、identity 操作的错误码声明和测试均有明确接点，详见 D4。

最值得担心的三件事：

1. **会话故障恢复的保证过强**：一次“服务端已提交、客户端未收到”的刷新就会使合法用户被当成重复使用者退出；本轮真实 E2E 4 次均复现。这是安全与可用性的取舍，不能用服务端 4 秒／客户端 8 秒的大小关系证明它不存在。
2. **非安全上下文的恢复能力较弱**：localStorage 租约并非严格互斥；同时，存储写入失败会直接卡住启动。这两个事实分别由真实 Chromium 的锁交错和实际配额耗尽证明。
3. **“全绿”的保障范围需说准**：`security: [{}]` 被测试误读，257 个相关测试仍通过；把真正保存的新密码改错，A7 页面故事仍通过。前者会影响 M2 新接口的验收基础，后者并不意味着整套测试都漏检。

建议先补 R1–R4 的实现／基础测试，再完善 R5–R6。D1、D2 应明确为产品支持边界或另立恢复协议，不建议为了消除被迫登录而直接取消旧刷新令牌的重复使用检测。

## 2. 审查范围与方法

### 2.1 阅读与去重

已阅读并对照：根 README、`docs/README.md`、总体设计的 6.1、6.2、7、8、9.4、10.1、12.4、12.5、第 13 节；M1 总设计、六个 Phase 的设计与结果；P1–P6 与 M1-closeout 共 7 份既有审查；M1 的三份 handoff 与 `M2-workspace/handoffs/M1-identity.md`。

审查分为后端凭证／并发、前端会话／页面、平台／契约／部署三条独立线，主审执行完整门禁及跨层故障注入，并复核原始日志、探针源码、发现的限界和级别。范围涵盖用户给出的 12 类检查项，第 7 节给出覆盖矩阵。

没有重新列为新发现的既有事项包括：NFKC 规则修复、旧代 MAC、JWT 严格解码、`password_user` 按账户、`last_used_at` 的 SKIP LOCKED、读答复覆盖写答复、PAT 对话框卸载、焦点修复、运行时角色授权、注册策略参数不足、事务外 `ShareActiveAccount`、首个停用注册者的接线行为测试。对有意决定的质疑均放在第 5 节，说明本次增加了什么证据。

### 2.2 隔离和环境

主仓库只读，唯一新增文件是本报告；HEAD 始终为 main 的 `f873f69`。开始时已有未跟踪的 `.claude/`，未修改。没有提交、推送，也没有操作 `/Users/xiaoruan/project/nerve-project`。

四条审查线分别从主仓库克隆到以下临时目录，各自 detached checkout `f873f69`，测试、变异与探针都在克隆中：

| 用途 | 临时克隆的父目录 |
|---|---|
| 完整门禁、跨层 E2E | `/tmp/nerve-m1-review.fIkY3V` |
| 后端身份与事务 | `/tmp/nerve-auth-review.BZJUZb` |
| 前端与浏览器 | `/tmp/nwiki-frontend-review.lGhXdk` |
| 平台、契约、部署 | `/tmp/nwiki-platform-review.ioOFjE` |

各父目录下的仓库名均为 `nerve-wiki-m1-review`。证据日志和临时测试副本留在克隆外，便于本次复核；下文同时提供关键交错、变异和输出，不依赖临时目录永久存在。

### 2.3 实际运行结果

| 命令／验证 | 本次结果 |
|---|---|
| `pnpm install --frozen-lockfile` | exit 0；412 个依赖，锁文件未变化 |
| `make check` | exit 0；两处 Go lint 均 0 issues，前端 lint／格式／类型／knip 通过；Go 主模块 30 个有测试包通过，另有 bodyshape 分配预算复跑及 tools 包，共 32 条成功包执行记录；vitest **36 文件、415 测试通过**；前端构建成功 |
| `make gen-check` | exit 0；Go、OpenAPI bundle、TS 生成物无差异 |
| `make e2e` | exit 0；Chromium **47/47**，15.2 秒，包含 A1–A14 和 S1–S4 |
| `make image-smoke` | exit 0；`nervewiki:0.1.0-dev`，提交 f873f69，`modified=false`；5 条迁移、探针、内嵌前端、注册关闭、管理员建账户／登录、非 root、优雅停止通过；之后 `docker rmi nervewiki:0.1.0-dev` 成功 |
| `go test -race -count=1 -json ./internal/modules/identity/...` | exit 0；9 个有测试包，227 顶层＋148 子测试＝**375** 通过 |
| 平台定向 `-race -count=1 -json`：httpserver/...、ratelimit、config、identity/http、jobs、bootstrap、migrations、configs | 合计 10 包，230 顶层＋400 子测试＝**630** 通过；与总门禁及 identity 的覆盖有重叠，不能相加当作独立用例总数 |
| 前端独立基线及还原后 `pnpm --filter @nervewiki/web test` | 两次均 **415/415** |
| 后端两个临时交错探针 `go test -race -count=3 -v -run '^TestReview' ./internal/modules/identity` | 两个探针各 3 次达到断言并预期失败：R3 和 D3；没有 race 报告 |
| 刷新响应丢失的临时 Playwright 故事 | 首次 1 次、另 `--repeat-each 3`，**4/4** 观察到 D1；断言的是缺陷现象，所以这些探针是通过 |
| 前端临时 Vitest 探针 | 租约同令牌并发、保存中的草稿，各 1 个预期失败；真实 Chromium 浏览器脚本 exit 0，复现 D2 和 R1 |
| A7 错误密码落库的反向变异 | 1 个 API 故事失败、1 个页面故事通过；还原并重建后 **2/2** 通过 |
| A10 在生成到期时间后注入 6 秒调度停顿 | 正确产品代码下故事失败，收到 422；删除停顿后原故事通过 |

主审日志：上述第一目录的 `install.log`、`check.log`、`gen-check.log`、`e2e.log`、`image-smoke.log`、`probe-refresh-loss*.log`、`mutation-a7.log`、`restored-a7.log`、`probe-a10-stall.log`、`restored-a10.log`。各分工的 `evidence.md` 记录了详细命令、原始输出和清理结果。

### 2.4 反向对照与还原

实际做了 8 类代码／契约变异：去掉旧代 MAC、反向读取 XFF、去掉 River context 隔离、删 MAINTAIN 授权、添加未返回的 problem 码、取消旧读保护、security 改成 `[{}]`、改错真正保存的新密码。前 6 类均被现有测试捕获；第 7 类幸存；第 8 类仅页面故事幸存。另做了 A10 调度停顿对照，不把它算作产品代码变异。具体结果见第 6 节。

生产文件、契约 source／dist、临时测试均已还原或移出克隆；四个克隆 `git diff --exit-code` 成功，`git status --porcelain` 为空。各测试进程退出，前端临时 Vite／Chromium 已停止；Testcontainers 与镜像冒烟创建的自有容器已清理。本轮构建镜像已删除。没有按名称模糊匹配杀进程，没有操作已有的开发库或其他容器。

## 3. 发现清单

等级按委托定义；本轮没有为了达到某个数量而把协议取舍升级成漏洞。

| 编号 | 级别 | 类别 | 一句话 | 位置（文件:行） | 状态 |
|---|---|---|---|---|---|
| R1 | Minor | 前端／正确性 | localStorage getter 成功但写入失败时，启动异常逸出，停在无重试／退出入口的加载页 | `web/apps/web/src/main.tsx:25,34`；`web/apps/web/src/session/refresh-lock.ts:88`；`web/apps/web/src/session/token-manager.ts:210` | 已证实 |
| R2 | Minor | 前端／并发 | 保存期间继续编辑显示名，旧请求完成会把当前未保存草稿标为“已保存” | `web/apps/web/src/app/display-name-form.tsx:46,51,65`；`web/apps/web/src/pages/settings/profile-page.tsx:30` | 已证实 |
| R3 | Minor | 正确性／并发 | PAT 改密码与管理员改邮箱并发时，新密码规则仍使用旧邮箱 | `server/internal/modules/identity/app/change_password.go:51,55`；`server/internal/modules/identity/app/current_password.go:58,62` | 已证实 |
| R4 | Minor | 契约／测试 | 契约测试把允许匿名的 `security: [{}]` 误判为必须认证，相关全包测试仍通过 | `server/internal/platform/httpserver/apitest/problems.go:49`；`server/internal/platform/httpserver/apitest/rules_test.go:189`；`server/internal/platform/httpserver/apitest/operations.go:107` | 已证实 |
| R5 | Minor | 测试 | A7 页面版不验证新密码能登录，实际密码被改错仍然通过 | `e2e/stories/identity/a7-change-password.spec.ts:111`；`e2e/fixtures/assert/identity.ts:232` | 已证实 |
| R6 | Minor | 测试 | A10 把有效 PAT 的准备与首次成功调用压在 5 秒墙钟窗口内，调度停顿会让正确代码失败 | `e2e/stories/identity/a10-api-tokens.spec.ts:103` | 已证实 |

## 4. 每个发现的详情

### R1 — 存储写入异常没有回到会话状态机

**现象与触发条件。** `browserStorage()` 仅捕获读取 `window.localStorage` 属性时的异常；属性可访问不代表 `setItem` 可以成功。已有认证记录、存储配额耗尽、没有 Web Locks 时，启动首先写 `nwiki.auth.lease`，这里的异常没有被转成 `unavailable` 或其他可恢复状态。

关键路径：`main.tsx:34–43` 返回原始 Storage；`:25` 以 `void session.start()` 启动；`refresh-lock.ts:88` 直接 setItem；`token-manager.ts:210` 对非 `SessionUnavailableError` 重新抛出。页面仍为 `starting`，只显示 Loading。

**复现。** 使用真实 Chromium、生产前端源码，由 Vite 提供页面；API 是 mock，不能把这个探针称为完整后端 E2E。先写入合形的认证记录，再把同源 localStorage 填到实际浏览器配额，确认 getter 正常、增加 auth／lease 值会抛 `QuotaExceededError`；删除 `navigator.locks` 模拟受支持的 LAN HTTP 路径，打开 `/settings/security`。故障发生在发刷新请求之前，因此 mock 令牌是否可供后端认证不影响本项结论。

实测输出节选：

```text
filled.size = 5242866
getterWorks = true
authWriteError = QuotaExceededError
locks = false
loading = 1; signOut = 0; retry = 0
Unhandled rejection: QuotaExceededError ... 'nwiki.auth.lease' exceeded the quota
```

没有记录时再登录，mock 登录成功后保存认证记录同样失败，页面显示 `Something went wrong (QuotaExceededError).`，auth 仍为空。保留记录的启动路径更差：连这个错误提示也没有。

**影响与限界。** 这是可读不可写存储环境中的应用内死路；用户得在浏览器外清理数据。没有证明普通使用本项目的少量 auth／偏好数据会自行填满 5 MiB，也没有因此发现凭证泄露，所以定为 Minor，而不是普遍登录不可用。

**建议。** 存储依赖应区分可读和可写；启动／续期／退出外围应捕获 Storage 操作错误，提供可见的恢复或清除本地会话入口。若退到内存，必须明确旧持久化记录怎么处理，不能静默留下已经被轮换的旧令牌，导致下一次加载触发 D1。只在启动时试写一次不足以保护运行途中配额改变。

**修复验收／反向对照。** 保留“getter 正常、setItem 失败”的测试，覆盖首次登录、带记录启动、轮换后写入、removeItem 失败；必须离开无限 starting，且用户能选择恢复／退出。去掉错误处理后应重新出现本探针的未处理异常。探针脚本保存为 `/tmp/nwiki-frontend-review.lGhXdk/probes/review-browser-probe.mjs`，结果在 `browser-result.json`。

### R2 — 旧保存请求给新草稿显示成功

**现象。** 显示名字段在 sending 时仍可编辑，只有提交按钮禁用。提交捕获 First name 后，用户改为 Second name；onEdit 先清掉 saved，随后旧请求完成又无条件 `saved()`，把 saved 设回 true。

最小交错：

```text
输入 First name → 点 Save → 扣住 PATCH 响应
字段改为 Second name → 放行 First name 的成功响应
实际发送／服务端结果：First name
输入框：Second name；状态区域：Saved.
```

`display-name-form.tsx:46–51` 保存提交时快照，`:65–67` 仍接收新编辑；`profile-page.tsx:30–32` 未核对提交对应的草稿版本便显示成功。临时 Vitest 测试同时断言已发送的值是 First name、输入框是 Second name，最后期望状态区域为空：

```text
FAIL review: editing the display name while saving must not mark the unsaved value saved
Expected: ""
Received: "Saved."
```

**影响。** 用户看到成功后离开页面，第二份草稿没有被保存。这里只证明资料页反馈错误，不声称后端已提交数据丢失。此前 P5 的读／写计数保护防止的是旧 GET 覆盖 store；它不能保护组件里的独立草稿，这不是重复报告该旧问题。

**建议。** 简单方案是发送中禁用输入；允许继续编辑的方案则保存提交时的草稿版本，只有版本没变化时才显示 Saved。后者交互更好，但需要明确保存后仍有未提交编辑的状态。

**修复验收／反向对照。** 保留“提交 A、编辑 B、答复 A”的临时测试，确认 B 不被标成已保存；之后提交 B，再核对数据库／响应和状态。去掉版本比较应再次失败。源码与日志：上述前端目录中的 `probes/review-profile.probe.test.tsx`、`profile-probe.log`。

### R3 — 密码规则没有复核变动的邮箱

**现象与代码。** 新密码校验在锁外以 `account.Email` 执行；`CurrentPassword.Confirm` 锁下只比较 PasswordHash。管理员 `set-email` 会改邮箱并撤销会话，却按设计保留 PAT 和密码哈希（`app/set_email.go:56–66`）。所以 PAT 调用方通过凭证复核后，仍能使用旧邮箱的规则结果。

**确定性复现。** 使用真实 HTTP handler、Argon2 和 PostgreSQL，只给管理员 EmailChanger 加 gate 固定交错：

1. 注册 `alice@corp.com`，建立 PAT，当前密码为测试用的 `correct horse battery`。
2. 管理员取得 users 行锁，在写邮箱前暂停，准备改为 `zqxj@corp.com`。
3. PAT 请求把密码改为 `zqxj2026!`；先读旧的已提交邮箱，通过规则，然后阻塞于账户行锁。
4. `pgtest.WaitForLockWaits` 确认阻塞后，让管理员提交。
5. 改密码取得锁，哈希没变、PAT 有效，于是返回 204；新邮箱加该密码可以登录。等改邮箱结束后，再串行提交同样的新密码则为 422。

三轮结果相同：

```text
admin committed email=zqxj@corp.com
concurrent change status=204
login with disallowed new password=200
same change after email commit=422
password rule used obsolete email: status=204, want 422
```

**影响与限界。** 规则结果取决于并发时序，需要有效 PAT、正确当前密码和管理员同时改邮箱。管理员串行改邮箱本来也可能令既有密码接近新邮箱；这里不推断账户接管或显著增强攻击能力。

**建议。** 将锁下账户交给轻量校验回调，复核邮箱相关规则；或把邮箱纳入快照变动检测后重算规则。昂贵 Argon2 仍应在锁外。若产品明确选择“请求开始时邮箱”，应记录语义，而不是宣称以当前账户校验。

**修复验收／反向对照。** 临时 `TestReviewPasswordRulesAfterConcurrentEmailChange` 在基线 `-race -count=3` 下三次失败。修改后同交错应 422 且不改密码；取消邮箱复核应恢复失败。完整测试副本：`/tmp/nerve-auth-review.BZJUZb/review_probes_test.go`，放回克隆的 `server/internal/modules/identity/` 可复跑；其中第二个测试属于 D3，不应混同本项。

### R4 — security 测试误读 OpenAPI 的空 requirement

**事实。** OpenAPI 3.1 security 数组是可替代的要求，数组中的 `{}` 允许匿名。因此 `[{}]` 和 `[{bearer: []}, {}]` 都不能表示“必须 Bearer”。见 [OpenAPI 3.1 Operation.security](https://spec.openapis.org/oas/v3.1.0.html#operation-object)。

但当前代码：

```go
// apitest/problems.go:49
func needsToken(op *openapi3.Operation) bool {
    return op.Security != nil && len(*op.Security) > 0
}
```

`operations.go:107` 用 `!needsToken(op)` 推导 Public；规则测试仅遍历 requirement 的 key 查 scheme 是否存在（`rules_test.go:194`），空 map 没有 key，恰好逃过检查。bootstrap 的公开操作行为测试因此与它一起采用错误期望。

**反向对照。** 只把 getMe 的 source 与 bundled 契约 security 同时改成 `[{}]`，运行：

```sh
cd server
go test -race -count=1 -json ./internal/platform/httpserver/apitest ./internal/bootstrap ./internal/modules/identity/adapter/http
```

exit 0，**3 包、97 顶层＋160 子测试＝257 全过**。没有 `-run`／`-short` 跳过契约覆盖检查。服务仍拒绝匿名，契约却已允许匿名，语义漂移漏检。这个对照没有跑整个 `make check`／`make gen-check`，不将它说成所有门禁都通过。

另在未改的测试基础设施上加入两种 requirement 的反例，两个子用例均失败：

```text
needsToken=true for an OpenAPI requirement permitting anonymous access
authoring rules accepted security other than [] or [{bearer: []}]
```

**影响与限界。** 正式 f873f69 契约都使用当前约定形式，没有因此开放现行接口。问题是新接口或误改契约时的验收缺口。此前“将 getMe 改成公开操作”的反向对照不覆盖这个不同的语义错误。

**建议与修复验收。** 本项目只有公开／Bearer 两种形式，可在 authoring rules 中严格限定 `[]` 或恰好 `[{bearer: []}]`；否则应完整解析 OR 语义并拒绝运行时未支持的 scheme。两种反例应失败于规则检查，标准两种形式仍通过。原始日志与测试文本：`/tmp/nwiki-platform-review.ioOFjE/mutation-security-full.json`、`probe-security.go.txt`、`probe-security.log`。

### R5 — A7 页面版没有证明保存的是用户输入的新密码

**现象。** 共享断言仅检查哈希仍是 Argon2 且与旧值不同、会话撤销范围正确；页面故事末尾只验证旧密码失败。API 故事还验证新密码登录 200（`:66–67`），页面版没有相同断言。

**实际变异。** 在临时克隆把 `app/change_password.go:64` 的：

```go
hash, err = c.d.Hasher.Hash(ctx, in.New)
```

临时改为：

```go
hash, err = c.d.Hasher.Hash(ctx, in.New + "!")
```

其余校验、会话撤销完全不动。`make build` 后跑完整 `a7-change-password.spec.ts`：

```text
A7 (API): Expected 200, Received 401  [新密码登录断言，line 67]
A7 (page): passed
1 failed, 1 passed
```

还原源码、重新 `make build` 后同命令 **2 passed**。

**影响与限界。** 这证明页面验收不能独立完成“用户知道的新密码已经生效”的闭环，不是整个系统没有密码正确性测试。API 故事准确杀死了变异；前端单测还对请求体有精确断言，也不能说前端传参完全无人守护。

**建议／修复验收。** 页面改密后经登录接口使用用户刚输入的新密码，确认 200，且旧密码仍为 401；可进一步在另一个浏览器上下文登录。重复以上变异时，API 和页面两个版本都应失败。日志在主审目录 `mutation-a7.log`、`restored-a7.log`。

### R6 — A10 的准备阶段依赖 5 秒内完成

**现象。** 故事先在测试进程生成 `Date.now() + 5_000`，随后创建 PAT、查询数据库、首次 GET 期望 200，再轮询 401。15 秒 poll timeout 只能保护最后的等待，保护不了创建与首次成功断言之前的 5 秒。

**时序对照。** 不改产品代码，在 `a10-api-tokens.spec.ts:103` 算完 expiresAt 后只插入：

```ts
await new Promise((resolve) => setTimeout(resolve, 6_000)); // 模拟调度停顿
```

单跑 `A10 (API): a token stops when it expires`，exit 1：

```text
Expected: 201
Received: 422
errors: [{field: "expires_at", code: "out_of_range", message: "must be in the future"}]
```

移除停顿后原故事通过。本轮无扰动的 47 个故事没有偶发失败；证据证明的是调度预算脆弱，不是已经观察到持续集成频繁红。

**建议／修复验收。** E2E 用足够长的有效期建立并验证 PAT，再由测试专属时钟或数据库夹具确定地把该行推进为到期状态，下一请求确认 401；精确 `<=` 时间边界交给注入时钟的 Go 集成测试。代价是 E2E 不再靠真实等待证明自然时间流逝，需要两层各自说清验收目标。不要只无限增大 sleep／timeout。修复后的准备阶段插入同样 6 秒停顿应通过；禁用到期判断则应失败。日志：主审目录 `probe-a10-stall.log`、`restored-a10.log`。

## 5. 设计层面的质疑

以下是已核实的限制或取舍，**不加入上述 6 项新增缺陷计数**。

### D1 — 服务端期限不能消除刷新“提交成功、响应丢失”的窗口

定位：`app/refresh.go:73–93` 提交后返回新令牌；`domain/session.go:121` 对有效旧代进入 Reuse；`token-manager.ts:245–264,282–284` 把网络异常当作暂不可用并保留旧记录；`:216–217` 安排重试。

**本次增加的证据。** 在真正构建的二进制、真实数据库和 Chromium 上注册完成引导的用户，拦截第一页启动的刷新请求。Playwright `route.fetch()` 等到服务端真实 200，并核对数据库 generation=1、未撤销；随后仅对浏览器 `route.abort('connectionreset')`，丢掉响应。下一次刷新正常转发：

```ts
const response = await route.fetch();
if (first) {
  first = false;
  // 此时已核对数据库 generation=1、revoked_at=null
  await route.abort("connectionreset");
} else {
  await route.fulfill({ response });
}
```

首次及三次重复的输出均为：

```json
{"upstream":[200,401],"generation":1,"revoke_reason":"reuse_detected","local_record":null}
```

页面回到 Sign in；没有发生令牌窃取。完整临时故事副本在主审目录 `probes/refresh-loss.spec.ts`。它使用已有 fixtures，可放回 `e2e/stories/identity/` 执行。本轮没有把整个产品的时延拉长，只丢掉一次已提交答复。

**判断。** 严格一次性轮换按设计工作，不能称作重复使用检测实现错误。[RFC 9700 §4.14.2](https://www.rfc-editor.org/rfc/rfc9700.html#section-4.14.2) 也明确接受检测重放后迫使合法客户端重新授权的代价。本次证明该代价还会由网络不确定性触发，不证明发生频率，也未证明任何业务数据丢失。

需要收窄 `README.md:75` 的两处表述：旧代再用不一定是“被别人拿到了”；“超时令牌不变、可以重试”只适用于确认提交前回滚的情形。`postgres/tx.go:37–44` 明确允许语句完成后脱离请求取消提交，并承认 COMMIT 无响应时结果未知。P2 既有审查已讨论后一情况，但 README 的概括仍易让运维和客户端作者误以为所有网络失败都可无损重试。

**替代方案与代价。** 最小方案是保留严格撤销，明确“网络异常可能要求重新登录”，在界面区分会话结束与暂不可用。若产品要求无损恢复，就需要额外设计受约束的刷新重试协议、请求身份或发送方绑定及有限恢复窗口；必须评估旧令牌被盗时的重放代价。不能简单让所有旧代再次成功，也不能用更长的超时当作正确性证明。验收应同时保留本故障注入及 A5 的真实旧代／伪造旧代安全测试。

### D2 — localStorage 租约只能尽力串行

定位：`refresh-lock.ts:41–45` 已承认无 compare-and-set；`:85–90` 是独立的 read、write、100ms settle、read。当前接口注释 `:5` 的“没有其他持有者”以及对 A4 严格串行的验收解释应加上范围。

**确定性交错。** 用真实 Chromium 两页加载生产 leaseLock，CDP 在 A 已读到“无租约”、尚未 setItem 时暂停 A。让 B 写租约、等过 100ms、进入一个保持未结束的 task。再恢复 A，A 使用过时读取覆写 owner、再等 100ms，也进入 task。输出：

```text
beforeResume: B entered=true, lease.owner=B
afterResume:  A entered=true, B entered=true, lease.owner=A
```

这是生产前端＋真实浏览器存储／调度的证明，API 在该脚本中为 mock。另一个 TokenManager 单元探针使两方同时发 `rt-0`，按已有刷新协议返回成功／reuse 拒绝，最终记录被删除；“两个请求体不相同”的断言在基线失败。并发同旧令牌会在真实后端撤销的结论也由原有并发刷新集成测试覆盖。本次没有伪称将三者合为一次完整浏览器＋真实后端的故障实验。

**判断。** 这不是首次发现“租约非原子”：源码已经承认。新证据反驳的是“100ms settle 加 A4 两页通过就证明严格串行”。A4 的请求级扣留不覆盖 read/write 之间暂停这个窗口。[HTML Standard 的 Web Storage 说明](https://html.spec.whatwg.org/multipage/webstorage.html#introduction) 同样要求作者不要假定 localStorage 存在跨执行环境的锁。

**替代方案与代价。** 对需要严格串行的部署要求 HTTPS／Web Locks，并把 LAN HTTP 标为有限保障，是最小且诚实的选择；继续支持 fallback 则需要服务端容忍受控重复请求的协议，或更强的协调机制及故障模型。随意增大 settle／租约只能改变概率，不能消除该交错。HTTP 传输本身的机密性风险亦应由部署文档说明，但本轮没有做网络窃听测试，也未把它算新漏洞。

### D3 — “锁下仍未过期”实际采用请求开始的时刻

定位：`app/create_api_token.go:55–67` 先取得 now，再密码校验和等锁；`current_password.go:58` 将同一 now 传入锁下复核；`credential_lock.go:41–48` 读取新状态，却用旧 now 比较期限。

真实 HTTP／Argon2／PG 探针：PAT 在固定时钟一秒后到期；请求入站时有效并知道当前密码；持 users 行锁，等请求确实等待，再推进时钟两秒、放锁。三轮均为：

```text
create=201; new permanent tokens=1; fresh request with old PAT=401
```

**判断。** 请求开始时合法，随后完成是一种可接受语义，且受请求期限限制，不能直接称作“到期后新发请求绕过认证”。因此不计新漏洞。但 P3／长期约定中的“锁下复核未过期”应明确究竟是入站时点还是取得锁的时点。若要求后者，把 Clock 的读取移到锁取得之后；不要仅改变测试期望却不明确协议。对应探针是 backend 副本中的 `TestReviewPATExpiresWhileWaitingForCredentialLock`。

### D4 — M2 能注册哪些部分，哪些必须回改 M1

| 接点 | 核实结论与建议 | 替代方案／代价 |
|---|---|---|
| 停用否决者／事务内订阅者 | 两路共用 `DeactivationSteps`；真实 PG 测试守住失败回滚、共享账户锁与停用互斥。M2 可提供注册者，无需改停用用例主体 | 组合根仍要把注册者交给 HTTP 和 Admin；第一个真实注册者必须有两入口行为测试，既有 handoff 已要求，不重报 |
| `ShareActiveAccount` | `identity/accounts.go:30` 的窄端口合理，但底层 `postgres.DB` 没有事务即退回 pool；事务外语句结束就释放锁。已有 handoff 已记录 | 建议 M2 首次接入时增加“必须处于事务”检查，并测缺事务立即失败；代价是平台增加一个显式事务能力入口。继续只靠约定也可行，但每条增长路径都要有并发回归 |
| 邀请注册 | `app/ports.go:27` 的 AllowSignup 只有 ctx，拿不到邮箱／邀请；请求体也没有邀请。不能只换实现 | 按既有例外扩展端口、注册输入和组合根，同时协调 instance 的 signup_enabled 与前端注册入口；把邀请形状留给 M2 是合理延后，但应把这些改动列为 M2 正式工作量 |
| 否决错误码 | M2 业务码要加进 identity 的 deactivateMe 契约、该操作所属 adapter/http 的测试、前端文案 | 这是现有 apitest 按模块核对的明确成本；暂时维护显式清单比预建动态错误码框架简单 |
| 引导步骤 | 前端步骤注册表能追加；后端只记录 id，不能把记录步骤当成业务前置条件已满足的证明 | M2 的创建／加入工作区必须独立校验真实成员状态；同时更新 E2E onboardingSteps 及步骤数断言。两个注册表的维护成本目前可接受 |
| 全局锁顺序 | 当前 M1 顺序与 SKIP LOCKED 路径经测试成立；尚无真实 workspace 表与注册者 | M2 要把 workspace／member 锁纳入顺序，并测两个管理员并发停用、停用与加入／移除成员。M1 的 users FOR SHARE 测试不能代替“最后管理员”不变量的测试；本轮不预测尚不存在代码一定死锁 |

### D5 — 分层、存储与复杂度

没有证据支持“存在上帝文件”或“必须引入新架构框架”。按本次 diff 的非生成 Go 文件统计，最大的是测试基础设施 `apitest/operations.go` 358 行，bootstrap/app.go 332 行；前端 token-manager.ts 377 行。用例小文件、窄端口、组合根接线和架构规则相互对应；CurrentPassword 抽取了真实重复逻辑，问题是快照内容不完整（R3），不是抽象本身多余。

真正需要关注的隐性契约有三个：事务经 context 传递、浏览器 Storage 的运行期失败、令牌轮换与网络提交结果的不确定性。前两项适合增加小型、明确的能力检查／状态分支；第三项需要产品和协议取舍，不能靠再加一层通用 service 解决。

刷新令牌放 localStorage、CSP 限制脚本来源是一项显式安全选择。现有脚本 CSP 为 self，没有本轮证据证明可利用的 XSS；但 CSP 不把 localStorage 变成 HttpOnly，未来同源脚本一旦被执行就可能读取记录。改为 HttpOnly cookie 会改变当前无 cookie／Bearer 统一模型并引入 CSRF、浏览器／API 双认证路径的成本，不应把它当成无代价的“修复”。M4–M7 渲染和附件接入必须继续执行现有安全约定。

## 6. 测试缺口

### 6.1 本次实际反向对照汇总

| 改坏的内容 | 运行目标／结果 | 判断 |
|---|---|---|
| 旧代 Reuse 去掉 `&& tagValid` | domain＋identity，3 个原测试失败：JudgeRefresh、伪造旧代不撤销、换钥；还原后三者通过 | 既有安全修复有效，不重报 |
| XFF 从右读改为从左读 | `TestClientIP` 的 5 个子用例失败 | 防伪造方向有有效回归 |
| River Start 去掉 `WithoutCancel` | `TestStopLeavesTheStartedClientToItsOwnStop` 失败 | 生命周期隔离有有效回归 |
| runtime-grants 删除 river_job 的 MAINTAIN | 原权限目录测试失败，指出 river_job 缺 MAINTAIN | 权限清单不是只检查文件存在 |
| refreshTokens 声明多一个从未返回的 problem code | 完整 identity/http 包 exit 1，TestMain 报未答过的码 | 双向码覆盖有实效 |
| AccountStore 去掉读／写计数保护 | 原 2 个测试中 1 个失败，onboarding_steps 被旧值覆盖；还原全前端 415 通过 | P5 修复有效，与 R2 草稿问题不同 |
| getMe source＋dist security 改 `[{}]` | 257 个相关测试全绿；新增反例两个子用例失败 | R4：真实语义漏检 |
| 改密码 Hash 输入改成 `in.New + "!"` | A7 页面通过、API 失败；还原 2/2 通过 | R5：两版本断言不对等，整套仍能抓住 |
| 正确实现不变，A10 算 expiresAt 后暂停 6 秒 | 期望创建201、实际422；还原故事通过 | R6：测试输入的墙钟预算，而非产品故障 |

### 6.2 正常通过仍不能证明的内容

- A4 验证了它实际制造的请求交错，不证明 localStorage 的读／写原子性；补 D2 的调度级实验，或将其保证明确限制在 Web Locks。
- 现有暂不可用测试侧重“请求被拒／未成功”的答复；需补 D1 的“服务端已成功提交、答复未到达”。只改 fetch 为抛错、不让真实后端执行，不能证明恢复语义。
- 415 个前端单测原样全过，但没有覆盖 R1 可读不可写存储及 R2 在途继续编辑。新增故障探针均可在基线上暴露现象。
- 原有账户锁测试确实守住多个凭证变动交错；它们没有覆盖“规则输入邮箱变化、哈希不变”的 R3。把只测试哈希快照正确称作全部账户快照正确，范围过大。
- 本轮未观察原始 E2E 随机失败。R6 是实际注入停顿后的确定性反例，不能据此写“CI 已频繁不稳定”。

## 7. 核实过、没有问题的部分

以下“没有问题”限定为已读代码及本次测试覆盖，不表示穷尽证明。

| 检查项 | 核实结果与证据 |
|---|---|
| 1. 刷新判定、签名、凭证状态 | 当前代哈希、旧代 MAC、撤销／到期优先、未来代拒绝、竞争重读、换钥分支均有真实测试；旧 MAC 变异被抓住。JWT 限 EdDSA、使用 Ed25519 公钥、要求 exp、严格 base64url、验签后处理过期、sub/sid 校验；没有依 kid 拉外部密钥的路径。PAT 的归属／撤销／过期／active 逐请求检查通过。网络窗口另见 D1 |
| 1. 比较方式与账户枚举 | MAC 用 hmac.Equal，Argon2 派生结果用 ConstantTimeCompare；当前刷新令牌比较 SHA-256 用 bytes.Equal，这本身不构成恢复随机秘密的证据。未知邮箱和错密码使用相同错误并执行哈希校验；停用状态在密码正确之后才答403。本轮没有统计证明网络耗时严格相同，见第8节 |
| 2. 并发与事务 | 原 interleavings 的登录／改密／PAT／重置／邮箱变化均通过；users 首锁、FOR NO KEY UPDATE 与增长 FOR SHARE 的互斥成立；回滚和订阅者失败回滚通过。last_used_at／清理 SKIP LOCKED 不等待被持有行。未找到新的死锁；提交之后业务上可能失败的签发步骤未被发现，网络／COMMIT结果未知不在此保证内 |
| 3. 密码 | domain 规则、Hash／Verify、前端长度都使用 NFKC；名单、主干、长度／超长体结构与上限测试通过。长度按明确约定为 UTF-16 单位，不把文档的字符泛称当成新的规范绕过。Argon2 名额、MaxWait、取消与503路径通过；改密保当前会话／保PAT、管理员重置撤全部、改邮箱保PAT的行为与文档一致 |
| 4. 限流与IP | failure gate 先 Reserve 再认证，成功／仅访问令牌过期／故障退款，失败保留；50并发容量3的测试只运行3次认证。password_user 按账户，同账户不同凭证不能倍增额度。IPv6前缀、mapped IPv4、XFF可信链／畸形项、配置边界测试通过；桶以单调时间计时，满桶闲置key按分钟访问触发清理。没有证明任意持续高基数负载下存在固定内存上限 |
| 5. 契约与错误 | 当前13个操作路由、公开清单、PAT、破坏请求体／参数、problem头声明测试通过；401/403/404/422/429/503的现有场景及固定错误文本通过。所有API响应 no-store；不存在把底层签名解析字符串直接答给客户端的路径。security的另类合法写法例外见R4 |
| 6. 管理命令 | 本轮 make check 的 cmd／bootstrap 测试与 A12 覆盖 stdin、错误退出、结果／日志分流和机密检查；镜像内实际 stdin建账户并登录通过。reset／set-email／deactivate／activate 的撤销范围、邮箱规范化／唯一性、重复启停幂等及恢复未撤销未过期PAT通过。未重做人工终端交互 |
| 7. 任务与部署 | 等迁移、启动失败结束serve、HTTP先于jobs停机、超时取消、池关闭路径通过；1000一批、固定截止时刻、SKIP LOCKED清理通过。runtime role以恰好grants运行HTTP／CLI／清理／REINDEX，重复授权和目录权限检查通过；Down全回滚再Up通过；prod关闭注册／必需私钥、非prod非回环告警通过 |
| 8. 前端会话 | Web Locks及通常fallback交错、同标签去重、跨标签退出／换账户、loginId分代、SWR缓存按代、SessionChangedError、next拒绝外部路径等原测试与E2E通过；没有发现开放重定向或脚本执行证据。fallback严格互斥和Storage异常的边界另见D2/R1 |
| 9. 页面 | PAT只在对话框state显示、关闭卸载／迟到结果不显示、DOM／存储／地址／控制台检查、复制、撤销焦点；停用本地会话终止、改密码保当前会话；字段错误映射、焦点、一直挂载的live region均由原测试／故事通过。显示名新草稿例外见R2；没有以自动化通过替代人工读屏验收 |
| 10. 扩展点 | 否决在写前、订阅在同事务、失败全回滚，自助和管理员两路、共享账户锁交错均通过；M2未实现部分不假定已完成。D4逐项说明回改成本 |
| 11. 测试有效性 | 本轮有杀死变异的证据，也有幸存变异和定向失败探针；没有把旧审查的压测次数、时延数字或CI状态冒充本轮结果 |
| 12. 设计质量 | 分层／导入限制／sqlc范围／组合根检查为绿；未发现超范围架构重做的必要。事务前提、Storage能力与跨网络状态机值得显式表达，见D5 |

## 8. 未能验证的

- **登录耗时枚举的统计界限**：读了 dummy hash 实现，跑了功能测试；没有本轮不同网络／负载下的分布、样本量及显著性分析。不能把 P2 旧报告的30次测量当成本轮证明，也没有证据把现实现报为可枚举漏洞。
- **浏览器自然调度的发生概率**：D2用CDP精确暂停证明可达交错；没有测实际用户后台冻结、休眠、GC时出现的频率。Web Locks下长期持锁／冻结时用户如何退出，也未做完整故障实验，不列发现。
- **存储受限浏览器的完整矩阵**：R1证明真实Chromium满配额＋无Web Locks的路径；未在Safari／Firefox各种隐私策略上逐一复现，不把该结果推广成所有浏览器都会失败。
- **网络／数据库极端故障矩阵**：已测试丢刷新答复；没有另行模拟数据库在COMMIT响应中断、电源故障、跨主机时钟漂移、多实例密钥不一致。D1只使用已获得的提交及浏览器观察证据。
- **规模与长期任务**：没有重复百万会话停机压测、长期高基数限流内存压测、River每日REINDEX被中断后残留 `_ccnew` 的实测。普通任务、权限、批处理和生命周期已经跑到，不把这些边界猜测列缺陷。
- **人工可访问性与终端**：未用真实读屏器逐页验收，未重做Linux／macOS伪终端的回显与Ctrl-C人工流程；相应自动测试已过。
- **M2真实业务并发**：没有workspace实现，不能验证“两个最后管理员同时停用”等完整业务不变量；仅能评估M1提供的端口与事务条件。已有handoff中的延期事项不重复计新问题。

本轮未修复产品代码。上述“修复验收／反向对照”中建议新增的修复后测试是验收要求，不是已经完成的修复声明；本轮实际执行的命令、故障注入和变异结果已在第2、4、5、6节区分列明。

## 9. 处置（Claude，2026-10-01）

逐项核实之后全部采纳：6 项发现都成立，修复在分支 `m1-codex-review`（`0955afd`），每项都有反向对照；5 项设计质疑按各自的结论处理。

| # | 处置 | 反向对照 |
|---|---|---|
| R1 | 会话装配把存储包一层：写入（含租约）失败一律成为 `SessionStorageError`。令牌管理器遇到它就结束这个会话：用手上最新的刷新令牌尽力退出（续期已成功、写不进时是新令牌；租约写不进时是记录里的），能删就删记录，标签页回到登录页，不再停在加载中；登录写不进时把新会话退出，登录页说明"此浏览器无法保存登录状态"。租约释放失败不再盖掉任务的结果。测试：写不进续期后的记录、写不进租约、写不进登录的记录三种，各断言发出的退出与最终状态；`errorText` 的文案 | 去掉包装、去掉结束会话的处理，对应的测试失败 |
| R2 | 显示名的表单记下编辑次数：发出之后又编辑过，答复回来不标"已保存"，引导也不前进；再提交发出字段里的名字。测试：提交 A、编辑 B、A 的答复回来，状态为空，再提交 B 才"已保存" | 去掉次数的比较，测试失败 |
| R3 | `CurrentPassword.Confirm` 把锁住的账户交给写入；改密码在锁下发现邮箱变了就按新邮箱再判断一次规则（便宜，不重算哈希），不通过答 422、哈希不变。测试：PAT 改密码停在校验当前密码时，管理员改邮箱，放行之后答 `common_password` | 去掉锁下的再判断，测试失败 |
| R4 | 写法规则把 `security` 限定为 `[]` 或恰好 `[{bearer: []}]`，`[{}]`、`[{bearer: []}, {}]`、别的 scheme、带 scope 都报错。规则用例各一 | `getMe` 的源描述与打包的契约都改成 `[{}]`，`TestContractFollowsAuthoringRules` 失败 |
| R5 | A7 的页面版本改密码之后，新密码登录答 200、旧密码答 401 | 改密码哈希 `in.New + "!"`：接口版本与页面版本都失败 |
| R6 | A10 的到期改为：有效期一小时的令牌先用一次（200），再在数据库里把它的创建与到期时间各往回挪两小时，服务端自己的时钟判它过期（401）；精确的边界由 `authenticate_pat_test.go` 的固定时钟守住 | 建令牌之前插入 6 秒停顿，故事照样通过；去掉服务端的到期判断，故事失败 |
| D1 | 接受，严格轮换的代价，不改协议。A5 的页面版本加一个用例钉住这个行为：续期的答复在返回途中丢失，页面再用旧令牌，会话以 `reuse_detected` 结束，页面到登录页、记录被删。README 改写两处措辞；M1 总设计的风险表加一行 | — |
| D2 | 接受。`refresh-lock.ts` 的接口与租约注释写明只能尽力串行、最坏情况与 HTTPS；README 与 M1 风险表同步；总体设计 13.2 第 1 条写明 | — |
| D3 | 保持语义：锁下核对的是其间的变化，期限按请求的时刻判断，与认证中间件一致。`CredentialLock` 的注释与总体设计 13.1 第 18 条写明 | — |
| D4 | 已在 M1→M2 移交中的不重复；补两项：引导步骤的记录不是业务结果的证明（第 5 项），工作区、成员接进加锁顺序与"最后一个管理员"的并发测试（第 9 项） | — |
| D5 | 同意，不改。刷新令牌放在 localStorage 是显式的选择，M4–M7 的渲染与附件照现有的安全约定 | — |

门禁：本地 `make check`（vitest 421 个）、`make gen-check`、`make e2e`（48 个；另跑 `--repeat-each 3`，144 个）为绿。
