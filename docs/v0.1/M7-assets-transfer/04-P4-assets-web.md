# M7/P4 附件（前端）：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M7/P4 附件（前端） |
| 状态 | 进行中（A 已合并 `e44b417`，B 已合并 `32e175c`） |
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

实现、审查与修复核对之后照实际改写（第 9.1 节）。

### 3.1 文件

| 文件 | 内容 |
|---|---|
| `web/apps/web/src/services/asset.service.ts`（新）、`services/upload-fetch.ts`（新） | `AssetService`：列出（一页 100）、读一个、上传；上传经 `XMLHttpRequest` 实现的 `fetch`（3.2） |
| `web/apps/web/src/stores/asset.store.ts`（新）、`stores/root.store.ts`、`stores/context.tsx`（`useAssets`） | `AssetStore`：一个笔记本每代一个；列表的读、在途的上传、名称、上传之后的重读、换代时中止（3.3） |
| `web/apps/web/src/stores/page-tree.store.ts` | `wrote()`：上传答复之后的树读；被答复重叠的读交回最近发出的那次（3.3） |
| `web/apps/web/src/lib/upload-name.ts`（新）、`lib/title-key.ts`（新） | 上传的名称：照导入的规则修正，在兄弟与在途的上传之间取空着的；标题键，树与上传共用（3.4） |
| `web/apps/web/src/lib/unload-warning.ts`（新）、`lib/asset-kind.ts`（新） | 关闭页面的提醒（每个笔记本一项）；附件的类型、是否内联、嵌入的写法 |
| `web/apps/web/src/pages/page/attachments-section.tsx`（新）、`attachment-row.tsx`（新）、`upload-rows.tsx`（新）、`asset-dialogs.tsx`（新） | 附件一节、它的行与菜单、在途的上传、改名与移动的对话框（3.5）；阅读视图的拖放区 `AttachmentDrop`（3.6） |
| `web/apps/web/src/pages/notebook/parent-options.tsx`（新） | 上级的选项，移动页面与移动附件共用 |
| `web/apps/web/src/pages/page/page-layout.tsx`、`pages/notebook/notebook-home.tsx` | 页面的中栏、笔记本首页放附件一节；阅读视图包在 `AttachmentDrop` 里；页面已不在树里、只因未保存的编辑仍显示时（`gone`）两者都不放 |
| `web/apps/web/src/pages/notebook/page-tree-item.tsx` | 删除页面的确认数上子树里的附件（3.7） |
| `web/apps/web/src/app/file-drop.tsx`（新）、`app/layout.tsx` | 文档的拖放保护（外壳里挂一次）；拖放区的 `useFileDrop`（3.6） |
| `web/apps/web/src/components/form-field.tsx`、`app/rename-form.tsx` | 输入框之后的后缀（扩展名），后缀的说明与错误并存 |
| `web/apps/web/src/events/handlers.ts`、`app/event-stream.tsx` | `pages` 带 `tree` 时，树读完、显示之后再重读这本笔记本挂着的附件列表；连上时附件与阅读视图、右栏同在最后一层 |
| `web/apps/web/src/i18n/messages/en.ts`、`zh-CN.ts` | 文案 |
| `web/apps/web/src/test/transfer.ts`（新）、`test/attachments.ts`（新）、`test/page-server.ts`、`test/fakes.ts` | 假的传输（逐步的进度、答复、错误、中止）；附件一节的测试工具；假服务端的附件操作（照真实服务端的规则：每页的 `limit`、按标题键与 id 排序、`link`、名称、`.md`、大小、角色、父节点、`title_taken`，中止的上传不建） |
| `e2e/stories/asset/as1-upload.spec.ts`、`e2e/stories/collab/c4-guard.spec.ts` | AS1 的页面版本；C4 删除笔记本时附件列表的 404 |

### 3.2 上传的服务

