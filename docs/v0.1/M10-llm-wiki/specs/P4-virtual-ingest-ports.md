# P4 端口、DTO 与契约（正式接受设计）

所有新增签名属于主持者在正式三审及原席复核后接受的设计；实现与生成仍待。现有入口以固定6992源码为准。模块分别定义同构类型，仅由bootstrap逐字段转换，不跨app／adapter引用。context、time、uuid与domain沿仓库用法；决定71／72见主稿§2.3。

## 1. 路径／虚拟 read（page 所有）

```go
// page/app; page root aliases this narrow interface and its own NotebookState DTO.
type NotebookStates interface {
    StateOf(ctx context.Context, id uuid.UUID) (domain.NotebookState, bool, error)
}
func NewNodePaths(store PathStore, notebooks NotebookStates) NodePaths
// page root: change existing NewNodePaths(pool) so no module reaches notebook adapter.
func NewNodePaths(pool *pgxpool.Pool, notebooks NotebookStates) NodePaths

// page/domain; ErrVirtualFile remains the existing sentinel.
type VirtualFileError struct { Name string }
func (e *VirtualFileError) Error() string
func (e *VirtualFileError) Unwrap() error // existing ErrVirtualFile
func VirtualLookup(state NotebookState, next Lookup) Lookup
func CanonicalVirtualRead(path Path) (VirtualName, bool)
type VirtualName string
const (VirtualIndex VirtualName = "index.md"; VirtualLog VirtualName = "log.md")

// page/app; uses page/domain.VirtualName, aliased at page root, not arbitrary strings.
type VirtualReadScope struct {
    NotebookID uuid.UUID
    Notebook domain.NotebookState
}
type VirtualFiles interface {
    GenerateInSnapshot(ctx context.Context, scope VirtualReadScope,
        name domain.VirtualName) (string, error)
}
func NewReadPath(notebooks Notebooks, paths PathStore, nodes Nodes,
    auth shared.Authorizer, snapshots shared.Snapshots) *ReadPath
func (r *ReadPath) WithVirtual(v VirtualFiles) *ReadPath
```

`WithVirtual`复制配置，不并发修改 Module 共享 usecase。NewReadPath 用现 page.Deps.Snapshots；Tools 检查 Virtual 不为nil再配置副本。虚拟结果在现 Read/PageRead 与 list条目增加 `Virtual bool`，adapter只在true输出 `virtual:true`、不输出version；真实页仍原version/bytes/updated，零byte不丢。ReadPath.Execute参数及TextMeasure、domain.Cut不变。NodePaths Find/Walk在已授权后读StateOf，unit路径仍使用ctx已持book锁。batch的pure tree直接接plan已有state，不引入无锁再次查模式。列表/open_notebook自行取已有授权后的state；不得绕Lookup漏`list index/`。

现两处page.ToolDeps字面构造为bootstrap/mcp.go:48及batch_cost_test.go:64；S1-3给!race fixture接兼容Virtual依赖（窄stub须拒绝意外virtual调用），保留成本recipe／预算。它们独立于23文件／30处NodePaths审计。

CanonicalVirtualRead必须与现ParsePath实际语法匹配：normalize／标题键判定来自共享fixture；只把规范单扩展页面读取映射为生成器名，`index`/`index.md.md`等仍交VirtualLookup拒绝。四根键与nested/standard对照见矩阵；新函数不改变ParsePath的422顺序。

## 2. 目录（linking 所有，page 窄元数据）

```go
// Extend existing app.IndexQuery; public protocols expose disjoint parent forms.
type IndexQuery struct {
    Type, Tag, Parent *string
    ParentID *uuid.UUID
    AtRoot bool
    ExcludeArea bool
    Catalogue bool
    Limit *int
    Cursor *string
}
type IndexItem struct {
    PageID uuid.UUID
    ParentID *uuid.UUID
    Title string
    Path string
    Type, Summary *string
    Tags []string
    ChangedAt time.Time
    Inbound int
    ParentPath *string
    Cut []string
    Cursor string
}
type IndexType struct { Type string; Count int }
type IndexSourceArea struct { RootID uuid.UUID; Sources int64 }
type IndexPage struct {
    Items []IndexItem
    Next string
    Types []IndexType
    SourceArea *IndexSourceArea
    TypesComplete bool
    UnfilterableTypePages int64
    Mode string // "standard" | "llm_wiki", from the same call's notebook state
}
// Existing page/root and linking/app PagePath both gain these two fields.
type PagePath struct {
    Path string
    ChangedAt time.Time
    ParentID *uuid.UUID
    Title string
}
// Existing NodePaths.Pages returns live same-notebook pages only, including metadata.
// Parent validation uses Pages(..., []UUID{parentID}); missing => page.not_found.
// ChildPagesAt distinguishes nil(root) from a real zero-UUID parent in SQL.
// Preserve legacy ChildPages for existing consumers; ListIndex moves to this pointer form.
func (p nodePaths) ChildPagesAt(ctx context.Context, notebookID uuid.UUID,
    parentID *uuid.UUID) ([]uuid.UUID, error)

type IndexNotebookState struct { Mode string; SourceRootID *uuid.UUID }
type IndexNotebooks interface {
    StateOf(ctx context.Context, notebookID uuid.UUID) (IndexNotebookState, bool, error)
}
type IndexSourceNode struct { ID uuid.UUID; Asset bool; Bytes int64 }
type IndexSourceReads interface {
    Nodes(ctx context.Context, notebookID, rootID uuid.UUID) ([]IndexSourceNode, error)
}
```

