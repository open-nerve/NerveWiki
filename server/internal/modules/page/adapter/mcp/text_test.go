package mcpadapter

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// fitOf is the words of a tool's error with k of its items, as mcpserver
// asks a fit for them: the text and the members it adds, as JSON.
func fitOf(t *testing.T, err error, k int) (string, string) {
	t.Helper()
	var te *mcpserver.ToolError
	if !errors.As(err, &te) || te.Fit == nil {
		t.Fatalf("%v has no fit", err)
	}
	text, extra := te.Fit.Build(k)
	return text, membersJSON(t, extra)
}

// membersJSON is extra as JSON, its members sorted by name.
func membersJSON(t *testing.T, extra map[string]any) string {
	t.Helper()
	if extra == nil {
		return ""
	}
	b, err := json.Marshal(extra)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// pagesLocked is linking.pages_locked naming locks, as linking's
// PagesLocked makes it: page's adapter answers any refusal of locks but
// page.locked as the rewrite's of a move.
func pagesLocked(locks ...shared.LockHolder) error {
	e := shared.NewError(shared.KindConflict, "linking.pages_locked", "Pages whose links the change would write again are being edited.")
	e.Locks = locks
	return e
}

// A path that names nothing is worded with the path as it was written and
// its hint (M9/P1 design 3.10.5), each quoted as it is: the characters a
// title may hold that %q escapes, U+3000, U+00A0, U+200D and U+00AD, are
// the path's own (M9/P2 design 3.10.4); page.not_found stays its code; any
// other error is the use case's. The hint goes when the answer has no room
// for it (a fit of one), and to's parent is told as move's.
func TestAPathThatNamesNothingIsWorded(t *testing.T) {
	for _, tt := range []struct {
		missing *domain.PathNotFound
		want    string
	}{
		{&domain.PathNotFound{Path: "A/B/"}, `page.not_found: Nothing is at "A/B/".`},
		{&domain.PathNotFound{Path: "Guide", Hint: domain.PageHint("Guide.md")},
			`page.not_found: Nothing is at "Guide". A page's path ends with .md: "Guide.md".`},
		{&domain.PathNotFound{Path: "A\u3000B/x\u00a0y.md"}, "page.not_found: Nothing is at \"A\u3000B/x\u00a0y.md\"."},
		{&domain.PathNotFound{Path: "a\u200db", Hint: domain.PageHint("a\u200db.md")},
			"page.not_found: Nothing is at \"a\u200db\". A page's path ends with .md: \"a\u200db.md\"."},
		{&domain.PathNotFound{Path: "so\u00adft.md", Hint: domain.AttachmentHint("so\u00adft")},
			"page.not_found: Nothing is at \"so\u00adft.md\". That is an attachment: its path is \"so\u00adft\"."},
		{&domain.PathNotFound{Path: "B.md", Hint: domain.MoveParentHint, ParentOf: "to"},
			`page.not_found: Nothing is at "B.md", to's parent. move creates no parent pages: write the parent first, ` +
				`or move to an existing page's path.`},
	} {
		err := toolErr(tt.missing, mcpserver.Call{}, "")
		if text, _ := wordsOf(err); text != tt.want || !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("toolErr(%+v) = %q, want the text %q", tt.missing, text, tt.want)
		}
	}
	err := toolErr(&domain.PathNotFound{Path: "B.md", Hint: domain.MoveParentHint, ParentOf: "to"}, mcpserver.Call{}, "")
	if text, extra := fitOf(t, err, 0); text != `page.not_found: Nothing is at "B.md", to's parent.` || extra != "" {
		t.Errorf("the not found without room for its hint = %q, %s", text, extra)
	}
	for _, err := range []error{errors.New("boom"), domain.ErrNotFound} {
		var te *mcpserver.ToolError
		if got := toolErr(err, mcpserver.Call{}, "A.md"); errors.As(got, &te) || !errors.Is(got, err) {
			t.Errorf("toolErr(%v) = %#v, want it as it is", err, got)
		}
	}
}

