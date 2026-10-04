# M5 多人编辑：收尾审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | `main` 的 `310bea6`（M5 的六个 Phase 全部合并、P6 文档写好之后），M5 的改动是 `4db58bb..310bea6`（59 个提交，311 个文件，+21794/−1036）。对照[文档约定](../../../README.md)的"M 完成"、[M5 总设计](../00-M5-design.md)、六个 Phase 文档与审查记录、收到的六份移交、[输入法清单](../../M4-pages/manual/P6-ime-checklist.md)与[总体设计](../../v0.1-design.md)中涉及 M5 的部分 |
| 审查方式 | 三位独立审查者（Opus）并行（A 后端、B 前端与端到端、C 完成标准与文档），各在自己的 `git archive` 快照上做探针，仓库与别的 Docker 容器都没动过：<br>• **A**：`make lint-go`、`go test -race -count=1 ./...`（58 个包）、耗时预算、`server/tools`、`make gen-check`；page、events、markdown、bootstrap 另跑 `-race -count=3`；交错 42、45–51 与 page 模块根的 `TestTheEditLockRefusesASecondOpening` 另跑 `-race -count=10`；deadcode；任务项扩展对病态输入的耗时；55 项反向对照<br>• **B**：门禁（vitest 跑 6 遍共 1594 个、lint、格式、knip、tsc、构建）；e2e 约 2280 次运行，含 2 个 worker、CPU 降速 4 倍与 6 倍、并行负载，没有环境之外的失败，持续集成的两次失败没能复现；反向对照：vitest 96 项、注册表清空 13 项、e2e 8 项<br>• **C**：14 个整个程序上的 Go 测试；组合根的注册者逐个换成空 7 项；逐条核对完成标准、故事表的每一句、移交与文档 |
| 日期 | 2026-10-04 |
| 结论 | 代码没有必须修的缺陷。处理完移交（C-I1、C-I3）、前端按类型的处理表（B-I1、C-I2）、第 13 节（C-I4）、README（C-I5）与故事的句子和落库断言（B-I2、C-I6）之后，**等负责人执行完输入法清单即可收官**：<br>• 门禁、全部故事、持续集成全绿；M5 的完成标准与 12.5 逐条满足，人工验收一项待执行（见下）；<br>• 规模：Go 生产代码 +2680/−370（66 个文件）、测试 +5060/−280（58 个）、生成物约 +590；前端生产 TS/TSX +3059/−287、测试 +4114/−256；端到端 +1740；接口描述 +725；SQL 33 行；<br>• 没有上帝文件：events 模块生产代码 1069 行、测试 1232 行；M5 最大的 Go 生产文件是 `events/adapter/http/handler.go`（184 行）、`page/app/unit_session.go`（150 行）、`platform/postgres/listener.go`（149 行） |

## 完成标准核对