- `AssetService(api, transfer)`：`list(notebookId, parent, cursor)`（`limit` 100）、`get(id)`、`upload(notebookId, { parent, name, file }, { progress, signal })`。只有 `services/` 导入客户端（13.2 第 6 条）。
- **经会话的客户端**：上传是 `api.POST("/api/v0/notebooks/{notebook_id}/assets", { bodySerializer: () => undefined, fetch })`，openapi-fetch 按请求换 `fetch`：`uploadFetch(form, options, transfer)` 以 `XMLHttpRequest` 实现（`fetch` 没有上传进度）。交给中间件的请求不带正文，传输发闭包里的 `FormData`，`Content-Type` 由 XHR 自己写（带它的边界）；请求的其余头（令牌）照抄。中间件 401 之后续期重发（`options.fetch(copy)`）时照样带新令牌重传整个表单；换代之后中间件在发送之前就以 `SessionChangedError` 拒绝（13.2 第 1 条）。
- 传输由 `AppStores` 的第四个参数注入（`() => Transfer`，默认是 `XMLHttpRequest`），测试交 `test/transfer.ts` 的假传输。
- 答复照 `fetch` 的样子交回（`new Response(body, { status, headers })`，204、205、304 没有正文），`unwrap` 照常；`Response` 不收的答复（状态在 200–599 之外）以 `TypeError` 拒绝，不会一直挂着。`signal` 中止时 `xhr.abort()`，以 `AbortError` 拒绝，答复之后的中止不再起作用；网络错误、超时以 `TypeError` 拒绝，同 `fetch`。进度按 XHR 上传的 `progress` 事件（已发与总字节）。
- 字段依次 `parent_id`（根下不加）、`name`、`file`（契约的次序）。
- **已知的限制**：服务端在读完正文之前就答复（例如 413、409）并关闭连接时，浏览器可能报网络错误，上传以 `TypeError` 失败，显示"上传失败"，不换名重试（`TypeError` 时重试可能重复上传）。

### 3.3 附件的 store

- `AssetStore`：一个笔记本每代一个（13.2 第 15 条，`RootStore.assetsOf` 按笔记本 id 缓存）。这一代结束（标签页的会话离开这个登录）时中止在途的上传；`RootStore` 在第一次 `assetsOf` 时才订阅令牌，订阅时会话已换就立即中止。
- **列出**：SWR 键 `["assets", 笔记本 id, 父节点 id 或 "root"]`，fetcher 是 `load(parent)`：读它已有的页数（第一次一页），一页 100 项；"加载更多"是 `more(parent)`，读下一页、答出它加进来的项。同一父节点的读一个接一个（`oneAtATimeById`），排着的 `load` 被再次要求时就是那一次；接页时按 id 去重，以后读到的为准。读不到经 `NotLoaded`（13.2 第 7 条）。`pages` 事件带 `tree` 时，树经 `refresher` 读完、React 显示之后（`shown()`，已不在的页随之卸载、不读它的列表），再重读这本笔记本挂着的附件列表；连上时的整体刷新里，附件与阅读视图、右栏同在最后一层。
- **上传**：`upload(parent, files)` 逐个文件开始（并行，不经 `PageTreeStore` 的 `oneAtATime` 队列：50 MB 的上传会挡住树的每个写，13.2 第 1 条的例外）。每个上传是可观察的一项：名称、父节点、已发与总字节、是否已答复、失败的原因、上传成的附件、`cancel()`。
  - 在途的上传占着名字，只在同一父节点下；失败、被拒的不占。
  - 答复之后（拒绝也算）重读树（`PageTreeStore.wrote()`）与这个父节点的列表。`wrote()` 同时只有一次读在途，其间答复的合成下一次；每个上传在它答复之后开始的第一次树读完成时结算。树的读出去之后有答复的会被丢弃，被丢弃的读交回最近发出的那次读（那次也失败时再读）：写的读不会因为上传的答复交回旧树。
  - 成功的上传留在列表里直到列表有了它的附件：附件排在已读的页之后的，接着读后面的页，直到读到它（之后重读也读这么多页）；整个列表读完都没有它的，是其间被删除或移走了，不显示。
  - `409 page.title_taken`（别的标签页、或客户端与服务端比法不同的名字）换下一个空着的名字再传，至多三个名字（同新建页面）。
  - 取消：中止请求、立即移出；请求可能已到达，照样重读。失败之后的重读期间取消的，同样移出、不显示失败。
- **关闭页面的提醒**：有在途的上传时挂上 `beforeunload`（13.2 第 21 条），没有了就摘掉（`lib/unload-warning.ts`，各笔记本各算）。
- **改名、移动、删除**：经 `PageTreeStore` 的 `rename`、`move`、`remove`（同一个 `oneAtATime` 队列：它们改兄弟的名字与位置，与页面的写同一个次序），答复之后照样重读树，再重读改到的列表（拒绝也重读）。删除答 `page.not_found` 当作已完成（13.2 第 1 条）。

### 3.4 名称

