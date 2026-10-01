# M3/P5/S1 数据：实施计划

上级：[P5 文档](../05-P5-web-ownerless.md) 3.2；13.2 第 1、15 条。

## 任务

1. `services/ownerless.service.ts`：`OwnerlessService`（list、takeOver、remove、auditEvents），转出 `OwnerlessNotebook`、`NotebookAuditEvent`。
2. `stores/ownerless.store.ts`：`OwnerlessStore`（load、takeOver、remove；404 移出并抛出；同一本一次一个）。
3. `stores/audit.store.ts`：`AuditStore`（load 第一页、more 接在后面并去重、重读丢弃在途的 more）。
4. `stores/root.store.ts`：`ownerlessOf(workspace)`、`auditOf(workspace)`；`context.tsx` 的钩子随用到它们的 S2 加（knip）。
5. `i18n/format.ts`：`formatBytes(bytes, locale)`（B、KB、MB、GB，1024 进位，按语言的数字格式）。

## 测试

- `OwnerlessStore` 的表格：重叠的读丢弃；接管、删除移出；答 404 移出并抛出；答 403 保留并抛出；同一本的两个写按序发出。
- `AuditStore`：第一页、`more` 去重、最后一页没有游标、重读回到第一页、重读期间在途的 `more` 被丢弃。
- `root.store.test.ts`：两个缓存。
- `formatBytes` 的表格。
- 反向对照：404 当作成功 → 测试失败；`more` 不去重 → 测试失败；重读不丢弃在途的 `more` → 测试失败。

## 完成检查

`make check` 为绿。

## 实现注记

- 审查修复（P5 审查 M2、Q2）：`more` 的页按游标接上，重读接得上时保留尾部；`series` 改为只管第一页的先后（`reads`）。
