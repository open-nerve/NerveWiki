package bootstrap

import (
	"net/http"
	"testing"
)

// The deletion subscriber's last hop (M3 design 8; M5 design 4.10): each
// path that deletes notebooks ends the edit sessions of their pages, whose
// ends reach the streams that see them as lock events, then resets those
// streams, through the whole program; the streams that see none of the
// notebooks go on. bob and carol are members of acme; alice's Eng is open
// to them, with a marker page. A deletion of the workspace resets every
// stream that sees one of its notebooks.
func TestEveryNotebookDeletionResetsItsStreams(t *testing.T) {
	for _, tt := range []struct {
		name string
		// setup readies the path: a page of the notebooks it deletes, the
		// account whose session on it the path ends, and its trigger.
		setup func(t *testing.T, tm acmeTeam, eng string) (page, editor string, trigger func())
		reset []string
		kept  []string
	}{
		{"a notebook deleted", func(t *testing.T, tm acmeTeam, _ string) (string, string, func()) {
			nb := tm.createNotebook(t, "alice", "Gone")
			tm.addNotebookMember(t, "alice", nb, "bob", "editor")
			return tm.createPage(t, "alice", nb, "", "Page"), "bob", func() {
				tm.send(t, request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, ""), http.StatusNoContent)
			}
		}, []string{"alice", "bob"}, []string{"carol"}},
		{"an ownerless notebook deleted", func(t *testing.T, tm acmeTeam, _ string) (string, string, func()) {
			nb := tm.createNotebook(t, "bob", "Bob's")
			tm.addNotebookMember(t, "bob", nb, "carol", "editor")
			page := tm.createPage(t, "bob", nb, "", "Page")
			tm.send(t, tm.removal("alice", "bob"), http.StatusNoContent)
			return page, "carol", func() { tm.send(t, ownerlessDeletion("alice", nb), http.StatusNoContent) }
		}, []string{"carol"}, []string{"alice"}},
		{"the workspace deleted", func(t *testing.T, tm acmeTeam, eng string) (string, string, func()) {
			return tm.createPage(t, "alice", eng, "", "Page"), "bob", func() {
				tm.send(t, request("alice", http.MethodDelete, "/api/v0/workspaces/acme", ""), http.StatusNoContent)
			}
		}, []string{"alice", "bob", "carol"}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "member")
			eng := tm.openNotebook(t, "alice", "Eng")
			marker := tm.createPage(t, "alice", eng, "", "Marker")
			page, editor, trigger := tt.setup(t, tm, eng)
			session := tm.openSession(t, editor, page)
			tm.settle(t, eng, marker)
			streams := map[string]*eventStream{}
			for _, name := range append(append([]string{}, tt.reset...), tt.kept...) {
				streams[name] = openStream(t, tm.base, name, tm.tokens[name])
			}

			trigger()

			for _, name := range tt.reset {
				streams[name].lock(t, page, session)
				streams[name].reset(t, "notebooks_deleted")
			}
			if len(tt.kept) == 0 {
				return
			}
			var kept []*eventStream
			for _, name := range tt.kept {
				kept = append(kept, streams[name])
			}
			tm.quiet(t, eng, marker, kept...)
		})
	}
}
