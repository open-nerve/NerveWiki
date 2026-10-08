# M7/P4 附件（前端）：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M7/P4 附件（前端） |
| 状态 | 进行中 |
| 基线 | `31f894a`（P3 合并、文档补完之后的 main）；本文提交之后开分支 `m7-p4a`，A 合并之后开 `m7-p4b`，B 合并之后开 `m7-p4c` |
| 上级文档 | [M7 总设计](00-M7-design.md) 4.7、4.8、第 5、7–9 节；[P3 文档](03-P3-assets-links.md)第 9.2 节（交给 P4 的三项）；移交：[M4 附件的扩展](handoffs/M4-extensions.md)第 2、3 项（编辑器的一跳）；总体设计 13.2 第 1、6、23、25、26 条 |

---

## 0. 分三部分

P4 把附件交到读者与写者手里：面板与上传、阅读视图里的附件、编辑器里的粘贴与拖入。三部分各自独立可用，照 P3 的先例分开实现、审查、核对修复、合并：

- **A：面板与上传**：上传的服务（经会话的客户端，带进度、可取消）；附件的 store；页面中栏与笔记本首页的附件一节（列出、上传、打开、下载、复制嵌入、改名、移动、删除）；发送之前先查；文档的拖放保护，拖到阅读视图上上传到这一页；删除页面的确认数上附件。e2e：AS1 的页面版本。
- **B：阅读视图里的附件**：阅读视图的增强 `assets`（附件的链接：内联类型在新标签页打开、其余下载；加载失败时的重读）；视图与右栏属性的到期重读；保留正在播放的音视频；服务端标出不内联的附件（`download`）、属性答出到期。e2e：AS3。
- **C：编辑器的粘贴与拖入**：编辑器的扩展 `assetUpload`（粘贴、拖入的文件上传，完成后插入 `![[link]]`）；`EditorContext.uploadAsset`、`EditorControls.whenComposed`；编辑栏显示进度与取消；输入法清单加一步。e2e：AS2。

第 1、2 节三部分共用，第 3–5 节是 A、B、C 的设计，第 6–9 节共用。

## 1. 基线

- **契约**：`uploadAsset`（`x-raw`，multipart，字段依次 `parent_id`、`name`、`file`）、`listAssets`（按父节点，名称的键与 id 排序，游标分页，一页至多 100）、`getAsset`；`Asset` 有 `name`、`link`（没有扩展名时为 null）、`mime`、`byte_size`、`width`、`height`、`content_url`、`download_url`、`expires_at`。改名、移动、删除附件走节点的操作（`renameNode`、`moveNode`、`deleteNode`）。`InstanceInfo.asset_max_bytes`。生成的 TS 客户端有这些操作的类型；`e2e/fixtures/assets.ts` 经 `bodySerializer` 交 `FormData` 上传。
- **阅读视图的 HTML**（P3B）：附件的标记 `nw-asset`，地址是签名的内容地址，路径里有附件的 id（`/api/v0/assets/<id>/content?…`）；`<a class="nw-asset" href data-nw-size>`、`<img>`、`<audio>`、`<video>`；没有地址的是文字。`PageView.assets_expire_at`（写出的地址里最早的到期），`PropertyLink.url`（附件的签名地址，右栏现在一律在新标签页打开）。内容地址对图片、音视频与 PDF 内联，其余按附件下载（`domain.Inline`）。
- **前端**：`PageTreeStore.nodes` 是全部节点（页面与附件），`tree` 只取页面，`siblingsOf` 给全部兄弟（命名空间共用）；`pages` 事件带 `tree` 时经 `refresher` 至多每 500 毫秒重读一次树。阅读视图的增强有 `appLinks`（不碰附件的链接）等七个；编辑器的扩展有失锁只读、自动保存、闲置退出、补全。`SourceEditorHandle.whenComposed(act, drop)` 已有，`EditorControls` 没有。`i18n/format.ts` 有 `formatBytes`。

## 2. 目标与范围

**做**（总设计第 7 节 P4 一行；4.8）：第 0 节的三部分；13.2 第 1 条（上传不经 `oneAtATime`）、第 6 条（上传经会话的客户端）、第 23 条（`uploadAsset`、`whenComposed`、`assets` 增强）、第 25 条（面板与媒体的上限）随实现改写。

**不做**：

