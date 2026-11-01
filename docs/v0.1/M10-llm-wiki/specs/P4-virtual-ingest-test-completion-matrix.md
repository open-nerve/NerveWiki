# P4 测试、单故障变异与完成标准矩阵

状态：正式接受设计的未执行计划，136个控制ID，原114个ID全保留；每腿分别baseline→单改→业务首败→restore。五个交错仍待实施。编译／fixture错误、通用超时与空集等同比较不算捕获。

## 1. 测试文件与命令模板

| 组 | 候选测试文件／命令范围 |
|---|---|
| VP | 新 `server/internal/bootstrap/mcp_virtual_test.go`；`go test ./internal/bootstrap -run '^TestVirtualPaths/<子名>$' -count=1`；两scope子项各实际调用 |
| VL | 新 `server/internal/modules/page/app/virtual_list_test.go`；`go test ./internal/modules/page/app -run '^TestVirtualListing/<子名>$' -count=1`；wire层另用VP/realSDK |
| VR | 新 `server/internal/bootstrap/interleavings_virtual_test.go`；`go test ./internal/bootstrap -run '^TestVirtualRead/<子名>$' -count=1` |
| VF | 新 `server/internal/modules/wiki/domain/virtual_files_test.go`；`go test ./internal/modules/wiki/domain -run '^TestVirtualFiles/<子名>$' -count=1` |
| DI | 新 `server/internal/modules/linking/app/index_directory_test.go`＋现 postgres index测试；`go test ./internal/modules/linking/... -run '^TestDirectoryIndex/<子名>$' -count=1` |
| EX | 新 `server/internal/bootstrap/virtual_export_test.go`；`go test ./internal/bootstrap -run '^TestVirtualExport/<子名>$' -count=1`，真实serve zip不是mock sink |
| IB | 新 `server/internal/modules/linking/app/ingest_before_test.go`；`go test ./internal/modules/linking/app -run '^TestIngestBefore/<子名>$' -count=1`，重要Capture用真PG适配或bootstrap对应腿 |
| IC | 新 `server/internal/modules/linking/app/ingest_coverage_test.go`与postgres `ingest_test.go`；`go test ./internal/modules/linking/... -run '^TestIngestCoverage/<子名>$' -count=1` |
| IQ | 新 `server/internal/modules/linking/adapter/postgres/ingest_test.go`；`go test ./internal/modules/linking/adapter/postgres -run '^TestIngestCandidates/<子名>$' -count=1`；真实SQL对手写oracle |
| II | 新 `server/internal/bootstrap/ingest_test.go,interleavings_ingest_test.go`；`go test ./internal/bootstrap -run '^TestIngestUnit/<子名>$' -count=1` |
| HM | 新 page postgres `ingest_history_test.go`与现http/mcp history测试；`go test ./internal/modules/page/... -run '^TestIngestHistory/<子名>$' -count=1` |
| PG | 新 page postgres `ingest_purge_test.go`；新bootstrap `ingest_invariant_test.go`；`go test ./internal/modules/page/... ./internal/bootstrap -run '^TestIngestCleanup/<子名>$' -count=1` |
| W | 新/现 web组件test文件见P4-virtual-ingest-files.json；原vitest入口精确文件+`-t 'P4/<子名>'`（重绑现package脚本，不能伪造已跑命令） |
| EW | 新LW9/10/13、现LW12 Playwright故事；按现三browser配置与heavy锁内fresh build逐phase运行指定test标题，见§5 |

Go命令cwd均`<正式StepWT>/server`；表只是未来执行模板，本轮不调用。每个新测试必须被真实Make/CI选择器收进对应片，尤其bootstrap独立recipe表达式；重绑后写真实argv与超时，不以宽泛`go test ./...`替代设计规定的原生单进程成本。

## 2. 单故障矩阵

