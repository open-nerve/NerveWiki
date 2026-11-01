# M10/P4 设计初审、两轮修订核对与主持者接受

状态：三位fresh初审及两轮原席核对已完成，主持者已依据实际接受回执关闭全部已核实finding并采用决定71／72。设计已接受，正式自身文档commit/CI尚未知待验，S1–S3未开始；没有P4产品/性能/实施放行结果。

固定main 6992fd042ff06c110a4e59e67fb89bfe5f43fbd9，tree bd74094f1e4b4fbaf04085431e2ba641f7e6c134；P3 ownCI与release已真实接受，仍保指定4e6e/df9 pair身份，6992 Test463/309/362/404/313不算新增测试容量保证。P4实施决定71/72已采用，负责人可以改判；00053为固定6992尾号52后的计划号，尚未实施。

## 精确输入与处置

| 输入 | SHA256 |
|---|---|
| initial-design-01/manifest.json（12988B） | f2bd2142e946dd1bfa4cea4c9da2804a366a4e04c92f787ed28d7ac942615f8a |
| review-a-01/review.md | bba03d7ad6b36d89c7702967774e13d42db7afb651914a3fbd1b8af42916117b |
| review-a-01/manifest.json | 503a63363eef4140ef5af28c0ac900f4b943f3e88d151e5d773abe837bd7e15e |
| review-b-01/review.md | 8bc2e8f59602c98c15fb5991c1b9b4b45b24c6cd3ddd8db02db3f67592a62ee5 |
| review-b-01/manifest.json | cdfad838978a668bfdfda300a11a52556337666080f51dde90535bfe818db7d0 |
| review-c-01/review.md | 1badf2a45f4ae433d873a6cd5f20be4679a12a3def02b55e521df71e348fa956 |
| review-c-01/manifest.json | 2d8489f513f9d1d9cc6ecd53fa00c0f3f34a764d78b5cc4310a9380dc90655e0 |
| root-dispositions-01.md | bf64048d941bd6d77added11ad448b5024b75985c74fccba444a59107a7bbccb |
| root-dispositions-02-old-f.md | 717fa81076f817fd6b844f339f9c7f9259779fdfa3a1d2e253da6d0806b1164e |
| root-initial-review-input-acceptance-01.json | 72f64eac6359f5d8868f415db1df5c361cb26fd0e114a1c07c80a5a1c347b72d |

原报告/evidence和PREP均保留原冻结位置，外部修订inputs逐项pin；不在仓库复制旧身份/逐行审计历史。root-dispositions-02明确授权必要紧凑old-F关系，这是存储承诺修订而非“初稿政策未变”；当时要求的原B/C设计核对已完成，原成本门禁仍待实施。

## 初审处置与修订1提交时状态（历史）

下表是修订1提交时的pending状态与该冻结输入行号；实际两轮结论与当前接受见后节，不把历史pending当当前结论。