| 标准 | 结论 | 证据 |
|---|---|---|
| 所有 Phase 完成，各有审查记录 | 满足 | P1–P6 完成，各有审查记录与修复的核对 |
| 本 M 的故事全部通过（本地与持续集成）；之前各 M 的故事仍通过 | 满足 | 本地 `make e2e` 182 个；持续集成的 e2e 任务为绿。收尾之前的两次偶发失败没能复现，嫌疑处已修（B-M6），之后的失败写成运行的注释 |
| 故事表每一行、每个版本都有端到端测试，并做到表里写的每一句 | 满足（修复后） | C1–C10 共 10 个文件。B、C 逐句核对，缺的句子与落库断言见 B-I2、C-I6、C-M4，都已补上 |
| 改写的 M4 测试 | 满足（修复后） | P1 的 PG4、PG10、PG8 与交错 42，P3 的 PG3，P5 的 PG7，P6 的 PG5；第 3 节原来漏了 PG10、PG3、PG5，又写着 P5 改写 PG9、PG10（实际不改，P5 3.9），照实际改写（B-M7） |
| 对等验收：两个版本调用同一组断言；例外 | 满足（修复后） | `e2e/fixtures/assert/collab.ts`（`expectAliveSessions`、`expectContentWritten`、新的 `expectToggleRevision`）与 `page.ts` 的 `expectSessionGone`；例外：C5、C9 与 C6 的退出、关闭、闲置只有页面版本，C4 的相同正文与 C7 的"看不到的人收不到"只有接口版本（C7 这一条收尾时写进第 3 节） |
| 人工验收：输入法清单 | **待执行** | 第 10–15 步，结果记在下面"人工验收"一节；通过之前 M5 不改为已完成 |
| 人工验收：事件流经 Caddy 与 nginx | 满足 | P2 由作者用两个反向代理的容器执行，可用的配置在 README 的部署一节（[M0/P1 移交](../handoffs/M0-P1-sse-proxies.md)） |
| 本 M 建立的扩展点已建好，有测试证明注册者能挂上 | 满足（修复后） | 事件类型：服务端 `events/app` 的 `publisher_test.go`、`hub_test.go` 与整个程序上的 `events_lab_test.go`（`links`），前端的处理表原来没建（B-I1、C-I2），已建并有替身测试；problem 的扩展成员：`lock`、`ended_by` 由契约与 `httpserver` 的测试守住 |
| 本 M 注册的扩展点：经每条触发路径各有整个程序上的行为测试，组合根交空时失败 | 满足（修复后） | `events_pages_test.go`、`events_access_test.go`、`events_deletion_test.go` 等与 e2e；C 把注册者逐个换成空，7 项全部失败；A 的 55 项反向对照 53 项失败，存活的两项（A-M1、A-M2）已补测试 |
| 权限矩阵覆盖每个新操作，用笔记本级的列 | 满足 | `getEditLock`、`releaseEditLock`、`toggleTask` 用 `notebookColumns()`，心跳另有"别人的会话"；`streamEvents` 整模块豁免（按可见性过滤），可见性由 `events_lab_test.go` 按列核对 |
| 12.5：用 PAT 完整操作 | 满足 | `TestEveryOperationAcceptsAPersonalAccessToken` 由契约推导，长连接的流读它的头；C1–C4、C6–C8、C10 的接口版本都用 PAT |
| 12.5：先写描述，再写代码 | 满足 | `api/modules/page.yaml`、`events.yaml`；`gen-check` 无差异 |
| 12.5：架构测试、depguard、前端静态检查 | 满足 | 都为零 |
| 12.5：本 M 没有 `open` 的 handoff | 满足（收尾后，一项随人工验收） | 收到的六份都改为 `done`（C-I1）；M4 的 M0/P1 编辑器一份随清单 |
| 文档约定的"M 完成" | 满足（收尾后，待人工验收） | 六个 Phase 各有审查记录；本记录、第 13 节、M5 总设计、README 在收尾时完成；M5 的状态在人工验收通过之后改为已完成 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| B-I1 / C-I2 / A-M3 | Important | **前端按类型的处理表没建**：总设计 8 与总体设计 12.4 说后面的 M 在前端加一项就能收到自己的类型，实际帧只认 `pages`、`lock`，hub 丢掉不认识的，`event-stream.tsx` 是写死的 `switch`；服务端也没有"别的模块经 `Publisher` 发一个后来的类型、经整个程序到达流"的测试 | 前端：新的 `events/handlers.ts`，`eventHandlers` 是 `[类型, 处理函数]` 的表（`pages`、`lock`），由 `main.tsx` 经 context 给出；不认识的类型成为帧 `other`，带着数据经持有者、频道到达每个标签页，`EventStream` 按类型查表。`hub.test.ts`（`links` 带数据到达每个标签页）、`event-stream.test.tsx`（后来的类型到达它的处理函数）、`frames.test.ts`；`page-lock.test.tsx`、`page-tasks.test.tsx` 经组合根的表。服务端：`events_lab_test.go` 在同一个事务里给每本笔记本再发一个 `links`，每一列收到的 `links` 与 `pages` 都要等于它能读的笔记本。反向对照（表为空、hub 丢掉 `other`、`other` 不带数据、`EventStream` 不查表、hub 丢掉后来的类型）全部失败 |
| C-I1 | Important | 收到的六份移交都还是 `open` | 逐项对照代码改为 `done` 并写明处置；[M4/P6 编辑器](../handoffs/M4-P6-editor.md)的真实输入法部分随清单。M4 的 [M0/P1 编辑器移交](../../M4-pages/handoffs/M0-P1-editor.md)更新处理情况，清单通过之后改为 `done` |
| C-I3 | Important | 写给后面的移交缺几份：事件类型的最后一跳（M6、M10、M11）、P2 审查 B-m3 的工作区级事件、M12 的多实例与时钟、重连与续期的代价、PostgreSQL 大版本升级时 `NOTIFY` 的全局锁；M12 体验移交缺 P5、P6 的几项；M6、M7 的两份移交过时 | 见下"handoff" |
| C-I4 | Important | 总体设计第 15 节说要在收尾时改写的第 13 节各条还没改 | 见下"第 13 节" |
| C-I5 | Important | README 没写任务项的勾选、阅读视图的 `data-task`，前端的页面与编辑两条没有锁、接管、强制解锁、只读的说明、自动保存、闲置、关闭标签页的释放、实时推送 | 照代码补上；e2e 一节写明持续集成的失败是运行的注释（C-N4） |
| B-I2 / C-I6 / C-M4 | Important | **故事表的句子与落库断言不全**：C8 的"被移出工作区"两个版本都没有；C10 两个版本没断言变更集、revision、`content_updated_by` 与哈希，页面版本没有 revision 落后；C4 页面版本不调用落库断言，没有改名、移动照常，没有"编辑时删除笔记本"（4.9 整个没有 e2e）；C3 页面版本没断言编辑者、阅读者看不到"解除锁定"；C7 页面版本的"看不到的人收不到"在第 3 节没列为例外 | C8 两个版本各加被移出工作区（接口：流以 `reset access` 结束，重连之后收不到这个工作区的笔记本的事件；页面：读着其中一页时被移出，工作区显示找不到，其中的东西不再当作找不到去读）；C10 两个版本调 `expectContentWritten` 与新的 `expectToggleRevision`，页面版本加 revision 落后的 409 与提示；C4 页面版本调 `expectAliveSessions`、`expectSessionGone`，B 改名并移到顶层、A 的编辑器跟着；新的"C4（页面，笔记本）"：编辑中、未保存时删除笔记本，编辑器留住文字并说明页面已不在；C3 的编辑者标签页看不到"解除锁定"；C7 写进第 3 节的例外 |
| A-M1 | Minor | 把笔记本对成员关闭（`workspace_access` 改为 `none`）的可见性事件没有整个程序上的测试，反向对照 VIS-update-closing 存活；以管理员身份接受邀请也没有一行 | `events_access_test.go` 加"a notebook closed to the members"与"an invitation accepted as an admin"两行；VIS-update-closing 现在失败 |
| A-M2 | Minor | 过期的会话随页面删除而结束时不告诉订阅者，没有整个程序上的测试（第四次开启先删掉了过期的第三个），反向对照 S-expired-told 存活 | `events_pages_test.go`：让一个会话过期，再删它的页，流只收到树、没有这个会话的 `lock`；S-expired-told 现在失败 |
| B-M1 | Minor | 心跳的 `signal` 没有测试：去掉之后 vitest 全过 | `page.service.test.ts` 加心跳带 `signal`、中止时拒绝；假服务器在中止时拒绝 |
| B-M2 | Minor | 四个常量没钉住：事件重读的合并 5 秒、勾选的期限 60 秒、自动保存 2 秒、闲置 30 分钟 | 各自的测试断言常量（引总设计的节号），四项反向对照失败 |
| B-M3 | Minor | 断言偏弱：C4、PG4、C10 的锁只比 `page_id`；C1 在按"编辑"之后才断言"A 正在编辑"，409 只数控制台；C2 墓碑的结束只看 204；C6 的"已保存"来自闲置之前的自动保存；C9 选举之后 `opened()===1`；C8 退出登录没断言流的错误 | 锁整体比较；C1 在按"编辑"之前断言说明，409 比较整个答复体的 `lock`；C2 断言墓碑被删；C9 只在有 Web Locks 时断言一条，租约时断言接手；C8 两个标签页的 `eventStreamErrors` 为空。C6 闲置自己的保存不改：由组件测试守住（`idle-exit`、`page-edit` 的闲置用例），e2e 的作用是整条链 |
| B-M4 | Minor | 退出登录丢掉 2 秒之内打的字：`endEdits` 直接结束会话 | `PageEditing.close()`：有未保存的修改时先经编辑器的安静保存（`savesThrough`）再结束，仍在 2 秒的上限之内；`page-lock.test.tsx` 断言先 `PUT` 再 `END`。反向对照（退出登录直接结束、`close` 不保存）失败 |
| B-M5 | Minor | 强制解锁成功之后焦点落到 `body` | 确认框的 `focusAfter` 交给外壳：能编辑时"编辑"，否则标题；`edit-lock-note.test.tsx` 断言。反向对照失败 |
| B-M6 | Minor | 持续集成偶发失败的嫌疑：C9 开十个标签页、默认 30 秒；租约选举在慢机器上两边都持有；C10 的焦点依赖答复的先后（事件的重读先到时丢焦点）；PG8、PG10 假设流在会话过期之前已连上；几处窗口偏紧 | C9 `test.slow()`，接手的等待放宽，租约时断言 `opened()` 增加而不是恰好加一；`reading-view.tsx` 在发送之前记下勾的项、失败时清掉（新测试：事件的重读先于勾选的答复）；PG8、PG10 照 C7 先扣住事件流，第一次读答复之后放行，等连上时的重读再编辑（先写的"等读两次"不扣住事件流，在合并提交的持续集成上失败一次，见"核实修复"）；`expect.timeout` 10 秒，C8 令牌的等待 20 秒；服务端的夹具等日志"notification listener listening"（A-Q3）。持续集成改用 github 报告器之后各次运行都绿 |
| B-M7 / C-M5 | Minor | 第 3 节没写 P1 改的 PG10、P3 的 PG3、P6 的 PG5；"P5 改写 PG9、PG10"没做；Phase 文档几处与代码不符 | 照代码改写（见下"文档与代码的不一致"） |
| B-M8 / C-M3 | Minor | 输入法清单缺锁的推送一步、Safari 关闭标签页的释放、被冻结的持有者；结果写在哪两处不一致；第 10 步要先离开"IME 二"，"没有 PUT"要说明 | 第 10–15 步：第 13 步经 `curl` 推一个锁，第 14 步关闭标签页的释放，第 15 步被冻结的持有者；结果第 1–9 步记在 M4/P6 的记录，第 10–15 步记在本记录；第 10 步先离开"IME 二"、写明为什么没有 `PUT` |
| B-M9 | Minor | M12 的体验移交缺几项 | 加第 5、6 项，第 3 项改写 |
| B-M10 | Minor | 13.2 的例外与改写 | 见下"第 13 节" |
| C-M1 | Minor | 总体设计 3.9、6.1、12.2、12.6、第 14 节、8.4、8.5、第 15 节没跟上 M5 | 见下"第 13 节" |
| C-M2 | Minor | M5 总设计几处与代码不符：`events.stream` 不是动作；`max(expires_in, 5)`；`NewEditLock` 的参数；`Notify` 带频道；`API_MODULES` 不含 events | 照代码改写 |
| A-N1 | Nit | `events.Module.Streams()` 没有调用者 | 删掉 |
| A-N2 | Nit | `Publish` 不核对类型：`hello`、`reset`、空的、带冒号或换行的类型要到写帧时才丢，`hello`、`reset` 甚至会发出去，与流自己的帧混淆 | `domain.CheckType`，`Encode` 与写帧都调用：这样的类型 `Publish` 答 `ErrNoType`，整个写入单元回滚；`TestAnEventOfNoTypeHasNoPayload`。反向对照（任何类型都编码、保留的类型放行）失败 |
| A-N3 | Nit | 总设计第 5 节把 `events.stream` 写成动作 | 删掉 |
| A-N4 | Nit | 总体设计 6.1 没写墓碑与 `ended_by` | 见下"第 13 节" |
| A-N5 | Nit | P2 审查 B-m3（工作区级的事件到不了一部分人）只在 P2 文档里 | 写进 M6 的事件移交第 4 项 |
| A-N6 | Nit | 总设计第 8 节的表没有强制解锁（不经守卫、不发 `pages`）；`events_stream_test.go` 的注释引 4.11，应为 4.10 | 照代码改写 |
| B-N1 / C-M2 | Nit | `expires_in` 的写法 | 照代码改写 |
| B-N2 | Nit | `edit-session.ts` 手写的 `LockHolder` 与生成的 `EditLockHolder` 重复 | `page.service.ts` 导出生成的类型，`edit-session.ts` 用它 |
| B-N3 | Nit | `.oxlintrc.json` 的 `!../session/testing/**` 让生产的 events 代码也能导入测试替身 | 不改：要把整个目录规则块复制一份成测试专用的覆盖，重复比风险大；替身的导入由 knip 与代码审查兜住 |
| B-N4 | Nit | `refused()`、`pause()` 各有两份；`leadership.ts` 的 `either()` 每轮给长寿的 `signal` 加一个监听、从不移除 | 监听的泄漏修掉：`either()` 返回 `{signal, release}`，每轮在 `finally` 里释放；两处重复不合并（各在自己的模块里，合并要加跨模块的依赖） |
| B-N5 / B-N6 | Nit | `page-tree.store` 混着按页的视图、勾选、锁与释放；`page-edit.tsx` 六个 effect；`main.tsx` 的接线只有 e2e 覆盖 | 接受：拆分留给 M6 加链接状态时一起看；`main.tsx` 的表由 `page-lock.test.tsx` 等经 `eventHandlers` 覆盖 |
| C-N1 | Nit | 状态的写法"完成"与"已完成" | M5 的文档一律"完成"，与之前各 M 的 Phase 文档一致 |
| C-N2 | Nit | `page/domain/session.go` 的注释指向 `stores/page-editing.ts`，常量已在 `stores/edit-session.ts` | 改正 |
| C-N3 | Nit | 合并提交的说明与 `docs/README.md` 的格式 | 记下：合并说明照 Phase 的惯例，不改历史 |
| C-N4 | Nit | README 没写 e2e 的 github 报告器；9.4 说事件流的读不受换代检查保护 | README 写明；9.4 见下"第 13 节" |

