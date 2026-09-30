# M1/P2 会话与限流：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m1-p2-sessions-ratelimit`（`51bc900..6accf06`，S1–S4），对照 [02-P2-sessions-ratelimit.md](../02-P2-sessions-ratelimit.md)、各 Step 计划、[M1 总设计](../00-M1-design.md)与 [P1 审查](P1-identity-foundation-review.md) |
| 审查方式 | 独立审查者在仓库的克隆上实测：<br>• 门禁：`make gen-check`、`make check`、`make e2e`；`--repeat-each 5`（85 次）与 `--workers 1 --repeat-each 3`（51 次）全部通过；`make image-smoke`（非默认版本）；identity、httpserver、ratelimit、config、clock、pgtest、bootstrap 各 `go test -race -count=5`；<br>• 与 Nerve 逐文件 diff：`ratelimit`、`limit.go`、失败闸门、续期、退出、判定表、会话的 SQL 只有注释与路径不同；<br>• 重跑第 5 节的反向对照，另自拟约 20 项（只去掉哈希条件、去掉重读、当前代也要求 MAC、锁下不核对快照、先插会话再锁、成功或故障不退还、IPv6 键用完整地址、`Retry-After` 向下取整、handler 不设期限等）；<br>• 活体探针：自起 PostgreSQL 18 与 `nervewiki serve`，跑 IPv4、`[::1]` 加可信代理、小闸门三轮：等时登录（30 次中位数 16.4 对 16.3 ms）、续期与伪造与重复使用的库内状态、5 路并发续期、闸门空了之后有效令牌也答 429、IPv6 前缀、行锁下的续期期限、SQL 停用账户之后的续期与登录、表被锁时的闸门、超长密码与请求体、三轮日志中搜索邮箱、密码与令牌 |
| 日期 | 2026-09-30 |
| 结论 | 质量高，移植忠实，没有 Critical 与 Important：判定表、条件轮换加重读、MAC 与当前代的分工、换钥、失败闸门先预留再认证与三种退还、IPv6 前缀、等时登录、快照重试、锁顺序、期限的配置校验，都经实跑、反向对照与活体验证。<br>2 项 Minor、5 项 Nit：全部在合并前处理（N3 写进 P3 的计划与测试）。10 项有意的决定中 9 项同意，1 项部分同意（测试各自构建 `httpserver.API`，M2 出现第 4 处时提取辅助） |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| M1 | Minor | 续期与退出的期限是 handler 内派生的上下文，而 `APIErrors.Write` 按 `r.Context()` 判断请求期限：4 秒到期不走平台的 warn "API request deadline exceeded"，记成 error "API handler failed"。锁竞争、数据库变慢时这是协议内的正常结局，却被报成基础设施故障 | 平台的期限中间件接受逐路由的期限（`APIConfig.RequestTimeouts`，按路由模式，须在 0 与 `RequestTimeout` 之间）；identity 声明续期与退出的期限为 `auth.refresh_deadline`，handler 不再自己派生。到期由平台照常记 warn、答 500。配置另校验 `refresh_deadline` 不超过 `server.request_timeout`。<br>没有改答 503：到期时语句在 COMMIT 之前被放弃，令牌仍是当前代，500 已足够让客户端重试；而要在 handler 里区分"语句被放弃"与"COMMIT 超时、结果未知"，需要事务管理器另给错误类型，得不偿失。<br>测试：平台的路由期限（答 500、记 warn、其他路由不受影响）与参数校验；identity 的集成测试持有会话行，续期在 200 ms 的期限内答 500、会话不变，放行之后同一令牌重试答 200。<br>反向对照：模块不声明路由期限，集成测试失败（按 5 秒的请求期限才答） |
| M2 | Minor | 每个 429 记一行 info：被闸门与平台的桶拒绝的请求不受任何桶约束，客户端以请求速率换日志行 | 平台的桶（`anonymous`、`authenticated`、`auth_failure`）改记 debug：访问日志已在 info 级记下每个 429，再记一行只是把日志翻倍；模块的桶（登录、注册）保留 info，它们是撞库的信号。README 写明 |
| N1 | Nit | 设计第 5 节反向对照第 1 条"去掉代数条件 → 并发续期测试失败"不成立：哈希条件同样让第二次更新落空，真正失败的是仓储的 `another_generation` | 改文档：代数条件由仓储的逐条件测试守住；并发续期的测试守的是重读与重判（去掉重读它就失败），两个条件都去掉时它也失败 |
| N2 | Nit | `Retry-After` 精确断言为 `"60"`：从第一次取单位到被拒绝超过 1 秒就答 59，慢的持续集成可能偶发失败 | bootstrap 的限流测试与 A14 接受 59 或 60 |
| N3 | Nit | 续期不读 `users.is_active`：停用账户的会话续期答 200。它依赖"停用即撤销全部会话"，P2 没有停用路径，接口上不可达 | 写进 P3：停用撤销全部会话是扩展点之外的不变量，P3/S4 的测试覆盖"停用之后刷新令牌答 401"；续期不加账户的联表，理由同 Nerve：撤销是唯一的真相，P4 的 `users activate` 不复活旧会话 |
| N4 | Nit | `TestLoginRehashesAnOldHash` 的注释说"输家再校验一次"，去掉锁下的快照核对它仍通过；"哑哈希的参数是当前配置的"没有测试 | 注释改为只声称结果一致，快照重试由用例的替身测试守住；设计 3.9 与 S2 计划的措辞改为用例测试与真实数据库各自守住什么；哑哈希由同一个哈希器生成，参数一致是结构上的，不另加测试 |
| N5 | Nit | 过时或不准的注释：`issuer.go` 仍写"from P2"；模块的包注释没提登录、续期、退出；`CredentialLocker` 说"每个签发凭证的事务先取"，注册却不取；`WaitForLockWaits` 的失败信息 "fewer than 1 statements" | 逐一改正；`WaitForLockWaits` 的失败信息改为"n statement(s) waited for a lock within …, want at least m"，带上最后一次读到的个数 |

