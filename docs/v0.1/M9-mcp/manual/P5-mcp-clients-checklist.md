# M9/P5 人工验收：真实的 MCP 客户端

| 项 | 内容 |
|---|---|
| 来源 | [M9 总设计](../00-M9-design.md) 4.16"人工的"；[M0/P1 MCP 会话的移交](../handoffs/M0-P1-mcp-sessions.md)第 1、3 项；[P1](../01-P1-platform-reads.md)、[P2](../02-P2-writes-history.md)、[P3](../03-P3-batch-revert.md)、[P4](../04-P4-grep-search.md) 各自第 8 节给 P5 的清单的几步；[P5](../05-P5-stories-docs.md) 4.2（README 里客户端的每一句与核对它的步）、第 5 节（反向代理）；每一步出自哪里见 P5 设计 2.3 |
| 执行人 | 负责人，在 v0.1 的最后与[输入法清单](../../M4-pages/manual/P6-ime-checklist.md)一起（M9 总设计第 11 节的取舍第 12 条）；负责人同意时，P5 的实施者可以先跑一遍 |
| 客户端 | Claude Code 与 Codex 的当前正式版，各一遍。开始之前记下 `claude --version`、`codex --version` 与日期 |
| 结果 | 每一步每个客户端：通过、不通过或"记下"（只要原文的步），写在"记下的"一栏，原文照抄（截到一两行）。整份写进 M9 收尾审查记录的"人工验收"一节。**通过**：每一步不是"记下"的预期都成立，或者每个不成立的旁边写着负责人的处置（照 P5 设计 4.2 改 README、交 M12，或改服务端）；M9 在这份清单通过之后才改为已完成。README 里客户端的句子（P5 设计 4.2 的表）照记下的改，改的提交写明照哪一步 |

这份清单补上自动化测试做不到的：e2e 的 MC1 经 TypeScript 官方 SDK 的客户端连接，整个程序的测试用官方 Go SDK 的客户端（M9 总设计 4.16"自动的"），它们不是 Claude Code 与 Codex，也没有模型、审批与界面。

**怎样读"预期"**：服务端的答复、日志与库里的结果由代码定；客户端与 agent 的做法都未核对（这份清单就是核对它们的），出自客户端的文档的标"照文档"，M0/P1 的实验记下过的标"M0/P1"。不符时照实记下：多半是那一句写错了，不是客户端坏了。

## 不碰你自己的客户端配置

整份清单不改 `~/.claude.json` 里的 MCP 服务、`~/.claude/settings.json`、`~/.codex/` 与任何项目里已有的 `.mcp.json`：

- **Claude Code**：一律在 `$CC`（"准备"第 5 步的临时目录）里以 `claude --mcp-config "$CC/<文件>" --strict-mcp-config` 启动：只用这个文件里的服务（照文档；插件带来的服务与 claude.ai 的连接器是否也不起未核对，A1 看 `/mcp` 的列表）。不在别的目录里执行 `claude mcp add`（本地与用户的范围写 `~/.claude.json`）；第 A3 步只在 `$CC` 里以 `--scope project` 写那个目录自己的 `.mcp.json`，前后各数一次 `~/.claude.json` 里的 `"nervewiki"` 核对它。不执行 `claude mcp list`：它会连一遍你配置过的每个服务。Claude Code 照常在 `~/.claude.json` 里记下它进过的目录（信任与历史），在 `~/.claude/projects/` 下记每个会话（那些目录怎样起名未核对）：那是它自己的状态，不是 MCP 的配置，可以留着；要删时照"做完"的第 4 步，只删列出的那一个。审批的几步在只给它们的临时目录 `$CCD` 里做（第 D 部分的开头），用 `$CCD/.claude/settings.json`（项目的设置），不改你自己的设置。**开始之前记下**你自己的 `~/.claude/settings.json` 里的 `permissions.defaultMode` 与提到 `mcp__` 的放行或拒绝的规则：它们会改变第 D 部分的结果。
- **Codex**：一律在一个临时的 `CODEX_HOME` 里、从一个空的临时目录 `$CX` 启动（"准备"第 6 步）：Codex 只读 `$CODEX_HOME/config.toml`，你的 `~/.codex/config.toml` 里的别的 MCP 服务不会起（照文档；`codex -c mcp_servers.x.url=…` 是叠在 `~/.codex/config.toml` 之上的，那样它们照样起）；从空的目录启动，它不会拿到某个项目自己的配置，也不会在你的代码里读写文件。登录，按这个次序选：
  1. **用 API 密钥**（推荐）：`printenv OPENAI_API_KEY | codex login --with-api-key`（照文档），凭证写在临时的 `CODEX_HOME` 里，与你自己的登录无关。
  2. **在临时的 `CODEX_HOME` 里另登录一次**：`codex login`（ChatGPT 的账户）。做完之后在一个没有设 `CODEX_HOME` 的终端里启动你自己的 Codex，记下它是否照常可用；不能用时在那个终端里（你自己的 `~/.codex`）`codex login` 再登录一次。第二次登录会不会让第一次的失效未核对。
  3. **最后才用：复制 `auth.json`**。复制时记下原来的那一份的指纹：`shasum ~/.codex/auth.json`，写进"记下的"。清单进行的期间**不要在别处用 Codex**（别的终端、编辑器的插件）：任何一边的一次续期都会轮换刷新令牌，另一边的那一份随之作废。做完时（"做完"的第一步，在删掉临时目录之前）：原来的那一份的指纹没变、临时的那一份变了，就把临时的复制回 `~/.codex/auth.json`；原来的那一份变了，不复制；两份都变了，其中一份已经作废，在你自己的环境里 `codex login` 再登录一次。