## 疑问与判断

| # | 疑问 | 判断 |
|---|---|---|
| A-Q1 | 每个账户能开多少条流没有上限：一个令牌能先开一批、之后每秒再开一些，每条占一个连接、一个 goroutine 与 64 个事件的缓冲，每个心跳重新认证一次 | v0.1 不加（负责人可改判）：团队规模下按凭证的限流够用，加上限要定超出时的行为（结束最早的一条，还是答 429）。写进 [M12 的移交](../../M12-release/handoffs/M5-performance.md)第 6 项，压测时一并看 |
| A-Q2 | 闲置 30 分钟只在前端：PAT 或 MCP 一直心跳就一直持锁，只有管理员能强制解锁；令牌的接管让本人浏览器里的编辑器悄悄变成只读 | v0.1 不加服务端的闲置上限（负责人可改判）：负责人的决定 1、3 是对编辑器说的，令牌的客户端没有"闲置"可言。写进 [M9 的移交](../../M9-mcp/handoffs/M5-locks.md)第 5 项，由 MCP 的设计决定要不要"长时间没有写入就过期" |
| A-Q3 | `/readyz` 不看 Listener，e2e 的夹具只等 `/readyz`：流在 LISTEN 之前打开会答 503 `not_ready`，控制台多一条没声明的错误，可能是持续集成偶发失败的来源 | 夹具改为等日志"notification listener listening"（`e2e/fixtures/server.ts` 的 `waitForLog`）；`/readyz` 不改（README 写明：流在 Listener 就绪之前答 503 并带 `Retry-After`，客户端照它重试） |
| B-Q1 | 被移出笔记本时，心跳答 404、重开答 404，编辑器说"页面已不在"，而不是"没有权限了" | 有意如此：服务端对看不到的东西不区分"不存在"与"无权"，前端无从分辨。写进 M5 总设计 4.9 |
| C-Q1 | M9 的单元合并移交说前后都空的 `Change`：`pageEvents` 与锁的守卫怎样处理 | 都没有特别处理：从改动集里去掉这个节点时二者不见它，不去掉时 `pageEvents` 把它发成一页。写进 [M9 的移交](../../M9-mcp/handoffs/M4-P2-unit-merge.md) |
| C-Q2 | M11 的冻结与 M5 的锁都拒绝时答哪一个 | 登记在前的那个：几个守卫（或会话的否决者）都拒绝时，答复的是组合根 `pageRegistrants(pool)` 里登记在前的那个的码。写进总体设计 12.4 的写入守卫与编辑会话两行，由 M11 在设计里写明冻结排在锁之前还是之后 |

