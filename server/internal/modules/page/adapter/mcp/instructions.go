package mcpadapter

// The server's built-in instructions, in English (M9/P1 design 2.2,
// 3.15.4): the notebook entry's, which a handshake sends and open_notebook
// gives when a notebook has no AGENTS page, and the account entry's. Each
// tool of an entry has its line, "- <name>:" or "- <name> " at its start,
// which bootstrap's TestTheBuiltInInstructionsNameEveryTool holds to
// api/mcp/tools.json: a phase that adds a tool rewrites "Tools".
const (
	NotebookInstructions = `This is a notebook of Nerve Wiki, a team's wiki of Markdown pages. It has no AGENTS page, ` +
		`so these are the server's built-in instructions.

` + pathsAndTools + `

Conventions
- A root page titled AGENTS gives this notebook's agents their instructions: its content, without its frontmatter, ` +
		`replaces these at the next connection.`

	AccountInstructions = `This is Nerve Wiki, a team's wiki of Markdown pages, for your whole account: pages live in ` +
		`notebooks, notebooks in workspaces.

- notebooks lists the notebooks you can see, with their ids.
- open_notebook gives a notebook's instructions for agents and an overview of it. Follow those instructions while ` +
		`you work in it.
- Every other tool takes the notebook's id as notebook.

` + pathsAndTools
)

// pathsAndTools are both entries' "Paths" and "Tools".
const pathsAndTools = `Paths
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
- Calls over 64 KiB run one at a time per token; the next waits at most 2s. Send large writes sequentially.`
