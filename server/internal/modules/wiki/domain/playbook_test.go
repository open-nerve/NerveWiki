package domain

import (
	"strings"
	"testing"
)

// The upgrade playbook (M11/P3 design 3.17): its imperative first line,
// each sentence it must hold, a cell a sentence so that a rewrite that
// drops one kind of advice fails by name, within MaxPlaybookBytes, the
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
		// Who writes while the notebook is frozen, and what refuses the others.
		"only the agent of the admin who started the upgrade",
		"notebook.upgrading means you are not that agent, the upgrade is no longer upgrading, or your user is no longer an admin of this notebook",
		// The eight gates, as upgrade_verify names them.
		"AGENTS has a body",
		"the source root is a live page and not AGENTS",
		"no node at the root is titled index, log, index.md or log.md",
		"every page outside the source root has valid frontmatter with type and summary",
		"every sources reference is a link to a page or attachment under the source root",
		"no original page links to more missing pages than when the upgrade started",
		"every original page and node is still there, its body unchanged",
		"the link index is up to date",
		"continue next_cursor until it is null",
		"Call upgrade_verify again after each round of changes",
		// Sorting the pages; the source root's own pages (decision 214).
		"Pages already under the source root that are not sources",
		"folder pages too",
		`sources: ["[[raw/Note.md]]"]`,
		// The guardrails' codes and what each wants.
		"page.upgrade_immutable",
		"has no closing --- line",
		"is not valid YAML",
		"Keep an existing AGENTS page whose body is not empty",
		"page.upgrade_protected",
		"never pass update_links: false, or the change is page.upgrade_keep_links",
		"A link by name alone resolves after a move and is not rewritten",
		"A new page's title becomes the target",
		"read the ambiguous rule's items after creating pages",
		"page.upgrade_batch_rewrite",
		"write the page first and move afterwards, or use two batches",
		"A page changed and changed back within one batch passes",
		"Splitting the batch does not make a body change acceptable",
		"count toward its read bound; page.batch_too_large means split the batch",
		// The submission's answers.
		"wiki.upgrade_not_ready names the gates that did not pass",
		"wiki.upgrade_stale_index means the link index is out of date: tell your user that a server admin runs nervewiki reindex",
		"still being checked means wait a moment and call upgrade_verify before submitting again",
		"wiki.upgrade_state otherwise means the upgrade is already in review or no longer under way: tell your user",
		"server_busy means wait retry_after_seconds",
		// The review, and the last word.
		"approves it, sends it back for more work, or rejects it",
		"Never claim a gate passed that upgrade_verify did not report as passed.",
	} {
		t.Run(s, func(t *testing.T) {
			if !strings.Contains(text, s) {
				t.Errorf("the playbook does not say %q", s)
			}
		})
	}
	if !strings.HasSuffix(text, "Never claim a gate passed that upgrade_verify did not report as passed.") {
		t.Errorf("the playbook ends %q, want its last sentence", text[len(text)-80:])
	}
}
