# P4 目录 types：exact 身份、输出界与验证

本方案由主稿§2.3实施决定72采用，三位fresh初审、原席复核和主持者设计接受已完成。它补足00未定的type身份碰撞、超长值、catalogue计数范围与输出界，保留既有exact筛选。尚无P4真实成本结果，设计接受不证明运行可行。

## 固定6992代码与批准文字

- `linking/app/list_index.go:132–145` 的长度判定是Go `len(*t)>MaxIndexType`；`domain/index.go:26` 常量1024，单位是**UTF-8字节**，不是SQL字符。空字符串/NUL另拒。type值本身的写入没有这个1024B限制。
- `queries/index.sql:9–28` 先首ASCII键（translate TYPE）、按position LIMIT1；`value_start`是前257**字符**，`IndexPages`只返回前128字符供排序；短filter比较前缀，≥257字符且长度匹配才读精确JSON值。MCP type filter是原始完整字符串精确相等，不按TitleKey、jsTrim或显示前缀相等。
- `domain/index.go:43–66,89–133` 的cursor/order是Typed、最多64B的type key、UUID。key由前128字符full-casefold/NFC得到；不同真实type可能casefold相同，或前缀/64B截断碰撞，现用id排同key组，不证明它们是同一个exact filter值。
- P4按00 §4.10修“有类型”：frontmatter_valid且首ASCII键为非空jsTrim字符串。jsTrim只决定是否缺失，**不把原type值trim后拿去filter**。首type为非字符串不能跳到第二个。长空白前缀后才有非空字符需P3同一全值慢分支。
- 00 §5的listIndex行只说types为出现类型与各自页数并排来源；§4.16说从它选择类型。**没有明确**type/tag/parent筛选是否缩types计数，没有规定key碰撞、超过1024B类型是否可选、目录答复types整体字节界。总体5.5也没有补这三项。当前主稿把“全本排来源、独立筛选”列为推荐，不把它说成已批准文字。

补充存储归一事实：`adapter/markdown/facts.go:23–26,76–89`经jsonOf/text把YAML解码的U+0000转为U+FFFD，`facts_test.go:166–198`已有fixture覆盖；PG保存jsonb。本文“完整/原/exact type值”一律指**既有索引实际保存的完整值**，不是原Markdown字节或转义拼写；保持现NUL→FFFD规则。请求参数本身NUL仍由ListIndex.check拒绝，不新增“NUL无法筛选”页计数。

## 有界方案（决定72；实施与成本待验）

1. 仍以**精确完整存储值**识别可筛选的type，相同exact值才合并count；canonical key只排序，复用既有IndexKeyer（自身先取128字符、再fold/NFC并截64B），碰撞后以完整存储值字节排。绝不拿256B显示前缀、128字符头、64B key或trim后的值作filter参数。
2. 推荐types/count范围为全本可列wiki页、LLM排来源，type/tag/parent筛选之前；00目前没有明确这一层，A11提出修订。IndexQuery新内部Catalogue=true只由REST设置，MCP不增加这笔读取。普通ListIndex仍不自开事务，types_complete表示**未因catalogue限额省略**，不是跨语句RR快照保证；其只处理当次第一遍完整索引头所定有限IDs，不能追逐并发不断新增节点。
3. 维持拟定types=[{type,count}]，每个type只包含真实exact且UTF-8≤1024B的完整存储值。新增候选必填 `types_complete:boolean` 与 `unfilterable_type_pages:integer`；后者是有类型但存储type>1024B的**页数**，不是distinct类型数，不冒称把多个长type合成一项。候选代码常量 `MaxIndexTypes=100`、`MaxIndexTypesJSONBytes=32*1024`：序列化types数组的实际JSON字节（含转义/分隔）不超32KiB，最多100项；没有新配置。实施实测若要求改判这些已定常量，须向主持者交具体证据重新处置，不自行放宽或取消界。
4. 全部eligible IDs按已拥有的有序有限数组分成≤512ID批，`IndexCatalogueHeads`逐ID点取首属性，每请求保留一行（ip消失也显式typed=false）。只物化一批；app维持最多100个exact值的map+最大堆，按(type-key,完整存储值)留最小100。阈值只下降，被排除/淘汰的值不可能后来重新入选，因而保留项从首次出现到EOF计数完整；遇到第101个distinct就知道不完整，无需保存全distinct集合。EOF后按稳定顺序尾部移除直到实际JSON≤32KiB，也标不完整。任一long页存在同样complete=false。不得在没有EOF时声称count最终准确，DB/Scan/ctx错误照原读失败，不返回貌似完整前缀。
5. type完整存储值的长度检查复用value_chars/value_start：≤257字符前缀就是全值；257<chars≤1024时完整字符串最多4096 UTF-8字节，才读该有界首JSON值来判1024**字节**；chars>1024一定不可筛选，不为catalogue读整个长值。唯有长type前257字符全是JS空白时，为判是否缺type仍须原P3全值jsTrim慢分支；它有现合法content/payload界，必须量TOAST，不能假装完全不读长值。只读首type索引值，不读/重解析正文。
6. UI label沿可读截断展示，URL参数始终完整exact。`types_complete=false`时显示“仅显示部分可筛选类型”，不伪装穷举菜单；long页数非零另显示“有N页的类型名称过长，无法按该类型筛选”。页面条目的type仍沿原256B表示合同；长类型不在菜单制造假exact选项，其页仍在目录可见。已有URL选择若不在有限菜单中，仍以当前完整参数作选中展示，不发新自动请求／不清除筛选；不因此把选中值加入服务端catalogue或假造count。这些新增响应字段与UI文字已随**决定72及必要00/新REST修订**接受设计，依据见正式审查记录；实际行为与预算仍待实施验证。

