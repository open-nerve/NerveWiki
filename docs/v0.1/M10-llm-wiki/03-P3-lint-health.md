# M10/P3 lint 与健康度：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M10/P3；两个 Step：S1 服务端与协议；S2 健康度网页 |
| 日期 | 2026-10-29 |
| 状态 | 原49–60及决定62设计已接受，设计自身CI与S1入口已核；S1实施中。S1-0容量调整已提交89d36454；S1-0a已实现来源表示与Extractor 2，结果和余项见§10.1。S1/S2未完成，三审、完整成本、最终门禁与merge/record两main仍待；P4/P5/M11未开始 |
| 原设计精确基线 | `main = origin/main = f6bd0cdee2b7570252a43581cf71cd1e71592f76`；P2 已完成 |
| 上级 | [M10 总设计](00-M10-design.md) §4.10–4.12、§5、§7–9；[总体设计](../v0.1-design.md) §5.6、§13；[推进方式](../claude_to_codex/02-workflow.md)；[P2 移交](02-P2-source-area.md#9-给后面-phase-与-m-的移交) |
| 记法 | `page/`、`linking/`、`wiki/` 在 `server/internal/modules/` 下；`bootstrap/`、`platform/`、`shared/` 在 `server/internal/` 下；`W/` 是 `web/apps/web/src/`。本文拟新增的文件、类型、测试名均是设计，不冒称已有实现 |
| 决定 | 主持者接受连续实施决定49–60（负责人可以改判），见[设计审查记录](reviews/P3-design-review.md)；索引、容量及所有运行结果仍须完成本文实测准入。见§2.2 |

**来源表示前置（S1-0a）**：writer沿已有 `Link.InFrontmatter()` 传等长bool，实际生成SQL用CASE保留正文NULL与空属性非NULL空串，当前Extractor为2；M6/README/API说明同步。S63/S64/S65/S68的验证与代表成本集中记在§10.1，S66/S67/S69仍归S1-2；原79行总账及S1最终复跑不改。[来源修订记录](reviews/P3-origin-design-review.md)保留决定62、三审、搜索校准与原失败历史。

## 0. 两个可独立合并为绿的 Step

| Step | 交付 | 单独为绿的边界 |
|---|---|---|
| S1，`m10-p3-s1` | 来源表示/Extractor 2兼容前置；来源整区 metadata；linking 八条规则、sources 索引、流式分批读取；wiki 快照、名额、期限与 singleflight；两个 REST 操作；两入口的 lint/pending_sources；权限矩阵、说明余量、生成物、错误双语、真实契约 fake；LW8 API 与两个 MCP scope；计划/物理工作/成本及容量闭环 | 服务端新操作全部可达；S1 自带前端生成类型、错误字典、fake 与其测试，尚无健康入口，不引用 S2 才存在的导出 |
| S2，`m10-p3-s2` | 健康路由、导航、摘要/每卡分页、刷新世代、模式提示、路径与附件落点、双语、LW8 page | 只消费已合并 S1 的契约；沿现有 UI 组件；搜索与全局分页默认行为不变 |

每Step的每个提交都以单独build/vet/lint/short为交付要求；涉及 Web 时同时 tsc/knip/vitest。每 Step 经三审、修复复核、全部运行时变异、门禁、分支 CI、no-ff 合并 CI、record CI 后才释放后一步。只完成设计不是 S1 完成。

## 1. 基线与入口证据

P2完成提交`f6bd0cde…`的[push workflow 40861860430](https://github.com/open-nerve/NerveWiki/actions/runs/40861860430) attempt1，全11 jobs及必需步骤成功。强接受原件为`ci-p2done-f6bd0cde/root-ci-acceptance.json`；P3释放原件为`root-p2-completion-and-p3-release-f6bd.json`，SHA-256 `67fa89c356c85bcb4e08beb5789aecd2d43d6a724f672eb1e848a47a025500d8`。本文按 root 已核的精确 main 编写，不把 S3 分支树相同当 Phase 已完成。

**设计后的入口维护（以下保留601c写入时的历史状态；该记录自身CI现已接受，见本文首段及来源修订记录）**：本设计落库提交 `9e38a38f648ffa19130afb8a8b468b792dad1381` 的 [CI 40897835721](https://github.com/open-nerve/NerveWiki/actions/runs/40897835721) attempt1实际失败：既有M8搜索守卫generic `go`首页40.621258 ms超过原40 ms界，后八条costs命令NOT_RUN，e2e/image skipped。原设计审查接受的是冻结设计语义，并不构成这次运行通过。主持者实施决定61选择有明确来源与检测取舍的整表参考维护；文档 `aa860be1b32bb98410e6a6811d2d87569de78b25` / CI40921122246 attempt1，以及实际no-ff合并 `b957fdbe0367a9a6ff0f65d2afc303db49a755c2` / 首次main CI40925905708 attempt1，均已分别由主持者按精确对象接受，见[搜索守卫校准记录第7节](reviews/search-guard-calibration-review.md#7-维护文档合并与main-ci)。**本维护记录自身提交/CI仍待产生；新发现的合法空名称frontmatter链接来源丢失问题仍开放，决定62的具体设计修订与三位独立审查待完成。P3/S1仍未放行，后续须绑定该修订的实际设计提交/CI，并由主持者重新绑定P3放行回执与S1-0候选**；不把9e38的原失败attempt写成绿。该维护记录当时不改本文设计决定49–60、SQL/契约/预算、容量阈值、变异与所有实施准入，新增来源修订当时尚未落入本文；本提案现仅按决定62明确列出的来源表示/兼容合同作修订，其余保持，具体第二稿现已完成三位独审及原席复核并由主持者接受设计语义；最终事实稿投影/实际设计提交自身CI/产品入口仍待，S1/S2结果仍待实施。

已完整阅读三份只读调查及 authority/capacity 补充，核对关键实际源码；调查的旧头为 f6b397b7/ecd79c77，主持者已在 f6bd 验证相关代码未变。当前事实：

- `page.SourceArea` 只有 Forget；SourceHeads 含 trash、ProbeSources 只按 hash 匹配，二者均不是整区枚举；source_hash 非 NULL 也不是来源资格。
- linking 已保存完整 target、range_start、kind、property_key、resolved_asset、frontmatter_valid/extractor 和完整属性 JSON；TitleKey 是 NFC→完整 Unicode fold→NFC。
- wiki 没有 lint HTTP/MCP 模块，没有 `api/modules/wiki.yaml` 或 health 页；P2 没做这些不是缺陷。
- sqlc `:many` 会累积 slice；模块禁止自行 Query/raw SQL。本文使用真正有行数/字节界的多次 sqlc 读，不能把全量 `:many` 改名为 stream。
- 本稿没有执行测试、build、PG/container、网络或读写客户端配置；附录引用的耗时均为已接受历史样本。

## 2. 目标、范围与上级修订

读者打开健康页或调用工具时，得到同一快照的提示；普通模式四条、LLM Wiki 八条。不拒绝任何正常写入，不维护写时计数。只读 page metadata、linking 已存索引及最终返回附件的 metadata；lint请求不读正文、不即时Markdown parse、不reindex。决定62另设S1-0a来源表示前置：复用已有事实，把真实空属性路径保为非NULL空串，writer/generated与Extractor 1→2同一提交；存量由管理员用新版命令重建，不能把此离线操作放进lint请求。来源表示前置已实现，当前为v2，见§3.0/§10.1；本段其余lint行为仍待后续S1。

不做 P4 的虚拟文件/目录/ingest，不做 P5 prompt/系统说明段，不改真实客户端配置，不做 M11 升级，不顺手改搜索或全局分页交互。

### 2.1 对 00 的明确修订

1. §4.11 wanted 的“SQL raw 计数后 Go 合组”改为 §3.6 的有界边读取、按 source 局部集合做 union；不能相加 raw COUNT DISTINCT。明确真实读取是本本合格未解析边与其目标字节，不谎称只读唯一 raw 字符串。
2. 分组身份是完整 canonical；新增 wire `item_key`，公开游标有版本/规则/范围及有界排序载荷。listLintItems 另带 `mode/computed_at/stale_pages`，供不同快照的分页与模式提示；REST 每项也带 cursor。
3. 核定 MaxLintRuns、LintTimeout 及每 caller 的剩余期限预算；第一条快照语句仍读实际 mode/root；显式范围不能与隐式范围合并。
4. Web 每次进入含 Back/Forward/从条目返回；location.key 之外加一次挂载/进入的 nonce，刷新另有 generation。新 summary 成功后折回各卡的 5 项；失败保留旧数据且明确错误。
5. source 页对附件的关系仍读索引；多页引用采用“任何已消化来源页则消化，否则由任何非空、当前 extractor 的待消化来源页代表”的规则，详见 §3.8。
6. `page_properties_sources_idx` 必做。普通复合 notebook/source/range 索引仅是流式读取的测量候选，先量再加；禁止 resolved_id IS NULL 部分索引。不能把候选写成已批准迁移。
7. 总体设计 §13.4#8 当前分片句同步实际 Makefile，保留历史失败/慢样本；P3 移界另有正式决定与新实测，不用文字同步冒充布局已变。
8. 总体 §5.4 与00 §4.11 两条 sources/附件规则同步首键合法 slot、先数全部匹配边唯一性、L/J身份、current非空来源父页及多父any语义；根/AGENTS坏sources照查，不扩大豁免。
9. 00 §4.14、§8 linking扩展点及P4阶段合同改为§3.8.1的 OldQualifiedFacts；删旧索引前不等于树改动前。旧JSON形状/匹配边与unit起点资格必须一并保全，未改页混合改树的近似仍保留。P3只交可复用纯判定与交接，不实施ingest。
10. 00 §4.11/§4.18、API表、§7/8/9/12/13及头状态同步真正接口/运行owner/Stop；MCP notebook入口Gate的HTTP404在tool之前，account及gate后用例才按内部码次序。
11. P1/P2 §9在实际P3/S1完成后追加承接证据；设计接受时仅补“合同已细化、实现待P3”，不提前写完成。全部语义替换与未来完成投影按§8分开。

12. 主持者的决定（负责人可以改判）62：明确推翻整个P3“不升Extractor/不需reindex”的旧范围承诺，仅为保存已有的body/property区别。00§4.10/4.11、M6移交注意2与总体§13.3#6同步；原sources深度/位置丢失、MaxLinks、正文限制和parser保持。F明确wikilink/link/embed/image；PageProperties保留IS NOT NULL，v2新写/重建后key=""进入原API/面板。P4补旧事实Unknown与未改页coverage，不能以空union推新ingest。S1-0a已同步M6/README/API的当前格式说明；原审查历史保留，实测与剩余完成义务分别见§10.1/§8.1a。

### 2.2 需要负责人确认

无。用户已授权M10内的小选择由主持者决定。SQL/算法、游标、容量候选与Web九项选择已按实施决定49–60完成原席设计复核，见[设计审查记录](reviews/P3-design-review.md)；真正无法满足本Phase范围的架构冲突再报告，不能擅自降级成“实施时再想”。

主持者的决定（负责人可以改判，连续实施编号，不覆盖00总设计1–40系列）：49 wanted有界source union；50索引测量准入；51 item_key/有界游标/StartCursor/LJ身份；52附件跟随；53运行预算/flight/交付前再授权；54 MCP B/C容量候选；55 Web九项；56无rule却给limit/cursor的组合参数422；57各规则独立豁免、root/AGENTS真实坏sources仍查。58 wiki 唯一 Runs owner 的幂等 Stop/recover 与 bootstrap 全出口收尾；59 P4 在删旧索引前捕获旧合格引用、旧发出页资格来自 unit pre-change facts；60 从经 Planned 的 compactNodes 构建候选 kind 存在表，不调用 ByKeys。root于2026-10-29核定49–57进入三审、58–60用于第一轮修订；不能据此写成原席已关闭、产品、性能或CI已经验收。

**主持者的决定（负责人可以改判）62**：采用现有nullable property_key的NULL/空串区分、同提交Extractor 2、明确管理员reindex和已有读者/API兼容；S1-0之后先S1-0a，新增S63–S69而不改原72行；P4无法证明旧事实/未改页覆盖时不新增无法证明的标记，正常编辑继续。方向由root核定，本次具体第二稿经三位独审及原席复核后已由主持者接受设计语义；b02设计自身CI与S1入口已接受；S1-0a来源表示前置结果见§10.1，S1/S2、完整成本与P3收尾仍待。

### 2.3 方案比较

- **推荐：有界 sqlc keyset + source 局部集合。** 完整 Unicode 规范化仍用 Go 唯一实现；source 集合只留当前一页，raw 关联不进全局内存。代价是读取本本合格边、分批往返及可能需要一个经测量证明的复合索引。
- **不选：SQL lower/SQL 自写 Unicode TitleKey。** 与现有完整折叠及流安全 NFC 的版本差异会造错组，且要长期维护两套规范化。
- **不选：raw 聚合加 source_ids 数组后 Go 合组。** 能修正 distinct，但全量来源关联集合随边数增长；不能满足内存承诺。
- **退路只用于比较测量，不默认交付：本本 source ids 一次读完，逐页按 PK 读链接。** 能保证 notebook 隔离且无新索引，但 10 万页可能 10 万次往返；若候选 keyset 的物理范围/写入代价不接受，必须重新审查该取舍，不偷偷提高期限。

来源表示另比较三种：推荐复用nullable字段，以等长bool参数保存Link.InFrontmatter的已有事实；新增origin列仍不能从旧NULL恢复来源，也需未知状态/版本及重建，增加DDL和重复不变量，无当前需求；保留NULL近似虽省重建，却会把空属性算正文，错误消化D/F并污染P4旧union，不采用。仅改writer而不升版本、把旧行UPDATE为2、设非法0都不正确。该修正有全量旧页stale和升级成本，但利用已有索引/版本/事务重建机制，属于局部索引表示修正；无重大未解架构冲突。

## 3. 服务端

### 3.0 来源表示与版本兼容前置（S1-0a；决定62）

当前链为PageFacts保留Property/Quote→Link.InFrontmatter识别合法空键属性→linksParams逐边传等长InFrontmatter bool→实际InsertLinks用CASE保存属性原路径或正文NULL；Extractor为2，parser及合法空YAML键不变。M6/P3A的L6保留版本1当时的处置，M6当前格式说明已同步；现有读者/重建验证见§10.1，D/F规则与P4仍待各自实施。

| 输入已有事实 | 当前v2的property_key | origin读义 |
|---|---|---|
| Link.InFrontmatter()==false | SQL NULL | 真实正文 |
| true且Property=="" | 非NULL空字符串 | 真实空路径属性，不能计body |
| true且Property非空 | 原完整路径 | 属性；是否首sources slot仍按§3.8 |

linksParams逐边追加InFrontmatter bool输入；它与ranges/kinds/property_keys/targets等所有数组等长、同序。InsertLinks仅把property_key的NULLIF改为CASE WHEN u.in_frontmatter THEN u.property_key ELSE NULL::text END；anchor/display/target-key的原NULLIF不改。sqlc输入及generated SQL与writer、domain.Extractor=2同一完整提交；版本描述包括“持久化事实的表示”，即使Extract产物没变也需这个边界。来源表示不加列/表/新授权/DDL；格式转换经管理员reindex。00026的property_key IS NOT NULL索引自动收空路径行，不重复造。SQL/Go的nullable值不能COALESCE或按空字符串判body。

旧v1的NULL仍有歧义，不能UPDATE冒充v2或用extractor=0（00020要求>=1）。所有旧indexed页对版本敏感读者成为stale；lint按§3.4计数/跳过，绝不请求时补parse。真实内容写/恢复经ReplacePage/AddPage升为v2；纯改树/重新解析resolution/普通no-op不必替换事实，不能当修复路径；来源通常只读，所以仍需管理员reindex。

| 现有消费者 | v2实施后的兼容边界；其余语义不改 |
|---|---|
| Views.Resolve | 版本或revision不匹配时用阅读已解析的链接重新resolve；不回写索引，不给lint开正文例外 |
| Rewrite.followed、BatchLinks.Indexed/Reached | 旧extractor跳过索引链接改写；batch另核revision。真正改写仍用重新解析的InFrontmatter |
| Backlinks | 保留旧索引的页/计数，不给非current行正文context |
| PageProperties | 三组IS NOT NULL投影保持：v2的key=""可返回并签asset地址，body不返回；v1空键仍遗漏直至重建 |
| IndexPages/IndexItems、inbound counts、tags/aliases、export LinkedPages、LinksReached/SetResolutions | 当前无统一Extractor过滤；本修订不扩大它们的过滤，resolution更新不使旧来源可信 |
| module.Extractor、Store.View/IndexedOf/Backlinks版本输送、checkLinks及fixture | 常量由domain公开；更新“当前=1”的期望，保留literal 1作为合法旧格式/DDL下界；不能一概替换全部1 |

PropertyLink.key已允许空字符串，Web的exact-path Map/queue也能用空键；这是明确的可见修正，不加字段、不改无关UI。S1-0a须真实REST与asset签名及Web正反证，不能以schema容许替代。sources key/path/depth/MaxLinks碰撞合同原样。

管理员以新版nervewiki reindex在已有单实例升级顺序中于migrate up后、恢复serve前执行。启动/auto_migrate不会自动修复Extractor。每本的**linking/rekey重建阶段**由Rebuild.Notebook在一个事务中执行：notebook FOR NO KEY UPDATE→index锁→rekey→删linking派生事实→逐页parse/AddPage→resolve→links事件；该事务持锁并阻该本写，成功则提交，本阶段失败/撞键不留下本阶段的部分改动。它不包住后续search入队。

bootstrap在Rebuild返回后另调SearchIndex.Requeue；除缺失本等既有提前返回路径外，ctx仍可用时，即便linking失败/撞键也尝试入队。Requeue按id找缺失/旧tokenizer搜索行，QueueSearch每批至多10000节点、各自事务提交；当前tokenizer不入队。后批失败不撤销前批，queue失败不回滚已提交的v2链接索引，linking的笔记本/index锁也不覆盖后续queue工作。命令在同一本内就可能部分成功，全实例亦非原子；不能从命令失败推断链接仍是v1或队列未变。普通失败下全本模式记录失败并继续其它本、最后返错；单本模式遇linking错误时，ctx可用仍先尝试queue，再连同queue错误返回；只queue失败或撞键最终也返错。缺失本、取消、输出错误等提前路径照bootstrap现实现，不改命令退出/继续行为。

旧v4搜索行可能触发README已有的后续重建代价；入队与之后serve的搜索job构建另分阶段。回退旧binary会视v2非current，旧writer可再造v1，重回新版须再重建；不允许混跑writer。这里不授权操作真实用户库。

必须实测测试自有库的v1→v2重建、写入代价及锁/失败/搜索队列边界，见§7.1/7.4。历史330秒只是旧估计，既有M6/M8数字不是本次测量；生产停机/WAL/磁盘与负载由M12真实环境复核。

### 3.1 文件与依赖方向

| 文件（新增除标“改”） | 所有权与职责 |
|---|---|
| linking/domain/facts.go、adapter/postgres/store.go、queries/links.sql及gen（改） | S1-0a的bool输入/CASE、同提交Extractor 2及来源注释；真实parser/存储与旧格式测试 |
| linking既有读取/API、bootstrap reindex/REST tests、W/pages/page/page-properties.test.tsx及必要schema说明（改） | v1兼容、v2空key API/asset/Web正反证；不加新公开字段 |
| `page/source_area.go`（改）、`page/adapter/postgres/lint_metadata.go`、`queries/lint_metadata.sql`、相应 gen | `SourceArea.Nodes` 的严格整区 metadata；`NewLintNodes(pool)` 的紧凑节点事实；仍只读 page 表 |
| `linking/domain/lint.go`、`lint_target.go`、`lint_sources.go` | 规则名、canonical、条目身份、sources JSON 判定；不依赖 wiki 或适配器 |
| `linking/app/lint_reads.go`、`linking/adapter/postgres/lint.go`、`queries/lint.sql`、`linking/lint.go` | 已决定范围后的窄读取端口；全部 SQL 经 sqlc/Planned；分批读取与规则聚合 |
| `00051_linking_lint_sources.sql`（合并时核号）、`server/sqlc.yaml`、`migrations/schema_test.go`（改） | sources 部分索引；可选复合索引按实测另一个迁移，不能预填 |
| `wiki/domain/lint.go`、`wiki/app/lint.go`、`lint_runs.go`、`ports.go`（改） | 对外值、访问次序、snapshot 编排、独立 bounded flight、immutable 结果 |
| `wiki/module.go`（改）、`wiki/adapter/http/{handler,lint}.go`、`adapter/http/gen/`、`wiki/adapter/mcp/{tools,lint}.go` | 新模块装配、REST 与 MCP 投影 |
| `bootstrap/{wire,mcp,wiki,app}.go`、permission_matrix 系列（改） | 字段逐个转换；真实路由/两个scope、同一Runs Stop全出口与close兜底；规则表与targetViolation |
| `api/modules/wiki.yaml`、`api/openapi.yaml`、`api/mcp/tools.json`、生成物 | 两个 REST 与两工具的先行契约；所有新码完整声明 |
| `page/adapter/mcp/instructions.go`、测试（改）；README 配置段（改） | 缩共用内置段并加入两工具；每入口至少剩 300 SDK 字节；只解释 lint 运行预算 |

wiki 不建表、不写跨模块 SQL。page/linking/asset/notebook 端口均服从 ctx 中同一个 snapshot；bootstrap 转换自己的窄 DTO，不把另一模块 app/domain 导入 wiki。

### 3.2 端口与范围

以下为拟定签名，实施导出的 alias 在模块根，内部类型各模块自有。保持一次运行唯一 owner；app 不持 pgx.Rows。

```go
type LintScope struct {
    NotebookID uuid.UUID
    Contract *Mode           // nil = 隐式；非 nil = 内部显式契约
    AreaRootID *uuid.UUID     // 隐式必须 nil；显式 LLM 必须非 nil
}
type LintQuery struct { Rule *string; Limit *int; Cursor *string }
type Lint struct { /* Access, Notebooks, Sources, Nodes, Reads, Assets, Paths, Snapshots, Runs, Clock, Logger */ }
func (l *Lint) Execute(ctx context.Context, s LintScope, q LintQuery) (LintResult, error)

// notebook adapter 第一条快照语句；未找到是 notebook.not_found。
type NotebookState struct { Mode Mode; SourceRootID uuid.UUID }
type Notebooks interface {
    WorkspaceOf(context.Context, uuid.UUID) (uuid.UUID, bool, error)
    StateOf(context.Context, uuid.UUID) (NotebookState, bool, error)
}
type SourceNode struct {
    ID uuid.UUID; Kind NodeKind; CreatedAt time.Time
    PageBytes int64 // 仅 page；asset 字节不在此端口伪装成 0
}
func (s page.SourceArea) Nodes(ctx context.Context, notebook, root uuid.UUID) ([]page.SourceNode, error)

type LintNode struct {
    ID uuid.UUID; ParentID *uuid.UUID; Kind NodeKind
    NameKey string; Linkable bool; CreatedAt time.Time; PageBytes int64
}
// 与展示路径不同的紧凑树事实；只有标题键，无正文与展示名。
func page.NewLintNodes(pool *pgxpool.Pool) page.LintNodes
// LintNodes.Nodes(ctx, notebook) ([]LintNode, error)

type IndexedHead struct { ID uuid.UUID; Extractor int; FrontmatterValid bool }
type EdgePos struct { SourceID uuid.UUID; Start int32 }
type EdgeHead struct { EdgePos; TargetBytes int64 }
type LintEdge struct {

    // PropertyKey nil为v2正文，指向空串仍是属性；旧版本先由current门排除。
    EdgePos; Kind string; PropertyKey *string; Target string
    TargetKey, AltKey *string; ResolvedID *uuid.UUID; ResolvedAsset, Ambiguous bool
}
type LintReads interface {
    Heads(context.Context, uuid.UUID) ([]IndexedHead, error)
    EdgeHeads(context.Context, ReadScope, EdgePos, bool, int) ([]EdgeHead, error)
    Edges(context.Context, uuid.UUID, []EdgePos) ([]LintEdge, error)
    // Count/Items 每种规则使用同一个资格关系，见 §3.7–3.9。
    RuleCount(context.Context, ReadScope, Rule) (Count, error)
    RuleItems(context.Context, ReadScope, Rule, Position, int) ([]Item, bool, error)
}
```

`ReadScope` 是本次快照核定的不可变值：notebook、effective mode、root、完整 source ids、当前 extractor 页集合、各规则资格集合及 sources 首键信息；不能传截断到 10k 的来源集合。10k 只限制一次移入，既有区可超过它。`RuleCount/RuleItems`是语义端口，adapter内拆具体sqlc方法，不用rule拼SQL。sources两规则的count/items不是各自再读一遍：同一次规则准备返回不可变SourceResults（D及按资格裁决的count/preview/请求页），两个语义投影只读它；单规则也使用同一收集器。准备值属于本snapshot、请求结束即释放，不作进程缓存。

只为需要的规则准备数据：摘要准备全八条；单规则不顺便跑其余规则。来源区读取至多一次；普通隐式 scope 不读来源区。显式 LLM scope 即使 DB mode=standard，仍按提议 root 读取严格后代，不读 source_hash；核 root 是本本 live page。非法内部 scope 为程序错误，不能暴露为无文档的 REST 新码。

### 3.3 来源、节点 metadata 与时序

`SourceArea.Nodes` 推荐复用 page 内的紧凑 metadata 读取和纯树筛选；只返回 root 的严格后代，root 自己永不返回。不能调用 SourceHeads/ProbeSources/SubtreeWithin 代替。树超过 10k 仍完整；循环/丢祖先是内部不变量错误，不能静默返回半区。`LintNodes.Nodes` 供 folder/AGENTS/相对路径准备；这与最终展示 `NodePaths.Paths` 是两个职责。首版容许两个 metadata 读各在本本范围，来源读可以沿现有完整 Subtree；测量必须含这两份工作，不能谎称“只读一次整树”。如合并成一次 page 内快照读复用，须维持公开 SourceArea 的严格成员语义及所有查询次序测试。

```sql
-- name: LintNodeMetadata :many
SELECT n.id, n.parent_id, n.kind, n.name_key, n.created_at,
       COALESCE(c.byte_size, 0)::bigint AS page_bytes,
       (c.node_id IS NOT NULL)::boolean AS has_content,
       (n.kind='page' OR
         (strpos(reverse(n.name),'.')>1 AND
          strpos(reverse(n.name),'.')<char_length(n.name)))::boolean AS linkable
FROM nodes n
LEFT JOIN LATERAL (
  SELECT c.node_id, c.byte_size FROM page_contents c
  WHERE c.node_id = n.id AND c.deleted_at IS NULL
  LIMIT 1
) c ON n.kind = 'page'
WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.deleted_at IS NULL;
```

该查询经page adapter的 `s.planned(ctx)`，不能复用cached的LinkTargets.ByKeys。linkable仅返回布尔，不返回原name；SQL最后点非首非尾的条件与`Node.Linkable`的原name条件一致，含无扩展名、点开头、尾点、多点及Unicode手写/差分例（决定60）。全部live候选进入presence，包括source及stale页；发出者过滤不能施加到候选。

只有 page 的 bytes 有含义；live page 缺 content 不得以 COALESCE 0 掩盖不变量，adapter 必须拒绝，返回只含 notebook_id/node_id 的内部错误 `lint metadata: live page has no content`，不把这页当空来源或 folder。has_children 从 live metadata 的 parent_id 构造，子附件也算子节点。folder = page 且 bytes=0 且有子节点；区外空叶页不是 folder。来源 page 无论有没有孩子，bytes=0 均不待消化。created_at 来自 nodes，不能用 UpdatedAt/Modified 或尚不存在的 ingested_at。

snapshot 内次序固定：**StateOf → SourceArea.Nodes（需要时）→紧凑节点/索引 heads、候选与规则→返回项的附件 Infos.Of→最终一次 NodePaths.Paths**。最后两口只交最终返回的 ids，摘要至多 40 项；去重 ids 后各最多一次；空集合不发 SQL。REST 也可复用相同内部结果但投影不带 path；snapshot 交错以 MCP 的路径为真见证。

### 3.4 八规则资格表：过滤早于 cap

`current(P)` 为当前 extractor 的 indexed_pages 行；不读正文 revision 对照、不寻找没有索引行的页；不可信旧 extractor 计入一次 `stale_pages`。stale count 数本本索引旧行（含来源/AGENTS），各规则不能重复累计。候选 page 若 stale 不在 orphan/missing/frontmatter/pending 候选中；stale emitter 不为任何规则贡献边；asset 没有 extractor，本身不当 stale page。

本次拟定current为Extractor 2；S1-0a未实施前实际常量仍为1。旧v1页不靠空值猜回：stale来源target仍可属area、故invalid_sources不因其stale报错，但不能给D/P提供消化证据；asset无extractor，旧来源父不在F时可独立pending。mixed版本结果带stale_pages，不宣称完整/干净。P4的Unknown不能直接沿用“跳过即为空”，见§3.8.2。

| 规则 | 候选/发出者 | 特有排除与计数单位 |
|---|---|---|
| wanted | current 发出者；LLM 去掉来源页 | 根/AGENTS 仍可发出；完整规范化及附件/虚拟排除；一 canonical 组一项，pages 为不同 source 的 union |
| orphans | current live page；所有模式排根下 AGENTS；LLM 再排来源及来源根 | 排 folder；入边为 current 的其他页、非 resolved_asset，LLM 排来源发出的边；根/AGENTS 发出的普通链接仍能提供入链，不扩写未批准豁免 |
| ambiguous | current 发出者；LLM 排来源 | 按 page_id + 完整 raw target 去重，真实 ambiguous 标志；resolved page 若 stale 跳过该候选（asset 不伪造 extractor） |
| invalid_frontmatter | current indexed page，frontmatter_valid=false | LLM 排来源；根/AGENTS 不额外豁免 |
| missing_type/summary | current、frontmatter_valid=true 的区外 page | 排来源根、根下 AGENTS；空叶/普通 folder 没有额外豁免；缺定义见 §3.7 |
| undigested_sources | 严格来源中的 current 非空 page 与 asset | 合格 wiki 发出者为 current 区外 page，排 root/AGENTS；正文边或首 sources 的属性边；附件从属见 §3.8 |
| invalid_sources | current 区外 page 的首个 ASCII sources 键 | **root 与根下 AGENTS 仍检查**（root 已决定）；真实坏 sources 报错，fenced 示例不入属性所以不报；frontmatter 失败没有属性，不重复报 |

“虚拟页”只在 effective LLM mode 中识别根下 index/log 保留目标；P3 提供 linking domain 的只读目标谓词和样例，不实现 P4 路径包装。普通模式的真实同名页/未解析目标不排除。来源根可在嵌套处，身份按 ID 祖先关系，不按 raw 名字/路径字符串。

所有资格排除在该规则 count 和 items 共用关系里完成；SQL 规则不能先 LIMIT 10000 再 Go 删除来源/stale 等。wanted/invalid_sources 的 Go 语义筛选发生在它们的累计器 cap 之前。`count==10000` 时 count_capped=true（下界，不宣称一定还有第10001项）；低于它为快照内精确值。wanted 全聚合仍须做完以排序，cap 只界摘要报告 count；group.pages 是全聚合的精确不同页数。

### 3.5 必做 sources 索引与其他索引准入

```sql
-- +goose Up
CREATE INDEX page_properties_sources_idx ON page_properties (source_id, position)
WHERE translate(key, 'SOURCE', 'source') = 'sources';
-- +goose Down
DROP INDEX page_properties_sources_idx;
```

迁移只归 linking，进 sqlc schema 与索引名检查；sources索引迁移本身不另升Extractor；§3.0的来源表示以S1-0a单独升至2，无新表授权。必须以保存 5000 条链接（含 sources 属性/非 sources 对照）测新增索引的写入代价，并实测迁移逐表扫描/构建时间、索引大小、SHARE 挡写不挡读的锁时间；写进升级说明/M12 handoff。不能只给查询成本而漏掉迁移代价。

keyset 读的测量候选为 **完整** `page_links_notebook_id_source_id_range_start_idx ON page_links(notebook_id,source_id,range_start)`，可 INCLUDE 仅短的固定 metadata，绝不 INCLUDE target/value。它不是 resolved_id IS NULL 索引。先跑现有索引的真实计划/工作量与前中尾批退化，再在测试自有库加候选比较；只有读范围或预算确实超界且该索引修正、5000-links 保存代价已量，主持者才批准迁移。摘要成本超线不能默认以索引数量解决；ambiguous/frontmatter 的可选部分索引也逐条相同准入。已有 property-links 索引不重复造。

### 3.6 wanted：SQL 分界、跨拼法的 distinct union 与真实空间

常量 `LintReadRows=512`、`LintReadTargetBytes=8*1024*1024` 只界内部一批物化，不限制笔记本或规则总量。先在same snapshot读Heads，relative folder由LintNodes的parent/key图迭代构造并缓存，不调用展示Paths。**候选不另读ByKeys**：从已经读出的全本live compactNodes建立`map[nameKey]kindBits`，page设置page位，asset只有Linkable为真才设置asset位；不按current/source/root/AGENTS筛候选。建表O(nodes)，每条target只O(1)查至多两末键，不物化候选祖先/展示名。

linking/domain现candidateSet还含exactly/ending；不要求presence伪造这两口。另提内部`kindPresence interface { has(string,bool) bool }`，把Target.form形参收窄到它；现nodeList和其他candidateSet天然满足、原保存/解析调用不变。公开窄`Presence interface { Has(string,bool) bool }`经domain内小适配器把Has转成has，新增`Target.ReadsAsAssetWith(p Presence) bool`只调用同一个form。已有ReadsAsAsset([]Node)继续传nodeList，不增读写热路round trip；lint只交kind位表。手写oracle与实际Node.Linkable/Target.form差分涵盖`.md`、Node.js/Node.js.md同canonical但page/asset形式不同、无扩展名/.hidden/trailing./a.b.c、casefold、同末键大量深路径、stale/source候选；不能创建伪全路径代表掩盖Linkable条件。删除LintWantedKeys及其端口/query/cost，最终Paths仍只读答出项（决定60）。

下面 SQL 均为 `queries/lint.sql` 的参数化查询，通过 `s.planned(ctx)`。excluded/current 通过完整 UUID 参数或 indexed_pages JOIN；source 数组用 `unnest` 关系及 anti join，不能以 generic 下逐行 `<> ALL(巨大数组)` 达到表面相同的结果。

```sql
-- name: LintHeads :many
SELECT node_id, extractor, frontmatter_valid
FROM indexed_pages WHERE notebook_id = sqlc.arg(notebook_id);

-- name: LintWantedEdgeHeadsFirst :many
WITH excluded AS MATERIALIZED (SELECT unnest(sqlc.arg(excluded_ids)::uuid[]) AS id)
SELECT l.source_id, l.range_start, octet_length(l.target)::bigint AS target_bytes
FROM page_links l
JOIN indexed_pages ip ON ip.node_id=l.source_id
LEFT JOIN excluded x ON x.id=l.source_id
WHERE l.notebook_id=sqlc.arg(notebook_id)
  AND ip.notebook_id=sqlc.arg(notebook_id)
  AND ip.extractor=sqlc.arg(extractor) AND x.id IS NULL
  AND l.resolved_id IS NULL AND l.target_key IS NOT NULL
ORDER BY l.source_id,l.range_start
LIMIT sqlc.arg(take)::integer;

-- name: LintWantedEdgeHeadsAfter :many
WITH excluded AS MATERIALIZED (SELECT unnest(sqlc.arg(excluded_ids)::uuid[]) AS id)
SELECT l.source_id, l.range_start, octet_length(l.target)::bigint AS target_bytes
FROM page_links l
JOIN indexed_pages ip ON ip.node_id=l.source_id
LEFT JOIN excluded x ON x.id=l.source_id
WHERE l.notebook_id=sqlc.arg(notebook_id)
  AND ip.notebook_id=sqlc.arg(notebook_id)
  AND ip.extractor=sqlc.arg(extractor) AND x.id IS NULL
  AND l.resolved_id IS NULL AND l.target_key IS NOT NULL
  AND (l.source_id,l.range_start) >
      (sqlc.arg(after_source)::uuid,sqlc.arg(after_start)::integer)
ORDER BY l.source_id,l.range_start
LIMIT sqlc.arg(take)::integer;

-- name: LintEdgesByPositions :many
WITH requested AS MATERIALIZED (
 SELECT unnest(sqlc.arg(source_ids)::uuid[]) AS source_id,
        unnest(sqlc.arg(starts)::integer[]) AS range_start
)
SELECT l.source_id,l.range_start,l.kind,l.target,l.target_key,l.target_alt_key,
       l.resolved_id,l.resolved_asset,l.ambiguous
FROM requested r CROSS JOIN LATERAL (
 SELECT l.source_id,l.range_start,l.kind,l.target,l.target_key,l.target_alt_key,
        l.resolved_id,l.resolved_asset,l.ambiguous,l.notebook_id
 FROM page_links l
 WHERE l.source_id=r.source_id AND l.range_start=r.range_start
 LIMIT 1
) l
WHERE l.notebook_id=sqlc.arg(notebook_id)
ORDER BY l.source_id,l.range_start;
```

初批调用First、有位置调用After；不拿全零UUID当“无游标”，不以OR破坏generic tuple下界。两条实际生成方法均有非空真库测试。读取 positions 的数组长度相等且去重，SELECT-list SRF 配对，不用隐式笛卡尔积。heads 最多 512 个，adapter 按累计 target_bytes≤8MiB 选前缀再取完整目标，未消费短 heads 暂存在 buffer；全部消费后才发下一次 heads 查询。不能从正文≤5MiB推出target≤5MiB：YAML的\L/\P可把2B写法解成3B UTF-8，PageFacts另有NUL替换。单target复用§3.8的保守MaxLintPayloadBytes（含解码/别名/JSON序列化增长的宽界）；超过8MiB但在该合法宽界内的目标独占一批并前进，不能按C报损坏。只有超过该宽界才为内部不变量失败。源范围大小不是 512。

sqlc `:many` 的每次 `Rows.Close`/`Rows.Err` 由生成函数保证：正常结束、Scan 错误与 ctx 取消都 close；Go 循环每批前、长规范化循环每固定小批检查 ctx；任一批失败丢弃整次结果，snapshot 回滚。不能将已读半份摘要答出。fixture 在第2批注入查询错误/取消后检查池 acquired 归零，随后同一小池真实请求成功；用仅计 fake calls 的测试不能证明连接释放。

资格在累计前逐条确认：

1. ParseTarget 原始完整 target；若无效、target_key=NULL、relative 越根，排除。
2. relative 从发出页的 **父 folder** 开始，先减 Up，再追加 Keys；rooted 只去掉 `/`；普通多段 `a/x` 直接为 `a/x`，不加发出 folder。每段 TitleKey 使用现有 shared.TitleKey；只剥末段一层 `.md`。
3. effective LLM 的虚拟根目标排除；ordinary 不排 index/log。image/embed 的 Name 经剥一层 `.md` 后仍 hasExtension 则排除。ReadsAsAsset 在全局候选已确定后排除。普通 `[[Node.js]]` 没有 asset 同名候选时仍算；`[[Node.js.md]]` 是 page 形式，即使 `Node.js` 读作 asset 也不能先计后减。
4. `canonical = strings.Join(absoluteKeys,"/")`，**完整**字符串是内部 map key。当前 `source_id` 的 `seen` 是 canonical 集合；同页新 canonical 才给 global[canonical].pages 加1，跨 raw/relative/rooted/大小写/.md 合组。source 不变时跨批保留 seen；遇到下一 source 才清；一页上限沿既有 MaxLinks=10000，所以 seen≤10000，与整本 source 数无关。
5. 代表文字来自所有保留下的合格边的最小 `(source_id,range_start)`；不是未过滤 raw 组的 min，不是最短拼法。流按 tuple 升序时首个合格命中即代表，保存其 UTF-8 边界 256B 展示值和 cut，不留无用完整 raw 值。
6. 完成全聚合再按 `(-pages, canonical前256原始字节, SHA256(完整canonical))` 排序，截报告 count、取前5或请求页；完整组身份不截断。同 source `[[a/x]]` 与 `[[/a/X.md]]` 必为1；第二 source 只写任一种时为2。

**空间/工作合同**：Go 峰值为 O(本本紧凑树与索引 heads + 完整 source/current ids + compactNodes派生的kind存在表 + 不同 canonical 的完整字节与组记录 + 当前 source≤10000组 + 正常一批≤8MiB目标，或一个≤MaxLintPayloadBytes的合法大target及规范化所需临时空间)。不是 O(所有 source×target 关联数)，不保存各组 source_ids，也不保存全量 edges。SQL读取本本未解析边的heads pass、精确target一次读及page紧凑metadata；DB排序/哈希/work_mem/spill另量，不藏进Go分配界。presence不读取候选祖先，仍实测一末键×大量深路径候选。完整excluded_ids会随每次heads请求重复序列化/构造unnest；返回512行/8MiB不是参数或DBjoin的界，必须将10万source与大量区外未解析边放同一形状，记录每批参数字节/分配、DB关系构建、总往返。初始完整摘要≤250MiB、10万页≤2s均是目标、未达证据。

**物理范围验收**：keyset 第一/中/尾/空页的实际 IndexCond 必须同时有 notebook 等值与有 after 时的 source/range tuple 界；不能全 PK 扫他本再 Filter，也不能每一页重扫/排序本本全部 links。若现索引做不到，§3.5 的候选复合索引必须先有失败/修正证据再进迁移；这项未满足不可合并 S1。所有目标长文只在精确 positions 点读；比对 SQL trace 与 TOAST 负正控制。非Wanted规则可以整本顺序扫描一次，不借此放宽 keyset 的逐批进度合同。

### 3.7 orphans、frontmatter 与 missing 两规则

下列 eligible 为调用者已经在 same snapshot 元数据中判定的完整资格 ids；它们有 notebook/current/source/root/AGENTS/folder 的具体含义，不是任意输入的 id。实际 SQL 仍复核 ip.notebook/extractor。查询参数的 source_ids 或 eligible_ids 是关系、NOT EXISTS/anti join；不对每条边重新线性扫 Go slice。

```sql
-- name: LintOrphans :many
WITH candidates AS MATERIALIZED (SELECT unnest(sqlc.arg(candidate_ids)::uuid[]) AS id),
 excluded_emitters AS MATERIALIZED (SELECT unnest(sqlc.arg(excluded_emitters)::uuid[]) AS id)
SELECT ip.node_id
FROM candidates c JOIN indexed_pages ip ON ip.node_id=c.id
WHERE ip.notebook_id=sqlc.arg(notebook_id) AND ip.extractor=sqlc.arg(extractor)
 AND NOT EXISTS (
  SELECT 1 FROM page_links l JOIN indexed_pages e ON e.node_id=l.source_id
  LEFT JOIN excluded_emitters x ON x.id=e.node_id
  WHERE l.resolved_id=ip.node_id AND NOT l.resolved_asset
   AND l.source_id<>ip.node_id AND l.notebook_id=sqlc.arg(notebook_id)
   AND e.notebook_id=sqlc.arg(notebook_id) AND e.extractor=sqlc.arg(extractor) AND x.id IS NULL
 )
 AND (NOT sqlc.arg(has_after)::boolean OR ip.node_id>sqlc.arg(after_id)::uuid)
ORDER BY ip.node_id LIMIT sqlc.arg(take)::integer;
```

items take=limit+1 发现 next；count 使用相同关系、无 cursor、take=10000，再 SELECT count(*)，不能 count 全部后截展示；预览用 items take=6。frontmatter 版只替换 NOT EXISTS 为 `NOT ip.frontmatter_valid`，其 candidate ids 按 §3.4，不借用 orphan 的额外豁免。生成 SQL 各有名、有实际参数测试。

type/summary 各一个静态 SQL，`translate` 谓词与各部分索引原样一致：

```sql
-- name: LintMissingType :many
WITH eligible AS MATERIALIZED (SELECT unnest(sqlc.arg(page_ids)::uuid[]) AS id)
SELECT ip.node_id
FROM eligible e JOIN indexed_pages ip ON ip.node_id=e.id
LEFT JOIN LATERAL (
 SELECT CASE
  WHEN p.value_start IS NULL THEN true
  WHEN btrim(p.value_start,sqlc.arg(trim_chars)::text)<>'' THEN false
  WHEN p.value_chars<=257 THEN true
  ELSE btrim(p.value #>> '{}',sqlc.arg(trim_chars)::text)=''
 END AS missing
 FROM page_properties p
 WHERE p.source_id=ip.node_id AND translate(p.key,'TYPE','type')='type'
 ORDER BY p.position LIMIT 1
) prop ON true
WHERE ip.notebook_id=sqlc.arg(notebook_id) AND ip.extractor=sqlc.arg(extractor)
 AND ip.frontmatter_valid AND COALESCE(prop.missing,true)
 AND (NOT sqlc.arg(has_after)::boolean OR ip.node_id>sqlc.arg(after_id)::uuid)
ORDER BY ip.node_id LIMIT sqlc.arg(take)::integer;
```

summary 只改静态谓词为 `translate(p.key,'SUMARY','sumary')='summary'`。count/preview/page 同一缺定义。`trim_chars` 固定常量，逐码点为 U+0009–000D、0020、00A0、1680、2000–200A、2028、2029、202F、205F、3000、FEFF，**无0085**；在真 PG 上与 jsTrim 的独立码点测试对照。首个键非字符串不能被后面的字符串挽救；frontmatter 坏只报 invalid_frontmatter。257字符前缀全空白且更长，必须读完整必要 JSON 值，后面有字则不缺；这种非结果行也计全部读取/TOAST/成本。P4 index 的 typed 反面复用这一定义，P3 不提前更改目录语义。

### 3.8 sources、消化、附件跟随

**唯一资格关系**：首个ASCII不分大小写sources键的根string、或根array中一层string，才产生合法slot；根array是集合，嵌套array/object与其他标量各是一项not_link，不递归把叶子升级为link。slot路径是实际首键（7个ASCII字节）或它加`.`及数组十进制下标。对该source_id/完整property_key的**全部持久化匹配边先数基数**，不先按resolved/source/asset/current目标过滤；恰1才交出具体range/target/resolution，0或≥2为一个not_link项，不消化任何匹配目标。同path碰撞的真实索引边仍独立适用wanted/ambiguous；坏JSON项自身不创造边。MaxLinks截断后的真实索引是权威；只剩另一顶层dotted键的一条同path边时仍不能恢复归属，这是已接受的depth/position丢失限制。首键position和原大小写保留；后一个大小写不同键不能补首键。

**SQL真正分批，Go唯一matcher**：不保留原按target的相关JSON EXISTS查询；`LintDirectSourceReferences`删除。摘要的invalid_sources与D共享一次sources访问和同一`SourceSlots`结果，单规则按需调用同一收集器。紧凑metadata/head得到完整eligible页UUID并排序，分成至多512个id交首键headers；首键JSON每页只解析一次，跨该页所有edge批保留计数器，只有确认该页终止才裁决/清空。没有全本JSON、边数组或各来源的引用者数组。

```go
type SourcePropertyHead struct { PageID uuid.UUID; Position int32; ValueBytes int64 }
type SourcePropertyPos struct { PageID uuid.UUID; Position int32 }
type SourceProperty struct { PageID uuid.UUID; Position int32; Key string; JSON []byte }
type SourceSpec struct { PageID uuid.UUID; Key string; Array bool; Items int32 }
type SourceSlotHead struct { EdgePos; PropertyKey string; TargetBytes int64 }
type SourceReadStore interface {
 SourcePropertyHeads(context.Context, uuid.UUID, []uuid.UUID) ([]SourcePropertyHead,error)
 SourceProperties(context.Context, uuid.UUID, []SourcePropertyPos) ([]SourceProperty,error)
 SourceSlotHeadsFirst(context.Context, uuid.UUID, []SourceSpec, uuid.UUID, uuid.UUID, int) ([]SourceSlotHead,error)
 SourceSlotHeadsAfter(context.Context, uuid.UUID, []SourceSpec, uuid.UUID, uuid.UUID, EdgePos, int) ([]SourceSlotHead,error)
 SourceSlotEdges(context.Context, uuid.UUID, []EdgePos) ([]LintEdge,error)
}
// Heads ids/specs≤512；两个UUID是当前JSON批first/last source；take≤512。
// Values/Edges只收按字节前缀选定的精确配对位置；app不持Rows。
// 模块根NewLintReads(pool,maxContentBytes)由bootstrap传page.MaxContentBytes，非用户配置。
func NewSourceSlots(first SourceProperty) (*SourceSlots,error)
func (s *SourceSlots) Add(edge LintEdge) error
func (s *SourceSlots) Finish() SourceJudgment // Qualified与Invalid共用同一基数关系
```

`SourcePropertyHeads`在page_ids≤512的关系上，按sources部分索引一次probe；返回固定宽度id/position/长度，没有未界定长key或JSON：

```sql
-- name: LintFirstSourceHeads :many
WITH requested AS MATERIALIZED (SELECT unnest(sqlc.arg(page_ids)::uuid[]) AS id)
SELECT ip.node_id,p.position,octet_length(p.value::text)::bigint AS value_bytes
FROM requested r JOIN indexed_pages ip ON ip.node_id=r.id
CROSS JOIN LATERAL (
 SELECT p.position,p.value FROM page_properties p
 WHERE p.source_id=ip.node_id AND translate(p.key,'SOURCE','source')='sources'
 ORDER BY p.position LIMIT 1
) p
WHERE ip.notebook_id=sqlc.arg(notebook_id) AND ip.extractor=sqlc.arg(extractor)
 AND ip.frontmatter_valid ORDER BY ip.node_id;

-- name: LintFirstSourceValues :many
WITH requested AS MATERIALIZED (
 SELECT unnest(sqlc.arg(page_ids)::uuid[]) AS id,
        unnest(sqlc.arg(positions)::integer[]) AS position
)
SELECT r.id AS node_id,p.position,p.key,p.value::text AS value
FROM requested r JOIN indexed_pages ip ON ip.node_id=r.id
CROSS JOIN LATERAL (
 SELECT p.position,p.key,p.value FROM page_properties p
 WHERE p.source_id=r.id AND p.position=r.position LIMIT 1
) p
WHERE ip.notebook_id=sqlc.arg(notebook_id) AND ip.extractor=sqlc.arg(extractor)
 AND ip.frontmatter_valid AND translate(p.key,'SOURCE','source')='sources'
ORDER BY r.id;
```

头部的`value::text`为了知道序列化大小会detoast/序列化，随后exact values又读一次，这是**明确两次必要DB工作**，不是声称只读一次TOAST；每页first-key probe只在headers发生一次，values按PK直取，不再找首键。adapter按value_bytes+7选≤8MiB前缀后才调用`:many`，未消费heads只留短buffer。单项大JSON独占一批，不能死循环或截值。界来自现解析器而非假称JSON≤5MiB：令C为固定`page.MaxContentBytes=5MiB`（非配置项），yaml.go现有maxYAMLNodes=10000、maxYAMLDepth=64、别名重复字节≤max(C,100000)，取保守`MaxLintPayloadBytes=6*(C+max(C,100000))+512*10000`作为单JSON及单target的共同宽上界（字符串JSON转义、别名展开与每节点数值/结构余量；PG JSONB::text对float64最小指数的展开可达约324位，512覆盖其符号/小数点/结构）；以现parser→PageFacts→真JSONB::text的边界fixture证明此界（指数数字、字符串转义、别名展开分别覆盖），并由bootstrap将page模块根公开的固定MaxContentBytes传给linking读取预算。不新增用户内容限制；这些payload界不另提升Extractor，§3.0的来源表示版本边界仍须执行；旧/非法SQL写入超过这个推导界是脱敏内部不变量失败，不能任意分配。正常批≤8MiB，峰值另含一个合法最大JSON及解码对象；成本须量最大形状。

当前JSON批的SourceSpec只带实际7B首键、根是否array、直接项数（现parser≤10000）；根既非string也无直接string的array时直接产not_link，不读取slot边。合法slot路径最长12B（7+1+4，下标0–9999）；**在SQL里先限制到这个语法后才允许投影property_key**，因此不是先把512条任意长YAML path装进slice再挑字节。窄headers虽含最多12B键，完整target尚未读；预算仍加key实际字节。

```sql
-- name: LintSourceSlotHeadsFirst :many
WITH spec AS MATERIALIZED (
 SELECT unnest(sqlc.arg(page_ids)::uuid[]) AS id,
        unnest(sqlc.arg(first_keys)::text[]) AS key,
        unnest(sqlc.arg(is_array)::boolean[]) AS is_array,
        unnest(sqlc.arg(item_counts)::integer[]) AS items
)
SELECT l.source_id,l.range_start,l.property_key,
       octet_length(l.target)::bigint AS target_bytes
FROM page_links l JOIN spec s ON s.id=l.source_id
JOIN indexed_pages ip ON ip.node_id=l.source_id
WHERE l.notebook_id=sqlc.arg(notebook_id)
 AND l.source_id>=sqlc.arg(batch_first_source)::uuid
 AND l.source_id<=sqlc.arg(batch_last_source)::uuid
 AND ip.notebook_id=sqlc.arg(notebook_id) AND ip.extractor=sqlc.arg(extractor)
 AND CASE WHEN NOT s.is_array THEN l.property_key=s.key
     WHEN octet_length(l.property_key) BETWEEN 9 AND 12
       AND left(l.property_key,8)=s.key||'.'
       AND substring(l.property_key FROM 9) ~ '^(0|[1-9][0-9]{0,3})$'
       THEN substring(l.property_key FROM 9)::integer<s.items
     ELSE false END
ORDER BY l.source_id,l.range_start LIMIT sqlc.arg(take)::integer;

-- name: LintSourceSlotHeadsAfter :many
WITH spec AS MATERIALIZED (
 SELECT unnest(sqlc.arg(page_ids)::uuid[]) AS id,
        unnest(sqlc.arg(first_keys)::text[]) AS key,
        unnest(sqlc.arg(is_array)::boolean[]) AS is_array,
        unnest(sqlc.arg(item_counts)::integer[]) AS items
)
SELECT l.source_id,l.range_start,l.property_key,
       octet_length(l.target)::bigint AS target_bytes
FROM page_links l JOIN spec s ON s.id=l.source_id
JOIN indexed_pages ip ON ip.node_id=l.source_id
WHERE l.notebook_id=sqlc.arg(notebook_id)
 AND l.source_id>=sqlc.arg(batch_first_source)::uuid
 AND l.source_id<=sqlc.arg(batch_last_source)::uuid
 AND ip.notebook_id=sqlc.arg(notebook_id) AND ip.extractor=sqlc.arg(extractor)
 AND (l.source_id,l.range_start)>(sqlc.arg(after_source)::uuid,sqlc.arg(after_start)::integer)
 AND CASE WHEN NOT s.is_array THEN l.property_key=s.key
     WHEN octet_length(l.property_key) BETWEEN 9 AND 12
       AND left(l.property_key,8)=s.key||'.'
       AND substring(l.property_key FROM 9) ~ '^(0|[1-9][0-9]{0,3})$'
       THEN substring(l.property_key FROM 9)::integer<s.items
     ELSE false END
ORDER BY l.source_id,l.range_start LIMIT sqlc.arg(take)::integer;

-- name: LintSourceSlotEdges :many
WITH requested AS MATERIALIZED (
 SELECT unnest(sqlc.arg(page_ids)::uuid[]) AS source_id,
        unnest(sqlc.arg(starts)::integer[]) AS range_start
)
SELECT l.source_id,l.range_start,l.property_key,l.target,l.kind,
       l.resolved_id,l.resolved_asset,l.ambiguous
FROM requested r CROSS JOIN LATERAL (
 SELECT l.source_id,l.range_start,l.property_key,l.target,l.kind,
        l.resolved_id,l.resolved_asset,l.ambiguous,l.notebook_id
 FROM page_links l WHERE l.source_id=r.source_id AND l.range_start=r.range_start LIMIT 1
) l WHERE l.notebook_id=sqlc.arg(notebook_id)
ORDER BY l.source_id,l.range_start;
```

spec批按source UUID全程向前，batch_first_source/min与batch_last_source/max界在SQL里显式给出；下一JSON批的first必须严格大于上一批last，First不从全本起扫，After继续同批tuple。允许扫描这个不重叠source区间内没有合格slot的边一次；不得每个JSON批从notebook首部重扫、也不得PK扫其他本。计划/physical测试同时核notebook等值、批区间及after tuple；spec关系可能筛掉的工作要计数，不能只报512返回行。无sources时不发slot查询。

上述First/After分别生成并测试，不用OR伪装两个计划。spec数组等长且id唯一，items形状与JSON核对，take固定≤512；SourceSlotEdges只吃headers给出的精确配对位置。数组中的非string ordinal可被headers返回，Go同一slot matcher忽略它；不能在SQL按目标资格预筛唯一性。target+12B以内key按≤8MiB前缀再读；单一target超过8MiB时按MaxLintPayloadBytes宽界独占一批；固定5MiB正文界不是解码target界。SourceSlots按ordinal（根string用专用root槽）计数至2即可，保留唯一边的完整target/resolution；第二条时丢弃该唯一候选并标冲突，后续边无需保留。同一页跨512行/8MiB头尾仍是同一个实例；当下一source到来，先Finish当前页；一页无匹配边也必须Finish。不同页JSON在该批完成后全部释放，不能跨全本积累。

D的正文部分另用短`LintBodyReferencesFirst/After`：与§3.6的本本tuple范围相同，JOIN current indexed_pages、完整wiki-emitter ids的MATERIALIZED关系，WHERE `property_key IS NULL AND resolved_id IS NOT NULL`，SELECT仅source_id/range_start/resolved_id/resolved_asset，ORDER tuple LIMIT512；无target/property JSON。每一短行通过完整area kind表，页候选需current非空，asset需kind一致，再向D的node-id集合记一次。sources部分等该页SourceSlots.Finish后，只把Qualified的resolved来源候选同样喂D；invalid_sources与D因此不能各自解释一套SQL。D是至多source节点数的集合，不是edge关联数组。

上述IS NULL只有在§3.0的v2/current关系内才证明正文；真实空属性路径为非NULL空串，不能用COALESCE(property_key,'')=''或Go空串判断替代。First/After与F各自保此资格；正常named/empty属性仍独立适用通用规则，只有首sources唯一slot能另入D。

正文短读的两个独立生成语句如下，不带has_after OR参数：

```sql
-- name: LintBodyReferencesFirst :many
WITH emitters AS MATERIALIZED (SELECT unnest(sqlc.arg(wiki_page_ids)::uuid[]) AS id)
SELECT l.source_id,l.range_start,l.resolved_id,l.resolved_asset
FROM page_links l JOIN emitters e ON e.id=l.source_id
JOIN indexed_pages ip ON ip.node_id=l.source_id
WHERE l.notebook_id=sqlc.arg(notebook_id) AND ip.notebook_id=sqlc.arg(notebook_id)
 AND ip.extractor=sqlc.arg(extractor) AND l.property_key IS NULL AND l.resolved_id IS NOT NULL
ORDER BY l.source_id,l.range_start LIMIT sqlc.arg(take)::integer;

-- name: LintBodyReferencesAfter :many
WITH emitters AS MATERIALIZED (SELECT unnest(sqlc.arg(wiki_page_ids)::uuid[]) AS id)
SELECT l.source_id,l.range_start,l.resolved_id,l.resolved_asset
FROM page_links l JOIN emitters e ON e.id=l.source_id
JOIN indexed_pages ip ON ip.node_id=l.source_id
WHERE l.notebook_id=sqlc.arg(notebook_id) AND ip.notebook_id=sqlc.arg(notebook_id)
 AND ip.extractor=sqlc.arg(extractor) AND l.property_key IS NULL AND l.resolved_id IS NOT NULL
 AND (l.source_id,l.range_start)>(sqlc.arg(after_source)::uuid,sqlc.arg(after_start)::integer)
ORDER BY l.source_id,l.range_start LIMIT sqlc.arg(take)::integer;
```

count/preview/items复用同次收集器；仅invalid_sources分页在处理完其页的所有slot后才截断，不能在一个冲突的第二条边前提前宣告它合法。

真实link解析不到或target不在完整严格area（含root/区外）→invalid_sources；其真实unresolved/ambiguous边仍可进入相应通用规则。resolved到stale来源页仍属来源，invalid_sources不报；D/P不能用旧extractor证明已消化。not_link不消化，本身不进其他规则；同页L项按完整raw target去重，J项按稳定JSON文字去重（UseNumber、map键稳定），身份含L/J标签，显示同文仍可为两项。根/AGENTS真实坏sources要报，fenced示例无属性不报。

**独立oracle与差分**：手写`Sources:["[[raw/a]]"]`加`Sources.0:"[[raw/b]]"`应为一个J坏项、a/b都不进入D；其中一条resolved到区外、为NULL或asset也不能先过滤后变成唯一。无冲突数组应为L合格、旧对象`{"0":"[[raw/s]]"}`应为J，根string/重复文字/MaxLinks截断/第二大小写键/长非slot key分别给独立预期。SQL返回关系→Go结果与手写期望比，不以两个共用错误matcher的产物互比；真实SQL边跨行/字节批、byte headers短投影、必要JSON只按每页读取均留负控。

附件集合：P为current非0来源页，D为上述合格wiki直接引用；F(A)为P中通过**真实正文**（v2的property_key IS NULL）wikilink/link/embed/image、resolved_id=A且resolved_asset=true引用A的不同页。A∈D或任一F(A)∈D则已消化；否则F(A)非空由未消化来源页代表、不另列；F(A)空则A独立待消化。stale/0父不加入F；被wiki直接引用的附件不自动消化它的父来源页。F用resolved_id范围EXISTS，完整P ids关系，不array_agg父页。未消化page=P\D；按(created_at,id)排序，最终答出asset再取实际bytes。source边只为F读取，不贡献wanted/ambiguous/orphans。

kind已由page_links约束为上述四值，F不需额外kind筛选；若显式筛选必须含全部四种。普通[[asset.pdf]]是wikilink，须非空正例。空路径或named来源属性不能形成F。pending资产输出只需A不在D且F(A)为空的判定；不为测试新造未消费的“已消化父”查询。F的实际EXISTS/父关系读单独控制，D的First/After不能代证。

**R-I1/A2-I1合法增长见证**：真实Markdown parser→PageFacts→生产保存→读取 fixture使用带`---` frontmatter的`sources: "[[a`，加1,800,000个字面YAML双引号转义`\L`（另有`\P`小例），再`z/x]]"`。正文严格小于固定C=5MiB，手写完整expected target为`a`+同数量U+2028+`z/x`，persisted target大于C；不能用被测canonical/matcher生成期望。a/z保留前段两端，短末段`x`使真实保存后的`target_key='x'`非NULL，`resolved_id=NULL`；索引发出页须current、在来源区外，目标无source/virtual/asset排除，首sources只有这条合格slot边。先核真实保存的完整target、key及resolution，再分别沿生产wanted与source-slot读取：`LintWantedEdgeHeadsFirst`→`LintEdgesByPositions`和`LintSourceSlotHeadsFirst`→`LintSourceSlotEdges`均须有此(source_id,range_start)非空证据，完整target与手写值相等；实际wanted有该组且pages=1，invalid_sources有该未解析L项。记录实际源/target字节、两种heads长度、批次及前进，不能用wanted空集合代证。原不带`/x`的单长末段只可留作MaxKey使target_key=NULL的过滤对照，不能作wanted守卫见证。

大样本进!race成本/LP，纯domain另小手写正例；SQL直接造长target只能做反向非法数据控制，不能替代合法性证明。S62的wanted/source-slot两个子项分别只把各自**target读取检查**收紧为`target>C即报损坏`，JSON的MaxLintPayloadBytes宽界保持不动；各自独立基线→单改→指定非空子测试失败→恢复，留失败阶段，不能一次改共用常量让更早JSON失败抵账。两条生产读取均由正例trace与返回目标证明已到达。本稿仅核静态源码与样例构造，真实parser/保存/读取及成本仍待S1，未声称运行复现。

### 3.8.1 给P4的旧合格事实合同（决定59；本Phase只交纯判定与文档）

现`Index.PagesChanged`在page unit最终树上运行；Event的Before/After只有parent/name，不含旧source/root/AGENTS资格。**删旧属性/链接之前不等于改树之前**。P4不得从删后的新JSON或最终树反推旧资格；也不得只保存旧(resolved_id,property_key)+首键名。

拟定P4窄合同（模块各自定义DTO、bootstrap逐字段转；这里不新增P4存储/运行接线）：

```go
type IngestNodeBefore struct {
 ID uuid.UUID; Exists bool; Kind NodeKind
 InSource, IsSourceRoot, IsRootAgents bool; PageBytes int64
}
type IngestBefore interface {
 NodesBefore(ctx context.Context, ids []uuid.UUID) ([]IngestNodeBefore,error) // ≤512，unit起点
}
type QualifiedReference struct {
 TargetID uuid.UUID; TargetIsAsset bool
 Origin ReferenceOrigin // Known且v2时的Body或FirstSourcesSlot；空属性不在其中
 Start int32            // 旧persisted range，不带raw target/JSON
}
type OldQualifiedFacts struct {
 PageID uuid.UUID; Indexed, Current, WikiEmitter bool

 State OldFactState // P4定义Unknown/KnownAbsent/KnownIneligible/Known；示意合同，非现有类型
 References []QualifiedReference // 仅Known时可信；当前一页≤MaxLinks，不以空数组冒充Unknown的旧贡献
}
type QualifiedBeforeScope struct { NotebookID uuid.UUID; Before IngestBefore }
// linking app拥有，组合Store的旧索引读与page的Before端口；每被改页一次。
func (q QualifiedBefore) Capture(ctx context.Context, scope QualifiedBeforeScope,
 pageIDs []uuid.UUID, emit func(OldQualifiedFacts) error) error
```

P4在Index的同一索引锁下、第一次ReplacePage/DeletePages删除任何旧属性/边前，对本unit写正文或删除的页调用Capture；Store仍只读linking表，以§3.8的短headers/512/8MiB/单合法JSON界抓旧首键和完整未过滤匹配边，旧indexed_pages判current，SourceSlots作唯一资格；Body/FirstSourcesSlot分别标Origin，resolved_id/asset取旧索引，不在最终树重新resolve。Before.NodesBefore给旧发出页和旧resolved目标的source/root/AGENTS/kind/bytes；合格wiki emitter与非空来源目标都用旧事实。只保当前页References，emit后释放；Index只把Known的引用并入**旧已消化来源ID union**，另保留Unknown传播（§3.8.2）；KnownAbsent/Ineligible的空贡献须有各自证明，去重后至多完整来源数，不保存全unit页×target关联。新合格引用得到候选后，减旧union可以排除已知的“本页已有”及“其他被写/删页原已消化”的两种情况；仍不能仅凭剩下候选证明新颖性；须通过§3.8.2旧事实/未改页coverage闸门，并批量查未被写/删页，沿00的一条候选语句/索引合同。**这条批量SQL是P4必须另行设计并实测的合同，P3没有交付可直接复用的direct EXISTS**；P4也须首JSON每相关页一次、同source/path的全部匹配边先数唯一性，再按candidate target筛，与同一资格关系及手写oracle差分，不能带回按边重扫JSON或目标预筛。不是每来源目标重读旧JSON。

page的Before端口在P4用unit起点的窄事实重建：在每个`applyStep`真实写之前，把首次变动节点的Before(parent/name/kind)记入unit memo（新建记起点不存在；删除子树逐节点记；只取第一次，不被后续临时名/move覆盖）；首次正文写另记旧PageBytes。最终observer请求某ID时，以当前page port节点事实叠加这些起点Before，沿父链递归重建初始祖先（父节点移动/删除也在memo；不变祖先走现page.Nodes/Ancestors），按Write.Notebook起点source_root_id和原name判source/root/根AGENTS。此前已变过祖先而后来才被写的子页，也从同一逆向memo重建，不能拿那一刻的Step.Lines直接当unit起点。当前树缺失节点从删除memo可读，起点不存在明确返回Exists=false；缺必要祖先是内部错误，禁止退回当前树。只缓存请求过的紧凑节点/路径事实，不读正文；tree锁和unit事务沿现Writer边界。Event新增unit拥有的BeforeIngest只读能力，bootstrap把Write.Kind/Notebook/ChangesetID与该能力一起交linking；P4阶段新增真实捕获与删除/改树测试，P3不提前为P4改Event/Store写热路；S1-0a的既有来源写入参数修正按§3.0单列，不实现Capture或ingest。

**近似范围保留**：在§3.8.2所需旧事实已知、未改页覆盖足够时，被写/删页的旧JSON/解析与发出页资格按上述起点事实；C没有写也没删的可信页仍按C之后树/解析判断。混合改树导致这些未改页改解析/跨source边界时历史标签可能多/少一次，仍是00§4.14明确近似，不把整个ingest称为旧快照精确。

以下旧/新引用反例以Known旧事实及足够的未改页覆盖为前提；Unknown输入按下一节保守不加标记，不能用这些正例去豁免旧v1。

交接反例：旧对象`sources:{"0":"[[raw/s]]"}`改为数组，应允许成为新ingest；旧数组改为同数组（伴随正文edit）不能重复标；旧dotted冲突改为唯一数组应允许新ingest；旧正常数组变冲突不能新消化。原来源页被移出再写、原wiki页移入再删、祖先改名使AGENTS身份变动、同batch临时名/先删父再处理子，都用unit起点资格。P3纯matcher用旧/新JSON成对的手写oracle先守JSON差别；OldQualifiedFacts实际类型/捕获由P4实现；P4 Capture/Before端口/真实ingest与成本是P4必须兑现的交接，不写本期已实施。

### 3.8.2 P4旧事实未知与未改页覆盖（决定62；仅交接，待P4）

OldQualifiedFacts的Indexed/Current与BeforeIngest.Exists必须合成明确状态，空References本身不是旧事实充分的证明：

| 旧状态 | 允许的推论 |
|---|---|
| KnownAbsent | unit起点确实不存在的新页；旧引用已知为空 |
| KnownIneligible | 可信unit起点事实证明不是合格wiki emitter；其旧贡献为空由资格证明，不由缺索引证明 |
| Known | 已存在的合格页具有当前v2索引、旧JSON/旧resolution与起点资格，可按§3.8.1得到确切引用 |
| Unknown | 已存在且可能合格页的索引缺失/非current，或判定所需旧事实不足；不可当空union |

一个v1页原有真实body/sources指向S，普通edit保留该链接只改旁文时，跳过旧行再把新v2引用当新证据会错误标ingest。已有页缺索引与新页起点不存在必须分开。旧empty-property→新真实body/唯一sources在Known状态可新消化；旧真实body/sources→相同引用不能重复标。

未改页一侧同样必须有足够coverage：新建P虽KnownAbsent，未改的旧Q也许早已引用S；过滤Q后NOT EXISTS不能证明S原未消化。新标记前，所有对该候选历史判定有影响的旧/未改事实须已知；完整相关live事实在unit边界current是一个充分条件，P4可设计更窄但同样充分的证明。不得只检查被改页Current，或只在最终树扫描stale而漏掉本unit已删/刚升级的旧页。

未知时正常编辑和索引维护继续成功、既有标记保留，不新增无法证明的ingest。升级存量须先以新版reindex恢复可信事实，之后恢复可靠判定；不补造此前标签。这个保守缺标与§3.8.1既有“已知未改页按最终树”的混合改树近似分开记录。P4须具体设计高效coverage/Unknown传播、脱敏诊断和索引锁内成本，不因此许可无界每次全树扫描、新持久化ready字段、请求时正文修复或新公开错误。

Unknown是旧索引/证据不足的显式状态，不吞掉数据库读取失败、取消或既有内部不变量错误；§3.8.1“缺必要祖先是内部错误、禁止退回当前树”保持。正常证据缺口的保守缺标与执行失败分别处理，不能为了继续编辑把真正错误伪装成KnownAbsent或Unknown。

P4实施必须有真实普通edit、stale/missing旧Q被改/删、新P加未改stale Q、全部可信且真正新P、重建后不把保留旧链接标新等非空oracle；Unknown→KnownEmpty与丢未知未改页的NOT EXISTS分别做运行时变异。这些属于P4表/门禁，不计入P3已实施或已跑。

### 3.9 ambiguous、身份、排序与游标

ambiguous 的 qualified rows 为 current emitter、LLM非source、l.ambiguous=true；解析到 stale page 的候选不可信，按 §3.4 跳过。使用 §3.6 相同有界边机制但静态 WHERE 换成 ambiguous，不与 wanted predicate 拼字符串。按source局部完整 raw target去重，代表取最早range；排序 `(page_id ASC, SHA256(完整rawtarget) ASC)`。invalid_sources 同样按 `(page_id ASC, identity_hash ASC)`，identity_hash 输入含 L/J；page规则按page_id，pending按created_at/id。每条 items 与 count/preview使用同资格累计器；开始page游标后仍读该page全部相关边以正确合并，不因按range继续丢组。

wire统一 `item_key`：page项 `p:<UUID>`；wanted `w:<SHA256完整canonical>`；ambiguous `a:<pageID>:<SHA256完整raw>`；invalid_sources `s:<pageID>:<SHA256(L或J+长度前缀+完整文字)>`；pending `n:<UUID>`。内部组 map 一律完整键；SHA只作稳定外部身份与有界排序尾（碰撞按加密哈希安全假设，不用截断显示作相等）。UTF-8字符串截断经现有安全helper，target_cut独立。

游标用 shared Encode/Decode 的唯一编码，最大2048字节，在 decode前拒过长；载荷示例：

```go
type LintCursor struct {
 V int `json:"v"`      // 1
 R string `json:"r"`   // 必须等于请求rule
 N uuid.UUID `json:"n"`
 S string `json:"s"`   // I=隐式；E:<mode>:<root>=显式
 C int64 `json:"c,omitempty"` // wanted pages
 K string `json:"k,omitempty"` // wanted前256原始字节的base64url，避免半个UTF-8
 H string `json:"h,omitempty"` // 64位lowerhex SHA256
 I *uuid.UUID `json:"i,omitempty"` // page/node；array UUID 的 omitempty 不省零值，故用指针
 T string `json:"t,omitempty"`   // pending RFC3339Nano UTC唯一格式
 B bool `json:"b,omitempty"`     // 唯一开始位置，仅true且无其他排序字段
}
```

内层K须base64url严格decode后reencode与原串完全相同，decoded长度≤256B；外层shared.DecodeCursor的唯一编码不能替代内层规范检查。用于flight的canonical从实际解码排序值重建。

不同规则只容许它自己的字段组：wanted C/K/H；ambiguous/invalid_sources I/H；pending T/I；其他 I。禁止不适用字段、零/负pages、hash非规范、时间不可往返、未知version/rule、跨notebook或scope；这些都是400。隐式游标不绑旧mode：降级后的专属规则走409；显式scope身份隔离M11。分页独立快照，条目可能移动/重复/消失，不保证一次翻完的全局快照；客户端按 item_key 去重。

每个 item 带 cursor；next_cursor 为最后实际答出的item位置，后面没有时null。wanted排序分页比较 C降序/K原始字节升序/H升序，不能比较K的base64文本顺序。页面游标可在MCP Fit的中途续；10000 cap不截断 items 后续分页。

### 3.10 一次真实snapshot、Runs owner、期限与收尾

`wiki.New(Deps)`创建进程唯一`Runs`，REST、两MCP scope和内部显式scope共用；不能每handler建信号量。`MaxLintRuns=max(1,floor(floor(database.max_conns/2)/2))`来自请求半池的一半、至少1；合法最小池6给1。`LintTimeout=min(10s,server.request_timeout/2)`；ParseMaxWait>0且<R使合法R≥2ns，式子正且<R；默认15s给7.5s、测试4s给2s。没有新配置键、不提高原HTTP/Go期限。这个界覆盖worker等slot与计算，不保证事务清理或任意外层已耗尽deadline也能及时503。

每caller按§3.11在平台入口之后执行RequireActor、cursor纯形状、自己的授权、rule/mode/limit；等待ctx取caller剩余期限一半与LintTimeout较短者。主动cancel答原ctx取消，内部等待/计算预算到期为`shared.ServerBusy(time.Second)`；剩余已耗尽不假装有完整LintTimeout可用。交付完整结果前，在snapshot外用caller自己的ctx再次Authorize(lint.read)，撤权/删除答404。flight不保存可复用授权，actor不构成共享数据范围。

key=`(NotebookID,implicit或explicit(Contract,AreaRootID),queryKind,Rule,decodedCursorCanonical,effectiveLimit)`。pending_sources与相同参数undigested列表可合并；摘要独立；immutable结果每caller投影，Fit不改共享slice/map。implicit不可把外层读到的mode写成explicit；显式standard本拟议LLM根仍独立计算。没有跨打开缓存。

```go
// wiki app内部；模块根只暴露生命周期，不泄漏信号量/flight。
type Runs struct { /* mutex, closed, flights, workers, active, drained, slots, timeout */ }
func NewRuns(max int, timeout time.Duration, log Logger) *Runs
func (r *Runs) Do(ctx context.Context, key RunKey,
 run func(context.Context) (LintResult,error)) (LintResult,error)
func (r *Runs) Stop(ctx context.Context) error // 幂等：拒新flight、取消、等worker清理
// module.New只建一个Runs；同module注册HTTP/tools/内部入口。
func (m *Module) Stop(ctx context.Context) error { return m.runs.Stop(ctx) }
// bootstrap app持有这个module的停止口，不另造owner。
type lintStopper interface { Stop(context.Context) error }
// app字段: lintRuns lintStopper
```

**建flight与运行**：mutex下先核closed；closed立即busy，不能同key重新启动。已有未终结flight可join；否则在同一mutex下加入flights和workers、active++再启动worker，避免Stop与Add竞态。followers在slot外；新worker用`WithTimeout(WithoutCancel(leaderCtx),LintTimeout)`，保存自己的CancelCause/timeout cancel到workers；leader取消不传入，owner Stop会显式cancel所有workers（包括已expired、已从flights摘掉但还在cleanup的worker）。worker取得slot后进入WithinSnapshot，第一条应用SQL为StateOf；implicit从中定mode/root且专属rule再次409，explicit先核本本存在再按显式合同/root。所有后续端口拿同snapshotctx，顺序依§3.3，读事务不加行锁。

**三个状态分别拥有**：可合并map只存accepting flight；一次终结通知/result用mutex或sync.Once；workers/active直到goroutine完全清理才移除。deadline的AfterFunc先把该flight标expired，只有map当前值仍是自己才删，再发布一次busy/关闭通知；不能因旧worker晚结束删掉新同keyflight。caller可先503，slot直到snapshot清理完才归还。leader与任意follower各自监听自己的ctx.Done与flight终结，取消一个不取消worker/其他caller。owner Stop也只终结一次；release只在acquired=true时执行一次。

**worker自己的recover（B-I3）**：worker最外层defer是fault/finish边界。Inside Snapshot的defer先运行（TxManager.end独立rollback/commitTimeout，可能超LintTimeout）；再由外层recover截住panic，设固定`internal failure`，不重新panic、不使用panic原值作error/日志。清理后释放已得slot，mutex下按身份删flights及workers、active--，若尚未终结则发布成功/普通错误/脱敏internal一次，唤醒全部未取消caller；closed且active=0关闭drained。若expiry/Stop已发布busy，panic只在最终统计里记枚举fault，不能第二次发布/close；panic与清理错误合并为脱敏internal类别，绝不把SQL参数/原panic写日志。单一defer保证panic、普通error、Acquire失败与Stop均完成所有权清理。调用者可以已返回，最终worker日志只记一次outcome/cleanup枚举与id/耗时/计数。

**Stop（决定58）**：mutex下首次closed=true并阻止所有后续创建；取得workers的cancel列表、已accepting flight在同一状态机中终结busy；mutex外逐个cancel，已取slot仍等snapshot清理。drained是同一owner的一条channel；closed时active=0立即close，否则最后worker关闭，重复Stop只等同一drained或自己的ctx，没有每次新增后台Wait goroutine。timeout返回ctx错误，owner保持closed且后台清理继续；不能承诺卡死外部端口一定drain成功。 已被deadline取消的expired worker再次cancel不改变ctx/cause，也不取消TxManager.end的独立cleanup；省掉这次重复cancel但仍保留workers/active并等待drain是等价变异，不要求杀死。S52分别守尚未到期的运行/排队worker取消，以及expired-cleanup仍纳入active/drain，不能为杀等价变异改变清理语义。

**真实bootstrap接线位置**：wireApp在创建wiki module后把同一个值交HTTP Register、mcpTools及`app{lintRuns: wk}`。`app.run`改命名返回`(runErr error)`，函数开头、`warnIfExposed/awaitDatabase/Up/CheckDatabase`之前登记defer：独立`context.WithTimeout(context.WithoutCancel(ctx), cfg.Database.CommitTimeout + a.poolCloseTimeout)`调用`a.lintRuns.Stop`，将错误`errors.Join`入runErr；沿现HTTP→listener→jobs顺序收尾，HTTP正常返回/监听失败、startJobs cancel cause、run ctx取消和前置DB/迁移失败都经此defer，不能只监听newApp的上游ctx。清理调度余量沿现poolCloseTimeout=5s、无新配置；run ctx即使已取消仍有独立清理界。`app.close`在migrator.Close及pool.Close**之前**再调用相同幂等stop helper，覆盖仅构造就close/前置失败；兜底错误只按枚举stage记录有限告警，不直接slog.Any潜在输入。Stop超时后close仍按现pool关闭策略继续并记录未drain界，不无限等。模块CLI template构造不创建闲置后台worker；不公开httpserver私有stoppingKey、不造通用任务框架。

实际DB错误保留internal，不能一概变busy；自己的budget已到导致rollback附带错误按已发布终止原因分类，cleanup错误只记阶段/SQLSTATE。调用前认证、共享结果前再授权、每caller取消、worker expiry、recover与owner Stop各有独立catcher，见§7.2/7.3。

### 3.11 码次序、访问与日志

平台层先行：MCP notebook入口`POST /mcp?notebook=N`保留现Notebooks.Gate。不可见/删除N时先返回**HTTP404问题响应**，SDK/tool handler根本未运行，哪怕tool同时带坏cursor。account入口没有这个book Gate，book由工具参数交用例；认证/JSON-RPC binding等仍属平台层。下面只描述REST绑定之后及account/gate通过后的tool-app次序，工具错误是structuredContent.error，不能把它与入口HTTP问题响应混为一条：

| 入口 | 用例内部次序 |
|---|---|
| getLint | notebook404→503预算 |
| listLintItems / lint带rule / pending_sources | cursor400→notebook404→rule404→外层mode409→limit422→503；得slot后snapshot首读再次mode409 |
| lint不带rule | cursor纯格式400→notebook404→带limit/cursor却无rule的组合422→503；不默默忽略 |
| 显式内部scope | 每caller授权；内部调用者保证Contract/root结构合法，不公开接收它们 |

两scope必须分别实测多重无效：不可见N+badcursor+bad-rule+limit0，notebook入口先HTTP404、handler0次；account是tool400，修cursor后tool404。换为可见standard本，notebook Gate通过和account均先badcursor400，再unknown rule404，再专属mode409，修为通用rule后limit422，再预算503。Gate通过后发生撤权由用例notebook404/交付前复核捕获；不为lint绕过/重排Gate。

`lint.read`规则reader/editor/admin都允许，workspace admin对私密不可见本仍404。真实REST矩阵新增两个操作；rule参数只在前段为lint的`{rule}`时豁免其值核对，前面的notebook_id仍核所在列工作区。negative test把notebook_id换别列应失败，不把整个路由放notTargets。MCP每入口lint/pending_sources恰一行；普通/LLM变体不是把真实授权替成fake；unknown rule、standard专属规则和busy均真实答出。

每次实际flight记录一条info `lint computed` 或 `lint stopped`，含 notebook_id、有效mode、rule枚举、每规则duration/count/capped、stale_pages、wait_ms、outcome；followers不重复记运行行，平台自己的每工具日志保留。绝不记node title、raw/canonical target、path、sources JSON、cursor、用户输入rule的原文；未知rule只记固定reason。日志测试从本子测试起点取行，包含只在该子测试内容中的秘密字符串，并核一条共享运行而多条独立工具调用。

### 3.12 REST schema 与真实 generated 消费

`api/modules/wiki.yaml`先写在S1相同提交，配置oapi-codegen、入口refs、Makefile自动API模块枚举，运行make gen并提交Go/API bundle/TS。返回对象additionalProperties:false，null显式，数组即使空也是[]。所有operations声明503/Retry-After；新404码写中文“没有这条健康检查规则。”、英文“This health check rule does not exist.”，加入problem-messages映射；已有server_busy/not_llm_wiki文案复用。

核心schema（字段表是完整最低合同，oneOf每个分支禁止其他字段；实际YAML逐支定义，不能any）：

```yaml
LintRule:
  type: string
  enum: [wanted, orphans, ambiguous, invalid_frontmatter, missing_type, missing_summary, undigested_sources, invalid_sources]
LintSummary:
  type: object
  additionalProperties: false
  required: [mode, computed_at, stale_pages, rules]
  properties:
    mode: { type: string, enum: [standard, llm_wiki] }
    computed_at: { type: string, format: date-time }
    stale_pages: { type: integer, minimum: 0 }
    rules: { type: array, items: { $ref: '#/components/schemas/LintRuleSummary' } }
LintRuleSummary:
  type: object
  additionalProperties: false
  required: [rule, count, count_capped, items, next_cursor]
  properties:
    rule: { $ref: '#/components/schemas/LintRule' }
    count: { type: integer, minimum: 0, maximum: 10000 }
    count_capped: { type: boolean }
    items: { type: array, maxItems: 5, items: { $ref: '#/components/schemas/LintItem' } }
    next_cursor: { type: [string, 'null'], maxLength: 2048 }
LintPage:
  type: object
  additionalProperties: false
  required: [mode, computed_at, stale_pages, data, next_cursor]
  properties:
    mode: { type: string, enum: [standard, llm_wiki] }
    computed_at: { type: string, format: date-time }
    stale_pages: { type: integer, minimum: 0 }
    data: { type: array, maxItems: 100, items: { $ref: '#/components/schemas/LintItem' } }
    next_cursor: { type: [string, 'null'], maxLength: 2048 }
```

| LintItem分支 | 必填成员（均另有rule/item_key/cursor） |
|---|---|
| wanted | target:string（UTF-8≤256B），target_cut:boolean，pages:integer≥1 |
| orphan/frontmatter/missing | page_id:uuid |
| ambiguous | page_id、target、target_cut |
| invalid_sources | page_id、target、target_cut、not_link:boolean |
| undigested_sources | node_id:uuid、kind:page/asset、bytes:int64≥0、created_at:date-time |

item.rule为判别字段，items必须与所属summary.rule一致；JSON Schema必要时以每规则summary oneOf钉住，服务端/契约测试不只靠TS。item_key maxLength=140且ASCII固定前缀，cursor≤2048。target byte界不是OpenAPI字符maxLength所能独自证明，由真实适配器测试量UTF8。REST不传展示path。

分页若summary预览被Fit减到0：有数据的规则`next_cursor`使用专用**StartCursor**（V/R/N/S加`B:true`，不得与其他排序字段共存）。规则items接口收到它从真正首项开始；它不是默认全零UUID/时间/计数。没有数据规则才null。正常REST固定前5，不触Fit0，但接受StartCursor与MCP互通。

### 3.13 MCP 两入口、Fit 与内置说明

wiki.Module.Tools()在bootstrap.mcpTools真实拼入；notebook、account入口均登记lint/pending_sources。account照其他工具带notebook UUID，notebook入口不给该参数。工具描述直说“按调用快照计算；pages may move between calls；count_capped是下界；target_cut不是完整身份；用next_cursor续读”；pending_sources就是undigested_sources同一用例。

每个工具经mcpserver.Fit与真实SDK TextBytes≤49152，不能拿len(json.Marshal)替代：

- 规则页：Fit(n, prefix k)保留mode/time/stale；k<n时cursor=最后实际项的cursor，k=n用app next；必须至少有1项推进。以最大合法路径/256B目标/最长cursor实证1项仍能放下；若不成立先修投影与契约，禁止空data+原cursor无限循环或截断可执行path。
- 摘要：Fit(5, k)表示**每一规则均取min(k,len(items))**，计数/capped/stale/time/mode始终保留；不同规则不能依次吃完预算。k=0时非空规则带StartCursor；k>0但裁短时带该规则最后实际项cursor，否则沿app next。每个候选都用真实SDK编码计数。
- 共享结果不被Fit改写；截取、追加cut标记都用caller私有投影。REST与MCP比较去掉path后的完整值，而不比较不同快照的computed_at字面。

当前内置说明notebook=3957/account=4027 SDK字节（原始字节3867/3936），只有139/69余量。S1重写共用pathsAndTools，保留每工具一行的检查、路径语义、版本/锁/batch差异、大请求串行建议；添加：

```text
- lint: health checks for this notebook; omit rule for counts and previews, or give rule and next_cursor for more. Counts at the cap are lower bounds.
- pending_sources: undigested sources of an LLM Wiki, with paths, kinds, sizes and creation times. Follow next_cursor; empty source pages are omitted.
```

缩短候选共用文字如下（保留两入口现有intro/conventions，account的notebooks/open_notebook行照旧；这是正文建议，实际SDK核验后定数）：

```text
Paths
- Paths start at the notebook root; a leading / is optional. Join segments with /. No ./, .. or empty segments.
- Pages are ancestors' titles plus their title and .md: Projects/Roadmap.md. A page also holds children.
- Attachments keep their names (never ending in .md): Projects/diagram.png. A page titled x.png is x.png.md.
- Segments compare without case. Tool paths address nodes; content links use wiki targets: [[Projects/Roadmap]].
Tools
- list: children of the root (empty or /) or a page (Projects/), in tree order; other patterns are path globs. * and ? stay in a segment, ** crosses segments. Follow next_cursor, even after an empty costly-glob page.
- read: stored lines, using offset/limit, about 45 KiB per call. Continue with next_offset and next_column for a long line. Attachments return metadata and an image or download link.
- grep: Go regular-expression matches with paths and read's line numbers; glob narrows pages.
- search: words and Chinese text in titles and content, best first with excerpts. Bulk queued pages reach search and keyed grep once indexed; building reports this.
- backlinks: linking pages with context.
- index: pages by type, tag or parent, with summary, tags and inbound counts.
- write: create a page and missing parents without base_version; replace with the version you read.
- edit: replace old_string byte for byte with new_string; it must occur once unless replace_all is set.
- move: rename or move to the full new path (B/x.md, not a destination folder). Links follow unless update_links is false.
- delete: trash a node; a page with children needs recursive.
- batch: up to 100 write/edit/move/delete ops, one change and message, all or none. Each sees earlier ops; base_version is from before the batch. Links follow all moves together, from before the batch or their page's last write, ignoring intermediate changes, including alias changes. An edited page refuses at its write.
- versions: list versions or read one revision's lines.
- log: newest changes; path, actor (me) and client (mcp) filter them.
- changeset: one change. People restore or revert in the web; agent history is read-only.
- lint: health counts/previews, or rule items continued with next_cursor. Capped counts are lower bounds.
- pending_sources: LLM Wiki sources still undigested, with path/kind/bytes/created_at; follow next_cursor. Empty source pages are omitted.
- Versions are <node id>:<revision>. If base_version is stale, read again.
- Web editing, including your own, refuses writes, edits, deletes and moves that rewrite links. Wait for edit mode to end; the lock cannot be taken over.
- Calls over 64 KiB run one at a time per token; the next waits at most 2s. Send large writes sequentially.
```

以TestTheBuiltInInstructionsNameEveryTool守每工具一行，TestTheBuiltInInstructionsAreShort改为每入口`TextBytes<=4096-300`并对SDK真编码。实际字节数与余量写S1结果和M11既有handoff；本文不编造修改后数值。P5系统段、prompt与真实客户端人工验收仍P5。

## 4. 网页

### 4.1 S1随契约交付

生成TS schema、problem两语言、`W/test/lint-server.ts`与其独立测试。Web REST fake按REST用例码次序/规则适用性/游标scope/start/capped/item_key/必填mode写；unknown rule404、普通专属409、badcursor400、limit422、busy503带Retry-After都能造。它只模拟已给定规则事实与分页，不复制SQL/性能算法；同页不同显示前缀也用item_key区分。S1不加未使用的health service导出或导航，以免knip失败。

### 4.2 S2文件与九项具体决定

新增`W/pages/notebook/health-page.tsx`、`health-rule.tsx`、`health-item.tsx`、局部`health-load-more.ts`、`W/services/wiki.service.ts`与测试；RootStore只提供每loginId绑定client的service，不存结果。routes的NotebookLayout下加health；NotebookPlaces在History前加Health（reader也见）；useNotebook/useDocumentTitle复用。

1. **每次打开**：直接访问、左栏重进、从条目Back、Back/Forward到健康页每次都新运行；同地址NavItem再点击也触发一次打开标记。key=`['lint',loginId,workspaceID,notebookID,location.key,entryNonce,generation]`。entryNonce每次进入/重新挂载变；不能因history重用location.key恢复旧结果。离开清该entry所有summary/cards的SWR缓存（含inflight/dedupe）；只保当前打开，避免Back永久Loading。
2. **刷新与旧在途**：点击重新检查创建下一generation、abort旧summary/cards，旧数据暂留且标“正在重新检查”；成功原子替换summary，所有卡回到新预览、清旧更多页/错误/展开数量；失败保留旧computed_at与卡、显式错误/手动重试。旧more即使cursor与新轮相同也不得append、播报或聚焦；guard同时核session/notebook/entry/generation/rule，不只核cursor。
3. **更多焦点**：健康页局部wrapper复用LoadMoreButton/watchReader，但成功新增且用户未移走时，无论是否末页都聚焦首个实际新增项；与现useLoadMore默认“末页才移动”不同，不能改其默认而回归搜索。重复页不算新增；空尾页按钮消失时回最后实际项或卡标题；用户滚轮/键盘/指针已移走则不抢焦点。快速双击只一请求；output从首次render即存在，同文本再次播报也有效。
4. **busy与retained error**：summary与每卡自己的onErrorRetry遇到server_busy直接return；其他错误沿既有有限重试策略。已有data也渲染独立错误行+Try again，不让NotLoaded只在data===undefined掩盖刷新失败；卡重试只重该卡页，不重跑summary。fake timers跨所有自动delay后仍仅一次busy请求，点击才第二次。
5. **modeMismatch**：summary.mode或新卡page.mode与useNotebook.mode不符，保留旧快照作说明、显示“模式已改变，请重新检查”，禁所有More，保留重新检查按钮。专属卡409也锁More并提示，不自动算。真实mode事件只更新NotebookStore；漏事件后reconnect重读notebooks发现差异，lint请求数不增。刷新成功移除旧规则；standard也有四通用规则。
6. **路径**：取当前PageTreeStore全部node和ancestor，复用nodeTarget/openOf；不能distinctName、裸UUID、拼假asset URL。树首读中健康结果仍可显示target/count，node位显示Loading；树失败有自己的NotLoaded/retry，不重跑lint。结果的node树里缺失，当前entry共享一次tree reload；仍缺保留条目/计数，显示“节点已不可用”，不隐藏成count不符。路径按当前树变化，移动后的asset点击重取当前parent，落父页并state.attachment定位；这是当前导航而非旧健康snapshot的路径保证。只把nodeTarget/useNodeNavigate的通用片段窄移到通用文件；不把duplicate专用refusal文字带过来。
7. **取消**：wiki service的两个GET接可选AbortSignal；每代一个summary controller，每卡一次more一个controller；退出/刷新/session替换abort。所有副作用仍有scope guard，因为fake/代理/服务端可能已完成而abort不能撤回答复。HTTP abort只结束该caller等待；服务端共享flight不被leader断开取消。测试分别观测request.signal.aborted与晚promise不污染，不把二者混称。
8. **零规则/顺序**：固定wanted、orphans、ambiguous、invalid_frontmatter、missing_type、missing_summary、undigested_sources、invalid_sources；只列适用的。非零各一卡，0放一个可展开“没有问题（N项检查）”details，展开列规则名/说明/0，不能凭零伪造more。count_capped用Intl.NumberFormat(10000)+“+”；target_cut有文字标识；每列表aria-label；重新检查aria-busy，顶部总在output播报实际可用结果；stale>0说明请管理员运行nervewiki reindex。
9. **P4锚点**：固定`#undigested_sources`，卡DOM id同rule；从P4目录来源行进health#undigested_sources时正常新运行，结果到达后展开对应卡（即使0则展开零规则区并聚焦该规则标题）。仅当用户未动作且仍同entry才聚焦；本Phase不增加目录入口。日期使用现formatDateTime完整本地日期/时间，不只手写HH:MM。

卡state与summary都在SWR，不新增按本的结果store。完整item_key用于跨页去重；同目标的两种原始写法归同wanted组，但截断后同文的不同组仍显示两项；保留旧数据只属于正在显示的这次打开，不能跨login/notebook/entry共享。

### 4.3 双语与可访问性

补规则名/一句解释、健康入口、计算时刻、刷新/正在检查、分页不同快照说明、10,000+、截断、not_link、stale管理员动作、模式变化、节点不可用、零规则details、更多新增播报。locale键Record<LintRule,…>穷尽所有规则，Messages测试保双语同键/占位符；不要把实现术语flight/SWR/SQL放产品文字。沿现Panel/NotLoaded/LoadMoreButton/链接与排版，不做无关视觉打磨。

## 5. E2E：LW8

新增`e2e/stories/wiki/lw8-health.spec.ts`，S1 API和MCP、S2 page在同文件分组。fixture准备走真实REST、每MCP scope独立PAT，不假造数据库返回；只读故事不新建assert/wiki.ts，不重复断言每次种子的DB写入。大量分页种子由既有fixture/合理请求期限准备，不让4s请求期限把种子阶段伪装成lint失败。

| 版本 | 真实动作与断言 |
|---|---|
| API（S1） | 两模式、reader、8规则语料；standard四条、LLM八条；新建中英文LLM两本干净；summary预览→rule游标limit=1→终页；目标截断但item_key不同；source空页/asset/创建时刻；更新一页后下次算见变化；读者、不可见404 |
| MCP notebook/account（S1各一组） | tools/list完整对象；lint摘要/单规则，pending_sources与REST undigested投影一致；account显式notebook；分别unknown rule404、standard pending409、独立PAT的reader成功；不可见本+badcursor在notebook入口HTTP404、account入口tool400，分清Gate与工具错误；cursor真正跨中页续走 |
| Page（S2） | 从左栏健康真实打开；全部非零卡/0折叠、读者入口；更多新增首项焦点、截断/not_link、刷新后变化；真实page点击/Back/Forward每次请求新一轮；asset跳当前父页及attachment焦点；375px布局、window sentinel不reload |
| Mode（S2 page） | 两context，持原事件连接，另处降级，当前页只出现模式提示；点击重新检查变四条。另独立用例断线漏mode再reconnect，仍只有notebook读取而lint不增；不能拿第一例冒充第二例 |

busy、取消同游标、错误保留等精细交错主要在Vitest及S1 Go；真实page至少失败→手动retry成功并观测请求次数，PageWatch照常检查console/CSP。三浏览器焦点探查按已有经验执行并记录；正式全套仍按仓库Chromium配置，不改全局项目矩阵。

## 6. 实施步骤：每个提交独立为绿

以当前接受设计后的main建Step分支；由实施者按下列提交单元交付，每个单元先有能失败的测试/反例再实现。提交信息统一`<范围>: <具体结果> (M10/P3/S1)`或S2，不把未绿的骨架拆成一个提交。本文是Phase设计兼实施顺序，不另造docs/superpowers目录。

### 6.1 S1提交次序

| 提交 | 完整内容及文件 | 本提交的最小证据 |
|---|---|---|
| S1-0 容量处置 | root按§7.6核定已触线后的窄移界；Makefile/ci注释当前布局、清单生成核对；不改超时 | 旧470清单按候选167/70/118/115；完整新布局本机双轮，先证明现有测试不丢不重；结果是本机，不代后续新代码/main CI |
| S1-0a 来源表示/版本兼容 | §3.0的writer/等长bool/generated与Extractor 2同提交；API空key/asset/Web、literal v1/读者/真实重建/升级说明和必要生成物；不新增lint operation | OF/OS/OB/OW及每提交build/vet/模块lint/short，Web同时tsc/knip/vitest；当前v1说明按§8.1a同期修订。此时无D/F实现，不能宣称S66/67/69已跑 |
| S1-1 数据读取 | sources迁移、page.SourceArea.Nodes/LintNodes、linking heads/短边头/精确边读、sqlc生成；对候选复合索引先量后定；索引名/计划/物理范围/metadata-only测试与独立Makefile行 | 真实>10k整区、root/空页/trash/asset/currentbytes/created_at、explicit standard root；source索引缺失与keyset退化反例；所有模块lint/build/short |
| S1-2 规则内核 | domain canonical/sources/身份游标、linking规则读与聚合、真实SQL与各口fakes、sources旧对象/数组纯oracle；合法parser转义大target/JSON边界读与计数；完整count/preview/items一致；README迁移测量入口可以随本提交 | 表格、跨拼法union、8规则独立真库、上下界、Unicode/长JSON/TOAST；本提交不公布不存在的REST |
| S1-3 运行编排与内部授权 | wiki app Execute/Runs/recover/Stop、模块根窄内部入口、bootstrap app.run/close全出口接线；同提交ActionLintRead、wiki.Actions与access规则/规则测试；真实snapshot/配额/leader和follower取消/撤权/生命周期 | 真Authorizer成功进入内部用例；actions全集测试绿；PG backend快照与清理占名额；panic/Stop/所有退出真实接线，尚无HTTP路径 |
| S1-4 REST契约 | api/wiki、wiki HTTP、bootstrap Register、两REST矩阵/规则路径校验、错误双语、所有生成物和lint fake/fake tests一次提交 | 真实get/list、逐码次序、每个声明码至少一适配器答出、TS/knip/vitest、schema路由一致 |
| S1-5 MCP契约 | wiki两工具/两入口真实mcpTools拼接、tools.json完整对象、mcpToolCodes/矩阵、Fit/StartCursor、缩共用说明与余量测试 | tools/list、两scope逐码、REST对照、真实SDK48KiB、Fit0及中页续、说明余量≥300 |
| S1-6 故事与最终量 | LW8 API/notebook MCP/account MCP；100k八规则/summary成本、分配、长属性/candidate形状、索引保存与迁移记录；新增测试真实容量与最终门禁 | §7全表、每条成本行双轮与真实CI；新增代码不把旧布局样本充当当前完整两轮 |
| S1-7 审查与文档 | 三审/逐组修复/原席复核；root受控脚本应用§8的S1文档与审查记录 | 全变异复跑、完整门禁、精确分支CI绿；merge/record两main样本后才能放行S2 |

S1-1的可选索引若还没有证据，不可先提交“未来会测”的迁移；可以同一提交准备query/tests、在未提交工作树测清再决定包含哪个索引。S1-2/S1-3不提前增加未接线的公开HTTP/MCP operation。S1-3的lint.read内部action已有Execute使用者，必须同提交注册wiki.Actions与access rule；action并不要求先有公开operation，不能混为一类。新HTTP码/生成物/双语与真实handler必须同提交。

S1-0→S1-0a→S1-1..S1-7顺序固定。S1-0a不是新Step或维护旁支；属于S1的一个独立绿提交，不把版本与writer拆成过渡提交。S63/S64/S65/S68在S1-0a已有catcher后执行，S66/S67/S69待S1-2真实规则/生成查询到位；S1最终复跑本Step原S01–S62及新增S63–S69共69行（含S68既有属性面板的Web控制），S2仍交W01–W10十行，Phase总79行不漏。重建成本原件可在S1-0a建立代表样本，§7.4/7.5完整成本与最终输入复核在S1收尾完成。

### 6.2 S2提交次序

| 提交 | 内容 | 证据 |
|---|---|---|
| S2-1 健康首屏 | WikiService+RootStore注册、路由/导航、summary、全部规则投影、0/stale/截断双语、真实树path；相关组件与服务同时被消费 | load失败→重试、再读变化、reader/两模式、type/knip；不留不用的导出 |
| S2-2 生命周期与更多 | entry/generation、每卡SWR、local焦点wrapper、start/next cursor、mode mismatch、busy手动、retained errors、abort与晚答抑制 | §7 Web交错全覆盖，同cursor跨代、Back/Forward、fake timers、真实handlers/reconnect |
| S2-3 LW8 page | 真浏览器故事、附件父页与焦点、375px、模式两种交错、三浏览器定向焦点探查 | 全LW8与现SE1/LW3/LW12回归；不改搜索默认焦点/缓存 |
| S2-4 审查与记录 | 三审/修复/完整变异与门禁、§8文档；merge/record后P3done | 所有实际CI绿与容量；各事实可追原件，P4保持未开始 |

## 7. 测试、交错、变异、成本与容量

### 7.1 功能测试归属与可执行命令

以下测试名是实施时必须创建的真实测试，不是已存在绿证据。命令从仓库根经统一heavy锁执行；本机Go一律-p=1（make门禁由GOFLAGS=-p=1传入），不能另开锁并行Docker/PG/构建/变异。表内Go命令写成`go -C server test -p=1 ...`，实施执行时前缀均为`docs/v0.1/claude_to_codex/tools/heavy.sh`。

| 缩写 | 精确命令/拟文件 | 用途 |
|---|---|---|
| PD | `go -C server test -p=1 -race -count=1 ./internal/modules/page/adapter/postgres -run '^TestLint(SourceAreaIsComplete\|MetadataKeepsItsScope\|MetadataReadsNoBody)$'`；`lint_metadata_test.go` | source完整性、metadata/TOAST |
| LD | `go -C server test -p=1 -race -count=1 ./internal/modules/linking/domain -run '^TestLint'`；`lint_target_test.go/lint_sources_test.go/lint_cursor_test.go` | canonical/JSON/identity/cursor纯规则 |
| LQ | `go -C server test -p=1 -race -count=1 ./internal/modules/linking/adapter/postgres -run '^TestLint(Rules\|CountsAndPages\|Sources\|MissingProperties\|WantedUnion\|EdgeBatches\|CrossNotebook)'`；`lint_test.go/lint_sources_test.go` | 实际SQL、cap前筛选与分批 |
| LP | `go -C server test -p=1 -count=1 -v ./internal/modules/linking/adapter/postgres -run '^TestLint(Plans\|PhysicalRanges\|JSONToast)'`；`lint_plan_test.go`，!race、short跳过 | 四种计划状态、真实PREPARE/工作量 |
| PP | `go -C server test -p=1 -count=1 -v ./internal/modules/page/adapter/postgres -run '^TestLintMetadata(Plans\|PhysicalRanges\|PresenceDoesNotReadAncestors)$'`；!race、short跳过 | page metadata真实Planned路由与自身读取范围；不代生产wanted消费链 |
| BC | `go -C server test -p=1 -count=1 -v ./internal/bootstrap -run '^TestLintOfAHundredThousandPagesKeepsItsTimeAndMemory$'`；!race、非short | 复用§7.5第2行摘要成本，真实wanted consumer/SQL trace；不另增大suite |
| WA | `go -C server test -p=1 -race -count=1 ./internal/modules/wiki/app -run '^TestLint'`；`lint_test.go/lint_runs_test.go` | 访问/码次序/flight/cleanup/immutable |
| WH | `go -C server test -p=1 -race -count=1 ./internal/modules/wiki/adapter/http -run '^TestLint'` | HTTP binding/schema/headers/码 |
| WM | `go -C server test -p=1 -race -count=1 ./internal/modules/wiki/adapter/mcp -run '^TestLint'` | 真实SDK字节、Fit共同缩预览、中段和0预览 |
| BL | `go -C server test -p=1 -race -count=1 -timeout 15m ./internal/bootstrap -run '^(TestTheLint\|TestLint\|TestTheWikiTools)'`；`wiki_lint_test.go/interleavings_lint_test.go/wiki_lint_runtime_test.go` | 整个程序最后一跳、真实PG交错 |
| MX | `go -C server test -p=1 -race -count=1 -timeout 15m ./internal/bootstrap -run '^(TestThePermissionMatrixCoversEveryOperation\|TestThePermissionMatrixCoversEveryTool\|TestTheToolListsAreTheContract\|TestLintMatrixTargets\|TestTheWikiToolsAnswerEveryCode\|TestTheBuiltInInstructionsNameEveryTool)'` | 矩阵、工具表、码、内置说明 |
| AC | `go -C server test -p=1 -race -count=1 ./internal/bootstrap -run '^TestTheRuleTableIsTheModulesActions$'` | S1-3 action全集与真内部授权；现有actions_test.go确切测试名 |
| MI | `go -C server test -p=1 -race -count=1 ./internal/modules/page/adapter/mcp -run '^TestTheBuiltInInstructionsAreShort$'` | 两入口真实SDK≤3796，独立于bootstrap工具名检查 |
| WB | `pnpm --filter @nervewiki/web exec vitest run src/pages/notebook/health-page.test.tsx src/pages/notebook/health-rule.test.tsx src/test/lint-server.test.ts` | 健康/fake竞态 |
| EW | 下方同一heavy锁中的“build当前源→指定Playwright”命令 | 每轮重建真实嵌入前端；正式门禁仍make e2e |

**S1-0a的拟新增catcher与精确命令**（名称须实际创建；本提案没有执行）：

~~~sh
# OF：真实parser事实；quoted-empty/wiki、quoted-empty/markdown、single/double、same-page-body。
docs/v0.1/claude_to_codex/tools/heavy.sh go -C server test -p=1 -race -count=1 ./internal/modules/linking/adapter/markdown -run '^TestLinkOriginFacts$'
# OS：实际writer与PageProperties；NULL与空串必须分别断言。
docs/v0.1/claude_to_codex/tools/heavy.sh go -C server test -p=1 -race -count=1 ./internal/modules/linking/adapter/postgres -run '^TestLintOrigin(Rows|Properties)$'
# OB：真实组合根内容写/读取/管理员重建路径与REST；不拿手工Facts替代合法来源见证。
docs/v0.1/claude_to_codex/tools/heavy.sh go -C server test -p=1 -race -count=1 -timeout 15m ./internal/bootstrap -run '^TestTheLintOrigin(Compatibility|Reindex|Properties)$'
# OW：在既有文件内新增describe("empty property key")，三个真实渲染用例。
docs/v0.1/claude_to_codex/tools/heavy.sh pnpm --filter @nervewiki/web exec vitest run src/pages/page/page-properties.test.tsx -t 'empty property key'
~~~

OF期望来自字面文档/位置，不调用被测InFrontmatter产生期望。OS的TestLintOriginRows/current-format断言literal 2、empty-path非NULL空串、相同页body为NULL、named路径不变，覆盖AddPage/ReplacePage；TestLintOriginProperties/empty-key包含page/unresolved/asset并排除body。OB的Compatibility/literal-v1先建真实旧格式fixture（明确literal extractor=1、empty-path已塌为NULL），证明新写=2、旧页不能靠tree-only/no-op升版、Views fallback/rename与batch skip/backlinks计数保留而无context；不能用Extractor-1构造“旧”以免变异变成非法0。Reindex各子测试经实际bootstrap.Reindex命令编排，按下表分别断linking与queue的最终状态、命令结果及全本继续；核内容/revision不改，不能把非nil命令错误等同于整本未变。它们仍在同一OB总选择器内，用index/现有读者断言，不依赖尚不存在的D/F。Properties/empty-key经真实REST及asset签名，OW分别核resolved page/unresolved link/attachment的exact-path消费。

**OB重建子名与状态oracle（首轮A finding1/B-M1/C-I1修订；均待实施）**：下列为TestTheLintOriginReindex下必须创建的实际子名，已有OB的^TestTheLintOrigin(Compatibility|Reindex|Properties)$选择器全部涵盖。故障三组及clash-queues分别再有single/all子项：single调用指定notebook，all调用全本且放一个后续可成功的本作继续执行证人；断错误/输出和真实数据库状态，不只看退出状态。

| 子名 | 真实输入/故障与独立期望 |
|---|---|
| literal-v1 | 合法literal 1旧链接行经真实parser/新writer重建为literal 2，正文与revision不改，合法空属性仍非NULL空串；linking成功不靠最终命令结果推断 |
| linking-fails-queue-succeeds/single、all | 在测试自有库用触发器令linking事务已开始修改后的真实写失败；旧linking/rekey快照保留且无该阶段提交事件，ctx仍可用，缺失/旧tokenizer节点实际入队。命令返错；all仍处理后续本 |
| clash-queues/single、all | 用既有同胞键冲突路径；linking事实/旧键不变，搜索queue可提交变化，命令返错；all继续其它本。保留现有CLI撞键仍入队的断言 |
| linking-succeeds-queue-fails/single、all | 测试自有page_search_queue触发器使首批QueueSearch失败；linking已提交为literal 2及links事件，队列该批无部分写入。命令返错；all仍处理后续本；不把已提交v2回退为1 |
| queue-partial-commit/single、all | 准备超过10000个合格live节点，按确定id让第一批10000成功、下一批在真实QueueSearch失败；从另一个连接确认已提交的第一批完整保留，失败批无部分写入，linking仍已v2；命令返错、all继续。不可缩小生产requeueBatch或把两批放进测试外层事务伪造原子性 |
| tokenizer-eligibility | 同一真实命令下混合缺失、旧tokenizer、当前tokenizer；前两者入队，当前行及其原有queue状态不因本命令被改写。不得把入队成功当搜索job已构建；当前/旧版本事实用独立预期 |

错误由实际命令的既有可控路径/测试自有触发器造成，不改生产编排或mock一个返回错；queue状态oracle在测试自有库不启动消费队列的search job，避免后台清队列干扰。queue阶段可用同一触发器屏障核linking已提交及其锁已释放，不声称linking锁持续覆盖queue。准备/执行代价仍须按§7.5重新量，不能因后批控制超过10000节点而增加原timeout或省略真实分批。

保留cmd/nervewiki/reindex_test.go现有TestReindexTellsANotebookWhoseNodesAreNotQueued（成功重建后queue失败、全本继续及单本合并错误）和TestReindexReportsClashesAndFailuresAndRefusesNoNotebook（撞键仍入队、linking旧状态及缺失本）。它们不被新OB替代，S1-0a用以下现有CLI选择器复核；本提案未执行：

~~~sh
docs/v0.1/claude_to_codex/tools/heavy.sh go -C server test -p=1 -race -count=1 ./cmd/nervewiki -run '^TestReindex(TellsANotebookWhoseNodesAreNotQueued|ReportsClashesAndFailuresAndRefusesNoNotebook)$'
~~~

S1-2在LQ中新增TestLintSources/origin与TestLintSources/origin-stale。它们含D First与After各有首/后一批非空合格行，空属性负例不能只落在未读批；F通过真正被pending消费的EXISTS，分别有digested-parent、undigested-parent负例和四种body-kind正例。current/stale目标、named属性、仅空属性与同页真实body/sources对照共用既有count/preview/page判定，预期为手写确切IDs。全应用mixed-v1的stale_pages另在TestTheLintOriginCompatibility/rules-after-S1-2加入，S1-0a不得预建无人调用的规则骨架。

EW不可再套表首的heavy前缀（避免嵌套mkdir锁）。基线、单项变异、恢复三轮各独立执行下列完整命令，不在变异中gen，不复用bin：

```sh
docs/v0.1/claude_to_codex/tools/heavy.sh env GOFLAGS=-p=1 NWIKI_E2E_VERSION=0.1.0-dev bash -euo pipefail -c '
make -j1 build VERSION="$NWIKI_E2E_VERSION"
sha256sum bin/nervewiki
pnpm --dir e2e exec playwright test stories/wiki/lw8-health.spec.ts
'
```

每轮留当前源输入manifest/diff SHA、web/apps/web/dist与server/internal/platform/webui/dist的文件hash清单、bin/nervewiki SHA、构建版本、完整命令与退出码，核Web产物被当前二进制嵌入。fixture只启动bin、不会代建；因此WB的promise/组件断言与EW的真实浏览器焦点/History导航义务分别记账，不能互抵。Go命令保存test2json或-v事件，指定catcher必须至少一个run/pass或预期断言fail；0测试/仅编译失败不算证据。MI/MX是不同包、不得拿工具行覆盖冒充SDK余量。

每条测试表至少非空正反例：真实正常emptyLLM两语言也是**先确认模板/规则实际存在**后的空结果，不拿两个空集合比较。规则fakes应拒错误scope/超大内部批/缺context，不用production canonical生成期望；手写有限oracle与真SQL逐行对照。新顶层分布见§7.6；短测试不能起数据库。

### 7.2 确定性交错与最后一跳

| 测试 | 怎样逼出/证据 | 能杀什么 |
|---|---|---|
| `TestTheLintSnapshotAndRename` | 种未消化asset和返回的区外orphan；测试连接ACCESS EXCLUSIVE持asset_blobs。真实MCP lint在规则算完后等附件metadata，pgtest.WaitForLockWaits只作全库等待提示；用query tracer取得lint backend PID，再以pg_locks/pg_stat_activity核该PID正等asset_blobs冲突锁；另一连接改名orphan并提交，放锁。返回旧path；trace记录StateOf最先、所有query同backend/snapshot/RR/read-only、Paths最后 | 去掉snapshot/某端口用pool丢ctx/提前读展示path（需另外顺序观测） |
| `TestTheLintSnapshotAndDowngrade` | 同上阻点，另一连接降级提交；getLint mode=llm_wiki且LLM规则仍在，同一次读；结束checkPages/checkLinks/sourcehash | 取外层mode与规则不一致；**不宣称单独杀去snapshot**，无snapshot也可能保旧mode |
| `TestTheLintQueuedModeIsReadAgain` | 最小配置1slot，第一个真实run持asset锁；第二个专属rule已过外层mode且加入等slot（只观察的hook）；降级、释放第一个，第二个取得slot后StateOf读standard，409 | 只有外层模式预检/用旧root运行 |
| `TestTheLintFollowersSurviveTheirLeader` | 同key两真实HTTP/MCP调用（两不同reader PAT），确认worker一次、第二已join，再cancel leader；放数据库barrier，follower成功，worker1次 | leaderctx继承、每caller先占slot、共享结果被一callerFit修改 |
| `TestTheLintFollowerCancelsAlone` | leader、F、G以不同caller ctx/PAT确实join同key（观察hook确认3人）；worker持DB barrier。只cancel F，先确认F取消返回，leader/G与worker仍等待；放锁后两者成功，计算1次且slot只归还1次。测试等待均有界 | 只让leader监听Done、follower主动取消仍等待共享结果；不与撤权混算 |
| `TestLintWorkerRecovers` / `TestTheLintWorkerRecoversThroughModule` | app用可控Snapshots在callback读口panic，清理barrier先被触发；再模块组合真PG端口，仅在一个确定读口执行原调用后panic。followers均internal、不挂，pool/slot/flight清理，同key下一次成功；另让panic与cleanup错误/expiry交错，终结一次且日志无panic秘密 | 缺worker自己的recover、清理前release、双发布、泄漏flight或原panic日志 |
| `TestLintRunsStop` | 先确认一运行一排队worker均未到期，Stop与join同mutex交错；在各worker原deadline前见ctx取消、caller醒，cleanup前slot不归还，closed后同key不开新worker。另一例让worker已deadline取消并从flights摘下，持独立cleanup barrier；Stop仍等其active，放行后才drain。重复/并发Stop等同一drained，有界超时不假成功 | closed后Add、漏cancel未到期运行/排队worker、仅按accepting flights等而漏expired-cleanup导致过早drain/pool close、重复release；expired重复cancel单删记等价 |
| `TestTheLintStopsWhenAppRunEnds` | 真newApp上游ctx保持alive，先确认独立lint worker开始；分例run正常ctx结束、已占地址造成ListenAndServe失败、startJobs启动失败、run前置已取消ctx/DB或迁移失败。实际app.run返回前必须Stop/drain，trace见worker取消/清理先于pool.Close；close再调幂等，另测只构造就close。对真实app.run defer和close兜底分别删接线 | 只监听newApp ctx、漏run某出口Stop、pool先关或只构造路径漏Stop；module单测不代替真实接线 |
| `TestLintCallersKeepTheirAuthority` | A已join运行；B不可见先404且不join；C可见join后在另一事务撤权限，放barrier，A200/C404。准备与断言日志以各自起点 | 只授权leader、结果前不复核、跨本flightkey |
| `TestLintScopesNeverShare` | 同DB standard本，implicit摘要与explicitLLM+proposedRoot同时调内部入口，观察两flight、各mode/规则不同，explicit包含空page/asset并不靠hash | key漏Contract/AreaRootID、implicit外层读模式定scope |
| `TestLintBusyReturnsBeforeCleanup` | app层以真正Runs和可控Snapshots清理barrier分离deadline与cleanup，caller503先到、slot仍占、过期flight不再join，释放后新flight继续；另真PG取消慢查询+pool acquired归零+下一真实读成功，核Rows.Close/rollback | 在cleanup前release、过期flight当缓存、caller阻塞等cleanup、漏release |
| `TestTheWikiToolsReachServe` | 真实serve两scope调用新工具，返回非空不同规则及pendingasset；REST注册另真实GET；将bootstrap.mcpTools的wiki列表/HTTP Register删掉，应分别失败 | adapter测试够不到的最终接线 |

barrier只观察/暂停并随后执行原操作，不替换生产检查或建立外层事务（WithinSnapshot不接受嵌套）。所有等待有ctx期限、不sleep；等结束核query/log/真实写状态，不能只“调用次数到N”。长fixture在请求外准备；测试持锁不阻另一方真正需要的nodes/notebooks表。不同的失败义务不混称一个mutation都能证明。

### 7.3 逐条运行时变异表

每条先在干净树执行其同命令为绿，再只改该项、同命令观察**该断言**失败、备份恢复、不运行gen，最后同命令复绿。grep命中/编译失败不算杀死。更多审查变异追加在原表之后，不覆盖历史存活/不等价记录。

| ID | 改坏点（一次一条） | 真正抓它的测试、命令 |
|---|---|---|
| S01 | SourceArea包含root | TestLintSourceAreaIsComplete/root；PD |
| S02 | 来源改用source_hash非NULL/ProbeSources枚举 | 同测empty-page、explicit-standard-root、>10k；PD |
| S03 | 整区截10000或使用SourceHeads含trash | 同测large/trash；PD |
| S04 | asset以page_bytes=0答/created_at改updated_at | TestLintRules/pending-metadata与TestTheWikiToolsAreRESTs；LQ+BL |
| S05 | metadata查content列/对每页取正文 | TestLintMetadataReadsNoBody（TOAST实际读取正负控制）；PD |
| S06 | source/root/AGENTS/folder/stale在count cap之后过滤 | TestLintCountsAndPages：10000个应排项在前、至少7个合格项在后；LQ |
| S07 | wanted按lastkey合组 | TestLintWantedCanonical/paths-a-x-b-x；LD |
| S08 | Unicode用lower/漏NFC/full-fold | 同测Straße/STRASSE、组合字、多段；LD |
| S09 | relative用页自身folder/越根夹到根/保rooted斜线 | 同测nested-folder、above-root、rooted-pair；LD |
| S10 | raw COUNT页数相加 | TestLintWantedUnion/raw-spellings-on-same-page；LQ |
| S11 | 同source每批清seen | TestLintEdgeBatches/same-page-across-byte-and-row-cuts；LQ |
| S12 | 先计Node.js后按asset组减计数 | TestLintWantedUnion/page-and-asset-spellings；LQ |
| S13 | image/embed带扩展名也当wanted，或所有带点目标都排掉 | TestLintWantedCanonical/kinds-and-extensions、presence原name无扩展名/.hidden/尾点/多点/.md/casefold/全本source-stale候选；LD+LQ |
| S14 | virtual谓词在standard生效/LLM根规则漏relative绝对化 | TestLintWantedCanonical/virtual-scope；LD |
| S15 | 代表取过滤前最早边/最短raw | TestLintWantedUnion/representative-after-kind-filter；LQ |
| S16 | 用显示256B作map/item_key/cursorhash | TestLintCountsAndPages/long-prefix-distinct-groups；S1为LD+LQ，S2再以WB核两个同显示条目，S1 fake不冒称UI验收 |
| S17 | `:many`全本edges/忽略8MiB前缀/不消费cursor | TestLintEdgeBatches/max-materialized-bytes-and-progress；LQ；成本分配为补充 |
| S18 | keyset去notebook或tuple界，或每批从首扫 | TestLintPhysicalRanges/first-middle-tail-hidden-A-B-A；LP |
| S19 | 精确positions数组交叉配对/漏notebook复核 | TestLintEdgeBatches/paired-positions；LQ |
| S20 | orphans计自己/asset/来源或stale发出边 | TestLintRules/orphans独立四反例；LQ |
| S21 | 所有规则误用统一root/AGENTS豁免 | TestLintRules/root-agents-scopes；LQ（坏sources报、fenced不报） |
| S22 | 首type/summary键跳过非string找下一个 | TestLintMissingProperties/first-key-wins；LQ |
| S23 | jsTrim含0085或漏FEFF | TestLintMissingProperties/trim-codepoints；LQ |
| S24 | 只看257空白前缀 | TestLintMissingProperties/long-whitespace-then-text；LQ+LP |
| S25 | sources不用首ASCII键/把非链接strings或nested array/object当link | TestLintSources/first-key-and-json-items；LQ+LD |
| S26 | actual unresolved sources边一概从wanted删除 | TestLintSources/real-link-and-not-link-boundary；LQ |
| S27 | L/J同文字合并，或按截断文字去重 | TestLintSources/identity-tags-and-long-values；LQ |
| S28 | 来源发出边全部丢弃使附件不能跟随 | TestLintSources/attachment-follows-source；LQ |
| S29 | 多父须全已消化/直接wiki引用不能覆盖/空或stale父吞附件 | TestLintSources/multi-parent-direct-empty-stale；LQ |
| S30 | 0字节只豁免有子来源页，或区外空叶当folder | TestLintRules/empty-page-versus-folder；LQ |
| S31 | 去sources部分索引/first probe不LIMIT1 | TestLintPlans/first-sources-probe（真实SQL计划+读工作）及5000属性无sources反例；LP |
| S32 | Planned改成默认cached且不覆盖真实generic | TestLintPlans/cache-route和actual PREPARE counters；LP（仅linking，page独立S49/PP；观察实际mode） |
| S33 | snapshot外运行或Paths端口丢ctx | TestTheLintSnapshotAndRename；BL |
| S34 | 快照首读不StateOf/排队后不复核mode | TestTheLintQueuedModeIsReadAgain与snapshot trace；BL |
| S35 | 继承leaderctx（原slot/scope义务拆S59/S60） | TestTheLintFollowersSurviveTheirLeader；BL+WA |
| S36 | 只授权leader/没有交付前再授权（各单改；原follower取消义务移S51） | TestLintCallersKeepTheirAuthority；BL+WA |
| S37 | deadline固定大于短合法request或等待不计时（各单改；cleanup移S61） | TestLintTimeoutFollowsTheRequest；WA+BL |
| S38 | worker error/Scan cancel漏Close/slot归还 | TestLintEdgeBatches/cancel-and-query-error、TestLintBusyReturnsBeforeCleanup；LQ+BL |
| S39 | shared结果被Fit原地截/并发写map | TestLintFollowersOwnTheirProjection；WA+WM，race |
| S40 | REST/MCP wiki注册删掉 | TestTheWikiToolsReachServe、TestTheLintReadsBothModesThroughServe；BL |
| S41 | rule route加入notTargets跳整条、漏notebook_id列归属 | TestLintMatrixTargets/foreign-notebook-column；MX |
| S42 | list先limit再notebook/mode/cursor | TestLintJudgesInTurn+HTTP多重无效输入逐次修复；WA+WH |
| S43 | Fit用rawJSON长度/单规则吃预算/少发最后item cursor | TestLintFitsTheSDK、TestLintSummaryShrinksTogether、TestLintPageResumesMidway；WM |
| S44 | preview0非空规则next=null或默认零位置 | TestLintSummaryAtZeroPreviewsStartsAtTheFirstItem；WM+WH |
| S45 | 删除生产内置说明的lint或pending_sources工具行（各单改；原字节义务移S57） | TestTheBuiltInInstructionsNameEveryTool；MX |
| S46 | 日志带target/key/path或只读前一子测日志 | TestLintLogsItsRunNotItsInputs；WA+BL |
| S47 | 同path≥2仍合法，或先过滤NULL/区外/asset目标再数唯一性（各单改） | TestLintSources/slot-collision-unfiltered-cardinality；LQ+LD，手写dotted-key oracle与跨批 |
| S48 | 共用SourceSlots把旧对象{0:string}错误当数组slot（仅改本期已交付纯matcher） | TestLintSources/old-qualified-json-oracle；LD+LQ；P4实际Capture/旧树负控明确留P4 |
| S49 | page metadata的Planned改cached，或生产wanted consumer重走ByKeys全祖先读（分别单改） | 路由由TestLintMetadataPlans/cache-route的PP抓；consumer由TestLintOfAHundredThousandPagesKeepsItsTimeAndMemory的BC抓，§7.5第2行一末键×深路径、非空wanted及真实query trace；不得用测试自造presence代替 |
| S50 | worker去recover/重新panic/先发结果再清理/原panic入日志（各单改） | TestLintWorkerRecovers、TestTheLintWorkerRecoversThroughModule；WA+BL，清理错误及expiry独立交错 |
| S51 | 已join的follower不监听自己的Done | TestTheLintFollowerCancelsAlone、TestLintFollowerCancelsAlone；BL+WA；F先取消而leader/G仍等 |
| S52 | Stop不拒新flight/漏cancel尚未到期运行或排队worker/仅按flights等而漏expired-cleanup导致active未清便drain（各单改） | TestLintRunsStop；WA，deadline前取消与expired清理barrier分例；重复Stop及超时正例；已deadline取消者省重复cancel且仍正确drain是等价，不强报kill |
| S53 | 删除真实app.run的Stop defer、或删除close前Stop兜底（各单改） | TestTheLintStopsWhenAppRunEnds/run-exits与construct-close；BL，newApp ctx始终alive，pool前drain |
| S54 | 只校外层cursor，不校内层K规范编码/256B（各单改） | TestLintCursor/inner-key-canonical；LD+WA，YQ/YR及超长反例 |
| S55 | slot heads投影任意长property_key/先物化后挑8MiB，或换页前清slot计数 | TestLintSources/short-heads-long-nonslot-key及cross-byte-row-batch、TestLintPhysicalRanges/source-slot-disjoint-batch-ranges；LQ+LP，单JSON合法大项进度 |
| S56 | 绕过notebook Gate或把account码次序套成入口HTTP404 | TestTheWikiToolsJudgeTheirEntryBeforeArguments；BL+MX，真实两scope多重无效逐次修复 |
| S57 | 生产内置文字真实SDK垫到3797–4096B，保留全部工具行，测试3796阈值不动 | TestTheBuiltInInstructionsAreShort；MI。MX工具行应仍通过；只放宽测试阈值不是变异证据 |
| S58 | S1-3把Execute授权action改为未登记字面量，或从wiki.Actions/access规则表删除相应行（各单改、保留声明可编译） | TestTheRuleTableIsTheModulesActions及TestLintReadsBothModesWithRealAuthority；AC+BL；真Authorizer到达snapshot |
| S59 | singleflight移入slot之后 | TestTheLintFollowersSurviveTheirLeader/join-before-slot；BL+WA，单slot下真实3caller加入 |
| S60 | key遗漏Contract或AreaRootID（各单改） | TestLintScopesNeverShare；BL+WA，implicit与两个不同proposedRoot的显式scope |
| S61 | cleanup前release/过期flight仍可join/旧worker删新flight（各单改） | TestLintBusyReturnsBeforeCleanup；WA+BL，deadline→cleanup→新同key有序 |
| S62 | 分别只将wanted或source-slot路径的target读取上界收紧为C=5MiB；JSON宽界不动 | TestLintJSONToast/legal-escaped-target/wanted与source-slot；LP各自基线/单改/恢复。真实parser→PageFacts→保存、手写a+U+2028重复+z/x、target_key=x；两生产heads/完整target均非空且规则确有项，非SQL伪种子或更早JSON失败 |
| S63 | linksParams仅用Property非空判origin，丢掉Quote（只改writer，不改期待） | TestLintOriginRows/current-format；OS。OF补事实见证，实际写入断言须失败 |
| S64 | 实际执行的generated InsertLinks只把property CASE改回NULLIF；bool参数保持可编译，不运行gen | TestLintOriginRows/current-format；OS，实际SQL的非NULL空串断言失败 |
| S65 | domain.Extractor回退1、writer保持新表示 | TestLintOriginRows/current-format及TestTheLintOriginCompatibility/literal-v1；OS+OB。literal 2/current、literal 1/legacy独立，不用current-1 |
| S66 | D First、D After、F的真正body谓词各自改为COALESCE(property_key,'')=''（三次独立单改） | TestLintSources/origin/d-first、d-after、f分别独立执行下方OQ命令；每次空属性目标会错入D或F，断言exact pending IDs |
| S67 | F的kind条件排除wikilink；另将link/embed/image各单独排除，共四次；无原kind条件时添加此真实过滤 | TestLintSources/origin/f-kind/wikilink、link、embed、image；OQ-K分别执行。普通body [[asset.pdf]]必须改变非空pending结果 |
| S68 | PageProperties三个平行SQL投影一致加property_key<>''（一次逻辑故障，数组仍等长）；另独立把Web taking的超长key排除改为同时排除key.length===0 | SQL故障：TestLintOriginProperties/empty-key与TestTheLintOriginProperties/empty-key，OS+OB；Web故障：OW的三个empty property key渲染用例。分别baseline/单改/断言失败/恢复，不用假服务端OW冒充SQL kill |
| S69 | D First、D After的SQL extractor复核分别移除；F生成父资格关系的有效current门移除，三次单改 | TestLintSources/origin-stale/d-first、d-after、f；OQ-S分别执行。D语句边界负控显式给含literal-v1的候选id关系，固定extractor=2，真实生成SQL须拒旧行；另有真实scope端到端排除，不能把只删冗余门的等价变异伪称kill |
| W01 | Back复用旧entry/inflight/dedupe；nonce单删若实际等价须记录等价理由，不强报kill | health-page“每次返回重新计算、在途离开再返回不Loading”；WB+EW每轮重build，选择实际会违约的重进场景 |
| W02 | more只比较相同cursor、不比较generation | health-rule“旧more在新summary之后答且两轮同cursor”；WB |
| W03 | busy沿全局自动retry/retained data不显示error | summary/card各“timer推进仅一次、手动再试”及“有旧data仍报错”；WB |
| W04 | mode事件/reconnect自动lint或仍可More | health-page“真实handlers mode/漏mode重连仅notebook读”；WB+EW |
| W05 | 非末页more不聚焦首新项/读者动过仍抢焦点 | health-rule“非末页首新项/reader moved”；WB+EW |
| W06 | item_key改target显示文本 | health-rule“同前256B两组/同页LJ”及重复页；WB |
| W07 | asset导航旧parent/拼pageURL/缺失项直接隐藏 | health-page“移parent后点击/树重读失败/已消失保项”；WB+EW |
| W08 | 去AbortSignal传递、或去晚答scope guard | 两个独立测试：signal已abort、忽略abort仍迟答无影响；WB |
| W09 | 卡重试重跑summary/刷新成功保留旧展开页错误 | health-rule“局部retry/新summary原子reset”；WB |
| W10 | output结果后才mount、0规则乱序、锚点不展开 | health-page“重复播报/固定序/零pending锚点”；WB |

**S63–S69执行分解与证据**：原S01–S62/W01–W10的72行文字及历史保持，新增7行为79行；一行内的每次单改都需自己的baseline→断言失败→恢复。以下给每个reader/kind完整选择器，命令中的子名也是必须创建的实际子测试名。D First/After低层SQL复核用过宽候选关系作防御负例，明确这是adapter语句边界测试，不冒称合法ReadScope或端到端调用；完整scope的stale过滤另由真实组合根验证。F以P关系为current门时就改该生产关系，不只删除被P重复保证的SQL条件；若实施中出现冗余门，记录等价并选会破坏真实资格的同一逻辑故障，不能取消本行语义义务。

~~~sh
# OQ：分别在D First、D After、F三个body故障下独立运行对应一行。
docs/v0.1/claude_to_codex/tools/heavy.sh go -C server test -p=1 -race -count=1 ./internal/modules/linking/adapter/postgres -run '^TestLintSources$/^origin$/^d-first$'
docs/v0.1/claude_to_codex/tools/heavy.sh go -C server test -p=1 -race -count=1 ./internal/modules/linking/adapter/postgres -run '^TestLintSources$/^origin$/^d-after$'
docs/v0.1/claude_to_codex/tools/heavy.sh go -C server test -p=1 -race -count=1 ./internal/modules/linking/adapter/postgres -run '^TestLintSources$/^origin$/^f$'
# OQ-K：每次只删除本行所检验的一个kind，四行分别执行。
docs/v0.1/claude_to_codex/tools/heavy.sh go -C server test -p=1 -race -count=1 ./internal/modules/linking/adapter/postgres -run '^TestLintSources$/^origin$/^f-kind$/^wikilink$'
docs/v0.1/claude_to_codex/tools/heavy.sh go -C server test -p=1 -race -count=1 ./internal/modules/linking/adapter/postgres -run '^TestLintSources$/^origin$/^f-kind$/^link$'
docs/v0.1/claude_to_codex/tools/heavy.sh go -C server test -p=1 -race -count=1 ./internal/modules/linking/adapter/postgres -run '^TestLintSources$/^origin$/^f-kind$/^embed$'
docs/v0.1/claude_to_codex/tools/heavy.sh go -C server test -p=1 -race -count=1 ./internal/modules/linking/adapter/postgres -run '^TestLintSources$/^origin$/^f-kind$/^image$'
# OQ-S：D两语句自身的current复核和F有效父资格分别破坏、分别执行。
docs/v0.1/claude_to_codex/tools/heavy.sh go -C server test -p=1 -race -count=1 ./internal/modules/linking/adapter/postgres -run '^TestLintSources$/^origin-stale$/^d-first$'
docs/v0.1/claude_to_codex/tools/heavy.sh go -C server test -p=1 -race -count=1 ./internal/modules/linking/adapter/postgres -run '^TestLintSources$/^origin-stale$/^d-after$'
docs/v0.1/claude_to_codex/tools/heavy.sh go -C server test -p=1 -race -count=1 ./internal/modules/linking/adapter/postgres -run '^TestLintSources$/^origin-stale$/^f$'
~~~

所有catcher保存至少一个指定子测试run与真正断言fail/pass，0 tests、编译失败、其它测试先失败均不算kill。S65的literal-v1兼容在S1-0a可由现有读者观察，S1-2之后追加真实stale_pages/D/F证明；不把阶段不足包装成已覆盖。P4 Unknown变异只记§3.8.2的后续义务，不加入P3已跑计数。

### 7.4 计划、物理工作、TOAST与迁移

**计划矩阵**在自己的库中跑，无统计/ANALYZE × force_custom/实际 PREPARE force_generic；另跑真实Store的DescribeExec多次，查pg_prepared_statements计数区分“无named cache”和“已实际generic”，不说Planned强制custom。种子：读者本1000–3000页，另一不可见本50000页以上；部分索引首键有一页1000无关属性、10000边及dotted冲突跨批、少/全source、空/10k/100k source参数；keyset分别第一/中/尾/空，合法最大内部batch512及8MiB、两相邻source跨批。所有planner开关正常，不以enable_seqscan=off伪造生产选择。

Physical读要计backend-local rows/scans/index tuples/heap fetched/blocks（含cache hit），先确认track_counts；用新插入无旧版本fixture为正证，不将数字推广到MVCC生产恒等式。隐藏本A→B→A改变相邻键分布，总结果不变，目标本读取关系/索引范围也不随别本规模/词变化；Btree高度控制。负控删notebook过滤、tuple条件、改LATERAL为全表join，每一条应真正失败。

正文TOAST放不可压缩大正文，lint不触page_contents TOAST；真的读正文为正控制，不能只grep SQL。属性TOAST分两类：正常prefix足够不读value；257全空白+后文必须读取的属性（缺/不缺都含）确实读且计时；sources JSON必要读单列，不误称所有TOAST为零。target TOAST也必要：短heads不读取完整target，所选positions读的字节和批上限有证人。把query text tracer当次序/投影证据，不能代替物理计数。

sources迁移和每个获准额外索引分别测：自有库10万页、至少200万properties/含真实sources分布的代表种子；空sources与密集sources两形状，独立完整Up事务至少两轮、表扫描/索引build/占空间、持SHARE导致写等待但读通过；记录PG/CPU/cache状态，暖表诊断不冒冷启动停机承诺。每个索引在5000条链接的一次生产保存上分别开/关对照、三种统计状态；sources索引还测1000无关属性与首sources位置。没有P3实测前不填ms/MB阈值；结果据§13.4三倍规则与最慢CI校准，迁移时间写升级说明/M12实机再测边界。

**来源表示成本与升级证据（决定62）**：bool输入增加一列有界参数，空路径新进入已有property部分索引，须在5000-links生产保存上与普通named/body对照计入，不能只量新增sources索引。另在测试自有库按代表notebook/page/link/正文形状实际执行新版reindex，分列每本linking事务的wall/CPU/峰值/锁等待、WAL/空间、成功提交或本阶段回滚，以及后续Requeue各批耗时/已提交批数/队列变化/失败位置；另记命令整体wall与结果，不能把全部queue工作计作linking持锁时间。当前tokenizer、缺失/旧tokenizer及之后搜索job构建分账；linking失败仍入队、linking成功但queue失败、queue后批失败保留前批都在失败/部分成功样本中核状态，不能按整个命令“回滚”记零或把search代价说成纯linking。代表样本接既有TestLintIndexesCostTheirScanAndWrites的独立!race recipe；若进程预算不足按§7.5实测拆分，不提高原timeout。本提案无新数值/阈值，历史330秒只为旧估计，M12仍须真实数据复测。

### 7.5 10万页成本与新行安排

成本测试!race、short跳过，每条在Makefile单独`-v -count=1`；新lint大成本优先进costs，当前guards已接近30min作业窗口，不把成本都塞进去。至少：

1. `TestLintRulesOfAHundredThousandPagesTakeTheirTime`（linking postgres）：八规则每一条分别计query/Go归组；summary实际组合以另条真编排测。wanted分别少数热目标×大量source、多raw拼法同页、全独特长目标、relative深folder、一末键×大量深路径同名page/asset（确认无候选祖先读）、多kind；一页10000重复目标跨批；来源10万集合与大量区外未解析边必须同场，记录excluded_ids每批序列化/分配与DB join/往返；另有大量0page/stale/root/AGENTS在cap前。
2. `TestLintOfAHundredThousandPagesKeepsItsTimeAndMemory`（bootstrap，放costs一行）：真snapshot所有端口、十万页摘要与单wanted页，统计GoTotalAlloc/峰值、各阶段读取工作；初始2s/250MiB只作目标；最终断言据真实双轮至少三样本中位数按13.4校准，不提高请求或旧cost阈值。 S49 consumer负控复用这条BC成本：在一末键×大量深路径page/asset形状中，种子准备结束后才开始trace，经bootstrap真实wiki实例的Execute运行摘要与单wanted页，让生产compactNodes→presence→ReadsAsAssetWith链消费候选，断言wanted非空及手写page/asset预期。读取trace须出现LintNodeMetadata与非空wanted heads/精确target查询，不得出现LinkTargetsByKeys或其全候选祖先读取；最终NodePaths.Paths只查答出ids，独立记录，不把这次合法展示读误判成候选祖先读。仅把生产consumer改回ByKeys时由这条trace断言失败；PP只核page metadata自身，测试不得直调NewLintNodes后自建presence冒充consumer。基线/变异/恢复各跑同BC并留非空测试事件；这条成本仍待量，不另增独立100k suite。
3. `TestLintPropertiesReadOnlyNecessaryValues`（linking postgres，costs独立行）：长type/summary前缀空白且最后有字/无字，完整sources值与非sources长值、JSON变换和分配；报告本本必要value总字节与最大一项。
4. `TestLintIndexesCostTheirScanAndWrites`（bootstrap，costs独立行）：§7.4迁移与5000links保存；如果完整迁移测量较长从CI小代表中分出独立本机大样本，明确哪个守卫在CI，不能省掉义务。
5. `TestLintPlans/PhysicalRanges/JSONToast`（linking postgres，costs各自或经实测可同一行）；page metadata实际范围/Planned为独立一行，一末键大量深路径的生产presence消费链复用第2行BC；不能混进race的窄时限测试。

每行本机两次的process均值×2.4，超过其既有Go进程期限一半（默认10m为300s）就拆成更细行；新行也不借本任务提高任何原-timeout。若单一新成本样本仍大，拆fixture/子形状为独立有完整正反证的顶层，而非删最坏形状。作业costs当前25m、guards30m，S1总recipe预估和实测均记录；若新增行让costs逼近其上限，按同一已测规则拆独立作业而非提高时限，需root具体处置及新全清单/CI。绝对算法门槛由真实快几次×3与CI余量定，进程容量倍数2.4不能误当算法门槛。

### 7.6 bootstrap容量：入场已在线上，先处理再增加

历史仍保留：S3指定merge/record Test均值438.5/447/304/364.5s<450；最新record/P2done为425/447/249/363与379/453/229/357，入场均值**402/450/239/360**，第二片恰450，不满足严格<450；这触发容量处置，不推翻S3已接受的指定配对。

已接受本机package均值311.268/317.5735/259.437/260.616；470顶层分167/74/114/115。`p3-capacity-readonly-s3-test-timings.json`从既有日志导出run→terminal，非新测量、非CI；第二片TheMCP35个合126.122377s、Batch3个57.999055s、Contents2个30.524814s、Source33个132.048248s。不能说整个MCP肯定装得下第三片。

**推荐具体候选（主持者实施决定54，仍待实测验收）**：FIRST/THIRD不变；SECOND的`The(MCP|Contents)`改为`The(MCP[C-Z]|Contents)`，即MCP在B/C前缀移界。当前恰4个顶层从二到三：BacklinksAreRESTs、BatchIsRefusedWhole、BatchMeetsAWebRename、BatchReachesItsUseCase；run→terminal双轮合均值59.183336s，×1.6=94.693338s。按新入场配对预测二/三为355.306662/333.693338s；按更保守S3指定pair447/304算352.306662/398.693338s。以上全是**候选估计**，不是CI或已批准布局。静态旧470变167/70/118/115；正式生成的测试清单必须一致才接受。

新增root顶层分布明确：`TestTheLint*`与`TestTheWikiTools*`在第三片；`TestLint*`在第四片；不再以`TestSource*`或`TestTheMCP*`新增重型P3套件挤第二片。八规则边界优先放domain/adapter/app合适层，root只留final-hop/快照/权限交错，不在每种协议重复一整套100k fixture。Round1新增panic、Stop全出口、follower主动取消、Gate与candidate/slot catcher后，原第三≤20s/第四≤35s预算不得原样称可达；暂把规划包络重估为第三≤25s（×1.6=40s，保守398.693+40=438.693）、第四≤45s（360+72=432），增加5s/10s只供排程，尚无新测量。这些是设计预算分配，实施必须实测，不以命名规避应有测试。超预算先减重复准备，不能删必要断言；按真实邻居剩余量再定窄移界或增份。

S1-0a的TestTheLintOriginCompatibility/Reindex/Properties按TestTheLint前缀在第三片；adapter/markdown/SQL/Web证据留各层。加入这些内容后重新量完整当前输入，不能借旧第三片25s规划包络宣称容得下，更不以名字躲移界；S1-0的旧容量样本不代S1最终双轮与指定两main。

S1-0先重建当前编译测试清单、验证四片互斥完备并完整两轮；S1最终新代码全部新增race测试各两轮、**完整新四片各两轮**，允许复用同精确输入的门禁样本，不用S1-0旧产品样本顶替。每轮保存raw GoJSON、run→terminal/package/wall，清单顶层非空与所有run终止、既有skip清单；每片无重复无遗漏。最终合并与record这**指定两main**的真实Test步骤各均值<450，逐job/run/attempt/head核对，所有实际作业必需步骤成功；若触线按邻居估计移界，否则新增份，原timeout不提高。S2同样复核真实入口及两main样本。

最新已有成本Test/job=635/796s、guards=1463/1486s、E2E=263/319s。新成本不得只看单条预算而漏作业总量；e2e不先分份。既有IndexItems30ms/grep600ms义务保留，新lint通过不抵消它们，M10收尾继续看稳定性。

### 7.7 串行门禁与证据保存

所有重任务同一`docs/v0.1/claude_to_codex/tools/heavy.sh` mkdir锁；本机统一`GOFLAGS=-p=1`，定向Go命令显式`-p=1`，再以make -j1分次执行。make -j1本身不限制Go包并行，不能只写它。本机前置不改CI原命令、作业并发或原timeout：

```sh
docs/v0.1/claude_to_codex/tools/heavy.sh env GOFLAGS=-p=1 make -j1 lint gen-check knip build
docs/v0.1/claude_to_codex/tools/heavy.sh env GOFLAGS=-p=1 make -j1 test-go-race
docs/v0.1/claude_to_codex/tools/heavy.sh env GOFLAGS=-p=1 make -j1 test-go-bootstrap-1
docs/v0.1/claude_to_codex/tools/heavy.sh env GOFLAGS=-p=1 make -j1 test-go-bootstrap-2
docs/v0.1/claude_to_codex/tools/heavy.sh env GOFLAGS=-p=1 make -j1 test-go-bootstrap-3
docs/v0.1/claude_to_codex/tools/heavy.sh env GOFLAGS=-p=1 make -j1 test-go-bootstrap-4
docs/v0.1/claude_to_codex/tools/heavy.sh env GOFLAGS=-p=1 make -j1 test-go-costs
docs/v0.1/claude_to_codex/tools/heavy.sh env GOFLAGS=-p=1 make -j1 test-go-guards
docs/v0.1/claude_to_codex/tools/heavy.sh env GOFLAGS=-p=1 make -j1 test-go-short test-web
docs/v0.1/claude_to_codex/tools/heavy.sh env GOFLAGS=-p=1 make -j1 e2e
```

不跑plain make test/test-go；显式所有组成覆盖等价完整门禁。成本/变异/新race双轮/三浏览器探查另列真实命令、最终输入SHA、退出码、原日志SHA、是否复用；不只写“all green”。工具结果/测试生成需要时单独记录真实gen命令与gen-check；失败和重试全保留。设计链接检查为root受控脚本和`python3 docs/v0.1/claude_to_codex/tools/check_links.py`，不是产品门禁。

## 8. 完成标准与逐项文档新文字

### 8.0 设计接受时的语义同步（不宣称实现）

本轮exact-old/new包读取精确f6bd原文，原A/B/C复核后无未关闭Important/Minor，主持者接受设计语义同步。设计文档按受控脚本先在干净归档副本核对再落库；未来完成句仍在8.1/8.2等待真实实施证据。

| 目标 | 本次设计要写的具体合同 |
|---|---|
| 总体§5.4/5.6 | 首sources合法slot且全部匹配边唯一、current发出者；附件current非空正文父页、多父any/direct、stale/empty不吞附件；坏sources根/AGENTS照查；旧页资格与未改页近似分开 |
| 00§4.1/4.2、4.11规则表/算法/成本 | SourceArea严格后代、LintNodes紧凑NameKey/Linkable、Planned presence；sources迁移必做、普通tuple索引测量准入；wanted bounded union；sources/附件新口径；候选不读ByKeys；sources两次必要JSON长度/值读取及slot基数范围 |
| 00§4.11 REST/MCP、§4.18、§5接口表 | item_key/每项cursor/list mode-time-stale/StartCursor；内层K规范；notebook入口Gate HTTP404与tool-app顺序分层；summary组合422；SDK真实48KiB、说明≤3796 |
| 00§4.11运行、§4.18运行行、§8/wiki/bootstrap | 真实名额/期限；recover/Stop、active与slot清理、newApp上游alive仍全run出口Stop、close在pool前兜底；不把LintTimeout称cleanup保证 |
| 00§4.14、§7 P4、§8 linking/page扩展 | OldQualifiedFacts与unit起点BeforeIngest窄事实；删旧索引前每被改页一次，Body/唯一sources slot区别、旧target union；P4另行兑现未改页一条候选SQL资格/成本；保留混合改树近似 |
| 00§7 P3、§8 access/注册/组合根、§9测试 | S1-3内部action+rule+Actions同时绿，S1-4公开operation；两scope真实注册、Stop负控；原72行加S63–S69共79行变异与MI/MX/PP/AC/EW真正catcher，EW每轮build |
| 00头/§12/§13、P3头 | P2done精确CI已接受；P3设计审查状态按原席真实复核填写，S1/S2未实施，P4/P5/M11未开始；原历史记录不改 |
| P1§9、P2§9 | 只补链接到P3的当前合同、承接待实现；完成后再追加8.1所列具体证据，不追改旧Phase事实 |
| 总体§13.4#8 | 只转录目前P2决定44后的Makefile；候选MCP B/C在P3实施测量验收前不写作现行 |

### 8.1 S1必须满足

两个REST/两个工具两scope真实可达；八规则、metadata/数组/Unicode/JSON/identity一致；真实snapshot/名额/flight/授权交错；所有声明码真实答出；sources迁移、可能的新索引均有计划/实际工作/写成本与扫描义务；48KiB/说明余量/本机容量双轮及两main指定配对；LW8 API/MCP。全部原/审查追加变异都有干净基线、失败断言、恢复证据。S1三审无Important/Minor，审查记录写清接受/后续。

由文档起草者出受控exact-old/new脚本，root审核应用；每段旧文恰一次，先副本check，不替换整篇历史。以下是各目标应新增的**新文字**（结果中的实际数字须以真证据填入；无证据不能投影为完成）：

| 目标 | S1完成后应落的新文字 |
|---|---|
| 00 §4.11 | “八规则已按P3合同实现：bounded source union、compact presence、唯一sources slot、附件跟随、各自豁免、identity/StartCursor及独立快照分页。” 写实际Sources/Nodes/各查询/最后asset-Paths证据；设计文字不得代替实现验收 |
| 00 §5/§7/§12/§13、P3头/10.1 | “P3/S1服务端、来源整区、八规则、REST/MCP两scope及LW8 API/MCP已完成；S2健康页待完成。” 带实际审查/合并链接；未知合并号先写见00§12，之后record填 |
| 总体5.4/5.6/13.1#4 | “lint.read读者以上；getLint为404→503，rule页为cursor400→notebook404→rule404→mode409→limit422→503，取得名额后在snapshot首读再次核模式；每调用者独立授权，完成后交付前再核；notebook MCP保留外层Gate HTTP404，account与gate后才进tool-app次序。” |
| 总体13.1#10/#11 | “wiki经模块根接线HTTP及两个scope工具；pool-only page/linking读口使用同snapshot ctx。每实际lint运行只记id、规则枚举、数量、耗时与等名额时长，不记目标、路径、属性值或游标。” |
| 总体13.1#23 | “lint从请求半池取max(1,floor(floor(max_conns/2)/2))个运行名额；LintTimeout=min(10s,request_timeout/2)含等待和计算，singleflight在名额外，各caller有独立等待期限/授权；事务清理仍占slot，结束时间不假称在计算期限内。” |
| 总体13.1#27/#31 | “lint游标绑定version/rule/notebook/隐式或显式scope，≤2048B；item_key来自完整身份。摘要每规则最多5项，limit1–100默认50，count10000下界，目标256B截断；Fit0非空规则发StartCursor，MCP真实SDK48KiB。” |
| 总体13.4#3/#4 | “LW8 API与notebook/account MCP是只读故事，无DB断言；lint快照改名由最后path旧值杀去snapshot，降级只证明mode/规则同次读；排队后降级另核409；独立caller与真实注册有last-hop控制。” |
| 总体13.4#8/#9/#12/#13 | “P3按实测增加各lint成本recipe及迁移/写成本；keyset真实notebook+tuple范围、无/有统计与实际generic、隐藏本A→B→A、正文TOAST负正控制；jsTrim/JSON slot两处规则用差分守一致。” 写真实预算/余量，不能写本稿2s/250MiB为已达 |
| README配置说明 | “健康检查按调用重新计算；上式名额/期限由现有池和request_timeout派生，没有新配置键。繁忙返回server_busy/Retry-After；共享计算可能在caller离开后继续到自身期限，进程Stop取消并等待清理，清理可能晚于计算预算。” 迁移段写实际索引SHARE挡写与测量边界 |
| P1§9 | 追加“lint.read/Actions/access规则与两REST矩阵已接；MCP两scope LLM变体与新中英文模板8规则干净有实证，见P3/S1及其审查/CI。” 保留P1原先交办和各阶段记录 |
| P2§9 | 追加“SourceArea.Nodes严格完整整区已承接；空页、asset真实bytes、created_at、显式standard拟议root/>10k、非hash枚举已验；lint/健康两工具与预算说明分别见P3/S1/S2。” 健康完成句等S2才写 |
| M6 handoff | 六行/注意1–4逐条记实际实现：wanted完整key而非旧target_key；snapshot/extractor跳过不reparse；模块边界、来源metadata、八规则与真实证据。全部完成后改done，旧表说明当年事实保留并链接现行修订 |
| M7 handoff | “第5项lint分page/asset及附件跟随已完成，证据P3/S1；第1/2/6项仍P4，整份open。” |
| M9 handoff | “第8项两工具/矩阵/码/Fit、§6内置说明余量完成；两入口实际SDK字节…/…与至少300余量。虚拟P4、prompt/系统说明/人工清单P5，整份open。” 数字以实际测试填 |
| M11/handoffs/M10-mode.md | “内部LintScope显式Contract+AreaRootID与implicit隔离，standard拟议root仍能读且不依赖hash；调用必须不在已有事务内，名额与期限仍适用；item_key/cursor/规则口径按P3。” 只移交，不开始M11；补真实说明余量 |
| M12性能handoff | 新lint八规则/summary、长属性/目标/来源数组、singleflight并发/池与取消清理、索引迁移和5000links保存的本机真数；真实服务器负载/停机仍待M12 |

### 8.1a 决定62的实施文档与兼容完成标准（S1-0a与S1收尾）

S1-0a已同步当前writer/版本/API的说明，前置实测与未测项见§10.1；S1收尾仍须完成全部reindex/成本/变异/审查及CI。下表保留逐项交付要求，不以本前置替代完整S1验收：

| 实施时必须更新的目标 | 必需新文字/证据 |
|---|---|
| M6/00§4.3、M6/P3§3.2、M6/P4§3.3、M6/P5§4 | 原v1空键限制标明历史适用版本，新增v2正文NULL/空路径空串；rewrite仍重parse的事实不改；PageProperties的空key可返回。P3A L6原处置/全部旧review保留并链接当前修订，不能抹掉原“不改”历史 |
| README链接索引与部署升级段 | 新版writer/Extractor 2已落地后，明确stop旧单实例、原备份/迁移/权限流程、新binary reindex后再serve；启动不会自动重建；linking/rekey每本事务持锁阻写、本阶段失败/撞键不留部分改动；后续search入队独立分批提交，linking失败仍可入队、queue失败不回滚已提交v2/此前批次，锁不覆盖全部queue，整条命令在单本内亦可能部分成功、全实例非原子；旧视图/改写/backlinks行为、缺失/旧tokenizer队列及后续job副作用、回退后再重建；只写实测数字并说明M12待验 |
| API linking.yaml与相应gen描述（如增说明），properties读者与Web测试 | key可以为空字符串；无新字段；真实REST page/unresolved/asset与Webexact-path证明，旧v1直至重建仍原行为 |
| M6 handoff处理进展、00§4.11/§7/§8/§9/§10/§12/§13、总体§11/§13.3#6/§13.4/§15 | 按真实实现写v2与兼容证据，六行/注意项全完成后才done；全Phase79行各单改证据按Step交付，原72行和历史不删，S1-0a及后续D/F分别记账 |
| M11模式handoff | literal-v1会令stale_pages>0，按已有M11闸门拒批准或先reindex；仅移交，不启动M11 |
| M12性能handoff | 必需5000-links保存与v1→v2真实代表重建结果；linking事务和独立search批的锁/WAL/空间/提交及失败结果分账，含队列后批失败保留前批；真实服务器停机/负载仍open，不用历史330秒补缺数 |
| P4交接 | KnownAbsent/Ineligible/Known/Unknown、旧/未改coverage与保守缺标、新增诊断/成本及负控归P4；真实来源与未知老事实不能退回“看空union”；不预写Capture已实现 |

S1完整验收须兼容/存储/API与真实D/F一起成立；正文TOAST禁止仍用于lint请求，合法管理员reindex的正文读取单列，不拿一边的trace冒充另一边。

### 8.2 S2及P3完成投影

| 目标 | 新文字 |
|---|---|
| 00 §4.12、P3§4、§10.2 | “每次进入（含Back/Forward）新算；generation隔离旧more，同cursor也不串。busy手动、保留数据仍报错、mode差异禁More不自动算。路径取当前树，missing保项；asset去当前父页。焦点局部策略，不改搜索默认。” |
| 总体13.2#7/#15 | “健康summary/各卡读取在SWR，按login/notebook/entry/generation隔离，离开abort并清缓存；成功刷新重置所有分页，失败保留旧数据并显示错误；无按本结果store。” |
| 总体13.2#12/#17/#19/#26 | “output总在、重复播报有效；每规则列表有可读名，more成功首个新项在读者未移走时获焦点，按完整item_key去重；缺节点保留中性说明，asset按当前父页定位。” |
| 总体13.2#16/#22/#23 | “health在NotebookLayout下，useNotebook/useDocumentTitle；无focus/reconnect/pages/mode事件lint重算，mode只更新本列表，发现差异由人重新检查。” |
| 总体13.4#3、LW8/00故事表 | “LW8 page完成两模式reader/更多/刷新/BackForward/附件与两种mode交错；API/MCP仍沿S1。只读故事无DB断言。” |
| P4交接 | “health#undigested_sources是固定定位锚点；零规则也可定位。SourceArea完整严格后代、sources首键/消化/JS trim与P3共享；旧事实Capture/BeforeIngest按§3.8.1在P4实现，未改页批量SQL须独立兑现同一资格；目录不在lint名额外偷跑pending。” |
| 00头/§12/§13、P3头/§10.3 | S1/S2全部验收、两次main容量达标后写“P3已完成，P4下一Phase；M10进行中”；独立`docs: M10/P3 done (M10/P3)`及精确main CI成功才释放P4 |

最终核对单不得缺任何项：全部合同字段有真实全链；权限矩阵每新操作/工具有行且wrong-notebook负控；新码中英/真实答出；source/stale/root/AGENTS/bytes/folder各适用范围；count/preview/page一致；distinct union与代表；identity/cursor/SDK进度；每端口ctx与真实snapshot；flight生命周期/清理；全部原与审查变异；CI所有必需job与步骤、失败/重试原件；成本/迁移/容量实数；M6正确done，M7/M9正确open；M11未开始。未完成任何一项不能以“实现已完成，测试文档以后补”宣布P3done。

## 9. 交给后面的Phase

P4只复用P3实际定下的SourceArea/metadata、SourceSlots纯资格、jsTrim缺定义、稳定identity；OldQualifiedFacts与unit起点BeforeIngest、删前Capture、未改页批量SQL按§3.8.1/00§4.14在P4实现并测量；P3没有替它实现ingest或可直接套用的direct SQL。按自己的index/log/export/ingest合同增加功能，来源数量不等同pending数量。P3只提供虚拟target只读谓词，P4仍负责路径lookup/render/web样例与virtual工具行为。health#undigested_sources锚点可直接用，P4目录仍自己的SWR数据，不暗调lint计数。

决定62新增交接：P4只信实际落地的v2来源表示；已存在页的stale/missing旧索引和未改页coverage不足均为Unknown，不是KnownAbsent或空union，按§3.8.2保守不加无法证明的标记。管理员重建恢复可靠判断；P4高效判定/诊断/测试与成本仍未实施。

P5仍负责prompt注册/分类/大小、系统说明段、人工清单和根README完整用法；S1缩短的是现有内置段，300字节是给M11预留而非P5随意占掉。M11显式scope已交接，用户未说开始M11。M12接真实测量与部署/负载复测；既有IndexItems/grep稳定性不因本Phase通过而关闭。

## 10. 结果（实施后填写）

### 10.1 S1

S1整体尚未完成：后续规则/运行时/接口、三审、完整变异与成本、最终门禁和merge/record两main仍待。本节按前置分别记录，不以局部验证代替Step完成。

#### S1-0a 来源表示前置（2026-10-30）

入口是已接受的S1-0容量提交 `89d3645420c7b2399e06765bd410132c6958047a`。此前设计提交b02的CI40942551297 attempt1及S1入口已接受；9e38的原CI40897835721失败、搜索校准和维护CI各保留原历史。当前实现把已有 `InFrontmatter()` 逐链接传成等长bool，生成的InsertLinks用CASE保留正文NULL与合法空属性非NULL空串，并同时提升Extractor到2。无DDL、启动修复或旧行版本UPDATE；PageProperties三组IS NOT NULL投影和API字段不变，空key沿现有page/unresolved/asset与Web完整路径消费。M6/README及API描述随本前置同步，原M6/P3A L6不改。

**定向验证。** 统一输入清单SHA256为 `dd6e065f549262435befcbf85808de2a6b42a40dafe8f60dbd2bb1d34acd4bea`，3,210个跟踪及新文件与当时源码逐字节一致。下列每项两轮共12次，命令均沿§7.1的selector、heavy锁与原timeout，exit0且选中终态非空，每次输入前后相同；时间为整个命令进程wall，不是单个测试时间。Go定向含race，代表成本不含race，OW为Vitest。

| 项 | 实际覆盖 | 第一轮 / 第二轮进程秒 |
|---|---|---|
| OF | TestLinkOriginFacts：真实parser的空/命名属性与正文事实 | 1.501 / 1.376 |
| OS | TestLintOriginRows、TestLintOriginProperties：AddPage/ReplacePage、literal2、NULL/空串和三个属性分支 | 2.977 / 2.977 |
| OB | TestTheLintOriginProperties/Reindex/Compatibility：实际REST与附件签名、literal-v1读者及真实重建 | 37.342 / 37.278 |
| OW | empty property key的page/unresolved/asset三个选中用例及命名/缺路径对照 | 1.794 / 1.736 |
| CLI | 两个既有Reindex错误/撞键/缺笔记本及搜索未入队控制 | 5.324 / 4.006 |
| 成本 | TestLintIndexesCostTheirScanAndWrites的四种保存与分阶段重建代表样本 | 27.919 / 24.992 |

OB逐项验证View fallback不修索引、rewrite及batch的Indexed/Reached跳过旧格式（含revision不匹配和current对照）、反链保留页/计数但无旧上下文、真实正文写入才升级触及页，tree/no-op不升级。真实 `bootstrap.Reindex` 的single/all阶段失败、撞键、linking成功而queue失败、后批失败保留前批、后续笔记本继续、tokenizer资格与linking锁在queue前结束均有独立状态断言；两个后批故障样本保留第一批10,000项，失败批不提交，已提交v2与事件不回滚。REST三分支核空key/命名对照、正文不返回及签名附件可取；Web证据只证明消费，不代SQL/HTTP。

**失败和反向对照。** OF与OW在旧实现基线本已通过，如实保留；04 REST和修正fixture后的05 OS因真实空key遗漏/旧版本断言失败。03的fixture修正、12锁例中writer自身等queue行的失败及14换独立current-tokenizer writer后的恢复均保留，未提高timeout。16首轮成本的whole-command-current-tokenizer实际是v2→v2，不计作升级实测；改成每次全命令前恢复并独立核literal1/collapsed origin后，以20及第二轮重新测量。

S63/S64/S65/S68四行分成五种独立单改、七组目标命令，共21次baseline→真实断言失败→字节恢复后的成功，已由主持者独立核对接受：

| 单改 | 实际catcher与失败 | 恢复 |
|---|---|---|
| S63 writer只认Property非空 | OS current-format的AddPage/ReplacePage丢空属性来源 | 原字节恢复，原命令成功 |
| S64执行的generated CASE改回NULLIF | OS同一真实SQL的非NULL空串断言失败；bool绑定仍在 | 原字节恢复，原命令成功；变异时不gen |
| S65 Extractor回退1 | OS literal2及OB literal-v1读者两组命令分别失败 | 两组各自baseline/失败/恢复成功 |
| S68 SQL三数组一致排空串 | OS属性及OB REST两个独立catcher均漏空key，数组仍等长 | 两组各自baseline/失败/恢复成功 |
| S68 Web taking排空key | 三个选中用例各有真实AssertionError | 原字节恢复，三例成功 |

这些变异的统一baseline输入与最终双轮输入只差成本测试的全命令旧格式准备修正；产品、catcher与期望未改。不是79行全部完成：S66/S67/S69仍待S1-2，S1收尾69行服务端复跑及后续S2十行仍待。

**代表成本。** Linux/arm64、Go1.27.1、4 CPU，request 4s/commit 2s。每次保存真实5,000条链接；body、named、mixed-named、mixed-empty的wall分别为58.323/59.786、78.957/83.606、68.690/68.278、68.348/68.112 ms。mixed-empty确有一条非NULL空路径，混合两例各有2,500条正文；写后核总数、literal2和NULL/空串，不能把它们当成v1/v2保存差额测量。

重建样本仅一本10,004页：一个56字节正文含3条链接，其余10,003页正文为空，不代表真实万页内容。诊断阶段先独立量linking提交、queue故障/恢复和后续job；两次整命令另各从literal1及塌缩的空属性来源开始，结束核全部10,004页为2、正文/版本历史不变和确切queue IDs，不能以诊断阶段相加冒充整命令。

| 阶段 | 两轮wall（秒） | 两轮WAL（字节）/结果 |
|---|---|---|
| 诊断linking提交 | 6.713 / 6.773 | 各3,158,640；仅linking事务 |
| 诊断queue第二批失败 | 0.041933 / 0.042875 | 各3,998,808；批次提交10,000/0 |
| 诊断queue恢复 | 0.049560 / 0.054916 | 各5,205,192；批次10,000/3 |
| 后续search job | 0.562857 / 0.573838 | 各29,275,568；队列清空、全部搜索行当前 |
| 整命令v1→v2，current tokenizer | 6.726 / 6.703 | 各3,010,376；不新增queue |
| 整命令v1→v2，missing/old tokenizer | 6.735 / 6.771 | 各7,007,368；确切10,003项入队 |

原始20条阶段JSON还记录Go进程CPU、分配量、采样heap峰值与前后linking/queue/property-index空间；CPU/内存包含测试server但不含PostgreSQL，WAL是集群级读数。queue独立批次的SQLwall另有4组原件；linking取锁查询0.658/0.640 ms含SQL往返，不是纯争用等待。上述数据不覆盖linking rollback的CPU/WAL、纯锁等待、完整sources索引迁移/100k lint成本或生产停机。OB/成本两轮进程均值×2.4为89.544/63.494秒，分别低于局部450/300秒排程界；这不是完整新分片双轮或CI容量验收。S1-0旧产品样本不能代替最终新输入，M12仍须真实环境复核。

**原件与余项。** 树外证据位于 `p3-s1/s1-0a/author/`：`targeted-and-mutation-summary.json`（SHA256 `23509f2658f1893fcf3ff184c83254fac7b51ac8c7dbc4767ec6b1001d351793`）逐项列input/result/output哈希；`targeted-{OF,OS,OB,OW,CLI}-round{1,2}`、`20-cost-corrected-first`及`targeted-cost-round2`保存原始命令/日志/源码，`mutation-*`保留全部单改及恢复，早期03/04/05/12/16不删除。主持者接受回执为同目录上级的 `root-targeted-acceptance.json`（SHA256 `14b2733a205c2f92baf7ec3c6693a725dbf5d663728623b9c4c76531d4488e2c`）及 `root-mutation-acceptance.json`（SHA256 `9e3d993bb2e0e205043584541b99623e2e55e2f3ee7de6296198cc8bbed52073`）。

本段截至定向验收时点，只记来源表示前置；每提交七项及Web门禁随后在文档投影候选上执行，原生gen-check按仓库约定在提交后执行，实际结果由提交/外部验收记录承接，后续S1文档汇总，不在此预填成功或本提交自身SHA/CI。后续S1规则/运行时/REST-MCP、S66/67/69、全部成本/容量、三审与Step/合并/record CI，S2网页和P4 Unknown/未改页coverage仍待；M6/M11/M12移交保持open。

### 10.2 S2

待S1正式释放后JIT核main；记录UI九项的真实行为、测试、焦点探查、三审与最终CI。

### 10.3 P3完成

待§8全项和独立P3done精确main CI。此处不用自身尚未知SHA/run写自引用；后续既定P4设计补入口证据。

## 附录：已核代码与只读证据

- page/source_area.go:12/25只有Forget；page/app/source_area.go的SourceHeads/ProbeSources是匹配/restore用途；source_hash迁移00050与unit_sources的0page规则。
- linking/domain/target.go:47 ParseTarget、:126 ReadsAsAsset；shared/title_key.go；page/link_targets.go及queries/links.sql；indexed_pages/page_links/page_properties迁移00020/21/23、部分索引00047。
- linking/adapter/markdown/facts.go:49 MaxLinks、:76完整属性JSON、:184 jsTrim；queries/index.sql首ASCII键/prefix；现有Backlinks读正文，因此不能接成lint。
- platform/postgres/planned.go描述真实DescribeExec；tx.go:137 snapshot、:158以后清理；archtest/rawsql_test.go禁止模块自行Query；sqlc生成:many defer Rows.Close及Rows.Err。
- bootstrap/wire.go:170/216–225、mcp.go:43–46尚未wiki HTTP/tools；wiki/module.go:57 Actions为nil；permission_matrix_coverage_test.go:130尚不认rule。
- mcpserver/answer.go:17/127 SDK字节/Fit；page/adapter/mcp/instructions.go实际共用段；合法request_timeout下界由config/validate.go:119–127推出。
- W/routes/NotebookLayout/NotebookPlaces、search-page/load-more、SessionRoot/RootStore/authMiddleware、P2 nodeTarget/openOf/useNodeNavigate、events handlers/refreshedOnConnect；健康页无现成实现。
- 三份只读调查：`p3-data-readonly-recon.md`、`p3-ui-readonly-recon.md`、`p3-root-runtime-readonly-recon.json`；authority-sync与capacity同目录。原调查的旧头状态不改写，本稿入口按已释放f6bd；完整机器路径在树外证据清单，仓库文档以文件名/SHA与CI链接标识。
- P2 §7.5/8.3/9/10.4、M6/M7/M9 handoffs按本Phase分工承接；Phase骨架按P1，流程按02-workflow/05-lessons。

本文设计使用已宣布的brainstorming/writing-plans方法，本轮按receiving-code-review先核真实链再修订；用户自主授权覆盖普通确认菜单，仓库Phase骨架/位置优先。设计者只写外部草稿，root负责受控应用、审查调度与提交推送。