// Another module's tool says a page's path that names nothing in page's
// tools' words, and an attachment's path where a page's is asked for, each
// quoted as it was written; page.not_found stays the code, any other error
// is as it is (M9/P4 design 3.9.1).
func TestAnotherModulesPathIsWordedAsPagesTools(t *testing.T) {
	err := PathErr(&domain.PathNotFound{Path: "Guide", Hint: domain.PageHint("Guide.md")})
	if text, _ := wordsOf(err); text != `page.not_found: Nothing is at "Guide". A page's path ends with .md: "Guide.md".` ||
		!errors.Is(err, domain.ErrNotFound) {
		t.Errorf("PathErr = %q, %v", text, err)
	}
	boom := errors.New("boom")
	var te *mcpserver.ToolError
	if got := PathErr(boom); errors.As(got, &te) || !errors.Is(got, boom) {
		t.Errorf("PathErr(%v) = %#v, want it as it is", boom, got)
	}
	err = NotAPage("A/x\u3000y.png")
	if text, _ := wordsOf(err); text != "page.not_found: \"A/x\u3000y.png\" is an attachment; a page's path ends with .md." ||
		!errors.Is(err, domain.ErrNotFound) {
		t.Errorf("NotAPage = %q, %v", text, err)
	}
}

// The writes' errors in their words (M9/P2 design 3.11.5): the text quotes
// the call's own path, or the path the error is at, as it is; the
// structure adds what the module alone knows, the version there now and
// the locks with their paths.
func TestTheWritesErrorsAreWorded(t *testing.T) {
	alice, bob, carol := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	me := mcpserver.Call{UserID: alice}
	p1, p2, p3 := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	token := domain.VersionToken{Node: p1, Revision: 4}
	paths := map[uuid.UUID]string{p1: "A/B.md", p3: "A/C\u3000D.md"}
	for _, tt := range []struct {
		name  string
		err   error
		code  string
		text  string
		extra string
	}{
		{"a deletion of a page with children", toolErr(&domain.Recursive{Field: "recursive", Pages: 3, Assets: 2}, me, "A.md"), "validation_failed",
			`validation_failed: "A.md" has 3 pages and 2 attachments under it: set recursive to true to put them in the trash with it.`,
			""},
		{"a deletion of a page with one child", toolErr(&domain.Recursive{Field: "recursive", Pages: 1}, me, "a\u00a0b.md"), "validation_failed",
			"validation_failed: \"a\u00a0b.md\" has 1 page under it: set recursive to true to put them in the trash with it.", ""},
		{"a write's page there", writeErr(&domain.PathTaken{Path: "A/B.md"}, me, "A/B.md"), "page.title_taken",
			`page.title_taken: "A/B.md" is a page already. To replace its content, write it with base_version, the version read gave.`,
			""},
		{"a write's attachment there", writeErr(&domain.PathTaken{Path: "A/x.png.md", Asset: true}, me, "A/x.png.md"),
			"page.title_taken", `page.title_taken: An attachment holds the name of "A/x.png.md" there: a page there needs another title.`,
			""},
		{"a move's to a page holds", moveErr(&domain.PathTaken{Path: "B/x\u200dy.md"}, me, "A/x.md"), "page.title_taken",
			"page.title_taken: \"B/x\u200dy.md\" is taken: a page there has that title. to is the full new path of what moves, never " +
				"a page to move it into.", ""},
		{"a move's to an attachment holds", moveErr(&domain.PathTaken{Path: "B/x.png", Asset: true}, me, "A/x.png"),
			"page.title_taken", `page.title_taken: "B/x.png" is taken: an attachment there has that name. to is the full new path of ` +
				`what moves, never a page to move it into.`, ""},
		{"another page at the path", toolErr(&domain.VersionMismatch{Path: "A/B.md", Current: token, Why: domain.OtherNode}, me,
			"A/B.md"), "page.revision_mismatch", `page.revision_mismatch: "A/B.md" is another page now than the one base_version was ` +
			`read from: read it, and write from what you read. Its version is "` + token.String() + `".`,
			`{"current_version":"` + token.String() + `"}`},
		{"a page changed", toolErr(&domain.VersionMismatch{Path: "A/B.md", Current: token, Why: domain.Changed}, me, "A/B.md"),
			"page.revision_mismatch", `page.revision_mismatch: "A/B.md" changed since base_version: read it again, and write from ` +
				`what you read. Its version is "` + token.String() + `".`, `{"current_version":"` + token.String() + `"}`},
		{"a page that kept changing", toolErr(&domain.VersionMismatch{Path: "A/B.md", Current: token, Why: domain.KeptChanging}, me,
			"A/B.md"), "page.revision_mismatch", `page.revision_mismatch: "A/B.md" changed twice while the edit was made: read it ` +
			`again, and edit what you read. Its version is "` + token.String() + `".`, `{"current_version":"` + token.String() + `"}`},
		{"another's lock", toolErr(&domain.LockPaths{Err: domain.Locked(p1, bob, "Bob"), Paths: paths}, me, "a/b.md"),
			"page.locked", `page.locked: "a/b.md" is being edited by Bob in the web editor; try again after they leave edit mode. The ` +
				`agent cannot take the lock over.`, `{"lock":{"page_id":"` + p1.String() + `","path":"A/B.md","user_id":"` +
				bob.String() + `","display_name":"Bob","you":false}}`},
		{"the caller's own lock, its path not read", toolErr(&domain.LockPaths{Err: domain.Locked(p2, alice, "Alice")}, me, "A/E.md"),
			"page.locked", `page.locked: "A/E.md" is being edited by you, in a browser; try again after you leave edit mode. The ` +
				`agent cannot take the lock over.`, `{"lock":{"page_id":"` + p2.String() + `","path":null,"user_id":"` +
				alice.String() + `","display_name":"Alice","you":true}}`},
		{"a deletion's locks", toolErr(&domain.LockPaths{Err: domain.LockedAll([]shared.LockHolder{{PageID: p1, UserID: bob,
			DisplayName: "Bob"}, {PageID: p2, UserID: bob, DisplayName: "Bob"}, {PageID: p3, UserID: alice, DisplayName: "Alice"}}),
			Paths: paths}, me, "A.md"), "page.locked", `page.locked: "A.md" has 3 pages being edited in the web editor, by Bob ` +
			`and by you, in a browser: deleting it would end their edits. Try again after they leave edit mode; the agent cannot ` +
			`take the locks over.`, `{"lock":{"page_id":"` + p1.String() + `","user_id":"` + bob.String() +
			`","display_name":"Bob","you":false},"locks":[{"page_id":"` + p1.String() + `","path":"A/B.md","user_id":"` +
			bob.String() + `","display_name":"Bob","you":false},{"page_id":"` + p2.String() + `","path":null,"user_id":"` +
			bob.String() + `","display_name":"Bob","you":false},{"page_id":"` + p3.String() + `","path":"` + paths[p3] + `",` +
			`"user_id":"` + alice.String() + `","display_name":"Alice","you":true}],"locks_count":3}`},
		{"a move's rewrite of pages in edit sessions", toolErr(&domain.LockPaths{Err: pagesLocked(
			shared.LockHolder{PageID: p1, UserID: carol, DisplayName: "Carol"}, shared.LockHolder{PageID: p3, UserID: alice,
				DisplayName: "Alice"}), Paths: paths}, me, "A\u00ad.md"), "linking.pages_locked",
			"linking.pages_locked: Moving \"A\u00ad.md\" would rewrite links in 2 pages being edited in the web editor, by Carol " +
				"and by you, in a browser. Try again after they leave edit mode, or move with update_links false to leave their " +
				"links as they are; the agent cannot take the locks over.", `{"locks":[{"page_id":"` + p1.String() +
				`","path":"A/B.md","user_id":"` + carol.String() + `","display_name":"Carol","you":false},{"page_id":"` +
				p3.String() + `","path":"` + paths[p3] + `","user_id":"` + alice.String() + `","display_name":"Alice","you":true}],` +
				`"locks_count":2}`},
	} {
		text, extra := wordsOf(tt.err)
		if code, _ := mcpserver.ErrorAnswerBytes(tt.err); code != tt.code || text != tt.text {
			t.Errorf("%s: %s %q, want %s %q", tt.name, code, text, tt.code, tt.text)
		}
		if got := membersJSON(t, extra); got != tt.extra {
			t.Errorf("%s: the members are %s, want %s", tt.name, got, tt.extra)
		}
	}
}