- 导入导出的界面（P5、P6）。
- 点图片放大、图片的灯箱、音视频的播放列表（总设计 4.8：点图片不做什么，与 Obsidian 的默认相同）。
- 附件的预览（面板里只有类型的图标，不加载缩略图：一页至多 100 项，每项一个签名地址的图片会让面板打开就下载一百个文件）。

## 3. A：面板与上传

### 3.1 文件

| 文件 | 内容 |
|---|---|
| `web/apps/web/src/services/asset.service.ts`（新）、`services/upload-fetch.ts`（新） | `AssetService`：列出、读一个、上传；上传经 `XMLHttpRequest` 实现的 `fetch`（3.2） |
| `web/apps/web/src/stores/asset.store.ts`（新）、`stores/root.store.ts`、`stores/context.tsx` | `AssetStore`：一个笔记本每代一个；在途的上传、名称、上传之后的重读、换代时中止（3.3） |
| `web/apps/web/src/lib/upload-name.ts`（新） | 上传的名称：照导入的规则修正，在兄弟与在途的上传之间取空着的（3.4） |
| `web/apps/web/src/pages/page/attachments-section.tsx`（新）及其对话框（改名、移动） | 附件一节（3.5） |
| `web/apps/web/src/pages/page/page-layout.tsx`、`pages/notebook/notebook-home.tsx` | 页面的中栏、笔记本首页放附件一节 |
| `web/apps/web/src/pages/notebook/page-tree-item.tsx` | 删除页面的确认数上子树里的附件（3.7） |
| `web/apps/web/src/app/file-drop.tsx`（新）、`pages/page/reading-view.tsx` | 文档的拖放保护；拖到阅读视图上上传到这一页（3.6） |
| `web/apps/web/src/events/handlers.ts` | `pages` 带 `tree` 时一并重读这本笔记本挂着的附件列表 |
| `web/apps/web/src/i18n/messages/en.ts`、`zh-CN.ts` | 文案 |
| `web/apps/web/src/test/page-server.ts` | 假服务端加附件的操作（照真实服务端的规则：名称、`.md`、大小、角色、`title_taken`） |
| `e2e/stories/asset/as1-upload.spec.ts`、`e2e/fixtures/` | AS1 的页面版本 |

### 3.2 上传的服务

- `AssetService(api)`：`list(notebookId, parent, cursor)`、`get(id)`、`upload(notebookId, { parent, name, file }, { progress, signal })`。只有 `services/` 导入客户端（13.2 第 6 条）。
- **经会话的客户端**：上传是 `api.POST("/api/v0/notebooks/{notebook_id}/assets", { bodySerializer, fetch })`，openapi-fetch 按请求换 `fetch`：`uploadFetch(form, progress, signal)` 以 `XMLHttpRequest` 实现（`fetch` 没有上传进度），发闭包里的 `FormData`，用中间件交来的 `Request` 的头（令牌），**不用** 它的 `Content-Type`（`Request` 序列化同一个表单时用的是另一个边界，由 XHR 自己写）。中间件 401 之后续期重发（`options.fetch(copy)`）时照样带新令牌重传整个表单；换代之后中间件在发送之前就以 `SessionChangedError` 拒绝（13.2 第 1 条），不另做核对。
- 答复照 `fetch` 的样子交回（`new Response(body, { status, headers })`），`unwrap` 照常；`signal` 中止时 `xhr.abort()`，以 `AbortError` 拒绝；网络错误以 `TypeError` 拒绝，同 `fetch`。进度按 XHR 上传的 `progress` 事件（已发与总字节）。
- 字段依次 `parent_id`（根下不加）、`name`、`file`（契约的次序）。
- vitest 注入假的 XHR（`test/` 里的可控传输：逐步的进度、答复、错误、中止），测中间件的续期重发、换代、中止。

### 3.3 附件的 store