空items不是空catalogue的依据：合法空父children、TagKey=false、TagPages空、type零命中和cursor过尾均仍返回当前mode/source_area和全本排来源的types／计数／完整性字段。REST将现非错误早退改为itemsKnownEmpty后继续metadata，真实错误保持原优先次序；MCP继续廉价空返回。D04每腿都另有未筛选eligible typed页，防止空集自证。

最坏输出有终止边界：types<=32KiB加两个固定大小标量；100个完整≤1024B值的保留集；单批<=512行，每行最多一个≤1024B filter值。完整输入工作仍随本次有限eligible页数变化，属于真实100k成本，不声明O(1)；原IndexPages全头、source集合与ID编码已有/新增heap均要量。计数不读前100个源页就停，不能以有限输出当有限证明输入。

## 有限备选

- `types_next_cursor`续取独立catalogue页：能逐次浏览所有可筛选types，但要定义绑定notebook/filters、exact/key碰撞游标、跨写一致性、总输出与每页计划；长type仍需上述显式处理。它新增第二分页维度与UI，在首次P4优先选有限catalogue+complete标志，交三审改判。
- 只设项/字节界而不标未完整、静默合并prefix、把long全丢掉：与完整type承诺冲突，不推荐。
- 改MCP filter为key、限制所有写入type长度、新建长期catalogue表：会扩大既有产品行为/迁移范围，不推荐。

没有实测就不能证明上述额外读取满足4s和100k原预算。若实现/计划不达标，要向主持者提交实际证据并修正式设计；不能降低预算，不能把完整性标志改成一直false来省略全输入计数。

## 必须实测与手写oracle

- A/a、同128字符/64B-key但尾不同的type各2/3页，保留时各count准确，点击仅自己的2/3；一个被淘汰type再多次出现不污染保留项count。乱序ID数据与手写完整oracle比Top-K，不用同一个heap算法算期望。
- 100/101distinct；32KiB前后转义膨胀；字节cap导致不足100项；complete真/假分别明确。真实max JSON bytes断言不是sum原string长度。selected URL在catalogue外仍可精确filter。
- 1024B两侧多字节字符串；1024B可选、1025B原filter422；long页数精确（不是distinct个数），类型页仍显示，菜单不发非法filter。空/JS空白/首非string/invalidFM不typed；长空白后x为typed；source后代排除，source root按正常资格。
- type/tag/parent各组合items变化但推荐catalogue计数范围不变；普通MCP与REST items同序，MCP无catalogue SQL。
- 真实100k短type、全部distinct、重复长值、少量合法最大长值、key碰撞、type短而无关JSON巨大、另一更大本；noStats/ANALYZE/custom/真实generic、TOAST/临时文件/Go heap/编码/HTTP字节、用例墙钟。完整EOF+非空类型列表，不能以空返回或always-incomplete规避。
