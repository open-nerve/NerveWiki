# M1/P6/S4 端到端：实施计划

上级：[P6 文档](../06-P6-settings.md) 3.8。

## 任务

1. `e2e/fixtures/settings-pages.ts`：`changePasswordWith`、`createTokenWith`、`holdAnswer`、`registerOnboarded`。
2. A7、A8、A10、A11 的页面版本（第 3.8 节的表）。
3. README 的前端一节：设置页。
4. 核对 M1 第 3 节的故事表：每个故事的每个版本都有对应的测试。

## 测试

- `make e2e` 全部通过；`--repeat-each 3` 与 `--workers 1` 各一轮。
- 反向对照：设置页的偏好不写设备 → A8 失败；停用之后不结束本地会话 → A11 失败；令牌在"完成"之后仍在页面上 → A10 失败。

## 完成检查

`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿。
