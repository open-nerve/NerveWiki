# M3 笔记本与权限：Codex 对抗性质量评审

日期：2026-10-02。评审基线：`3470299c655c13868b42eab423ba27c8edac18f6`（`3470299`）；改动范围：`424a2fc..3470299`，308 个文件、29,242 行新增、516 行删除。下文源码、测试与文档行号均指这个基线。

## 1. 结论

**常规门禁全部通过，后端本次选取的权限与并发反例没有打破基线实现；前端仍有四处 Important 行为错误和一处同名对象辨识问题，权限一致性测试另有一个可重复的盲点。建议先修复这些问题、补齐回归，再把 M3 作为已满足长期约定的基础交给 M4。** M4 的设计工作可以继续，本轮未发现需要重做后端模块边界与扩展点的证据。

正式发现 **7 项：Critical 0、Important 4、Minor 3、Nit 0**。其中 5 项为实现／交互问题，1 项为测试缺口，1 项为文档问题。没有证实基线中的后端越权、访客获得别人的邮箱、私密名称进入日志、持久化业务数据损坏或死锁。R5 中错误的列表角色来自刻意变异，不能当作基线漏洞；R2 是降级后已加载界面没有及时收起，未取得新的受限数据。

最值得关注的三件事：

1. **写队列和页面生命周期没有完全覆盖成员管理。** 角色菜单的发送状态随组件卸载消失，同一成员的两个写可以重叠；旧答复把界面角色覆盖成旧值，数据库已经是新角色（R1）。
2. **数据按工作区隔离，不代表回调和草稿也隔离。** 侧栏的新建对话框不在带 `key` 的 Outlet 内：切换工作区后，旧创建回调仍把用户带回 A，未发送的 A 草稿则能被提交到 B（R3）。添加成员也遗漏了对在途编辑的保护（R4）。
3. **“列表等于逐项读取”的测试缺少最关键的一种角色组合。** 正式成员的显式 reader 与 editor 开放程度冲突时，列表单独退化为“显式优先”，相关 Go 测试和全部 35 个笔记本 E2E 仍然通过（R5）。M4 将区分 editor／reader 的实际写入能力，应在接入前补齐。

## 2. 评审范围与方法

### 2.1 阅读、分工与去重

评审分为权限／审计／契约、并发／级联／扩展点、前端／真实浏览器三条独立线；主审执行完整门禁、CLI 组合探针、假服务端对照，并独立复跑主要浏览器反例。

阅读和核对范围包含 `docs/README.md`、v0.1 总体设计的指定章节和第 13 节、M3 总设计与五个 Phase、全部对应 plans、设计／Phase／收尾审查、M2→M3 handoff、M3→M4／M5／M7 的指定 handoff，以及 M2 的 Codex 评审。历史设计按最终决定解释，不把已经明确调整的行为重新列作问题。

没有重新报告：有效角色提示留给 M4、`main` 与侧栏布局、`LockHoldings` 的快照前提、跨模块 RESTRICT 的失败即停、组合根空注册集合的最后一跳、fake 的 reader／editor 变异已有解释等。R2、R3、R6 是有新实证的修复不完整，详情分别与 P5/M1、P4/m1、P5/N3 区分。

### 2.2 隔离与门禁

原仓库始终在 main，开始时 `git status --short` 为空。四份 `git archive 3470299` 快照位于 `/tmp/nwiki-m3-review.VXiSVG/{main,access,concurrency,frontend}/repo`；各自执行 `pnpm install --frozen-lockfile` 并初始化快照 Git 提交。所有构建、临时测试、变异均在这些快照中进行。