## 第 13 节

由一位 Opus 起草、作者逐条对照代码核对，写进[总体设计](../../v0.1-design.md)第 13 节（第 15 节记一行修订）：

- **13.1 后端**：第 1 条加接管与强制解锁（经单元的锁与判定、不记变更集、不调守卫、不发页面事件）与已经是那个状态的勾选；第 3 条点名 `TestTheStreamSeesWhatEachReadAllows`；第 4 条写墓碑的码（本人的答 `taken_over`、`unlocked`，后者带 `ended_by`；心跳先照活着的会话判定笔记本；结束答 204 并删行；别人的照旧）、`page.locked` 带 `lock`，以及 `toggleTask`、`releaseEditLock`、`streamEvents` 的码的次序；第 5 条写正文行锁下的次序（先删过期的行，接管与强制解锁改成墓碑，再调否决者）、心跳在会话行锁上等过之后重判、守卫不加锁地读、`ended_by_id` 的外键只取 `FOR KEY SHARE`、`NOTIFY` 的全局队列锁在提交时、不进加锁的次序，交错 45–51；第 6 条加墓碑；第 8 条加 problem 的扩展成员；第 11 条加 M5 的模块入口；第 14 条加 `API.LongLived` 的次序与流的处理器；第 15 条改正心跳常量的位置（`stores/edit-session.ts`）、租约 120 秒，写明 `events.heartbeat_interval` 经 `hello` 下发与只在前端的常量；第 21 条写会话的订阅者另得到开启、结束的原因、过期与墓碑不告诉、M5 的两个构造例外、登记的次序决定答谁的码、M5 注册与建立的扩展点的测试；第 23 条写 Listener 的启动与停机的次序、定时任务也删过期的墓碑；新增第 29 条"编辑锁"、第 30 条"实时推送"（发布、载荷与退化、几条与合并、类型、过滤、连接的建立、工作区级的类型、不补发）。
- **13.2 前端**：第 1 条写不经 `oneAtATime` 的三处（勾选、编辑会话、强制解锁）、换代核对管不到的两处（事件流由 hub 中止、关闭标签页时的释放）、退出登录先保存再结束；第 6 条写 `events/` 的依赖方向；第 7 条改阅读视图的键为 `["page-view", 笔记本 id, 页 id]`、锁的说明、失锁的码，新增"SWR 之外的重试"；第 9 条写事件流的 4xx 记在 `eventStreamErrors`；第 15、16 条加锁的键、`RootStore.editPage` 与 `edits`、每代一个 hub、外壳在有未保存的编辑时留住、连上时由外向内重读；第 17 条写 M5 的焦点；第 20、21 条写失锁时的快捷键与提醒；第 23 条写三个注册表与上下文的新字段。
- **13.3 Markdown**：第 4、5 条写任务项的 `input` 与 `data-task`、扩展在 `platform/markdown/tasks`。
- **13.4 测试**：第 3 条加事件流与同一账户第二个标签页的夹具、等 Listener 生效、`expect` 默认 10 秒、`collab.ts`；第 4 条加交错 45–51（49 的做法）与 `checkPages` 的一行；第 5 条加 `x-long-lived` 与 `settle`；第 6 条加 M5 的三行与 events 的豁免。
- 同时改了 2.3（事件流的锁、租约与频道）、3.9（失锁的编辑器只读、横幅与"回到阅读"）、6.1 与 7.2（墓碑、`ended_by`）、8.4（停机次序、`/readyz` 不等 Listener）、8.5（定时任务删墓碑）、9.4（事件流的换代）、12.2 的 M5 一行、12.4（前端的处理表、工作区级的类型、几个守卫都拒绝时答谁的码，C-Q2）、12.6（"进行中（待负责人执行输入法清单）"）与第 14 节（SSE 与反向代理、输入法两行改写，新增 PostgreSQL 大版本升级时复核 `NOTIFY` 的全局锁）。

