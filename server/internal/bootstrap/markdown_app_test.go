package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	xhtml "golang.org/x/net/html"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
)

// appMarkdown is the Markdown serve builds, with the extensions it
// registers, a reading view's links leading where m's resolve tells, the
// attachments shown as its assets tells.
func appMarkdown(t *testing.T, m mode) (*markdown.Markdown, []markdown.Extension) {
	t.Helper()
	exts := markdownExtensions(m.resolve, m.assets)
	md, err := markdown.New(exts)
	if err != nil {
		t.Fatal(err)
	}
	return md, exts
}

// mode is where a reading view's links lead in a test of the application's
// Markdown, and what the attachments show.
type mode struct {
	name     string
	resolve  obsidian.Resolve
	assets   obsidian.Assets
	toAssets bool   // every link leads to an attachment
	embed    string // how an embed starts
}

// modes are every link resolved to one page, to none, and to one
// attachment of each kind of markup (M7/P3 design 5.9): an image, of its
// own size, an audio, a video and a PDF, each as large as its markup
// gets, its address as the content route's is (signedPath), and one
// Assets does not answer.
func modes() []mode {
	return []mode{
		{"to a page", everyLink, nil, false, `<a class="nw-wikilink nw-embed" data-nw-node=`},
		{"to none", nil, nil, false, `<a class="nw-wikilink nw-embed nw-unresolved"`},
		{"to an image", everyAsset, shownAs("image/png", maxSide, maxSide), true, `<img class="nw-asset" src=`},
		{"to an audio", everyAsset, shownAs("audio/mpeg", 0, 0), true, `<audio class="nw-asset" src=`},
		{"to a video", everyAsset, shownAs("video/webm", 0, 0), true, `<video class="nw-asset" src=`},
		{"to a PDF", everyAsset, shownAs("application/pdf", 0, 0), true, `<a class="nw-wikilink nw-embed nw-asset" href=`},
		{"to an attachment not shown", everyAsset, nil, true, `<span class="nw-wikilink nw-embed nw-asset">`},
	}
}

// maxSide is the widest an image's own width and height are written.
const maxSide = 10_000

// everyLink resolves every link, to one page: the largest markup of a
// reading view's links but for the targets of those that resolve to none.
func everyLink(_ context.Context, _ markdown.Page, links []obsidian.Link) (map[int]obsidian.Target, error) {
	to := make(map[int]obsidian.Target, len(links))
	for _, l := range links {
		to[l.Range.Start] = obsidian.Target{Node: uuid.Max()}
	}
	return to, nil
}

// everyAsset resolves every link to one attachment.
func everyAsset(_ context.Context, _ markdown.Page, links []obsidian.Link) (map[int]obsidian.Target, error) {
	to := make(map[int]obsidian.Target, len(links))
	for _, l := range links {
		to[l.Range.Start] = obsidian.Target{Node: uuid.Max(), Asset: true}
	}
	return to, nil
}

// shownAs shows every attachment as one of mime, width and height, of the
// largest size, at its signedPath.
func shownAs(mime string, width, height int) obsidian.Assets {
	return func(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]obsidian.Asset, error) {
		out := make(map[uuid.UUID]obsidian.Asset, len(ids))
		for _, id := range ids {
			out[id] = obsidian.Asset{MIME: mime, Bytes: math.MaxInt64, Width: width, Height: height, URL: signedPath(id),
				Expires: time.Unix(math.MaxInt32, 0)}
		}
		return out, nil
	}
}

// signedPath is an address of the attachment id's content as the content
// route signs it (asset's ContentURL): its blob, its expiry and its
// signature, each as long as it gets. The whole program's test reads real
// ones (assets_view_test.go).
func signedPath(id uuid.UUID) string {
	return "/api/v0/assets/" + id.String() + "/content?b=" + uuid.Max().String() + "&e=" + strconv.Itoa(math.MaxInt32) + "&s=" +
		strings.Repeat("A", 22)
}

// aSignedPath is an address signedPath writes, as the HTML has it.
var aSignedPath = regexp.MustCompile(`^/api/v0/assets/[0-9a-f-]{36}/content\?b=[0-9a-f-]{36}&e=[0-9]+&s=[A-Za-z0-9_-]{22}$`)