- **发送之前先查**（总设计 4.8）：角色（只给能写页面的人上传按钮与拖放区）、大小（`InstanceInfo.asset_max_bytes`，超过的不发，说明上限）、不是 `.md`（说明用导入）。
- **修正**（照导入的规则，总设计 4.11）：NFC；`/ \ : * ? " < > | # ^ [ ]`、控制字符、行与段的分隔符、双向控制符换成 `_`；去掉首尾的空白与 `.`；Windows 保留的名字（`CON`、`com1.txt` 等）在主名后加 `_`；超过 255 字节的在字符边界截短、保留扩展名（扩展名本身太长时也截，留下主名的第一个字），截短只去掉末尾的空白与 `.`；空的叫"未命名"（随界面语言）。
- **空着的名字**：在父节点的全部兄弟（页面与附件，`siblingsOf`）与同一父节点下在途的上传之间按标题键比较，撞上的在扩展名之前加序号：`a.png`、`a 2.png`、`a 3.png`（与导入相同）；重试时从修正后的名字算起。
- **标题键**（`lib/title-key.ts` 的 `titleKey`，树的同名判断与上传共用）：NFC、先大写再小写、词尾的 ς 换成 σ、ß 换成 ss、NFC，接近服务端的完整大小写折叠。仍有的差别都让客户端更粗（无点的 ı 当作 i、服务端的表还没有的几个字母），只会多加一个序号；更细的地方服务端答 409，由上一节的重试兜住。
- 修正与取名是纯函数，表格测试；服务端的拒绝（422 的字段、`page.title_taken` 三次之后）照常显示。

### 3.5 附件一节

- **位置**：页面的中栏、子页面列表之下（总体设计 9.2）；笔记本首页列根下的附件。没有附件时，读者看不到这一节，写者看到它的标题、上传按钮与"拖到这里上传"的说明。
- **列表**：名称、大小（`formatBytes`，界面语言）、类型的图标（图片、音频、视频、PDF、其余）；一页 100 项，"加载更多"。读完最后一页时焦点落在它加进来的第一项，没有新项时落在最后一项，列表空了落在标题；读者其间动过、或焦点已不在"加载更多"上就不移（13.2 第 26 条）。"加载更多"带着焦点消失时（它自己或上传读到了最后一页），离开文档之前把焦点交给这一节的标题。
- **每一项的菜单**（写者全部、读者前三项）：打开（内联类型在新标签页，带看不见的"在新标签页打开"；其余是下载地址）、下载（`download_url`）、复制嵌入（`![[link]]`，`link` 为 null 时不给）、改名、移动到…、删除。名称本身是"打开"的链接。
- **复制嵌入**：复制成功显示并播报"已复制嵌入"。没有剪贴板（不经 HTTPS 的部署）或复制被拒时，在这一节里显示一个只读的字段，标签带附件名、说明怎么手动复制，取得焦点并选中：菜单关闭时由这一节把焦点交给它（`onCloseAutoFocus`），关闭之后才失败的，字段出现时自己取焦点。
- **签名地址的到期**：列表里每一项的地址一到两小时有效；一节在最早的 `expires_at` 之前一分钟重读它挂着的页（不在点击之后再去签：新标签页要在用户的点击里打开，等一次请求之后浏览器会当作弹窗拦下）。每次读到列表都重新安排：至少隔 30 秒（时钟与服务端差得远时不会一直重读），至多读后 59 分钟（地址至少签一小时）。
- **改名**：对话框只改主名，扩展名照旧显示在输入框之后、不可改（附件改名不能去掉扩展名，总设计 4.2）；说明"扩展名保持不变"与错误同时给出（两个 `aria-describedby`）。没有扩展名的附件改整个名字。拒绝（422、409、引用它的页正在编辑时的 `linking.pages_locked`）留在对话框里，同改名页面。
- **移动到…**：上级的选项与移动页面共用（`parent-options.tsx`：根与全部页面），放在末尾，不给位置，没有说明文字；选它现在的上级时关闭、不发。拒绝同移动页面。
- **删除**：确认对话框；对话框是模态的，读者不能在其间做别的，关闭之后焦点回到这一行的菜单按钮，行已不在时到这一节的标题。改名、移动的对话框同样，对话框由这一节持有，行被重读拿走时不随之消失。
- **上传**：上传按钮（`<input type="file" multiple>`）与拖到这一节上传；在途的上传列在这一节的顶上（列表名"上传队列"），带进度条（`progress`，有可读的百分比）。行上只有一个按钮：发送中是"取消"，全部字节发出或已答复之后是不可用的"正在完成…"（名称带文件名），失败时是"移除"，焦点留在这个按钮上；有焦点的行离开时焦点到上传按钮。失败的显示原因（`role="alert"`）。
- **播报**：一个看不见的 `aria-live="polite"` 区域说"正在上传 N 个文件"（这一节或阅读视图上开始的，被拒的不算）、"已上传：…"与"已复制嵌入"。同一轮（任务）里开始或离开的合成一句（两样都有时两句一起说），这一轮过后再说，名称按界面语言连接（`Intl.ListFormat`）：每个上传离开是一次单独的 action，React 只显示一轮里最后设的那句。同一句再说时末尾交替加一个不换行空格，区域照样变化，读屏会再读。
- **行可以拖进编辑器**：`dataTransfer` 的 `text/plain` 是 `![[link]]`（`link` 为 null 的不可拖），CodeMirror 自己的拖放就插在落点。