ListIndex增加`Notebooks IndexNotebooks`与`Sources IndexSourceReads`；linking.Deps收同接口，bootstrap逐字段适配notebook.NewNotebooks与page.NewSourceArea。Access还是现index.list决定，读取state一次。按既有候选的内部Catalogue明确两条读取分支：

- REST设置Catalogue=true与ExcludeArea=true：IndexPages(nil)读取未按type/tag/parent过滤的完整首遍头，排严格来源后代；Sources数量为asset或Bytes>0。对该有限IDs集合分批CatalogueHeads，以完整存储值统计types/count并产生Type匹配IDs，然后应用type/tag/parent及分页。不得先用IndexPages(typeIs)缩小统计范围，也不以显示前缀代exact值。
- MCP设置Catalogue=false与ExcludeArea=false：继续IndexPages(typeIs)的既有精确type SQL，不调用CatalogueHeads、不承担catalogue全量统计扫描，保留真实页目录。

两分支共用资格、排序与游标，不重排现TagPages/parent读取与错误先后。上述是原Catalogue仅REST方案的消歧，未新增产品决定，也无成本已验证结论。

types exact指既有索引保存的值（沿现NUL→FFFD归一，不是原Markdown字节），key碰撞、1024B/项数/JSON字节界及计数范围见 `P4-virtual-ingest-types-boundary.md`。决定72采用有限exact catalogue、显式types_complete和不可筛选long页数；00 A11与新REST两字段已接受设计，实施与成本待验。内部Catalogue仅REST设true。IndexReads新增窄口：

```go
type IndexCatalogueHead struct { ID uuid.UUID; Typed bool; FilterType *string }
// FilterType=nil with Typed=true means complete stored UTF-8 value exceeds MaxIndexType.
type IndexCatalogueReads interface {
    CatalogueHeads(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) ([]IndexCatalogueHead, error)
}
```

ids为完整首遍头集合排来源后的≤512子批，每request保留一行。app从全部有限批计算有界top-K/准确保留项count；若REST带Type筛选，同流的完整FilterType同时产生匹配IDs，不截断后拿显示前缀筛，也不为每个type重复整本读取。MCP仍既有IndexPages(typeIs)精确SQL。基于原未锁读的concurrent消失仍沿现弱一致性，types_complete只表示无额度截断，不新宣称snapshot。

REST的items筛选与响应metadata分离。6992 ListIndex:93–104遇到合法空children、TagKey=false或TagPages空会直接return none；Catalogue=true必须把这些非错误情形记为itemsKnownEmpty，保留items=[]／next空并继续mode、source_area、完整首遍头／catalogue收集；type零命中、cursor过尾同样如此。只省不再需要的item筛选读取；所有已遇到的真实校验／授权／parent／数据库错误保持原优先次序，后续metadata错误也传播。Catalogue=false的MCP继续原廉价空返回，不为此新增catalogue扫描。

## 3. 文件数据与纯生成（模块分别所有）

