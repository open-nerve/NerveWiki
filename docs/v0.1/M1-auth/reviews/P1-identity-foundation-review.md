# M1/P1 身份基础与默认拒绝：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m1-p1-identity-foundation`（`main..1ab005e`，S1–S5），对照 [01-P1-identity-foundation.md](../01-P1-identity-foundation.md)、各 Step 计划、[M1 总设计](../00-M1-design.md)第 4–8 节 |
| 审查方式 | 独立审查者在仓库的克隆上实测：<br>• 门禁：`make gen-check`、`make check`、`make e2e`；`--repeat-each 8 --workers 8`（72 次）与 `--repeat-each 3 --workers 1`（27 次）全部通过；`make image-smoke`（非默认版本）；identity、httpserver、config、shared 各 `go test -race -count=5`；<br>• 重跑第 5 节的反向对照，另自拟约 25 项（HKDF info、401 不带质询、XFF 从左读、刷新令牌去掉严格解码与代数上限、过期判定的顺序、错误带上私钥路径、`/0` 与 IPv4 映射的可信代理前缀、公开操作也查令牌、认证故障答 401 等）；<br>• 活体探针：自起 PostgreSQL 与 `nervewiki serve`（debug 日志）：畸形的 Bearer、900 KB 的 Authorization 头、重复键的 JSON、同一邮箱 20 路并发注册、prod 下各种坏私钥文件、日志中搜索邮箱、密码、令牌与 XFF；PG18 的 `lower()` 与 Go 的规范化对 12.6 万个合法邮箱逐一比对 |
| 日期 | 2026-09-30 |
| 结论 | 质量高，没有 Critical：默认拒绝、公开操作按 `r.Pattern` 匹配、401 的质询头、JWT 校验（固定的失败原因、签名先于过期）、HKDF 的已知答案测试、argon2 的并发名额与不可用哈希、注册顺序、私钥与日志脱敏，都经实跑与反向对照验证。<br>1 项 Important（引导步骤的 CHECK 可被含逗号的元素绕过）、4 项 Minor、7 项 Nit：Important 与 Minor 全部在合并前修复；Nit 中两项随 P2、一项随 P3 处理，其余修复。9 项有意的决定中 7 项同意，2 项附条件（已按条件处理） |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| I1 | Important | `users_onboarding_steps_check` 把数组用逗号拼接后做正则，没挡住元素本身含逗号：`'{"profile,workspace"}'` 能写入，一个由 40 个 id 拼成的单元素也能，既绕过 id 格式也绕过 32 个的上限。P1 的领域一侧还没有这项校验，数据库是唯一的防线；迁移上线之后再改代价高 | CHECK 加一条：拼接前的元素不含逗号（`strpos(array_to_string(onboarding_steps, ''), ',') = 0`），迁移注释写明原因；数据库测试补"一个元素里两个 id""一个元素里 40 个 id"两个反例。<br>反向对照：去掉这一条，两个反例失败 |
| M1 | Minor | 名单只收 8 个字符以上的条目，主干规则因此只对 8 个字符以上的主干生效：`Qwerty123!`、`Summer2024!`、`Welcome1!`、`Letmein1!`、`Admin123!`、`Monkey123!`、`Dragon2024!`、`Michael1!` 全部通过，这正是撞库与密码喷洒最先尝试的形式 | 生成脚本另收每个条目的主干（去掉两端非字母，5 个字符以上；4 个字符的 love、blue 是太多正常密码的一部分），同一份名单既比整串也比主干，名单从 46,483 条变为 67,396 条；Go 的名单测试按新的条件核对每一条；上面 8 个密码进入测试。<br>反向对照：加载名单时丢掉 8 个字符以下的条目，8 个全部通过 |
| M2 | Minor | 设计写"收到头却没配置时告警"，代码却对任何不受信的对端都告警，已配置可信代理时也一样：任何直连的客户端都能让告警点名自己，文案还建议把它加进 `trusted_proxies`（照做就让它能伪造 IP，P2 的按 IP 限流随之失效）；这次告警被占掉之后，真正的代理漏配不再告警 | 只在没配置可信代理时告警；配置之后，其他对端带来的头静默忽略，测试 `TestForwardingFromAClientIsIgnoredSilently` 证明它既不告警、也不占掉代理漏配的那一次 |
| M3 | Minor | `jwt.WithStrictDecoding()` 没有测试守住：去掉它全部测试照样通过（"padded base64"用例与严格解码无关）；探针实测，改掉签名末字符的无用低位，宽松解码仍接受这个令牌 | 固定原因的测试加"签名的非规范写法"一例（改掉末字符的无用低位），期望 `errMalformed`。<br>反向对照：去掉严格解码，这一例失败 |
| M4 | Minor | 密码没有做 Unicode 规范化：同一个 `café` 的 NFC 与 NFD 得到不同的哈希，跨设备登录可能失败；现在还没有哈希落库，是最便宜的决定时机 | 采纳 NIST SP 800-63B 5.1.1.2：哈希与校验的都是密码的 NFKC 形式（预组合与分解的重音、全角与半角字母是同一个密码）。放在 argon2 适配器：domain 与 app 只用标准库，而注册、登录、改密码与之后的管理员命令都经过这一个哈希器；`golang.org/x/text` 由间接依赖变为直接依赖。测试覆盖分解的重音、全角字母、连字相等，西里尔字母 а 与拉丁字母 a 不相等 |
| N1 | Nit | `PasswordHasher` 含 `Verify`，P1 唯一的使用者注册从不调用，与"端口按用例细分"不符 | 随 P2 的登录拆成 `PasswordHasher{Hash}` 与 `PasswordVerifier{Verify}`（P2/S2）。`RefreshTokenMAC.Verify` 由 P2 的续期调用，不拆 |
| N2 | Nit | `config.yaml` 的注释"旧的哈希在下次登录时按新参数重算"在 P1 不成立：没有登录 | P2/S2 的登录实现了重算，注释随之成立，不改 |
| N3 | Nit | 测试断言 8 个空格是可接受的密码；探针实测 8 个 NUL 也能注册 | 同一字符的重复、只有空白与控制字符的密码按 `common_password` 拒绝，测试改为期望拒绝，并覆盖全角空格、制表符、NUL、重复的 emoji |
| N4 | Nit | JWT 的 `exp` 是整秒，截断之后最多比 `now + 15m` 早 1 秒，而 `access_token_expires_in` 固定答 900 | `exp` 向上取整到秒：令牌至少活到响应所说的时长（最多多活不到 1 秒），`expires_in` 保持如实 |
| N5 | Nit | A1 只证明行与刷新令牌一致，没有证明访问令牌的 `sub`、`sid` 指向新建的账户与会话 | `expectNewSession` 解码访问令牌的 payload（不验签，签名归服务端），核对 `sub` 与 `sid` |
| N6 | Nit | Makefile 对 `GEN_GO_OUT` 的注释"gen 目录里先有 oapi-codegen.yaml"对 `postgres/gen` 不成立：模块第一次产出 sqlc 生成物时，这个目录在读 Makefile 时还不存在 | 注释写明：`postgres/gen` 第一次出现时不在检查之列，没提交的生成代码由持续集成的编译兜底 |
| N7 | Nit | `TestParametersThatDoNotBindAnswer400` 在 0 个用例时仍然启动整个应用和数据库 | 与有意决定 6 一致：P3 出现第一个路径参数（`DELETE /api-tokens/{token_id}`）时加"至少推导出一个用例"的守卫，写进 P3 的计划 |