核对时按代码改正了起草的两处：发布端口原写"不核对类型、写帧时才丢"，修复之后 `Publish` 答 `ErrNoType` 并回滚（A-N2）；`NOTIFY` 全局锁的论证原说在 P2 文档里，实际在 M5 总设计 4.10，总设计第 10 节与 M12 的移交一并改正。`events.Module.Streams()` 已删（A-N1），不写进第 11 条。

## handoff

- **收到的**：[M0/P1 事件流与反向代理](../handoffs/M0-P1-sse-proxies.md)、[M3/P2 可见性变化](../handoffs/M3-P2-visibility.md)、[M4/P1 树的重读](../handoffs/M4-P1-tree-refresh.md)、[M4/P3 Markdown 扩展](../handoffs/M4-P3-markdown-extensions.md)、[M4/P4 编辑会话](../handoffs/M4-P4-edit-sessions.md)逐项对照代码都已落实，改为 `done` 并写明处置；[M4/P6 编辑器](../handoffs/M4-P6-editor.md)改为 `done`，真实输入法的部分是清单的第 10–15 步，随 M4 的 [M0/P1 编辑器移交](../../M4-pages/handoffs/M0-P1-editor.md)跟踪，清单通过之后那一份改为 `done`。
- **M5→M6**：新的[事件类型](../../M6-links/handoffs/M5-events.md)（服务端经 `Publisher` 发布、前端在 `eventHandlers` 加一项、两侧最后一跳的测试、工作区级的事件到不了一部分人，即 P2 审查 B-m3）；[编辑锁与链接改写](../../M6-links/handoffs/M5-locks.md)加第 4 项（选"守卫放行改写"时清单加"组合中收到正文的外部更新"）；[Markdown 扩展](../../M6-links/handoffs/M4-P3-markdown-extensions.md)第 7 项与 `renderApp` 的说法按 M5 的现状改写。
- **M5→M10、M11**：[M10](../../M10-llm-wiki/handoffs/M5-events.md)、[M11](../../M11-upgrade/handoffs/M5-events.md) 的笔记本模式各一份，指向 M6 的那份。
- **M5→M7**：[附件的扩展](../../M7-assets-transfer/handoffs/M4-extensions.md)里 `renderApp` 的说法改写。
- **M5→M9**：[编辑锁](../../M9-mcp/handoffs/M5-locks.md)加第 5 项（服务端没有闲置的上限，A-Q2）；[单元合并](../../M9-mcp/handoffs/M4-P2-unit-merge.md)写明 M5 对没有改动的变更的处理（C-Q1）。
- **M5→M12**：新的[部署与性能](../../M12-release/handoffs/M5-performance.md)（多实例各自 LISTEN、时钟偏差、重连与续期的代价、自动保存与树写、PostgreSQL 大版本升级时复核 `NOTIFY` 的全局锁、每个账户的流没有上限，A-Q1）；[体验的打磨](../../M12-release/handoffs/M5-polish.md)加第 5、6 项，第 3 项改写（B-M9、C-I3）。

