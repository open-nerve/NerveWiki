# M0/P3 服务端平台层：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m0-p3-server-platform`（`9d29741..99d248f`）对照 [03-P3-server-platform.md](../03-P3-server-platform.md)、各 Step 计划与 [M0 总设计](../00-M0-design.md) |
| 审查方式 | 独立审查者在仓库副本上实测：跑全部门禁；`-race -count=8` 查不稳定的测试；以构建出的二进制在隔离的数据库上做第 6 节的冒烟检查；对照 Go 1.27.1 与 go-sdk 的源码核实行为；重跑第 5 节的反向对照，并自拟 8 项；两处缺陷各用探针复现 |
| 日期 | 2026-09-30 |
| 结论 | 分层干净，没有上帝文件，M0 对 P3 的要求都已满足，反向对照有效。2 项 Important、7 项 Minor、9 项 Nit，全部处理；6 处实施时的偏离经审查确认合理 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| I1 | Important | 中间件的状态记录器只看得到 `Write` 与 `WriteHeader`，`ResponseController.Flush()` 绕过它提交了响应。事件流先 flush、再 panic 时，恢复中间件以为响应还没开始，在已发出的 200 后面追加一段 500 problem，连接正常结束，访问日志记为 500。go-sdk 的 MCP 流与常见的 SSE 写法都这样 flush | 记录器实现 `FlushError`（`ResponseController` 调用它）与 `Flush`（断言 `http.Flusher` 的库），flush 与写入一样标记响应已开始。新增测试：两种 flush 方式之后 panic，都中断连接、不追加内容。反向对照：去掉标记即复现审查者描述的输出 |
| I2 | Important | 语句执行中请求期限到期或客户端断开时，pgx 关闭连接，随后的 ROLLBACK 因连接已关闭而失败，`WithinTx` 返回"回滚失败"，调用方用 `errors.Is` 认不出期限到期。P4 起每个接口操作都有期限，这类请求会被当成基础设施故障记成 500 | ROLLBACK 失败且调用方的 context 已结束时，返回的错误包装 context 的错误（以及 ROLLBACK 的错误），fn 的错误仍只保留文字；服务端会自己回滚被放弃的事务。新增测试：`pg_sleep(5)` 配 200 ms 期限，结果匹配 `context.DeadlineExceeded`、数据已回滚、连接池可用 |
| M1 | Minor | 文档没有跟上实施时的偏离：3.4 与 S3 计划仍写解除读写期限；3.3 缺 `datlocale`，"惰性连接、在 `/readyz` 首次暴露"已不成立；3.6 与 S5 仍说 `buildinfo.version` 要标 `//nolint`；`config.yaml`、`pool.go`、`.golangci.yml` 的注释同样过时 | 全部改正；P3 文档第 7 节记录 6 处偏离；S3、S5 计划注明实施时的修正；总体设计 7.1、3.11 同步 |
| M2 | Minor | 启动时对数据库的等待没有上限：丢包的主机会让 `serve` 卡到操作系统放弃 TCP 连接（一两分钟）；`auto_migrate` 关闭且数据库不可达时拒绝启动，没有测试；README 没说 `serve` 需要能连上数据库 | 组合根新增 `awaitDatabase`：`serve` 与 `migrate` 命令启动时最多等 10 秒。新增测试：只接受 TCP、从不应答的服务器，200 ms 后报"database unreachable"；连接被拒时拒绝启动且没有监听。README 补一句。反向对照：去掉等待，测试挂到超时 |
| M3 | Minor | "第二次信号立即退出"没有测试，删掉对应代码所有测试照样通过 | 抽出 `signalContext`，新增子进程测试：第一次 SIGINT 后子进程的停机挂住，第二次 SIGINT 让它被信号终止。测试最初偶发失败，查出实现本身的竞态：`context.AfterFunc` 在另一个 goroutine 里恢复默认处理，context 已取消时可能还没恢复，紧接着的第二个信号会被吞掉。改为先 `signal.Stop` 再取消 context，连续 20 次通过；反向对照：去掉 `signal.Stop` 即失败 |
| M4 | Minor | M0 总设计要求"按路由豁免请求期限，两种行为都有测试"，而请求期限中间件已移到 P4，这条要求没有交接 | M0 总设计的要求表新增 P4 一行：逐路由中间件不包裹 `LongLived` 路由，用测试固定 |
| M5 | Minor | 第 5 节列了"就绪检查的超时"，测试只断言检查带有期限 | 新增测试：挂住的检查在 2 秒预算用完时返回 503 |
| M6 | Minor | goose 的 Provider 没有会话锁，两个进程同时 `migrate up`，或多个实例同时带 `auto_migrate` 启动时互相竞争，输的一方启动失败 | Provider 加 PostgreSQL 会话级咨询锁，每秒重试、最多等 5 分钟。新增测试：库已在版本 1，两个进程同时执行耗时 0.5 秒的第 2 条迁移，都成功、只应用一次。第一版测试在空库上重叠不起来（goose 在第一条迁移的事务里建版本表，天然串行），去掉锁也通过；改为先迁移一条后，去掉锁即报 `duplicate key`，证明测试有效 |
| M7 | Minor | 迁移器与命令里"没有迁移"的分支和两条测试是 Nerve 早期的历史，本项目的迁移集合不可能为空 | 空集合改为 `NewMigrator` 报错，删掉 5 处判空、`status` 的对应分支和两条测试 |
| N1 | Nit | `clocktest` 没有使用者，而且不是并发安全的，它将来的用法是 HTTP 集成测试 | 删掉，写进 [M1 的移交](../../M1-auth/handoffs/M0-P3-platform.md)：引入时加锁并加进规则 8 |
| N2 | Nit | `addr_file` 不是机密，却在日志和错误里隐去路径，"permission denied" 无从排查；`errors.Unwrap` 遇到非 `PathError` 会打印 `%!w(<nil>)` | 日志记录路径，错误原样包装 `PathError`；指向机密的 `*_file` 键怎样记录，写进 M1 的移交 |
| N3 | Nit | JSON 日志中的时长是纳秒数 | 配置日志里的时长统一输出 `"5s"` 这样的字符串，并有 JSON 测试 |
| N4 | Nit | 命令行直接导入 `configs`、`platform/config`、`buildinfo`，与 3.2 的依赖图不符 | 依赖图补上这条边（命令行负责配置从哪里来、打印版本） |
| N5 | Nit | 迁移的上下行测试写死了迁移条数和"没有表"，每加一条迁移都要改它 | 条数取自迁移目录；断言改为"全部回滚后 schema 为空、再次迁移后与第一次相同" |
| N6 | Nit | 命令行测试先监听再关闭来挑端口，可能与别的进程竞争 | 改用 `127.0.0.1:0` 加 `addr_file` |
| N7 | Nit | README 没说改 `NWIKI_DEV_DB_PORT` 要同时覆盖 `database.url`，也没说个人覆盖文件相对于工作目录 | 补上 |
| N8 | Nit | 一处注释折行错乱 | 重排 |
| N9 | Nit | 自检失败时说"用这条命令创建它"，而这个库已经存在 | 改为"库的这些设置无法修改：导出、重建、导入" |

