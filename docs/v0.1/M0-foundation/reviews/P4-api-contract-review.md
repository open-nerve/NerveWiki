# M0/P4 接口契约与代码生成：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m0-p4-api-contract`（`2aebc4e..63a2168`），对照 [04-P4-api-contract.md](../04-P4-api-contract.md)、各 Step 计划与 [M0 总设计](../00-M0-design.md) |
| 审查方式 | 独立审查者在仓库的 worktree 上实测：<br>• 跑全部门禁，`make check`、`make gen-check`；<br>• `-race -count=5` 查不稳定的测试；<br>• 二进制在 testcontainers 建的临时库上冒烟；<br>• 对照 Go 1.27.1、oapi-codegen v2.8.0、kin-openapi 的源码核实行为；<br>• 重跑第 5 节的反向对照，另外自拟 10 项探针 |
| 日期 | 2026-09-30 |
| 结论 | 分层干净，没有上帝文件，M0 对 P4 的五条要求都已落实，反向对照全部有效。<br>发现 1 项 Important、7 项 Minor、6 项 Nit，全部处理。<br>实施时的 4 处偏离与各项有意的裁剪，经审查确认合理 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| I1 | Important | 公共组件的 Go 类型（`apigen`）用 oapi-codegen 的默认映射生成，没有模块模板的 `nullable-type` 与 `type-mapping`，而 bodyshapegen 按模块配置建结构表。<br>模块的请求体引用公共组件时，strict handler 解码进的是 `apigen` 的类型，两者的假设不一致：<br>• `number` 生成 `float32`，超出范围的数通过结构检查，到解码时才得到没有字段信息的 400；<br>• `email` 在解码时校验，绕过领域层的 422；<br>• 可为空的字段区分不了"没传"和"传 null"。<br>三者都没有门禁能发现，只有 `uuid` 会被架构测试抓到 | `apigen` 的配置补上 `nullable-type` 与整段 `type-mapping`，当前的生成结果不变。<br>bodyshapegen 新增测试：`apigen` 与每个模块的配置在 `compatibility`、`name-normalizer`、`nullable-type`、`type-mapping` 上一致。<br>反向对照：`apigen` 删掉 `nullable-type`，或者模块的 email 映射改回 `openapi_types.Email`，测试都失败 |
| M1 | Minor | `apitest` 的 `TestCheckResponseRecordsTheAnsweredCode` 在 `-count>1` 时失败。进程级的记录器在重复运行之间累积，而测试断言它在调用前为空（从 Nerve 继承） | 测试开头清掉自己那条操作的记录（测试文件中的 `forget`）。`-race -count=5` 为绿 |
| M2 | Minor | P4 文档 3.2 规定可为空的对象写成 `oneOf: [{$ref}, {type: 'null'}]`，而 bodyshapegen 在请求体里只接受 `anyOf`。第一个带可为空对象的 PATCH 会照着文档写，然后生成失败 | 写法约定、规则测试的提示与样例都改为 `anyOf`：oapi-codegen 与 openapi-typescript 对两种写法的生成结果相同 |
| M3 | Minor | TS 的类型检查用例没有固定 `error` 的类型：生成的类型里没有 default 响应时，`error` 是 `never`，`const problem: Problem = error` 照样编译 | 加一行带 `@ts-expect-error` 的 `const unreachable: never = error`。反向对照：删掉 `schema.gen.ts` 中的 default 响应，类型检查失败 |
| M4 | Minor | 逐路由中间件的顺序只固定了"上限在结构检查之前"。把请求期限挪到最里层，全部测试照样通过；而 M1 要在中间插入认证与限流，期限必须在最外层 | 新增测试：请求体读取时停顿 300 ms，处理器剩下的预算必须少于请求期限减去这段停顿。反向对照：期限挪到最里层，测试失败 |
| M5 | Minor | README 与 Makefile 说 `make check` 是"持续集成的全部门禁"，而持续集成另跑 `gen-check-go`、`gen-check-web`；照此在本地跑的人会推上过时的生成物 | 文字改为：`make check` 与提交之后的 `make gen-check` 合起来是全部门禁。`gen-check` 要求生成物已提交，不并进 `check` |
| M6 | Minor | `apitest.Main(m, "instance")` 是约定，有两个漏洞：<br>• 没有检查要求每个模块的适配器测试都调用它；<br>• 新模块照抄这一行，核对的是 instance 的错误码。<br>两种情况都会让"声明的码必须被答过"这一半静默失效 | `Main(m)` 从调用者的源文件路径得出模块名，不在模块的 `adapter/http` 里调用时直接退出。<br>新增测试：每个 `api/modules/<m>.yaml` 对应的 `adapter/http` 都有调用 `apitest.Main` 的 `TestMain`。<br>反向对照：去掉 instance 的调用，测试失败 |
| M7 | Minor | 推迟到 M1 的内容没有记全：<br>• 裁掉的两条写法规则、problem 响应头对 `WWW-Authenticate` 的要求不在 P4 文档第 2 节；<br>• M1 的移交没有 P4 的任何一项；<br>• 文档写 `New(Deps)` 与 `PublicOperations()`，代码是 `New()` | P4 文档第 2、3.5、7 节，M0 总设计第 7、8 节修订。<br>新建 [M1 的移交](../../M1-auth/handoffs/M0-P4-api-contract.md) |
| N1 | Nit | `check-committed` 依赖 shell 展开 `modules/*/adapter/http/gen`：作为 git 的 pathspec，这个 glob 什么都匹配不到，将来有人给参数加引号，检查就静默失效 | 改用 `$(wildcard …)` 在读 Makefile 时展开，并注明原因 |
| N2 | Nit | `apierrors_test.go` 有三处残留的 Nerve 引用（`identity`、"M0-P3 handoff 2"），在本仓库中是悬空的 | 删掉 |
| N3 | Nit | `BadRequest` 按结构读取绑定错误的参数名，只用外形相似的替身测过；instance 的 `gen` 已经生成了全部 6 种真实类型 | instance 的适配器测试把 6 种生成的类型逐一交给 `BadRequest`，断言参数名与错误码。oapi-codegen 升级时改了字段名会在这里失败 |
| N4 | Nit | 两处缺口：<br>• 契约中以 `/` 结尾的路径会注册成 ServeMux 的子树模式，吞掉所有子路径，也遮住 `/api/` 兜底；<br>• 整个程序的测试只比较 `/api/v0/` 下的路由，`GET /api/admin` 这类路由逃过契约 | 规则测试新增"路径不以 `/` 结尾"。整个程序的测试改为比较 `/api/` 下除平台兜底之外的全部路由。<br>反向对照：注册 `GET /api/admin`，测试失败 |
| N5 | Nit | 临时分支 `tmp-p4-stale-gen` 的持续集成结果无法核实 | 那次运行（run 36699877466）的 `server`、`web` 两个任务都在"Generated code"一步失败，符合预期；分支在审查结束前已删除 |
| N6 | Nit | 请求期限到期的处理器错误记成 ERROR 级的"API handler failed"，运维分不清超时与基础设施故障 | `Write` 在请求的 context 已过期限时记 WARN 级的"API request deadline exceeded"，仍然答 500 `internal_error`：错误码集合不变。<br>新增测试：请求已过期限 → WARN；处理器自己的较短超时 → ERROR |

