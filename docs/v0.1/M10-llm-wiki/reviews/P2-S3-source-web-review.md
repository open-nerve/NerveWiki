# P2/S3 来源网页：实现、审查与集成记录

| 项 | 内容 |
|---|---|
| 对象 | 分支 `m10-p2-s3`；基线 `8904a8769646c3b8919ff6a4c503eeae3ac70727`；三份独立初审头 `1da533667955cb618a67dfe22991680f099bec19`；修复提交 `8c17b5cceb65b2a0f70976bbe4752daeaabaee05`；最终代码 `ae4501aa166771db153dae5dfa710ef16ee26ac4`，tree `8e31387857571a2db6baf48c092382b66adc77d4`。 |
| 权威与范围 | [P2 设计](../02-P2-source-area.md) §4–§8.3、[M10 总设计](../00-M10-design.md) §4.7/4.16、[总体设计](../../v0.1-design.md)、[工作流](../../claude_to_codex/02-workflow.md) §3。只完成来源网页、真实编辑失去、来源添加与重复反馈；P3 lint、P4 虚拟文件/ingest、P5 prompt、M11 升级仍不在本 Step。 |
| 日期、状态 | 2026-10-29；最终代码、原审复核、本机门禁与精确代码分支 CI 按下表验收。本记录是合并前的分支文档；自身文档 CI、合并及记录两个 main CI 尚待真实发生，P2/M10 仍进行中。 |
| 独立审查 | A/B/C 均从新上下文在各自 detached worktree 对 BASE→1da 读规范、diff、代码并自行复现；先固定独立初稿，后审计根提供的命令/日志。初审合计 0 Important / 3 Minor / 0 Nit；C 另有两个流程/证据 Q，见下文。 |
| 修复复核 | A-M1、B-M1、C-M1 的修复、原席实际核对和边界分别列明；不是用作者自测代替原审。 |
| 文档与合并 | 本记录与投影由带标签的 exact-old-once Python 脚本生成；提交前须在干净 archive 演练、由主持者核对并检查链接。文档提交及其 CI 后续核；合并见 [00 号文档第 12 节](../00-M10-design.md#12-phase-进度表)。 |

## 实施提交与实际差异

| 提交 | 实际范围与本地证据 |
|---|---|
| `c87726408d13a9d5708c434871fbb3268f37d6b4` | S31，全节点严格来源判断、来源页/附件角色能力、移入确认与队列末跳。26 文件；每提交门禁和 #30/#31 七项反向对照按当时输入接受。 |
| `424601ec392e1025f6b3baaf8b8567b36b18c66b` | S32，source loss 真正结束服务端 session、late-open/卸载幂等、真实 editor registry readonly、停止 autosave、保留 unsaved。8 文件；173 Web 文件/3664 测试；四项反向对照。真实服务释放当时留 S34，未提前声称完成。 |
| `98bc8f79e671e4146d305a112930e5e82667a0ed` | S33，显式标题 paste、同一次 create 的正文、持续 mixed upload、严格 UTF-8/原字节界、signal 与分页结果分型。23 文件；176 Web 文件/3771 测试；八项设计变体加三项追加。 |
| `1da533667955cb618a67dfe22991680f099bec19` | S34，统一 duplicate 可读路径、全部消费者接线、LW3–7/LW12 真实页面故事、决定48模板单句。28 文件；当时 short 8208=6915 pass+1293 skip、Web177文件3801、Wiki46通过。S34-03仅修 LW5 console 注册计数，Go/Web按未变输入桥接；不是全套重跑。 |
| `8c17b5cceb65b2a0f70976bbe4752daeaabaee05` | 审查修复：来源根祖先移动能力按目标父判断；失败 Markdown 新重试按当前角色与真实队列末跳重验；已有区外 duplicate 目标进入来源后也单次补读；撤销三个新增 slow，保留旧 API slow。最终实证和原席复核如下。 |

S31–S34 的本地可提交接受不等于整个 S3 已完成。`PageTreeStore` 的全节点 `nodeById`/`nodeAncestors`/`isSource` 保留既有页面专用树；来源根自身和普通区不算来源，包含附件，mode/root/祖先变动即时重算。`canMoveNode` 接收目的父：writer 携带来源根的祖先不能换父，同父排序与改名保持允许，admin 保持可换父；菜单选项、DnD 命中与实际排队发送前共享该能力，不以服务端403作为预知的正常UI流程。

`PageService.createPage` 的可选 content/signal 与 `PageTreeStore.createWithContent` 共用树队列。每本 `AssetStore` 管理来源 mixed uploads；结果为 `{kind:'page',page}` 或 `{kind:'asset',asset}`，编辑器 `uploaded(upload): Promise<Asset>` 保持附件专用。文件只规范化建议标题，不改正文；fatal UTF-8、ignoreBOM、原始5 MiB界和各类换行由手写字节 oracle 守住。关闭来源文件 modal 继续；generation/logout 中止排队和已发请求；页面成功只走树重读，不进入附件分页。父页移动后的既有文件请求仍由服务端按当时来源身份最终裁决。失败行的下一次用户重试必须符合当前角色，开始和队列末跳均再检查，不取消已经发送的合法流程。

duplicate renderer 分清 `duplicate_of` 与被拒 `node_id`，page 完整路径末尾追加 `.md`，asset 保留文件名，经 `openOf` 到当前父页定位；缺 ID 不造回滚链接。缓存中不存在或尚不能解析为当前来源的目标均最多补读一次，失败/仍缺不循环；点击前复核当前 mode、source 身份和父位置。树 Alert 在实际导航前被消费，释放原触发元素的焦点；共享 `useLocated`/读者活动保护保持不变。changeset-page 的 carried-refused renderer 已接线，但自然 source409 留在 RevertDialog，未伪造 route state 宣称该分支实际走过。

source loss 复用既有 session 幂等结束：捕获原 id 后 end，迟到 open 也 end，readonly/autosave 不依赖结束网络成功；断网沿原租约兜底，不承诺断网即时释放。Web 组件链与真实 LW4 服务 session/lock 断言分开；后者不是用 fake heartbeat 注入冒充真实 heartbeat 一定答 source_immutable。

## 初审发现、处置与原席复核

| 编号 | 级别、初审位置与实际情形 | 处置与证明边界 |
|---|---|---|
| A-M1 | Minor；`app/source-permissions.ts:47`、`source-move-confirmation.tsx:129` 等。writer 将仍携来源根的普通祖先换父，Move/DnD 两入口都发出已知无权完成的 MOVE；实际拒绝后树未变。 | 根接受。新增目标父感知的能力并用于菜单/DnD/queue，保留祖先改名/同父排序和 admin 换父。初审是 UI→真实 store→fake HTTP 复现加真实 Go 契约静态链，非独立数据库越权证据。 |
| B-M1 | Minor；`upload-rows.tsx:208/230`、`asset.store.ts:295`。持久失败 MD 行在当前 NotebookStore 已变 reader 后仍可点 Add source，并发新 POST。 | 根接受。当前角色驱动 controls/submit，caller-provided beforeSend 在 retry 开始和排队真正发送时再判；同一 File/Upload/draft 保留，恢复 writer 可再试。真实服务403且状态不变，属于UI能力缺口，不是越权。 |
| C-M1 | Minor；`duplicate-text.tsx:145–150`。缓存里已有但区外的 Known 被另一管理员真实移入来源；流延迟，create409后的普通重读失败；helper因UUID存在而不补读，隐藏本可访问来源链接。 | 根接受。以“能否解析为当前有效来源目标”决定单次补读，保留 mode/notebook边界、in-flight去重和来源复核。组件实际1读/应2读及真HTTP/数据库浏览器缺链均复现；首例 finally 放流后的快照后来有链不否定释放前失败。 |
| C-Q1 | 三个新增 `test.slow()` 缺必要性；LW4 两新例、LW12 扩展例。 | 撤三处新增 slow，保留 BASE 已有 LW12 API slow。C baseline/default/restored 每轮17通过、0skip/retry/flaky；default明确30s，观察5085/1759/2358ms。属测试预算加强，不计 production mutation kill。最终 CI 另核。 |
| C-Q2 | 1da #37 三阶段 `--output` 随归档目录变化。 | 精确表述为同 selector/worker/timeout、输出目录不同，不称逐字同argv；修后固定执行目录、阶段结束再归档的实际情况按最终控制原件记录，不重写旧历史。 |

原 A：在产品修复8c17b5cc原席关闭A-M1，新增0I/M/N；七文件173通过，ancestor能力4→3失败/1通过→4、queued beforeSend3→3失败→3、真实adapter注册/hitbox/ring提示3→1失败/2通过→3，均按原件同argv。writer同父排序/改名和admin换父保留。新提示测试随后由根改为完整公共payload并采用，产品结论继续绑定8c；最终测试提交按下述输入桥另核，未冒称A重跑全Step/Go/E2E。

原 B：在产品修复8c17b5cc原席关闭B-M1，当前0I/M/N；六文件174通过，live-role与queue-role两控制各2个实际选择例0→1→0，另19项为selector未执行。独立invalid-title旧复现和真实共享树队列重现均无额外POST，恢复editor沿同一File/Upload/draft写成。fake初始化时捕获写角色，故只证明客户端不发POST，真实403/no-write仍引用初审独立服务观察；本轮未重跑浏览器/大门禁。

原 C：原C在8c关闭C-M1及预算Q1，当前0I/M/N。三完整组件文件+原独立复现94→3失败91通过→94；同一条件负控每phase实际build的真实LW3 created/moved-in+原独立三例3→2失败1通过→3，构建均成功，真实产物SHA与同argv/固定输出均核。恢复后18个相关page故事全部默认30秒通过，0skip/retry/flaky；保留旧LW12 API slow。其证据不充根组件组数或后续fullStep/CI，旧C-location存活及#37旧输出路径历史保持。

产品修复与最终测试头的来源：原席产品结论绑定 `8c17b5cceb65b2a0f70976bbe4752daeaabaee05`；最终 `ae4501aa166771db153dae5dfa710ef16ee26ac4` 只有测试差异 `web/apps/web/src/pages/notebook/source-drag-hints.test.tsx`。根核全部 3198 份既有Git entries/blob与文件字节未变；产品未改、测试增量另按根回执接受，原席未被记为重跑后续全Step。原件 `/home/parallels/.local/state/nervewiki/m10/p2-s3/s35-test-only-final-code-bridge.json`，SHA-256 `3c6d20185ec954adfb532ea6014f2f3eac16301710eff9090c3e1830ac797a03`。

## 独立验证与证据边界

A 完整 focused 七文件159通过；独立删除实际 move 队列 beforeSend，选择两条交错得到2通过→2失败→2通过。祖先移动两条临时回归因真实多发 MOVE 而失败，文件逐字恢复；A 未独立执行真服务 E2E/Go，不将静态契约链计成真库实测。

B 六文件172通过；独立 CR→LF 和 queued signal 两控制均同argv 0→1→0，断言实际正文/AbortError。真实 LW3 mixed uppercase Markdown 使用服务、PG、blob、手写 BOM/混合换行字节、modal关闭与路由保持。B-M1 客户端 reader 已刷新的证明为组件1→probe0→恢复1；真实 observer-v3只证明后端 reader/403/no-write。v1非法 workspace_access 导致422、v2未等到列表刷新且没试 retry 都保留为无效诊断，不冒充 B-M1 证据。最早 JSON reporter 覆盖故障保留，不纳入完整语义控制；后续带 verbose 和即时归档的 full 三阶段才被接受。

C focused 三文件87通过；自行构建真实产物并执行17个 page 故事。独立 current-source/current-parent/refused-target 三控制各0→1→0，分别捕获区外/降级误导航、附件旧父路由、误用node_id。**C-location 单项删除存活：17通过→17通过→17通过**；额外2条观察证明目标缺失时旧 Link 已被外层卸载，mounted守卫已阻止迟到导航。没有构造仍挂载 handler 跨路由的真实错误，不为凑kill伪造store/route，不把防御冗余自动记产品缺陷。

三审先独立形成报告，随后审计作者原件。C `workflow-integrity.json` 核四提交26/8/23/28文件与Git精确一致、1da 28组84测试+6build共90份日志SHA、全部0/1/0，并核33条历次gate run（含明确reuse）。这只接受 **1da 修复前历史**，不外推到后续修复头。

## 运行时控制：前期与最终输入分开

设计 #30–37 是8表行/24个变体，S33另加排队abort、transport signal、普通Untitled末跳三项；S35在1da上又加树Alert焦点负控，形成28组历史：26 Web变体+2完整LW12浏览器变体，84测试执行+6build。原件接受 `s35-root-replay-acceptance-01.json` SHA-256 `ab9a2e2cf046ab7244dcd6cf9a2c5d3295411cb9564945333030a9c594a04a37`。旧 #37 runner01路径碰撞只到baseline，不计kill；最终正式02与独立witness另有原件。

修复后最终控制按以下真实清单逐项记录，包含审查/修复追加；下表不由28旧组数推算：

W 的文件/选择器按 P2 §7.3 展开；本轮共同 heavy 编排下各 Vitest 命令另包 `timeout 10m`。PW 是 e2e 工作目录的 `timeout 15m pnpm exec playwright test FILE --workers=1`，每阶段先 `timeout 15m make -j1 build`。完整 argv（含固定输出路径）和实际失败断言在原始回执，不省略在证据中。

| 控制组 | 实际 catcher | 原版→改坏→恢复 exit | 结果 |
|---|---|---|---|
| `30-root` | `W(src/stores/source-area.test.ts, strict source membership follows all nodes and mode)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `30-page-only` | `W(src/stores/source-area.test.ts, strict source membership follows all nodes and mode)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `30-unobserved-mode` | `W(src/stores/source-area.test.ts, strict source membership follows all nodes and mode)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `31-mod-e` | `W(src/pages/page/source-actions.test.tsx, source controls stay read only at every entry)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `31-task` | `W(src/pages/page/source-actions.test.tsx, source controls stay read only at every entry)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `31-restore` | `W(src/pages/page/source-actions.test.tsx, source controls stay read only at every entry)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `31-edit-here` | `W(src/pages/page/source-actions.test.tsx, source controls stay read only at every entry)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `32-clear-id-before-end` | `W(src/stores/edit-session.test.ts, source loss ends the server session exactly once)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `32-late-open-not-ended` | `W(src/stores/edit-session.test.ts, source loss ends the server session exactly once)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `33-registry-without-readonly` | `W(src/stores/page-editing.test.ts, source loss locks the mounted editor and preserves unsaved text)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `33-autosave-keeps-running` | `W(src/stores/page-editing.test.ts, source loss locks the mounted editor and preserves unsaved text)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `34-bom-stripped` | `W(src/pages/notebook/source-uploads.test.tsx, Markdown uploads preserve the exact UTF-8 bytes)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `34-newlines-normalized` | `W(src/pages/notebook/source-uploads.test.tsx, Markdown uploads preserve the exact UTF-8 bytes)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `34-lenient-utf8` | `W(src/pages/notebook/source-uploads.test.tsx, Markdown uploads preserve the exact UTF-8 bytes)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `34-characters-not-bytes` | `W(src/pages/notebook/source-uploads.test.tsx, Markdown uploads preserve the exact UTF-8 bytes)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `35-md-title-numbered` | `W(src/pages/notebook/source-uploads.test.tsx, source uploads keep their queue and explicit title)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `35-modal-cancels-files` | `W(src/pages/notebook/source-uploads.test.tsx, source uploads keep their queue and explicit title)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `35-generation-keeps-uploads` | `W(src/pages/notebook/source-uploads.test.tsx, source uploads keep their queue and explicit title)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `35-page-enters-asset-pagination` | `W(src/pages/notebook/source-uploads.test.tsx, source uploads keep their queue and explicit title)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `S33-queued-create-ignores-abort` | `W(src/stores/page-tree.store.test.ts, source creation)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `S33-create-request-drops-signal` | `W(src/services/page.service.test.ts, PageService.createPage with source content)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `S33-queued-ordinary-auto-names-source` | `W(src/pages/notebook/source-add-dialog.test.tsx, queued ordinary creation)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `36-attachment-as-page` | `W(src/pages/notebook/duplicate-text.test.tsx, duplicate targets include attachments and stale trees)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `36-missing-target-leaks-refused-id` | `W(src/pages/notebook/duplicate-text.test.tsx, duplicate targets include attachments and stale trees)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `36-stale-tree-never-refreshed` | `W(src/pages/notebook/duplicate-text.test.tsx, duplicate targets include attachments and stale trees)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `S34-tree-alert-retains-focus` | `W(src/pages/notebook/source-moves.test.tsx, duplicate tree drop link)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `37-stale-notebook-mode` | `PW(stories/wiki/lw12-downgrade.spec.ts)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `37-standard-still-rejects-same-byte-page` | `PW(stories/wiki/lw12-downgrade.spec.ts)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `A-move-before-send` | `W(src/pages/notebook/source-moves.test.tsx, a confirmed move still queued behind another write rechecks\|a queued ordinary move stops)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `B-carriage-return-only` | `W(src/stores/asset.store.test.ts, Markdown keeps its UTF-8 bytes while only the suggested title is trimmed and composed)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `B-queue-signal` | `W(src/stores/page-tree.store.test.ts, source creation cancelled in the tree queue never calls its send observer or service)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `C-current-source` | `W(src/pages/notebook/duplicate-text.test.tsx, clicking a retained link after)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `C-current-parent` | `W(src/pages/notebook/duplicate-text.test.tsx, a retained attachment link uses its new parent)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `C-refused-target` | `W(src/pages/notebook/transfer-background.test.tsx, a duplicate report distinguishes its refused attachment)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `C-location-survivor` | `W(src/pages/notebook/duplicate-text.test.tsx, "")` | 0 → 0 → 0 | 冗余位置守卫存活 |
| `fix-A-ancestor-reparent` | `W(src/pages/notebook/source-moves.test.tsx, source-root ancestor\|subtree gains the source root)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `fix-B-current-role` | `W(src/pages/notebook/source-uploads.test.tsx, a failed Markdown upload cannot be retried after becoming reader)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `fix-B-retry-queued-role` | `W(src/pages/notebook/source-uploads.test.tsx, a Markdown retry queued before becoming reader)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `fix-C-existing-outside-target` | `W(src/pages/notebook/duplicate-text.test.tsx, an existing outside)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `A-drag-hint-binding` | `W(src/pages/notebook/source-drag-hints.test.tsx, source-root ancestor drag hints)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |

最终ae4501上共40控制组、120次测试和6次真实build，126份原日志；39组0→1→0，C-location为已核0→0→0，不算kill或独立产品bug。控制组不等于独特缺陷数；B队列取消与既有S33同语义但选择器来源分别保留。三阶段测试argv逐字一致；#37固定执行输出目录后逐阶段归档，不再沿用1da时输出目录各异的说法。所有source/input SHA与最终Git一致，恢复后工作树干净。125份producer日志校验值直接匹配raw；B-carriage-return-only的mutant断言输出含2个真实CR，producer read_text().encode()规范化换行后算hash，首轮raw核对因此拒绝。根保留原日志/原报告，另记录raw SHA与准确规范化SHA桥，不把它当成重新运行，也不改写历史。C原席在8c另作组件94→3红→94和实际每phase build的LW3三例3→2红→3；test-only桥核3198既有文件全不变，浏览器证据与根40组执行数分列。 原C席只读独立复算最终桥、40组、126日志及唯一规范化关系：final-evidence-audit.json SHA-256 461c10c32b0bd4a419c13ae7dcba084fe8ebea0cfebf8f90f1dec361e3f78139。该轮未重跑测试/build/container，不冒称新的产品验证。

## 门禁、测量与代码 CI

最终受测头为 `ae4501aa166771db153dae5dfa710ef16ee26ac4`，全范围实数：Web 178 文件/3812 通过；short 8208 run=6915 pass+1293 明确 skip；完整 E2E 108 文件/329 通过，0 skip、0 retry、0 flaky。

| 门禁 | 本次范围 | wrapper 秒 | 结果 |
|---|---|---:|---|
| `race` | 最终代码输入新跑 | 1100.969 | pass |
| `bootstrap-1-round1` | 最终代码输入新跑 | 314.974 | pass |
| `bootstrap-2-round1` | 最终代码输入新跑 | 317.963 | pass |
| `bootstrap-3-round1` | 最终代码输入新跑 | 255.320 | pass |
| `bootstrap-4-round1` | 最终代码输入新跑 | 262.871 | pass |
| `costs` | 最终代码输入新跑 | 444.634 | pass |
| `guards` | 最终代码输入新跑 | 877.449 | pass |
| `e2e-all` | 最终代码输入新跑 | 295.788 | pass |
| `bootstrap-1-round2` | 最终代码输入新跑 | 309.390 | pass |
| `bootstrap-2-round2` | 最终代码输入新跑 | 319.021 | pass |
| `bootstrap-3-round2` | 最终代码输入新跑 | 265.440 | pass |
| `bootstrap-4-round2` | 最终代码输入新跑 | 260.254 | pass |
| `lint-gen-knip-build` | 已验原件按逐文件未变输入复用 | 9.044 | pass |
| `web-all` | 已验原件按逐文件未变输入复用 | 113.498 | pass |
| `vet-server` | 已验原件按逐文件未变输入复用 | 1.409 | pass |
| `vet-tools` | 已验原件按逐文件未变输入复用 | 0.103 | pass |
| `short-server` | 已验原件按逐文件未变输入复用 | 159.727 | pass |

非组合根race实际7579 run = 7578 pass + 1 skip；package 90 pass/22 skip。既有成本测试的race skip由非race成本/guard原recipe补跑；不把skip当作pass。完整命令、每轮原始日志SHA及最终3199份输入恢复均在验收原件。

明确复用五项：ae4501测试增量的static/gen/knip/build与全Web；8c17产品修复的server/tools vet与short，经3198个原Git条目/文件字节不变桥加新增测试绑定最终3199输入。Web178文件3812测试、short8208=6915pass+1293skip取各自实际原件。完整Go race、全部cost/guard、双轮bootstrap与完整E2E均新跑；旧47例Wiki子集没有冒充本次完整329例E2E。

新增 race 测试/两轮：最终S3相对8904没有新增或修改Go测试/Go源码（实际差异为两份嵌入raw模板）；race编译清单实际470个组合根顶层，分片167/74/114/115，新增Go race测试为0。因此没有冒用S2旧新增测试清单；模板嵌入输入改变后的全Go门禁和四份双轮另按最终头执行。

四份完整 bootstrap race 的最终清单与双轮：

编译后的实际清单为470个顶层测试；四片167/74/114/115个，每轮并集完全相等、交集为空、每个顶层恰好执行一次。以下为Go package秒，另列进程wrapper秒，不能当CI的Test步骤秒。

| 片 | 顶层数 | 第一轮package | 第二轮package | package均值 | 两轮wrapper |
|---|---:|---:|---:|---:|---|
| 1 | 167 | 314.009 | 308.527 | 311.268 | 314.974/309.390 |
| 2 | 74 | 317.072 | 318.075 | 317.573 | 317.963/319.021 |
| 3 | 114 | 254.386 | 264.488 | 259.437 | 255.320/265.440 |
| 4 | 115 | 261.947 | 259.285 | 260.616 | 262.871/260.254 |

既有race跳过项（按测试名去重）：`TestTheAppsLinksAreWithinTheirBound`, `TestTheAppsMarkdownCostsAboutItsSize/to_a_page`, `TestTheAppsMarkdownCostsAboutItsSize/to_an_image`, `TestTheAppsMarkdownRendersCheckedHTML`。这些非race专属测试在完整guards原recipe实际执行；未提高测试预算、未改变分片边界。两次main的Test均值留待合并及记录CI真实验证。

成本及旧性能义务：最终输入完整执行Makefile原17个costs和15个guards recipe，命令清单逐行吻合且无空选择。包含IndexItems、grep、来源新增/移入/恢复/撤销/十万Forget和计划测试，所有原界通过；原始样本保存在costs.log/guards.log。这是最终输入的一轮完整门禁，不是重做600万迁移或新建重复成本基线。旧IndexItems30ms、grep600ms持续观察及M12真实服务器/数据/并发负载义务保持，代码CI样本已按下列原件独立核定。

精确代码CI 40843325190的20项成本指标和45个真实HTTP样本（5种操作×3统计状态×3次）已核原始日志、预算与状态。各操作HTTP最大秒数：create 0.359200391，move 1.047166084，restore 3.228866466，revert 3.970788992，forget 2.227704099。revert离原4秒请求期限仅29.211ms；保留为后续CI/M10收尾的窄余量观察，不由单次通过宣称稳定。guards的Test 1553s，整个job 1571s（原job上限1800s）；Test与job分别记录。没有抬高任何预算，也不以较快的样本替换这次最慢请求。

精确 `ae4501aa166771db153dae5dfa710ef16ee26ac4` 的 [CI 40843325190](https://github.com/open-nerve/NerveWiki/actions/runs/40843325190)（m10-p2-s3 push，attempt 1）实际 11 个作业及列明必需步骤均成功；四片 Test 步为 453.0/311.0/289.0/371.0 s。Test 步、job、package 与 wrapper 是不同量纲；完整本机与分支 CI 都不能代替合并/记录两次 main。IndexItems30ms/grep600ms 的持续稳定性和 M12 实机测量义务保持，不靠来源新成本通过抵消。

## 已发生的失败与纠正

- S31 初次 full Web 旧 mock 参数期望失败；S33 缺模块探索不算 RED，干净 source 上传 RED 到真实附件区拒 `.MD`/无POST。S33两次fixture失败分别为真实附件分页100却按50断、父移入来源后旧modal已卸载；修为101附件非空陷阱与当前持久队列，未绕过fake最终裁决。排队普通Untitled父变来源的两次实际POST是另一个已修产品缺陷。
- S33首轮全Web旧source按钮名断言失败；第二轮静态/build/fullWeb新跑，Go vet/short以明确未变输入桥接，不称重跑。S34首次wiki为42/46：LW2旧菜单、LW5树Alert焦点、LW6预览恢复后的已有trash-item409；分别保留实际原因与处置。焦点回归先RED再由消费Alert的最小产品修复转绿，未放宽focus或共享读者保护。
- S34第一轮静态三处ByRoleOptions `exact` 参数错误；第二轮修类型后全Web/short，第三轮仅修LW5三次console累计期望（每次追加1，旧误1+2+3），静态+wiki重跑，未变Go/Web复用。源模板输入已包含；旧#36/#37前期实测不自动变成模板修改后最终证据。
- 初审修复第一轮只因新回归两处 `filter()[0]` lint，等价改 `find` 后重跑。阶段结果是各自历史；最终门禁仍以上述精确修复头为准。已知 TimeoutNaNWarning 保留而未压制；真实fixture没有扩大console白名单、timeout或弱化持久断言来通过。

## 主持者的决定（负责人可以改判）

**48——来源模板使用实际两个入口名。** 沿实施序列47接续，与00总设计审查1–40分开。P2 §4.4 的“Paste Markdown / Upload files”“粘贴 Markdown / 上传文件”保持；对P1 §9旧单入口名称/不同时改模板的约定作窄偏离，仅改双语raw模板的一句操作名称。agent单次write路径、段落与代码结构、来源不可修改与管理权限不变；只影响今后新建笔记本，不改存量raw正文，不迁移用户内容，不提前实现P5系统说明。现行raw659/605 B、AGENTS2209/2107 B，总5580 B；P1/S1旧648/593/5557测量保留历史。go:embed改变Go输入，已纳入最终验证，不以旧S2样本接受新模板。

## 合并、Phase边界与移交

S2入口在本次既定文档自然补记：精确 `66d8236c12aaa8923616f6f42f2ee33b772884f9` 的 [CI 40800844731](https://github.com/open-nerve/NerveWiki/actions/runs/40800844731)（main push，attempt 1）实际 11 个作业及列明必需步骤均成功；四片 Test 步为 431.0/358.0/308.0/322.0 s；精确 `8904a8769646c3b8919ff6a4c503eeae3ac70727` 的 [CI 40804121398](https://github.com/open-nerve/NerveWiki/actions/runs/40804121398)（main push，attempt 1）实际 11 个作业及列明必需步骤均成功；四片 Test 步为 435.0/448.0/290.0/317.0 s。两次 main 四片 Test 均值为 433/403/299/319.5 s，均<450 s；主持者已接受并放行 S3。旧失败与阶段原件保留。

本分支文档的精确CI尚须在提交后核验。之后按workflow做no-ff合并、精确main CI、合并记录提交及其精确main CI；用两次main四片实际Test均值核<450s，再做P2done独立文档和精确CI。不预填上述未来SHA/run，不为自身SHA循环制造记录提交。合并见00第12节。

根逐项核P2 §8.3/P1 §9与移交：M8的S1/S2服务端和S3历史页面义务已有对应证据，分支阶段保持open，P2done最迟done；M7去重页面完成、lint/贡献者等P3/P4仍open；M9写契约与页面消费完成，工具/虚拟/prompt属P3–P5仍open；M5 mode消费已验且done保持；M6六行与注意项仍P3/open。M11只交接既有来源/hash/rewrite契约且升级未开始；M12保留既有6M与真实生产主机/负载/停机窗口的未验边界。无新打磨事项，因此M5-polish不改。最终门禁/CI与S3合并、记录、P2done是后续实际验收条件，不预填完成。

M8来源历史的实现义务已由S1–S3证据逐项覆盖；本分支仍等待S3集成，P2done最迟将M8改done。M7§3与M9§7记录页面最终验收，整份仍open；M5done、M6lintopen保持。M11只接已实现的同事务hash重建/rewrite契约，M12保留生产实机/负载/停机窗口；不设计升级。P3只下一Phase，未开始；P4/P5及M10收尾继续待办。

## 证据索引

以下为树外原件标识和SHA，需随项目交接保全；不是仓库外文件的永久相对链接。各原件包含精确argv/cwd/输入/退出/日志hash，正式JSON由主持者按最终受测头填写。

| 标识 | 原件 | SHA-256 |
|---|---|---|
| A初审 | `review-a/initial-report.txt` | `0149e4dfb052434056460536c57a2b067b37b0cee6e965db86a9f13a3a388fd4` |
| B初审 | `review-b/review-report.json` | `3f8631f2cc104621be41b925fefe53b8fb95e6614c0f5db7166d15f07b42638f` |
| C初审 | `review-c/initial-report.txt` | `189c527ca5f801eaa5cb1ee2ebe825ae8fb4c822bbef4d0acea413274989e871` |
| C历史原件审计 | `review-c/workflow-integrity.json` | `ad8560a2d54131350497f7272b3e6f582f472c2a7d2e1fc685ee1c2fa44562f7` |
| S34本地提交接受 | `s34-local-commit-acceptance.json` | `0457ef48e90a134e6a27cf1a6ab3c4898b29da1aba6ece462c7d8e7dad17beb3` |
| final_source_manifest | `/home/parallels/.local/state/nervewiki/m10/p2-s3/s35-doc-final-source-inputs-ae4501.json` | `fb248f9404892f3a9d0fe643685b815a7b2c8c178259e77f0b9f716317866713` |
| per_commit | `/home/parallels/.local/state/nervewiki/m10/p2-s3/s35-final-per-commit-index.json` | `4d4bee9bf1ff77775628e7e20eaa7e36c3fd11557b68df9f1149abf7b9d1687d` |
| a_recheck | `/home/parallels/.local/state/nervewiki/m10/p2-s3/review-a/fix-check-1/fix-check-1-report.txt` | `7a1c932d7ece576116cf4d51e303ae0bf3308b3cc863818eb4db05e6005df1e8` |
| b_recheck | `/home/parallels/.local/state/nervewiki/m10/p2-s3/review-b/fix-check-1/review-report.json` | `565376f4ee0390c272c0c745ef9ef8ef2fe07207d3ac67ff42388988da12c610` |
| c_recheck | `/home/parallels/.local/state/nervewiki/m10/p2-s3/review-c/recheck-8c17b5cc/report.txt` | `14772e86f0f59c6776c7f09bf009eee47792c66cb9961a39df6757f90ee0b877` |
| controls | `/home/parallels/.local/state/nervewiki/m10/p2-s3/s35-final-root-replay-acceptance.json` | `adb59c436474a13bfab09bf330b750a443459aabf475d5ec908d2f6603dda100` |
| gates | `/home/parallels/.local/state/nervewiki/m10/p2-s3/s35-final-step-gates-acceptance.json` | `0ca0a4dca0cacb7b0fe751301554b8bc2cbd98266f11cdf566b9f813eacff71f` |
| bootstrap | `/home/parallels/.local/state/nervewiki/m10/p2-s3/s35-final-step-gates-acceptance.json` | `0ca0a4dca0cacb7b0fe751301554b8bc2cbd98266f11cdf566b9f813eacff71f` |
| code_ci | `/home/parallels/.local/state/nervewiki/m10/p2-s3/ci-code-ae4501/root-ci-acceptance.json` | `ecf596568c1c6e432b21a3043894f0ee444646f925fc05e38e3721294c29c2bc` |
| s2_main_pair | `/home/parallels/.local/state/nervewiki/m10/p2-s3-current-preparation/root-s2-completion-and-s3-release-8904.json` | `795a5508863326df0f51ae8ccb40867642a16e60190149b26de2d188b53bb235` |
| handoffs | `/home/parallels/.local/state/nervewiki/m10/p2-s3/s35-branch-handoff-root-acceptance.json` | `a34ee31725fb6d6b307c779fd47872827db53dcc050b665279ae503f913bc415` |
