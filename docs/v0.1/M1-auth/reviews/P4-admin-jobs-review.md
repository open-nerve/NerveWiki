# M1/P4 管理命令与后台任务：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m1-p4-admin-jobs`（`3550a52..0678c93`，S1–S6），对照 [04-P4-admin-jobs.md](../04-P4-admin-jobs.md)、各 Step 计划、[M1 总设计](../00-M1-design.md)与 P1–P3 的审查记录 |
| 审查方式 | 独立审查者在仓库的克隆上实测：<br>• 门禁：`make gen-check`、`make check`、`make e2e`；`--repeat-each 5`（150 次）与 `--workers 1 --repeat-each 3`（90 次）全部通过；`make image-smoke`（非默认版本，distroless 镜像里由标准输入 `users create`，建好的账户能登录）；jobs、identity、bootstrap、`cmd/nervewiki`、archtest 各 `go test -race -count=5`；<br>• 与 Nerve 逐文件 diff：`platform/jobs` 只去掉启动的重试（C1 逐字保留）；River 迁移逐字相同，核对由 README 的 SHA-256 换成与 `rivermigrate` 内嵌 SQL 的逐字比较；管理用例、读取密码、命令的组合、授权文件的差异逐项说明（停用、启用只在状态变化时生效，Ctrl-C 恢复终端，注册者一处组合，登录在锁下核对邮箱）；<br>• 反向对照 60 个变异：设计第 5 节的 11 项与两个变体全部被抓住；自拟 46 项中 10 项没被抓住（见发现）；<br>• 活体探针：自起 PostgreSQL 18.6 与 `nervewiki serve`，12 轮混合并发（重置、改邮箱、登录、创建 PAT、改密码）与 45 轮单项并发，以运行时角色再跑 24 轮，共 1,032 个请求没有 5xx，不变量全部成立；缺少各项授权的表现；分角色首次部署的顺序；待执行的迁移之后不重启即启动任务；200 万条过期会话清理中途 SIGTERM（约 40 万行/秒，1.0 秒退出）；12 次启动后立即停止；同一账户上并发的管理命令（另用去掉锁的二进制对照）；macOS 伪终端上的提示、回显与 Ctrl-C；957 个机密的 7 种写法（6,699 个串）在 6,216 行日志与输出中 0 命中，服务与命令的日志中也没有邮箱 |
| 日期 | 2026-10-01 |
| 结论 | 质量高，移植忠实，并有几处改进（迁移的逐字核对、Ctrl-C 恢复终端、停用与启用只在状态变化时生效、注册者一处组合、登录在锁下核对邮箱）。没有 Critical 与 Important。<br>2 项 Minor、4 项 Nit：3 项 Nit 与两项 Minor 在合并前处理，N4 不做（理由见下）。12 项偏差中第 9 项部分同意，第 6 项不属于 P4，其余同意 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| M1 | Minor | 管理员命令的账户行锁没有测试守住：`LockUserByEmail` 去掉 `FOR NO KEY UPDATE`，全部测试通过（交错五、六守不住它：重置的 `UPDATE users` 本身就等行锁）。去掉锁的二进制上，两个并发的 `users deactivate` 都跑了否决者、写入与订阅者（15/15），两个并发的 `set-email` 都"成功"（15/15）；M1 总设计第 8 节"持有 `FOR SHARE` 的事务让并发的停用等待"只在自助停用一路有证明 | 加交错：`ShareActiveAccount` 持有 `FOR SHARE` 并写入成员关系，管理员的停用等锁，提交之后否决者看到它、答出否决的码，账户仍可用。不加锁时否决者在加入提交之前运行、放行，停用随后完成。<br>反向对照：去掉 `FOR NO KEY UPDATE`，测试失败 |
| M2 | Minor | README 说"少了 River 的权限时 `/readyz` 仍是 200，只记在日志里"，实测缺 `river_queue` 时 River 的 `Start` 同步失败、serve 退出（exit 1）；"启动失败即停下 serve"这条路径（`app.go` 的 `cancel(err)`）没有测试 | README 按表写明：缺 `river_queue` 时服务启动失败退出，缺其他表的只记日志（`river_notification` 目前不写，看不出来）；授权文件在启动服务之前执行。加测试：运行时角色撤销 `river_queue` 之后，`run` 以 `start the jobs: … permission denied` 返回。反向对照：去掉 `cancel(err)`，测试以期限失败 |
| N1 | Nit | 等迁移的 warn 把任何检查失败都说成"待执行的迁移"：端口被占用时记它（原因是 `context canceled`），运行时角色在授权之前也记它（原因是没有权限建版本表） | 措辞改为 "background jobs wait for the migration check to pass"（带原因）；ctx 已结束时不记。另断言几次轮询只记一次 |
| N2 | Nit | 几处测试缺口：重置的规则不带邮箱（用例 `alice2026!` 不带邮箱也被名单拒绝）；重置在事务内算哈希；管理员停用交给注册者的是键入的地址；管理员停用被否决时不记日志；自助停用记成 `by=cli`；改邮箱用未规范化的旧地址比较；有待执行的迁移时 HTTP 自己停下，`run` 会一直等 | 各补测试，逐个反向对照都失败：规则用只因邮箱而常见的 `zqxj2026!`；哈希替身记录是否在事务内；管理员停用以 `" Alice@Corp.com "` 调用、断言否决者拿到规范化的地址；否决时的日志带码与 `by=cli`；自助停用的两条日志带 `by=self`（字面量改为常量 `bySelf`）；改邮箱以未规范化的旧地址调用；`run` 在地址被占用时返回错误 |
| N3 | Nit | README 对 River 停机 ERROR 的触发条件可以写准：启动后立即停止时 12/12 出现，运行几分钟之后停止 0/2 | README 写明"启动之后不久就停止时（重启循环、端到端测试）通常有，运行了一段时间的服务一般没有" |
| N4 | Nit（可选） | `users` 不检查迁移是否执行完，未迁移的库上答原始的 SQL 错误 | 不做：错误已经说出缺的表；goose 的"是否最新"检查在没有版本表的库上会建表，只读的命令不该有这个副作用。就绪检查仍是服务侧的关口 |

