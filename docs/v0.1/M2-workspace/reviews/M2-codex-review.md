# M2 工作区：Codex 对抗性质量评审

日期：2026-10-01。评审基线：`646964ab91fd3b0f6223ec928a120f6adfe7764b`（`646964a`）；改动范围：`0eb3b15..646964a`。下文源码、测试和设计文档的行号均指这个基线，不指本报告提交后的 HEAD。

## 1. 结论

**常规门禁全部通过，但尚不宜把 M2 作为已经守住全部不变量的基础直接交给 M3。** 本轮证实了非空工作区可以失去全部有效管理员、旧会话的邀请请求回调干扰新会话、在途表单覆盖后来输入的草稿，以及邀请令牌的严格解码承诺不成立；另有两类现存测试无法捕获的真实错误变异。

正式发现 **6 项：Critical 0、Important 4、Minor 2、Nit 0**。其中 4 项是基线实现问题，2 项是测试缺口。没有证实基线中的跨工作区越权、访客邮箱泄露、邀请令牌泄露、MAC 伪造或持久化业务数据损坏。变异代码产生的错误授权只用于证明测试缺口，不能冒充基线漏洞。

最值得关注的三件事：

1. **锁顺序正确不等于业务不变量完整。** 最后一个管理员独自停用后，旧邀请仍可让普通成员／访客加入；CLI 也能恢复普通成员。工作区从允许的“无人”变成“有人但无人可管理”。正反交错的结果不同，现有 13 个交错没有覆盖这个组合（R1）。
2. **分代只隔离 store，未隔离所有副作用。** 旧邀请接受请求的 200 答复在退出／换账户之后仍调用 `navigate`，把当前页面带到旧工作区或登录页。指定探针中未观察到旧列表进入新 store，不能据此推出所有旧回调都安全（R2）。
3. **输入仍可编辑，成功回调却按旧快照重置它。** 保存名称时新写的草稿被旧名称替换；发送邀请时下一位收件人的草稿被清空。按钮禁用只防止重复提交，不保护发送期间的编辑（R3）。

建议在进入 M3 的实现阶段前修复 R1–R4，并补上对应的回归测试；R5–R6 的测试应同时收口。M3 的扩展点主体、事务边界与权限层次可以继续使用，无须推倒重做；跨模块清理的失败隔离和首批注册者的实际接线验收需要明确处理（D1、D2）。

## 2. 评审范围与方法

### 2.1 阅读、基线与去重

改动范围为 376 个文件，30,131 行新增、765 行删除。审查分为权限／邀请、并发／清理／命令、前端／浏览器三条独立线，主审执行完整门禁、契约变异，并复核探针源码、原始日志、基线行号和结论的限界。

团队已阅读 `docs/README.md`、v0.1 总体设计的指定章节与整个第 13 节、M2 总设计、P1–P6 文档及全部 plans、六份 Phase 审查与收尾审查、M1→M2 handoff、M2→M3／M4／M7 的全部指定 handoff，以及 M1 同类评审。历史计划与已明确记载的实施差异不另算缺陷。

没有重复列为新发现的事项包括：锁后重读、邀请密钥 info 钉住、清理 `NOT EXISTS`、外壳 `key`、SWR 工作区键、store 写计数和去重、`texts` 只处理 problem 码、W4 页面删除断言、加入与改角色无事件、组合检查只证明静态可达、退出整页后的焦点。R4 是对“Strict 已完整保证唯一拼法”的新反例；R3 是同一表单请求期间的编辑覆盖，区别于 P5/T1 的跨工作区组件复用和 P5/T13 的去空白要求。

### 2.2 隔离与环境

主目录始终留在 main；测试、构建、依赖安装、变异与临时测试均在仓库之外的 `git archive 646964a` 快照中执行。四份快照位于 `/tmp/nwiki-m2-review.l1VNEH/{main,access,concurrency,frontend}/repo`，分别 `git init` 并提交快照。各份快照执行了 `pnpm install --frozen-lockfile`。

工具为 Go 原生测试与 race detector、PostgreSQL `18.6-trixie` Testcontainers、pnpm/Vitest、Playwright Chromium、SQL 锁等待探针、Python 变异脚本和 git。浏览器探针运行构建出的真实 Go 服务、真实数据库与内嵌前端；`route.fetch()` 只延迟真实答复的交付，不伪造 API 成功结果。

未运行 `make image-smoke`，未推送，未访问外部业务系统、Obsidian 笔记库或另一个开发项目。开始时存在的 `.claude/` 未触碰。报告是主仓库唯一新增和提交的文件。

### 2.3 门禁实测数字

| 命令／验证 | 实际结果 |
|---|---|
| `pnpm install --frozen-lockfile`（主审快照） | exit 0；412 个依赖全部复用，锁文件不变 |
| `GOFLAGS='-p=3' make lint knip` | exit 0；两处 Go lint 均 0 issues，oxlint／格式／类型检查／knip 通过 |
| `GOFLAGS='-p=3' make check` | exit 0；Go 主模块 **38 个有测试包**，另有 bodyshape 分配预算复跑和 tools 包，共 **40 条成功包执行记录**；Vitest **51 文件、685 测试**；前端构建成功 |
| `GOFLAGS='-p=3' make gen-check` | exit 0；Go、OpenAPI bundle、TS 生成物无差异 |
| `NWIKI_E2E_VERSION=0.1.0-dev GOFLAGS='-p=3' make e2e` | exit 0；Chromium **82/82**，15.2 秒 |
| `NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test stories/workspace --repeat-each=3 --workers=3`（在 e2e 目录） | exit 0；**34 个工作区故事 × 3 = 102/102**，19.6 秒 |
| 主审契约反向对照 | 2 项均被捕获；还原后的 workspace HTTP 全包通过 |
| 权限／邀请相关 9 个 Go 包（还原后） | `workspace/... access/... bootstrap`：**201 顶层＋418 子测试＝619 条通过记录**（557 个叶子用例），exit 0 |
| 新增权限／邀请探针 | 7 个顶层、35 个叶子场景；规范拼法反例按正确契约断言而失败，隔离／Unicode 身份对照通过；双接受／双撤回与 Unicode 注册另重复 3 轮通过 |
| 新增并发、清理与扩展点探针 | 10 个顶层、16 个叶子场景；累计 **52 次叶子执行**达到探针断言，包括缺陷现象与正确性对照 |
| 真实浏览器新增探针 | **9/9**，5.7 秒；4 条复现 R2／R3，5 条正常／观察性验证；临时修复对照另 **4/4**，4.0 秒；还原后定向 Vitest **8 文件、82/82** |

