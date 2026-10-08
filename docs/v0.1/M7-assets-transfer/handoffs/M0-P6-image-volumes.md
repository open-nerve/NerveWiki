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

## 处理进展

- M7/P1（2026-10-08，合并 `d7af7cf`）：第 1 项完成（构建阶段建 `/out/data`，`COPY --chown=65532:65532` 进运行时阶段的 `/data`，`VOLUME /data`，`NWIKI_STORAGE__DIR=/data`）；第 2 项完成（`storage.OpenLocal` 探测根与各区，不可写时 `serve` 拒绝启动、写明目录与 uid、gid；README 的附件目录一节；`image-smoke` 在属主为 root 的 tmpfs 上核对拒绝启动）。第 3 项（上传、重启、读出）留给 P2（[P1 文档](../01-P1-storage-stream.md)第 7 节）。
