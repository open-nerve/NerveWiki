# M12/P1 S1（e2e 的帮手）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-p1-s1`。实施的 8 个提交，从 `main` 的 `179a7aed` 开始，审查时在 `8e128a2d`：<br>A6 第二个页面测试自己开的标签页先受监视 `479d14e5`；<br>帮手 `e2e/fixtures/navigation.ts`、`watchPage` 调 `followRequests`、109 个故事与 `wiki-health.ts` 机械地换上 `bafbd418`；<br>门禁 `tools/e2e-navigation/check.mjs`，进 `make lint-web` 与 knip 的入口 `c18562fe`；<br>`PageWatch.signInLandings` 与失败时页面状态的第一行 `6cf1c32c`；<br>A15 七格、`threeBrowsers`、门禁的表 `ce2cc0fc`；<br>`networkSettled` 等待时的一行日志 `c330217f`；<br>门禁核对读到了帮手的模块 `e3059737`；<br>A15 第 1 格末尾的 `replaceState` `8e128a2d`。<br>118 个文件，加 1,115 行、减 279 行（`git diff --shortstat`）；只改 `e2e/`、`tools/e2e-navigation/`、`Makefile` 的 `lint-web` 与 `knip.jsonc`，没有网页、服务端、契约与依赖的改动。<br>修复一轮在它之上 5 个提交，头 `3c2425c1`：先看锁或租约、再看记录，A15 第 8 格与帮手的说明 `6ec05d13`；A15 第 9 格（没有 Web Locks 时的租约）`995ebf02`；门禁读八种结尾、认三种引号、九个样例与它的文件头 `91718e48`；A15 第 1 格末尾的来回 `6e48227d`；那一行只在落到过 `/sign-in` 时写 `3c2425c1`。4 个文件，加 185 行、减 50 行。<br>主持者在修复核对之后另加 `937df215`，只改 `pageState` 的说明（P1S1A-N2 = C-N8）。分支共 14 个提交；`179a7aed..937df215` 118 个文件，加 1,252 行、减 279 行 |
| 审查方式 | 三位审查者（Opus）并行，各在自己的分离的工作树里（挪到 `8e128a2d`），重的都经 `heavy.sh`，不往外发东西，不碰实施者的工作树：<br>A 帮手与它的偏离：帮手的每条路径；给文档令牌的入口逐个数；两个探针 `a15-zprobe-a`（在这个文档里登录时锁被持住；跟着别的标签页登录）与 `a15-zprobe-k`（编辑页 `pagehide` 的 keepalive 请求），三种浏览器；变异 `lock-first`（先看锁再看记录）、`refresh-only`（只认续期），跑 A15、A3、S4 与 `stories/identity/` 整组。<br>B 测试、变异、浏览器与 CI：A15 七格在三种浏览器共 220 次（含 8 个 worker 的加压）；设计 7.2 的 S1 各行、实施者的 x1–x4 与自己的七条变异，各三种浏览器三轮；8 个提交逐个 `make lint-web`、`make knip`、`pnpm -r run check:types`；全量 `make e2e` 三次（`bafbd418`、头、插记录的帮手）；CI 的作业日志与步骤时间。<br>C 替换、门禁与文档草稿：自己写的计数与两种反向比较（去掉空白相同；反向之后经 oxfmt 逐字节相同）；没改的门禁对各种写法的探针；门禁的 17 条变异；实施报告第 10–12 节的文档草稿逐句对照代码、设计、00-M12 与 02-workflow 3.5 |
| 日期 | 2026-11-15（实施、审查、修复一轮、修复核对一轮） |
| 结论 | Important 0。Minor 4：在这个文档里登录、注册，写记录还在等锁时帮手不等就换文档（P1S1A-M1）；"跟着别的标签页登录的文档会等满 10 秒"不成立，却要写进设计（P1S1A-M2）；没有 Web Locks 时等租约的一支没有测试（P1S1B-M1）；文档草稿漏了要照实改的各节（P1S1C-M1）。Nit 9（三份合计 10，P1S1B-N2 = C-N1；P1S1A-N1 与 C-N4 的后一半是同一处说明，C-N4 另有门禁的注释，各算一条）。三位都确认：偏离"令牌的答复认续期、登录、注册"对且必要，给文档令牌的只有这三种答复；替换是机械的；门禁抓得到；自己加的三个提交都必要；每个提交单独为绿；全量 e2e 绿。代码一侧的由修复一轮改，文档一侧的在本提交里改。修复核对一轮：三位都没有 Important、Minor，第一轮的发现都关上；新的 Nit 三条（P1S1A-N2 = C-N8、C-N9、B-N3），一条由主持者的 `937df215` 改注释，文档的都在本提交 |

## 审查的发现与处置