```go
// linking root exports bounded existing index primitives without public REST PageSize.
type FileIndexHead struct { ID uuid.UUID; Typed bool; TypeStart string }
type FileIndexFacts struct { Type, Summary *string; Tags []string }
type IndexFileReads interface {
    Heads(ctx context.Context, notebookID uuid.UUID) ([]FileIndexHead, error)
    Items(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]FileIndexFacts, error)
    Inbound(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]int, error) // cap 1000
}
func NewIndexFileReads(pool *pgxpool.Pool) IndexFileReads

// page root; tree names/parents once, then selected pages' ChangedAt via existing PageStates.
type VirtualTreeNode struct { ID uuid.UUID; ParentID *uuid.UUID; Name string; Asset bool }
type VirtualTreeReads interface {
    Nodes(ctx context.Context, notebookID uuid.UUID) ([]VirtualTreeNode, error)
    ChangedAt(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]time.Time, error)
}
func NewVirtualTreeReads(pool *pgxpool.Pool) VirtualTreeReads
// SourceArea.Nodes is existing page-owned complete strict membership, not linking SQL.

type VirtualLogChange struct {
    NodeID uuid.UUID
    Name string
    Path *string // nil after disappearance; retained position
    Kind string // existing ChangeKind
    Asset bool
}
type VirtualLogSource struct {
    NodeID *uuid.UUID // nil after trash/purge, never removed from ordered preview
    Path *string
    Asset bool
}
type VirtualLogChangeset struct {
    ID uuid.UUID
    Kind, Client, ByName string
    CreatedAt time.Time
    Message *string
    Changes []VirtualLogChange // first 3 by existing change order
    NodeCount, ContentCount int
    IngestCount int // S1 zero, S2 actual
    IngestSources []VirtualLogSource // S1 []; S2 first 5 by seq
}
type VirtualLogReads interface {
    Recent(ctx context.Context, notebookID uuid.UUID, limit int) ([]VirtualLogChangeset, error)
}
func NewVirtualLogReads(pool *pgxpool.Pool, names Names) VirtualLogReads
```

Recent内部校验1..1000；调用现History.ListChangesets等数据库读基础，不调用公开Execute(limit1000)。Names用page已有Names窄接口（identity root由bootstrap适配），不让page store跨identity表。路径与kind由page已授权scope内批量读；null位置保留。S1即定义内部Ingest零值字段以供S2扩展，S1不向公开API添加这些字段；同commit由纯renderer消费，不留未用导出。Tree.Nodes用现page ListNodes/路径树基础，SourceArea.Nodes按既有page规则单独一次；不能为file重新实现来源资格或反复逐页读整树。两次大集合的真实成本仍须量。

现行公共 Nodes.ListNodes 仍返回完整 domain.Node；本候选 VirtualTreeReads 的四字段文件DTO与撤销专用 RevertTrees/六字段 RevertTreeNode 分属不同合同。此次修复不要求把 VirtualTreeReads 改接撤销端口；若后续另选投影，须在正式设计与成本审查中说明。IngestNow/Heads/Before 所需 revision、bytes、已删除行与完整资格仍由其窄元数据合同提供，不能从只有活节点结构字段的 RevertState.Tree 补猜。

```go
// wiki-owned mirrors above, never aliases across module app packages.
type FileName string
const (IndexFile FileName = "index.md"; LogFile FileName = "log.md")
type FileProfile uint8
const (ReadFile FileProfile = iota; ExportFile)
type ExportPath struct { ID uuid.UUID; Asset bool; Path string }
type VirtualExportScope struct { Paths []ExportPath } // immutable; this snapshot/call only
type VirtualScope struct {
    NotebookID, SourceRootID uuid.UUID
    Export *VirtualExportScope // required for ExportFile; nil for ReadFile
}
type VirtualSourceNode struct { ID uuid.UUID; Asset bool; Bytes int64 }
type VirtualSourceReads interface {
    Nodes(ctx context.Context, notebookID, rootID uuid.UUID) ([]VirtualSourceNode,error)
}
type VirtualFileDeps struct {
    Index IndexFileReads; Tree VirtualTreeReads; Sources VirtualSourceReads; Log VirtualLogReads
}
func NewVirtualFiles(d VirtualFileDeps) *VirtualFiles
func (v *VirtualFiles) GenerateInSnapshot(ctx context.Context, scope VirtualScope,
    name FileName, profile FileProfile) ([]byte, error)
type FilePath struct { Label string; Destination *string } // nil => folder/missing history non-link
type IndexFileItem struct {
    ID uuid.UUID
    Path FilePath
    ParentPath, Title string // export mapped group key; original readable title
    Type, Summary *string; Tags []string; ChangedAt time.Time; Inbound *int
}
type RenderIndexInput struct { Items []IndexFileItem; Total int; SourcePath FilePath; Sources int64 }
type RenderLogChange struct { Path FilePath; Kind string; Asset bool }
type RenderLogChangeset struct {
    ID uuid.UUID; Kind, Client, ByName string; CreatedAt time.Time; Message *string
    Changes []RenderLogChange; NodeCount, ContentCount, IngestCount int
    IngestSources []FilePath // preserve ordered null positions as non-link labels
}
type RenderLogInput struct { Items []RenderLogChangeset; Total int }
func RenderIndex(in RenderIndexInput, maxBytes int) ([]byte,error)
func RenderLog(in RenderLogInput, maxBytes int) ([]byte,error)
```

