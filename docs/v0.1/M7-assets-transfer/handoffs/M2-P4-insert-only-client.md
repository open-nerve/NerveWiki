```yaml
status: open
from: M2/P4
to: M7
created: 2026-10-01
```

# 只投递的 River 客户端

[M1 移交](../../M2-workspace/handoffs/M1-identity.md)第 10 项转来：M2 没有由请求投递的后台任务（软删除的清理是定时的，见 [M2/P4 文档](../../M2-workspace/04-P4-deactivation-commands-purge.md) 3.4），第一个由请求投递的任务是 M7 的导入导出。

M7 加导入导出的任务时：

1. **只投递的客户端**：接口一侧（`serve` 的 HTTP 处理）投递任务，不运行 worker。`platform/jobs` 现在只有服务的客户端（`jobs.New`：运行 worker、投递定时任务，`Start` 之后才能用）；[M1/P4 文档](../../M1-auth/04-P4-admin-jobs.md)第 2 节把只投递的客户端留到第一个由请求投递的任务出现时。建议：一个不配 `Queues` 的 River 客户端，在请求的事务里用 `InsertTx` 投递，与业务写入同一个提交，回滚时任务也不存在。
2. **停机顺序**已经让 HTTP 先于后台任务停（同一文档 3.4）：正在处理的请求投递任务时，任务一侧还没停。
3. **组合检查**（`archtest/composition_test.go`）：命令行的组合不构建 `platform/jobs` 与 River。命令行若要投递任务（例如管理员触发的导出），要么经接口，要么修订这条规则，并在 M7 设计里写明。
4. **权限**：`deploy/runtime-grants.sql` 已给 River 的表读写；只投递的客户端不需要别的权限。
