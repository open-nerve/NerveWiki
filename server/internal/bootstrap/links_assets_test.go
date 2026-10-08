package bootstrap

import (
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
)

// Attachments as the links' targets through serve (M7/P3 design 4.9): the
// index, the rewrite, the landing, the completion, the properties and the
// attachments' own link, with the reading view of part A.

// assetsResolved is whether each link of the page id resolves to an
// attachment, in the order written.
func (tm acmeTeam) assetsResolved(t *testing.T, id string) []string {
	t.Helper()
	return queryStrings(t, tm.pool, `SELECT resolved_asset::text FROM page_links WHERE source_id = $1 ORDER BY range_start`, id)
}

// An attachment is a link's target (M7/P3 design 4.3, 4.4): uploaded, the
// links to its path resolve to it, as an attachment, which another of its
// name elsewhere does not change; the page above it renamed, the
// attachment moved and renamed, the links to it, embeds, Markdown links
// and images, are written again in the same unit and changeset, by its
// path while its name is another's too, whose pages event lists the page
// written, and the links event the attachment; deleted, the links resolve
// to none. Each links event follows its unit's pages event, and after each
// write the index is its rebuild.
func TestAnAttachmentIsALinksTargetThroughServe(t *testing.T) {
	tm, nb, marker, s := eventsTeam(t)
	a := tm.createPage(t, "alice", nb, "", "A")
	s.tree(t, nb, map[string]int{a: 1})
	b := tm.createPage(t, "alice", nb, "", "B")
	s.tree(t, nb, map[string]int{b: 1})
	other := tm.upload(t, "alice", nb, b, "x.png", pngFile)
	s.tree(t, nb, map[string]int{})
	src := tm.createPageWith(t, "alice", nb, "", "Src", "![[A/x.png|300]] [Pic](A/x.png) ![](A/x.png)\n")
	s.tree(t, nb, map[string]int{src: 1})
	tm.resolves(t, src, "", "", "")

	x := tm.upload(t, "alice", nb, a, "x.png", pngFile)
	s.tree(t, nb, map[string]int{})
	s.links(t, nb, []string{src}, []string{x.ID})
	tm.resolves(t, src, x.ID, x.ID, x.ID)
	if got := tm.assetsResolved(t, src); !slices.Equal(got, []string{"true", "true", "true"}) {
		t.Errorf("the links resolve to attachments: %q, want all", got)
	}
	tm.checkRebuilt(t, nb)

	tm.send(t, nodeRename("alice", a, "C"), http.StatusOK)
	s.tree(t, nb, map[string]int{src: 2})
	s.links(t, nb, []string{}, []string{x.ID})
	tm.wrote(t, src, "![[C/x.png|300]] [Pic](C/x.png) ![](C/x.png)\n", 2)
	tm.checkRebuilt(t, nb)

	tm.send(t, nodeMove("alice", x.ID, ""), http.StatusOK)
	s.tree(t, nb, map[string]int{src: 3})
	s.links(t, nb, []string{}, []string{x.ID})
	tm.wrote(t, src, "![[x.png|300]] [Pic](x.png) ![](x.png)\n", 3)
	tm.checkRebuilt(t, nb)

	tm.send(t, nodeRename("alice", x.ID, "y.png"), http.StatusOK)
	s.tree(t, nb, map[string]int{src: 4})
	s.links(t, nb, []string{}, []string{x.ID})
	tm.wrote(t, src, "![[y.png|300]] [Pic](y.png) ![](y.png)\n", 4)
	tm.resolves(t, src, x.ID, x.ID, x.ID)
	if n := count(t, tm.pool, `SELECT count(*) FROM page_revisions r JOIN changeset_items i ON i.changeset_id = r.changeset_id
		WHERE r.node_id = $1 AND r.revision = 4 AND i.node_id = $2`, src, x.ID); n != 1 {
		t.Errorf("the rename's changeset holds %d of its rewrite, want Src's revision 4", n)
	}
	checkLinks(t, tm.pool)
	tm.checkRebuilt(t, nb)

	tm.send(t, nodeDeletion("alice", x.ID), http.StatusNoContent)
	s.tree(t, nb, map[string]int{})
	s.links(t, nb, []string{src}, []string{x.ID})
	tm.resolves(t, src, "", "", "")
	if got := tm.assetsResolved(t, src); !slices.Equal(got, []string{"false", "false", "false"}) {
		t.Errorf("the links resolve to attachments: %q, want none", got)
	}
	tm.quiet(t, nb, marker, s)
	if n := count(t, tm.pool, "SELECT count(*) FROM page_links WHERE resolved_id = $1", other.ID); n != 0 {
		t.Errorf("%d links resolve to the other x.png, want none", n)
	}
	tm.checkRebuilt(t, nb)
	checkLinks(t, tm.pool)
	checkPages(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
}

// An attachment's rename whose rewrite reaches a page being edited is
// refused whole, as a page's is (M6/P4 design 4.1; M7/P3 design 4.4): 409
// linking.pages_locked with the page's lock, nothing changed; the lock
// gone, the rename writes the link again.
func TestARewriteOfAnAttachmentsLinksInAPageBeingEditedIsRefused(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	x := tm.upload(t, "alice", nb, "", "x.png", pngFile)
	src := tm.createPageWith(t, "alice", nb, "", "Src", "![[x.png]]\n")
	tm.openSession(t, "bob", src)

	was := tm.snapshot(t, nb)
	tm.refused(t, nodeRename("alice", x.ID, "y.png"), map[string]string{src: "bob"})
	if now := tm.snapshot(t, nb); now != was {
		t.Errorf("the refusal changed the notebook:\n%s\nwas\n%s", now, was)
	}
	if got := queryStrings(t, tm.pool, "SELECT name FROM nodes WHERE id = $1", x.ID); !slices.Equal(got, []string{"x.png"}) {
		t.Errorf("the attachment is named %q after the refusal, want x.png", got)
	}

	tm.send(t, unlock("alice", src), http.StatusNoContent)
	tm.send(t, nodeRename("alice", x.ID, "y.png"), http.StatusOK)
	tm.wrote(t, src, "![[y.png]]\n", 2)
	checkLinks(t, tm.pool)
	checkPages(t, tm.pool)
}

// An attachment's name has no landing (M7/P3 design 4.5): a target read as
// an attachment's is target_is_asset whether it resolves or not, and so is
// a page's that an attachment where the page would go has the title of; a
// target with ".md" reads as a page's, which lands where no attachment has
// its title, an attachment without an extension's beside it too. The
// completion lists the attachment, of its kind, with its link and no
// aliases, but not one without an extension, which no link leads to; a
// property link to it is of its kind; and the reading view shows the
// links to it at its content's address, none as leading to a page or
// nowhere (M7/P3 design 5.5).
func TestAnAttachmentsNameThroughServe(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	a := tm.createPage(t, "alice", nb, "", "A")
	x := tm.upload(t, "alice", nb, a, "x.png", pngFile)
	src := tm.createPageWith(t, "alice", nb, "", "Src", "---\ncover: \"[[x.png]]\"\n---\n![[x.png]] [[x.png]]\n")
	inA := tm.createPageWith(t, "alice", nb, a, "InA", "")
	tm.upload(t, "alice", nb, a, "data", "plain")

	lands := func(from, target, want string) {
		t.Helper()
		status, body := ask(t, tm.contract, http.MethodGet,
			tm.base+"/api/v0/pages/"+from+"/link-landing?target="+url.QueryEscape(target), tm.tokens["bob"], "")
		if status != http.StatusOK || strings.TrimSpace(body) != want {
			t.Errorf("the landing of %q = %d %s, want %s", target, status, body, want)
		}
	}
	isAsset := `{"landing":null,"node_id":null,"reason":"target_is_asset"}`
	lands(src, "x.png", isAsset)
	lands(src, "B/x.png", isAsset)
	lands(src, "X.PNG", isAsset)
	lands(src, "x.png.md", `{"landing":{"parent_id":null,"title":"x.png"},"node_id":null,"reason":null}`)
	lands(inA, "x.png.md", isAsset)
	lands(src, "data", `{"landing":{"parent_id":null,"title":"data"},"node_id":null,"reason":null}`)
	lands(inA, "data", isAsset)

	var list struct {
		Data []struct {
			ID      string   `json:"id"`
			Kind    string   `json:"kind"`
			Name    string   `json:"name"`
			Link    string   `json:"link"`
			Aliases []string `json:"aliases"`
		} `json:"data"`
	}
	tm.get(t, "bob", "/api/v0/notebooks/"+nb+"/link-targets", &list)
	var targets []string
	for _, l := range list.Data {
		targets = append(targets, l.ID+" "+l.Kind+" "+l.Name+" "+l.Link+" "+strings.Join(l.Aliases, ","))
	}
	if want := []string{a + " page A A ", x.ID + " asset x.png x.png ", src + " page Src Src ", inA + " page InA InA "}; !slices.Equal(targets, want) {
		t.Errorf("link targets %q, want %q", targets, want)
	}

	var props struct {
		Links []struct {
			Key    string  `json:"key"`
			NodeID *string `json:"node_id"`
			Kind   *string `json:"kind"`
		} `json:"links"`
	}
	tm.get(t, "bob", "/api/v0/pages/"+src+"/properties", &props)
	if len(props.Links) != 1 || props.Links[0].Key != "cover" || props.Links[0].NodeID == nil || *props.Links[0].NodeID != x.ID ||
		props.Links[0].Kind == nil || *props.Links[0].Kind != "asset" {
		t.Errorf("property links %+v, want cover to the attachment %s", props.Links, x.ID)
	}

	status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+src+"/view", tm.tokens["bob"], "")
	if status != http.StatusOK {
		t.Fatalf("bob's reading view = %d %s", status, body)
	}
	var v struct {
		HTML string `json:"html"`
	}
	decodeAnswer(t, body, &v)
	if strings.Count(v.HTML, "/api/v0/assets/"+x.ID+"/content?") != 3 || strings.Contains(v.HTML, "data-nw-target") ||
		strings.Contains(v.HTML, "data-nw-node") {
		t.Errorf("the reading view %q does not show its three links to the attachment at its address", v.HTML)
	}
	checkLinks(t, tm.pool)
	checkPages(t, tm.pool)
}