- `AssetStore`：一个笔记本每代一个（13.2 第 15 条，`RootStore` 按笔记本 id 缓存，换代时随之丢弃并中止在途的上传）。
- **列出**：SWR 键 `["assets", 笔记本 id, 父节点 id 或 "root"]` 读第一页，"加载更多"接着读下一页（游标），页数由组件的状态记，重读时回到第一页；读不到经 `NotLoaded`（13.2 第 7 条）。`pages` 事件带 `tree` 时，与树一起（经 `refresher`、同样的间隔）重读这本笔记本挂着的附件列表；连上时的整体刷新同样（`refreshedOnConnect`）。
- **上传**：`upload(parent, files)` 逐个文件开始（并行，不经 `PageTreeStore` 的 `oneAtATime` 队列：50 MB 的上传会挡住每个树的写，13.2 第 1 条的例外）。每个上传是可观察的一项：名称、父节点、已发与总字节、状态（发送中、失败及原因）、`cancel()`。答复之后（拒绝也算）重读树与这个父节点的列表；在途的上传占着名字（3.4）。`409 page.title_taken`（别的标签页、或客户端与服务端比法不同的名字）换下一个空着的名字再传，至多三个名字（同新建页面）。
- **关闭页面的提醒**：有在途的上传时挂上 `beforeunload`（13.2 第 21 条），没有了就摘掉。
- **改名、移动、删除**：经 `PageTreeStore` 的 `rename`、`move`、`remove`（同一个 `oneAtATime` 队列：它们改兄弟的名字与位置，与页面的写同一个次序），答复之后照样重读树；附件一节另重读它的列表。删除答 `page.not_found` 当作已完成（13.2 第 1 条）。

### 3.4 名称

- **发送之前先查**（总设计 4.8）：角色（只给能写页面的人上传按钮与拖放区）、大小（`InstanceInfo.asset_max_bytes`，超过的不发，说明上限）、不是 `.md`（说明用导入）。
- **修正**（照导入的规则，总设计 4.11）：NFC；`/ \ : * ? " < > | # ^ [ ]` 与控制字符换成 `_`；去掉首尾的空白与 `.`；Windows 保留的名字（`CON`、`com1.txt` 等）在主名后加 `_`；超过 255 字节的在字符边界截短、保留扩展名；空的叫"未命名"（随界面语言：这是用户给的名字的代替）。
- **空着的名字**：在父节点的全部兄弟（页面与附件，`siblingsOf`）与在途的上传之间按标题键（NFC、小写）比较，撞上的在扩展名之前加序号：`a.png`、`a 2.png`、`a 3.png`（与导入相同）。
- 修正与取名是纯函数（`lib/upload-name.ts`），表格测试；服务端的拒绝（422 的字段、`page.title_taken` 三次之后）照常显示。

### 3.5 附件一节

- **位置**：页面的中栏、子页面列表之下（总体设计 9.2）；笔记本首页列根下的附件。没有附件时，读者看不到这一节，写者看到它的标题、上传按钮与"拖到这里上传"的说明。
- **列表**：名称、大小（`formatBytes`，界面语言）、类型的图标（图片、音频、视频、PDF、其余）；一页 100 项，"加载更多"。
- **每一项的菜单**（写者全部、读者前三项）：打开（内联类型在新标签页，带看不见的"在新标签页打开"；其余是下载地址）、下载（`download_url`）、复制嵌入（`![[link]]`，`link` 为 null 时不给）、改名、移动到…、删除。名称本身是"打开"的链接。
- **签名地址的到期**：列表里每一项的地址一到两小时有效；一节在最早的 `expires_at` 之前重读它挂着的页（不在点击之后再去签：新标签页要在用户的点击里打开，等一次请求之后浏览器会当作弹窗拦下）。
- **改名**：对话框只改主名，扩展名照旧显示在输入框之后、不可改（附件改名不能去掉扩展名，总设计 4.2）；没有扩展名的附件改整个名字。拒绝（422、409、引用它的页正在编辑时的 `linking.pages_locked`）留在对话框里，同改名页面。
- **移动到…**：上级只列页面与根（可以放附件的地方），放在末尾；拒绝同移动页面。复用 `MovePageDialog` 的上级选择，不给位置。
- **删除**：确认对话框；删除之后焦点到这一节的标题（`focusAfter`），读者其间动过就不移（13.2 第 26 条）。
- **上传**：上传按钮（`<input type="file" multiple>`）与拖到这一节上传；在途的上传列在这一节的顶上，带进度条（`progress`，有可读的百分比）与取消；失败的显示原因、可以关掉。
- **行可以拖进编辑器**：`dataTransfer` 的 `text/plain` 是 `![[link]]`（`link` 为 null 的不可拖），CodeMirror 自己的拖放就插在落点。