| 命令／验证                                                                                           | 实测结果                                                                                                                                                              |
| ---------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `pnpm install --frozen-lockfile`                                                                     | 四份快照均 exit 0，锁文件未改                                                                                                                                         |
| `GOFLAGS='-p=3' make lint knip`                                                                      | exit 0；两处 Go lint 均 0 issues；Markdown 样例 61＋改写 4；oxlint、格式、类型检查、knip 通过                                                                         |
| `GOFLAGS='-p=3' make check`                                                                          | exit 0；Go 主模块 43 个有测试包、13 个无测试包，另跑 bodyshape 预算和 tools：共 45 条成功包执行记录；包含 `-race -count=1`；Vitest **63 文件、976/976**；前端构建成功 |
| `GOFLAGS='-p=3' make gen-check`                                                                      | exit 0；SQL、Go、OpenAPI bundle、TS 生成物无差异                                                                                                                      |
| `NWIKI_E2E_VERSION=0.1.0-dev GOFLAGS='-p=3' make e2e`                                                | exit 0；Chromium **117/117**，15.6 秒                                                                                                                                 |
| `NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test stories/notebook --repeat-each=3 --workers=3` | exit 0；原有 35 个笔记本用例 × 3＝**105/105**，19.8 秒                                                                                                                |
| CLI 新组合探针 `review-cli.spec.ts --repeat-each=3 --workers=1`                                      | **3/3**，3.6 秒；停用→改邮箱→启用→恢复，两本归还、接管／删除的不归还，再恢复无重复审计                                                                                |
| 并发新探针＋既有关键交错 `-race -p 3 -count=10`                                                      | **140 次顶层、610 次含子测试通过事件**，101.8 秒，0 fail                                                                                                              |
| 最终新增并发／生命周期探针 `-race -p 3 -count=3`                                                     | 7 组，共 **21 次顶层、102 次含子测试通过事件**，0 fail                                                                                                                |
| 前端基线与变异                                                                                       | 独立 Vitest 同为 976/976；4 项错误变异均被现存测试捕获                                                                                                                |
| 真实浏览器新增探针                                                                                   | **13 条：7 个正向通过，6 个断言失败对应 5 项发现**，29.1 秒；其中 R3 的两个现象归为一个发现。主审另独立复跑 9 条：3 个正向通过、6 个反例均重现                        |
| 权限／游标新增探针                                                                                   | **5 个顶层、55 个逻辑场景**全部通过；其中 HTTP 的 4 组在 `-race` 下复核，游标 1 组；矩阵基线 367 格通过，其中名称含 Notebook／Notebooks 的 M3 格 271                  |
| 假服务端探针                                                                                         | 2/2 断言出与真实服务端不同的默认写行为，见 D1                                                                                                                         |

Go 测试都使用 `-p 3`，Makefile 经 `GOFLAGS=-p=3` 传入。各测试集重叠，上表的重复次数和父／子测试事件不能相加成独立业务用例数。浏览器反例按期望的正确行为断言，因此退出码 1 是缺陷证据，不是“门禁失败”；原有测试的成绩单与新增探针分开保存。

新增探针共 **28 个测试单元、94 个逻辑场景**：权限／游标 5／55，并发／生命周期 7／23，浏览器 13／13，CLI 1／1，fake 对照 2／2；不含 prepare、重复执行或原有故事。逻辑场景包括表格循环，不冒充 Go 子测试事件。错误变异共 **38 类：37 类被既有测试捕获，1 类在所执行的相关 Go 与笔记本 E2E 中幸存**（R5）。没有把短单测筛选中的幸存直接算作缺口：放宽 notebook.update 授权、锁下使用旧无主状态两项在扩大到现有权限矩阵／确定性交错后都被捕获。

### 2.3 工具、证据与实验失误

工具为 Go 测试与 race detector、PostgreSQL `18.6-trixie` Testcontainers、pnpm／Vitest、Playwright Chromium、SQL 锁等待握手、Python 变异脚本和 Git。浏览器使用构建出的真实 Go 服务、真实数据库、真实内嵌前端；`route.fetch()` 取得真实服务端答复后仅延迟交付，没有伪造成功响应。

可重放的临时测试、变异 diff、命令和原始输出保存在上述父目录。主审门禁日志是 `main/{install,lint-knip,check,gen-check,e2e,notebook-repeat}.log`；独立线各有 `evidence.md`、变异清单与探针文件。下文也给出场景、关键命令与输出，结论不只依赖临时文件。

保留但不计作产品问题的实验失误：主审曾同时运行 gen-check 与 E2E 构建，后者撞上生成文件暂时被删除，已顺序重跑成功；CLI 探针最初误把停用 PAT 的认证失败期待为 403，核对认证实现和既有测试后改为 401；一次浏览器筛选正则没有选中测试；权限探针初版用了不存在的测试响应字段。浏览器旧会话响应在 auth middleware 收到 headers 后即抛 `SessionChangedError`，不消费 body；探针额外等待 `Response.finished()` 的两次尝试悬挂，已按确切 runner PID 中断并完成 fixture 清理。最终用被拦请求对应的 response 事件和主页面两次渲染周期核对隔离，不声称旧 body 被业务消费。没有把这些测试设施问题当作产品发现。

未运行 `make image-smoke`，未访问其他项目、Obsidian 或外部业务系统，未推送。只操作自己启动的测试进程与容器。四份快照最终 `git status --porcelain` 为空，生产变异与临时测试均已复原／移出；access 的变异二进制也已重建为基线。复查了全部容器（包括 Created／Exited）与日志中的自有 ID，本轮没有残留测试容器；没有路径指向这四份快照的测试进程。其他任务在评审期间创建或结束的容器未干预。原仓库提交前仅新增本报告，暂存区只含本报告。

