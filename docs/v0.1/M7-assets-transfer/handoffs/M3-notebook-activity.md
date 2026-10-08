```yaml
status: open
from: M3
to: M7
created: 2026-10-02
```

# 附件作为笔记本活动的来源

M3 建了只读的扩展点 `NotebookActivitySource`（[M3 总设计](../../M3-notebook/00-M3-design.md)第 8 节；总体设计 13.1 第 21 条）：注册者按一组笔记本 id 返回各自的字节数与最后写入的时刻，无主清单的"大小"是各来源字节数之和，"最后更新"取最晚的。M3 没有注册者，组合根交的是空集合；模块根的 `notebook/activity_test.go` 证明经 `notebook.Deps.ActivitySources` 交进去的来源到达清单。M4 注册页面的来源（[M3→M4 移交](../../M4-pages/handoffs/M3-notebooks.md)第 5 项）。

M7 注册附件的来源时：

1. **字节数**是这本笔记本里未删除的附件的大小之和，**最后写入**是它们最晚的上传时刻；在调用方的读取里执行，不加锁。
2. **整个程序的行为测试**：经 `bootstrap` 的 `notebookDeps` 接线之后，`GET /workspaces/{slug}/ownerless-notebooks` 的 `size_bytes` 与 `last_activity_at` 算上附件；组合根没交这个来源时它失败（M3 收尾审查 A-M1）。

## 处理进展

- M7/P2（2026-10-09，合并 `48c62c0`）：第 2 项完成（`bootstrap/assets_lifecycle_test.go` 经无主列表核对大小，组合根交空时失败）。第 1 项改为只报字节数：上传是树的一个单元，它的变更集已经由页面的来源算作同一时刻的写，附件不再另报最晚的上传（P2 审查 C3，[M7 总设计](../00-M7-design.md) 4.6）。M7 收尾时关闭。