### 3.6 拖放的保护

- 文件拖到拖放区之外时浏览器会打开它、离开应用：文档上对带文件的 `dragover`、`drop` 一律 `preventDefault`，`dropEffect` 为 `none`（`app/file-drop.tsx`，在应用的外壳里挂一次）。拖放区（附件一节、阅读视图、C 部分的编辑器）自己接住，`dropEffect` 为 `copy`。
- 拖到阅读视图上（写者，不在编辑时）上传到这一页，同附件一节的上传；文件夹不接（`webkitGetAsEntry` 是目录的），说明用导入。

### 3.7 删除页面的确认

- 确认对话框数上子树里的附件（`PageTreeStore.nodes` 里父节点在子树里的附件）："它下面的 3 个页面与 5 个附件会一起删除"。

## 4. B：阅读视图里的附件

### 4.1 文件

| 文件 | 内容 |
|---|---|
| `server/internal/platform/markdown/obsidian/`：`asset.go`、`obsidian.go` | `Asset.Inline`；不内联的附件的链接写 `download`（4.2） |
| `server/internal/modules/asset/app/embeds.go`、`bootstrap/assets.go` | 答出是否内联与到期 |
| `server/internal/modules/linking/app/`：`ports.go`、`properties.go`；`adapter/http` | `PropertyLink.inline`、`PageProperties.assets_expire_at`（4.3） |
| `api/modules/linking.yaml` | 同上 |
| `web/apps/web/src/reading/assets.ts`（新）、`reading/enhancement.ts` | 增强 `assets`（4.4–4.6） |
| `web/apps/web/src/pages/page/page-view.ts`、`reading-view.tsx`、`page-properties.tsx` | 到期重读；右栏的附件链接 |
| `tools/md-fixtures/`、`server/internal/platform/markdown/obsidian/*_test.go`、`bootstrap/assets_view_test.go` | 标记与最后一跳随之改 |
| `e2e/stories/asset/as3-render.spec.ts`（新） | AS3 |

### 4.2 不内联的附件的链接

- 内容地址对图片、音视频与 PDF 内联，其余按附件下载。前端要分开它们：内联的在新标签页打开，下载的不开新标签页（Firefox 会留下空白的标签页）。HTML 里没有类型（P3B 去掉了 `data-nw-asset`，也不写 MIME），所以服务端给不内联的附件的链接写 `download` 属性（同源的地址，浏览器照它下载，不跳转；没有增强时也对）。
- `obsidian.Asset` 加 `Inline bool`（组合根按 asset 的 `domain.Inline(mime, false)` 给出）；`Markup` 的 `a` 加 `download`。每个链接多 9 个字节，在 `MaxShown` 的总量之内（`TestTheAppsAttachmentsAreWithinAQuarterOfHeadroom` 照旧核对）。

### 4.3 属性的内联与到期

- `PropertyLink` 加 `inline`（必有，可空：附件以外为 null）；`PageProperties` 加 `assets_expire_at`（必有，可空：没有附件的地址时为 null）。linking 的 `AttachmentURLs` 答出每个地址、是否内联与到期。
- 右栏：内联的在新标签页打开、带看不见的提示，其余照 `download` 下载（审查 C12）；`assets_expire_at` 之前重读属性（审查 C13，同 4.5）。

### 4.4 增强 `assets`：附件的链接

- `a.nw-asset[href]`：没有 `download` 的加 `target="_blank"`、`rel="noopener noreferrer"` 与看不见的"（在新标签页打开）"（文字随界面语言，增强的 `t`）；有 `download` 的不动。不改 HTML 的结构之外的东西：提示是加进链接里的一个 `span`，增强清理时移除（13.2 第 23 条）。
- 点图片不做什么。

### 4.5 到期与加载失败

- **到期重读**：`usePageView` 的视图过了 `assets_expire_at` 当作没有加载（不显示缓存里已过期的 HTML，重读）；还没过的，在它之前一分钟安排一次重读（`mutate`）。没有附件的地址（`null`）的视图不安排。属性同样（4.3）。
- **加载失败**：增强在容器上以捕获阶段听 `error`（它不冒泡）：图片、音视频加载失败，而视图的到期时刻已过时，经 `context.reload()` 合并成一次重读；同一个页面、同一个到期时刻至多重读一次（增强的闭包按页面记下已为哪个到期时刻重读过）：文件不在（404）、限流（429）、解不开的图片，在同一个小时里重读也一样失败，不能循环。