`M5-collab-editing/handoffs/` 没有 `open` 的了。

## 文档与代码的不一致

照代码改写（C-M2、C-M5、B-M7、A-N3、A-N6、C-N2、修复核对 NT-4–NT-6）：

- **[M5 总设计](../00-M5-design.md)**：第 3 节写明 P1 改的 PG10、P3 的 PG3、P6 的 PG5，PG9、PG10 不因自动保存改写，C7 的"看不到的人收不到"列为例外，人工验收的结果分记两处、第 10–15 步的内容；`NewEditLock(pool, names)`、`expires_in`、`Notify(ctx, channel, payload)`、`API_MODULES` 不含 events；第 5 节去掉 `events.stream` 这个动作；4.9 写明失去访问时编辑器说"页面已不在"（B-Q1）；第 8 节的表加上强制解锁（不经守卫、不发 `pages`）；第 10 节 `NOTIFY` 全局锁的论证指向 4.10。
- **Phase 文档**：P1 的 `pagehide` 释放是 P4 的；P3 的 `FrameParser.push`、帧 `other` 带数据、`run(lead)` 与 `#lead`、`subscribe` 的 `other`；P4 的 `collab.ts`；P5 只改 PG7 与 C4；P6 的文件清单；P5、P6 的"给 M5 收尾"写明清单扩为第 10–15 步。
- **[输入法清单](../../M4-pages/manual/P6-ime-checklist.md)**：第 10 步先离开"IME 二"、说明为什么没有 `PUT`；新增第 13 步（组合中收到锁的推送）、第 14 步（关闭标签页时的释放）、第 15 步（被冻结的持有者，1–1.5 分钟）；结果分记两处；M6 的那一步写明随 M6 的选择（B-M8、C-M3）。
- **README**（随代码提交）：任务项的勾选、阅读视图的 `data-task`，前端的页面与编辑两条（锁、接管、强制解锁、失锁的只读与横幅、自动保存 2 秒、闲置 30 分钟、关闭标签页的释放、一个浏览器一条流的实时推送、勾选），e2e 一节的 github 报告器（C-I5、C-N4）。
- **代码注释**：`page/domain/session.go` 指向 `stores/edit-session.ts`（C-N2）；`events_stream_test.go` 的节号 4.10（A-N6）；`events/app/publisher.go` 的 `Publish` 写明类型的限制。