`GOFLAGS=-p=3` 使 Makefile 中所有 Go 测试命令同样受 `-p 3` 限制；定向测试用 `-p 1` 或 `-p 3`。各测试集有重叠，不能把它们相加当作独立用例总数。新探针中有的断言缺陷现象而通过，有的按正确契约断言而预期失败，下文逐项说明。

### 2.4 证据与还原

日志与临时测试副本保存在上述父目录而不是主仓库。主审日志为 `main/{install,lint-knip,check,gen-check,e2e,workspace-repeat}.log`；两项契约变异的脚本、日志与退出码在 `main/contract_mutations.py`、`main/contract-mutations.json`。分工证据见 `access/`、`concurrency/`、`frontend/`；下文列出可复现步骤和关键输出，不把结论只藏在临时文件里。

共做 **14 项错误变异，12 项被现有测试捕获、2 项幸存**，见第 6 节；前端另做 4 条临时修复对照。新增探针共 26 个顶层、60 个叶子场景（Go 与 Playwright 分别计数后相加，不含重复执行与修复对照）。

实验过程中的三类失败未当作产品发现：一条浏览器探针起初写错字段 label；双撤回探针起初误把共享父锁的排队当成子行独占锁的完成顺序，改为验证无序的 204＋404 后重复通过；一次 FK 探针在 ryuk 启动阶段超时，业务没有运行，后独立重跑 3/3 通过。原始失败日志保留，未以重跑掩盖产品断言失败。

四份快照最终 `git status --porcelain` 均为空，生产文件与契约均已还原，临时探针移到快照外保存。自有测试进程已结束，自有容器已清理；一份启动超时留下的 `created` 容器经日志 SessionID 与容器标签核对后，按精确 ID 删除。最终复查包括未运行容器，不只检查 `docker ps` 的运行列表。未按名称模糊匹配杀进程，未操作其他任务的容器。

下文 Go 命令均在对应快照的 `server/` 执行，Playwright 命令在 `e2e/` 执行。复跑新增探针时，先把保留的测试副本放回它原先的包／故事目录；测试后移出即可恢复快照。浏览器探针前运行 `make build`，使用 `NWIKI_E2E_VERSION=0.1.0-dev`。

## 3. 发现清单

| 编号 | 级别 | 类别 | 一句话描述 | 基线位置 |
|---|---|---|---|---|
| R1 | Important | 实现 | 最后一位管理员独自停用后，接受旧邀请／CLI 恢复可产生有成员但无管理员的工作区 | `workspace/domain/member.go:40`；`workspace/app/accept_invitation.go:87`；`workspace/app/reactivate_member.go:79` |
| R2 | Important | 实现 | 已退出或换账户，旧邀请接受的成功回调仍跳转当前页面 | `web/.../pages/invitation.tsx:140`；`web/.../session/auth-middleware.ts:41` |
| R3 | Important | 实现 | 改名与邀请的成功回调覆盖请求期间新输入的草稿 | `web/.../pages/workspace/general-page.tsx:66`；`web/.../pages/workspace/invitations-section.tsx:95` |
| R4 | Important | 实现 | 邀请令牌的 Strict 解码仍接受 CR／LF，同一令牌有多种有效拼法 | `workspace/adapter/mac/tokens.go:45` |
| R5 | Minor | 测试缺口 | 邮箱比较换成 EqualFold 后相关 9 个后端包的现存测试全绿，却允许另一规范化邮箱接受邀请 | `workspace/app/accept_invitation.go:84`；`server/internal/shared/email.go:19` |
| R6 | Minor | 测试缺口 | 变异为只通知首个成员结束订阅者后，现存相关测试仍全部通过 | `workspace/app/end_membership.go:62`；`workspace/extension_test.go:280` |

表中 `workspace/` 是 `server/internal/modules/workspace/`，`web/.../` 是 `web/apps/web/src/`。详情使用完整的仓库相对路径。

## 4. 每个发现的详情

### R1 — 空工作区再次增长时，没有守住有效管理员不变量

**场景。** Alice 是工作区唯一有效成员、角色为 admin。她已经给 Dana 发出 member 或 guest 邀请。规则二允许 Alice 停用，因为当时没有别人；停用只删除发给 Alice 邮箱的邀请。Dana 随后仍可接受之前的邀请，得到正常的 200，并成为唯一有效成员，但没有任何有效管理员。

另一条路径不需要邀请：Bob 曾是 member，离开后成员行仍保留；Alice 独自停用；服务器管理员执行 `workspaces reactivate-member --workspace acme --email bob@example.com`，成功恢复 Bob 为 member，结果同样是 0 位管理员、1 位有效成员。

**复现。** 在临时快照的 `server/internal/bootstrap/` 加载 `concurrency/review_concurrency_test.go`，运行：