bootstrap只转现transfer.Scope.Nodes的ID／Asset／Path，wiki校验并建调用内ID索引，不import transfer app或复刻plan。ExportFile要求完整非nil Export、ReadFile要求nil。每条path唯一、相对合法、folder-only尾斜杠与kind相容；live tree／index／source-root逐ID完整覆盖。重复／缺live／错kind／非法路径均error；历史缺ID保留原null／非链接label。index的Path.Label和ParentPath按映射路径分组排序，Title保持原名；来源行、log changes及ingest sources也按ID映射。folder-only Destination=nil并显示尾斜杠；有文件时Destination是完整映射Path。read用完整page域路径。映射及renderer不再开snapshot，消费结束即释放。

renderer把字面%先编码%25，再编码<>并加angle；只有显示值可中和／截短，绝不改变Destination。非法UTF8／控制字符路径返回无名称内部error。实际字节预算包括编码扩张；整条装不下就停，不截链接。Log.Total为实际recent范围≤50/1000，不另读第limit+1条，footer“Showing N of the most recent M changesets.”中N为实际写出数。Index.Total为完整资格页数；40KiB包含头／来源行／footer。生产profile固定原界，不开放用户maxBytes；纯函数可用小界测试。wiki没有Snapshots/Auth/Runs依赖，read/export入口负责授权scope和ctx。

## 4. Before、旧D/F、coverage与一次最终证据查询（S2）

```go
// page/app; page root aliases only its own app types. linking mirrors these DTOs.
type IngestNodeBefore struct {
    ID uuid.UUID
    Exists bool
    Kind domain.Kind
    InSource, IsSourceRoot, IsRootAgents bool
    PageBytes int64
    PageBytesKnown bool
    Revision int
    RevisionKnown bool
}
type IngestBefore interface {
    NodesBefore(ctx context.Context, ids []uuid.UUID) ([]IngestNodeBefore,error)
}
// Event adds BeforeIngest; StepContent adds BeforeRevision from first locked content.
type IngestHead struct {
    ID uuid.UUID
    Exists, Live bool
    Kind domain.Kind
    HasContent bool
    Revision int
    PageBytes int64
    InSource, IsSourceRoot, IsRootAgents bool
}
type IngestHeadBatch struct { Rows []IngestHead; Last *uuid.UUID; Done bool }
type IngestNow interface {
    Now(ctx context.Context, notebookID, sourceRootID uuid.UUID, ids []uuid.UUID) ([]IngestHead,error)
}
type IngestMetadata interface {
    IngestNow
    Heads(ctx context.Context, notebookID, sourceRootID uuid.UUID,
        after *uuid.UUID, take int) (IngestHeadBatch,error)
}
func NewIngestMetadata(pool *pgxpool.Pool) IngestMetadata
```

每次Now/Before严格有序唯一≤512 IDs，每请求恰一行；不存在显式Exists=false，Before只有可信起点不存在才如此。RevisionKnown／PageBytesKnown=false不能使用其数值证明旧资格，W首锁保存旧revision/bytes，新页不存在为0；SHARE下late target metadata不能冒充Before。首次memo在真正写前，删子树全部保存，逆memo缺祖先/环返回内部错误。Now/Heads只plain SELECT，不补content锁。raw Heads含trash、按rawLast及raw EOF推进；live page缺content、非法祖先/source-root返回错误。take1..512，app用512。

WriterDeps新增独立IngestNow；保留当前RevertTrees、TrashReads.RevertTrees与六字段RevertTreeNode（含stored NameKey/原SortOrder）；nil未读与非nil空树分开，不借它证明revision/bytes/deleted/coverage。NewWriter/NewRevertChangeset签名形式不为此改变，不捏造现有全局nil验证。

