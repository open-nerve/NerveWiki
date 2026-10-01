# M2/P1/S3 工作区的数据与规则：实施计划

上级：[P1 文档](../01-P1-access-workspaces.md) 3.4、3.5、3.6。

## 任务

1. 迁移 `00006_workspace_workspaces.sql`、`00007_workspace_workspace_members.sql`，列、约束名、索引照 3.5。
2. `sqlc.yaml` 加 workspace 一条；`runtime-grants.sql` 给两张表授权；`schema_test` 列出约束与索引名，加 CHECK 的反例。
3. `workspace/domain`：
   - `slug.go`：`CheckSlug`；
   - `reserved.go` 与 `reserved_slugs.txt`：三段，按调用解析；
   - `workspace.go`：新工作区的取值，一次列出全部字段问题；
   - `actions.go`：`ActionRead`、`Actions()`；
   - `errors.go`：`not_found`、`creation_disabled`、`slug_taken`。
4. `workspace/adapter/postgres`：
   - `queries/workspaces.sql`：插入工作区、插入成员、按 slug 读、我的工作区、slug 是否被占用、`RoleOf`；
   - 仓储，把 `workspaces_slug_key` 的违反翻译成领域错误。
5. `workspace/memberships.go`：`NewMemberships(pool)`，实现 access 的事实端口。
6. 一致性测试：
   - `bootstrap/reserved_test.go`：`[server]` 段 = 根路由器上 `/` 之外的顶层段，加 `assets`；
   - `web/apps/web/src/app/reserved-slugs.test.ts`：`[app]` 段 = `routes.tsx` 的顶层静态段，加 `public/` 的顶层目录。

## 测试

- slug 的表格：合法的边界（1、48 个字符，`_`、`-`）；大写、空白、49 个字符、非 ASCII、空串。
- 保留名单的解析：未知的段、段外的行、不合 slug 的名字、重复的名字，各自报错。
- 仓储：
  - 插入与 slug 冲突的翻译；删除之后 slug 可以再用；
  - 列表的顺序（名称，再按 `id`），过滤已结束与已删除的；
  - `RoleOf` 只认有效、未删除的行，并在事务内读。
- 迁移的上下往返；约束与索引名；CHECK 的反例。
- 反向对照：从 `[server]` 删掉 `readyz`，一致性测试失败；在路由里加一个顶层段而名单不跟上，vitest 失败。

## 完成检查

`make check`、`make gen-check` 为绿。