### 3.6 拖放的保护

- 文件拖到拖放区之外时浏览器会打开它、离开应用：文档上对带文件的 `dragover`、`drop` 一律 `preventDefault`，`dropEffect` 为 `none`（`app/file-drop.tsx` 的 `FileDropGuard`，在应用的外壳里挂一次），页面里或别的标签页开始的也一样。拖放区（附件一节、阅读视图）自己接住外来的文件，`dropEffect` 为 `copy`。
- 保护不管编辑器（`contenteditable` 里的，目标是文字节点时看它的父元素）：CodeMirror 自己处理拖进来的东西，C 部分再接文件。
- 页面里开始的拖动（Chromium 拖阅读视图里的图片时带着文件）不当作外来的文件：文档的 `dragstart` 记下一次页内拖动，`dragend`、`drop` 结束它；拖动源在拖动中被移出文档时 `dragend` 到不了文档，取消时也没有 `drop`，指针下一次按下、或不按键的移动就结束它（拖动中没有这样的指针事件；Firefox 在拖动开始时还会派发按着键的 `pointermove`）。不往拖动里写自己的数据：WebKit 里页面写过的拖动不再带浏览器默认的数据，写进去的类型还会带到别的标签页。
- 拖放区不接经 portal 冒泡上来的事件（对话框、菜单）：它们交给文档的保护。
- 拖到阅读视图上（写者，不在编辑时，页面还在树里）上传到这一页，同附件一节的上传：`page-layout.tsx` 把阅读视图包在 `AttachmentDrop` 里；文件夹不接（`webkitGetAsEntry` 是目录的），说明用导入。

### 3.7 删除页面的确认

- 确认对话框数上子树里的附件（`PageTreeStore.nodes` 里父节点在子树里的附件），只在对话框打开时数（`attachmentsUnder`）："它下面的 3 个页面与 5 个附件会一起删除"；两句之间按语言连接（`page.sentences`：英文一个空格，中文没有）。

## 4. B：阅读视图里的附件

实现、审查与修复核对之后照实际改写（第 9.2 节）。

### 4.1 文件

| 文件 | 内容 |
|---|---|
| `server/internal/platform/markdown/obsidian/`：`asset.go`、`obsidian.go` | `Asset.Inline`；不内联的附件的链接写 `download`（4.2） |
| `server/internal/modules/asset/`：`embeds.go`、`domain/types.go` | 嵌入答出是否内联（`domain.Inline(mime, false)`；显示的类型表只建一次） |
| `server/internal/modules/linking/`：`app/ports.go`、`app/properties.go`、`module.go`、`adapter/http/handler.go`；`bootstrap/assets.go`、`deps.go` | `AttachmentAddresses`；`PropertyLink.inline`、`PageProperties.assets_expire_at`（4.3） |
| `api/modules/linking.yaml`、`page.yaml` | 同上；`PageView.assets_expire_at` 的说明 |
| `web/apps/web/src/reading/assets.ts`（新）、`reading/unfold.ts`（新，从 `reading-view.tsx` 移出）、`reading/enhancement.ts`、`reading/reading.css` | 增强 `assets`（4.4–4.6）；`ReadingContext` 加 `locale`、`assetsExpire`、`assetAddress` |
| `web/apps/web/src/pages/page/assets-expiry.ts`（新）、`page-view.ts`、`page-edit.tsx`、`page-properties.tsx`、`reading-view.tsx`、`attachments-section.tsx` | 到期重读（4.5）；右栏的附件链接（4.3） |
| `web/apps/web/src/stores/asset.store.ts` | `address(id)`：重签的地址 |
| `web/apps/web/src/i18n/`：`format.ts`、`messages/*` | `asset.sizeAfter`；`formatBytes` 留下数字的格式 |
| `server/.../obsidian/*_test.go`、`markdowntest/check.go`、`platform/markdown/sanitize_test.go`、`bootstrap/assets_view_test.go`、`markdown_app_test.go`、`permission_matrix_linking_test.go` | 标记与最后一跳随之改 |
| `e2e/stories/asset/as3-render.spec.ts`（新）、`e2e/fixtures/assets.ts`；`links/l5-completion.spec.ts`、`l6-panel.spec.ts` | AS3；`oggOpus`；属性的新字段 |

