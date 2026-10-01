# M2/P5/S5 端到端：实施计划

上级：[P5 文档](../05-P5-web-shell-workspaces.md) 3.10。

## 任务

1. 夹具：`fixtures/workspace-pages.ts`；`fixtures/auth.ts` 的 `onboardingSteps` 加 `workspace`。
2. 故事：
   - W1、W2、W4 加页面版本；
   - W3 新写（页面、接口）；
   - W11 新写（页面三种情况、接口）。
3. 调整：S2（落在创建页、用户菜单的版本）、A3（`expectCreatePage`）、A4、A6（账户没有完成引导，落在引导的第二步）、A9（步数与第 2 步）；`stepRecorded` 按答复等记录。

## 测试

- 新故事另跑 `--repeat-each 3`；全部故事通过。
- 反向对照：落点不看最后访问，W3 失败；创建关闭时仍有入口，W2 失败；引导的一步不自动继续，W11 失败。

## 完成检查

`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿。