```sh
go test -race -count=3 -p 1 -v \
  -run '^TestReview(SoleAdminDeactivationAndInvitation|SoleAdminDeactivationAndReactivate|SequentialInvitationAfterSoleAdminDeactivation|DeleteAndReactivate)$' \
  ./internal/bootstrap
```

HTTP 版本按正常 API 创建工作区、邀请、停用、接受；CLI 恢复经真实 `bootstrap.Workspaces`。确定性交错沿用 `pgtest` 的持锁与等待握手，锁工作区父行后依次放行两条请求，不靠 sleep。另有完全顺序的复现，问题不依赖极窄竞态。

关键输出：

```text
first=deactivation statuses=204/,200/ active_admins=0 active_members=1
alice_active=false dana_role=member invitation=accepted
remaining member rename: status=403 ... "code":"forbidden"
first=acceptance statuses=200/,409/workspace.sole_admin active_admins=1 active_members=2
first=deactivation ... reactivation err=<nil> active_admins=0 active_members=1 bob_role=member
```

member 与 guest 两种邀请都复现。接受／恢复先提交时，停用正确答 409；停用先提交时，增长路径没有对应检查。这排除了“没有正确串行”这个解释：串行成立，后半个状态转换没有维护不变量。

**根因与位置。** `server/internal/modules/workspace/domain/member.go:40–41` 的规则二只看停用时的人数；`app/accept_invitation.go:77–93,105–116` 锁下验证邀请和邮箱后直接加入／恢复；`app/reactivate_member.go:67–80` 锁下恢复原角色。两条增长路径都没有核对“恢复成非空的工作区是否仍有有效管理员”。现有 `server/internal/bootstrap/interleavings_deactivation_test.go:115` 检验的是**被邀请人自己停用**与接受的交错，覆盖不到最后一位管理员停用、另一人加入的组合。

**影响与级别。** 已进入工作区的人不能改名、管理成员、管理邀请或删除工作区；只能依赖服务器管理员恢复原管理员。违反“不能把其他有效成员留在无管理员工作区”的规则，并会影响 M3 的权限与无主处理，因此为 Important。设计允许“无人”的空工作区，不能用这个例外解释“有非管理员成员”的状态。

**建议。** 在工作区锁下明确空工作区重新开放的规则：无有效管理员时，非 admin 的接受／恢复拒绝并说明先恢复管理员；admin 邀请或恢复管理员可以重新建立不变量。若另选“最后管理员停用就失效所有待接受邀请”，还必须处理 CLI 恢复路径。新测试应覆盖 member／guest／admin、插入／恢复、两个先后与顺序调用，并保持账户锁在工作区锁之前。

### R2 — 邀请接受的旧回调在换代后仍能导航

**场景与复现。** 在公开邀请页以被邀请人登录，点击接受。Playwright 用以下方式让真实服务端完成事务，暂不把 200 交给页面：

```ts
await page.route(`**/api/v0/workspace-invitations/${invitation.id}/accept`, async route => {
  const response = await route.fetch(); // 真正的 Go 服务已提交
  ready.release();
  await held.promise;
  await route.fulfill({ response });
});
```

在 `ready` 后分别执行：①页内点击 Sign out，等待 `nwiki.auth` 消失；②通过另一标签页写入从真实注册接口获得的另一账户会话，等待当前页换代。两种情况下先确认页面还在邀请路径，再释放 `held`。第二种使用项目已有的 `writeRecord(newRecord(tokens))` 会话夹具模拟跨标签页登录传播，不伪造账户或令牌。

临时故事：`frontend/review-browser.spec.ts` 中两个 `PROBE accepting invitation ...`；命令为：

```sh
NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test \
  stories/workspace/review-browser.spec.ts --workers=1 --grep 'accepting invitation'
```

基线输出：

```text
sign-out: status=200, record=null
before=/invitations/<id>
after=/sign-in?next=%2F<old-workspace-slug>

account-switch:
before=/invitations/<id>
after=/<old-workspace-slug>
newAccount=<另一个测试账户>, visible=["Page not found"]
```

**根因。** `web/apps/web/src/pages/invitation.tsx:140–142` 在 `await workspaces.accept(link)` 之后直接导航，组件卸载／RootStore 换代并不会取消这个闭包。`web/apps/web/src/session/auth-middleware.ts:41` 对非 401 答复直接返回，不进行当前代检查；只有 401 分支检查 `loginId`。所以 13.2 第 1 条“旧一代请求以 SessionChangedError 结束”不是对所有答复成立的保证。9.4 的“答复只写回旧 store”本身也不足以保护导航。

临时对照在 `onResponse` 入口加入代次核对，两个探针均等到实际进入拒绝分支的测试事件后，确认页面仍在邀请路径。这个实验支持根因判断，但只覆盖本次首次成功答复，不代表已完成续期重发等全部响应窗口的修复。

**影响与限界。** 新会话的当前页面被旧请求夺走，公开邀请页退出后保持原地址的约定被打破。已证明的是导航错误；没有证明旧账户的 workspace store 数据进入新账户，也没有把邀请令牌带进 `next`。服务端在退出前已经提交的接受不应回滚，问题在客户端回调，因此为 Important，不算权限绕过。

**建议。** 在成功、失败以及重试后的响应路径统一定义换代语义，或在产生导航等外部副作用前核对发起代次／组件是否仍有效。只禁用当前页的退出按钮无法解决另一标签页换账户。测试必须让原请求先发出再换代、最后收到真实 2xx；只测旧客户端不能发新请求、旧 401 不续期，覆盖不到本项。

### R3 — 发送期间仍能编辑，成功回调却抹掉新草稿

**复现。** 与 R2 一样，延迟真实后端成功答复，不改它的内容：