## 3. 发现清单

| 编号 | 级别      | 类别     | 一句话描述                                                  | 基线位置                                                         |
| ---- | --------- | -------- | ----------------------------------------------------------- | ---------------------------------------------------------------- |
| R1   | Important | 实现     | 成员角色写没有 store 队列，页面重新挂载后迟到响应覆盖新角色 | `web/apps/web/src/stores/notebook-member.store.ts:62`            |
| R2   | Important | 实现     | 审计“加载更多”的 403 不跟随角色降级，管理员界面继续保留     | `web/apps/web/src/pages/workspace/audit-section.tsx:50`          |
| R3   | Important | 实现     | 侧栏新建对话框跨工作区复用，旧草稿与创建回调越过作用域      | `web/apps/web/src/pages/workspace/create-notebook-dialog.tsx:37` |
| R4   | Important | 实现     | 添加成员的旧成功回调清空请求期间的新选择                    | `web/apps/web/src/pages/notebook/add-member-section.tsx:79`      |
| R5   | Minor     | 测试缺口 | 列表单独退化为“显式角色优先”，相关 Go 和笔记本 E2E 全绿     | `server/internal/modules/notebook/app/read_test.go:19`           |
| R6   | Minor     | 实现     | 同一原所有者的同名无主笔记本仍无法区分操作对象              | `web/apps/web/src/pages/workspace/ownerless-row.tsx:34`          |
| R7   | Minor     | 文档     | README 仍称引导只有两步，遗漏已强制加入的笔记本步骤         | `README.md:170`                                                  |

## 4. 每个发现的详情

浏览器探针的重放方式：将 `frontend/review-probes.spec.ts` 放回快照的 `e2e/stories/notebook/`，在该快照先 `make build`，再从 `e2e/` 执行下列命令；设置 `NWIKI_E2E_VERSION=0.1.0-dev`。探针编号是证据文件的编号，不等于本报告发现编号。

### R1 — 成员角色的旧答复覆盖新答复

**场景与复现。** 管理员把 Bob 从 editor 改为 reader。请求已在服务器提交，扣住其真实 200 答复；点击笔记本设置的 General，再回 Members，给 Bob 改成 admin。第二个请求成功、页面显示 Admin 后，释放第一个答复。

```sh
NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test \
  stories/notebook/review-probes.spec.ts --grep 'R3 role writes' --workers=1
```

`frontend/R3-role-order.json` 与主审 `main/browser-recheck/R3-role-order.json`：

```text
requests: 2
expected: Admin
actual: Reader
dbRole: admin
```

**根因。** `web/apps/web/src/stores/notebook-member.store.ts:62–64` 直接发送更新，再把整个成员资源写回列表，没有 `oneAtATimeById`。`web/apps/web/src/app/role-menu.tsx:35–45` 的 `sending` 只属于当前菜单实例，页面卸载再挂载后重新为 false。旧写仍属于同一登录代、同一本笔记本，不能由会话分代挡住。`changesAnswered` 仅解决读写重叠，不能解决两次写答复反序。

**影响。** 数据库保留 admin，但界面告诉操作者此人是 Reader；权限管理的状态判断失真，直到后续重读。违反 13.2 第 1 条的整个资源写排队约定。P4/M1 已修的是 `NotebookStore` 的名称／开放程度两个表单，不包括这个新反例。

**建议。** 在每代持久的成员 store 中按成员 id 排队角色修改及同资源的其他写，页面的 sending 只用于交互提示；加入卸载／重挂后两次写的真实响应交错测试，核对数据库与 UI 一致。

### R2 — 审计下一页绕过角色跟随

**场景与复现。** 管理员打开有 51 条审计的无主页，首屏已经加载 50 条。另一名管理员经真实 API 把他降为 member。不触发窗口焦点变化，点击 Load more，服务端正确答 403 `forbidden`。

```sh
NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test \
  stories/notebook/review-probes.spec.ts --grep 'R2 audit Load more' --workers=1
```

证据 `frontend/R2-audit-role.json`：点击后新增请求只有审计下一页，没有 `GET /api/v0/workspaces`；页面出现拒绝提示，但仍有 Ownerless notebooks 管理员导航和原来的 50 条审计，未切成“Only the workspace's admins see its ownerless notebooks.”。主审独立复跑得到同样失败。

**根因。** `audit-section.tsx:43–45` 只把 `useFollowRole()` 交给 SWR 的第一页读取。`50–58` 的 `more()` 直接调用 `audit.more()`，catch 只 `setFailure`。它不会进入 SWR 的 onError。

