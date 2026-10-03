package bootstrap

import (
	"net/http"
	"testing"
)

// The visibility subscriber's last hop (M3/P2 handoff item 1): every path
// that changes which notebooks of acme an account may see resets the
// streams it concerns, through the whole program, and leaves the others
// streaming. bob is a member of acme and carol a guest; alice's Eng is open
// to the members, with a marker page that tells a stream left alone. A
// path whose change may reach every member resets every stream of acme.
func TestEveryVisibilityChangeResetsItsStreams(t *testing.T) {
	notebookMember := func(t *testing.T, tm acmeTeam, nb string) string {
		t.Helper()
		return tm.addNotebookMember(t, "alice", nb, "bob", "reader")
	}
	for _, tt := range []struct {
		name string
		// setup readies the path and returns its trigger.
		setup func(t *testing.T, tm acmeTeam) func()
		reset []string
		kept  []string
	}{
		{"a private notebook created", func(t *testing.T, tm acmeTeam) func() {
			return func() { tm.createNotebook(t, "bob", "Mine") }
		}, []string{"bob"}, []string{"alice"}},
		{"an open notebook created", func(t *testing.T, tm acmeTeam) func() {
			return func() { tm.openNotebook(t, "bob", "Open") }
		}, []string{"alice", "bob", "carol"}, nil},
		{"a notebook opened to the members", func(t *testing.T, tm acmeTeam) func() {
			nb := tm.createNotebook(t, "alice", "Private")
			return func() {
				tm.send(t, request("alice", http.MethodPatch, "/api/v0/notebooks/"+nb, `{"workspace_access":"viewer"}`), http.StatusOK)
			}
		}, []string{"alice", "bob", "carol"}, nil},
		{"a notebook member added", func(t *testing.T, tm acmeTeam) func() {
			nb := tm.createNotebook(t, "alice", "Private")
			return func() { notebookMember(t, tm, nb) }
		}, []string{"bob"}, []string{"alice"}},
		{"a notebook member restored", func(t *testing.T, tm acmeTeam) func() {
			nb := tm.createNotebook(t, "alice", "Private")
			tm.send(t, request("alice", http.MethodDelete, "/api/v0/notebook-members/"+notebookMember(t, tm, nb), ""), http.StatusNoContent)
			return func() { notebookMember(t, tm, nb) }
		}, []string{"bob"}, []string{"alice"}},
		{"a notebook member removed", func(t *testing.T, tm acmeTeam) func() {
			id := notebookMember(t, tm, tm.createNotebook(t, "alice", "Private"))
			return func() {
				tm.send(t, request("alice", http.MethodDelete, "/api/v0/notebook-members/"+id, ""), http.StatusNoContent)
			}
		}, []string{"bob"}, []string{"alice"}},
		{"a notebook member leaves", func(t *testing.T, tm acmeTeam) func() {
			nb := tm.createNotebook(t, "alice", "Private")
			notebookMember(t, tm, nb)
			return func() {
				tm.send(t, request("bob", http.MethodPost, "/api/v0/notebooks/"+nb+"/leave", ""), http.StatusNoContent)
			}
		}, []string{"bob"}, []string{"alice"}},
		{"an invitation accepted as a member", func(t *testing.T, tm acmeTeam) func() {
			inv := tm.invite(t, "dana", "member")
			return func() { tm.send(t, accept("dana", inv), http.StatusOK) }
		}, []string{"dana"}, []string{"alice", "bob"}},
		{"a member made a guest", func(t *testing.T, tm acmeTeam) func() {
			return func() {
				tm.send(t, request("alice", http.MethodPatch, "/api/v0/workspace-members/"+tm.members["bob"].String(), `{"role":"guest"}`),
					http.StatusOK)
			}
		}, []string{"bob"}, []string{"alice"}},
		{"a guest made a member", func(t *testing.T, tm acmeTeam) func() {
			return func() {
				tm.send(t, request("alice", http.MethodPatch, "/api/v0/workspace-members/"+tm.members["carol"].String(), `{"role":"member"}`),
					http.StatusOK)
			}
		}, []string{"carol"}, []string{"alice", "bob"}},
		{"a member removed from the workspace", func(t *testing.T, tm acmeTeam) func() {
			return func() { tm.send(t, tm.removal("alice", "bob"), http.StatusNoContent) }
		}, []string{"bob"}, []string{"alice"}},
		{"a member leaves the workspace", func(t *testing.T, tm acmeTeam) func() {
			return func() { tm.leave(t, "bob") }
		}, []string{"bob"}, []string{"alice"}},
		{"a member deactivates their account", func(t *testing.T, tm acmeTeam) func() {
			return func() { tm.send(t, deactivate("bob"), http.StatusNoContent) }
		}, []string{"bob"}, []string{"alice"}},
		{"a member deactivated by command", func(t *testing.T, tm acmeTeam) func() {
			deactivation := tm.deactivateByCommand(t, "bob")
			return func() {
				if err := deactivation.command(); err != nil {
					t.Fatal(err)
				}
			}
		}, []string{"bob"}, []string{"alice"}},
		{"a removed member restored by an invitation, an ownerless notebook returned", func(t *testing.T, tm acmeTeam) func() {
			tm.createNotebook(t, "bob", "Bob's")
			tm.send(t, tm.removal("alice", "bob"), http.StatusNoContent)
			inv := tm.invite(t, "bob", "member")
			return func() { tm.send(t, accept("bob", inv), http.StatusOK) }
		}, []string{"bob"}, []string{"alice"}},
		{"a member who left restored by reactivate-member", func(t *testing.T, tm acmeTeam) func() {
			tm.leave(t, "bob")
			reactivation := tm.reactivateByCommand(t, "bob")
			return func() {
				if err := reactivation.command(); err != nil {
					t.Fatal(err)
				}
			}
		}, []string{"bob"}, []string{"alice"}},
		{"an ownerless notebook taken over", func(t *testing.T, tm acmeTeam) func() {
			nb := tm.createNotebook(t, "bob", "Bob's")
			tm.send(t, tm.removal("alice", "bob"), http.StatusNoContent)
			return func() { tm.send(t, takeOver("alice", nb), http.StatusOK) }
		}, []string{"alice"}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "guest")
			eng := tm.openNotebook(t, "alice", "Eng")
			marker := tm.createPage(t, "alice", eng, "", "Marker")
			trigger := tt.setup(t, tm)
			tm.settle(t, eng, marker)
			streams := map[string]*eventStream{}
			for _, name := range append(append([]string{}, tt.reset...), tt.kept...) {
				streams[name] = openStream(t, tm.base, name, tm.tokens[name])
			}

			trigger()

			for _, name := range tt.reset {
				streams[name].reset(t, "access")
			}
			var kept []*eventStream
			for _, name := range tt.kept {
				kept = append(kept, streams[name])
			}
			tm.quiet(t, eng, marker, kept...)
		})
	}
}