| 表单 | 发出的请求 | 等答复期间的输入 | 交付答复后的字段 | 数据库 |
|---|---|---|---|---|
| 工作区改名 | `Submitted A` | `New draft B` | 被替换成 `Submitted A`，显示 Saved | `Submitted A` |
| 新邀请 | `first-review@example.com` | `next-draft@example.com` | 被清空 | 仅有第一份邀请 |

两个输入框在发送期间均可编辑，用户的后续输入不是脚本强行写入 disabled 控件。临时故事 `PROBE rename response ...`、`PROBE invitation response erases ...`，运行：

```sh
NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test \
  stories/workspace/review-browser.spec.ts --workers=1 --grep 'editable'
```

输出摘录：

```text
rename: status=200 before="New draft B" after="Submitted A" database.name="Submitted A"
invite: status=201 before="next-draft@example.com" after="" database.email="first-review@example.com"
```

**根因。** `web/apps/web/src/pages/workspace/general-page.tsx:62–67` 使用提交时闭包里的 `name`，在 await 后无条件 `setName(name.trim())`、`setSaved(true)`；输入的 `onChange` 没有使这个回调失效。`web/apps/web/src/pages/workspace/invitations-section.tsx:93–97` 在 await 后无条件 `setEmail("")`。按钮禁用只限制下一次请求。

临时给两个表单增加编辑计数后，同样的浏览器交错分别保留 `New draft B` 与 `next-draft@example.com`，数据库仍只保存第一次提交。R2／R3 的四条对照均通过，见 `frontend/repair-probes.log` 与 `repair-probe.diff`；这些改动已全部还原。

**去重与影响。** P5/T1 已修的是切换工作区时的组件状态，P5/T13 要求保存后去空白；本项不换工作区，并证明 T13 的无条件回写不保护请求期间的编辑。M1/R2 曾发现显示名的成功标志问题，M2 的新表单没有继承那里的草稿保护。这里确实丢失用户已输入但尚未提交的内容，属于 Important 行为错误；数据库正确保存了当时提交的 A，没有证实持久化数据损坏。

**建议。** 记录编辑代次，只有草稿仍对应这次提交时才规范化、清空或显示其成功状态；也可以明确禁用相关输入，但须把交互策略说清。改名与邀请都加真实在途编辑测试，不能只检查按钮 disabled。

### R4 — Strict Base64url 不等于唯一拼法

**证据。** `server/internal/modules/workspace/adapter/mac/tokens.go:36–46` 的注释承诺一个令牌只有一种拼法，但 `base64.RawURLEncoding.Strict().DecodeString` 仍忽略 CR、LF。它约束未用的末位，不禁止换行；当前代码未校验编码文本的定长／字符集，也未重编码比对。

用合法邀请令牌 `T` 构造以下 JSON 字符串值，经过真实 HTTP 请求而非只调用库函数：

```text
T[:8] + "\n" + T[8:]       → preview 200
T[:12] + "\r" + T[12:]     → preview 200
T + "\r\n"                → preview 200；关闭注册时带邀请 register 201
带 LF 的 T                 → accept 200
```

这里 `\n`／`\r` 是 JSON 解码后真实的换行字符，不是两个可见字符。纯 MAC 探针有 4 种位置变体；原 `TestTokens`、`TestTokenKnownAnswer` 仍通过。分别把 `access/canonical_probe_test.go` 放入 `server/internal/modules/workspace/adapter/mac/`，把 `access/http_probe_test.go` 放入 `server/internal/bootstrap/`，运行：

```sh
go test -race -count=1 -p 1 -v -run '^TestReviewInvitationCanonicalSpelling$' ./internal/modules/workspace/adapter/mac
go test -race -count=1 -p 1 -v -run '^TestReviewInvitationCanonicalHTTP$' ./internal/bootstrap
```

探针按应当拒绝的契约断言，预期失败。日志见 `access/{canonical-probe,http-probes}.log`。

**影响与级别。** P3/3.2、13.1 第 25 条及函数注释的严格解码保证不成立；第 25 条又将此实现约定推广到后续派生令牌，故按本次委托的“违反长期约定并延续到后续里程碑”标准定为 Important。别名仍必须包含正确的完整 128 位 MAC；本轮没有证明它扩大邀请权限、降低猜测成本或泄漏秘密，不当作 MAC 伪造。既有 `TestTokens` 覆盖末位未用位；P3/T2 将篡改测试改为修改有效位，两者均未覆盖 CR／LF。

**建议。** 限定前缀之后恰好 22 个 Base64url 字符，并保留 Strict 与 `hmac.Equal`；或者解码后重编码，要求文本完全相等。增加前缀后、编码中间、结尾的 CR／LF 反例，并至少从 preview、accept、关闭注册三条入口核对。

### R5 — EqualFold 错误授权的变异逃过相关 9 个后端包的现存测试

**基线正确。** `server/internal/shared/email.go:19–20` 的规则是 TrimSpace 后 ToLower；`ſ@example.com`（U+017F，长 s）与 `s@example.com` 仍是两个合法、不同的规范化邮箱。`server/internal/modules/workspace/app/accept_invitation.go:84` 当前用精确比较，实测发给前者的邀请由后者接受会得到 **403 `workspace.invitation_email_mismatch`**。

**变异。** 只改比较方式并补 import：

```diff
- if inv.Email != email {
+ if !strings.EqualFold(inv.Email, email) {
```

运行原有测试（临时 HTTP 反例不放进这轮集合）：

```sh
go test -race -count=1 -p 1 -json \
  ./internal/modules/workspace/... ./internal/modules/access/... ./internal/bootstrap
```