```go
// linking/app owns these types; bootstrap maps page Event field by field.
type IngestNotebook struct { Mode string; SourceRootID *uuid.UUID }
type PagesChanged struct {
    WorkspaceID, NotebookID, ChangesetID uuid.UUID
    Changes []domain.Change
    Kind string
    Notebook IngestNotebook
    BeforeIngest IngestBefore
}
type IngestRevision struct { ID uuid.UUID; Revision int }
type IngestIndexedHead struct {
    ID uuid.UUID
    Present bool
    PageRevision, IndexedRevision, Extractor int
    FrontmatterValid bool
}
type IngestIndexHeads interface {
    IndexedHeads(ctx context.Context, notebookID uuid.UUID,
        revisions []IngestRevision) ([]IngestIndexedHead,error)
}
type OldFactState uint8
const (OldFactsUnknown OldFactState = iota; OldFactsKnownAbsent; OldFactsKnownIneligible; OldFactsKnown)
type OldUnknownExtent uint8
const (OldUnknownAll OldUnknownExtent = iota; OldUnknownPossibleTargets)
type OldUncertainty struct { Extent OldUnknownExtent; PossibleTargets []uuid.UUID }
// ReferenceBody is NEW; existing linking/app BodyReference DTO and Bodies signatures stay unchanged.
type ReferenceOrigin uint8
const (ReferenceBody ReferenceOrigin = iota; FirstSourcesSlot)
type QualifiedReference struct { TargetID uuid.UUID; TargetIsAsset bool; Origin ReferenceOrigin; Start int32 }
type OldQualifiedFacts struct {
    PageID uuid.UUID
    State OldFactState // wiki direct role only
    References []QualifiedReference // one page only, released after union
    Unknown OldUncertainty // direct upper bound; NOT yet the inherited closure
}
type OldSourceFacts struct {
    PageID uuid.UUID
    State OldFactState // source-body F role, independently qualified
    AssetIDs []uuid.UUID // sorted distinct; transfer ownership, no raw payload
    Unknown OldUncertainty // finite possible assets or All (assets only)
}
type QualifiedBefore struct { Heads IngestIndexHeads; Reads *LintReads }
// Capture all W D first, then all W source F, before ANY ReplacePage/DeletePages.
// Each callback completes before the next page; callback errors abort the unit.
func (r QualifiedBefore) Capture(ctx context.Context, notebookID uuid.UUID,
    before IngestBefore, writtenOrDeleted []uuid.UUID,
    direct func(OldQualifiedFacts) error, followed func(OldSourceFacts) error) error
```

IndexedHeads每批≤512，携当前/Before metadata的revision，missing显式Present=false、同本连接、Extractor=2且indexed revision相等才current。本P4增加revision配对，P3既有current仅Extractor。Before所有W与resolved target的旧头须在第一次Replace/Delete前取；不事后读更新头证旧target。来源page目标还需live、严格source非root、非空；asset不需page head。Now literal-v1／missing／mismatch目标不成为新候选。旧representation/资格缺证分别传播Unknown；真实SQL/Scan/DTO/ctx错不降格Unknown。

D与F状态分开。source的D=Ineligible不意味F空；F只收真实正文四种link kind、property=NULL、resolved_asset=true及Before同本source asset，排frontmatter和filesystem parent。先完成所有D_W Known/possible贡献，再将F父已属D_W的资产折叠；若F缺证则保资产类不确定性。其余F用每S排序asset-ID切片保留到最终D_U：这是决定71对初稿不存W×关系承诺的明确修订。只retain必要distinct(S,A)和资格／不确定性；无旧正文、JSON、peers、raw links。

最终metadata证明S不可能进入D_U（已删／出来源／空）且D_W known/possible已完整处理后可释放；head缺失不能当资格否定，F父也可能已在旧D_W。system Rewrite的S可能仍live/current，不能按不可变性丢其旧F。Q/S都W且同批删除仍由D_W/F_W完整覆盖；Q不在W时只用已批准的最终tree/解析D_U近似，不偷偷改成Q的Before。

```go
// All following proof/retention structs are private to one PagesChanged observer.
type oldFollowed struct { source uuid.UUID; assets []uuid.UUID; possible bool }
type ingestBounds struct {
    knownDirect, possibleDirect []uuid.UUID // possibleDirect includes known
    allDirect bool
    knownFollowed, possibleFollowed []uuid.UUID // possible includes known
    allInheritedAssets bool
    retained []oldFollowed // no R-sized copy in SQL params
    unknownFParents []uuid.UUID // complete unknown body, assets-only influence
}
type ingestScope struct {
    identity *ingestUnit // private unit identity, never a public ready token
    notebookID, rootID uuid.UUID
    extractor int
}
type CoverageState uint8
const (CoverageIncomplete CoverageState = iota; CoverageUnknown; CoverageComplete)
type CoverageReason uint8
const (CoverageNoGap CoverageReason = iota; MissingIndex; StaleExtractor; RevisionMismatch; WorkBudget)
type coverageResult struct {
    state CoverageState
    reason CoverageReason
    visited int64
    scope ingestScope
    excludedW, eligibleEmitters []uuid.UUID
    currentSources []IngestRevision // includes live W sources, complete Now qualification
    untouchedSources, unknownFParents []uuid.UUID
}
func (c IngestCoverage) check(ctx context.Context, scope ingestScope,
    writtenOrDeleted []uuid.UUID) (coverageResult,error)
```

coverage只在实际新direct candidate经≤512 Now/head点读成立后启动。没有candidate时释放旧Capture并返回，普通非no-op保存零全本Heads/SourceArea/未改heads/NovelSources。完整raw EOF＋全部适用头检查才Complete；eligible wiki缺头/stale使D覆盖Unknown，来源F缺头/stale单独留UnknownFParents。有限不确定关系的全局上界是D_possible∪F_possible(D_possible)，须含所有known贡献；保内部关系式而非物化所有资产允许，但不能将它说成Known。AllAssets只挡asset，不误挡独立page T。无新增低总cap；512不是总保留界。