**影响与去重。** 违反 13.2 第 18 条及 P5 3.2 的“这些只给管理员的读答 403 后重读工作区”约定。P5/M1 已处理首页／清单／审计第一页和按 id 写，这次证实分页分支漏掉了。这里没有新的后端信息泄露：保留的是在有权限时取得的数据；问题是权限界面已知道请求被拒仍不跟随角色。

**建议。** 第一页与更多页共用同一个拒绝处理；403 时刷新工作区，普通网络失败仍保留就地重试。增加无焦点重读的“50＋1 条、降级、更多页 403”用例。

### R3 — 侧栏新建对话框的草稿和回调跨工作区复用

**场景与复现。** 先访问工作区 B，再经切换器进入 A；从 A 左栏打开 New notebook，提交创建。服务器已经在 A 写入，扣住真实 201 响应。用浏览器 Back 回 B，确认地址已是 B，再释放响应。

```sh
NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test \
  stories/notebook/review-probes.spec.ts --grep 'R(12|13) ' --workers=1
```

主审 `main/browser-recheck/R12-create-workspace-switch.json`：

```text
expected: /b-8896600a19a5
actual: /ws-8896600a19a5/notebooks/01a0fa91-2fb2-71eb-844e-01ab453ee63e
db: workspace_id 是 A，name 是 Created in A
```

**第二个独立复现。** 在 A 的新建对话框输入 `Draft for A`，还没有发送，浏览器 Back 到 B。对话框保持打开、名称保留；点击 Create，真实请求发往 B，SQL 也落在 B。主审独立复跑了两个现象；第二个证据在 `main/browser-recheck/R13-create-draft-workspace.json`：

```text
originalWorkspace(A): 01a0fa99-721d-7418-b076-f25c173622b5
currentWorkspace(B):  01a0fa99-7220-74ac-9582-96a0c68826df
postedTo: /api/v0/workspaces/b-2a58fdb50ffb/notebooks
DB: workspace_id=B, name="Draft for A"
```

探针末尾以 A/B id 不应被表单静默替换作反向断言而失败；修复后的推荐行为是切换工作区时关闭并丢弃该对话框，随后在 B 新开表单，而不是让留在 B 的用户继续写 A。

**根因。** `workspace-layout.tsx:82` 的 `NotebookNav` 在 `:86` 带 `key={workspace.id}` 的 Outlet 外，切工作区时仍复用。`create-notebook-dialog.tsx:29–45` 的轮次只跟随打开／关闭；`creating()` 捕获旧 `workspace.slug`，未检查路由或工作区已变，答复仍执行 navigate。未发送时，`:77–89` 的表单 state 保留旧名称，却已经从新 workspace 取到 B 的 store。

**影响与去重。** 用户明确离开 A 后被旧请求带回 A，或把 A 中的草稿提交到 B；store 的工作区键本身没有串数据，但草稿和路由副作用越过了边界。P4/m1 修过取消后的导航，本次是没有触发 `onOpenChange` 的工作区切换，并未受保护。直接违反 13.2 第 16 条“前一个工作区的表单状态不能带到下一个”。此次两个工作区均为操作者有权写入的工作区，没有证实权限绕过。

**建议。** 工作区边界必须覆盖侧栏对话框，且异步完成回调在作用域改变／卸载时失效；不能只依赖组件重新挂载，因为旧闭包仍可完成。创建已经在服务器成功时保留它在 A 的列表更新，避免再改变 B 的路由。同时加入未发送草稿的 R13 场景，不能仅修掉迟到导航。

### R4 — 添加成员清掉后来选择的候选人

**场景与复现。** 成员添加表单选候选 A，发送 POST；真实服务端已成功，暂缓响应。下拉框仍可编辑，用户改选候选 B。释放 A 的答复后，候选被重置为“Choose a member”，B 的新选择消失。

```sh
NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test \
  stories/notebook/review-probes.spec.ts --grep 'R1 add member' --workers=1
```

证据 `frontend/R1-add-draft.json`：`expected` 是候选 B 的 user id，`actual` 为 `""`；数据库断言确认只添加了原先请求的 A。主审重复观察到同一结果。

**根因。** `add-member-section.tsx:79–81` 在 await 后无条件 `setUserId("")`；`:108–110` 允许在途改选，没有编辑版本计数。禁用的是提交按钮（`:142`），不是两个下拉框。

**影响。** 丢失尚未提交的新选择，违反 13.2 第 12 条已经明确的“成功回调清空字段前核对发送后的编辑”。这与 M2 改名／邀请表单已处理的问题同型，但位置是 M3 新的添加成员表单，不是重新报告旧代码。丢失的是 UI 草稿，未发生数据库数据丢失。

**建议。** 保存发送时的编辑版本，仅在候选／角色未再改动时重置字段；测试同时覆盖候选改变和角色改变。