### 4.6 保留正在播放的音视频

- HTML 每小时因签名而变，别人的编辑、`links` 事件也让它变，`innerHTML` 会毁掉正在播放的媒体。增强清理时，把正在播放、或已开始又暂停在中间的 `audio.nw-asset`、`video.nw-asset` 从旧的 HTML 里取下来，按附件 id（地址的路径里）与它在这个 id 里的次序记下；下一次运行时，新 HTML 里同一个 id、同一个次序的元素换成取下来的那个（保留播放的位置与状态），对不上的丢掉。上限：至多保留 20 个（`MaxMedia`）。
- 取下来的元素的地址仍是旧的签名：暂停超过它的期限之后，下一次按 `Range` 读失败（`error`）时，经 `ReadingContext.assetAddress(id)`（`GET /assets/{id}`）重签，换上新地址，回到原来的位置；附件已删除时（404）不再重试。
- `ReadingContext` 加 `assetAddress(id: string): Promise<string>`（只读，任何角色都有）。

## 5. C：编辑器的粘贴与拖入

### 5.1 文件

| 文件 | 内容 |
|---|---|
| `web/apps/web/src/editor/registry.ts`、`editor/source-editor.tsx` | `EditorContext.uploadAsset`、`EditorControls.whenComposed`；注册 `assetUpload` |
| `web/apps/web/src/editor/loaded/asset-upload.ts`（新） | 扩展：接住粘贴、拖入的文件，上传，插入（5.2） |
| `web/apps/web/src/pages/page/page-edit.tsx`、`page-editing-bar.tsx` | 上下文的 `uploadAsset`；编辑栏的进度与取消（5.3） |
| `docs/v0.1/M4-pages/manual/P6-ime-checklist.md` | 输入法清单加一步（5.4） |
| `e2e/stories/asset/as2-paste.spec.ts`（新） | AS2 |

### 5.2 扩展 `assetUpload`

- `load` 的扩展，模块在 `editor/loaded/`（13.2 第 23 条）。以高优先级的 `domEventHandlers` 先于 CodeMirror 自己的拖放接住带文件的拖入（它会把文件当文本读进来）与粘贴。
- **粘贴**：剪贴板里只有文件、没有 `text/plain` 时才上传（Excel、Word、Numbers 复制的单元格同时有文字与图片：粘贴文字）；插在选区（替换选中的）。剪贴板里没有文件名的图片取名为 `Pasted image 20261008123045.png`（照 Obsidian：本地时间到秒，扩展名按类型；不随界面语言，它是存下来的内容）。
- **拖入**：插在落点（`posAtCoords`）；拖入的 `.md` 交给 CodeMirror（插入它的文字）；文件夹不接，提示用导入。
- **插入**：上传完成后在原位置插入 `![[link]]`：位置随之后的输入映射（`StateField` 里的位置经每次改动的 `mapPos`）；输入法组合中经 `controls.whenComposed` 等组合结束。编辑器已换了正文（`onClose`）、已关闭或已只读（失锁）的，不插入，提示一句，附件留在面板里；`link` 为 null（没有扩展名）的不插入，提示一句；上传失败时提示，不插入任何东西。几个文件一起时依次插入，各占一行。
- 上传放在页面下（`parent` 是这一页），名称照 3.4。

### 5.3 上下文与编辑栏

- `EditorContext.uploadAsset(file, name)`：经 `AssetStore` 上传到这一页，答出 `Asset`；进度与取消在 store 里，编辑栏（`page-editing-bar.tsx`）显示这一页的在途上传（名称、百分比、取消），文档里不加任何东西。
- `EditorControls.whenComposed(act, drop)`：同 `SourceEditorHandle` 的，编辑器先关闭时调用 `drop`。

### 5.4 输入法清单

- 加一步（总设计第 9 节）："组合输入时上传完成：嵌入在 `compositionend` 之后插在映射后的位置，组合出的文字完好"。M4–M6 的清单等负责人执行，M7 的这一步随同一份清单。

## 6. 实施步骤

**A**（分支 `m7-p4a`）：

