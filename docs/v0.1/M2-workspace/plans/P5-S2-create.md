# M2/P5/S2 创建工作区：实施计划

上级：[P5 文档](../05-P5-web-shell-workspaces.md) 3.5。

## 任务

1. `app/slug.ts`：`slugProblem(slug)`（与服务端 `ValidSlug` 相同的格式）、`slugFrom(name)`。
2. `app/create-workspace-form.tsx`：名称与 slug；slug 随名称直到被改；格式通过、停止输入 300 ms 之后经 SWR 检查可用性；`useForm` 的 `onField` 把 `workspace.slug_taken` 放到 slug 下方；成功之后 `onCreated`。
3. `pages/create-workspace.tsx`：创建打开时是表单，成功之后转到 `/:slug`；关闭时只有说明（回到 `/` 用顶栏的 Nerve Wiki）。替换 S1 的占位。
4. 文案（两种语言）。

## 测试

- `slugProblem`、`slugFrom` 的表格。
- 表单：名称与 slug 的本地检查不发请求；slug 随名称、改过之后不再跟随；可用性的三种答复，格式不对时不请求；409 在 slug 下方；422 在各自字段下方；403 在上方；成功之后进入工作区，左栏有它。
- 创建关闭时没有表单。
- 反向对照：slug 改过之后仍随名称、409 不放到字段下方，各自的测试失败。

## 完成检查

`make check` 为绿。