// checkAttachments checks that no data-nw-node of html names an
// attachment, that is, there is none when every link resolves to one; and
// that each src, and each href of a link to an attachment (nw-asset), is a
// signed path of an attachment's content.
func checkAttachments(html string) error {
	z := xhtml.NewTokenizer(strings.NewReader(html))
	for {
		switch z.Next() {
		case xhtml.ErrorToken:
			return nil
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			t := z.Token()
			asset := slices.ContainsFunc(t.Attr, func(a xhtml.Attribute) bool {
				return a.Key == "class" && slices.Contains(strings.Fields(a.Val), "nw-asset")
			})
			for _, a := range t.Attr {
				switch {
				case a.Key == "data-nw-node":
					return fmt.Errorf("<%s> leads to an attachment as to a page", t.Data)
				case a.Key == "src" || a.Key == "href" && asset:
					if !aSignedPath.MatchString(a.Val) {
						return fmt.Errorf("<%s %s=%q>, not an attachment's signed path", t.Data, a.Key, a.Val)
					}
				}
			}
		}
	}
}

// The application's Markdown renders every fixture and every generated
// input to HTML that passes the check with its extensions' markup, and
// is no larger than CheckSize allows, every link resolved to a page, to
// none, or to an attachment, shown as each kind is or not shown (M4/P3
// design 3.10, M6/P3 design 6.8, M7/P3 design 5.9).
func TestTheAppsMarkdownRendersCheckedHTML(t *testing.T) {
	if markdowntest.Race {
		t.Skip("rendered in every mode without the race detector (make test-go runs it)")
	}
	inputs := map[string]string{"ordinary": markdowntest.Normal(64 << 10)}
	for _, f := range markdowntest.Fixtures(t) {
		inputs[f.Name] = string(f.Content)
	}
	for _, in := range markdowntest.Pathological() {
		inputs[in.Name] = in.Make(16 << 10)
	}
	for _, in := range markdowntest.Amplifying() {
		inputs[in.Name] = in.Make(markdowntest.AmplifyingSize)
	}
	page := markdown.Page{NotebookID: uuid.NewV7(), PageID: uuid.NewV7()}
	for _, m := range modes() {
		md, exts := appMarkdown(t, m)
		// Each mode is what it says: an embed's markup tells where it leads.
		if view, err := md.Render(context.Background(), md.Parse([]byte("![[p.x]]")), page); err != nil ||
			!strings.HasPrefix(view.HTML, "<p>"+m.embed) {
			t.Errorf("%s: an embed is %q, %v; want it to start %s", m.name, view.HTML, err, m.embed)
		}
		for name, content := range inputs {
			view, err := md.Render(context.Background(), md.Parse([]byte(content)), page)
			if err != nil {
				t.Fatalf("%s, %s: %v", m.name, name, err)
			}
			if err := markdowntest.CheckHTML(view.HTML, exts...); err != nil {
				t.Errorf("%s, %s: %v", m.name, name, err)
			}
			if err := markdowntest.CheckSize([]byte(content), view.HTML); err != nil {
				t.Errorf("%s, %s: %v", m.name, name, err)
			}
			if err := checkAttachments(view.HTML); m.toAssets && err != nil {
				t.Errorf("%s, %s: %v", m.name, name, err)
			}
		}
	}
}

// The links, every one resolved to a page, to none or to an attachment of
// each kind, are within the bound of CheckSize without its headroom, which
// a page's largest content would not have (M6/P3B review L3; M7/P3 design
// 5.9): the state each carries, an image its address, and an attachment's
// its markup and signed address, each time it is written or a reference
// used.
func TestTheAppsLinksAreWithinTheirBound(t *testing.T) {
	if markdowntest.Race {
		t.Skip("measured without the race detector (make test-go runs it)")
	}
	const n = 512 << 10
	inputs := map[string]string{
		"images of a short address":           "[x]: p#&\n\n" + strings.Repeat("![x] ", n/5),
		"images of a short address, unspaced": "[x]: p#&\n\n" + strings.Repeat("![x]", n/4),
		"images of an address of '&'":         "[x]: &&&&\n\n" + strings.Repeat("![x] ", n/5),
		"links of a short address":            "[x]: p#b\n\n" + strings.Repeat("[x] ", n/4),
		"wikilinks with anchors":              strings.Repeat("[[a#b]]", n/7),
		"embeds":                              strings.Repeat("![[p]]", n/6),
		"embeds of a size":                    strings.Repeat("![[p|1x1]]", n/10),
		"wikilinks":                           strings.Repeat("[[p]]", n/5),
		"images of a size":                    "[x]: p\n\n" + strings.Repeat("![|1x1][x]", n/10),
		"wikilinks of a long anchor":          strings.Repeat("[[a#"+strings.Repeat("Ⱥ", 64)+"]]", n/134),
		// In the property table, below the YAML's limit of values (M6/P6 design 4).
		"property links of a display of '&'": "---\na: [" + strings.Repeat("'[[&|"+strings.Repeat("&", n/9000-12)+"]]',", 9000) + "]\n---\n",
		"property links":                     "---\na: [" + strings.Repeat("'[["+strings.Repeat("a", n/9000-8)+"]]',", 9000) + "]\n---\n",
	}
	page := markdown.Page{NotebookID: uuid.NewV7(), PageID: uuid.NewV7()}
	for _, m := range modes() {
		md, _ := appMarkdown(t, m)
		for name, content := range inputs {
			view, err := md.Render(context.Background(), md.Parse([]byte(content)), page)
			if err != nil {
				t.Fatalf("%s, %s: %v", m.name, name, err)
			}
			t.Logf("%s, %s: %.1f times", m.name, name, float64(len(view.HTML))/float64(len(content)))
			if len(view.HTML) > markdowntest.Amplification*len(content) {
				t.Errorf("%s, %s: %d bytes of HTML for %d of content, more than %d times", m.name, name, len(view.HTML), len(content),
					markdowntest.Amplification)
			}
		}
	}
}