| Finding | 严重度 | 修订1作者修订、当时位置 | 当时原席 |
|---|---|---|---|
| A-I1 | Important | 百分号目的地身份独立于显示；编码后字节界；A12唯一锚修正。 [04-P4-virtual-ingest.md L126–126](../04-P4-virtual-ingest.md)；[P4-virtual-ingest-ports.md L224–224](../specs/P4-virtual-ingest-ports.md)；[P4-virtual-ingest-test-completion-matrix.md L55–57](../specs/P4-virtual-ingest-test-completion-matrix.md) | pending |
| A-I2 | Important | 实际Scope映射同snapshot传入；index/source/log变化/ingest均按ID；folder-only、碰撞、missing/invalid规则与真ZIP控制。 [04-P4-virtual-ingest.md L138–138](../04-P4-virtual-ingest.md)；[P4-virtual-ingest-ports.md L187–187](../specs/P4-virtual-ingest-ports.md)；[P4-virtual-ingest-test-completion-matrix.md L57–60](../specs/P4-virtual-ingest-test-completion-matrix.md)；[P4-virtual-ingest-test-completion-matrix.md L134–134](../specs/P4-virtual-ingest-test-completion-matrix.md) | pending |
| A-M1 | Minor | 单列两ToolDeps构造，batch_cost_test.go保!race并S1-3接Virtual，独立于23/30 NodePaths。 [P4-virtual-ingest-ports.md L41–41](../specs/P4-virtual-ingest-ports.md)；[P4-virtual-ingest-files.json L1947–1947](../specs/P4-virtual-ingest-files.json) | pending |
| B-I1 | Important | 显式修改old-F保留政策；全四象限D/F与同批删除；完整Unknown闭包；SQL仅parent IDs/app投影；120k/4M与100ops/100k成本待验。 [04-P4-virtual-ingest.md L142–219](../04-P4-virtual-ingest.md)；[P4-virtual-ingest-ports.md L226–405](../specs/P4-virtual-ingest-ports.md)；[P4-virtual-ingest-queries.sql L123–250](../specs/P4-virtual-ingest-queries.sql)；[P4-virtual-ingest-test-completion-matrix.md L127–138](../specs/P4-virtual-ingest-test-completion-matrix.md) | pending |
| B-I2 | Important | 来源page Before/Now都须相应revision+Present/Extractor2；asset不需head；P4新增证明明示。 [04-P4-virtual-ingest.md L157–157](../04-P4-virtual-ingest.md)；[P4-virtual-ingest-ports.md L318–318](../specs/P4-virtual-ingest-ports.md)；[P4-virtual-ingest-test-completion-matrix.md L120–126](../specs/P4-virtual-ingest-test-completion-matrix.md) | pending |
| C-I1 | Important | W06改可达的目录no-op retry变异；旧healthRetry仅等价审计，不新增503。 [P4-virtual-ingest-test-completion-matrix.md L158–158](../specs/P4-virtual-ingest-test-completion-matrix.md)；[P4-virtual-ingest-test-completion-matrix.md L168–168](../specs/P4-virtual-ingest-test-completion-matrix.md) | pending |
| C-I2 | Important | 真实PG timeout/ctx双腿、原错身份与禁止后续marks/publish/commit尝试、真实rollback/连接复用，保WithoutCancel。 [P4-virtual-ingest-test-completion-matrix.md L170–170](../specs/P4-virtual-ingest-test-completion-matrix.md)；[04-P4-virtual-ingest.md L217–217](../04-P4-virtual-ingest.md) | pending |
| C-M1 | Minor | S1-3补PageRead判别元数据与21行/8文件version消费者审计，writtenOf不改合同。 [P4-virtual-ingest-ports.md L481–481](../specs/P4-virtual-ingest-ports.md)；[P4-virtual-ingest-files.json L1959–1959](../specs/P4-virtual-ingest-files.json) | pending |
| C-M2 | Minor | 共享Go/TS fixture完成文字归S3，与S3-2及文件表一致。 [04-P4-virtual-ingest.md L367–367](../04-P4-virtual-ingest.md) | pending |
| C-N1 | Nit | M12现1–32，未来S3在唯一末句前新增selector项，实际尾号时定，保持open。 [04-P4-virtual-ingest.md L369–369](../04-P4-virtual-ingest.md) | pending |

修订1提交时，6 Important、3 Minor、1 Nit均已形成修订内容；“形成修订”不是原席确认关闭。若原席发现回归，继续新修订目录与原席核对，不改此输入冻结物。最终Important/Minor须归零，才能由主持者采用设计、受控00与本记录为一个文档提交；必需门禁及精确main CI后才开始S1。S1/S2/S3仍4/3/3代码提交，各可单独绿。

## 修订1范围和未执行义务（历史计数）

136个独立控制ID：原114全保留，其中112行不变、C10/W06两行修正，新增22；多腿仍逐腿baseline→单fault→指定首败→restore，绝不计作已执行。五交错保持。命名SQL仍17，但NovelSources实质改成candidate+parent证据、Bodies旧source读取量扩大，不能说17条语义未变。source pins255、产品路径186（原176＋10），23 NodePaths路径/30 occurrences、六Writer/Trash、另两ToolDeps，15修复依赖均保留。00原13 old锚不变、修订其中4个new合同并新增2个old锚，共15；当前P4状态行必须用唯一上下文更新，历史重复行/P3旧pending原文不可全局替换。