### R5 — 列表有效角色的非等价错误变异幸存

**基线正确，缺口在测试。** `server/internal/modules/notebook/app/list_notebooks.go:44` 正确调用 `shared.EffectiveNotebookRole`。仅在构造列表 View 后加入：

```go
if n.Explicit != "" {
    views[i].Role = n.Explicit
}
```

该改动让“工作区正式成员＋显式 reader＋editor 开放”在列表里被错误降为 reader，单本读取仍答 editor。

**复现与结果。** 变异 diff 是 `access/A18-list-explicit-first.patch`，脚本与逐条输出在 `access/`。临时探针先移出，然后对 `shared`、`access/...`、`notebook/...`、`bootstrap` 的 12 个相关包逐包执行 `go test -count=1 -p 3 -json`，**220 个顶层、1035 个含子测试的通过事件，全部通过**（3 个无测试文件包未虚算测试）；以变异源码重新构建，再跑：

```sh
make build VERSION=access-review
# 转到快照 e2e/ 后：
NWIKI_E2E_VERSION=access-review pnpm exec playwright test stories/notebook --workers=2
```

这里的 version 与重建产物一致；完整基线门禁使用前述 `0.1.0-dev`。原有 **35/35** 通过，9.9 秒。新增 `TestReviewListUsesEffectiveRoleProbe` 在基线通过，变异后失败：

```text
bob: list=[{Name:Open Role:reader MemberCount:3}]
     read={Name:Open Role:editor MemberCount:3} want both role=editor
bob explicit reader: list role reader, get role editor, want editor
carol explicit reader: list role reader, get role reader, want reader
```

**为什么现有“等价”检查没抓到。** `app/read_test.go:19–23` 只有私密 reader、开放但无显式角色、开放 admin，没有默认权限高于显式权限的非访客成员。`bootstrap/notebook_visibility_test.go:24–55` 确实比较列表和单本读取的角色，并非只有可见 id；但它复用的种子也缺这个组合。N4 的接口版本检查的是单本 GET 的 editor，没有同时断言该账户的列表角色；M3 页面主要区分 admin 与非 admin。

**影响与去重。** 这不是收尾 B-Q2 的前端假服务端算法变异，而是真实后端列表响应单独漂移、现存相关集成和 E2E 都无法发现。M4 将区分 reader／editor 的写入能力；若页面使用列表 role，这个遗漏可能使界面与实际授权漂移，应在接入前补回归。

**建议。** 增加正式成员的 reader＋editor 组合，并保留访客 reader＋editor 的对照；在同一个真实请求场景核对列表与 GET 都为正确角色。列表一致性检查宜按 id 比较，避免笔记本允许同名而 `map[name]role` 折叠对象。

### R6 — 加上原所有者仍不足以区分同名无主笔记本

**场景与实证。** 同一账户成功创建两本 `Same name`，被移出工作区后两本一起无主。真实浏览器中两条可见信息均为相同名称、原所有者、Private、0 名成员、同一天日期、0 B；两个接管按钮具有完全相同的 accessible name，description 和 title 都为空。两个删除按钮以及输入名称确认也不能区分。

```sh
NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test \
  stories/notebook/review-probes.spec.ts --grep 'R7 same former owner' --workers=1
```

`frontend/R7-duplicate-actions.json` 与主审副本：两条不同 notebook id 对应同一个 `former_owner_id`；`labels.length=2`，`new Set(labels.map(l => l.label)).size=1`，期待 2 的断言失败。

**根因与影响。** `ownerless-row.tsx:34–35,66,73` 只把名称与原所有者组合为标签，两者都不具有唯一性。P5/N3 的修复只解决了不同原所有者的同名笔记本；总体设计 13.2 第 17 条承诺的“能区分的属性”在这里仍不成立。管理员尤其通过控件列表或语音操作时无法确认对应哪一本；不是已经证实的误删或数据损坏。

**建议。** 为同名对象提供稳定、实际可区分的属性，必要时使用短 id；同时在可见行、控件可访问名称和删除确认中保持一致。补“同一所有者、同名、相同日期和计数”的用例。

### R7 — README 的引导流程仍停在 M2

`README.md:170` 明写“现在两步：资料，工作区”，但 `web/apps/web/src/onboarding/steps.ts:27–52` 已注册 profile、workspace、notebook 三步。新增真实浏览器探针 `R5 onboarding` 走完流程并核对数据库：

```text
Step 1 of 3 → Step 2 of 3 → Step 3 of 3
onboarding_steps: [profile, workspace, notebook]
```

