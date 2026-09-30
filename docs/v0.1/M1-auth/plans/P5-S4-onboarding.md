# M1/P5/S4 新手引导：实施计划

上级：[P5 文档](../05-P5-web-session.md) 3.7。

## 任务

1. `onboarding/steps.ts`：`OnboardingStep`、`onboardingSteps`、`pendingSteps(me)`。
2. `onboarding/profile-step.tsx`：显示名（预填），"继续"：改过就 `updateMe`，再记录 `profile`。
3. `pages/onboarding.tsx`：第 i 步共 n 步，显示第一个未完成的步骤，完成之后去 `next` 或 `/`。
4. `Onboarded` 守卫，接进路由表；已完成访问 `/onboarding` 直接离开。
5. 文案。

## 测试

- 注册表：id 的格式、不重复、不超过 32 个。
- `pendingSteps`：顺序、忽略服务端多出的 id、为空即完成。
- 引导页：记录之后显示下一步（测试注册两步）；完成之后离开并带上 `next`；422 显示在字段下方、停在原步；没改显示名时不发 `updateMe`。
- `Onboarded`：未完成去 `/onboarding?next=`。
- 反向对照：`pendingSteps` 不按注册表顺序 → 测试失败。

## 完成检查

`make check` 为绿。