更正后的retention夹具计划用实际有扩展名的aa.b、每source唯一5B文本前缀避source_duplicate；400×10000关系、每页80005B、总32002000B，仅rename共享一asset时reached-links=400。仍须真实parser/guard/PG/request测量；120000常量不是旧F界，单rename可能超400页。最终SQL只编码parent IDs，app每S asset-ID切片仍有约16R逻辑payload、capacity/临时sort/GC/旧新集合同时峰值，不能拿算法算式宣称预算通过。

作者只做包内构造、固定Git身份/行号/old唯一锚/路径状态/相对链接/差异静态核对。实际构造错误和每次exit保留外部construction-history与原stdout/stderr；失败未抹除。当时原席复核尚待；现在两轮已完成，结论见下节。产品实施、全部正反控制、实际SQL计划、容量/双轮成本、Make分片/指定main pair及设计提交门禁仍未执行。


## 修订核对第1轮

精确被审输入为revision-01/manifest.json，12154B，SHA256 `2171425c9580f52bc4c9f72c3bc85ee0072e24dc04027f8ee3dc790752fe2553`。以下为实际原席结论，root-dispositions-03-fix1已接受其设计层处置；不代表产品验证或最终设计接受。

| 原席 | 实际结论 | 报告SHA256 |
|---|---|---|
| A | A-I1/A-I2/A-M1 resolved；A-N1新Nit open | d0f40438e91c17195f0fcd3003e00c015d3761a5b9bd327e13071c45e7a093cb |
| B | B-I1/B-I2 resolved；独立确认root R-M1 Minor，不另编号 | eb376dcdc556137689e79a664e3b57463e2e62d8bda3a9e3f3a660ed6e13345b |
| C | C-I1/C-I2/C-M2/C-N1 resolved；C-M1部分修复仍open；独立确认R-M1 | 1ec52fdc8e9bd3b8fef882bc5a694eecb7680deade163aa077b863fe41e616b1 |

- a fix1 manifest SHA256 `08bb371b42df1c7beb8ca797969e115b637364511670400f988747bfe4a60185`；evidence SHA256 `f82fa3b6c44f85c91117969d2ff33df7046787b2fb5bfd23f0272c80b6d02b31`。
- b fix1 manifest SHA256 `3e9c2b07415f853184e9d9c8a0eb8acac984a3e4eaf4e77237a38837930f6346`；evidence SHA256 `f551b8f5a6a256f5a2a005a5e4a2c143b8e9772e57375429b2e7ca516c0016b0`。
- c fix1 manifest SHA256 `fbb74a324f2b4965ac60aa65ed58effaebe5d8fb6d1cc0c4fb9d7e5d8ed540ac`；evidence SHA256 `baab4f150c8ad7683f997d78e01a5331cfa508d0710ce2b2a26e9848451932ec`。
- root-dispositions-03-fix1.md SHA256 `41c68146ce352a7595d8abdb26a0dfc2e66ae702a7054a67d8569cbaf53d0e52`。

## 窄修订2提交时待核项（历史）

| 项 | 本轮修正 | 修订2提交时状态 |
|---|---|---|
| C-M1 Minor（原ID继续） | 补as2与三处解构读取，取值和base_version后续使用分列；24相关站点/9故事，S1-3真实页/version收窄；排除两本地JSON casts、保writtenOf与原断言 | 作者已修，原C窄核pending |
| R-M1 Minor（root ID） | 只改新origin常量为ReferenceBody，保既有BodyReference DTO/Bodies及iota语义；初稿已存在的声明冲突，无编译执行 | 作者已修，root/相关原席窄核pending |
| A-N1 Nit | 当前清单同步为187（186实现/生成/审计＋计划迁移1），初稿176与修订1的186只作为明确历史计数 | 作者已修，root/原A核对pending |
| C10文字澄清（非新finding） | 清理后池可执行新查询；ctx可能关物理连接，不要求旧backend PID。原错、无后续marks/publish/commit、真实回滚及WithoutCancel/rollback失败语义保持 | 原C-I2保持设计层resolved，本澄清待核 |

修订2固定清单为256 source pins、187产品路径（134既存含11生成物、53计划）、24相关version站点/9故事；原23/30 NodePaths、六Writer/Trash、两ToolDeps、15修复依赖不变。136控制ID及全部控制表行原样；只改C10后置说明段的池恢复文字，不能称matrix整文件字节不变。17 SQL整文件和15合同锚数据整文件原样；五交错与4/3/3编排不变，D/F算法与成本义务无改。

