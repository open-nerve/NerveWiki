```yaml
status: open
from: M6 收尾
to: M10
created: 2026-10-06
```

# lint 的数据在链接索引里

总体设计 5.6 的 lint 规则大多由 DB 查询算出。它们要的数据 M6 已经写进链接索引（[M6 总设计](../../M6-links/00-M6-design.md) 4.3；表在总体设计 7.2 的 `linking`），只记在 M6 的文档里（[M6 收尾审查](../../M6-links/reviews/M6-closeout-review.md) C-Q1）：

| lint 规则 | 索引里的数据 |
|---|---|
| 待建页面 | `page_links.resolved_id IS NULL` 的链接，按 `target_key`（目标最后一段的标题键）分组计数；`target_key` 为 `NULL` 的目标无论树怎样都解析不到 |
| 孤儿页 | 没有别的页的 `page_links.resolved_id` 指向它（反链的查询，`linking/app/backlinks.go`） |
| 链接有歧义 | `page_links.ambiguous`（几页在每一项偏好上都相同，按 id 选了一页） |
| frontmatter 解析失败 | `indexed_pages.frontmatter_valid = false`（这时没有属性、frontmatter 的标签与别名；正文里的 `#标签` 照记） |
| `sources` 指向无效、未消化的来源 | 属性链接带属性路径：`page_links.property_key` 是 `sources.0` 这样的路径（只有整个值恰好是一个链接的字符串才算，总体设计 4.4 末） |
| 缺类型、缺摘要 | `page_properties` 的顶层键（`position` 是书写的次序，值是 jsonb） |

注意：

1. **索引不总是当前的**：`indexed_pages.revision` 与正文的不同（读正文与读索引之间有写），或 `extractor` 不是当前的 `domain.Extractor`（升级改了提取规则、还没 `nervewiki reindex`）时，这一页的行不可信。lint 跳过这样的页，或者像阅读视图那样即时解析（M6 总设计 4.7）。
2. **提取规则变了就提升 `domain.Extractor`**，并在发布说明里要求执行 `nervewiki reindex`。M10 若加提取的东西（例如 `type`、`summary` 单独成列），照此办。
3. **索引表不带外键、没有 `deleted_at`**：被删页的行由删除路径删掉，`checkLinks`（`bootstrap/links_test.go`）守住不变式。linking 的 SQL 不 JOIN page 的表，要节点的状态经 page 的读端口（M6 总设计 4.3）。
4. **来源区与模式**：哪些页在来源区，由 M10 的笔记本模式决定；索引只给出链接解析到哪一页。

## 处理进展

- P3来源修订提案（2026-10-30；[新记录](../reviews/P3-origin-design-review.md)）：主持者的决定（负责人可以改判）62拟在S1-0a保留Link.InFrontmatter已有来源事实、writer/generated与Extractor 1→2同提交，管理员用新版reindex；当前实现仍是1，M6原空键限制仍为当前事实。注意1在P3选择跳过/计数并提示管理员，不在lint parse；注意2的版本边界含持久化表示。PageProperties保持IS NOT NULL，v2空key可见，View/rewrite/batch/backlink及搜索队列边界按P3§3.0；S1-2再交真实D/F四kind与原六行。原M6说明/README待实施时更新，不把计划当完成；六行及注意1–4全部验收前仍open。

- M10 开工（2026-10-24）：照[M10 总设计](../00-M10-design.md)第 7 节"移交的落实"的"M6 lint 的数据在链接索引里"两行（表的六行、注意 1–4）做，在 P3 的 S1（4.11）。仍是 `open`，做完的 Phase 改为 `done`，收尾核对。
- P2/S1（2026-10-27，合并见[00号文档](../00-M10-design.md)第12节）：S1仅提供写入契约的来源事实/rewrite，未实现本移交的整区lint读取/索引规则；六行及注意项仍归P3，状态open。S1范围见[P2结果](../02-P2-source-area.md)10.1。

- P2/S3（2026-10-29）：本轮只完成来源网页身份、系统rewrite既有契约消费和真实mode变化；未实现整区lint读取、链接索引规则或健康度。六行和注意1–4仍归P3，open保持；[P2结果](../02-P2-source-area.md#103-s3-完整来源网页)不替代此移交验收。