- **令牌**：给每个客户端建它自己的 PAT（"准备"第 3 步），各放在自己的变量里：Claude Code 的终端里 `NERVEWIKI_CLAUDE_TOKEN`，Codex 的终端里 `NERVEWIKI_CODEX_TOKEN`，都以 `read -rs <变量> && export <变量>` 设（粘贴、回车：不回显，不进 shell 的历史）。做完在网页上撤销它们。

## 准备

1. 起一个实例：`make dev-db`、`make build`。dev 不配置签名私钥时每次启动换一把，重启之后浏览器都要重新登录，所以在服务的终端里先生成一把、整份清单都用它（README"签名私钥"）：

   ```sh
   export KEYS=$(mktemp -d)
   openssl genpkey -algorithm ed25519 -out "$KEYS/jwt.pem"
   export NWIKI_AUTH__JWT__PRIVATE_KEY_FILE="$KEYS/jwt.pem" NWIKI_SERVER__PUBLIC_URL=http://127.0.0.1:8080
   make run
   ```

   （`make run` 提供上一次 `make build` 的前端；环境变量照常传下去。）之后的重启（E6、G1、G2）都在这个终端里 `make run`。下面 `$BASE` 是 `http://127.0.0.1:8080`。这个终端里看服务的日志：每个请求一行 `http request`（方法、路径、状态、用时 `duration`），每次 MCP 调用一行 `MCP tool call`，握手一行 `MCP initialize`（带客户端名称），大的请求体没有名额时一行 `large request bodies are saturated`。
2. 两个账户：A（你，工作区的管理员）与 B（同一工作区的成员）。B 的加入：A 在工作区的成员页邀请 B 的邮箱（README"邀请"），B 在另一个浏览器或无痕窗口里注册（dev 开放注册）并接受邀请。A 建两本笔记本，**一个客户端一本**：`MCP 清单 CC`（Claude Code 用）与 `MCP 清单 Codex`（Codex 用），B 是两本的编辑者。第 C、H 部分改写种下的页：若两个客户端在同一本里做，后一个会看到前一个改过的页，所以一个客户端一本。笔记本的 id 是网页地址里 `/notebooks/` 之后的一段，下面写 `$NB_CC` 与 `$NB_CODEX`；只说 `$NB` 的步，用这个客户端自己的那本。
3. A 在"设置"→"访问令牌"（`$BASE/settings/tokens`）建三个 PAT：`seed`（下一步用）、`claude-checklist`、`codex-checklist`。
4. 种下页与附件，两本笔记本各一次：
   - 准备三个文件，每个的字节都要是它的名字说的类型：服务端按开头的字节判断，名字是 `.png` 而字节是 JPEG 的附件存成 `application/octet-stream`，不直接放进答复（`asset/domain/types.go` 的 `TypeOf`）。`small.png` 是 PNG、每边不过 1,000 像素、几十 KB（例如一张截图以 `sips -Z 800 -s format png 截图.png --out small.png` 缩小）；`wide.png` 是 PNG、宽 2,500–7,000 像素、2.0–2.8 MiB（JPEG 或别的格式先转成 PNG：`sips -s format png 照片.jpg --out wide.png`，太大时加 `-Z` 与一个像素数缩小再转；以 `sips -g pixelWidth -g pixelHeight wide.png` 与 `ls -l` 核对：3 MiB 以上的不直接放进答复）；`doc.pdf` 是一个小的 PDF。做好之后 `file small.png wide.png doc.pdf`：前两个是 "PNG image data"，第三个是 "PDF document"。
   - 把下面的脚本存成临时目录里的 `seed.mjs`，`read -rs SEED_TOKEN && export SEED_TOKEN`（`seed` 的令牌），然后 `node seed.mjs "$BASE" "$NB_CC" small.png wide.png doc.pdf` 与 `node seed.mjs "$BASE" "$NB_CODEX" small.png wide.png doc.pdf`（Node 22 起；它只经 REST 建页、上传附件）：

   ```js
   // seed.mjs：在一本笔记本里种下这份清单要的页与附件（经 REST，令牌取自 SEED_TOKEN）
   import { openAsBlob } from "node:fs";
   const [base, notebook, small, wide, pdf] = process.argv.slice(2);
   const auth = { Authorization: `Bearer ${process.env.SEED_TOKEN}` };
   async function page(title, content, parent = null) {
     const r = await fetch(`${base}/api/v0/notebooks/${notebook}/pages`, {
       method: "POST",
       headers: { ...auth, "Content-Type": "application/json" },
       body: JSON.stringify({ parent_id: parent, title, content }),
     });
     if (!r.ok) throw new Error(`${title}: ${r.status} ${await r.text()}`);
     return (await r.json()).id;
   }
   async function asset(file, name, parent) {
     const form = new FormData();
     form.set("parent_id", parent);
     form.set("file", await openAsBlob(file), name);
     const r = await fetch(`${base}/api/v0/notebooks/${notebook}/assets`, { method: "POST", headers: auth, body: form });
     if (!r.ok) throw new Error(`${name}: ${r.status} ${await r.text()}`);
   }
   const han = "天地玄黄宇宙洪荒日月盈昃辰宿列张寒来暑往秋收冬藏";
   const cut = "甲".repeat(500) + "FIXME 在这里" + "乙".repeat(500) + "行尾在此";
   await page("AGENTS", "---\ntype: agents\n---\n这份清单的暗号是青石板。\n\n回答用中文。\n");
   const guide = await page("Guide", "# Guide\n\n他说“先读再写”，然后离开。\n重叠：aaa\n要删的一行\n- [ ] 一件事\n");
   await page("长文", Array.from({ length: 800 }, (_, i) => `第 ${i + 1} 行：${han}`).join("\n") + "\n");
   await page("命中", Array.from({ length: 600 }, (_, i) => `TODO 第 ${i + 1} 项：${han.repeat(7)}`).join("\n") + "\n" + cut + "\n");
   await page("长行", "x".repeat(4 << 20) + "完成\n");
   await page("锁", "被编辑的页。\n", guide);
   const pictures = await page("图库", "");
   await asset(small, "图一.png", guide);
   await asset(wide, "宽图.png", guide);
   await asset(pdf, "说明.pdf", guide);
   for (let i = 1; i <= 21; i++) await asset(small, `p${String(i).padStart(2, "0")}.png`, pictures);
   console.log("seeded");
   ```

   - `长文.md` 约 69 KB（约 2.2 万个汉字与全角的标点）：一次 `read` 给不完。
   - `命中.md` 前 600 行都有 `TODO`，每行约 520 字节：`grep` 的一次答复放 85 行就到 48 KiB，在 `limit` 的默认 100 之前；第 601 行约 3,000 字节，`FIXME` 在它的中间，`grep` 只给 `FIXME` 前后共约 1,000 字节（`cut`），行尾的"行尾在此"不在其中。
   - `长行.md` 是 4 MiB 的一行，`\pL{30}完成` 这样的模式只匹配得到它的开头（约 2.2 MB），这一行进 `partial_lines`。