编号照审查者的报告。审查时的代码位置是 `8e128a2d` 的；`W/` 指 `web/apps/web/src/`。修复一轮照主持者的修复说明（`p1s1/review/fix-1.md`）在 `8e128a2d` 之上加新提交，普通推送、不改写已推的历史，每个单独为绿；提交与结果见"修复一轮"、"核对"两节。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P1S1A-M1 | 中（帮手） | `e2e/fixtures/navigation.ts:128` 在记录为空时直接交回空，走不到 `:134-139` 的锁与租约。开始时没有记录的文档（登录页、注册页、没登录时的邀请页）里，登录或注册的答复读完之后，`TokenManager.signIn` 才在锁里写记录（`W/stores/auth.store.ts:34-41`、`W/session/token-manager.ts:164-172`）；这一段在途的集合已空、记录还没写，帮手就换了文档，令牌随旧文档丢了，新文档落到 `/sign-in`。A 的探针：登录页上别人持着锁，登录答 200 之后经帮手换页，三种浏览器都在放开之前约 1 秒就请求了新文档、落到登录页。今天没有故事这样换文档（`signInWith`、`signUpWith` 之后都先断言界面） | 修复一轮照 A 的改法：先看锁或租约，再看记录；照 A15 第 6 格的写法加一格"登录答复之后锁被持住时换文档"，在 `8e128a2d` 上要红。A 的 `lock-first` 下探针三种都绿、`stories/identity/` 69 个通过。修复一轮：`6ec05d13` 先判 `#root`，再判锁（没有 Web Locks 时判租约），锁之后才读记录；A15 第 8 格在旧帮手上三种浏览器各三轮 9 个全失败。核对 A：关上 |
| P1S1A-M2 | 中（文档） | 实施报告第 9 节第 1 条与 3.15、10.1 的草稿说"开始时没有记录、之后跟着别的标签页登录的文档，帮手会等满 10 秒"。不成立：那个文档经 `storage` 事件切成已登录之后，守卫显示的页面读 `me`（`W/app/guards.tsx:35`、`:65`，`W/pages/invitation.tsx:139`），这次读在锁里续期，`performance` 里就有了续期的一条，帮手等的是它。A 的探针三种浏览器都在 42–242 ms 内换了文档 | 本提交照 A 改：3.15 第 2 条与 10.1 的"留下的"都不写这一句；3.15 写"跟着别的标签页登录的文档在它下一次读（`me`）时于锁里续期，帮手等的是那次续期"。核对 A：关上（探针在 `3c2425c1` 上复跑，帮手 61–214 ms 就换了） |
| P1S1A-N1 | 细节（说明） | `navigation.ts:8-9`、`:83-84` 的说明仍只写"第一次续期" | 修复一轮照新代码改（`6ec05d13`：`navigation.ts:8-10`、`:85-86`、`:113-118`）。核对 A：关上 |
| P1S1B-M1 | 中（测试） | 没有 Web Locks 时看租约的一支（`navigation.ts:138-139`）没有测试：第 6 格只在有 Web Locks 的页面上持锁，7.2 第 18 行把锁与租约一起去掉，只由锁那一半抓住；没有 Web Locks 的 A4、C9 的 `goto` 都在 `about:blank` 上，走不到租约。B 的变异 mL（租约恒交回空）、mLK（租约的键改名）A15、A4、C9 全照过；插记录的全量里帮手没有一次读到租约。设计第 4 节"网页改了它们，A15 会先失败"对租约不成立 | 修复一轮照 B 的探针补进 A15（删 `Navigator.prototype.locks`，第一次续期答完之后写一个别的标签页的租约，帮手要等它删掉才换文档）；mL、mLK 要被抓到。B 的探针头上三种浏览器各三轮通过、去掉这一支各三轮失败。修复一轮：A15 第 9 格（`995ebf02`）。核对 B：关上，m21（去掉租约一支）、m22（租约的键改名）只有第 9 格三种 9/9 失败；设计第 4 节对租约也成立了 |
| P1S1B-N1 | 细节（文档） | 10.1 草稿里帮手对已有故事多花的时间只有墙钟的旁证（140 对 142 秒） | 本提交照 B 量到的数写进 10.1 与 7.4：A15 之外帮手只真的等过 6 次、合计 0.23 秒。核对 B：关上 |
| P1S1B-N2 = P1S1C-N1 | 细节（文档） | 10.1 草稿写"S1 的 11 行"，设计 7.2 里标 S1 的是 12 行（1–8、16–19） | 本提交写 12 行。核对 B、C：关上 |
| P1S1C-M1 | 中（文档） | 文档草稿只给了 10.1、3.15、13.4、移交、4.4 与决定 339，没有列出要照实改的各节；照草稿落笔之后，P1 设计的第 0 节、决定 339、3.2–3.6、3.14、第 5 节、6.1、7.1、7.2、7.4、第 8 节第 5、7 条与头，00-M12 的 `:725`、`:947`、`:212`，都与代码、与新的 13.4 不一致；设计 3.4 要贴进记录的替换脚本与输出也没提 | 本提交逐处照代码改（P1 设计 3.15 的开头列出各节）；替换的脚本与输出在本记录的"替换"一节。核对 C：关上（照修复报告第 6 节的清单；决定 342 与第 8 节第 3 条的数法另由 C-N8 加进来） |
| P1S1C-N2 | 细节（文档） | 决定 339 的改稿只在末尾加括注，粗体里仍是旧的规则 | 本提交连粗体一起改，括注写原来的说法（00-M12 第 11 节与 P1 设计 2.3）。核对 C：关上 |
| P1S1C-N3 | 细节（文档） | 4.4 的改稿说登录、注册的文档"之后不续期"：它们只是不发第一次续期，之后照样续期 | 本提交写"不发第一次续期"。核对 C：关上 |
| P1S1C-N4 | 细节（说明） | `check.mjs:5` 说帮手的模块"not read at all"，`:9-10` 又说没读到它就失败，还说"或只读了 `e2e/` 的一部分"也会失败（C 的变异 c2：`SKIP` 多一个故事分组，探针照过）；`navigation.ts:8`、`:83-85` 同 A-N1 | 修复一轮照代码改：`check.mjs:5` 写 "read but not checked"，`:9-10` 写 "…or by missing e2e/fixtures/ or the stories the table names"（`91718e48`）；`navigation.ts` 同 A-N1（`6ec05d13`）。核对 C：关上 |
| P1S1C-N5 | 细节（门禁） | 门禁只读 `.ts`、`.mts`、`.js`、`.mjs`，Playwright 默认还跑 `.tsx`、`.cts`、`.cjs`、`.jsx`；反引号写的逃生口不计数（oxfmt 不改它）；样例没钉住"按整个路径例外"（变异 c3：按文件名 `navigation.ts` 例外，七个样例都过） | 修复一轮照 C 改（`91718e48`）：`SOURCE` 是 `/\.[cm]?[jt]sx?$/`（Playwright 默认 `testMatch` 的八种结尾），每次运行先核对八种结尾都读；`LEAVE` 认三种引号；新样例"反引号写的逃生口"与"另一个叫 `navigation.ts` 的模块里的直接 `goto`"。`page["goto"]`、`.bind`、`location.href` 照决定 344 不管（"接受与推后的"）。核对 C：关上 |
| P1S1C-N6 | 细节（测试） | 落点去重的偏离（按上一次主框架的地址）的理由没有测试分得开：换成设计的"与上一条落点相同就不记"，A15 照过 | 修复一轮补了（`6e48227d`）：A15 第 1 格在 `replaceState` 之后再 `pushState` 到 `/`、回到同一个登录页地址，断言两条相同的落点；设计的写法 m23 在 `8e128a2d` 上活、在头上第 1 格三种浏览器 9/9 失败。核对 C：关上 |
| P1S1C-N7 | 细节（文档） | 每个失败的测试的页面状态都有 `sent back to sign in:` 一行，没有落点时是 `no`；"看有没有这一行"数出的是红了几次，不是被跳回几次 | 修复一轮取"只在页面落到过 `/sign-in` 时写"（`3c2425c1`，不改成计数）；探针：落到 `/sign-in` 之后失败的第一行是 `sent back to sign in: /sign-in?next=%2Fsettings%2Fprofile`，在 `/sign-up` 失败的没有这一行。核对 C：代码关上；数法见 P1S1A-N2 = C-N8 |
| P1S1A-N2 = P1S1C-N8 | 细节（说明与文档，修复核对） | `browser.ts:279-280` 的说明、`3c2425c1` 的提交说明与修复报告的 3.15 第 10 条说"有 `sent back to sign in:` 这一行的失败就是被跳回的"。不对：故事自己去登录页的也有这一行（没登录被守卫转去、在 UI 上登录、有意退出或停用），A15 第 8 格本身就是一例（断言落点 `["/sign-in?next=%2Fsettings%2Fprofile"]`，没有 401） | 主持者的 `937df215` 只改说明，行与文字不变；`3c2425c1` 的提交说明已推、不改，`937df215` 的说明更正了它。本提交把数法写成"数这一行带 ` after a refresh answered 401` 的失败，再除去有意造重用的故事（A5）"：P1 设计 3.3、3.15 第 10 条、决定 342（2.3）、第 8 节第 3 条、10.1 的"留下的"，00-M12 的决定 342 |
| P1S1C-N9 | 细节（文档稿，修复核对） | 修复报告写决定 342 "01-P1 `:115` 同样"改，可 00-M12 `:1204` 的锚在 01-P1 里 0 次，`:115` 的原文不同；说明 13.4 新句与原句的差别时漏了"、自己注册的文档不必等"（第一轮就加了），句子本身对 | 脚本对 01-P1 `:115` 照它的原文另写一个锚，断言恰好一次；说明补上（P1 设计第 8 节第 5 条与 00-M12 第 7 节第 12 条的括注） |
| P1S1B-N3 | 细节（文档稿，修复核对） | 修复报告与 10.1 草稿写修复之后那次 CI "纯接口测试 1.35 倍"，照两次 CI 日志重算不出：标题带 `(API)` 的 152 个，中位数 1.53；去掉 50 毫秒以下的 122 个，1.52；只取 0.5 秒以上的 15 个才是 1.31。结论不变，慢在 runner | 本提交写"约 1.5 倍（标题带 `(API)` 的 152 个，中位数 1.53）"（10.1、本记录） |