// With no room for them, a lock goes without its path and a list without
// its items, its count kept (M9/P2 design 3.10.4): a fit's k of none.
func TestALocksPathsGoFirstWhenTheyDoNotFit(t *testing.T) {
	alice, bob, page := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	me := mcpserver.Call{UserID: alice}
	paths := map[uuid.UUID]string{page: "A/B.md"}
	one := toolErr(&domain.LockPaths{Err: domain.Locked(page, bob, "Bob"), Paths: paths}, me, "A/B.md")
	if _, extra := fitOf(t, one, 0); extra != `{"lock":{"page_id":"`+page.String()+`","user_id":"`+bob.String()+
		`","display_name":"Bob","you":false}}` {
		t.Errorf("a lock with no room for its path = %s", extra)
	}
	holder := shared.LockHolder{PageID: page, UserID: bob, DisplayName: "Bob"}
	all := toolErr(&domain.LockPaths{Err: domain.LockedAll([]shared.LockHolder{holder}), Paths: paths}, me, "A.md")
	if _, extra := fitOf(t, all, 0); extra != `{"lock":{"page_id":"`+page.String()+`","user_id":"`+bob.String()+
		`","display_name":"Bob","you":false},"locks":[],"locks_count":1}` {
		t.Errorf("a deletion's locks with no room for them = %s", extra)
	}
	var te *mcpserver.ToolError
	many := make([]shared.LockHolder, 25)
	for i := range many {
		many[i] = shared.LockHolder{PageID: uuid.NewV7(), UserID: bob, DisplayName: "Bob"}
	}
	if !errors.As(toolErr(&domain.LockPaths{Err: pagesLocked(many...)}, me, "A.md"), &te) || te.Fit.N != mcpserver.MaxErrorLocks {
		t.Errorf("25 locks fit %+v, want at most %d items", te, mcpserver.MaxErrorLocks)
	}
}