新增修订只做固定源身份、路径/计数/唯一锚/链接与窄diff静态核验，实际exit在外部过程记录；不为未实施事项补运行证据。窄修订2提交时Important零open、两Minor和一Nit修正待核；此历史状态已由后述第2轮及主持者最终处置接续。采用文档、受控00与审查记录为一个提交，必需门禁及精确main CI接受之后才可开始S1。


## 修订核对第2轮与主持者接受

本轮精确输入为revision-02/manifest.json，6772B，SHA256 `463032d25f8b1df503eb23d00d765fe0395bda61ef3b50fa19f0bf6360f2d01c`。原A第1轮报告保留其A-I1/A-I2/A-M1 resolved结论；没有伪造A第2轮报告。原B/C已实际完成以下窄核，主持者已核报告全文、证据与身份。

| 核对者 | 实际结论 | 报告SHA256 |
|---|---|---|
| 原B第2轮 | R-M1 resolved；B-I1/B-I2保持resolved；无新增finding | f2c706a599671c19fa6802ebd6e9db5a6e9491ba01977d7e31e3fd938b025103 |
| 原C第2轮 | C-M1 resolved，R-M1修订有效；C-I1/C-I2/C-M2/C-N1保持resolved，C10池恢复澄清有效；无新增finding | 72ef7b5da5656854a79869c8b77ab76aa5f4edb30034251cad6f9fcf855c330d |
| 主持者 | 按02-workflow §1.7关闭A-N1：当前187路径，旧176/186仅历史；核对完整修订并接受71/72 | 接受回执：c0d9fa1c4c3fd176e306c7cbab7ac644b841b798457c8830c1b05451054810cb |

- B第2轮manifest SHA256 `3623604bbed5dda50ac63187711e5f74422a6b536bd80395db32c0d146ee70cb`；evidence SHA256 `130d751cf11a1ed25e5a4c6db28ca1d5d12a20a82b2099e9088b07e618380e30`。
- C第2轮manifest SHA256 `f04cb55285813e0944f8289c53cfb0797c98c8bb6f581513a7448eb99912dbcd`；evidence SHA256 `f62906238f0bb9a891b6e61a55365f321d5e42303c531a99808ab95d2a7ce290`。
- 主持者接受回执 `root-design-final-acceptance-01.json`，5423B，SHA256 `c0d9fa1c4c3fd176e306c7cbab7ac644b841b798457c8830c1b05451054810cb`；实际接受日期2026-11-01，状态 `ROOT_ACCEPTED_P4_FORMAL_DESIGN_REVISION_02`。

全部12项finding已在设计层关闭：A-I1、A-I2、A-M1、B-I1、B-I2、C-I1、C-I2、C-M1、C-M2、C-N1、R-M1、A-N1；当前Important/Minor/Nit/Q均零open。256源码pins、187路径、24version站点/9故事、136控制ID、五交错、17 SQL、15合同锚及4/3/3提交编排承接已接受修订2。最终发布转换仅同步事实/接受措辞和三处00发布状态；控制与实际产品验证义务没有变化。

## 主持者的决定（负责人可以改判）与发布状态

实施决定71采用必要紧凑old-F ID关系、完整D/F继承与Unknown上界、Before/Now current证明及最终parent IDs SQL后app投影；决定72采用REST有界exact types catalogue与完整性/long页字段，MCP保留原精确SQL。完整合同和代价见[正式设计§2.3](../04-P4-virtual-ingest.md#23-主持者的决定负责人可以改判)，同时登记00§11。设计接受不等于产品或性能接受；120k/4M、合法100操作、非空100k、原预算与双轮/指定main pair义务均待实施验证。

本次正式设计自身文档commit、push与精确main CI身份尚未产生，状态为待验；不得把P3 own CI或设计静态核对当本次CI。采用时先由主持者审发布payload、在干净归档副本演练，再以受控00、正式设计和本记录同一个文档提交，完成必需八项、本次postgen、正常push及精确main CI接受后才可开始S1。S1–S3、迁移00053、全部P4控制/成本/性能尚未执行；原13ed失败、4e6e/df9指定pair和6992单次Test463/309/362/404/313的容量限定保持。此处不宣称实施放行。