## 反向对照

审查者做的：
- **A**：55 项中 53 项失败；存活的两项（VIS-update-closing、S-expired-told）见 A-M1、A-M2。
- **B**：vitest 96 项中 88 项失败，存活的写成了发现（B-M1、B-M2 等）；注册表清空 13 项（11 项由 vitest 测出，`main.tsx` 的 2 项只由 e2e 测出）；e2e 8 项全部失败。
- **C**：组合根的注册者逐个换成空，7 项全部失败。

作者在修复中做的（全部按预期失败，改动全部还原）：
- **Go 5 项**：笔记本对成员关闭不发可见性事件（VIS-update-closing）、过期会话的结束告诉订阅者（S-expired-told）、任何类型都编码、保留的类型放行、hub 丢掉不认识的类型（`events_lab_test.go` 失败）。
- **前端 13 项**：处理表为空、hub 丢掉 `other`、`other` 不带数据、`EventStream` 不查表；退出登录直接结束、`close()` 不保存；解锁之后不交焦点；勾选的答复之后才记下勾的项；心跳不带 `signal`；事件重读的合并改为 3 秒、勾选的期限改为 2 分钟、自动保存改为 3 秒、闲置改为 31 分钟。
- **e2e**：改过的 8 个故事（C2、C3、C4、C8、C9、C10、PG8、PG10）各重复 3 遍，78 个全部通过。

## 核实修复

- 修复在分支 `m5-closeout`：`8b28da2`（审查的修复）、`b6b24b3`（第 13 节与总体设计）、`7f94016`（修复核对的意见），合并 `6fdbf8f`。每个提交之后本地 `make check`（最后一次 vitest 1605 个）、`make gen-check`、全套 e2e（182 个）为绿，上文的反向对照都按预期失败，改动全部还原。
- 合并提交 `6fdbf8f` 上 `make image-smoke` 为绿（`41bd439` 只改 e2e，镜像的代码相同）。
- 持续集成：`8b28da2`（run 37169167171）与 `7f94016`（run 37171211158）的四个任务为绿。合并提交 `6fdbf8f`（run 37171684102）的 e2e 任务失败一次：PG8 等阅读视图的第二次读（连上时的刷新），而事件流在阅读视图挂上之前就连上时，刷新没有东西可重读，等不到。这个等待是收尾修复照 B-M6 加的，C10 的同一做法在修复核对之后已在本地撞到并改掉，PG8、PG10 漏了。`a25e1ec` 照 C7 先扣住事件流，第一次读答复之后再放行（`holdStream`），PG8、PG10 各 20 遍、全套 182 个通过；合并 `41bd439`（run 37172986650）四个任务都为绿。