原有 A9／N12 也随全量 E2E 通过。README 的笔记本章节虽已补齐，这一面向用户与维护者的流程说明仍遗漏强制的新步骤，尤其没有说明已有完成前两步的账户仍会经过笔记本一步。

**建议。** 改为三步并说明已有笔记本、没有可创建工作区时的跳过行为，与实际注册表及 N12 一致。这是收尾 C-I1 未覆盖到的具体文档遗漏。

## 5. 设计层面的疑问

### D1 — “假服务端按真实规则答复”的承诺应该限定到哪里

13.4 第 7 条写法很广，但 `web/apps/web/src/test/notebook-server.ts:110–179` 的默认写入不检查权限、唯一管理员或自己的成员关系。主审 `review-fake.test.ts` 在实际 fixture 上验证了两例：reader 的 PATCH 成功改名；只有一个 admin 时 leave 成功并把成员清空。对应真实矩阵／规则测试分别为 403、409 `notebook.sole_admin`。

```sh
pnpm --filter @nervewiki/web exec vitest run src/test/review-fake.test.ts
# 1 file, 2 tests passed：断言的是以上两个反事实的默认成功行为
```

页面的拒绝测试目前通过 `answers: problem(...)` 显式提供失败，不能简单把每一处省略判定都叫产品 bug。本轮也没有找到某个现有页面断言依赖上述假成功，从而把已坏行为判对的完整证据，所以不计正式发现。建议明确 fixture 负责的规则边界，至少使未声明的敏感写默认拒绝，或由共用对照表约束它实际实现的部分。否则“较高角色、接管人数、404”三处已经修好，容易被误读为整套服务器规则已经模拟完整。

## 6. 测试缺口与变异结果

**唯一正式幸存项是 R5。** 本次 38 类变异的分布：

| 范围                                                     | 数量 | 结果                                                       |
| -------------------------------------------------------- | ---: | ---------------------------------------------------------- |
| 权限、角色、游标、审计、声明码、日志                     |   18 | 17 捕获；列表单独显式优先 1 项幸存                         |
| 规则一／二、设置／清除无主、归还、锁顺序、事件、活动来源 |   16 | 最终 16 捕获；旧无主状态在狭窄单测幸存，扩大既有交错即失败 |
| 前端分组、审计游标、表单编辑计数、403 角色跟随           |    4 | 4 捕获                                                     |

具体覆盖包括：访客获得默认角色、shared 全局显式优先、越权更新、跨工作区 notebook facts、访客邮箱、非规范游标、最后满页误给游标、分页位置错误、同时间漏行、越过工作区过滤、已删除审计可见、limit 上界、日志加入名称，以及契约删除实际返回码／加入从未答出的码。后端另一组修改规则一／二、移出被误否决、多个管理员也设无主、归还为 reader／不校验恢复行数、归还／工作区删除倒序加锁、吞订阅者错误、漏工作区加入／角色事件、漏删除事件／审计、接管不清无主、模块根不交活动来源。相关现有测试均检出了这些错误。

组合根本来为空的注册者不纳入“新幸存项”；A-Q1 已明确移交第一位注册者。本次对模块根活动来源断线的变异会失败，不能把它和组合根当前无来源混为一谈。

R1–R4 的浏览器反例也表明现有 976 个 Vitest、117 个 E2E 没有覆盖相应生命周期，但不把每个实现缺陷再重复计成一条测试缺口。R6 的同名同所有者也应作为修复回归。

## 7. 核实过、没有发现问题的部分

以下结论仅针对基线、列出的入口和实测场景，不是对所有未来组合的证明。

### 7.1 权限、隐私、游标与契约

- 有效角色在 shared 定义；access 的不可见／禁止次序、真实 SQL 的可见范围、成员列表的邮箱处理符合设计。正式成员 reader＋editor 的基线列表与 GET 都答 editor，访客同样显式 reader 则仍答 reader（R5 的正向对照）。前端成员行显示显式角色的局限已准确移交 M4。
- 私密笔记本对未显式加入的工作区管理员也是 404；按成员 id 的失败用 `notebook.member_not_found`。跨工作区、跨笔记本成员 id、已结束成员关系、已删除资源、停用账户的定向 HTTP 探针没有产生错误授权。
- 会话与 PAT 经同一业务用例；已结束的工作区访问两者均 404，停用后的凭证两者均 401。访客即使成为笔记本 admin，成员列表的邮箱仍为 null；规则二仅给 slug／数量，debug 日志未包含测试邮箱与私密名称。
- 103 条同一时刻的审计记录，分别以 1、2、50、100 的页长翻完，无重复／遗漏；最后满页、limit 边界、严格封套、多余／重复成员、非规范拼法、坏游标先于资源判定均有现存或新增证据。跨工作区复用位置只读到目标工作区的事件，篡改为同形位置不突破过滤。
- notebook HTTP 契约测试通过；删除实际返回的 `forbidden` 声明、加入从未返回的 `notebook.sole_admin` 两项反向对照都失败，双向检查有效。

