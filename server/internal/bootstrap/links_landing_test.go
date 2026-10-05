package bootstrap

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// A link's landing through serve (M6/P6 design 2): a writer asks where a
// page made for a link that leads nowhere would go, makes it there with
// createPage, and the link leads to it: the links event lists the page of
// the link, its reading view shows the link resolved, and the landing then
// answers the page. A target is the query's as written: one with a NUL or
// bytes that are not UTF-8 has no landing, without a fault; nor has one
// under a page as deep as pages nest.
func TestALinksLandingThroughServe(t *testing.T) {
	tm, nb, marker, s := eventsTeam(t)
	a := tm.createPage(t, "alice", nb, "", "A")
	s.tree(t, nb, map[string]int{a: 1})
	plans := tm.createPage(t, "alice", nb, a, "Plans")
	s.tree(t, nb, map[string]int{plans: 1})
	src := tm.createPageWith(t, "alice", nb, a, "Src", "[[Plans/2026.md]]\n")
	s.tree(t, nb, map[string]int{src: 1})

	landing := "/api/v0/pages/" + src + "/link-landing"
	lands := func(target, want string) {
		t.Helper()
		status, body := ask(t, tm.contract, http.MethodGet, tm.base+landing+"?target="+url.QueryEscape(target), tm.tokens["bob"], "")
		if status != http.StatusOK || strings.TrimSpace(body) != want {
			t.Errorf("the landing of %q = %d %s, want %s", target, status, body, want)
		}
	}
	nowhere := func(reason string) string { return `{"landing":null,"node_id":null,"reason":"` + reason + `"}` }
	lands("Plans/2026.md", `{"landing":{"parent_id":"`+plans+`","title":"2026"},"node_id":null,"reason":null}`)
	lands("./Plans/x", `{"landing":{"parent_id":"`+plans+`","title":"x"},"node_id":null,"reason":null}`)
	lands("x", `{"landing":{"parent_id":"`+a+`","title":"x"},"node_id":null,"reason":null}`)
	lands("../Plans/x", nowhere("parent_missing"))
	lands("Plans/a:b", nowhere("title_invalid"))
	lands("Plans//x", nowhere("target_invalid"))
	lands("x\x00", nowhere("target_invalid"))
	lands("x\xff", nowhere("target_invalid"))
	for query, want := range map[string]string{"": "required", "?target=" + strings.Repeat("a", 4097): "too_long"} {
		status, body := ask(t, tm.contract, http.MethodGet, tm.base+landing+query, tm.tokens["bob"], "")
		if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"target"`) || !strings.Contains(body, `"code":"`+want+`"`) {
			t.Errorf("a landing with %.20s… = %d %s, want 422 %s on target", query, status, body, want)
		}
	}
	// Pages nest 10 deep: no page goes under the tenth.
	var chain []string
	deep := ""
	for i := range 10 {
		parent := ""
		if i > 0 {
			parent = chain[i-1]
		}
		chain = append(chain, tm.createPage(t, "alice", nb, parent, fmt.Sprintf("D%d", i)))
		s.tree(t, nb, map[string]int{chain[i]: 1})
		deep += fmt.Sprintf("/D%d", i)
	}
	lands(deep+"/x", nowhere("too_deep"))
	lands(strings.TrimSuffix(deep, "/D9")+"/x", `{"landing":{"parent_id":"`+chain[8]+`","title":"x"},"node_id":null,"reason":null}`)

	made := tm.createPage(t, "bob", nb, plans, "2026")
	s.tree(t, nb, map[string]int{made: 1})
	s.links(t, nb, []string{src}, []string{made})
	tm.resolves(t, src, made)
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+src+"/view", tm.tokens["bob"], "")
	if want := `data-nw-node=\"` + made + `\"`; status != http.StatusOK || !strings.Contains(body, want) {
		t.Errorf("the reading view = %d %s, want the link resolved, %s", status, body, want)
	}
	lands("Plans/2026.md", `{"landing":null,"node_id":"`+made+`","reason":null}`)
	tm.quiet(t, nb, marker, s)
}