审查者另记，不列为发现：定时清理单次超过间隔时会重叠，`SKIP LOCKED` 使它们互不冲突；读密码的提示写到 stderr，stderr 被重定向时看不到提示。未验证：残留 `_ccnew` 索引与每天 WARN 的说法（按 River 的源码写）、River 自己每天的重建、Linux 终端。

## 对有意决定的判断

| 决定 | 判断 |
|---|---|
| River v0.47.0；原样导出 v2–v7，`StatementBegin/End`，不导 v1；升级另写迁移 | 同意 |
| 迁移与 `rivermigrate` 逐字比较 | 同意，比 SHA-256 强 |
| `schema_test` 排除 `river_` 表，up/down 清单加类型与函数 | 同意 |
| River 只由 `platform/jobs` 与 `adapter/river` 导入 | 同意（代码比设计更严，bootstrap 也不许） |
| 去掉启动的重试，`Start` 同步报错，启动失败停下 serve | 部分同意：快速失败是对的，但启动在迁移之后，"启动前已确认数据库可用"的理由不完全成立；缺 `river_queue` 的授权就会触发，README 写反、没有测试（M2，已修） |
| C1：`WithoutCancel`，只经 `client.Stop` 停止，之后才释放 | 同意 |
| `Stop` 的期限加 1 秒宽限，超时报错 | 同意 |
| 等迁移再启动任务，每 2 秒，只 warn 一次 | 同意；措辞见 N1（已修） |
| 停机顺序 HTTP → 任务 → 迁移器 → 连接池；最坏 36 秒，`docker stop -t 40` | 同意 |
| River 多占一个连接 | 同意 |
| 清理：`LIMIT` 加 `FOR UPDATE SKIP LOCKED`，每批 1000，只删过期的；`RunOnStart`，默认 1 小时，至少 1 秒 | 同意 |
| 管理用例按邮箱加锁、哈希在事务外、日志记 `user_id` 与 `by=cli` | 同意；锁的测试缺口见 M1（已补） |
| 停用、启用只在状态变化时生效；重置与改邮箱对已停用的账户照样执行 | 同意（并发下依赖 M1 的锁） |
| 登录在锁下核对邮箱；`email_unchanged` 只由命令行答出 | 同意 |
| 注册与 `CreateUser` 共用 `accountCreator`；`NewAdmin` 是包级函数，`Admin` 的方法是唯一入口 | 同意 |
| 注册者在 `registrants.go` 一处组合 | 同意 |
| 密码只从标准输入；终端两次不回显；非终端读一行、保留空格；Ctrl-C 恢复终端；配置 → 密码 → `awaitDatabase` | 同意（伪终端实测） |
| 错误格式与字段名映射；邮箱只进 stdout | 同意 |
| 组合检查沿静态调用图 | 同意 |
| 逐表授权；`river_job` 的 `MAINTAIN`；goose 只读；函数与类型靠 PUBLIC；文件幂等 | 同意 |