// The facts of the application's parse keep none of its tree (M6 design
// 4.7): a write keeps them through its unit, its extensions' results
// among them.
func TestTheAppsFactsOutliveTheTree(t *testing.T) {
	markdowntest.CheckFacts(t, markdownExtensions(nil, nil))
}

// nervewiki reindex's Markdown, which only parses and shows no attachment,
// takes of a content what serve's does (M7/P3 design 5.10).
func TestTheReindexsFactsAreTheServers(t *testing.T) {
	cfg := config.Config{Page: config.PageConfig{ParseBudgetBytes: 64 << 10, ParseMaxWait: time.Second}}
	serve, _, err := parsing(cfg, slog.New(slog.DiscardHandler), nil, shownAs("image/png", 1, 1))
	if err != nil {
		t.Fatal(err)
	}
	reindex, _, err := parsing(cfg, slog.New(slog.DiscardHandler), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	contents := [][]byte{[]byte(markdowntest.Normal(16 << 10)), []byte("![[x.png|c|1x2]] ![c](a.mp3) [[d.pdf]]")}
	for _, f := range markdowntest.Fixtures(t) {
		contents = append(contents, f.Content)
	}
	for _, c := range contents {
		if a, b := serve.Parse(c).Facts(), reindex.Parse(c).Facts(); !reflect.DeepEqual(a, b) {
			t.Errorf("%.40q: serve's facts %+v, reindex's %+v", c, a, b)
		}
	}
}

// The server's parse budget is the configuration's (M6 design 4.7): all but
// 8 KiB of it held, 8 KiB are free, and a byte more is busy once the
// configured wait has passed.
func TestTheServersParseBudgetIsTheConfigurations(t *testing.T) {
	cfg := config.Config{Page: config.PageConfig{ParseBudgetBytes: 64 << 10, ParseMaxWait: 50 * time.Millisecond}}
	_, budget, err := parsing(cfg, slog.New(slog.DiscardHandler), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// Takes past the least a take holds: the budget's last 8 KiB fit, and a
	// byte more does not.
	most, err := budget.Take(ctx, 56<<10)
	if err != nil {
		t.Fatal(err)
	}
	defer most.Release()
	last, err := budget.Take(ctx, 8<<10)
	if err != nil {
		t.Fatalf("the budget's last 8 KiB: %v", err)
	}
	last.Release()
	start := time.Now()
	if _, err := budget.Take(ctx, 8<<10+1); !errors.Is(err, markdown.ErrBusy) || time.Since(start) < cfg.Page.ParseMaxWait ||
		time.Since(start) > time.Second {
		t.Errorf("a byte beyond the budget = %v after %s, want busy after %s", err, time.Since(start), cfg.Page.ParseMaxWait)
	}
}

// The application's Markdown costs about the size of what it parses, with
// its extensions' parsers, every link resolved, to a page and to an image,
// the largest markup of an attachment: each must be linear too (M4/P3
// design 3.4; M7/P3 design 5.9).
func TestTheAppsMarkdownCostsAboutItsSize(t *testing.T) {
	for _, m := range modes() {
		if m.name != "to a page" && m.name != "to an image" {
			continue
		}
		t.Run(m.name, func(t *testing.T) {
			md, _ := appMarkdown(t, m)
			markdowntest.CheckCosts(t, md)
		})
	}
}