5. Claude Code 的配置，在一个新的临时目录 `$CC` 里（`export CC=$(mktemp -d); cd "$CC"`）。每一份都是 README"接入 agent（MCP）"的 `.mcp.json` 样例原样照抄，只把 `https://wiki.example.com` 换成 `http://127.0.0.1:8080`（或下面说的地址）、`<笔记本 id>` 换成 `$NB_CC` 的值（文件里写字面的 id）：
   - `$CC/mcp.json`：笔记本入口；
   - `$CC/account.json`：账户入口 `http://127.0.0.1:8080/mcp`；
   - `$CC/throttled.json`：账户入口，经第 F2 步的限速转发 `http://127.0.0.1:8081/mcp`；
   - `$CC/notoken.json`：同 `mcp.json`，去掉整个 `headers`（第 E1 步）。

   在这个终端里设 `NERVEWIKI_CLAUDE_TOKEN`（`claude-checklist` 的令牌，`read -rs`）。
6. Codex：`export CODEX_HOME=$(mktemp -d) CX=$(mktemp -d); cd "$CX"`，照上面的"不碰你自己的客户端配置"登录，把 README 的 `config.toml` 样例原样写进 `$CODEX_HOME/config.toml`，同样只换地址与 `<笔记本 id>`（`$NB_CODEX` 的值）；在这个终端里设 `NERVEWIKI_CODEX_TOKEN`（`codex-checklist` 的令牌，`read -rs`）。每次改了 `config.toml`，退出 Codex、再在 `$CX` 里启动。

## 步骤

下面"让 agent……"是在客户端的会话里用自然的话说；不要告诉它工具的参数，看它照工具的说明与服务端的说明自己怎样做。两个客户端都是能读写本地文件的 agent：每个会话的第一句话先说"页与附件都在 Nerve Wiki 的这本笔记本里，只经 nervewiki 的工具读写，不读写本地的文件"（只说是哪个服务，不说参数）。

在笔记本入口，种下的 `AGENTS` 页整个取代内置的说明（README"MCP"的"说明"）。C 部分里照说明做的几步（C3 的引号、C5 的唯一、C6 的完整路径、C12 与 C15 的 `partial_lines`、`cut` 与 `column`）靠的是工具自己的说明（`api/mcp/tools.json`），不受影响；F 部分要内置的说明里"大的写一个接一个地发"那一句（只在内置的说明里），所以 F 在账户入口做：它的握手照常给内置的说明，`open_notebook` 另给 `AGENTS` 页的正文。

### A：Claude Code 接上

| # | 操作 | 预期 | 记下的 |
|---|---|---|---|
| A1 | 在 `$CC` 里 `claude --mcp-config "$CC/mcp.json" --strict-mcp-config`，输入 `/mcp` | `/mcp` 只列出 `nervewiki`、已连上、14 个工具（照文档；列出别的服务时记下：它来自插件或 claude.ai 的连接器）；服务的日志里一行 `MCP initialize`，记下它的客户端名称。日志里若是 401：这个文件里的 `${NERVEWIKI_CLAUDE_TOKEN}` 没有展开（照文档它展开）：记下，之后改用 `$CC/mcp-literal.json`（同一个文件，令牌照字面写进去），`account.json` 与 `throttled.json` 也同样写成字面的（`account-literal.json`、`throttled-literal.json`，A5 与 F 用它们），三份都 `chmod 600`，"做完"删掉 | |
| A2 | 让 agent 说出"这本笔记本的说明里的暗号" | 它答"青石板"（握手时的说明是 `AGENTS` 页去掉 frontmatter 的正文；M0/P1 记下 Claude Code 收到了说明），不调用工具 | |
| A3 | 先数一次你的 `~/.claude.json` 里的 `"nervewiki"`：`grep -o '"nervewiki"' ~/.claude.json` 每处列一行，记下几行。在 `$CC` 里执行 README 的项目范围的命令，地址照第 5 步换：`claude mcp add --scope project --transport http nervewiki "http://127.0.0.1:8080/mcp?notebook=$NB_CC" --header 'Authorization: Bearer ${NERVEWIKI_CLAUDE_TOKEN}'`（双引号里的 `$NB_CC` 由 shell 展开，单引号里的令牌一段原样）；然后 `grep nwk_pat_ .mcp.json`，再 `claude --mcp-config "$CC/.mcp.json" --strict-mcp-config`，输入 `/mcp`；退出之后再数一次 `"nervewiki"` | `$CC/.mcp.json` 里是 `${NERVEWIKI_CLAUDE_TOKEN}` 原样、`grep` 什么也找不到（照文档）；连上，14 个工具（`${…}` 由 Claude Code 展开，照文档）。记下 `ls -a "$CC"` 里这条命令另外写了什么。两次数的相同：命令没有把服务写进你的 `~/.claude.json`（它的结构仓库里没有记录，数的只是出现的次数，未核对）。多了时记下，在 `$CC` 里执行 `claude mcp remove nervewiki --scope local`（未核对）去掉它，再数一次，仍然多时也记下；不要动你原有的 `nervewiki`（第一次数的不是 0 时） | |
| A4 | 只把 `$CC/mcp.json` 里的 `${NERVEWIKI_CLAUDE_TOKEN}` 换成一个不存在的变量 `${NO_SUCH_TOKEN}`，启动，`/mcp`；之后改回 | 记下 Claude Code 怎样对待没有设的变量（报错、留空、照原样发出）与服务端的日志（401？） | |
| A5 | `claude --mcp-config "$CC/account.json" --strict-mcp-config`，让 agent 列出它能用的笔记本，再打开 `MCP 清单 CC` 并说出暗号 | 16 个工具；它先调 `notebooks`、再调 `open_notebook`（说明在它的答复里），答"青石板"；之后的调用都带 `notebook` | |
| A6 | 第 C 部分做过之后，网页上打开两本笔记本的历史，按"MCP"筛选 | `MCP 清单 CC` 里 agent 的变更集显示"MCP（<A1 记下的名称>）"，`MCP 清单 Codex` 里的显示"MCP（<B1 记下的名称>）" | |

