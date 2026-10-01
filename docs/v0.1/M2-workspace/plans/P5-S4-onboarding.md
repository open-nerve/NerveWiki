# M2/P5/S4 引导的工作区一步：实施计划

上级：[P5 文档](../05-P5-web-shell-workspaces.md) 3.7。

## 任务

1. `onboarding/steps.ts` 追加 `workspace`；`onboarding/workspace-step.tsx`：
   - 已有工作区：挂载时记录一次（ref 守着），失败时显示原因与重试；
   - 没有、创建打开：`CreateWorkspaceForm`（"创建并继续"），创建之后列表不为空，由上一条记录；
   - 没有、创建关闭：说明与"继续"。
2. 文案（两种语言）。

## 测试

- 三种情况各一个；已有工作区时只记录一次；记录失败与重试；创建之后记录一次。
- 引导页的步数 "Step 2 of 2"。
- 反向对照：已有工作区时不自动继续、创建之后再调一次记录，测试失败。

## 完成检查

`make check` 为绿。