```go
// One final statement; relation R remains in app. All slices sorted, unique and complete.
type IngestSource struct { ID uuid.UUID; Asset bool; Revision int } // asset Revision=0
type IngestCandidateQuery struct {
    NotebookID uuid.UUID
    Candidates []IngestSource
    EligibleEmitters []uuid.UUID
    CurrentSources []IngestRevision
    UntouchedSources []uuid.UUID
    PossibleOldDirect []uuid.UUID // contains all known D_W
    ParentIDs []uuid.UUID // distinct retained-F + unknown-F parents, not pair arrays
}
type IngestCandidateEvidence struct {
    Candidates []IngestCandidateFact
    Parents []IngestParentFact
}
type IngestCandidateFact struct { ID uuid.UUID; Asset, Direct, FollowedUntouched bool }
type IngestParentFact struct { ID uuid.UUID; Direct bool }
type IngestCandidateReads interface {
    // SQL name IngestNovelSources retained; returns full evidence, not final app novelty.
    NovelSources(ctx context.Context, q IngestCandidateQuery) (IngestCandidateEvidence,error)
}
type IngestMarkResult struct { Added, Count int }
type IngestMarks interface {
    Add(ctx context.Context, notebookID, changesetID uuid.UUID,
        sourceIDs []uuid.UUID) (IngestMarkResult,error)
}
func NewIngestMarks(pool *pgxpool.Pool) IngestMarks // page owns storage; bootstrap mirrors
// linking root extends only its normal production constructor.
type IngestDeps struct {
    Metadata IngestMetadata
    Marks IngestMarks
    MaxContentBytes int
    Logger *slog.Logger
}
func NewIndex(pool *pgxpool.Pool, pages Pages, publisher Publisher, ingest IngestDeps) Index
```

参数在adapter验证长度/有序/唯一/子集/事务proof，CurrentSources在SQL核head资格；Candidate page revision必须对应其中一行。Parent可与candidate同ID，但输出以row_kind+ID区分，每requested candidate/parent恰一行；flag/null/错kind/缺漏/重复/Scan/rows.Err全扫后报错。不能先拿到novel就返回，也不能因结果novel空省掉error。SQL返回候选D_U、F_U∩(D_W_possible∪D_U)，以及完整parent D_U；app合并旧D上下界，线性投影retained并释放，再排折叠资产与unknown闭包。最终只将proven novel直接candidate交Marks；不自动添加跟随asset。

R≤Σdistinct_body_assets≤|W_source|×10000，表示约16R字节asset UUID＋每S元数据，slice capacity/单页sort临时/GC/其他旧新集合另算；最终SQL仅parent IDs，不复制R对UUID。MaxBatchRewriteLinks120000只数被rename/move达到的链接，非旧F上界；400×10000且每页80,005B的4M压力义务及100ops静态资格见正文§3.8。单rename不受400页batch上限，实际更大W依现约束测量，不能假称4M全局最大。新成本未测，原100k/最大合法batch/请求/CI门禁不变。

Normal NewIndex必须有Metadata/Marks和有效MaxContentBytes，Logger可nil只关闭debug；bootstrap.pageRegistrants显式传logger并同步调用点。Kind=edit且LLM且写正文若Before/marks/metadata缺失返回内部error。reindex现linking/admin.go:57–64直接构造app.Index，不走NewIndex；保留独立无ingest依赖、明确非edit且真实非空引用零mark回归。普通writer不能用空Kind逃过真实edit oracle。

复用现`LintReads.SourceValues(notebook,eligible)`、`SourceSlots(notebook,specs)`及`Bodies(notebook,emitters)`。本轮Bodies输入从wiki扩至W的source F，是新增实际读取工作；使用既有`LintBodyReferencesFirst/After`，无逐S独立查询或新命名SQL。512/8MiB/合法单大JSON界不变；SourceSlots全peers含invalid先验证，再exactly-one，完整EOF/Finish后才投影；HasSourceProperties=false仅跳属性，不跳正文。C10双腿在adapter/coverage处检查原57014或ctx cause，错误后marks/publish/commit尝试0，保留TxManager WithoutCancel与rollback失败语义。

## 5. 历史、purge与JSON契约