### 4.2 不内联的附件的链接

- 内容地址对图片、音视频与 PDF 内联，其余按附件下载。前端要分开它们：内联的在新标签页打开，下载的不开新标签页（Firefox 会留下空白的标签页）。HTML 里没有类型（P3B 去掉了 `data-nw-asset`，也不写 MIME），所以服务端给不内联的附件的链接写 `download=""`（同源的地址，浏览器照它下载，不跳转；没有增强时也对）。
- `obsidian.Asset` 加 `Inline bool`（`asset.Embed` 有同样的字段、同样的次序，组合根按 `domain.Inline(mime, false)` 给出）；`Markup` 的 `a` 加 `download`，用户手写的 `download` 照旧被净化器去掉。每个下载链接多 12 个字节；`MaxShown` 个链接的页面实测约 400 KB，仍在 `TestTheAppsAttachmentsAreWithinAQuarterOfHeadroom` 的余量之内。

### 4.3 属性的内联与到期

- `PropertyLink` 加 `inline`（必有，可空：没有 `url` 时为 null）；`PageProperties` 加 `assets_expire_at`（必有，可空：没有附件的地址时为 null，否则是最早的到期）。linking 的 `AttachmentAddresses.Addresses` 按 id 答出地址、是否内联与到期；只有附件的链接取地址（`!l.Asset` 的守卫：存储多答了也不用）。
- 右栏：内联的链接在新标签页打开，带看不见的"（在新标签页打开）"（空格在提示之外、链接之内，与附件一节的行相同）；其余的带 `download`。属性与视图走同一个到期重读（4.5）。右栏不显示大小（`PropertyLink` 没有字节数；阅读视图的属性表显示，见 4.4），记在 M12 的打磨移交。

### 4.4 增强 `assets`：附件的链接

- `a.nw-asset[href]:not([download])`：加 `target="_blank"`、`rel="noopener noreferrer"` 与看不见的提示（`asset.newTab`，文字随界面语言）；提示是链接里的一个空格与一个 `span.sr-only`。有 `download` 的不动。
- `a.nw-asset[data-nw-size]`：链接之后加 `span.nw-size`，按界面语言写大小（`asset.sizeAfter`：英文" (8 B)"、中文"（8 B）"；`formatBytes` 按 `ReadingContext.locale`），颜色取 `--muted-foreground`，不折行。没有地址的附件（文字）没有大小。
- 增强清理时移除加进来的提示与大小、去掉 `target` 与 `rel`，HTML 回到服务端的原样；增强在 `readingEnhancements` 的最后（13.2 第 23 条记下它加的标记：提示、大小与保留的媒体）。
- 点图片不做什么。

### 4.5 到期与加载失败

- **到期重读**（`pages/page/assets-expiry.ts`，视图与属性共用）：
  - SWR 的读经 `stamped` 记下读到的时刻（一个 `WeakMap`；没经 `stamped` 的答复按第一次见到的时刻），`eachRead`（`compare: Object.is`）让每次读都是新的答复：服务端一个小时之内签出同样的地址，内容相同的答复也要各自安排下一次重读。
  - `rereadIn(expires, read)`：到期前一分钟，至少读到之后 30 秒，至多 59 分钟（地址至少签一个小时，不论时钟怎样）。附件一节的列表用同一个函数。比服务端快一个小时以上的时钟因此每 30 秒读一次。
  - `useAssetsExpiry`：答复按时重读一次，视图与大纲两个读者只读一次（`readAgain`）；隐藏的标签页里到时不读，显示时由 SWR 的 focus 重读。缓存里的答复在第一次见到时已过期一分钟以上就不显示（返回 undefined），由 SWR 挂载时的读取代；那一读失败时显示"重试"。显示着的答复到期后照旧显示：标签页显示、恢复连线时 SWR 重读，其间加载失败的由增强重读。
  - `assets_expire_at` 解析不出时当作没有地址；`page-edit.tsx` 写入的视图也经 `stamped`（`readView`）。