### B：Codex 接上

| # | 操作 | 预期 | 记下的 |
|---|---|---|---|
| B1 | 在设了临时 `CODEX_HOME` 与 `NERVEWIKI_CODEX_TOKEN` 的终端里，在 `$CX` 启动 `codex`，输入 `/mcp` | 列出 `nervewiki` 与它的 14 个工具（照文档）；服务的日志里一行 `MCP initialize`，客户端名称 `codex-mcp-client`（M9 总设计 4.5） | |
| B2 | 让 agent 说出"这本笔记本的说明里的暗号" | 它答"青石板"；记下它有没有调用工具（读 `AGENTS.md` 得到的不算握手的说明：Codex 是否把服务端的说明交给模型未核对） | |
| B3 | 把 `config.toml` 的 `url` 换成账户入口 `http://127.0.0.1:8080/mcp`，重启 Codex，做 A5（打开 `MCP 清单 Codex`）；之后换回笔记本入口 | 同 A5 | |
| B4 | 退出 Codex，`unset NERVEWIKI_CODEX_TOKEN`，再启动 | 记下 Codex 怎样对待没有设的 `bearer_token_env_var`（不起这个服务并报错，或不带令牌连接、得到 401）；之后设回 | |

### C：读与写（两个客户端各做一遍，各在自己的笔记本里，笔记本入口）

| # | 操作 | 预期 | Claude Code 记下的 | Codex 记下的 |
|---|---|---|---|---|
| C1 | 让 agent 读 `Guide.md`，原样告诉你第 3 行 | 它答`他说“先读再写”，然后离开。`：`read` 的正文是文字块，没有 `structuredContent`（M9 总设计 4.7），客户端交给了模型 | | |
| C2 | 让 agent 在 `计划/本周/周一.md` 写一页（几行字） | 一次 `write`，缺的 `计划`、`计划/本周` 一并建成；网页的树里有三页，历史里一个变更集 | | |
| C3 | 让 agent 把 `Guide.md` 里"先读再写"改成"读过再写"（引号是弯引号） | 它照 `read` 给的字节写 `old_string`，一次成功；若它写成直引号得到 `not found`，记下它之后怎样做（重读、照提示改） | | |
| C4 | 让 agent 删掉 `Guide.md` 里"要删的一行"那一行 | 记下它的 `old_string` 与 `new_string`：只删文字、留下一个空行，还是连换行一起删（服务端只按字节；Claude Code 自己的 Edit 在 `new_string` 为空时连后面的换行一起删，M9 总设计 4.10）；结果的正文 | | |
| C5 | 让 agent 把 `Guide.md` 的 `aa` 换成 `bb` | `aa` 在 `aaa` 里出现两处（重叠也算），答 `old_string` 的 `found 2 times`；记下它是加上下文、带 `replace_all`，还是放弃 | | |
| C6 | 让 agent 把 `计划/本周/周一.md` 移到 `计划` 之下 | 它给 `move` 的 `to` 是完整的新路径 `计划/周一.md`；若它先写成 `计划`（"移进去"）得到错误，记下它之后怎样做 | | |
| C7 | 让 agent 从头读完 `长文.md`，说出最后一行 | 第一次 `read` 约 45 KiB（中文约 1.5 万字）、带 `next_offset`，它接着读完，答"第 800 行：…"；记下第一次的答复有没有被客户端截断或拒绝（Claude Code 的约 25,000 个 token，照文档） | | |
| C8 | 让 agent 读 `Guide/图一.png`，说出图里有什么 | 答复带图片，它说得出图里的内容；读 `Guide/说明.pdf` 时答复带 `http://127.0.0.1:8080/` 起头的下载地址 | | |
| C9 | 让 agent 读 `Guide/宽图.png`（2.0–2.8 MiB） | 图片直接在答复里（3 MiB、每边 8,000 像素之内）；记下客户端是否把它算进自己的输出上限、截断或拒绝 | | |
| C10 | 让 agent 依次读完 `图库/` 下的 21 张图，再读 `Guide/宽图.png`，然后接着聊一句 | 记下读过 20 张以上、其中一张宽过 2,000 像素之后，之后的请求会不会被模型的接口拒绝（一个请求里 20 张以上的图片时每张至多 2,000 像素，未核对） | | |
| C11 | 让 agent 用 `grep` 找 `TODO`，告诉你一共几处 | 第一次答复到 48 KiB 停下：85 处、带 `next_cursor`（不带 `limit`、或带的 `limit` 不小于 85 时都是这样；小于 85 时先到 `limit`，记下它带的 `limit`）；它接着调用（不带 `limit` 时共 8 次，最后一次 5 处），答 600 处；记下第一次的答复是否整个交给了模型 | | |
| C12 | 让 agent 用 `grep` 找 `\pL{30}完成` | 答复没有命中、`partial_lines` 列出 `长行.md` 第 1 行；记下它是否照说明以 `read`（`offset`、`column`）去读那一行的后面 | | |
| C13 | 让 agent 搜索"宇宙"，再搜索"宙" | 第一次以 `search` 找到 `长文.md`、`命中.md`；单个汉字只找标题，第二次什么也找不到或只有标题里有它的；记下它分不分得清 `search` 与 `grep` | | |
| C14 | 让 agent 做一次"整理"：在 `收件箱` 下建两页、改 `Guide.md` 的一行、把 `Guide/锁.md` 改名为 `Guide/已锁.md`，作为一次带说明的 `batch` | 一个变更集，历史里显示它的说明；网页上整组撤销它之后树与正文回到之前（`Guide/锁.md` 又是这个名字） | | |
| C15 | 让 agent 用 `grep` 找 `FIXME`，告诉你那一行最后的四个字 | 一处命中（`命中.md` 第 601 行），`cut: true`、带 `column`：给出的只是 `FIXME` 前后约 1,000 字节；它要再读那一行才答得出"行尾在此"：记下它是否照说明以 `read`（`offset` 601，必要时 `column`）读，还是猜 | | |