## 审查者对偏离与裁剪的判断

实施中的偏离：

1. **bodyshape 与 api-client 骨架提前到 S1**：合理。生成代码导入 bodyshape，`gen-web` 需要这个包，顺序由依赖决定。
2. **`instance.New()` 不带 `Deps`**：合理。空的 `Deps` 结构是凭空的设计，注释已写明有依赖的模块才带。
3. **长连接测试经过全局中间件链**：合理，而且更强。它验证真实的组合。两个反向对照都让它失败：把长连接路由包进逐路由中间件、把期限挪进全局链。
4. **`platformCodes`、`codePattern`、`moduleNames` 移进测试文件**：合理。apitest 的生产接口更小，也不需要为包级变量加 `nolint`。

有意的裁剪：

- **移到 M1 的部分**：认证、公开操作清单、`WWW-Authenticate`、bearer scheme、限流、客户端 IP、参数与请求体的整个程序测试及其用例推导、两条写法规则、二进制禁用清单对 runtime 的例外，都合理。规则测试已经要求每个操作显式声明 `security`、引用的 scheme 必须存在，M1 不会漏掉 scheme。
- **kin-openapi 的两个版本**：`server` 用 v0.149.0，`server/tools` 用随 oapi-codegen 的 v0.142.0。合理，不建议统一：bodyshapegen 必须与 oapi-codegen 用同一个加载器，apitest 是独立的；两版之间的变化在 API 层面，apitest 已显式设置 `NoopAuthenticationFunc`。

## 反向对照

设计第 5 节的各项，审查者重跑，全部按预期失败：

- 改描述而不重新生成，`gen-check-go`、`gen-check-web` 都失败；新模块的生成物未提交，也失败。
- `Problem` 的 `detail` 改名：平台契约测试失败。
- 响应缺一个必填字段、多一个字段：模块契约测试都失败。
- 声明了码而没有测试答它：`apitest.Main` 失败；答出一个未声明的码：`CheckResponse` 失败。
- 实现多一个路由、契约多一个没有实现的操作：整个程序的测试都失败。
- `shared` 多一个字段错误码：失败。
- 模块导入 `apitest`：规则 8 与二进制禁用清单都报错。
- 长连接路由包进逐路由中间件：失败。
- TS 去掉一处 `@ts-expect-error`：类型检查失败。

修复后新增的反向对照见上表各项。

## 核实修复

- 本地 `make check`、`make gen-check` 为绿；`-race -count=5` 重复跑 httpserver（含 apitest、bodyshape）与 instance 各包为绿。
- 修复后的持续集成（run 36702688431）为绿。
