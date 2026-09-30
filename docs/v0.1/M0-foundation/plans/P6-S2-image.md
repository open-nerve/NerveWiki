# M0/P6/S2 镜像：实施计划

上级：[P6 文档](../06-P6-e2e-delivery.md) 3.5、3.6。

## 任务

1. `deploy/Dockerfile`：web → server → runtime 三个阶段，基础镜像按摘要固定；运行时 distroless（默认变体）加 `USER 65532:65532`。
2. `.dockerignore`：排除的正好是 `.gitignore` 忽略的（依赖、构建产物、`e2e` 的结果与报告、本地的机密）；保留 `.git`（VCS 信息）。
3. `deploy/image-smoke.sh`：提交信息、临时网络、PostgreSQL、`migrate up`、`serve`、S1、内嵌的前端、S3、非 root、优雅停机、清理。
4. Makefile：`make image`、`make image-smoke`。
5. 持续集成：`image` 任务。
6. README：部署一节（镜像、先 `migrate up` 再 `serve`、探针、配置用环境变量）。

## 测试

- `make image-smoke` 本地与持续集成通过。
- 反向对照：去掉 `USER nonroot` → 失败；`VERSION` 不注入 → S3 的版本检查失败。

## 完成检查

`make check` 为绿；`make image-smoke` 本地与持续集成为绿。