## 对有意决定的判断

| 决定 | 判断 |
|---|---|
| 1. `ratelimit` 几乎原样拷贝，时间取单调时钟 | 同意：墙上时钟的跳变不应灌满或抽干桶；继承的单一全局锁与清扫在 v0.1 单实例够用 |
| 2. 限流在认证之后；闸门先预留，成功、过期、故障退还 | 同意。数据库变慢时在途的名额会增多、有效请求也可能先答 429（实测），已补进设计 3.3；过期令牌的洪泛只花一次验签，可以接受 |
| 3. `rate_limited` 只在顶层 | 同意 |
| 4. 登录邮箱不做格式校验 | 同意：不合法与不存在都答 401，只差一次查库（约 1 ms），透露的只是格式 |
| 5. `PasswordHasher` / `PasswordVerifier` 拆开，`RefreshTokenMAC` 不拆 | 同意 |
| 6. P2 不建 `RevokeReason` 类型 | 同意：两处字面量有 CHECK 兜底 |
| 7. `WaitForLockWaits` 提前引入 | 同意：并发续期因此是确定性的 |
| 8. identity 的测试各自构建 `httpserver.API` | 部分同意：现在 3 处，P2 给 `NewAPI` 加字段就改了 3 处。M2 出现第 4 处时在 `apitest` 提取构造辅助（M1 收尾时写进给 M2 的移交） |
| 9. 访问令牌 `exp` 向上取整，续期同样 | 同意，已核实 |
| 10. `password_user` 推迟到 P3 | 同意 |

## 文档与代码的不一致

审查者列出 9 处，全部处理：

- 设计第 5 节反向对照第 1 条：见 N1；
- 3.4 第 2 步写"读账户（id、密码哈希、是否可用）"：代码只读 id 与哈希，是否可用在锁下读，改文档；
- 3.1 把"顶层 x-problem-codes 加 rate_limited"写在 `api/modules/identity.yaml` 名下：实际在 `api/openapi.yaml`，改文档；
- 3.9 与 S2 计划的哑哈希、快照重试：见 N4；
- S4 计划说 `auth.ts` 增加 `logout`：退出在故事中直接调用，改计划；
- 3.3 的"代价"没提数据库变慢：见决定 2；
- 第 1 节说 P2 接手"判定表与撤销原因"：撤销原因的类型推迟到 P3，措辞对齐；
- 前端的 8 秒只在 `config/validate.go` 与契约描述里：写进 M1 总设计 P5 一行，令牌管理器用同一个值；
- P2 文档的状态与第 7 节：随合并更新。

## 反向对照

设计第 5 节的 6 项，审查者重跑：第 2–6 项按预期失败（旧代不核对 MAC、不做哑哈希、闸门在认证之后、过期不退还、期限的边界）；第 1 项不成立，见 N1。自拟的约 20 项全部被抓住，其中"锁下不核对快照"只被用例测试抓住（N4）。