## 对偏差的判断

| 偏差 | 判断 |
|---|---|
| 1 后台任务的接线从 S1 移到 S2 | 同意：River 拒绝没有 worker 的客户端 |
| 2 `WaitForLockWaitsOn` 按表计数 | 同意：River 在同一个库上也等锁；注释写明的例外准确 |
| 3 `DeactivationSteps` 共用，拒绝的日志带 `by` | 同意；两条日志的测试缺口见 N2（已补） |
| 4 登录在锁下核对邮箱，`LockedAccount.ID` | 同意 |
| 5 README 写 River 停机时的 ERROR | 同意；措辞见 N3（已改） |
| 6 `CheckName` 另拒绝 Zl、Zp 与双向控制字符 | 同意规则，但它在 P3 的审查修复中落地，不是 P4 的偏差（已从 P4 的结果中去掉） |
| 7 `migrations/embed.go` 的注释写明 River 的迁移 | 同意 |
| 8 错误格式在 `users.go` | 同意（内聚）；设计 3.1 与 S4 计划已同步 |
| 9 组合检查另断言 `Users` 与 `newApp` 都到达 `deactivationRegistrants` | 部分同意：静态可达只证明调用了，不证明结果交给了 `NewAdmin`/`New`。M1 没有注册者，可以接受；M2 的第一个注册者要有经命令行的行为测试（写进 M1→M2 的移交） |
| 10 e2e 只在设置密码的命令上断言有日志行 | 同意 |
| 11 `image-smoke` 以 `users create` 建第一个账户并登录 | 同意 |
| 12 运行时角色的测试另走接口与管理命令，授权文件执行两次 | 同意：这才抓得住缺失的授权与幂等 |

## 文档与代码的不一致

审查者列出 9 处，全部处理：

1. 设计 3.1 的文件表：错误格式在 `users.go`、读取密码在 `password.go`、`parts.go` 不含 `CredentialLock`（设计 3.6 同）。已改。
2. 设计 3.2：`.gitattributes` 只有 `linguist-generated=true`（LF 由全局规则保证）；River 不许 bootstrap 导入。已改。
3. 设计 3.4 的理由：River 的 `Start` 在没有它的表时同步失败，而不是启动成功之后反复记错误。已改，并写明启动失败与 HTTP 自己停下的两条路径。
4. README 的授权说法：M2（已改）。
5. 设计 3.6"地址不是合法 UTF-8 时不查库"：改为 `ValidEmail` 不通过的都不查库。
6. 设计 3.11 与 S2 计划的"`lock_timeout` 之内返回"：改为 2 秒的期限。
7. 设计 3.12"断言 stderr 有日志行"：改为只对设置密码的命令。
8. M1 总设计第 8 节"持有 `FOR SHARE` 的事务让并发的停用等待"：M1 的交错之后覆盖自助与管理员两路，第 8 节写明。
9. 设计第 7 节的规模数据：按审查者的统计填写。

## 没有失败的反向对照

- `LockUserByEmail` 去掉锁（M1，已补）；任务启动失败不停下 serve（M2，已补）；去掉 HTTP 返回之后的 `cancel(nil)`、重置的规则不带邮箱、重置在事务内算哈希、管理员停用交给注册者键入的地址、管理员停用被否决时不记日志、自助停用记成 `cli`、改邮箱用未规范化的地址比较（N2，已补）；等迁移时每次轮询都 warn（N1，已补）。
