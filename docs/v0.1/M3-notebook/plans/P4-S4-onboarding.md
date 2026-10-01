# M3/P4/S4 引导的笔记本一步：实施计划

上级：[P4 文档](../04-P4-web-notebooks.md) 3.6；[M2 移交](../handoffs/M2-workspace.md)第 5 项第二点。

## 任务

1. `app/landing.ts`：`notebookTarget(list, last)`。
2. `onboarding/notebook-step.tsx`：没有目标时说明并继续；已有看得到的笔记本时继续（`GoOn` 从 `workspace-step.tsx` 导出共用）；否则建私密笔记本，名称缺省"我的笔记"。`onboarding/steps.ts` 加第三步。
3. e2e：`fixtures/auth.ts` 的 `onboardingSteps` 加 `notebook`；`fixtures/onboarding-pages.ts` 加笔记本一步的定位与操作；A9（三步）、W11（工作区一步之后经过笔记本一步）、A4、W7 随之调整。

## 测试

- `notebookTarget` 的表格：最后访问的是成员、是访客；按名称的第一个；处处是访客；没有工作区。
- 引导一步：没有目标、已有笔记本直接继续、新建之后继续、完成失败之后重试不多建、读不到时说明与重试。
- 反向对照：已有笔记本时仍建 → 测试失败；目标取了访客的工作区 → 表格失败。

## 完成检查

`make check` 为绿；`make e2e` 为绿。

## 实现注记

- `GoOn` 单独成 `onboarding/go-on.tsx`。
- A4 不用改：它停在工作区一步。工作区一步的测试把笔记本一步记为已完成，只看工作区一步。