### D：审批

每一步记下：调用之前问没问、问的文字、选了"不再问"之后是否不再问。D1–D3 只对 Claude Code，D4–D8 只对 Codex，都在笔记本入口。

Claude Code 的 D1–D3 在一个只给它们的新的临时目录 `$CCD` 里做，免得 A、C 里答过的"不再问"与 D2 的放行带进别的步：

```sh
export CCD=$(mktemp -d)
cp "$CC/mcp.json" "$CCD/mcp.json"
cd "$CCD"
```

（A1 改用了 `mcp-literal.json` 的，复制它：`cp "$CC/mcp-literal.json" "$CCD/mcp.json" && chmod 600 "$CCD/mcp.json"`。）D1–D3 都在 `$CCD` 里以 `claude --mcp-config "$CCD/mcp.json" --strict-mcp-config` 启动（D3 加 `-p`）。Claude Code 把"不再问"记在哪里未核对：D2、D3 之前各执行一次 `rm -rf "${CCD:?}/.claude"`，清掉前一步在这个目录里留下的设置；每一步若有该问而没问的工具（像是从前面带过来的放行），记下它，并记下 `ls -a "$CCD/.claude"` 与其中各个文件的内容（没有这个目录时也记下）。

每类注解只试一个或两个工具：读的工具的注解都一样（`readOnlyHint`，`api/mcp/tools.json`），所以 `read` 与 `grep`（P4 问的四个读的工具之一）；写的分两类，`write`、`move`、`delete` 是 `destructiveHint` 加 `idempotentHint`，`edit`、`batch` 只有 `destructiveHint`，所以 `write` 与 `batch`。Claude Code 只试默认的许可模式（"不碰你自己的客户端配置"记下的 `defaultMode` 不是 `default` 时，D1–D3 的结果照它读）。

| # | 操作 | 预期 | 记下的 |
|---|---|---|---|
| D1 | Claude Code 在 `$CCD` 里启动，不放行任何工具，依次让它 `read`、`grep`、`write`、`batch` 一次：`read` 问时选"不再问"，别的问时只答这一次；之后再让它 `read` 一次 | 记下四个工具各问不问，与选了"不再问"之后的第二次 `read` 问不问 | |
| D2 | `rm -rf "${CCD:?}/.claude"`；在 `$CCD/.claude/settings.json` 写 README 的 `permissions.allow` 样例，重启，同 D1（这一次每个问的都只答这一次） | 读的工具不问，`write`、`batch` 照常问（照文档） | |
| D3 | `rm -rf "${CCD:?}/.claude"`（D2 的放行不带进来）；在 `$CCD` 里 `claude -p "读 Guide.md 的第 3 行" --mcp-config "$CCD/mcp.json" --strict-mcp-config`；再加 `--allowedTools mcp__nervewiki__read`；再以 `-p "在 Guide 下写一页 X"`（不放行 `write`） | 不放行时记下它怎样（拒绝、报错）；放行 `read` 时答出那一行；不放行的 `write` 不调用（照文档） | |
| D4 | Codex 的 `default_tools_approval_mode = "prompt"`（README 的样例），依次 `read`、`grep`、`write`、`batch` | 记下各问不问 | |
| D5 | 退出 Codex，把 `$CODEX_HOME/config.toml` 的 `default_tools_approval_mode` 改成 `"writes"`，再启动，同 D4 | 记下 | |
| D6 | 同样改成 `"auto"`，同 D4 | 记下 | |
| D7 | 同样改成 `"approve"`，同 D4 | 都不问（M0/P1） | |
| D8 | 改回 `"prompt"`，`codex exec "读 Guide.md 的第 3 行"`；再以 `"approve"` 执行一次 | `prompt` 时要审批的调用被拒（M0/P1），记下它的输出；`approve` 时答出那一行 | |