```go
// existing page types gain fields; true only, false/omitted preserve old query dispatch.
// app.LogQuery/PathLogQuery: Ingest *bool; domain.LogFilter: Ingest bool.
// domain.Changeset: IngestCount int; app.LoggedChangeset: IngestSources []*uuid.UUID.
// detail overrides the preview read to 20, list uses 3; internal log renderer uses 5.
type IngestPreview struct { ChangesetID uuid.UUID; Sources []*uuid.UUID }
func (s *Store) FirstIngestSources(ctx context.Context, changesetIDs []uuid.UUID,
    n int) (map[uuid.UUID][]*uuid.UUID, error) // n only 3,5,20 internal constants
func (s *Store) PurgeChangesetSources(ctx context.Context, before time.Time,
    batch int) (int, error) // one leaf statement; caller loops until short batch
```

零ingest为count0、sources=[]；null元素不能compress。source本身ID在trash/purge或读取时消失后才在response原位置置null，不改持久mark。REST列表/detail仍required count与非nullnullable-ID数组（maxItems3/20）；内部log.md读5，不加MCP cut，沿原40KiB read/1MiB export整条省略与footer。

决定76的MCP输出仅加在共享changesetFields（log.changesets[]和首次changeset.changeset）：

```go
IngestCount      int       `json:"ingest_count"`
IngestSources    []*string `json:"ingest_sources"`
IngestSourcesCut bool      `json:"ingest_sources_cut"` // required, no omitempty
// app.PathLog gains Paths map[uuid.UUID]string; ChangesetPage.Paths is reused.
// MCP logIn gains Ingest *bool `json:"ingest,omitempty"`.
```

三个输出键required且非null；count integer沿0..10000，sources数组item是完整path string或null，零值用非nil []；sources_cut为bool，false也出现。先按seq取P（log最多3/detail最多20）再映射、再仅取连续P[:s]；cut唯一s<len(P)，count超预览cap不是cut。零来源0/[]/false，非空P合法裁零[]/true，数组整体null无效；元素null占位，不跳长path/压缩null/补后项、不另分页。续页changeset:null只代表整个header缺席，三个键随header不出现，外层不补零值。Brief/VersionChangeset不加这些字段。

AtPath/InNotebook在各自原授权/changeset归属校验后批量取来源Paths；InNotebook与changes IDs合并一次读，原位置缺映射转null、真错失败；REST Execute不加路径扫描。log输入ingest optional、type=[null,boolean]，省略/null/false均不筛、仅true筛；account notebook仍required string，notebook scope从entry取得。仅InputSchema按原机制注册，不加OutputSchema框架；输出required由本合同和真实raw-member断言保证。

detail来源阶段以完整header、非nil空changes、n>0时真实Start（n=0原next）固定envelope做Fit，再固定P[:s]做原changes Fit：k=0/n>0保Start，0<k<n用Cursors[k-1]，k=n用NextCursor；续页header=nil跳过来源Fit，原errNoChangeFits保持。log先原完整rows Fit；仅n>0/k=0退让首行，next固定n>1时Cursors[0]、n=1时NextCursor。来源阶段Changes=[]且ChangesCut=(原Changes长度>0)固定；完整P裸header能装必保全P，再固定来源Fit changes；最终ChangesCut=(实留j<原长度)，false沿旧json:"changes_cut,omitempty"，不再无条件true。两个cut独立，来源退让后全部changes装回必须sources_cut=true/changes_cut=false或省略。不跳首行，至少前进一row；sources=[]固定包仍过界则保内部错误，不能删其它字段或抬48KiB。两个builder复制候选不污染原P/长度；此来源阶段固定envelope等价于先保全来源尝试changes Fit、仅build(0)过界才裁来源。独立s*/raw-token及过裁控制见matrix§2.1，不扩原rows/changes算法保证。

以下是待合入现module YAML的schema部分；与原项目OpenAPI3.1/nullable写法一致，UUID/Problem/Limit复用现common。`IndexType`碰撞/超长identity问题见§2，本稿不假定其新字段／界已被批准。