- **加载失败**：增强在容器上以捕获阶段听 `error`（它不冒泡）：图片、音视频加载失败，而视图的到期时刻已过时，经 `context.reload()` 重读；同一个页面、同一个到期时刻至多一次（增强的闭包按页面记下）：文件不在、限流、解不开的图片，在同一个小时里重读也一样失败，不能循环。

### 4.6 保留正在播放的音视频

- HTML 每小时因签名而变，别人的编辑、`links` 事件也让它变，`innerHTML` 会毁掉正在播放的媒体。增强清理时记下开始了（在播放，或暂停在中间）且没有播完的 `audio.nw-asset`、`video.nw-asset`，键是附件 id（地址的路径里）与它在这个 id 里的次序，至多 20 个（`MaxMedia`），以及有焦点的那个；同一页面的下一次运行里，新 HTML 里同键的元素换成记下的那个：属性取新 HTML 的（只改不同的），地址保留它自己的；有焦点的交还焦点，不滚动，它所在的折叠 callout 先展开（`reading/unfold.ts`）。对不上的丢掉；别的页面的不保留。
- **重签**：开始了的音视频（在播放、暂停在中间，或按了播放还没加载出来；接回的与这份 HTML 自己的一样）加载失败时（通常是地址过期：很长的录音、隐藏的标签页里不重读、读者离开了一阵），经 `ReadingContext.assetAddress(id)`（`AssetStore.address` → `GET /assets/{id}` 的 `content_url`）就地重签，同一个元素：换上新地址，接着失败时的位置与速率、读者其间留下的播放与否、音量与静音，原来在播放的接着播放（同一个元素，浏览器给它的播放许可还在），暂停的设 `preload="metadata"` 显示那一帧。
  - 重签途中不再签：它的旧地址其间再失败（Chromium 的控件在出错时按播放先重新加载）不理会。重签途中记下失败时的位置与速率；其间重新加载过的（`error` 已清，或又因失败设上）丢了它们，取记下的，否则取现在的（读者其间改的速率）。
  - 重签成功之后一分钟之内（`signedAgainAfter`）再失败，是文件的问题，不再签，按 4.5 交给视图的重读；之后的失败可以再签（隐藏的标签页里一段很长的录音跨过每一次到期）。
  - 地址没给（附件已删除、断网、限流），元素仍在页面里时按 4.5 重读视图；下一次失败可以再签。元素已不在页面里时不做什么。
  - 没开始的媒体失败（服务端写 `preload="none"`，只有按了播放才加载），按 4.5 交给视图的重读。
- **出错的不保留**（`error` 不为 null）：它重签过也失败，或签不了，不会再加载。新元素放在它的位置、速率、音量与静音上，有焦点的把焦点交给新元素（同样先展开），不预载、不播放，等读者；只有重签途中、在播放的，新元素（地址已是新签的）接着播放，位置同样取重签记下的。
- 放到位置上（`putAt`）：先挂 `loadedmetadata`（那时位置差 0.25 秒以上才再设一次），再设位置（不收的引擎抛错时由 `loadedmetadata` 补上）；`play()` 被拒就停在那里，读者再按播放。
- 已知的限制：替换时原生全屏退出（Fullscreen 规范的 removing steps）。
- `ReadingContext` 加 `assetAddress(id: string): Promise<string>`（只读，任何角色都有）、`assetsExpire`、`locale`。

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

### 9.1 A：面板与上传（2026-10-09，合并 `e44b417`）

- 提交：
  - 实施：附件一节、上传与它们的 store `1b7c62d`、e2e AS1 的页面版本 `f97f78b`；负对照的补测 `494534e`。
  - 审查的修复 `39133aa`、`3d2168c`；修复核对的修复 `11c701e`、`e3e0556`、`5b4d31f`、`6c14045`、`5235800`。合并 `e44b417`。