## 主持者的决定

负责人可以改判（修复说明 `fix-1.md` 与 Step 文档的起草说明里定下）：

- 代码一侧的 Minor（A-M1、B-M1）与 Nit（A-N1、C-N4–N7）由修复一轮改，每个提交单独为绿，A-M1、B-M1 各加一格、在 `8e128a2d` 上要红；文档一侧的（A-M2、C-M1、C-N1–N3、B-N1、B-N2）由本提交改。
- 决定 339 照实施改，连粗体：帮手等的第二样是"这个文档有了给它令牌的答复（文档的 `performance` 里有续期、登录或注册的一条）"，原来是"这个文档的第一次续期已答"。00-M12 第 11 节与 P1 设计 2.3 同改，括注写原来的说法与理由。
- 实施中定的几条（下一节）没有给正式编号，记在 P1 设计 3.15；主持者可以给号或改判。
- 修复核对之后不再开第二轮修复：三位都没有 Important、Minor。新 Nit 的处置：P1S1A-N2 = C-N8 由主持者的 `937df215` 改 `pageState` 的说明（只改注释），文档写按 ` after a refresh answered 401` 数；C-N9 与 B-N3 在本提交改文档。
- 本提交另照设计第 8 节第 6 条改 README 的 e2e 一节（帮手一条）与 05-lessons 第 6 节 `:80` 末尾的一句；05-lessons 另收审查的三条：前面的早退会盖住后面的检查（P1S1A-M1，第 6 节），自检的样例证明不了遍历读到了文件（门禁，第 1 节），没有一次运行走到的分支等于没测（P1S1B-M1，第 1 节）。

## 实施中定的

实施报告第 8 节第 7 条，三位审查者各在自己的范围里判过都对：

- A6 的标签页加 `watchPage`，不给帮手开"没被监视的空白页放过"的例外（A：照决定 341）。
- 文档的令牌认续期、登录、注册三种答复，A15 加第 7 格（A：给文档令牌的入口只有这三种；放宽之后漏不掉一次还没发出的第一次续期）。
- 页面里的一次读至少 1 秒（A：`refresh-only` 之下报错写出真在等的一样；最长约 11 秒，测试 30 秒）。
- 第 5 格先断言路由记下的结局（B：变异 1、3 立刻以 `Expected "answered", Received "dropped"` 失败）。
- 落点按上一次主框架的地址去重，A15 第 1 格的 `replaceState` 钉住"同一地址再报一次不记"（A、C；另一半见 P1S1C-N6）。
- 门禁核对读到了帮手的模块，内置样例比整张问题表（C：变异 5 由门禁自己抓到）。
- 第 50 条的那一行写 `ip` 第一次看到的全文（换行改成 ` | `），时限的秒数取 `settleMs`。

修复一轮的（修复报告第 7 节）：

- C-N7 取"只在落到过 `/sign-in` 时写那一行"，不改成计数：计数的写法还得读出数来比。
- 第 9 格去掉 Web Locks 照审查 B 的探针：上下文的 init script 删掉 `Navigator.prototype.locks`，先断言页面里 `"locks" in navigator` 为假；租约写成别的标签页的（`owner: "another-tab"`，5 秒），长于帮手的 1 秒窗口。
- 门禁的八种结尾不进 `SAMPLES`（`SOURCE` 用在遍历里，不在 `check()` 里），每次运行另核对 `x1-sample.spec.<结尾>` 八个名字都被 `SOURCE` 取到。

