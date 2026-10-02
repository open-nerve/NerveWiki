package bootstrap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// A page's content through serve (M4/P4 design 3.8, 3.9): its bytes kept
// as they were written, Unicode the body cannot spell refused, and the two
// routes of a content taking its largest.

// sessionOpening is by's opening of an edit session of the page id.
func sessionOpening(by, id string) step {
	return request(by, http.MethodPost, "/api/v0/pages/"+id+"/edit-sessions", "")
}

// contentWrite is by's write of content to the page id on base, in the
// edit session session unless it is "".
func contentWrite(by, id, content string, base int, session string) step {
	body := map[string]any{"content": content, "base_revision": base}
	if session != "" {
		body["edit_session_id"] = session
	}
	b, _ := json.Marshal(body)
	return request(by, http.MethodPut, "/api/v0/pages/"+id+"/content", string(b))
}

// openSession opens by's edit session of the page id and returns its id.
func (tm acmeTeam) openSession(t *testing.T, by, id string) string {
	t.Helper()
	c := sessionOpening(by, id)
	status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens[by], c.body)
	if status != http.StatusCreated {
		t.Fatalf("open a session of %s as %s = %d %s", id, by, status, body)
	}
	return idOf(t, answer{body: body})
}

// pageContent is a PageContent answer.
type pageContent struct {
	Content  string `json:"content"`
	Revision int    `json:"revision"`
	Hash     string `json:"content_hash"`
}

// content reads the page id's content as by.
func (tm acmeTeam) content(t *testing.T, by, id string) pageContent {
	t.Helper()
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+id+"/content", tm.tokens[by], "")
	if status != http.StatusOK {
		t.Fatalf("GET the content of %s as %s = %d %s", id, by, status, body)
	}
	var c pageContent
	decodeAnswer(t, body, &c)
	return c
}

// Each content comes back byte for byte, created with the page and
// written over it, with the hash and the size of its bytes: no line break,
// byte order mark, blank, invisible character or composition is touched.
func TestAPagesContentKeepsItsBytes(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	for i, content := range []string{
		"# Title\r\n\r\nLine one\r\nLine two\r\n",
		"a\rb\r\rc\r",
		"a\r\nb\nc\rd",
		"\ufeff# Title\n",
		"line   \nnext\t\t\n   \n",
		"\tindented\n\t\tcode\n",
		"a\u00a0b\u00a0\n",
		"a\u200bb\u200cc\u200dd\u2060e\ufefff\n",
		"a\u2028b\u2029c\n",
		"Cafe\u0301 and Caf\u00e9\n",
		"\x01\x07\x1b[31m\x7f <&> end",
	} {
		created := pageCreation("alice", nb, "Page "+string(rune('A'+i)))
		var b map[string]any
		_ = json.Unmarshal([]byte(created.body), &b)
		b["content"] = content
		body, _ := json.Marshal(b)
		created.body = string(body)
		status, answer := ask(t, tm.contract, created.method, tm.base+created.path, tm.tokens["alice"], created.body)
		if status != http.StatusCreated {
			t.Fatalf("create with %q = %d %s", content, status, answer)
		}
		var p struct {
			ID       string `json:"id"`
			ByteSize int    `json:"byte_size"`
		}
		decodeAnswer(t, answer, &p)
		for j, want := range []string{content, content + "\r\n" + content} {
			if j == 1 {
				tm.send(t, contentWrite("alice", p.ID, want, 1, ""), http.StatusOK)
				p.ByteSize = count(t, tm.pool, "SELECT byte_size FROM page_contents WHERE node_id = $1", p.ID)
			}
			sum := sha256.Sum256([]byte(want))
			if got := tm.content(t, "alice", p.ID); got.Content != want || got.Hash != hex.EncodeToString(sum[:]) || p.ByteSize != len(want) {
				t.Errorf("wrote %q, read %q, its hash %s and size %d; want it with %x and %d", want, got.Content, got.Hash, p.ByteSize, sum, len(want))
			}
		}
	}
	checkPages(t, tm.pool)
}