审查者对 6 处偏离的判断：

1. `LongLived` 只解除写期限：合理。Go 1.27.1 的 `startBackgroundRead` 在开始后台读之前清除读期限，请求没有请求体时立即开始，有请求体时在读到 EOF 时开始；解除读期限只会去掉对请求体的约束。补充说明：处理器不把请求体读完，就收不到客户端断开的通知（已写进 `LongLived` 的注释）。
2. 自检加 `datlocale = 'C.UTF-8'`：合理，这正是总体设计 7.1 的字面要求，`datlocale` 是唯一记录 builtin locale 的列。
3. `clocktest`：可以接受但理由弱，见 N1，已删除。
4. `buildinfo.version` 不需要 `//nolint`：合理，改名后 linter 即报错，名字 `version` 在它的内置放行清单里。
5. 相对 Nerve 删掉的部分：合理，各自的使用者都不在 M0。
6. 启动时连不上数据库就拒绝启动：方向合理，缺的上限与测试见 M2。

跨 M 的移交：[M1-auth/handoffs/M0-P3-platform.md](../../M1-auth/handoffs/M0-P3-platform.md)。其中各项都属于总体设计中 M1"以 Nerve 为起点拷贝"的范围，不是扩展点表或长期约定的遗漏，不需要另外修订总体设计。

## 核实修复

- 本地 `make check` 为绿；修复后的持续集成（run 36690330653）为绿；`-race -count=3` 重复跑 httpserver、postgres、bootstrap、命令行四个包为绿，信号测试单独连续 20 次为绿。
- 每项修复的反向对照都按预期失败：flush 不标记响应（I1）、不保留 context 错误（I2）、去掉启动等待（M2）、去掉 `signal.Stop`（M3）、去掉迁移锁（M6）。
