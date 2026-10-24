package domain

import "testing"

// A standard notebook's contract writes nothing; an LLM Wiki's writes two
// pages in each language, the source root first and alone; a mode there
// is none of has no contract.
func TestTheContractsAreTheModes(t *testing.T) {
	standard, ok := ContractOf(ModeStandard)
	if !ok || standard.Mode != ModeStandard || len(standard.Templates) != 0 {
		t.Errorf("ContractOf(standard) = %+v, %v; want no template", standard, ok)
	}
	wiki, ok := ContractOf(ModeLLMWiki)
	if !ok || wiki.Mode != ModeLLMWiki || len(wiki.Templates) != len(Locales()) {
		t.Fatalf("ContractOf(llm_wiki) = %+v, %v; want a template in each of %v", wiki, ok, Locales())
	}
	for _, l := range Locales() {
		pages := wiki.Templates[l]
		if len(pages) != 2 || !pages[0].SourceRoot || pages[1].SourceRoot || pages[0].Title != SourceRootTitle ||
			pages[1].Title != AgentsTitle || pages[0].Content == "" || pages[1].Content == "" {
			t.Errorf("%s: the template's pages are %+v, want raw, the source root, then AGENTS", l, pages)
		}
		if TemplateMessage(l) == "" {
			t.Errorf("%s: the template's changeset has no message", l)
		}
	}
	if wiki.Templates[LocaleEn][1].Content == wiki.Templates[LocaleZhCN][1].Content {
		t.Error("the two languages' AGENTS are alike, want each its own")
	}
	if c, ok := ContractOf("upgrading"); ok || c.Templates != nil {
		t.Errorf("ContractOf(upgrading) = %+v, %v; want none", c, ok)
	}
	if TemplateMessage("fr") != "" {
		t.Errorf("TemplateMessage(fr) = %q, want none", TemplateMessage("fr"))
	}
}