**201 个顶层测试＋418 个子测试＝619 条通过记录，exit 0**（557 个叶子用例）。但再运行新增 `TestReviewInvitationDistinctUnicodeAddress`：

```text
基线：invited U+017F account U+0073: status=403 ... invitation_email_mismatch
变异：invited U+017F account U+0073: status=200 ... "role":"member"
EqualFold=true normalizedEqual=false
```

证据为 `access/M04-unicode-equalfold.{diff,jsonl}`、`M04-unicode-equalfold-proof.jsonl`、`isolation-unicode-baseline.jsonl`。还原后同一集合再次得到 619 条通过记录。

**影响。** 这是安全语义的测试缺口：看似加强“大小写不敏感”的重构会把不同账户标识合并，当前相关单元、仓储、组合与权限测试都没有制止。基线没有这个错误，级别为 Minor。建议加入规范化后仍不同、但 EqualFold 等价的邮箱对，同时覆盖接受与关闭注册；不要把密码的 NFKC 规则或页面标题的完整折叠规则套到邮箱上。

### R6 — 多订阅者分发与后续订阅者失败缺少测试

**变异。** 在 `server/internal/modules/workspace/app/end_membership.go:62–65` 的循环里，第一个成功回调后加一条 `break`：

```diff
  if err := s.MembershipEnded(ctx, e); err != nil {
    return err
  }
+ break // 后面的订阅者永远收不到事件
```

运行两个完整的相关既有测试包（不含新增探针）：

```sh
go test -race -count=1 -p 1 -json ./internal/modules/workspace/app ./internal/modules/workspace
```

**61 顶层＋102 子测试＝163 条通过记录，0 fail、exit 0**；其中 app 包 139 条，模块根 24 条。没有声称该变异通过完整 server 或 E2E。`server/internal/modules/workspace/app/team_test.go:91–93` 与 `server/internal/modules/workspace/extension_test.go:270–274` 的共同夹具只注册一个订阅者；后者 `:318–341` 也只覆盖唯一订阅者失败。证据为 `concurrency/mutation-first-subscriber.{diff,jsonl}`。

**新增实证。** 临时模块集成测试注册两个真实写入测试表的订阅者，第二个可选择成功或返回错误。使用真实 PostgreSQL 与模块接线，以 HTTP 结束成员身份。变异后的输出：

```text
fail_second=false status=204 calls=1,0 committed_subscriber_rows=1 membership_ended=true
fail_second=true  status=204 calls=1,0 committed_subscriber_rows=1 membership_ended=true
registration list was truncated: calls=1,0
```

基线还原后实测：第一个场景 `status=204 calls=1,1 committed_subscriber_rows=2 membership_ended=true`；第二个场景 `status=500 code=internal_error calls=1,1 committed_subscriber_rows=0 membership_ended=false`，两个订阅者的写入与成员身份变更全部回滚。原始日志为 `concurrency/{mutation-first-subscriber-new-probe,extensions-restored}.log`，临时测试为 `concurrency/review_extensions_test.go`，放入 `server/internal/modules/workspace/` 后用 `go test -race -count=1 -p 1 -v -run '^TestReview' ./internal/modules/workspace` 复跑。

**与既有 handoff 的区别。** P4/Q1、收尾 A/F5 等讨论的是组合根当前没有业务注册者，不能证明列表真正传给模块。本项是在**已经传入模块的非空列表内部**截断分发，现存模块级测试也抓不到。M3 的第一个注册者即使补了单注册者行为测试，仍不足以保护随后 M5 等加入的第二个注册者。

**建议。** 建立至少两个订阅者／否决者的顺序与事务测试，包含“第一个已写、第二个失败”。本轮只证明成员结束订阅循环的缺口，不宣称所有扩展点都测不到；基线循环本身正确，定为 Minor。

## 5. 设计层面的疑问

### D1 — RESTRICT 保住所有权，但失败即停不能保证清理最终完成

这是对既有 P4/Q2、收尾 A-I1 的实测补充，不重复计为新发现。在快照测试库添加模拟跨模块的子表，以 `ON DELETE RESTRICT` 引用一个到期工作区；同一批里另有一个没有子行的健康到期工作区。调用真实工作区清理器，连续三次得到：

```text
run=1/2/3 SQLSTATE=23001 deleted=0 expired_parents_remaining=2
(one referenced, one healthy)
after repairing the child, the next pass deleted both parents
```

单个父行仍被引用会回滚整批，连健康父行也不前进；`server/internal/platform/jobs/purge.go:84–85` 随即返回，后面的清理器不运行。River 重试不能消除永久残留的引用，只能在子行后来确实被清掉时恢复进度。RESTRICT 是防止跨模块级联绕过附件文件清理的合理完整性护栏，不能把它当作故障恢复协议。

M3 可比较三条路径：①保留现方案，明确监控、诊断和修复残留引用的运维契约；②由拥有子表的模块提供阻塞父 ID／可删候选，父模块只消费候选，保留 SQL 归属边界；③按依赖分支／小批隔离失败，跳过不安全父分支，但允许无关清理器继续。第三种不能简单地“任意错误都继续”，否则失去先删文件、后删行的顺序保证。模拟表证明的是机制边界，尚不是 M3/M7 真正表形状上的故障。

### D2 — 扩展点足够承接 M3，但不能用静态接线测试代替业务验收

成员结束的载荷包含账户、一组工作区、原因、执行者、时刻；恢复和删除也带执行者与时刻。当前否决位于写前，订阅位于写后且同事务，单订阅者失败的现有测试与本轮双订阅者实际写入探针都支持这一点。R6 要补的是测试保障，不是重写扩展点。