### 7.2 规则、生命周期、并发、命令和清理

- 规则一与规则二的对象和计数符合设计；只靠 workspace_access 的使用者不算显式成员，结束关系不计入；工作区规则先于笔记本否决。移出不被规则二拒绝。
- 接管的新行、恢复旧行、提升现任成员三种路径；归还为 admin（包括以 guest 回来）；接管／删除后不再归还；`created_at` 保留、设置／清除无主不改 `updated_at`；审计与成员变更在同一事务，失败整体回滚。
- 新增两代所有权场景验证 former owner 随再次无主正确变更，第一代所有者回来不会拿回已由第二代管理的笔记本；只有最新原所有者恢复时归还，returned 审计恰好一次。
- 工作区 SHARE→笔记本 NO KEY UPDATE 与结束／恢复／删除的工作区总闸成立。新增接管／无主删除、离开／添加、开放／移出、删除工作区／七类写均跑双序，并检查管理员与无主恰有其一、有效 notebook member 仍有有效 workspace membership。
- FK 三事务探针实际观察了停用持 users NO KEY UPDATE、后排 set-email、创建的 KEY SHARE 仍前进；实际更新唯一 email 暂停时，接管审计会等原所有者 users，释放后完成，无环。
- CLI 新探针验证：停用后改邮箱，未启用先恢复成员退出码 1；activate 启用账户后，未撤销且未过期的 PAT 再次可认证，不恢复工作区／笔记本成员关系；reactivate-member 打印 returned: 2，接管和删除的不归还；再次执行不新增审计；日志不含测试邮箱／笔记本名称。规则二拒绝的输出／退出码由 N8 命令行版本及 Go 测试核对。
- 普通／无主删除共用软删除；工作区级联的同一时刻、审计随工作区清理、清理叶到根、成员被锁时父笔记本延期、SKIP LOCKED、RESTRICT 都通过现有集成测试与门禁。未把“失败即停”作为新发现；它确实是已明确接受的策略。

### 7.3 前端与长期约定逐项归纳

正常的分组、NotebookLayout 删除／离开后的路由与焦点、改名和开放程度仅在保存时发送、名称确认删除、外部更新跟随、无主管理、引导、审计追加，均在真实 Chromium 的原有故事中执行；新增探针另验证了设置请求排队、发送期间编辑保留、中英文标签和新建焦点。另有四个真实浏览器隔离探针：旧账户 notebooks、ownerless＋audit、notebook-members（管理员换成访客，旧答复含邮箱）分别延迟至新会话后；A 的 notebooks＋ownerless＋audit 三个读延迟至切到 B 后。最终均只显示新账户／B 的数据，访客未出现旧邮箱。它们检验的是这几种迟到读取，不能推导所有异步 UI 副作用都隔离，R3 正是反例。

对原有 N1–N13 的页面／PAT 对等性，用 TypeScript AST 统计所有 `fixtures/assert/*` 导入的断言及本地 helper 的传递调用；13 个故事里 API 版本使用的共同 SQL 断言函数集合均包含在页面版本中（`frontend/e2e-shared-assertions-ast.json`）。N11 包括 `expectOwned`、成员与审计，N13 包括笔记本／审计随工作区删除；N10 页面确实准备 51 条。N2 是纯可见性故事，两侧均无直接 SQL helper。集合包含不等于每个参数、分支、次数都对等，也不能填补 R5 没有调用列表读取的缺口。

