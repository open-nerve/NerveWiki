```yaml
status: done
from: M7 收尾
to: M9
created: 2026-10-10
```

# 附件与 MCP

M7 加了附件节点、签名的下载地址与附件的链接写法（[M7 总设计](../../M7-assets-transfer/00-M7-design.md) 4.2、4.5、4.7）。M9 的 MCP 工具读写笔记本时碰到它们（M7 总设计第 7 节"写给后面的 M"；[M7 收尾审查](../../M7-assets-transfer/reviews/M7-closeout-review.md) C-I1）。

1. **`read` 读附件**：附件的内容只经签名地址下载（`GET /api/v0/assets/{node_id}/content?…`，读权限核对之后签出，至多两小时，每小时一变）。MCP 的 `read` 要定附件给什么：元数据（名称、类型、大小、SHA-256）加一个地址，还是直接给内容（图片给模型时要限大小与类型）。
2. **对外的基地址**：签名地址是相对的（`/api/v0/…`），浏览器按本站补全；MCP 的客户端在别处，要绝对地址。要一个配置的对外基地址（实例的公开 URL），或者 MCP 一侧按请求的 `Host` 补全（经反向代理时要信任的转发头）。
3. **附件的路径写法**：解析的规则见总体设计 4.4 与样例 `tools/md-fixtures/resolve/` 016–027：笔记本里有名称等于最后一段的附件时只读作附件（找不到即解析不到）；名叫 `x.png` 的页面写作 `x.png.md`；没有扩展名的附件没有写法（链接为 null，补全不列，[P3 文档](../../M7-assets-transfer/03-P3-assets-links.md) 4.3）。MCP 按路径找节点、写链接时照这套规则，不另起一套。
4. **写**：MCP 不上传附件（v0.1 的范围）；它写的正文里指向附件的链接经同一个观察者进索引，改名、移动附件时经参与者改写（M7 P3）。

## 处理进展

- M9/P1、P2（2026-10-18、19，合并 `a36e92df`、`804c3561`）：第 1 项：`read` 给附件的元数据与签名地址，PNG、JPEG、GIF、WebP 在 3 MiB、每边 8,000 像素之内给图片本身（`page/domain/read.go` 的 `InlineImage`），SVG 与别的给下载的链接；字节经 asset 的 `AttachmentReads`，没有宽高的 WebP 从文件头读（`asset/adapter/sniff/webp.go`）；`TestTheMCPReadsGiveAttachments`（[M9 总设计](../00-M9-design.md) 4.9）。第 2 项：`server.public_url`，不按 `Host` 补全（M9 总设计第 11 节设计的取舍第 7 条）；为空时图片照样内联（`TestTheMCPReadsWithoutAPublicURL`）。第 3 项：MCP 的路径是寻址，与链接的规则不同的四处见 M9 总设计 4.8 的表（取舍第 6 条），正文里的链接仍照 M7 的规则。第 4 项：不上传；指向附件的链接经同一个观察者与参与者（`TestTheMCPWritesReachTheUseCases`、MC4）。
- M9 收尾（2026-10-23）：核对过各项的落实（[M9 收尾审查](../reviews/M9-closeout-review.md)"handoff"，C 逐项对照代码），关闭。