1. A1 上传的服务与假的传输；`AssetStore`；名称的纯函数。
2. A2 附件一节（页面、首页）、对话框、拖放保护与阅读视图的拖放、删除的计数、事件的重读；假服务端的附件操作。
3. A3 e2e AS1 的页面版本。
4. 反向对照、审查、修复核对、合并、main 上的文档。

**B**（分支 `m7-p4b`，A 合并之后）：

1. B1 服务端：`download`、`PropertyLink.inline`、`PageProperties.assets_expire_at`；样例与最后一跳随之改。
2. B2 增强 `assets`：链接、加载失败、保留媒体与重签；`usePageView` 与属性的到期；右栏的链接。
3. B3 e2e AS3。
4. 反向对照、审查、修复核对、合并、main 上的文档。

**C**（分支 `m7-p4c`，B 合并之后）：

1. C1 `EditorContext.uploadAsset`、`EditorControls.whenComposed`；扩展 `assetUpload`；编辑栏。
2. C2 e2e AS2；输入法清单。
3. 反向对照、审查、修复核对、合并、main 上的文档。

## 7. 测试与验证

- 每部分合并之前：`make lint-web knip test-web build-web`、`golangci-lint`（B）、`make test-go`、`make gen-check`、`make e2e`（本机的 Docker 起不来时以分支的 CI 为准，写进结果）。
- **vitest**：上传的服务（进度、中止、401 之后带新令牌重传整个表单、换代、网络错误、答复的头）；名称的表格（修正、Windows 保留名、截短、序号、在途的占名）；store（并行的上传、答复之后的重读、`title_taken` 换名、换代中止、`beforeunload`）；附件一节（列出、加载更多、菜单、读者与写者、上传与进度、取消、失败、改名与移动的拒绝、删除之后的焦点、读者动过不移）；首页；拖放保护；删除页面的计数；增强（链接的属性与提示、清理复原、到期重读、加载失败只重读一次、保留媒体与重签）；扩展（粘贴与拖入的判定、名字、位置映射、组合中等待、已关闭或只读不插入、`link` 为 null）。经组合根到达：增强经 `Enhancements`、扩展经 `EditorExtensions`、面板经页面（13.1 第 21 条的前端一跳：组合根交空时失败）。
- **e2e**：AS1 的页面版本（面板上传与打开、根下的附件在首页、改名、删除）；AS2（编辑器里粘贴、拖入图片，插入嵌入并显示；接口版本：上传，再写带 `![[link]]` 的正文，同一组数据库断言）；AS3（嵌入的图片、音频、视频——WebM、Ogg，Playwright 的 Chromium 没有 H.264、AAC——与附件链接的显示，内联的在新标签页、其余下载；附件改名之后嵌入改写；未解析的嵌入；`expectIndexedLinks` 带附件的 id）。
- **反向对照**（`mut.py`，前端的变异经 `../web/...` 的路径）：中间件的令牌不交给 XHR、重传不带表单、进度不报、中止不拒绝；名称的每条规则；占名不算在途的；上传经了 `oneAtATime`；答复之后不重读；删除不移焦点；拖放保护不 `preventDefault`；增强不加 `target`、给 `download` 的也加、清理不复原、失败重读两次、到期不重读、媒体对错了次序；扩展接住了有文字的粘贴、插在旧位置、组合中就插、只读时也插。
- 审查：每部分三位 Opus 审查者并行（正确性与状态、可访问性与交互、测试与文档），然后 Opus 修复核对，直到一轮没有行为问题。

## 8. 完成标准

1. 第 3–5 节每一条有 vitest，经组合根到达；e2e AS1（页面）、AS2、AS3 通过。
2. 上传经会话的客户端，有进度、可取消，401 续期之后重传；换代中止；不经树的写队列。
3. 读者看得到、打不开写的操作；写者的上传、改名、移动、删除都在答复之后重读树与列表。
4. 阅读视图与右栏里，内联的附件在新标签页打开、其余下载；签名地址到期之前重读，加载失败至多重读一次；正在播放的媒体在 HTML 换掉时保留。
5. 编辑器里粘贴、拖入的文件上传后插在映射后的位置，组合中等待，关闭或只读时不插入。
6. 总体设计 13.2 第 1、6、23、25 条随实现改写；移交 M4-extensions 第 2、3 项写明落实。

## 9. 结果

（各部分合并之后填写。）