// A body whose content is no text, a surrogate's half alone or bytes that
// are not UTF-8, is refused before any write: 400, the page as it was. A
// pair of halves is its character.
func TestAContentOfBrokenUnicodeIsRefused(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	id := tm.createPage(t, "alice", nb, "", "Notes")
	path := tm.base + "/api/v0/pages/" + id + "/content"
	for _, content := range []string{`a\ud800b`, `\ud83d`, `\udc00`, `\ude00\ud83d`, "a\xffb", "\xe2\x82", "\xc0\xaf"} {
		status, answer := ask(t, tm.contract, http.MethodPut, path, tm.tokens["alice"], `{"content":"`+content+`","base_revision":1}`)
		if status != http.StatusBadRequest {
			t.Errorf("PUT the content %q = %d %s, want 400", content, status, answer)
		}
	}
	if got := tm.content(t, "alice", id); got.Revision != 1 || got.Content != "" {
		t.Errorf("the content is %+v after the refusals, want it empty at revision 1", got)
	}
	tm.send(t, request("alice", http.MethodPut, "/api/v0/pages/"+id+"/content", `{"content":"\ud83d\ude00","base_revision":1}`), http.StatusOK)
	if got := tm.content(t, "alice", id); got.Content != "\U0001F600" {
		t.Errorf("the pair of halves wrote %q, want U+1F600", got.Content)
	}
}

// The two routes of a content take its largest, each byte written as
// JSON's longest escape (M4/P4 design 3.8): one byte more is the content's
// 422; a body beyond their limit, and any other route's beyond
// server.max_body_bytes, 413. The app has the base configuration's
// deadlines: the race detector makes decoding 30 MB of escapes take seconds.
func TestTheContentsRoutesTakeTheLargestContent(t *testing.T) {
	tm := newAcmeTeamWith(t, "member", "", func(c *config.Config) {
		c.Server.ReadTimeout, c.Server.WriteTimeout, c.Server.RequestTimeout = 30*time.Second, time.Minute, 15*time.Second
	})
	nb := tm.openNotebook(t, "alice", "Eng")
	id := tm.createPage(t, "alice", nb, "", "Notes")
	largest := strings.Repeat("\x01", domain.MaxContentBytes)
	creation := func(content string) string {
		b, _ := json.Marshal(map[string]any{"parent_id": nil, "title": "Large", "content": content})
		return string(b)
	}
	put := contentWrite("alice", id, largest, 1, "")
	for _, c := range []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"the largest content written", put.method, put.path, put.body, http.StatusOK, ""},
		{"the largest content created", http.MethodPost, "/api/v0/notebooks/" + nb + "/pages", creation(largest), http.StatusCreated, ""},
		{"a byte more written", put.method, put.path, contentWrite("alice", id, largest+"\x01", 2, "").body, http.StatusUnprocessableEntity,
			"validation_failed"},
		{"a byte more created", http.MethodPost, "/api/v0/notebooks/" + nb + "/pages", creation(largest + "\x01"),
			http.StatusUnprocessableEntity, "validation_failed"},
		{"a body beyond the route's limit", put.method, put.path, contentWrite("alice", id, strings.Repeat("\x01", domain.MaxContentBytes+11<<10), 2, "").body,
			http.StatusRequestEntityTooLarge, "payload_too_large"},
		{"another route's body beyond the server's", http.MethodPatch, "/api/v0/nodes/" + id,
			`{"name":"` + strings.Repeat("a", 1<<20) + `"}`, http.StatusRequestEntityTooLarge, "payload_too_large"},
	} {
		status, answer := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["alice"], c.body)
		code := ""
		if status >= http.StatusBadRequest {
			code = problemCode(t, answer)
		}
		if status != c.status || code != c.code || c.code == "validation_failed" && !strings.Contains(answer, `"content"`) {
			t.Errorf("%s = %d %.300s, want %d %s", c.name, status, answer, c.status, c.code)
		}
	}
	if got := tm.content(t, "alice", id); got.Revision != 2 || got.Content != largest {
		t.Errorf("the content is at revision %d, %d bytes; want the largest at revision 2", got.Revision, len(got.Content))
	}
	checkPages(t, tm.pool)
}