M3 handoff 第 1 项对“组合检查只证明静态可达”的判断准确。首个注册者必须经移出、离开、接口停用、CLI 停用、邀请恢复、CLI 恢复、工作区删除分别证明接线；不能因为构造函数在调用图里就算验收。

加入新成员与改角色没有事件也确实如此（`accept_invitation.go:108–110`、`update_member.go`）。当前按请求读取权限无需事件；将来降为 guest 会失去 workspace_access 的默认权限，M5 长连接的关闭不能只订阅结束／恢复事件。M3 应先明确事件全集再接入 M5，现有 handoff 已承认需要修改 M2 用例的例外，不再把它列为新遗漏。

### D3 — 邮箱的“规范化”应继续保持精确定义

实际规则是去首尾空白、ToLower，再精确比较；没有 NFC／NFKC、IDNA 或完整 Unicode case folding。65,315 个通过应用校验的 BMP 域名字符样本中，Go 和本次 PostgreSQL 的 lower 结果一致；带大小写／首尾空白的邀请注册成功，而 NFC／NFD、Unicode 域名／punycode 仍可为不同标识。这个结果不等于“支持所有邮箱服务商的等价规则”。

本轮不建议临时扩大等价关系：改变规范化会影响已有用户的唯一性、邀请对应关系和迁移，需要单独设计。R5 说明即使只换成 EqualFold，也可能引入错误授权。当前实现与 `shared.NormalizeEmail` 的既定规则一致，这里不是新的基线错误。

## 6. 测试缺口与变异结果

### 6.1 本轮实际变异

每项单独变异、执行、复原。表中 Go 范围均加 `go test -race -count=1 -p 1`（契约两项为 `-p 3`）；前端在 `web/apps/web/` 运行 `pnpm exec vitest run src/<所列文件>`，V12 同时指定两个文件。

| 编号 | 错误变异 | 现存测试范围 | 结果与证据 |
|---|---|---|---|
| V1 | `getWorkspace` 声明从未返回的 `workspace.review_probe` | `./internal/modules/workspace/adapter/http` | **捕获**，exit 1；`apitest.Main` 报 `declares problem codes that no test answered`；`main/mutation-contract-unanswered.log` |
| V2 | 从 source 与 bundle 删除真实返回的 `workspace.not_found` 声明 | 同上 | **捕获**，exit 1；`TestGetWorkspace` 报 `problem code ... is not declared`；`main/mutation-contract-undeclared.log` |
| V3 | MAC 解码去掉 `Strict()` | `./internal/modules/workspace/adapter/mac` | **捕获**，`TestTokens` 失败；`access/M01-nonstrict.jsonl` |
| V4 | 邀请密钥改用刷新令牌的 HKDF info | bootstrap 的 `TestTheInvitationKeyIsPinned` | **捕获**，已知答案不符；`access/M02-key-info.jsonl` |
| V5 | 成员列表给 guest 也附邮箱 | `./internal/modules/workspace/app` | **捕获**，`TestListMembersShowsEmailsToAdminsAndMembersOnly/guest` 失败；`access/M03-guest-email.jsonl` |
| V6 | 邀请邮箱精确比较改为 `strings.EqualFold` | workspace/...、access/...、bootstrap 的 9 个包 | **幸存**，619 条通过记录；新 HTTP 反例会失败，见 R5 |
| V7 | 成员结束仅通知第一个订阅者 | workspace/app 与 workspace 模块根 | **幸存**，163 条通过记录；新双订阅者两种场景都会失败，见 R6 |
| V8 | 把成员结束订阅循环移到成员写入之前 | `./internal/modules/workspace/app` | **捕获**，3 顶层＋2 子测试失败：停用、移出、离开；`concurrency/mutation-subscriber-before-write.jsonl` |
| V9 | 去掉 `<Outlet key={workspace.id}>` 的 key | `pages/workspace/workspace-layout.test.tsx` | **捕获**，1/9 失败；`frontend/M1-shell-key.log` |
| V10 | members SWR 键去掉 workspace.id | `pages/workspace/members-page.test.tsx` | **捕获**，1/14 失败；`frontend/M2-member-swr-key.log` |
| V11 | membersOf 缓存由 ID 改为 slug | `stores/root.store.test.ts` | **捕获**，1/4 失败；`frontend/M3-store-by-slug.log` |
| V12 | invitationLink 把 token 放进 query | `app/invitation-link.test.ts`、`pages/workspace/invitations-section.test.tsx` | **捕获**，6/24 失败；`frontend/M4-token-in-query.log` |
| V13 | DeactivateDialog 不传 problem 文案的 texts | `pages/settings/security-page.test.tsx` | **捕获**，1/8 失败；`frontend/M5-dialog-texts.log` |
| V14 | 邀请预览 SWR 键去掉 token | `pages/invitation.test.tsx` | **捕获**，1/14 失败；`frontend/M6-preview-key-token.log` |

前端 6 项共 73 次测试执行，11 失败、62 通过。还原后的相关 82 条 Vitest 全绿；Go 的契约包、权限／邀请 9 包、workspace app 包及双订阅者对照均再次通过。可执行变异脚本为 `main/contract_mutations.py`、`access/run_mutations.py`、`frontend/mutate.py`，并发线保留两个 `.diff` 及 `.jsonl`。

正式的新测试缺口只有 R5、R6。R1–R4 由基线上的额外探针直接揭示，不能把“已有测试没有失败”另复制成四项发现。前端临时修复对照用于确认根因，单独记录，不混入错误变异的捕获率。捕获率只描述本轮挑选的样本，不能外推整体覆盖率。

### 6.2 E2E 页面与接口版本的对等核对