```yaml
# api/modules/linking.yaml, new operation listIndex
/api/v0/notebooks/{notebook_id}/index:
  get:
    operationId: listIndex
    tags: [linking]
    security: [{bearer: []}]
    x-problem-codes: [bad_request, notebook.not_found, page.not_found, validation_failed]
    parameters:
      - $ref: '#/components/parameters/NotebookID'
      - {name: type, in: query, schema: {type: string}}
      - {name: tag, in: query, schema: {type: string}}
      - {name: parent_id, in: query, schema: {type: string, format: uuid}}
      - {name: at_root, in: query, schema: {type: boolean}}
      - $ref: '../common.yaml#/components/parameters/Limit'
      - $ref: '../common.yaml#/components/parameters/Cursor'
    responses:
      '200': {description: Notebook index, content: {application/json: {schema: {$ref: '#/components/schemas/NotebookIndex'}}}}
      default: {$ref: '#/components/responses/Problem'}
      # strict 400/404/422 codes above; no 503; value lengths judged in the existing usecase order
components:
  schemas:
    NotebookIndex:
      type: object
      additionalProperties: false
      required: [data, next_cursor, types, types_complete, unfilterable_type_pages, source_area, mode]
      properties:
        data: {type: array, items: {$ref: '#/components/schemas/NotebookIndexItem'}}
        next_cursor: {type: [string, 'null']}
        types: {type: array, maxItems: 100, items: {type: object, additionalProperties: false, required: [type, count], properties: {type: {type: string}, count: {type: integer, minimum: 0}}}}
        types_complete: {type: boolean}
        unfilterable_type_pages: {type: integer, minimum: 0}
        source_area: {type: [object, 'null'], additionalProperties: false, required: [root_id, sources], properties: {root_id: {type: string, format: uuid}, sources: {type: integer, minimum: 0}}}
        mode: {type: string, enum: [standard, llm_wiki]}
    NotebookIndexItem:
      type: object
      additionalProperties: false
      required: [page_id, parent_id, title, type, summary, summary_cut, tags, updated_at, inbound, inbound_capped]
      properties:
        page_id: {type: string, format: uuid}
        parent_id: {type: [string, 'null'], format: uuid}
        title: {type: string}
        type: {type: [string, 'null']}
        summary: {type: [string, 'null']}
        summary_cut: {type: boolean}
        tags: {type: array, items: {type: string}}
        updated_at: {type: string, format: date-time}
        inbound: {type: integer, minimum: 0, maximum: 1000}
        inbound_capped: {type: boolean}
# api/modules/page.yaml: add to Changeset and separately expanded ChangesetDetail; not Brief/VersionChangeset.
# required += [ingest_count, ingest_sources]
# ingest_count: {type: integer, minimum: 0, maximum: 10000}
# ingest_sources: {type: array, maxItems: 20, items: {type: [string, 'null'], format: uuid}}
# list preview schema maxItems:3, detail:20. listChangesets query ingest:boolean optional.
```

YAML沿固定api的tags/security/x-problem-codes与default Problem；index.list沿现Actions/Access/权限矩阵，不创x-action。S2-1分别更新REST两个required schema、HTTP详情手拷贝、原生生成types及fakes；MCP-only cut不进REST/数据库/虚拟DTO。两scope tools.json手工同步log ingest输入与log/changeset描述，由实际tools/list全对象比较，不靠gen写该文件。来源是最多3/20的非分页预览，cut只因wire；“reads one in full”限定全部changes可分页。虚拟list/read原metadata及strict额外成员/码/说明预算保持。旧asMCP仅log/detail增加required false和独立路径/null期望，page完整golden/相邻字段同S2-1；Brief/VersionChangeset不加，禁止剥键或放宽DeepEqual。

## 6. 真实MCP fixture消费者（C-M1，S1-3）

`e2e/fixtures/mcp.ts:219–221`的PageRead.meta用共享base字段加判别联合：`{virtual:true; version?:never}`或`{virtual?:false; version:string}`。通用读的version可选；virtual=true不许version，kind／窗口／byte_size仍沿read合同。readPage在JSON边界验证形状，不用cast、空串补version绕过合同。真实页版本调用先断言meta.virtual不是true及version为非空令牌，再把已收窄string传base_version；写入回答writtenOf必填version不变。文件表独立列9故事文件／24个相关站点：21个直接.meta.version访问＋3个PageRead.meta解构读取。解构取值与其base_version下游使用分别登记，不把同一变量的后续使用再计为新读取站点；S1-3同步，最迟不拖过S1-4 LW10。不提前P5 prompt helpers。


C-M1第2轮已确认补齐三处解构的设计清单；实施时都先保留PageRead结果，断言/收窄`meta.virtual !== true`及`typeof meta.version === "string"`且非空，再取version传base_version；既有runtime expect与业务断言保留，不能用cast或非空断言跳过验证。

| 固定6992取值站点 | 下游base_version使用 | 已有runtime version断言 | 提交 |
|---|---|---|---|
| e2e/stories/asset/as2-paste.spec.ts:113 | 138 | 无，按上面规则补真实页前置条件 | S1-3 |
| e2e/stories/page/pg12-page-permissions.spec.ts:153 | 156 | 154，保留并另做类型收窄 | S1-3 |
| e2e/stories/page/pg8-save-conflict.spec.ts:101 | 108、109 | 102，保留并另做类型收窄 | S1-3 |

`McpClient.call`的args是Record<string,unknown>（mcp.ts:45），可选version不一定触发tsc，因此这是消费者合同覆盖修订，不是已观察的编译失败。collab/c4-guard.spec.ts:97与mcp/mc8-history.spec.ts:152为各自JSON.parse cast，不消费PageRead类型，不列入24站点或本次修改范围；writtenOf仍必填version，P5不提前。