### E：HTTP 的答复与提示

| # | 操作 | 预期 | Claude Code 记下的 | Codex 记下的 |
|---|---|---|---|---|
| E1 | 不带令牌：Claude Code 用 `$CC/notoken.json`；Codex 退出，去掉 `config.toml` 的 `bearer_token_env_var`，再启动。`/mcp` | 服务端答 401，说明是"This server has no OAuth: MCP takes a personal access token in the Authorization header."；记下客户端显示的原文 | | |
| E2 | 接着 E1，看服务的日志 | 记下客户端在 401 之后请求了哪些路径：`/.well-known/` 之下（答 404）、`POST /register`（答 400 `invalid_client_metadata`），以及是否退到源站的 `/authorize`、`/token`（GET 是网页的 200 `index.html`，POST 是 405 `Allow: GET, HEAD`）。之后改回 E1 之前的配置 | | |
| E3 | 会话进行中在网页上撤销这个客户端的 PAT，再让 agent 读一页 | 服务端答 401 `Bearer error="invalid_token"`、"The bearer token is invalid or has expired."；记下客户端显示什么、是否转去 OAuth。之后建一个新的 PAT，以 `read -rs` 设进这个客户端的变量，重启客户端 | | |
| E4 | 取 A 的一个会话的访问令牌（表下的命令：密码不进 shell 的历史；没有 `jq` 时从浏览器的开发者工具里取）。把这个客户端的变量换成它，重启客户端，让 agent 读一页；之后换回 PAT、再重启 | 403，"MCP takes a personal access token, not a session's access token."；记下显示的原文 | | |
| E5 | 地址里的笔记本 id 换成一个随机的 UUID（`uuidgen`），重启客户端；之后换回 | 404 `notebook.not_found`；记下显示的原文 | | |
| E6 | （可选）在服务的终端里以小的桶重启：`NWIKI_RATELIMIT__AUTHENTICATED__PER_MINUTE=10 NWIKI_RATELIMIT__AUTHENTICATED__BURST=10 make run`，让 agent 连续读十几页；做完照常 `make run` | 429 `rate_limited` 带 `Retry-After`；记下显示的原文、客户端是否自己等着重试 | | |

E4 的访问令牌：

```sh
read -r EMAIL; read -rs PW
jq -n --arg e "$EMAIL" --arg p "$PW" '{email:$e,password:$p}' |
  curl -sS -X POST "$BASE/api/v0/auth/login" -H 'Content-Type: application/json' --data @- |
  jq -r .access_token
```

### F：大的请求体（账户入口）

F 部分在账户入口做：Claude Code 以 `$CC/account.json` 启动（F2 以 `$CC/throttled.json`），Codex 把 `config.toml` 的 `url` 换成 `http://127.0.0.1:8080/mcp`（F2 换成 `http://127.0.0.1:8081/mcp`），每次换了都重启客户端。每一步先让 agent 打开它的笔记本（`open_notebook`）。agent 有的说明：账户入口握手时的内置说明，其中有"超过 64 KiB 的调用每个令牌一次一个，大的写一个接一个地发"（`page/adapter/mcp/instructions.go:73-74`），与 `open_notebook` 给的 `AGENTS` 页的正文。下面 `$TOKEN` 是这个客户端的令牌（`$NERVEWIKI_CLAUDE_TOKEN` 或 `$NERVEWIKI_CODEX_TOKEN`），`$NB` 是它的笔记本。

| # | 操作 | 预期 | Claude Code 记下的 | Codex 记下的 |
|---|---|---|---|---|
| F1 | 在另一个终端里（设好 `$TOKEN`、`$NB`）以这个客户端的令牌**一直**占住它的名额：照表下的"F1 的占位"，先写 `body.json`（1 MiB 的写），再起循环：每一轮约 26 秒（短于 `server.read_timeout` 的 30 秒），一轮接一轮；答了 HTTP 的错误（`--fail`）的一轮之后先停 2 秒，不空转。第二轮起 `占位.md` 已是同样的正文，`write` 答 `changed: false`（不带 `base_version` 写同样的正文不算冲突，`page/app/write_path.go:176-190`），不要紧：名额在读请求体时就占着。服务的日志里出现它的第一行 `http request`（`POST /mcp`，约 26 秒）之后，回到会话，让 agent 写一页 `大.md`，内容是"lorem ipsum dolor sit amet "重复 2,500 遍（67,500 字节）。这一步做完在那个终端里 `kill %1`（`jobs` 看它的号） | **先看日志**：若是一串很快的 401、404 或 429，而不是约 26 秒一行的 200，先 `kill %1`，改正 `$TOKEN` 或 `$NB` 再起：错的令牌每一轮都耗认证失败的闸门（按客户端 IP、在认证之前取，每分钟 60 个，`platform/httpserver/api.go:346-350`），耗完了这台机器上每个带令牌的请求（agent 的与浏览器的）都答 429。agent 的调用发出时占位的那一轮必须正在传：服务的日志里，agent 的那一行 `POST /mcp` 之前有一行 `large request bodies are saturated`，`own: true`、`holders` 里是这个令牌（`pat:<id>`）。然后它多半（约九成：占位的一轮在那 2 秒里答完时它拿得到名额）等 2 秒后答 HTTP 503 `server_busy`（`Retry-After: 1`）；记下客户端怎样（重试、把错误交给模型、断开）、agent 之后是否等一等再发。agent 的调用答 200 时记下它落在占位的两轮之间 | | |
| F2 | 起一个限速的转发（下面的脚本，`node throttle.mjs 16384`：客户端到服务的方向每秒 16 KiB），客户端换到经它的账户入口（上面），让 agent **同时**写两页，各是 F1 那样的 67,500 字节 | 每个约 4 秒才传完。看服务的日志里这两个 `POST /mcp` 的时刻与 `duration`：重叠的（第二个在第一个答完之前开始），第二个等 2 秒后 503，记下 agent 看到什么、是否照说明一个接一个地重发；一个接一个的，记下这个客户端是一个接一个地发写的（那它碰不到每个令牌一个名额的代价）。两个参数合起来约 3 万个 token 的输出，客户端若写不出（输出的上限，未核对）也记下 | | |
| F3 | F2 之后看历史；停掉转发（Ctrl-C），客户端换回笔记本入口 | 两页都写成了（重发的那一次成功），各一个变更集 | | |