| 故事 | 共同数据库断言／明确例外 |
|---|---|
| W1 创建 | 两版调用 `expectNewWorkspace`、`expectNoWorkspaceAdded` |
| W2 开关／CLI | 接口拒绝后 `expectNoWorkspaceAdded`；CLI 成功 `expectNewWorkspace`、失败不新增。页面无创建入口，因此不制造写入 |
| W3 列表与外壳 | 读故事；两版没有业务落库断言，既有收尾 C-Q2 已说明 |
| W4 改名／删除 | 两版调用 `expectRenamed`、`expectDeletedWithItsMembers`、`expectInvitationsDeletedWith`，页面夹具确有其他成员与待接受邀请 |
| W5 邀请／撤回 | 两版调用 `expectPendingInvitation`、`expectInvitation` |
| W6 接受／恢复 | 两版使用 `expectInvitation`、`expectMembership`；预览为公开操作，接口故事另含已有成员不改角色、恢复保留创建时间 |
| W7 带邀请注册 | 两版检查拒绝不新增、注册不消费邀请与 `expectMembership`；页面额外检查新手引导。注册发生在取凭证之前，是设计允许的例外 |
| W8 角色 | 两版改角色后 `expectMembership`；访客列表另核对邮箱隐藏 |
| W9 移出／离开 | 两版 `expectMembership`、`expectInvitation`，页面实际展示并移除连带失效的邀请 |
| W10 停用／恢复 | 页面与接口共用 identity 的 `expectDeactivated`、workspace 的 `expectMembership`；CLI 恢复沿用 `expectMembership` |
| W11 引导 | 创建的两版 `expectNewWorkspace` 与 `expectOnboardingSteps`；已加入／关闭创建是页面的额外分支 |
| W12 清理 | `expectPurged`；后台任务单版本是设计允许的例外 |

这证明共享落库断言的结构落实了，不等于两版穷尽相同的全部分支。W6 的请求地址断言不自动覆盖 Referer，本轮另做真实浏览器头部采集。现有“每个认证接口接受 PAT”的组合测试之外，本轮为 10 类权限／错误优先级场景分别发会话令牌和 PAT，共 20 格，并增加同邀请两类凭证并发接受。

## 7. 核实过、没有问题的部分

### 7.1 权限、邀请与生命周期

| 检查项 | 证据与结论 |
|---|---|
| 权限规则、目标隔离、404／403／422／409 | 基线权限矩阵通过；额外 20 格覆盖另一工作区的成员／邀请 ID、无权时非法内容、改自己；顺序符合 13.1 第 4 条。非法 UUID／结构错误由边界先答 400，不属于领域取值优先级 |
| guest 邮箱 | 列表 JSON 的 email 为 null，错误答复没有受保护邮箱；角色矩阵、两凭证探针、W8 页面与接口都通过。业务日志用资源 ID，不记录邀请邮箱／令牌 |
| HMAC 与派生 | HMAC-SHA256 截断 16 字节、`hmac.Equal`；邀请与刷新使用不同 HKDF info。去掉 Strict、改成刷新 info 的变异被现有测试捕获；CR／LF 例外见 R4 |
| 预览与链接 | 预览只给工作区名称／slug与邀请角色；令牌在片段和请求体。浏览器登录错账户→接受被拒→退出→正确账户接受，共采集 32 请求（14 API）、31 个非空 Referer；URL、Referer、控制台均未见令牌，唯一控制台错误是预期的 403 |
| 注册关闭与邮箱 | 无邀请／错误邮箱／无效邀请被拒，合法邀请可注册而不自动接受；大小写与空白按同一规则处理。Unicode 等价边界见 D3、R5 |
| 接受／撤回／结束／删除 | 已消费／撤回／删除的邀请返回 404；结束成员身份删除发给当前邮箱的待接受邀请；恢复保留最初 created_at，有效成员接受不覆盖原角色 |
| 唯一待接受邀请 | 部分唯一索引与原有“双管理员邀请同邮箱”交错通过；新增同邀请会话／PAT 双接受，两个先后均得到先 200、后 404；双管理员撤回得到无序的 204＋404，重复 3 轮成立 |
| 停用、恢复与 CLI | 账户停用时增长被锁阻止；账户未启用、工作区已删除时恢复失败；删除与恢复两个先后均正确。R1 是另一个账户增长使空工作区重新变为非空的缺口 |
| slug 复用与保留期 | 部分唯一索引允许软删除后复用；新旧工作区按 ID 隔离；W4、W12 与额外清理边界探针通过 |

### 7.2 第 13 节的新约定逐项核对