- 审查：三位审查者（Opus），没有高；中低到低的行为缺陷十余条，测试缺口二十余条，都已处理。修复核对四轮：第一轮行为问题 7 条（低到中），第二轮 3 条（低），第三轮 1 条（低）与一处 Firefox 的风险，第四轮没有。逐条见[审查记录](reviews/P4A-assets-web-review.md)。
- 审查之后改了的设计（第 3 节已改写）：
  - 上传的请求不带正文，表单由注入的传输发出；`Response` 不收的答复以 `TypeError` 拒绝；早答被重置作为已知限制，不重试。
  - 树：上传答复之后的读经 `wrote()` 合并，每个上传在它之后开始的第一次读完成时结算；被重叠而丢弃的读交回最近发出的那次。列表：重读读已有的页数，同一父节点一次一个，排着的被再次要求时就是那一次，接页按 id 去重；成功的上传读到它所在的页才离开；附件在树读完、显示之后再读，连上时与阅读视图、右栏同一层。
  - 名称：`titleKey`（先大写再小写、ς、ß），树与上传共用；只有同一父节点下在途的上传占名字。
  - 附件一节：行上一个按钮（取消、正在完成、移除）与焦点的交接；播报的区域；没有剪贴板时的复制字段；到期重读的下限与上限；"加载更多"的落点；移动对话框不另加说明；页面已不在树里时没有这一节与拖放。
  - 拖放：保护不管编辑器，挡一切带文件的拖动；拖放区不接 portal 里的、页内开始的拖动（标志由指针的下一次按下或没有按键的移动结束）；拖放在 `page-layout.tsx` 里包住阅读视图。
- 反向对照：本机 54、81、92、106、9、1 个（实施之后、审查的修复之后、四轮修复核对之后），存活的都由补测抓到；细节见审查记录。
- CI 与发布：分支的 CI 在 `f97f78b`（C4 多一次 404）、`3d2168c`（AS1 的 `dropEffect`、C4 的计数）上失败，各由下一次修复修好；`e3e0556`、`5b4d31f`、`5235800` 上 server、web、image、e2e 全部通过（`image` 一步跑 `make image-smoke`）。本机的 Docker Desktop 起不来，数据库的测试、e2e 与合并之后的 `image-smoke` 都由 CI 跑。
- 交给 B 与 C 的：
  1. C：编辑器的扩展接文件时，`dragover` 要对一切带文件的拖动 `preventDefault`（文档的保护不管 `contenteditable` 里的，不接就由 CodeMirror 自己处理，它会把文件当文本读进来）；同时带文字与文件的拖放不交给 CodeMirror。页内开始的拖动（`app/file-drop.tsx` 的标志）同样不是外来的文件，扩展要用同一个判断（届时从 `file-drop.tsx` 导出）。
  2. C：上传经 `AssetStore.upload`（并行、占名、重试、换代中止、关闭页面的提醒），编辑器只管插入。附件一节播报这个父节点下的全部上传，编辑器开始的也在内：编辑栏若另说进度，由 C 决定怎样不重复。
  3. B：阅读视图里的附件链接的增强与附件一节的行用同一个 `lib/asset-kind.ts`（`opensInline`、`embedOf`）；不内联的附件在列表里的"下载"提示随 B 一起看（记在 M12 的打磨移交第 16 项）。
- 待人工确认（真实的浏览器，审查记录"接受与推后的"）：从访达拖文件到附件一节与阅读视图（Chromium、Safari、Firefox）；页内拖一张图片按 Esc 取消，再从访达拖文件进来；Firefox 里用键盘打开"打开"；不经 HTTPS 的部署里用读屏复制嵌入。
- 负责人可以改判的取舍：上传落在已读的页之后时读到它所在的页（也可以只读到一定页数，说"在后面的页里"）；早答被重置时显示"上传失败"、不重试（也可以按 `TypeError` 重读列表，看是否已到达）；"正在完成…"之后不能取消（服务端已收下全部字节）；页内开始的拖动的文件不上传（Chromium 拖图片时带着文件）；中文的列表名"上传队列"。

### 9.2 B：阅读视图里的附件（2026-10-09，合并 `32e175c`）

- 提交：
  - 实施：服务端 `ca78118`、增强与到期 `1616af4`、e2e AS3 `de0875e`、L5 与 L6 的新字段 `e83e3d7`；负对照的补测 `c792089`。
  - 审查的修复 `4e690b3`；修复核对的修复 `df560c3`、`e7a2766`、`8096579`、`ff985f4`、`c0898be`。合并 `32e175c`。