## 接受与推后的

- **接受**：
  - A-M1 改了之后剩下的那几个 microtask（Playwright 报 `requestfinished`、帮手读页面，与页面读完登录的答复、调 `navigator.locks.request` 之间的先后）：没造出来，要造得扣住页面读答复（A 的未核实的怀疑第 1 条）。修复把读记录放在锁之后，"读到记录为空之后、看锁之前记录被写下"的那一刻也盖住了（核对 A）。
  - 先锁后记录的副作用（核对 A）：没有记录的应用文档在别的标签页持锁时也等，持锁的只有续期、登录、退出，至多 8 秒，短于帮手的 10 秒；有记录、第一次续期正排在锁上的文档，报错写 "the refresh lock" 而不是 "the document's first refresh"，等的东西不变。
  - 租约不看过期的变异（核对 B 的 c-mLexp）没有一格失败：它只让帮手多等，对过期的租约等满 10 秒后明确报错，不会漏等；应用放开租约时会删掉它。
  - `LEAVE` 认混用的引号（例如 `"leave'`）：只会多数、不会漏数（核对 C）。
  - `tokenPaths` 不看答复的码：被拒的登录、注册同样留一条（C 的未核实的怀疑）。只在"开始时有记录、第一次续期还没答，同一个文档里又有一次被拒的登录"时少等一样，今天没有这样的流程；文档照代码写"有一条续期、登录或注册的答复"。
  - 门禁看不见 `page["goto"](…)`、`page.goto.bind(page)`、页面里的 `location.href = …`：决定 344 按文字找，今天 `e2e/` 里一处也没有（P1S1C-N5）。
  - C 的门禁变异 c1（有问题也 `exit(0)`）、c7（样例不比较）活下来：样例只证明 `check()` 看得见，进程的退出码由没改的门禁对探针 rc=1 证明，同 `tools/md-fixtures/check.mjs`；修复之后照旧活（核对 C）。c8（`DIRECT` 去掉 `\s*`）今天等价；c5（只读 `.ts`）审查时等价，修复之后由八种结尾的核对抓到。
  - 设计 7.2 末段的等价变异（只从在途的集合里去掉续期）没有跑；"读不了"交回 "the next document" 与 "the next document (pending)" 两支没有专门的测试（设计 3.2）。
  - 第 2、4、5、6、8、9 格的 1 秒与 2 秒的界在更慢的 runner 上会不会碰到：本机 4 与 8 个 worker、CI 两次（修复之后那台慢约 1.5 倍）都远在界内，看合并之后的 `main`（B 的未核实的怀疑）。
- **推后、交出去的**：
  - Chromium 里路由扣着、从没交回的请求，文档走了之后留在在途的集合里，同一页面之后再经帮手换文档会等满 10 秒：今天只有 A15 的两处逃生口留下这样的请求，之后那两格不再换文档（`followRequests` 的说明写了）。
  - 续期在网络一层失败（没有答复）的文档，帮手等满 10 秒（设计 3.2）。
  - WebKit 把编辑页 `pagehide` 里 keepalive 的 `DELETE /api/v0/edit-sessions/…` 报成 `Load request cancelled`，服务端是否收到没有查（A 的未核实的怀疑第 2 条）：产品一侧，不在 S1，交主持者看是否进 M5 的移交。

## 替换（设计 3.4）

一次性的脚本，不提交，照设计 3.4 贴在这里。实施者在 `479d14e5` 上跑 `node replace.mjs <仓库根>`，再 `pnpm run fix:format`：

