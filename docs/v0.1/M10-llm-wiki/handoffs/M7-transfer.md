```yaml
status: open
from: M7 收尾
to: M10
created: 2026-10-10
```

# 导出贡献者、附件与来源区

M7 建立了导出贡献者的扩展点，交空（[M7 总设计](../../M7-assets-transfer/00-M7-design.md)第 8 节；[P5 文档](../../M7-assets-transfer/05-P5-export.md) 3.10）；附件进了树与链接（4.2、4.7）。M10 注册贡献者、做来源区与 lint 时碰到它们（M7 总设计第 7 节"写给后面的 M"；[M7 收尾审查](../../M7-assets-transfer/reviews/M7-closeout-review.md) C-I1）。

1. **导出贡献者的最后一跳**（总体设计 13.1 第 21 条点名这一项）：`transfer.ExportContributor` 的 `Contribute(ctx, scope, sink)` 在导出的快照里运行，`ExportSink.Add(path, content)` 加的文件进 zip、路径记进 `meta.json` 的 `contributed`；与库里的路径冲突时导出失败 `contributor_conflict`，第一个错误即停。M7 只有模块根的 `transfer/contributors_test.go`（真库与 River）。M10 注册之后，在整个程序上加行为测试：经 `bootstrap` 接线的应用导出一本笔记本，zip 里有贡献者的文件、`meta.json` 列出它；组合根交空时这条测试失败。
2. **贡献者的文件在导入时不导入**：导入读到 `meta.json` 的 `contributed` 时跳过那些路径、不写进报告（[P6 文档](../../M7-assets-transfer/06-P6-import.md) 3.11）。M10 注册贡献者时核对这一条仍对（例如它的文件放在库的哪一层、`meta.json` 只认第一个）。
3. **来源区按 SHA-256 去重**：附件的行记着 SHA-256（`asset_blobs.sha256`），M7 不去重（同一个文件传两次是两个附件）。来源区要去重时按它；建附件的那一步交给写入守卫的 `Step.Asset`（服务端测定的类型、大小、SHA-256）就是为 M10 的守卫留的（`page/app/extension.go`）。
4. **来源区完整列出附件**：页面树只列页面，附件在各页的附件一节（总体设计 3.7）；来源区页面要完整列出附件，是这条的例外。
5. **lint 分开页面与附件**：链接索引的 `page_links.resolved_asset` 标出解析到附件的链接（迁移 `00028`）。lint 的"断链""孤立页"之类的查询按它分开，不把附件当作页、不把指向附件的链接算成指向页的。
