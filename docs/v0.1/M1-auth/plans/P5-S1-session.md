# M1/P5/S1 会话模块：实施计划

上级：[P5 文档](../05-P5-web-session.md) 3.2。

## 任务

1. 确认运行环境：把 Nerve 的 `token-manager`、`refresh-lock`、`auth-middleware` 与 6 个测试、3 个替身拷进 `src/session/`（替身放 `session/testing/`，`fake-nerve.ts` 改名 `fake-server.ts`），先在本项目的 jsdom 与 `test/setup.ts` 下原样运行；不行时这几个文件声明 node 环境，`setup.ts` 对 `document` 的操作加判断。结果记下来，写进第 7 节。
2. 改名与清理：`nwiki.auth`、`nwiki.auth.refresh`、`nwiki.auth.lease`；导入改成相对路径与 `@nervewiki/api-client`；删掉指向 Nerve 文档的注释，行为不改。8 秒的请求超时与服务端 `config/validate.go` 的 `webRefreshTimeout` 互相注明。
3. api-client 导出 `Middleware` 类型。
4. `session/session.ts`：`Session`（公开客户端、令牌管理器、续期锁的选择、`clientFor`、`start`、`dispose`）。
5. oxlint：`createClient` 的放行从 `main.tsx` 移到 `src/session/**`。
6. `main.tsx`：构造 `Session`（localStorage，不可用时退回内存；`navigator.locks`；storage 事件），`start()`；实例信息改用 `session.public`。页面不变。

## 测试

- 拷来的测试全部通过（多标签页的用例在 Web Locks 与租约下各跑一次）。
- 装配：没有记录时 `start` 不发请求；公开客户端的请求不带 `Authorization`；`clientFor(a)` 的请求带令牌、会话换成 b 之后以 `SessionChangedError` 结束；storage 事件到达令牌管理器与租约；`locks` 为 `undefined` 时用租约。
- 反向对照：`clientFor` 不绑定 loginId（中间件不检查会话）→ 装配的测试失败。

## 完成检查

`make check` 为绿。