| ID／Step | Catcher（上表组/子名） | 非空正控与精确失败断言 | 唯一变异 |
|---|---|---|---|
| V01/S1 | VP/lookup | llm root read alias/versions/backlinks/log/index-parent答virtual_file，nested/standard实页成功 | 从NodePaths删VirtualLookup包装 |
| V02/S1 | VP/batch | op0锁页、op1虚拟路径时failed=1且无写；反序对应op0 | 从batch模拟Lookup删包装 |
| V03/S1 | VP/list_directory | list index/答virtual_file而list real/返回真实child | 漏掉list目录路径包装 |
| V04/S1 | VP/move_destination | move到根index.md拒绝且原node位置不变；nested成功 | 去CheckContract最后目标保留键判断 |
| V05/S1 | VL/negative_orders | root实项sort=-3,-2,-1,0,1，limits1/2拼回index/log+5真实ID | 以-2/-1真实sort作virtual哨兵 |
| V06/S1 | VL/after_log | V2后首个真实ID是sort=-3，非根/standard合法V回真实first | V2沿O=0过滤真实节点 |
| V07/S1 | VL/canonical | V+nonzeroO/I与V3分别400，旧O/Icursor逐字节相同 | 不校验V字段组合 |
| V08/S1 | VL/fit_boundary | 真实SDK Fit恰停index之后，下一次只有log后才real，无重项 | Next取候选尾而非实际Fit尾 |
| V09/S1 | VL/glob_order | *.md与**/*.md恢复拼成全真实+两virtual按K序 | glob只加index遗漏log |
| V10/S1 | VL/overview | open_notebook含两virtual但page/asset counts=真实3/1 | counts加二 |
| V11/S1 | VP/metadata | list真实empty页bytes=0，virtual无bytes/version/updated；read无version | 用zero值省略所有真实bytes字段 |
| V12/S1 | VP/read_window | UTF8/长行offset,column跨界、next offset/column与手写slice；SDK≤48KiB | virtual绕过domain.Cut |
| V13/S1 | VP/read_fresh | 两次read之间写真实wiki，第二次index/log内容变化 | 跨调用缓存生成文本 |
| V14/S1 | VP/tool_dependency | 真实Tools缺Virtual panic；已给正常read成功 | 取消Tools的Virtual必需检查 |
| V15/S1 | VR/normal_unlocked | real read的snapshot spy=0，virtual=1，既有normal并发弱读行为 | 给整个ReadPath开snapshot |
| V16/S1 | VR/rename_snapshot | barrier前后另一tx改名，virtual答旧同一组path，下一read新path | 将virtual所有读移出RR |
| V17/S1 | VR/recheck_mode | mode hint后、snapshot首读前降级；无virtual生成，正常fallback | snapshot内不重读mode |
| V18/S1 | VR/recheck_visibility | hint后撤角色，snapshot内404且无生成内容 | 不在snapshot内重授权 |
| V19/S1 | VR/no_nested | 正常virtual快照一次、故意ctx已有tx答ErrNestedSnapshot，不join Writer | 让GenerateInSnapshot自己再开snapshot |
| F01/S1 | VF/escape_bytes | title含\[]、dest含<>、summary/messages含换行/控制字符；原样敌意片段不出现 | 省略label转义 |
| F02/S1 | VF/destination | 手写恶意dest最终仍一个Markdown目的地且无注入链接 | 省略dest转义 |
| F03/S1 | VF/newline | 注入“新标题/新列表”的message被同一行中和 | 不中和换行/控制字符 |
| F04/S1 | VF/limit_after_escape | 多byte与大量可转义字符逼近40KiB，输出≤界且UTF8完整 | 先按原值截断再扩张转义 |
| F05/S1 | VF/actual_footer | 至少3候选仅2条放得下，footer写真实2而非3 | footer用候选数 |
| F06/S1 | VF/source_line | 2非空page+1asset+1empty，source行数=3，无pending计数/调用 | 把所有source页含empty都计数 |
| F07/S1 | VF/percent_identity | x%41.md/xA.md、x%20.md/含空格路径两独立腿，解码生成链接回原ID | 每腿去字面%→%25 |
| F08/S1 | VF/percent_budget | 大量%编码扩张恰跨read/export界；整项省略、真实footer、dest未截断 | 按编码前字节计预算 |
| E07/S1 | EX/folder_only_paths | 真zip有A/与A/B.md、无A.md，A为非链接，B目的地命中entry | folder-only回退live A.md |
| E08/S1 | EX/renamed_subtree_paths | 真zip有N.md及N.md 2/c.md，child链接命中后者 | 丢映射并用live path补齐 |
| E09/S1 | EX/map_consumers | index／source行／log变化命中映射；重复ID/path/缺live/错kind/非法map各error；历史缺ID非链接 | 每腿仅跳过对应映射／校验；S2 ingest来源见U19 |
| D01/S1 | DI/typed_first | 首非string/空/JS空白/invalidFM不typed；长空白后x typed；有3有效项 | typed只看value_start非NULL |
| D02/S1 | DI/first_key | type first非string、laterstring；一个真正首string对照 | 过滤string之后才选first属性 |
| D03/S1 | DI/exclude | source后代typed页不在REST/items/types，root保留；MCP保留所有真实页 | 对MCP也启用ExcludeArea |
| D04/S1 | DI/types_scope | 另有未筛选typed页T；parent/type/tag筛选及空children、TagKey=false、空TagPages、type零命中、cursor过尾均保持全本catalogue／mode／source_area，空items不能使metadata消失 | 六腿各单改：a计数先套type；b–f分别恢复对应REST空返回／空metadata捷径；详见本节D04展开 |
| D05/S1 | DI/exact_collision | A/a及同128-char/key前缀不同尾：各2/3页，filter各只返回自身 | 按截断key合并types/count |
| D06/S1 | DI/type_bytes | 1024B UTF8 exact可filter、1025B仍422；catalogue长值页有显式说明 | 以rune数1024代替字节限值 |
| D07/S1 | DI/catalogue_bounds | 101distinct+escaped大值，≤100项且types JSON≤32KiB，complete=false | 忽略catalogue总字节界 |
| D08/S1 | DI/catalogue_complete | 小本3distinct complete=true；超项/超字节/long页各false，保留项count准确 | 截断后仍返回complete=true |
| D09/S1 | DI/judges_in_turn | 坏cursor+hidden400，badfilter+hidden422，合法+hidden404，parent冲突422 | 授权提前到cursor/值校验之前 |
| D10/S1 | DI/rest_mcp | ordinary REST/MCP拼同非空items/id序，MCP无Catalogue查询 | REST建第二套排序/用例 |
| D11/S1 | DI/stored_value_normalization | YAML转义\0的type在索引为FFFD，catalogue/选择/filter保持FFFD且返回1页；NUL请求原422 | catalogue投影把FFFD改回NUL |
| E01/S1 | EX/whole_last_hop | 真serve导出LLM zip两file＋contributed，真实页面同在 | bootstrap交空Contributors |
| E02/S1 | EX/scope | whole llm2file；standard/subtree0虚拟file，实体不丢 | 删除RootID=nil条件 |
| E03/S1 | EX/snapshot | export已有RR；中间rename/downgrade后path/mode同一旧视图 | contributor读不带export ctx |
| E04/S1 | EX/no_inbound | >1000cs/10000页输出受界，trace0 inbound查询 | export使用read profile并查inbound |
| E05/S1 | EX/meta_skip | zip realmeta contributed重导入，两个virtual不成为node | metaPaths忽略contributed |
| E06/S1 | EX/fallback_conflict | 无meta/oversizefirstmeta：LLM编号、standard真实导入；非法rootfolder冲突 | 静默跳过contributor_conflict |
| B01/S2 | IB/absent_distinct | 新P可信Absent可mark；原v1/missingP保留原链接不mark | 把missing index当KnownAbsent |
| B02/S2 | IB/has_negative | SourceValues无sources但有正文S，Capture仍非空旧S | HasSourceProperties(false)直接返回Absent |
| B03/S2 | IB/capture_before_replace | 改旧Q去S再新P加S不mark；旧Q引用已保存 | Capture放到Replace之后 |
| B04/S2 | IB/capture_before_delete | 删旧Q再新P加S不mark，Q后已不在DB | Capture放到DeletePages之后 |
| B05/S2 | IB/first_memo | rename/move/临时名反复、先改祖先后写子，回到原AGENTS/source资格 | 后次Before覆盖首次memo |
| B06/S2 | IB/before_revision | 同unit两次写/participant改写，旧head与第一次BeforeRevision一致 | 用最终revision-1推旧revision |
| B07/S2 | IB/missing_ancestor | 必要父链故意缺失使unit失败，content/index/cs全回滚 | 缺祖先回退最终tree |
| B08/S2 | IB/old_resolution | 旧边S→当前路径现在指T，Capture仍旧resolved S | Capture在最终树重新resolve |
| B09/S2 | IB/possible_state | 真实source0→非空交错返回Unknown/Possible{S}，不是Known | late非空bytes标Known |
| B10/S2 | IB/possible_not_all | 旧P完整U{S}、new{S,T}，T确切1mark | 任一未知target把完整U改All |
| B11/S2 | IB/possible_full | 旧known assetS＋unknown pageT的U必须含S/T，新R真新→仅R mark | U只留不确定T而丢已知S |
| B12/S2 | IB/possible_incomplete | 中断完整旧slots后StateAll，正常编辑成功但不误mark | 将U前缀当完整possible集 |
| C01/S2 | IC/missing_unrelated | 新P引用T；未改Q缺index且原正文含T，zero new mark | 只检查candidate reverse-hit头 |
| C02/S2 | IC/stale_unrelated | 未改Q literal extractor1，不因fixture常量更新变v2，zero mark | 将stale Q过滤后视为无贡献 |
| C03/S2 | IC/raw_eof | ≥1025raw nodes首批大量trash/source/asset，尾wikiQ挡T | eligible少于512便Done |
| C04/S2 | IC/zero_id | raw first ID=zero，末批满512再空EOF；全ID不漏重 | zeroUUID当已开始after游标 |
| C05/S2 | IC/page_content_missing | 一个live eligible page无content，error且unit回滚 | metadata INNER JOIN漏掉该页 |
| C06/S2 | IC/head_revision | page/head revision故意不等，Unknown；current健康T=1mark | 不比较page revision |
| C07/S2 | IC/no_candidates | 非no-op正文保存成功且index变化，全本coverage/未改页heads/Novel/整本SourceArea=0；W的旧Capture及≤512点Now允许 | 每次保存都先coverage |
| C08/S2 | IC/healthy_hundred_k | 100k healthy完整EOF，P新T确切1mark且保原预算 | 固定低总阈值使100k总Incomplete |
| C09/S2 | IC/budget_between | 健康查询之间软stop仅抑制mark、原edit/index成功 | softstop把前缀Complete |
| C10/S2 | IC/database_cancel | timeout／ctx两腿分别核原错身份、无后续marks/publish/commit尝试、真实回滚及连接恢复 | 每腿仅将coverage原错吞为Unknown；由身份或后续调用oracle首败 |
| Q01/S2 | IQ/old_json_shapes | object→array新标；array→same不标；冲突→唯一新标；反向不标 | 仅保旧key/resolved、不保旧JSON形状 |
| Q02/S2 | IQ/all_peers | 一个slot两peers分别candidateS/otherT，不消化S | candidate过滤在exactly-one之前 |
| Q03/S2 | IQ/duplicate_peer | 同slot两个不同range但同targetS，不消化S | peers先DISTINCT target再count |
| Q04/S2 | IQ/unresolved_peer | 同slot Sresolved＋unresolved peer，不消化S | count前滤resolved_id非NULL |
| Q05/S2 | IQ/body_empty_property | 正文NULL消化S，FM空key不消化；正常sources仍消化 | 空string property当Body |
| Q06/S2 | IQ/first_json_once | 一页许多candidate/slots，完整手写novel集；计划/计数首JSON一次 | 按每candidate边重读JSON |
| Q07/S2 | IQ/late_invalid | 前页novelS＋后页超合法array；必须error且zero persistedmark | adapter拿到首novel立即返回 |
| Q08/S2 | IQ/invalid_no_novel | 所有candidate已digested但晚malformed仍error | 空novel时跳过invalidflag |
| Q09/S2 | IQ/invalid_peer | slot中target空或asset=true/idNULL（独立两腿）各error | 忽略SourceSlots.Observe等价malformed检查 |
| Q10/S2 | IQ/stream_failure | Scan、rows.Err、字节流/DTO错行分别单故障，各error/rollback | catch-all转Unknown |
| U01/S2 | II/index_serial | I1两unit同S先者1后者0，checkPages/checkLinks真非空 | 去index advisory lock |
| U02/S2 | II/content_wait | I2 B已改Q行等index，A无content锁读旧committed对齐 | index内FOR UPDATE别人的content |
| U03/S2 | II/source_first_bytes | I3 source首次写SHARE可交错，Capture保守、独立T仍1 | 把index串行当Ttree bytes快照 |
| U04/S2 | II/kind_gate | import/version_restore/trash_restore/revert各0mark；同edit=1 | 移除Kind=edit闸门 |
| U05/S2 | II/direct_only | 直接assetS 1mark，sourcepage附件跟随不另计，empty/root0 | 将跟随附件都加marks |
| U06/S2 | II/marks_append | 同edit session先S、去S、再S：count恒1；再T=2 | 用当前引用集合替换marks |
| U07/S2 | II/marks_cap | 9999已有+重复+至少2新：10000且seq唯一，另cs可继续 | INSERT不受剩余容量限值 |
| U08/S2 | II/complete_candidates | 前10000候选均已digested，后有新T且当前cs有空间，T必mark | 在NovelSources前截candidate到10000 |
| U09/S2 | II/marks_rollback | marks故障整unit原content/index/cs保持、无新event | marks另事务提交/吞error（分腿） |
| U10/S2 | II/bridge | 真API PUT和MCP batch各新source=1mark | bootstrap不传Before/Kind/ChangesetID之一（分别单故障） |
| U11/S2 | II/reindex_no_history | v1重建后保留引用不补mark；后续真正新T=1 | reindex伪装edit生成历史 |
| B13/S2 | IB/target_literal_v1 | 新P仅直引非空source S，S头literal extractor1，zero mark；另current T真1 | Now目标只核bytes不核Extractor |
| B14/S2 | IB/target_missing_head | 非空S缺indexed head不mark；独立current T真1 | missing目标头当current |
| B15/S2 | IB/target_current | S同本/live/source/非空，Present+Extractor2+matching revision，S确切1mark | 把所有source page目标都排除 |
| B16/S2 | IB/asset_without_page_head | 合格source asset A无page head仍确切1mark | asset也强求indexed page头 |
| B17/S2 | IB/target_revision | source S头extractor2但revision错，S0/T1，Now DTO可见证据 | 只核Extractor、不比较target revision |
| B18/S2 | IB/before_target_head | W来源S合法system Rewrite从r到r+1，旧Q引用S的Before头必须为r且Known；DTO精确核旧revision、T1 | 旧target头读取移到Replace后，r+1不能证明Before r，DTO首败 |
| Q11/S2 | IQ/parent_evidence_complete | candidate先可novel、最后parent的Scan/flag/rows.Err独立故障仍原错回滚；每候选及parent恰一行、最终SQL调用1 | 只消费candidate行即提前成功 |
| U12/S2 | II/old_follow_untouched | 未改Q→非空current S，S正文→同级asset A；新P直引A/T，手写marks=[T] | 只查旧direct D、去F_U扩展 |
| U13/S2 | II/old_follow_changed_q | Q在W改去或删除（两腿），S未改且正文→同级A；新P→A/T，marks=[T] | F_U只交D_U、不交旧D_W |
| U14/S2 | II/old_follow_delete_both | Q→S、S正文→同级A；同批删Q和S，保A，新P→A/T，marks=[T]；D/F删前观测非空 | source D=Ineligible后不读F_W |
| U15/S2 | II/old_follow_rewrite_s | Q不在W仍指S，asset rename令S system Rewrite仍live/current；旧F A保到D_U，P→A/T仅T标 | Capture结束丢未与D_W相交pairs |
| U16/S2 | II/f_without_d | current S正文→A，但没有wiki指S；新P直引A/T，marks=[A,T] | 对全部旧F资产无条件当已消化 |
| U17/S2 | II/unknown_follow_closure | old Q的完整PossibleDirect={S,K}含known K，S的完整F={A}、K的F={B}；new A/B/T仅T标，状态仍Unknown；F missing腿仅asset保守而page T标 | 分腿仅漏S→A闭包／漏known K→B／把asset-only未知变AllDirect |
| U18/S2 | II/unknown_f_unreferenced | S旧F缺证但完整D_W/D_U均不含S，P直引独立A/T，marks=[A,T] | 见任何UnknownFParent就无条件AllAssets |
| U19/S2 | EX/ingest_export_map | 真whole zip，log ingest来源使用实际Scope映射；历史缺ID纯文字且不补后位，root/folder来源不指虚构md | ingest source沿用Notebook原路径 |
| U20/S2 | IC/retention_cost | 400×300、400×10000、100ops与100k分腿真实接受且A0/T1；R、bytes、head/F读量及peak/原预算逐值记录 | 丢全部retained以省成本；业务A0首败，不靠超时 |
| U21/S2 | II/untouched_deleted_source_approx | Q不在W、S在W被删，A同级存活，最终Q不再有可消化S，D_W空，P→A/T应[A,T]；与U14并列 | 为Q强造Before D或无条件保全部oldF |

| H01/S2 | HM/true_false | notebook/actor/page三filter TRUE只ingest；false/omit同原结果 | false也走专用ingest SQL |
| H02/S2 | HM/preview_order | 25来源seq中trashed/purged占位，list3/detail20/log5位置准确 | 压缩null再补后项 |
| H03/S2 | HM/seq_order | sourceUUID刻意与seq反序，preview仍seq顺序 | 预览按node_id排序 |
| H04/S2 | HM/actor_page_plan | 原TestTheLogReadsUseTheirIndexes新三支noStats/custom/generic非空命中正确索引 | 用optional OR替代专用TRUE SQL |
| H05/S2 | HM/protocol_roles | reader+两MCPscope得真实count/source，隐藏404，kind仍edit | ingest改为新kind或require writer（分腿） |
| P01/S2 | PG/leaf_loop | 2500marks/1cs和500cs×3各同run全可清且notebook被清 | leaf只删一次不循环 |
| P02/S2 | PG/count_return | 页purger返回包含leaf数，jobs同run继续直到notebook可清 | 返回漏leaf数 |
| P03/S2 | PG/parent_deleted | 保留leaf锁住时deleted父仍存在，其他可清 | deleted父去NOT EXISTS sources |
| P04/S2 | PG/parent_trimmed | 保留leaf锁住时trimmed父仍存在，其他可清 | trimmed父去NOT EXISTS sources |
| P05/S2 | PG/skip_locked | I5持一个cs锁，purge不等它而清另500cs | leaf去SKIP LOCKED |
| P06/S2 | PG/eligibility | 活未trim、未过retention或仍有items/revs不清marks | leaf比父资格更宽 |
| P07/S2 | PG/orphan_mark | SQL播一条无cs mark，checker明确报该ID | 去mark→cs不变量 |
| P08/S2 | PG/wrong_count | SQL改一个cs count，checker报差异；合法清理窗对照 | 去count不变量 |
| P09/S2 | PG/wrong_kind | SQL在import cs造mark，checker拒；edit对照 | 去kind不变量 |
| P10/S2 | PG/schema_negative | negative count/duplicate seq/duplicate source三个独立违反各拒 | 每次仅删对应constraint（分腿） |
| W01/S3 | W/virtual_rows | mode切换两链接消失，真实树ID全集不含伪node | 向PageTreeStore添加虚拟node |
| W02/S3 | W/reader_virtual | reader未resolved根virtual成功；resolved真实同名保持landing | reader早于virtual导航先拒绝 |
| W03/S3 | W/shared_cases | Go/TS同fixture逐行含嵌套/normalize/双扩展；非空positive/negative | TS剥两次.md或忽略根位置（分腿） |
| W04/S3 | W/key_generation | 两打开/filter/cursor的迟到答复只能落自身代，Back/Forward正确 | SWR key漏location.key |
| W05/S3 | W/filter_key | 切type/tag/ingest真实请求变，旧页不污染 | key漏筛选值（各字段独立腿） |
| W06/S3 | W/retry | current代目录普通网络失败，观测第二次真实retry请求，200展示3项 | 仅把目录onErrorRetry改为空操作 |
| W07/S3 | W/event_staleness | pages事件不自动目录重算；mode mismatch/重连显提示 | events触发目录重算 |
| W08/S3 | W/focus | More后非空新增项可读，用户已移动焦点则不夺焦点/滚动 | async完成无条件focus/scroll |
| W09/S3 | W/history_source | asset去父focus，null纯文字无link，ingest附原kind | 用pageURL导航asset |
| W10/S3 | W/catalogue_notice | caps/long两种明确提示，保留URL已选type，不用显示前缀请求 | incomplete/long当完整可选菜单 |
| EW01/S3 | EW/LW9-page | 浏览器真实写→历史filter/来源可见，至少一个mark | 页面请求漏ingest参数 |
| EW02/S3 | EW/LW10-page | 树链接/reader/真zip内容正确 | 真实route不注册index |
| EW03/S3 | EW/LW13-page | ordinary Places入口、type/tag/More、两非空组 | 普通本隐藏目录入口 |
| EW04/S3 | EW/LW12-virtual | 原双tab同stream，降级后两virtual消失，未reload | 不响应mode改变虚拟行 |

W06固定为当前代、普通网络失败、下一次200三项，观测真实retry请求。只抑制目录onErrorRetry会缺该请求并首败；原“套healthRetry”在此输入会委托同一retry，是等价审计，不能计作已杀变异。没有为listIndex新增503/server_busy。

C10两腿独立执行。timeout腿在已进入coverage的真实PG语句设置局部statement_timeout，以可观察锁等待触发；adapter／coverage边界errors.As得到PgError.Code=57014，在rollback成功时unit保留原错误链，不能变25P02或pgx.ErrTxCommitRollback。ctx腿在实际PG查询开始后由barrier取消请求ctx；边界errors.Is(err,context.Canceled)（驱动若返回57014，adapter以errors.Join保留ctx.Err及driver原cause），不只看err!=nil。两腿在真实TxManager旁加窄观察hook记录callback退出错误与Commit/Rollback尝试；出错后marks/publish/commit尝试均0、rollback被请求，真实content/index/cs/marks全回滚，清理后连接池能提供连接执行新查询；取消可能关闭物理连接，不要求复用原backend PID。hook只观察、不替代PG或改WithoutCancel。吞错即使被PG失败事务阻止提交，仍由原错身份／禁止后续调用oracle首败。rollback若另失败，保留当前TxManager优先报告基础设施错误的语义，在coverage边界核原cause。可追加非abort Scan腿，但不能代替两个真实取消腿。每腿分别baseline→单故障→指定业务首败→restore。

有“分腿”的行不是混合故障，每腿各自baseline→单改→首败→恢复，报告逐腿身份与数量；表中总行数不冒充控制执行数。Q06/H04等物理工作变异由真实SQL/trace或计划业务上限断言捕获，不能仅比同一段被测算法的计算结果。

D04在初稿已有六腿，本修订保留原行；本轮准确ID／改行计数见静态核对，不沿用“本稿114／113不变”的历史计数。D04六腿每腿各自baseline→单故障→业务首败→恢复，不合并故障：a原type先过滤catalogue；b合法空父children早退；c TagKey=false早退；d TagPages空早退；e精确type零命中返回空metadata；f cursor越尾返回空metadata。手写夹具另有一个不受当前筛选命中的wiki页T(type="topic")，所以每腿空items时仍有types=[{type:"topic",count:1}]；LLM夹具有一个非空来源S，断言mode=llm_wiki及source_area={root_id:raw,sources:1}，完整性字段也逐值核对。父页本身无type且不改变topic计数；normal模式对照source_area=null。真实parent／TagPages／catalogue读取错误仍沿原次序返回，不能吞成合法空结果。MCP对照保持原空返回且CatalogueHeads调用数0。这些仍为未执行计划。

U04的revert腿基于当前WriterDeps/TrashReads.RevertTrees接线。复用当前六字段Tree夹具与nil/empty差异，不恢复旧[]domain.Node或ListNodes撤销故障注入。现有完整活树/附件、ctx事务/取消、锁下重读、source facts与untouched witness回归作为需保留的修复基线列引用；不计为P4新控制已通过，也不替代U04真正kind-gate oracle。 实际修复回归行见源码事实 F26–F29；它们只要求保留，不计为本轮新执行结果。

## 3. 五个确定性双事务交错与手写oracle

**I1 两unit争同S**：先建非空source S与两个空wiki P/Q，不预置任何consumption。测试外连接持index key，A/P和B/Q均已写自身content并在advisory锁等待；明确放A获锁并commit，再放B。A cs sources=[S]、B sources=[]；反过来只交换谁1谁0。A在marks处故障回滚腿：A无content/version/mark提交，B=1。结束checkPages/checkLinks与source_hash不变量，不能只看总mark≤1。

**I2 未改emitter与已改未提交content**：A/P已获index并停在coverage前；B持自身Q content、写Q新内容但等待同index（观测锁等待，不sleep）。A的plain SELECT看Q旧committed revision r和indexed r；A完成后B才更新index r+1。A在index下再锁Q会形成A→B content、B→A index的环，反向控制应由锁/进度与业务断言证明，不能把通用超时当oracle。第二顺序B先commit，A后获index应见r+1。Q旧有S时A新增S不得mark；Q原无S时按完整旧事实A的新T应mark。

**I3 旧target bytes可达交错**：初始source页S bytes=0，wiki P旧正文含S边，另非空新source T无人引用。A是普通P正文PUT，book SHARE，先捕获P旧revision/bytes，再写P为S+T、停在index前。B是S第一次非空普通PUT，book SHARE兼容，取source-hash（A不取）、获index并commit；A再获index，late S bytes>0。手写真旧消化集不含S；但A没有S的Ttree bytes证明，Capture应Unknown/PossibleTargets={S}，本次S不新增、T恰一mark。捕获DTO状态是必要oracle：误写Known也可能同样零S，不能仅用历史数量证明旧事实正确。反向S非空→空的生产路径先核guard拒绝或Tree锁阻塞；不以任意SQL直写制造不可达竞态。

**I4 树/source-area变化**：A持book SHARE后开始多批raw coverage；B试创建UUID小于A当前after的新live页，或移动Q入source、删除root、downgrade／reindex，必须在notebook锁等待，不能混进本扫描。A自身Tree batch的改/删W走first memo，未改Q按最终tree近似单列。主控数据：旧Q引用S后被本unit删除、另P加S仍0；新T在健康未改页中无人引用必须1。若只扫最终tree，删Q控制会误标。

**I5 清理**：独立事务锁一个符合清理的cs；另500cs×3或单cs2500叶可清。purge先清未锁者、返回包含叶的数、同job继续清父/notebook；被锁cs与其marks保留。放锁后再run清剩余。对deleted父和trimmed父分别执行；不得停止/删除无归属数据库或构造系统级实验。

## 4. 语义／成本／完成文字覆盖矩阵

| 批准义务/新增候选 | 正向与反向覆盖 | 应形成的结果／文档 |
|---|---|---|
| 00 4.9/4.13根路径、MCP两scope与batch | V01–V14、D09、E01–E06 | S1§10、总体13.1路径/V、M9移交项3 |
| ReadPath正常无事务、virtual own RR、export reuse | V15–V19、E03、I2与真正后段读交错 | 00 A02、总体13.1读一致性、真实快照调用次数 |
| 中和／40KiB／read window／SDK界 | F01–F08、V08/V12 | 输出实际bytes/UTF8/实际N/M；总体13.4，不填估计通过 |
| Index typed/jsTrim/首key/REST/MCP/type catalogue | D01–D11；当前type新候选A11 | 真PG/HTTP、source排除、计数范围、exact collision、cap/complete/long；正式三审处置A11 |
| whole/subtree/standard、meta/fallback/conflict | E01–E09，LW10API/MCP/page | M7项1/2/6、M11旧保留根边界，不修M11 |
| Before初始资格／四态／Unknown界 | B01–B18、U12–U21、U03、I3/I4 | 00 A04–A08、P3移交兑现、Capture DTO与源byte证明 |
| coverage全rawEOF/current、无候选不扫、错误回滚 | C01–C10、U02、I2/I4 | 100k真实非空mark＋零无候选全扫；不能用“总Unknown”过门禁 |
| 首JSON一次／全peers／error全扫描 | Q01–Q11、U08 | SQL计划/真实JSON访问工作计数、手写oracle差分、late error rollback |
| mark edit/cap/session/index串行 | U01–U21、I1 | 10000/seq/unique/count实际行、Kind与桥接最后一跳；reindex零回填 |
| 迁移／runtime/sqlc／CHECK与indices | P10及UpDownUp/runtime-role/schema names；history plans | 计划00053迁移、ALTER到commit锁时长、数据规模、rollforward/Down丢标说明 |
| 清理／三不变量／同jobnotebookpurge | P01–P09、I5 | M8移交处理进展、总体13.1#6/13.4；retention/trimmed谓词准确 |
| history true/false/filter/前N/null/权限 | H01–H05、W05/W09、LW9 | API/tool/types/fakes同Step、总体13.2 URL/null/附件落点 |
| S3焦点/模式/keys/retry/共享fixture | W01–W10、EW01–04 | 三browser和runtime身份，原LW12同stream；总体13.2/13.4与M12selector推后 |
| read100k最坏inbound/export100k+1000cs | 真PG/bootstrap cost新selector；E04错误query路径 | 保原时间/内存线，cold/warm/noStats/custom/generic/更大另一本、实际SQL范围 |
| ingest100k/maxbatch/sets编码/JSON单合法大值 | C08/U08/真实cost；S→T和cap oracle非空 | Go heap/TotalAlloc、wire参数、PGbuff/temp、index锁时长；M12原义务 |
| 全distinct与长type工作上限 | D05–D08/cost；bounded输出与EOF | <=100项/<=32KiB候选界（正式三审可改判）、完整性明确、max合法长值慢支路真实TOAST |
| 门禁/CI/三审/文档真实身份 | 原02流程，源码/生成物/bin/HTTPassets身份链 | 全本地→精确绿色分支CI→三代码审查；修复CI先于原席复核；doc/main/record/doneCI |

## 5. 必须实测但本次未运行的命令与材料

正式Step使用原heavy.sh、原全局锁、锁后3GiB/T7，fresh build与源/runtime哈希。候选成本测试入口建议：`TestVirtualIndexOfAHundredThousandPagesTakesItsTime`、`TestVirtualExportOfAHundredThousandPagesTakesItsTime`、`TestIngestOfAHundredThousandPagesTakesItsTime`、`TestIngestAtTheBatchBoundaryTakesItsTime`、`TestIndexCatalogueHasBoundedWorkAndOutput`；全部是未来新test，最终Make recipe/timeout由正式main逐行重绑，沿原预算，不填当前不存在的实测bound数。

迁移测Up/Down/Up、已有数据不回填、schema名称/negative数据/runtime-role、实际goose事务从锁获得到commit；不得拿P2已做6M迁移数据顶本P4结果。SQL计划使用真实noStats/ANALYZE/custom/实际generic，另大本无关数据和MVCC实际影响照实报告。普通无candidate和健康100k candidate是不同数据集，不能用前者替后者。

每提交minimum，完整`make lint gen-check knip build`／`make test`／`make test-go-short`／`make e2e`；!race成本/guards独立。每规定进程双轮、算法每轮≥3样本，中位数/×3与process均值×2.4/timeout半界沿00；五bootstrap片、两指定mainTest均值<450s。浏览器每phase重新build当前源码，record actual identities，W/EW分账；SQLite/mock/只打印SQL都不能称真实PG验证。

## 6. 当前记录

本材料中全部测试、变异、容量、SQL执行、浏览器和门禁状态统一为 `NOT_RUN_FORMAL_DESIGN_REVISION_01`。上表测试选择器和新增文件是可审实现目标；没有填passed/failed计数、日期、CI或迁移完成。正式设计接受前应处置types A11与所有候选小决定；正式实现后再以原始日志填写实际首败/恢复及未完成义务。

新增old-F成本义务详见主文§3.8：400×300与400×10000关系、实际100ops以及100k完整非空mark，所有原预算保持；120000为reached rewrite links而非旧F上界。最终SQL仅parent IDs，app投影的slice/capacity/去重/sort/GC及旧新集合同时峰值实测。U20归IC/retention_cost，实际成本进新bootstrap/ingest_cost_test.go的独立!race recipe，与linking coverage catcher分开记原生命令。