// The holders are named each once, in their locks' order, three at most
// and the others counted, the caller last as "you, in a browser"; a
// holder with no name to give is someone (M9 design 4.7).
func TestTheHoldersAreNamed(t *testing.T) {
	me := uuid.NewV7()
	people := map[string]uuid.UUID{"you": me}
	for _, n := range []string{"Ann", "Ben", "Cat", "Dan", "Eve", ""} {
		people[n] = uuid.NewV7()
	}
	locks := func(names ...string) []shared.LockHolder {
		out := make([]shared.LockHolder, len(names))
		for i, n := range names {
			out[i] = shared.LockHolder{PageID: uuid.NewV7(), UserID: people[n], DisplayName: n}
		}
		return out
	}
	for _, tt := range []struct {
		locks []shared.LockHolder
		want  string
	}{
		{locks("Ann"), "by Ann"},
		{locks("you"), "by you, in a browser"},
		{locks("Ann", "you", "Ann"), "by Ann and by you, in a browser"},
		{locks("Ann", "Ben", "you"), "by Ann, Ben and by you, in a browser"},
		{locks("you", "Ann", "Ben", "Cat"), "by Ann, Ben, 1 other and by you, in a browser"},
		{locks("Ann", "Ben", "Cat"), "by Ann, Ben and Cat"},
		{locks("Ann", "Ben", "Cat", "Dan", "Eve"), "by Ann, Ben, Cat and 2 others"},
		{locks(""), "by someone"},
	} {
		if got := holdersText(tt.locks, me); got != tt.want {
			t.Errorf("holdersText(%v) = %q, want %q", tt.locks, got, tt.want)
		}
	}
}