F1 的占位（另一个终端，在放 `seed.mjs` 的临时目录里，设好 `BASE`、`TOKEN`、`NB`；表格里不写命令，免得从原文复制时带上转义）：

```sh
node -e 'process.stdout.write(JSON.stringify({jsonrpc:"2.0",id:1,method:"tools/call",params:{name:"write",arguments:{path:"占位.md",content:"y".repeat(1<<20)}}}))' > body.json
while :; do
  curl --fail -sS -o /dev/null --limit-rate 40k "$BASE/mcp?notebook=$NB" \
    -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' -H 'MCP-Protocol-Version: 2025-11-25' \
    --data-binary @body.json || sleep 2
done &
```

F2 的转发（存成 `throttle.mjs`，`node throttle.mjs <每秒字节数>`；Ctrl-C 停）：

```js
// throttle.mjs：127.0.0.1:8081 转到 127.0.0.1:8080，客户端到服务的方向限速
import net from "node:net";
const rate = Number(process.argv[2] ?? 16384);
net
  .createServer((client) => {
    const server = net.connect(8080, "127.0.0.1");
    server.pipe(client);
    client.on("data", (chunk) => {
      client.pause();
      server.write(chunk, () => setTimeout(() => client.resume(), (chunk.length / rate) * 1000));
    });
    client.on("end", () => server.end());
    const close = () => {
      client.destroy();
      server.destroy();
    };
    client.on("error", close);
    server.on("error", close);
  })
  .listen(8081, "127.0.0.1", () => console.log(`127.0.0.1:8081 → :8080, ${rate} bytes/s up`));
```

### G：服务重启

| # | 操作 | 预期 | Claude Code 记下的 | Codex 记下的 |
|---|---|---|---|---|
| G1 | 交互的会话里让 agent 读一页；在服务的终端 Ctrl-C 停掉 `make run`、再 `make run`（同一把签名私钥：浏览器不必重新登录）；回到会话，让 agent 再读一页、再写一页 | 不重新连接就成功（服务端没有会话：日志里没有新的 `MCP initialize`，或客户端自己又握了一次手，记下是哪一种）；写的变更集的客户端仍是 A1、B1 记下的名称 | | |
| G2 | 同 G1，但服务停着时就让 agent 读一页，再启动服务，再让它读 | 记下停着时显示什么、之后能不能不重启客户端就恢复 | | |

`claude -p` 与 `codex exec` 每次都是新的进程、新的握手，测不到重启：G 部分要交互的会话。

### H：编辑锁

| # | 操作 | 预期 | Claude Code 记下的 | Codex 记下的 |
|---|---|---|---|---|
| H1 | B 在网页上打开这个客户端的笔记本里的 `Guide/锁.md`（C14 的撤销之后它又是这个名字）并进入编辑；让 agent 改它的一行 | `page.locked`：文字说出 B 的显示名、"try again after they leave edit mode"、"cannot take the lock over"；记下 agent 怎样转述、是否反复重试 | | |
| H2 | B 离开编辑；A 自己在网页上编辑同一页；再让 agent 改它 | `page.locked`，文字说"by you, in a browser" | | |
| H3 | A 仍在编辑；让 agent 删除 `Guide.md`（带子页） | 不带 `recursive` 的第一次先答 `validation_failed`，说出它下面有几页、几个附件（数在锁之前）；带了 `recursive` 答 `page.locked`，列出子树里被编辑的那一页；Guide 与它的子页都还在 | | |

### I：反向代理

上游是本机的服务：容器里是 `host.docker.internal:8080`（Docker Desktop；连不上时服务以 `NWIKI_SERVER__ADDR=0.0.0.0:8080` 启动，只在这一节、只在可信的网络里）。

- nginx：把 README"部署"一节的 nginx 样例原样存成 `nginx.conf`，只在 `server_name` 之后加一行 `listen 8090;`，四个 `location` 的 `proxy_pass` 都换成 `http://host.docker.internal:8080;`；然后 `docker run --rm -d --name nw-nginx -p 127.0.0.1:8090:8090 -v "$PWD/nginx.conf:/etc/nginx/conf.d/default.conf:ro" nginx:1.29-alpine`（样例是一个 `server`，放在 `conf.d` 里）。
- I2、I3 的写：`node -e 'process.stdout.write(JSON.stringify({jsonrpc:"2.0",id:1,method:"tools/call",params:{name:"write",arguments:{path:"代理.md",content:"z".repeat(200000)}}}))' > body200k.json`（200 KB）与一个约 200 KB 的文件 `200k.bin`（`head -c 200000 /dev/urandom > 200k.bin`）。

