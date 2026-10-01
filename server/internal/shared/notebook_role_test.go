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
	for _, tt := range []struct {
		explicit shared.NotebookRole
		access   shared.WorkspaceAccess
		ws       shared.WorkspaceRole
		want     shared.NotebookRole
	}{
		// No membership: the access alone, to admins and members.
		{"", none, wsAdmin, ""}, {"", none, member, ""}, {"", none, gst, ""},
		{"", viewer, wsAdmin, reader}, {"", viewer, member, reader}, {"", viewer, gst, ""},
		{"", open, wsAdmin, editor}, {"", open, member, editor}, {"", open, gst, ""},
		// A membership: the higher of the two.
		{reader, none, member, reader}, {reader, viewer, member, reader}, {reader, open, member, editor},
		{editor, viewer, member, editor}, {editor, open, member, editor},
		{admin, none, member, admin}, {admin, open, member, admin}, {admin, viewer, wsAdmin, admin},
		// A guest has its membership's role, never the access's.
		{reader, open, gst, reader}, {admin, none, gst, admin}, {editor, viewer, gst, editor},
		// Values outside the sets give nothing of their own.
		{"owner", none, member, ""}, {"owner", open, member, editor}, {"", "public", member, ""},
		{"", open, "owner", ""}, {"Admin", none, member, ""},
	} {
		if got := shared.EffectiveNotebookRole(tt.explicit, tt.access, tt.ws); got != tt.want {
			t.Errorf("EffectiveNotebookRole(%q, %q, %q) = %q, want %q", tt.explicit, tt.access, tt.ws, got, tt.want)
		}
	}
}