// Each error of the writes fits in an answer at its worst (M9/P2 design
// 3.10.3, 3.10.5, review A-M6): paths of 2,818 bytes of "&", the costliest
// byte, the call's and the error's; 25 locks, each holder's name 100 "&"
// and each page's path ten levels of "&"; each measured as mcpserver
// answers it, its fit's search included, and answered with its code.
func TestEachErrorFitsAtItsWorst(t *testing.T) {
	segment := strings.Repeat("&", 255)
	worst := strings.Repeat(segment+"/", 10) + segment + ".md"
	tenth := strings.Repeat(segment+"/", 9) + segment + ".md"
	name := strings.Repeat("&", 100)
	if len(worst) != 2818 || len(tenth) != 2562 {
		t.Fatalf("the paths are %d and %d bytes", len(worst), len(tenth))
	}
	me := mcpserver.Call{UserID: uuid.NewV7()}
	token := domain.VersionToken{Node: uuid.NewV7(), Revision: domain.MaxRevision}
	holders := make([]shared.LockHolder, 25)
	paths := map[uuid.UUID]string{}
	for i := range holders {
		holders[i] = shared.LockHolder{PageID: uuid.NewV7(), UserID: uuid.NewV7(), DisplayName: name}
		paths[holders[i].PageID] = tenth
	}
	one := holders[0]
	for _, tt := range []struct {
		name string
		err  error
		code string
	}{
		{"a page's hint", toolErr(&domain.PathNotFound{Path: worst, Hint: domain.PageHint(worst)}, me, worst), "page.not_found"},
		{"an attachment's hint", toolErr(&domain.PathNotFound{Path: worst, Hint: domain.AttachmentHint(worst)}, me, worst),
			"page.not_found"},
		{"a title's hint", toolErr(&domain.PathNotFound{Path: worst, Hint: domain.TitleHint(worst)}, me, worst), "page.not_found"},
		{"the children's hint", toolErr(&domain.PathNotFound{Path: worst, Hint: domain.ChildrenHint(worst)}, me, worst),
			"page.not_found"},
		{"to's parent", toolErr(&domain.PathNotFound{Path: worst, Hint: domain.MoveParentHint, ParentOf: "to"}, me, worst),
			"page.not_found"},
		{"recursive", toolErr(&domain.Recursive{Field: "recursive", Pages: 1 << 30, Assets: 1 << 30}, me, worst), "validation_failed"},
		{"a write's page there", writeErr(&domain.PathTaken{Path: worst}, me, worst), "page.title_taken"},
		{"a write's attachment there", writeErr(&domain.PathTaken{Path: worst, Asset: true}, me, worst), "page.title_taken"},
		{"a move's to taken", moveErr(&domain.PathTaken{Path: worst, Asset: true}, me, worst), "page.title_taken"},
		{"another page", toolErr(&domain.VersionMismatch{Path: worst, Current: token, Why: domain.OtherNode}, me, worst),
			"page.revision_mismatch"},
		{"changed", toolErr(&domain.VersionMismatch{Path: worst, Current: token, Why: domain.Changed}, me, worst),
			"page.revision_mismatch"},
		{"kept changing", toolErr(&domain.VersionMismatch{Path: worst, Current: token, Why: domain.KeptChanging}, me, worst),
			"page.revision_mismatch"},
		{"a lock", toolErr(&domain.LockPaths{Err: domain.Locked(one.PageID, one.UserID, name), Paths: map[uuid.UUID]string{
			one.PageID: worst}}, me, worst), "page.locked"},
		{"a deletion's locks", toolErr(&domain.LockPaths{Err: domain.LockedAll(holders), Paths: paths}, me, worst), "page.locked"},
		{"a move's locks", toolErr(&domain.LockPaths{Err: pagesLocked(holders...), Paths: paths}, me, worst), "linking.pages_locked"},
	} {
		code, n := mcpserver.ErrorAnswerBytes(tt.err)
		if code != tt.code || n > mcpserver.MaxAnswerBytes {
			t.Errorf("%s: %s of %d bytes, want %s within %d", tt.name, code, n, tt.code, mcpserver.MaxAnswerBytes)
		}
		t.Logf("%s: %s, %d bytes", tt.name, code, n)
	}
}
