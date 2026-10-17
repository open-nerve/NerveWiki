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
- Every path is from the notebook's root, its segments joined by "/". A leading "/" is optional; "./", ".." and ` +
	`empty segments are refused.
- A page is a Markdown file: its path is its ancestors' titles, then its own title and ".md", as in ` +
	`"Projects/Roadmap.md". A page holds pages and attachments, as a folder does.
- An attachment is a file under a page or at the root, as in "Projects/diagram.png". Its name never ends with ` +
	`".md"; a page titled "x.png" is "x.png.md".
- Segments compare without case: "projects/roadmap.md" finds "Projects/Roadmap.md".
- Paths address nodes; they are not link targets. In a page's content, link as the wiki does: [[Projects/Roadmap]].

Tools
- list: the children of the root ("" or "/") or of a page ("Projects/"), in the tree's order. Any other pattern is a ` +
	`glob over the paths, in path order: * and ? within a segment, ** across segments; "**/*.md" is every page. Go on ` +
	`with next_cursor while it is set: a costly glob may answer a page with no items and a cursor.
- read: a page's lines exactly as stored, from offset for limit lines, about 45 KiB at most per call. Go on with ` +
	`next_offset, and with next_column within a line too long for one call. An attachment gives its metadata, and the ` +
	`image or a link to download it.
- A page's version, "<node id>:<revision>", tells which content you read.`
