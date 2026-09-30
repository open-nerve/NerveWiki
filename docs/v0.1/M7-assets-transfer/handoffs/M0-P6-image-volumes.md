```yaml
status: open
from: M0/P6
to: M7
created: 2026-09-30
```

# 镜像里的附件目录

M0/P6 的镜像（`deploy/Dockerfile`）只有 `/nervewiki` 一个程序：运行时是 distroless，没有 shell，进程以 uid 65532 运行（[P6 文档](../../M0-foundation/06-P6-e2e-delivery.md) 3.5）。M0 还没有要写磁盘的数据。M7 引入本地附件存储时（总体设计第 11 节："数据目录与附件目录挂载为卷"）：

1. **目录与卷**：在镜像里建好附件目录，属主是 65532，并声明为卷。distroless 没有 `mkdir` 与 `chown`，要在构建阶段建好目录，再用 `COPY --chown=65532:65532` 复制进运行时阶段。
2. **挂载的权限**：宿主机目录或卷的属主不是 65532 时，服务写不进去。启动时检查目录可写，不可写就拒绝启动，并在错误中写明 uid；README 的部署一节写明挂载方式。
3. **image-smoke.sh**：加一步上传附件、重启容器之后再读出，证明附件落在卷上。