## 审查者对有意决定的判断

| 决定 | 判断 |
|---|---|
| 1. `Authenticator` 在 P1 返回两个值，凭证键随 P2 的限流加入 | 同意：现在加是 YAGNI；设计 3.5 的签名要同步改（已改） |
| 2. 撤销原因、续期的判定表、显示名与引导步骤 id 的校验推迟到用到的 Phase | 同意；同一原则也适用于端口（N1） |
| 3. 名单只收 8 个字符以上的条目 | 部分不同意：条目本身这样取合理，但主干规则因此失效，见 M1（已按建议另收主干） |
| 4. 客户端 IP 三种告警 | 同意，附条件：不受信对端的那一条只在没配置可信代理时告警，见 M2（已按条件处理） |
| 5. `signupSwitch` 在组合根 | 同意：把配置翻译成端口的胶水，M2 换成考虑邀请的策略时由组合根替换 |
| 6. 参数破坏用例在 P1 没有"至少一个"的守卫 | 同意：现在没有路径参数，守卫会误报；随 P3 加入（N7） |
| 7. 公开操作的整个程序测试是行为测试 | 同意：它测的是接好线的应用本身，模块清单的漏写、多写、契约保护而模块写成公开，都能抓到（审查者以 getMe 做了对照） |
| 8. prod 的 `migrate up` 也要配置私钥路径 | 同意：配置只有一条校验路径，README 已写明；`migrate` 只检查已设置，不读取 |
| 9. `onboarding_steps` 为 nil 时由 HTTP 适配器答 `[]` | 同意：JSON 形状归 HTTP 适配器，`TestGetMe` 的 nil 用例守住（对照已验证） |

## 文档与代码的不一致

审查者列出 11 处，全部处理：

- P1 文档 3.5 的 `Authenticator` 签名、客户端 IP 的告警、3.7 注册关闭的 403 的位置（在领域校验之前，结构检查的 400 与上限的 413 仍在它之前）、3.9 的整个程序测试、3.4 的名单与密码规则、3.2 的 CHECK、3.1 与第 5 节中推迟的领域校验：按代码与上面的处置改写，出入列在文档第 7 节；
- S3 计划的反向对照"注册关闭时先校验请求体 → handler 测试失败"：handler 测试用的是假用例，实际失败的是 app 的 `TestRegisterWhileSignupIsOff` 与 A2，已改；
- `config.yaml` 的重算注释：见 N2；
- 3.10 没有提到 `countIdentity`：无害，不改。

## 反向对照

设计第 5 节的 8 项，审查者重跑，全部按预期失败：`getMe` 标成公开；中间件放过没有令牌的请求；sqlc 查询读 `goose_db_version`；模块里直接 `pool.Exec`；删掉 `users_onboarding_steps_check`；名单为空；注册顺序颠倒（app 测试与 A2 失败）；prod 缺私钥。S5 的"注册不写会话"让 A1 失败。

没被抓住的 3 项及结论：`WithStrictDecoding`（M3，已补测试）；`WithValidMethods`（纵深防御：jwt/v5 的密钥类型检查本身拒绝 HS256 与 none，可以接受）；请求信息与请求期限对调（两者的顺序对行为没有影响）。

## 审查过程的说明

审查者停止自己起的 serve 进程时，第一次用了按命令行匹配的 `pkill -f "bin/nervewiki serve"`，理论上会命中当时恰好在运行的其他 nervewiki 进程，之后改为按 PID 停止。以后的审查说明中写明只按 PID 停止自己起的进程。
