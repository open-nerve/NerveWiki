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
6. **贡献者的输出有界**（[M7 Codex 评审](../../M7-assets-transfer/reviews/M7-codex-review.md) D2）：`ExportSink.Add(path, content)` 要调用方先备好整份内容，贡献者在导出的快照里依次运行，`Add` 本身不限字节与文件数，大的虚拟 index、log 会拉长快照。注册第一个贡献者时定下它的输出有界（字节、文件数、生成时间），写明上限、由贡献者守住、测试钉住（总体设计 13.1 第 31 条）；真要很大的输出时，再加带上下文的流式写入与统一的额度，不让每个贡献者各自实现取消与预算。

## 处理进展

- M10 开工（2026-10-24）：照[M10 总设计](../00-M10-design.md)第 7 节"移交的落实"的"M7 导出贡献者、附件与来源区"一行做：第 1、2、6 项（贡献者、导入的跳过与加序号、输出的界）在 P4 的 S1，第 3 项（去重）在 P2 的 S2，第 5 项（lint 不把附件当页）在 P3 的 S1，第 4 项照今天、不另做（4.8、4.11、4.13）。仍是 `open`，做完的 Phase 改为 `done`，收尾核对。
- P2/S1（2026-10-27，合并见[00号文档](../00-M10-design.md)第12节）：S1已给附件树管理携Kind/祖先事实，未实现第3项SHA256去重；该项仍P2/S2，第5项lint仍P3，其余贡献者/导出仍P4，状态open。S1范围见[P2结果](../02-P2-source-area.md)10.1。
- P2/S2：第3项逐字节SHA256去重已在上传/移入/恢复、导入重复报告及后代保留的增量中核验，页与附件不跨kind；实施已验、等待最终S2签字。第5项lint留P3，导出/虚拟/贡献者留P4，整份open。 代码已推进至 `33b4da5a`，本机与精确分支11项CI已验；B-N1原审已关闭，main集成未决状态见[P2 §10.2](../02-P2-source-area.md#102-s2-来源-hash去重与验证)与[S2审查](../reviews/P2-S2-source-hash-review.md)。