- 审查：三位审查者（Opus）。服务端与契约没有行为缺陷；前端中 3 条（出错的音视频被保留、不再重签；内容相同的答复让到期重读停下；快一小时以上的时钟下回到页面一直"加载中"）、中低 4 条（保留的媒体丢焦点、每小时整篇替换、jsdom 不模拟加载算法、大小的显示在交接中掉了），都已处理。修复核对五轮：第二轮中 1 条（后台播放跨过过期时换成新元素，音量丢失）、中低 1 条，第三、四轮各有低的行为问题，都在重签的时序上；第五轮只剩 1 条此前就有的低，推后。逐条见[审查记录](reviews/P4B-assets-web-review.md)。
- 审查之后改了的设计（第 4 节已改写）：
  - 到期：SWR 的每次读都是新答复（`eachRead`），到期从读到的时刻算（`stamped`）：服务端一个小时之内签出同样的地址，原来的"每个答复安排一次"会停下。快一小时以上的时钟每 30 秒读一次，视图照常显示（取代原来的"至多一次"）；隐藏的标签页到时不读，显示时由 SWR 读；解析不出的到期当作没有。
  - 大小：链接之后按界面语言写大小（`data-nw-size`；M7 总设计 4.8 与 P3 的移交第 1 项承诺过，第 4.4 节原来漏了），不折行。
  - 音视频：开始了的（不只是接回的）加载失败时就地重签，同一个元素，接着位置、速率、音量与静音；重签途中的失败不理会，一分钟之内不再签，地址没给时按视图的过期重读。出错的不保留，新元素放到位置上、等读者（重签途中在播放的除外）。有焦点的交还，它所在的折叠 callout 先展开（`reading/unfold.ts`，与 `reading-view.tsx` 共用）。接回时属性只改不同的。
  - 服务端：`download=""` 每个链接 12 字节（原写 9），`MaxShown` 个链接的页面实测约 400 KB；显示的类型表只建一次；属性的 `!l.Asset` 守卫保留。
- 反向对照：本机实施之后 56 个、审查的修复之后 18 个、五轮修复核对之后 17、15、15、11（另回跑 19）、3 个，存活的都由补测抓到或判为等价；e2e 1 个（不保留媒体），AS3 的页面版失败。细节见审查记录。
- CI：`c792089` 上 server 一步的一个计时用例（与 P4B 无关）失败一次，本机连跑五次通过；其余每次提交 server、web、image、e2e 全部通过。本机的 Docker 这一部分可用，数据库的测试与 AS3 本机也跑过；本机负载很高时（别的会话），全量 vitest 并行会让各文件第一条用例超时，以较少的 worker 跑全部通过。
- 交给 C 与 P5、P6 的：
  1. P5、P6：附件一节按 MIME 前缀判断是否内联（`lib/asset-kind.ts` 的 `opensInline`），阅读视图与右栏以服务端为准（`download`、`PropertyLink.inline`）。P4A 交给 B 的第 3 项原意是两者共用 `lib/asset-kind.ts`；HTML 里没有 MIME，B 改成以服务端为准。今天两者等价（`TypeOf` 存下的 MIME 只会是显示的类型或 `application/octet-stream`，契约的 `Asset.mime` 写明）；导入写附件时同样要经 `TypeOf`，否则 `Asset` 加 `inline`，附件一节改用它。
  2. C：编辑器插入的嵌入在阅读视图里由 `assets` 增强处理（新标签页、大小、到期、保留与重签），C 不另做；编辑器里的附件预览不在 v0.1。
- 待人工确认（真实的浏览器，见审查记录"接受与推后的"）：Firefox 与 Safari 在 CSP `sandbox` 下显示 PDF；Safari 在元数据之前设位置；出错之后的 `paused` 与 `currentTime`；移出又放回时的暂停、全屏与焦点；折叠 callout 里先展开再给焦点；Safari 同一元素换地址之后在后台的 `play()`；Chromium 的控件在出错时按播放先重新加载；Firefox、Safari 的控件出错时能否重试。
- 负责人可以改判的取舍：
  - 每小时整篇替换文章推后到 M12（只有地址不同时就地更新地址），也可以改为 v0.1 内做；
  - 快一小时以上的时钟每 30 秒读一次视图、属性与附件列表；
  - 隐藏的标签页到时不重读，显示时读；
  - 出错的音视频换新时不自动播放；
  - 提示前可见的空格、复制带上提示、右栏不显示大小、被拒之后的恢复，推后到 M12 的打磨。
