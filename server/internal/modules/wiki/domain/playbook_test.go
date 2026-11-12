package domain

import (
	"strings"
	"testing"
)

// The upgrade playbook (M11/P3 design 3.17): its imperative first line;
// each sentence it must hold, a cell a sentence or a clause, so that a
// rewrite that drops one kind of advice, or keeps a condition or a code and
// turns what it asks for into something else, fails by name; each exactly
// once, so that a code two sentences name cannot stand in for either
// (M11/P3/S2 fix 1, P3S2B-M1, P3S2C-M1); within MaxPlaybookBytes, the
// file's white space around it trimmed.
func TestUpgradePlaybookText(t *testing.T) {
	text := UpgradePlaybookText()
	t.Logf("the upgrade playbook is %d bytes, of %d", len(text), MaxPlaybookBytes)
	if first, _, _ := strings.Cut(text, "\n"); first != "Upgrade this notebook to an LLM Wiki." {
		t.Errorf("the first line = %q", first)
	}
	if len(text) > MaxPlaybookBytes {
		t.Errorf("the text is %d bytes, over %d", len(text), MaxPlaybookBytes)
	}
	if text == upgradePlaybook {
		t.Fatal("the file has no white space around it: nothing tells that the text is trimmed")
	}
	if text != strings.TrimSpace(upgradePlaybook) {
		t.Errorf("the text is not the file trimmed: %q … %q", text[:20], text[len(text)-20:])
	}
	for _, s := range []string{
		// Who writes while the notebook is frozen, what refuses the others,
		// and what the upgrade wrote.
		"Read and follow AGENTS.md and this notebook's instructions.",
		"The notebook is standard while it upgrades, and frozen",
		"only the agent of the admin who started the upgrade, through MCP, may write, and only while the upgrade is upgrading.",
		"notebook.upgrading means you are not that agent, the upgrade is no longer upgrading, or your user is no longer an admin of this notebook: stop and tell your user.",
		"The upgrade proposed a source root, raw at the notebook's root, and AGENTS at the root; it may have written them.",
		// The eight gates, as upgrade_verify names them, and how to read them.
		"Start with upgrade_verify without a rule.",
		"It names eight gates and eight rules: AGENTS has a body;",
		"the source root is a live page and not AGENTS;",
		"no node at the root is titled index, log, index.md or log.md;",
		"every page outside the source root has valid frontmatter with type and summary;",
		"every sources reference is a link to a page or attachment under the source root;",
		"no original page links to more missing pages than when the upgrade started;",
		"every original page and node is still there, its body unchanged;",
		"and the link index is up to date.",
		"Read one rule's items with upgrade_verify and that rule, and continue next_cursor until it is null.",
		"Call upgrade_verify again after each round of changes.",
		// Sorting the pages; the source root's own pages (decision 214);
		// the frontmatter each wiki page gets.
		"Sort the original pages: a source is evidence you keep as it is; a wiki page is knowledge you maintain.",
		"Move each source, with the pages under it, under the source root.",
		"Pages already under the source root that are not sources: move them out of it; after the approval, everything under it is a source, read-only.",
		"Give each wiki page, folder pages too, frontmatter with type and summary; a page that rests on sources also lists them in sources, quoted links to their paths under the source root",
		`for example sources: ["[[raw/Note.md]]"].`,
		// The guardrails' codes, each in the sentence that says what it wants.
		"You may change an original page's frontmatter only: its body after the frontmatter, and after a byte order mark, must stay byte for byte, or the write is page.upgrade_immutable.",
		"A page that starts with --- and has no closing --- line has no frontmatter: add a valid block before it.",
		"A page whose block between two --- lines is not valid YAML: rewrite that block in place, for example keeping its old text in a quoted property, or move the page under the source root, where frontmatter is not checked;",
		"a new block before it would make the old one part of the body.",
		"Keep an existing AGENTS page whose body is not empty, or rename it and write a new AGENTS;",
		"rename any node at the root titled index, log, index.md or log.md.",
		"Never delete an original node or the proposed source root: that is page.upgrade_protected.",
		"Pages you created during the upgrade may be deleted.",
		"Moves and renames must let the system rewrite the links that lead to what moved: never pass update_links: false, or the change is page.upgrade_keep_links.",
		"A link by name alone resolves after a move and is not rewritten; a link that names a path is rewritten when its target moves, and a rename rewrites both.",
		"A new page's title becomes the target of existing short links that end with it: choose new titles knowing that, and read the ambiguous rule's items after creating pages.",
		"In a batch, once a move or a rename has rewritten an original page's links, or would by the links the page had before the batch, writing that page later in the same batch is page.upgrade_batch_rewrite, whatever you write: write the page first and move afterwards, or use two batches.",
		"A page changed and changed back within one batch passes, unless a move in between rewrote it or reached it.",
		"Splitting the batch does not make a body change acceptable: it is still page.upgrade_immutable.",
		"Large original pages that a batch writes whole count toward its read bound; page.batch_too_large means split the batch.",
		// The submission and its answers.
		"When upgrade_verify reports every gate passed, call upgrade_submit.",
		"It checks again and puts the upgrade in review, where nothing can be written.",
		"wiki.upgrade_not_ready names the gates that did not pass: the upgrade is upgrading again, so verify, fix and submit again.",
		"wiki.upgrade_stale_index means the link index is out of date: tell your user that a server admin runs nervewiki reindex; once upgrade_verify passes the index gate, submit again.",
		"wiki.upgrade_state saying the previous submission is still being checked means wait a moment and call upgrade_verify before submitting again;",
		"wiki.upgrade_state otherwise means the upgrade is already in review or no longer under way: tell your user.",
		"server_busy means wait retry_after_seconds, then submit again.",
		// The review, the report, and the last word.
		"Once submitted, an admin reviews the upgrade in the web app, then approves it, sends it back for more work, or rejects it, which puts the notebook back as it was.",
		"Report what you changed, which gates passed, and what remains for your user to decide.",
		"Never claim a gate passed that upgrade_verify did not report as passed.",
	} {
		t.Run(s, func(t *testing.T) {
			if n := strings.Count(text, s); n != 1 {
				t.Errorf("the playbook says %q %d times, want once", s, n)
			}
		})
	}
	if !strings.HasSuffix(text, "Never claim a gate passed that upgrade_verify did not report as passed.") {
		t.Errorf("the playbook ends %q, want its last sentence", text[len(text)-80:])
	}
}