修复的差异（`8b28da2`）另经一位独立审查者核对：在快照的副本上跑全套门禁（vitest 1602 个，Go 58 个包），前端反向对照 15 项中 14 项失败、服务端 7 项全部失败，自写探针两个，e2e 改动的 11 个文件各跑 1 遍、`--repeat-each=10 --workers=8` 330 个、页面版本 `--repeat-each=20` 320 个、全套 182 个都通过（另有一轮 16 个 worker 因 PostgreSQL 的连接数用尽失败，是环境问题）。没有 Major，Minor 三项、Nit 六项、疑问两项，处理在 `7f94016`：

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| MN-1 | Minor | 退出登录时保存超过 2 秒，`close()` 还在等它，登出之后结束会话的请求被换代拦下，锁滞留到租约结束（最多 2 分钟）；修复之前 `end()` 一开始就发出 | 保存只等 1.5 秒（`close(within)`，`endEdits` 交 `signOutSave`），到时就结束会话，结束的请求在 2 秒的上限之内、令牌仍有效时发出；`page-lock.test.tsx` 加"保存不答复时，1.5 秒结束会话，先于登出"，结束与登出记在同一个序列里断言先后。反向对照（等整个保存、把期限放到 2 秒）失败；后一项第一次存活：测试原来没比较两者的先后 |
| MN-2 | Minor | 勾选在途时别人的写先被重读（这一项没翻转），提前记下的 `toggled` 让焦点不恢复并被清掉，之后勾选答 409，焦点留在 `body`：修复解决了反方向的竞争，带来这个回归 | `toggled` 记下位置与要的状态，从发出到某一次阅读视图显示了它、或勾选失败为止；其间那一项取原来的状态或要的状态都算同一项。`page-tasks.test.tsx` 加"别人的写先到、再答 409，焦点留在原项"。反向对照（只认翻转之后的状态、只认原来的状态、答复之后才记下）失败 |
| MN-3 | Minor | C10 页面版本挂住 B 的阅读视图之前没等 B 的流连上并整体刷新：那次读若在挂住之前发出、在 A 的写之后处理，B 拿到 revision 4，勾选答 200 | 挂住阅读视图的 route 在 B 打开页面之前装上，先放行；开始挂住时，等已放行的读（含连上时的刷新）都已答复，A 才写。先照 PG8、PG10 等"读了两次"，全套里失败一次：B 的流若在阅读视图开始读之前连上，刷新没有东西可重读，只读一次 |
| NT-1 | Nit | e2e 等日志的函数插在 `waitFor` 的注释与函数之间；失败时仍说"没答 /readyz"；被信号结束时等满期限；服务端先写日志、后 `OnListening(true)` | 挪到注释之前；报错写"was not ready/live"；`signalCode` 也算退出；`listener.go` 先 `OnListening(true)` 再写日志 |
| NT-2 | Nit | C8 退出登录"不再打开"只在流数变成 0 的那一刻检查 | 记下 `opened()`，等过第一次退避（1.5 秒）再断言没变、错误为空 |
| NT-3 | Nit | `either().release` 没有测试：不移除监听时全部测试照样通过 | `leadership.test.ts` 加"多轮之后标签页的 signal 上不留监听"；反向对照失败 |
| NT-4 | Nit | M6 的移交引 `ErrTooLong`、`MaxPayload`，模块根没有导出它们；没写类型的限制 | 写明类型的限制（`ErrNoType`）、不要用 M5 的类型名，以及要用这些名字时先在模块根导出 |
| NT-5 | Nit | P3 文档的 `subscribe` 没有 `other`；P6 的文件清单读起来像 `internal/harden/render.go` | 照代码改写 |
| NT-6 | Nit | 清单第 15 步的 `$PAGE` 是"IME 二"的 id；"约 1 分钟"与实现不符 | 写出改名"IME 一"的命令；时间写 1–1.5 分钟（3 个心跳的沉默，每 20 秒查一次，加上连接） |
| Q-1 | 疑问 | `close()` 的保存不等输入法的组合结束 | 不改：退出登录只由本标签页的菜单发起，点到它时焦点已离开编辑器，浏览器随之确认组合；别的标签页退出登录时本标签页换代，不经 `endEdits` |
| Q-2 | 疑问 | `CheckType` 只保留 `hello`、`reset`，后来的模块用 `access`、`pages` 等会被当成 M5 的类型 | 靠约定，写进 M6 的移交第 1 项（NT-4）；发布端口不按"属于哪个模块"区分类型，加这一层要一张类型的登记表，现在没有第二个发布者 |

## 没能验证的风险

- 只用 macOS 上的 headless Chromium：Safari 在 `pagehide` 里带 `keepalive` 的释放、Safari 与 Firefox 冻结后台标签页时事件流的接手、真实输入法，由清单的第 10–15 步人工覆盖；窄屏与读屏的实际朗读没有验证。
- 多实例、实例之间的时钟偏差、`reached` 与令牌到期引起的整体重连与刷新的代价、每个账户的流数，只做了推理或单实例实测，见 [M12 的移交](../../M12-release/handoffs/M5-performance.md)。
- "一个事务几条 `NOTIFY` 只取一次全局锁"依据 PG 13–18 的源码，测不出取锁的次数；升级 PostgreSQL 大版本时复核。
- 持续集成在收尾之前两次偶发失败，本地约 2280 次运行（审查者 B）与修复核对的 650 次运行都没能复现；嫌疑处已修（B-M6），之后的失败会写成运行的注释，从注释读。
- 审查期间别的会话同时在跑测试容器；修复核对的一轮 16 个 worker 用尽了 PostgreSQL 的连接数，是环境问题，8 个 worker 稳定。

## 人工验收

[输入法清单](../../M4-pages/manual/P6-ime-checklist.md)的第 10–15 步，由负责人在 Chromium、Safari、Firefox 上执行；第 1–9 步的结果记在 [M4/P6 审查记录](../../M4-pages/reviews/P6-source-editor-review.md)的"人工验收"一节。通过之后 M4 与 M5 一起改为已完成。**待执行。**

| 步 | 内容 | Chromium | Safari | Firefox |
|---|---|---|---|---|
| 10 | 组合中停顿不自动保存 | 待执行 | 待执行 | 待执行 |
| 11 | 确认之后随即补存 | 待执行 | 待执行 | 待执行 |
| 12 | 组合中收到树的推送（改名） | 待执行 | 待执行 | 待执行 |
| 13 | 组合中收到锁的推送 | 待执行 | 待执行 | 待执行 |
| 14 | 关闭标签页时的释放 | 待执行 | 待执行 | 待执行 |
| 15 | 被冻结的持有者，别的标签页接手 | 待执行 | 待执行 | 待执行 |