| # | 操作 | 预期 | 记下的 |
|---|---|---|---|
| I1 | Claude Code 经 nginx 连（一份 `mcp.json` 的地址换成 `http://127.0.0.1:8090/mcp?notebook=…`），让 agent 读 `Guide.md` 的第 3 行（C3 改过它：答的是现在的那一行），再写一页 `经 nginx.md`（几行字；不靠 C 部分留下的页）；浏览器经 nginx（`http://127.0.0.1:8090/`）打开这一页的阅读视图，再让 agent 改它 | 都成；阅读视图不重新加载就显示 agent 的改动（事件流经 nginx 照常） | |
| I2 | 缓冲：`curl -sS -o /dev/null --limit-rate 20k "http://127.0.0.1:8090/mcp?notebook=$NB_CC" -H "Authorization: Bearer $NERVEWIKI_CLAUDE_TOKEN" -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' -H 'MCP-Protocol-Version: 2025-11-25' --data-binary @body200k.json`，再把地址换成 `$BASE` 直接发一次；经 nginx 以 `--limit-rate 20k` 上传 `200k.bin`（`curl -sS -o /dev/null --limit-rate 20k "http://127.0.0.1:8090/api/v0/notebooks/$NB_CC/assets" -H "Authorization: Bearer $NERVEWIKI_CLAUDE_TOKEN" -F file=@200k.bin`，传到笔记本的根下） | 服务的日志里：经 nginx 的 `/mcp` 那一行的 `duration` 很短（nginx 收完才转来），直接的约 10 秒；上传那一行的 `duration` 约 10 秒（不缓冲，服务端边收边写） | |
| I3 | 你的部署前面是 Caddy 时，在那里确认：照 README 的 Caddy 样例配置，同 I2（经 Caddy 的地址；上传另起一个名字：`-F 'file=@200k.bin;filename=200k-caddy.bin'`，免得撞上 I2 传到根下的 `200k.bin`，同名的上传在读文件之前就答 `page.title_taken`），看服务的日志。没有 Caddy 的部署时可以以容器代替：README 的 Caddy 样例存成 `Caddyfile`，`wiki.example.com` 换成 `:8091`、两处上游换成 `host.docker.internal:8080`，`docker run --rm -d --name nw-caddy -p 127.0.0.1:8091:8091 -v "$PWD/Caddyfile:/etc/caddy/Caddyfile:ro" caddy:2.10-alpine`；都没有时跳过（P5 已在 Caddy 2.10.2 上核对过这个写法，P5 设计 5.2）。然后照表下的"I3 的被拒的写"，以一个错的令牌把一个 8 MiB 的写经 Caddy 发到 `/mcp`，再直接发一次 | `/mcp` 那一行的 `duration` 很短、上传的约 10 秒，同 I2 经 nginx 的。被拒的大的写：直接的答 401（服务端在读请求体之前就答）；经 Caddy 的记下状态码与头。替身量过（P5 设计 5.2）：在读请求体之前答的 503、401、429，8 MiB 时经 Caddy 多半成了 502；503 的那些没有 `Retry-After`，401 的头替身没有记。这一步是在你的部署上、以真的服务端核对 | |

I3 的被拒的写（`CADDY` 是经 Caddy 的地址，如 `http://127.0.0.1:8091`）：

```sh
head -c 8388608 /dev/zero | tr '\0' 'a' > big.bin
for addr in "$CADDY" "$BASE"; do
  echo "== $addr"
  curl -sS -o /dev/null -D - -w '%{http_code}\n' "$addr/mcp?notebook=$NB_CC" \
    -H 'Authorization: Bearer nwk_pat_wrong' -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' --data-binary @big.bin
done
```

## 做完

1. **Codex 的登录**：照"不碰你自己的客户端配置"登录的第 3 种复制过 `auth.json` 的，先照那里的规则决定复制不复制回去（看 `shasum` 的两个指纹），**再**删临时目录；用第 2 种的，在一个没有设 `CODEX_HOME` 的终端里启动你自己的 Codex，记下它是否照常可用，不能用时在那里 `codex login` 再登录一次。
2. 在网页上撤销 `seed`、`claude-checklist`、`codex-checklist` 与 E3 新建的 PAT（服务还开着）。
3. 停掉 `make run`、`throttle.mjs`（若还在）与代理的容器：`docker stop nw-nginx nw-caddy`。
4. **Claude Code 的会话**：它在 `~/.claude/projects/` 下为 `$CC`、`$CCD` 记的会话可以留着，不碍事。要删时，在还设着 `$CC`、`$CCD` 的终端里、下一步删掉这两个目录之前，先列出名字里带着 `$CC` 的目录名最后那段随机的字母（`mktemp -d` 给的是 `tmp.` 加十个随机的字母与数字）的：`ls ~/.claude/projects | grep -F -- "${CC##*.}"`（`$CCD` 换成 `"${CCD##*.}"` 再列一次）。只在恰好列出一个目录时删它：`rm -rf ~/.claude/projects/` 之后照抄列出的那个名字，不用通配符。Claude Code 怎样给这些目录起名未核对：列出的不是恰好一个时都不删。
5. 删掉临时目录：`$CC`（含几份 `*-literal.json`，若有）、`$CCD`、`$CX`、`$CODEX_HOME`、`$KEYS` 与放 `seed.mjs`、`body.json`、`big.bin` 的那一个；关掉设过令牌的终端（或 `unset NERVEWIKI_CLAUDE_TOKEN NERVEWIKI_CODEX_TOKEN SEED_TOKEN TOKEN EMAIL PW`）。

## 不在这份清单里的

- 本地进程的（stdio）MCP 服务、OAuth：服务端没有它们（M9 总设计第 2 节"不做"）。
- 2026-07-28 的协议（`server/discover`、`subscriptions/listen`）：等真实的客户端发它再开（M9 总设计第 11 节主持者的决定第 2 条）；客户端若以它连接而被拒，记在"记下的"里，交负责人。
- Claude Code 的别的许可模式（`acceptEdits`、`bypassPermissions` 一类）：README 不推荐它们，是负责人自己的选择。
- prompts（ingest、query、lint）：M10 登记。
- 多个实例：v0.1 只支持单实例。