```js
// One-off (not committed): M12/P1/S1 design 3.4. Puts the stories' and wiki-health.ts's gotos and reloads on
// the helper of e2e/fixtures/navigation.ts. Usage: node replace.mjs <repo root>
import { readFileSync, readdirSync, writeFileSync } from "node:fs";
import { join, relative } from "node:path";

const root = process.argv[2];
const files = [];
const walk = (dir) => {
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) walk(p);
    else if (p.endsWith(".ts")) files.push(p);
  }
};
walk(join(root, "e2e/stories"));
files.push(join(root, "e2e/fixtures/wiki-health.ts"));

let changed = 0;
let gotos = 0;
let reloads = 0;
for (const file of files.sort()) {
  const before = readFileSync(file, "utf8");
  let g = 0;
  let r = 0;
  let text = before.replace(/\b([A-Za-z_$][\w$]*)\.goto\(\s*/g, (_m, x) => {
    g += 1;
    return `goto(${x}, `;
  });
  text = text.replace(/\b([A-Za-z_$][\w$]*)\.reload\(\)/g, (_m, x) => {
    r += 1;
    return `reload(${x})`;
  });
  if (g === 0 && r === 0) continue;
  const names = [g > 0 ? "goto" : null, r > 0 ? "reload" : null].filter(Boolean).join(", ");
  const fixture = file.endsWith("e2e/fixtures/wiki-health.ts");
  const source = fixture ? "./navigation" : "../../fixtures/navigation";
  const prefix = fixture ? "./" : "../../fixtures/";
  const line = `import { ${names} } from "${source}";\n`;
  // The import statements at the top, each with its source and where it starts.
  const imports = [...text.matchAll(/^import\s[\s\S]*?\sfrom\s+"([^"]+)";\n/gm)];
  const ours = imports.filter((m) => m[1].startsWith(prefix));
  if (ours.length === 0) throw new Error(`${file}: no import from ${prefix}`);
  const after = ours.find((m) => m[1] > source);
  const at = after ? after.index : ours.at(-1).index + ours.at(-1)[0].length;
  text = text.slice(0, at) + line + text.slice(at);
  writeFileSync(file, text);
  changed += 1;
  gotos += g;
  reloads += r;
  console.log(`${relative(root, file)}: goto ${g}, reload ${r}`);
}
console.log(`files ${changed}, goto ${gotos}, reload ${reloads}`);
```

它的输出是每个文件一行（`e2e/fixtures/wiki-health.ts: goto 1, reload 0` 起，110 行），末行 `files 110, goto 260, reload 10`。

计数，对整个文件（不按行；oxfmt 会把长的调用拆成几行）：

```python
# One-off (not committed): M12/P1/S1 design 3.4's count of the new form, over whole files (calls may span lines).
import re, sys, glob, os
root = sys.argv[1]
files = sorted(glob.glob(os.path.join(root, 'e2e/stories/**/*.ts'), recursive=True)) + [os.path.join(root, 'e2e/fixtures/wiki-health.ts')]
g = r = 0
old_g = old_r = 0
for f in files:
    t = open(f).read()
    g += len(re.findall(r'(?<![.\w$])goto\(\s*[A-Za-z_$][\w$]*\s*[,)]', t))
    r += len(re.findall(r'(?<![.\w$])reload\(\s*[A-Za-z_$][\w$]*\s*[,)]', t))
    old_g += len(re.findall(r'\.goto\(', t))
    old_r += len(re.findall(r'\.reload\(', t))
print(f'new form: goto {g}, reload {r}; old form: .goto( {old_g}, .reload( {old_r}; files {len(files)}')
```

改之前旧写法 260、10，新写法 0、0；改之后旧写法 0、0，新写法 260、10。

反向比较，以 `479d14e5` 为 `HEAD`：

```python
# One-off (not committed): M12/P1/S1 design 3.4's check that the change is the one transform. Each changed
# file, transformed back (goto(X, -> X.goto(, reload(X) -> X.reload(), the navigation import dropped), equals
# the file at HEAD once every blank is taken out of both.
import re, subprocess, sys
root = sys.argv[1]
names = subprocess.run(['git', '-C', root, 'diff', '--name-only', 'HEAD', '--', 'e2e/stories', 'e2e/fixtures/wiki-health.ts'],
                       capture_output=True, text=True, check=True).stdout.split()
same = differ = 0
for n in names:
    now = open(f'{root}/{n}').read()
    was = subprocess.run(['git', '-C', root, 'show', f'HEAD:{n}'], capture_output=True, text=True, check=True).stdout
    back = re.sub(r'(?<![.\w$])goto\(\s*([A-Za-z_$][\w$]*)\s*,\s*', r'\1.goto(', now)
    back = re.sub(r'(?<![.\w$])reload\(\s*([A-Za-z_$][\w$]*)\s*\)', r'\1.reload()', back)
    back = re.sub(r'^import \{ (goto|reload|goto, reload) \} from "(\.\./\.\./fixtures|\.)/navigation";\n', '', back, flags=re.M)
    strip = lambda s: re.sub(r'\s+', '', s)
    if strip(back) == strip(was):
        same += 1
    else:
        differ += 1
        print('DIFFERS', n)
print(f'{same} of {len(names)} files the same after the reverse transform; {differ} differ')
```

110 个文件都相同（`110 of 110 files the same after the reverse transform; 0 differ`）。`git diff --stat` 是 112 个文件：这 110 个，加 `navigation.ts`、`browser.ts`。

审查 C 用自己的脚本重做一遍，从 `git show <提交>:<文件>` 读，不经工作树：计数相同（头上 271、13，多的 11、3 是 A15）；反向比较 110/110，另做更严的一种，反向之后经 oxfmt 与改之前**逐字节**相同，也是 110/110；每个文件的接收者次序、导入的名字与路径、导入的位置都对；diff 里不含 `goto`、`reload` 的改动只有 `lw6-source-history.spec.ts` 拆行的一行。

## 反向对照

实施者在头 `8e128a2d` 上跑整张表（`tools/mut_e2e.py`、`tools/mut_gate.py`），先在没改的树上跑同样的命令（A15 三轮 60 个通过、3 个跳过；门禁 rc=0），每条之后从 `.mutbak` 复原；审查 B 用自己的脚本重跑 A15 与帮手的各行（改后核对 sha256），审查 C 重跑门禁的各行。A15 的变异在三种浏览器各三轮（`--repeat-each=3`），"3/3"是三轮都失败。

**A15 与帮手**（设计 7.2 的 1、2、3、16、17、18；实施者与 B 的结果相同）：

| # | 变异 | 失败的格（每种浏览器 3/3） | 照过的格 | 怎样失败 |
|---|---|---|---|---|
| 1 | `settle` 直接返回 | 2、4、5、6，三种 | 1、3、7 | 第 2 格在三种、第 4 格在 Chromium 与 Firefox 落到登录页（失败时页面状态第一行 `sent back to sign in: /sign-in?next=… after a refresh answered 401`）；第 4 格在 WebKit 以 `problems` = `["the old document never sent its refresh"]` 失败；第 5 格 `Expected "answered", Received "dropped"`；第 6 格新文档在放开之前就请求了 |
| 2 | `sessionWaits` 不看 `performance` | 4，三种 | 其余 | Chromium、Firefox 落到 `/sign-in?next=%2Fsettings%2Fsecurity`；WebKit 以 `problems` 失败 |
| 3 | 在途的集合不看 | 5，三种 | 其余 | 读被丢掉（"dropped"） |
| 16 | `reload` 不先等 | 2，三种 | 其余 | 落到 `/sign-in?next=%2Fsettings%2Fprofile` |
| 17 | `framenavigated` 不记 `/sign-in` | 1（三种）、3（Chromium、Firefox） | 其余 | `signInLandings` 是 `[]` |
| 18 | 不看锁也不看租约 | 6，三种 | 其余（第 4 格照过，同设计） | 新文档在放开之前就请求了；只钉住锁那一半（P1S1B-M1） |

**门禁**（设计 7.2 的 4–8、19；"探针"是在没改的门禁上改真的树）：

| # | 变异或探针 | 门禁 | 由什么抓到 |
|---|---|---|---|
| base | 没改的树 | rc=0，175 个文件、A15 ×2 | — |
| 探针 | `a5-refresh-reuse.spec.ts:17`、`wiki-health.ts:158` 各改回 `page.goto`；A15 第 2 格也写逃生口；新建 `e2e/perf/` 下跨行的 `.reload()`（B） | 各 rc=1，行号对 | 真的树 |
| 4 | 故事里的直接调用放过 | rc=1 | 内置样例 1 |
| 5 | 遍历只读 `e2e/stories` | rc=1，`the gate did not read e2e/fixtures/navigation.ts` | `e3059737` 的核对。之前（设计的门禁）照过：rc=0、123 个文件，只有在 `wiki-health.ts` 手写一处的探针看得出 |
| 6 | 例外按目录（`e2e/fixtures/`） | rc=1 | 内置样例 2 |
| 7 | 不核对逃生口的表 | rc=1 | 内置样例 4、5、6 |
| 8 | 正则不认跨行 | rc=1 | 内置样例 2 |
| 19 | 帮手模块里的逃生口也数 | rc=1 | 内置样例 7；真的树上帮手的报错信息被算作一处 |

**实施者加的**（设计 7.2 新加 x1–x7 七行）：

| # | 变异 | 结果 |
|---|---|---|
| x1 | `tokenPaths` 去掉注册 | A15 第 7 格三种 3/3 失败：帮手 10 秒后报 "the document's first refresh" |
| x2 | `tokenPaths` 去掉登录（跑 A3、S4 与 A15 各一轮） | A3 (page)、S4 失败（同上的报错）；A15 照过 |
| x3 | `refusedRefresh` 恒为假 | A15 第 1（三种）、3（Chromium、Firefox）格 3/3 失败：落点少了 " after a refresh answered 401" |
| x4 | 落点不去重 | `8e128a2d` 之前 A15 全部照过（因此有那个提交）；之后第 1 格三种 3/3 失败（两条相同的落点） |
| x5 | 事件流也算在途 | 第 5、6、7 格三种 3/3 失败，第 4、2 格多数失败（那一刻流还没连上的照过）；帮手报 `GET /api/v0/events` |
| x6 | 门禁的 `check()` 什么也不报 | rc=1：样例都看不见 |
| x7 | 门禁的遍历读 `web/apps/web/src/session` | rc=1：没读到帮手的模块，表里 A15 一行也对不上 |

**审查者加的**：

- A：`refresh-only`（`tokenPaths` 只有续期，设计原来的写法）A15 第 7 格、A3 (page)、S4 的页面一格都以帮手 10 秒的报错失败；`lock-first`（先看锁再看记录）不是变异，是 A-M1 的改法，`stories/identity/` 69 个通过、1 个跳过。
- B：m16g（`goto` 不先等）第 4、5、6 格三种 3/3 失败；x0（同 `refresh-only`）第 7 格三种 3/3 失败；mK（记录的键改名）、mR（`#root` 改名）第 4、6 格三种 3/3 失败；mN（锁的名字改名）第 6 格三种 3/3 失败；**mL（租约恒交回空）、mLK（租约的键改名）活下来**（P1S1B-M1）。
- C：门禁的 c1–c8：c2（`SKIP` 多一个故事分组）活下来、带探针照过（P1S1C-N4）；c3（按文件名例外）活下来（P1S1C-N5）；c4（逃生口只记有没有）由真的树上 A15 的 2 抓到；c6（行号差一）由样例的整张问题表抓到；c1、c7 活下来、c5、c8 今天等价（"接受与推后的"）。

**修复一轮**（实施者在 `8e128a2d` 的临时 worktree 上跑"改前"，在头 `3c2425c1` 上跑"改后"；第一轮的 A15 表照新代码更新后整张重跑；`tools/mut_gate2.py`）：

| # | 变异 | `8e128a2d` 上 | `3c2425c1` 上（每种浏览器三轮） |
|---|---|---|---|
| m20 | 先看记录再看锁（`8e128a2d` 的次序） | 帮手本身就是这样；第 8 格放进去 9/9 失败 | 第 8 格 9/9 失败："the next document is asked for once the sign-in wrote its record" |
| m18a | 只去掉锁的一支（租约留着） | — | 第 6、8 格各 9/9 失败 |
| m21 | 去掉租约一支（B 的 mL） | 活：60 通过、3 跳过 | 第 9 格 9/9 失败："the next document is asked for once the lease is let go" |
| m22 | 租约的键改名 `nwiki.auth.lease.v2`（B 的 mLK） | 活：60 通过、3 跳过 | 第 9 格 9/9 失败 |
| m23 | 设计的去重（比 `signInLandings.at(-1)`） | 活：60 通过、3 跳过 | 第 1 格 9/9 失败：少了第二条落点 |
| g1 | 门禁的 `SOURCE` 改回四种结尾 | — | rc=1："the gate does not read a story ending .spec.tsx"（`.cts`、`.jsx`、`.cjs` 同） |
| g1b | `SOURCE` 少 `x`（`/\.[cm]?[jt]s$/`） | — | rc=1：`.spec.tsx`、`.spec.jsx` |
| g2 | `LEAVE` 只认双引号 | — | rc=1："the gate does not see an escape written with backticks" |
| c3 | 例外按名字 `path.endsWith("/navigation.ts")` | 七个样例都过，`e2e/perf/navigation.ts` 的探针没看见 | rc=1："the gate does not see a direct goto in another module named like the helper" |

第一轮的行在头上：1 第 2、4、5、6、8、9 格；2 第 4 格；3 第 5 格；16 第 2 格；17 第 1、3、8 格；18 第 6、8、9 格；x1 第 7 格；x2 A3、S4 与 A15 第 8 格；x3 第 1、3 格；x4 第 1 格；x5 第 2、4–9 格（多数 9/9）。每条只在该失败的格失败，其余格三轮全过。门禁的第一轮的 15 条（4–8、19、x6、x7 与三个探针）在新门禁上与第一轮相同；审查之前的门禁对 `.spec.tsx`、`.spec.cts`、`.spec.jsx` 里的 `page.goto` 与反引号的逃生口都 rc=0、看不见，改后各 rc=1。

## 核对

**修复核对一轮**（2026-11-15）：三位各把自己的工作树挪到 `3c2425c1`，重的经 `heavy.sh`，之后工作树干净、没有 `*.mutbak`。都没有 Important、Minor。

- **A**：M1、M2、N1 关上。审查的探针在头上三种浏览器都绿：登录 200，此时记录为空、`pending` 里有 `nwiki.auth.refresh`；新文档在放开之后 51 / 36 / 134 ms 才请求（Chromium / Firefox / WebKit），落点只有起初那一条。把 `navigation.ts` 换回 `8e128a2d` 的（从备份复原），探针三种都失败（放开之前 987–996 ms 就请求了），第 8 格 9/9 失败；第 8 格先断言"答 200 之后记录还是空的"，把交错钉实了。A15 第 8 格三轮 9/9 通过，`stories/identity/` 75 个通过、1 个跳过。跟着别的标签页登录的探针在头上帮手 61–214 ms 就换了。新 Nit P1S1A-N2。
- **B**：M1、N1、N2 关上。A15 九格三种各三轮 78 个通过、3 个跳过（49 秒；第 8、9 格各 1.4–1.9 秒）；m21、m22 只有第 9 格 9/9 失败，m18a 第 6、8 格 9/9 失败。五个提交逐个检出，`make lint-web`、`make knip`、`pnpm -r run check:types` 都是 0，门禁每次 175 个文件、A15 ×2。全量 `make e2e` 565 个（564 个通过、1 个跳过），148 秒，第 50 条那一行本机照实打出（1173 ms）。CI run 43642007840 逐个测试与上次比，A15 之外 502 个的中位数 1.48 倍（四分位 1.29–1.68），A15 在 CI 上 26 个串行 68.7 秒，新加的 6 个共 15.8 秒；第 50 条那一行没有。新 Nit P1S1B-N3。
- **C**：M1、N1–N7 关上（N4 的代码部分、N5、N6、N7 的代码；M1、N1–N3 看修复报告第 6 节的文档稿）。第一轮的 16 个探针在新门禁上都报出；门禁的 20 条变异里，第一轮的 4–8、19 与 c4、c6 照旧被抓，c1、c7 照旧活（同第一轮，不是发现）；沿代码走 N6 的那一格：设计的写法第二次回到时不记，得一条，这格失败。五个提交从 `git archive` 出来的树上门禁各 rc=0；替换的 110 个文件本轮没动。新 Nit P1S1C-N8（= A-N2）、C-N9。

一轮之后没有 Important、Minor 留下，不开第二轮修复；新的三条 Nit 照"主持者的决定"处置。

## 实测

本机 8 核 arm64，4 个 worker，都经 `heavy.sh`：

- **全量 `make e2e`**：`179a7aed` 538 个通过（Playwright 2.3 分钟，墙钟含 `make build` 142 秒）；`bafbd418`（帮手与 110 个文件，没有 A15）538 个、140 秒（B：137 秒）；审查时的头 `8e128a2d` 559 个（558 个通过、WebKit 的 A15 第 3 格跳过）、2.5 分钟、149 秒（B：143 秒）。559 = 538 + 21。
- **帮手对已有故事多花的时间**（B 插记录的全量，559 个）：帮手被调 485 次（Chromium 340、Firefox 73、WebKit 72），A15 之外 450 次；真的等了的只有 6 次（N7、PG13、LW3、LW6 ×3，都是在途的读或写），每次 30–55 毫秒，合计 0.23 秒；不用等的 467 次读页面合计约 2.7 秒。A15 之外第一次续期与锁从没拦过帮手；没有一次读到租约。
- **A15 各格**（`8e128a2d` 上三轮，每格每种浏览器三次的最小–最大，秒，含注册与登录）：

  | 格 | Chromium | Firefox | WebKit |
  |---|---|---|---|
  | 1 逃生口、答复在途 | 0.5 | 1.0–1.2 | 0.7–0.8 |
  | 2 帮手、答复在途 | 2.3–2.4 | 2.4–2.5 | 2.7–2.8 |
  | 3 逃生口、第一次续期没发 | 0.3 | 0.4–0.5 | 跳过 |
  | 4 帮手、第一次续期没发 | 3.3–3.4 | 3.4 | 3.7 |
  | 5 帮手、读在途 | 2.3 | 2.4 | 2.7 |
  | 6 帮手、锁被持住 | 1.3 | 1.4–1.5 | 1.6–1.8 |
  | 7 帮手、自己注册的文档 | 0.4 | 0.6–0.7 | 0.7–0.8 |

  七格一轮串行约 35 秒；三轮 Playwright 报 40 秒。B 的三轮 39.7 秒，8 个 worker 六轮 44.3 秒、最慢的第 4 格 WebKit 4.0 秒。没改过的帮手上 A15 共 220 次通过、0 次失败（B）。
- **修复一轮之后**（头 `3c2425c1`）：全量 565 个（564 个通过、1 个跳过），实施者 151 秒、核对 B 148 秒；565 = 538 + 27。A15 九格三轮见"门禁与 CI"。
- **A15 第 1、3 格的失败的请求与控制台**（实施者的诊断运行，三种浏览器各两次）：与设计 3.6 的表里两个原型看到的相同。第 1 格 `apiFailures` 在 Chromium 是 `["401 POST /api/v0/auth/refresh"]`，Firefox 是 `["NS_BINDING_ABORTED POST /api/v0/auth/refresh", "401 POST /api/v0/auth/refresh"]`，WebKit 是 `["Load request cancelled POST /api/v0/auth/refresh", "401 POST /api/v0/auth/refresh"]`；第 3 格在 Chromium 与 Firefox 都只有那条 401。控制台的 401 一行在 Chromium 与 WebKit 有、Firefox 没有；`signInLandings` 都是一条，带 " after a refresh answered 401"。
- **第 50 条的正向对照**（设计 3.7，5 号提交之前在实施者的工作树上，各次从备份复原）：`tentative()` 第一次交回两行固定的文字、之后交回空，看到 `global setup: waited 101 ms for the host's tentative IPv6 addresses (inet6 fe80::1/64 scope link tentative |     valid_lft forever preferred_lft forever)`；恒交回一行，看到 `…waited 5019 ms …, some still tentative at the 5 s deadline`；恒交回 `undefined`，看到 `global setup: ip could not tell the host's tentative IPv6 addresses; waiting 2999 ms from PostgreSQL's start`；恒交回空，没有这一行。复原之后在本机每次都真的等了（冒烟 S1 四次 0.5–1.1 秒、全量两次 0.85–0.96 秒、B 的两次 0.2–0.3 秒，都是容器 veth 的链路本地地址），这一行照实出现；设计的"复原之后看不到"在本机做不出（P1 设计 3.15 第 8 条）。

## 门禁与 CI

- **逐个提交**（实施者在干净的临时 worktree 里，整个经 `heavy.sh`；B 在自己的工作树里另跑一遍）：8 个提交各自 `make lint-web`、`make knip`（B 另加 `pnpm -r run check:types`）都是 0，之后没有已跟踪的改动；碰到的故事：`479d14e5` A6 4 个，`bafbd418` 全部 538 个，`6cf1c32c` 身份、冒烟与 W9 59 个，`ce2cc0fc` 与 `8e128a2d` A15 20 个通过、1 个跳过，`c330217f` 冒烟 S1 3 个。门禁那一行：`c18562fe`、`6cf1c32c` 174 个文件、逃生口 0；之后 175 个文件、A15 ×2。
- **合并之前的门禁**（实施者，头 `8e128a2d`，工作树干净，每项之前 `df` 约 40G 可用）：`make lint gen-check knip build` 9 秒通过；经 `heavy.sh`：`make e2e` 149 秒，559 个（558 个通过、1 个跳过）；A15 三轮 40 秒，60 个通过、3 个跳过。不跑 `make test`、`make test-go-short`：S1 不改服务端。
- **分支的 CI**（`8e128a2d`，run 43632495423）：19 项第一次 attempt 全绿，没有重跑。e2e 作业 7:35（E2E 一步 6:03），559 个（558 个通过、1 个跳过），2 个 worker，Playwright 报 5.9 分钟；作业限 20 分钟（`ci.yml:263`），余 12 分 25 秒。S1 之前 `main` 的 e2e：`179a7aed` 7:21（E2E 6:10）、`a22adfd2` 8:16（6:51）、`b54cd679` 6:48（5:16）：帮手与 A15 多出的时间在机器的差别之内。A15 在 CI 上一轮串行约 41 秒。e2e 作业日志里 "Running 559 tests using 2 workers" 之前有一行 `global setup: waited 423 ms for the host's tentative IPv6 addresses (6: veth40e6840@eth0: … inet6 fe80::10f8:7dff:fe0c:d124/64 scope link tentative …)`：第 50 条的等待在 runner 上触发了一次。其余：web 4:03；八份组合根的 Test 一步 4:20、5:39、6:50、4:42、5:07、4:34、5:31、4:24（S1 不改服务端）。
- **修复一轮，逐个提交**（实施者在从 `8e128a2d` 起的干净临时 worktree 里，`tools/percommit-fix1.sh`，经 `heavy.sh`；`bin/nervewiki` 拷来，本轮不改 `web/`、`server/`）：五个各自 `make lint-web`（含两个门禁、oxlint、oxfmt 与 `pnpm -r run check:types`）、`make knip` 都是 0；故事：`6ec05d13` 全部 561 个通过、1 个跳过，`995ebf02` 与 `6e48227d` A15 26 个通过、1 个跳过，`3c2425c1` 全部 564 个通过、1 个跳过；`91718e48` 的门禁 175 个文件、A15 ×2。
- **修复一轮，门禁**（头 `3c2425c1`，干净的树，根分区 40G 可用）：`make lint gen-check knip build` 8 秒；经 `heavy.sh`：`make e2e` 151 秒，565 个（564 个通过、1 个跳过），第 50 条那一行照实打出（941 ms）；A15 三轮 50 秒，78 个通过、3 个跳过，每格（三种浏览器，秒）：第 1 格 0.5–1.2，第 2 格 2.3–2.8，第 3 格 0.3–0.5，第 4 格 3.3–3.8，第 5 格 2.3–2.8，第 6 格 1.3–1.8，第 7 格 0.4–0.8，第 8 格 1.4–1.9，第 9 格 1.4–1.8。
- **修复一轮，分支的 CI**（`3c2425c1`，run 43642007840，attempt 1）：19 项全绿，e2e 没有卡在 apt 镜像。e2e 作业 9:58（E2E 一步 8:25），565 个（564 个通过、1 个跳过），2 个 worker，余量 10 分钟。比 run 43632495423 慢在 runner：A15 之外每个测试的用时中位数是上次的 1.45 倍（实施者，335 个，四分位 1.26–1.63；核对 B 重算 502 个，1.48 倍），不经帮手的纯接口测试约 1.5 倍（标题带 `(API)` 的 152 个，中位数 1.53）。A15 在 CI 上一轮串行约 68 秒（上次七格 42 秒）。第 50 条那一行没有打出：那台 runner 没有暂定的地址。其余：web 5:26；八份组合根的 Test 一步 4:29、4:15、4:49、5:38、6:21、4:33、5:20、4:27；race 15:03，costs-1 11:47，costs-2 14:16，costs-3 14:35，costs-lint 10:33，guards-core 15:51，guards-sources 14:05，short 3:08，image 1:19。
- **主持者的 `937df215`**（只改 `pageState` 的说明）：run 43645603202（attempt 1），19 项全绿。e2e 作业 8:48（E2E 一步 7:15），565 个（564 个通过、1 个跳过），2 个 worker；第 50 条那一行打出（waited 733 ms）。八份组合根的 Test 一步 6:30、6:25、5:28、5:03、7:06、3:54、4:33、3:54（S1 不改服务端）

## 环境

实施与审查各在自己的工作树里，`TMPDIR` 与日志在 `~/.cache` 下各自的目录，锁固定在 `/tmp`，用完删掉；没有手动起容器，没有修剪缓存，没有往外发东西。要留的汇总在设计工作目录的 `m12/p1s1/`（`logs/`、`review/a/`、`review/b/`、`review/c/`）。