| 条文 | 结论／边界 |
|---|---|
| 13.1/1–2 事务与变更集归属 | 当前工作区写操作经事务，审计字段／扩展事件带执行者；未来页面的完整写入管线、变更集不属于已实现的 M2，不能由本轮提前验收 |
| 13.1/3 权限表、Actions 并集、Grant、豁免 | 代码与矩阵／架构检查一致；命令行不构建 access |
| 13.1/4 错误码优先级与资源自己的 404 | 额外跨工作区 ID、非法取值的双凭证探针成立；增长路径先检查自己的账户是已明确的例外 |
| 13.1/5 父行锁、第一条语句、增长锁、全局顺序 | 原 13 个交错通过。新增 FK 实测各 3/3：持 users 非键 UPDATE 并排队真实 `Users(SetEmail)` 时，HTTP 邀请的 FK KEY SHARE 可先完成；测试事务直接 UPDATE 唯一 email 列时，HTTP 邀请等待其提交后 201。两者未见死锁／5xx。R1 不否定锁序，否定锁下业务检查的完整性 |
| 13.1/6 软删同刻、清理登记与所有权 | 当前模块内逻辑与双清理探针成立；跨模块 RESTRICT 是完整性屏障，进度局限见 D1 |
| 13.1/7–8、13 迁移归属、错误码与生成物 | workspace 迁移与契约归本模块；HTTP 契约测试、生成物检查通过，额外的声明多一项／少一项两个反向对照均失败 |
| 13.1/9 同一时刻 | 前进时钟与 SQL 微秒级断言在基线通过；事件携带同一时刻 |
| 13.1/10 日志只记 ID | 用例、CLI 日志测试通过；额外权限探针确认失败响应不含受保护邮箱。这些 HTTP 探针使用 DiscardHandler，不能充当额外日志泄漏检测；未测试外部反向代理日志 |
| 13.1/11、21 注册者与模块入口 | 导入／组合检查与载荷核对成立；双订阅者基线事务对照成立，现存测试缺口见 R6；空列表接线局限见 D2 |
| 13.1/12、14–17 平台、配置、迁移与默认认证 | 架构、中间件、内置配置、迁移上下往返、精确运行时权限、公开操作与 PAT 全操作检查在完整门禁中通过；M2 没有新的长连接路由 |
| 13.1/18 账户共享锁与锁下邮箱 | 创建／接受／CLI 恢复从账户锁开始；set-email 交错与新探针符合锁下邮箱，不使用请求中提供的邮箱授权 |
| 13.1/19–20 事务工作与限流 | M2 邀请 MAC 无昂贵哈希／随机令牌生成；公开预览与受认证操作沿平台不同桶，现有 429／Retry-After 与顺序测试通过；未做流量压力验收 |
| 13.1/22 管理命令 | 共享组合、stdout／stderr、退出码 1、不变状态不发事件，与 README 及 W2／W10 一致；R1 的无人管理员状态例外应补契约 |
| 13.1/23–24 后台任务与 SQL 归属 | 按注册顺序、每批提交、失败即停属实；sqlc 模块边界与生成物检查通过 |
| 13.1/25 派生密钥／严格解码 | info、HMAC、常量时间比较属实；“唯一拼法”不成立，见 R4 |
| 13.1/26 顶层路径与保留名单 | 前后端一致性测试通过；已新增的 create-workspace／invitations 均在 app 段 |
| 13.2/1 分代、串行、写计数、去重、404删除 | store 层主体正确；旧邀请回调导航是 R2，不能写成旧请求都会失败 |
| 13.2/2–5、8–9 层次、生成类型、双语、CSP与控制台 | 静态门禁、构建、双语文案键与既有 CSP 故事通过；正常浏览器故事的控制台检查生效，预期 4xx 显式声明 |
| 13.2/6–7 依赖方向、SWR重试 | 静态门禁及每个页面的加载／重读测试通过；没有在用户表单上自动重发 |
| 13.2/10 公开邀请页与守卫 | 正常登录／注册／退出在页内成立；带邀请新用户正确经引导；在途旧回调例外见 R2 |
| 13.2/11–12 错误文案、texts、表单、确认对话框 | problem 码的三处文案替换有测试；字段码全局映射与收尾改正文档一致；表单草稿例外见 R3 |
| 13.2/13–14 秘密与日期 | 邀请链接是文档明确的可重复生成例外，实际泄漏检查见 7.1；既有 PAT 与日期测试在完整门禁中通过，未重新做全部 M1 对抗实验 |
| 13.2/15–16 按ID缓存、SWR键、外壳key、移除跳转 | 按 ID 缓存、成员 SWR 键、Outlet key 的三项独立变异均被捕获；删除／离开由既有真实故事验证，旧列表响应未污染新账户的有限浏览器观察与机制一致 |
| 13.2/17 列表命名、菜单、焦点、中英 | 真实 Chromium 的中英可访问树、方向键不发PATCH、选择后焦点返回、中文确认对话框取消后焦点返回均通过；整页离开的焦点仍是已知 M3 handoff |
| 13.4/3–6 对等验收、交错、契约、矩阵 | 原门禁成立；额外契约双向变异均失败。R5／R6说明特定语义仍缺样本，不能把门禁全绿等同于不变量穷尽 |

未在 M2 新增的基础约定（配置、迁移上下往返、默认认证、中间件、依赖方向、国际化键与静态检查）在完整门禁中仍通过；M4 以后的变更集、页面管线与树清理不能由 M2 的测试提前证明。

## 8. 未能验证的部分

- 没跑镜像冒烟，遵照委托要求，不对本次镜像构建、镜像标签或部署声明作结论。
- 浏览器只验证本机 Chromium；没有实测 Safari、Firefox、读屏软件的朗读、窄屏布局或非安全内网 HTTP 的全部行为。角色控件、焦点和中英文可访问树有实测，不等于完整 WCAG 审计。
- M3／M4／M7 的真实注册者、笔记本／页面自引用／附件文件清理尚未实现；RESTRICT 的实验是临时模拟子表，不能代替后续模块的集成测试。
- 没有大数据量／长期运行／多实例 River 选主压力测试，也没有声称有限并发探针穷尽所有调度；目前确认的是指定交错及正反顺序。
- 普通旧列表响应的浏览器负断言包含一个短观察窗口，只作为辅助观察；不能据此证明任意时延下所有旧回调均安全。R2 的错误导航使用实际 URL／可见结果的正断言，不依赖该窗口。
- 常量时间性质由 `hmac.Equal` 的代码使用确认，没有做微架构计时侧信道实验。邮箱扫描覆盖被应用接受的 BMP 域名字符，不是所有真实邮箱服务商、Unicode版本或 IDNA 策略的穷举。
- 本报告不修源码、不补正式测试、不代替作者处置。所有建议须在修复提交中补回归、重新执行对应门禁；本报告中的测试通过数字仅属于 `646964a` 与明确标注的临时实验。