| 指定长期条文         | 核对结果与证据范围                                                                       |
| -------------------- | ---------------------------------------------------------------------------------------- |
| 13.1 第 3、4 条      | 基线角色／权限／错误次序正确；R5 是等价测试的种子缺口，不是基线授权错误                  |
| 13.1 第 5、6 条      | 锁总闸、排序、外键、级联与清理符合代码与实测；M4 改成员写锁前提须按现有 handoff 重审     |
| 13.1 第 10–12 条     | 日志隐私、端口与组合根转换、shared 的 Unicode 例外符合实现；架构／日志测试通过           |
| 13.1 第 21 条        | 已有非空注册链、事务边界、错误回滚和分发测试有效；空集合最后一跳尚属未来验收             |
| 13.1 第 27 条        | 严格游标、页边界、参数次序、跨工作区隔离通过实证                                         |
| 13.2 第 1 条         | notebook／ownerless 队列有效；notebook members 的整个资源写遗漏队列，见 R1               |
| 13.2 第 7、10、11 条 | 加载块、守卫、problem 映射符合所查路径；首页提示失败静默是已写明的例外                   |
| 13.2 第 12 条        | 名称／开放程度表单符合；添加成员新草稿被旧答复清空，见 R4                                |
| 13.2 第 15、16 条    | store 与 SWR 按资源 id 分隔；侧栏创建没有随工作区重建，见 R3                             |
| 13.2 第 17 条        | 已测整页离开／新建焦点正确；同名操作的区分不完整，见 R6                                  |
| 13.2 第 18、19 条    | 第一页及已查按 id 写的角色跟随有效；更多页 403 漏跟随，见 R2；审计游标合并／去重测试通过 |
| 13.4 第 3 条         | N1–N13 原有故事通过；页面／PAT 的共同数据库断言及 CLI／清理例外按源码核对                |
| 13.4 第 4、6 条      | 交错有真实锁握手、矩阵经接线 HTTP；不等于所有状态组合都已覆盖，R5 给出了反例             |
| 13.4 第 7 条         | 已明确模拟的有效角色／接管计数／404 修复正确；默认写的模拟边界仍需澄清，见 D1            |

### 7.4 扩展点与交接

逐路径核对 `notebook/app/` 的 create、update、成员用例、`cascade.go`、`extension.go` 与模块根测试；完整 Go 门禁覆盖相应行为，变异 C10–C16 验证了错误回滚与若干断线确实会失败。

| 路径                             | 事件内容、条件和事务边界                                                                                                                                            |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 新建笔记本                       | 写本和首位 admin 后，通知创建者；开放程度非 none 时 `Reached=true`                                                                                                  |
| 改 workspace_access              | 仅跨过 none 时通知 `Reached=true`，此时 `UserIDs` 可以空；改名及 viewer↔editor 不通知                                                                               |
| 添加／恢复笔记本成员、移出、离开 | 成员变化后通知该账户；仅改笔记本角色不通知，因为可见集合未变                                                                                                        |
| 工作区新加入、改角色             | 接受邀请插入新行后有 Added 事件；notebook 注册者只对 admin/member 发可见性。角色事件带 from/to，仅跨 guest 时转为可见性。新建工作区本身不发 Added，当时还没有笔记本 |
| 工作区结束关系                   | 成员结束与设无主后，按涉及工作区各通知一次该账户；移出、离开、停用共用这条链                                                                                        |
| 工作区恢复／归还                 | 有本归还，或者恢复为 admin/member 才通知；guest 且归还 0 本不通知。多本归还仍只通知一次，归还没有另外再发一次                                                       |
| 接管                             | 成员新建／恢复／提升、清无主、记审计后，通知接管者；失败整体回滚                                                                                                    |
| 普通删除、无主删除、工作区删除   | 用删除事件，不另发可见性；包含工作区、笔记本 id 集合、执行者和时刻。工作区删除只在本次删除 id 非空时发一条合并事件；即使为空仍删除审计                              |
| 活动来源                         | 只读、无锁；字节数求和，最后活动从 notebook.updated_at 和各来源时间取最大值；多余 id 忽略，来源失败即返回错误                                                       |

写事件都在当前事务中；多个注册者依注册顺序调用，第一个错误即停。可见性值没有执行者是总设计第 8 节的明确例外；这里没有擅自按 13.1 第 21 条一般句式报不一致。删除改由 M5 同时订阅删除事件、活动来源不是写订阅者，在交接中均有说明。

M4 的删除／活动来源、M5 的可见性、M7 的活动来源 handoff，明确要求首个真实注册者验证整个程序的最后一跳；M5 还明确回调后事务可能回滚，不能在回调里直接关连接。当前空集合做法没有遗漏可立即证实的生产行为，但绝不能把模块根测试当成未来实际 SSE／页面／附件接线的验收。既有 handoff 的这项要求应作为进入相应里程碑的门禁保留。

## 8. 未能验证的部分

- 仅运行本机 Chromium；没有验证 Firefox、Safari、真实中文输入法、窄屏完整布局和读屏软件的实际朗读。R6 有真实浏览器可访问树证据，不声称做过读屏人工测试。
- 没有测大量笔记本／百万级审计、大工作区停用、长期清理积压的性能；103 条同时间审计验证的是正确性。
- M4 页面、M5 SSE、M7 附件尚无真实注册者；模块根替身与变异证明当前扩展契约，不证明未来组件性能、提交后副作用或跨实例推送。
- 确定性交错覆盖的是列出的锁序和请求组合，不构成无死锁的形式化证明。没有对用户开发容器或其他项目施加负载。
- 本轮按要求只交付报告，未在原仓库修复实现、添加回归测试或修改其他文档；报告中的处理建议尚未实施。
