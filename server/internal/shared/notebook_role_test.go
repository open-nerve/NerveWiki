package shared_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestEffectiveNotebookRole(t *testing.T) {
	const (
		admin, editor, reader = shared.NotebookAdmin, shared.NotebookEditor, shared.NotebookReader
		none, viewer, open    = shared.AccessNone, shared.AccessViewer, shared.AccessEditor
		wsAdmin, member, gst  = shared.WorkspaceAdmin, shared.WorkspaceMember, shared.WorkspaceGuest
	)
	// Every explicit role, none first, against every access and workspace
	// role: an admin or a member gets the higher of its membership's and
	// the access's, a guest its membership's alone.
	explicit := []shared.NotebookRole{"", reader, editor, admin}
	for _, tt := range []struct {
		ws     shared.WorkspaceRole
		access shared.WorkspaceAccess
		want   [4]shared.NotebookRole // by explicit
	}{
		{wsAdmin, none, [4]shared.NotebookRole{"", reader, editor, admin}},
		{wsAdmin, viewer, [4]shared.NotebookRole{reader, reader, editor, admin}},
		{wsAdmin, open, [4]shared.NotebookRole{editor, editor, editor, admin}},
		{member, none, [4]shared.NotebookRole{"", reader, editor, admin}},
		{member, viewer, [4]shared.NotebookRole{reader, reader, editor, admin}},
		{member, open, [4]shared.NotebookRole{editor, editor, editor, admin}},
		{gst, none, [4]shared.NotebookRole{"", reader, editor, admin}},
		{gst, viewer, [4]shared.NotebookRole{"", reader, editor, admin}},
		{gst, open, [4]shared.NotebookRole{"", reader, editor, admin}},
	} {
		for i, e := range explicit {
			if got := shared.EffectiveNotebookRole(e, tt.access, tt.ws); got != tt.want[i] {
				t.Errorf("EffectiveNotebookRole(%q, %q, %q) = %q, want %q", e, tt.access, tt.ws, got, tt.want[i])
			}
		}
	}
	// Values outside the sets give nothing of their own.
	for _, tt := range []struct {
		explicit shared.NotebookRole
		access   shared.WorkspaceAccess
		ws       shared.WorkspaceRole
		want     shared.NotebookRole
	}{
		{"owner", none, member, ""}, {"owner", open, member, editor}, {"", "public", member, ""},
		{"", open, "owner", ""}, {"Admin", none, member, ""},
	} {
		if got := shared.EffectiveNotebookRole(tt.explicit, tt.access, tt.ws); got != tt.want {
			t.Errorf("EffectiveNotebookRole(%q, %q, %q) = %q, want %q", tt.explicit, tt.access, tt.ws, got, tt.want)
		}
	}
}