// An attachment's link is how a wikilink leads to it alone (M7/P3 design
// 4.6), in the upload's answer, its metadata and the list of its parent's:
// its name while no other attachment has it, its path from the root since;
// null for one without an extension, which no link leads to.
func TestAnAttachmentsLinkThroughServe(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	a := tm.createPage(t, "alice", nb, "", "A")
	b := tm.createPage(t, "alice", nb, "", "B")
	first := tm.upload(t, "alice", nb, a, "x.png", pngFile)
	if first.Link != "x.png" {
		t.Errorf("the first x.png's link is %q, want x.png", first.Link)
	}
	second := tm.upload(t, "alice", nb, b, "X.PNG", pngFile)
	if second.Link != "B/X.PNG" {
		t.Errorf("the second x.png's link is %q, want B/X.PNG", second.Link)
	}
	data := tm.upload(t, "alice", nb, a, "data", "plain")
	if data.Link != "" {
		t.Errorf("an attachment without an extension's link is %q, want null", data.Link)
	}

	var got uploadedAsset
	tm.get(t, "bob", "/api/v0/assets/"+first.ID, &got)
	if got.Link != "A/x.png" {
		t.Errorf("the first x.png's link is %q since the second, want A/x.png", got.Link)
	}
	for parent, want := range map[string]map[string]string{
		"": {}, a: {first.ID: "A/x.png", data.ID: ""}, b: {second.ID: "B/X.PNG"},
	} {
		var list struct {
			Data []uploadedAsset `json:"data"`
		}
		path := "/api/v0/notebooks/" + nb + "/assets"
		if parent != "" {
			path += "?parent_id=" + parent
		}
		tm.get(t, "bob", path, &list)
		links := map[string]string{}
		for _, l := range list.Data {
			links[l.ID] = l.Link
		}
		if !maps.Equal(links, want) {
			t.Errorf("the list of %q's links %q, want %q", parent, links, want)
		}
	}
}
