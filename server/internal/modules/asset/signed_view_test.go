package asset

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
	"uuid"

	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/mac"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/tasks"
)

// signedViewSample is the sample of a reading view's attachments' addresses
// that the web reads too (web/apps/web/src/reading/patch.test.ts).
const signedViewSample = "../../../../testdata/m12/signed-view.json"

// sampleContent is the sample page's: an image of its own size, an audio, a
// video, and a link to a file the browser downloads, each an attachment of
// the notebook.
const sampleContent = "# Field notes\n\n![[chart.png]]\n\n![[talk.mp3]]\n\n![[clip.webm]]\n\nThe figures are in [[report.zip]].\n"

// sampleAttachments are the sample's attachments by name: their nodes' and
// their blobs' ids, their types, sizes and images' sides.
func sampleAttachments() map[string]domain.Blob {
	return map[string]domain.Blob{
		"chart.png":  sampleBlob("01", "image/png", 2048, 640, 480),
		"talk.mp3":   sampleBlob("02", "audio/mpeg", 4096, 0, 0),
		"clip.webm":  sampleBlob("03", "video/webm", 8192, 0, 0),
		"report.zip": sampleBlob("04", "application/zip", 1024, 0, 0),
	}
}

const (
	sampleNotebook = "0199a2b4-0000-7000-8000-0000000000c1"
	samplePage     = "0199a2b4-0000-7000-8000-0000000000d1"
	// sampleKey is the content key the sample's addresses are signed with.
	sampleKey = "the M12 sample view's content key"
)

// sampleBlob is an attachment of the sample notebook: its node's id ends
// with n, its blob's with n too, in another block.
func sampleBlob(n, mime string, size int64, width, height int) domain.Blob {
	return domain.Blob{
		ID:         uuid.MustParse("0199a2b4-0000-7000-8000-0000000b00" + n),
		NodeID:     uuid.MustParse("0199a2b4-0000-7000-8000-0000000a00" + n),
		NotebookID: uuid.MustParse(sampleNotebook),
		MIME:       mime, Bytes: size, Width: width, Height: height,
	}
}

// sampleRows reads the sample's attachments' rows.
type sampleRows struct{}

func (sampleRows) CreateBlob(context.Context, domain.Blob) error { return nil }

func (sampleRows) BlobOfNode(_ context.Context, id uuid.UUID) (domain.Blob, error) {
	for _, b := range sampleAttachments() {
		if b.NodeID == id {
			return b, nil
		}
	}
	return domain.Blob{}, app.ErrNoRow
}

func (r sampleRows) BlobsOfNodes(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.Blob, error) {
	out := map[uuid.UUID]domain.Blob{}
	for _, id := range ids {
		if b, err := r.BlobOfNode(ctx, id); err == nil {
			out[id] = b
		}
	}
	return out, nil
}

// at is a clock stopped at a time.
type at time.Time

func (a at) Now() time.Time { return time.Time(a) }

// sampleView is the sample page's reading view as of now, as the server
// writes it: the composition root's extensions (tasks and obsidian, as
// bootstrap's markdownExtensions builds them), its links resolving to the
// attachments, which the module's Embeds shows, converted as bootstrap's
// assetEmbeds.assets does, signed with sampleKey. Those two are written
// out again here: the root's Embeds has no exported fields and is made
// from a pool alone, so the composition root would need a database, or
// the part of Embeds.Of that calls ContentURL, the spelling the sample
// holds; each says to change this test with it.
func sampleView(t *testing.T, now time.Time) markdown.View {
	t.Helper()
	embeds := Embeds{embeds: app.NewEmbeds(sampleRows{}, macadapter.New([]byte(sampleKey)), at(now))}
	attachments := sampleAttachments()
	resolve := func(_ context.Context, _ markdown.Page, links []obsidian.Link) (map[int]obsidian.Target, error) {
		to := map[int]obsidian.Target{}
		for _, l := range links {
			if b, ok := attachments[l.Target]; ok {
				to[l.Range.Start] = obsidian.Target{Node: b.NodeID, Asset: true}
			}
		}
		return to, nil
	}
	assets := func(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]obsidian.Asset, error) {
		shown, err := embeds.Of(ctx, notebookID, ids)
		if err != nil {
			return nil, err
		}
		out := make(map[uuid.UUID]obsidian.Asset, len(shown))
		for id, e := range shown {
			out[id] = obsidian.Asset(e)
		}
		return out, nil
	}
	md, err := markdown.New([]markdown.Extension{
		tasks.Extension(), obsidian.Extension(obsidian.Options{Resolve: resolve, Assets: assets}),
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err := md.Render(context.Background(), md.Parse([]byte(sampleContent)),
		markdown.Page{NotebookID: uuid.MustParse(sampleNotebook), PageID: uuid.MustParse(samplePage), Revision: 1})
	if err != nil {
		t.Fatal(err)
	}
	return view
}

// The reading view's attachments' addresses as the web reads them (M12/P2/S4,
// P2-45): the same page's view at two times an hour apart, as the renderer,
// the module's Embeds, ContentURL and the content key's signer write it, is
// testdata/m12/signed-view.json byte for byte. The web's patch.test.ts reads
// the same file and finds the two views alike but for their addresses'
// expiry and signature: how an address is spelled changing on either side,
// one of the two tests fails.
func TestTheSignedViewSample(t *testing.T) {
	data, err := os.ReadFile(signedViewSample)
	if err != nil {
		t.Fatal(err)
	}
	var sample struct {
		Views []struct {
			At             time.Time `json:"at"`
			HTML           string    `json:"html"`
			AssetsExpireAt time.Time `json:"assets_expire_at"`
		} `json:"views"`
	}
	if err := json.Unmarshal(data, &sample); err != nil {
		t.Fatal(err)
	}
	if len(sample.Views) != 2 || sample.Views[1].At.Sub(sample.Views[0].At) != time.Hour {
		t.Fatalf("the sample has %d views, not two an hour apart", len(sample.Views))
	}
	for _, want := range sample.Views {
		got := sampleView(t, want.At)
		if got.HTML != want.HTML {
			t.Errorf("the view as of %s:\n%s\nthe sample's:\n%s", want.At.Format(time.RFC3339), got.HTML, want.HTML)
		}
		if !got.Expires.Equal(want.AssetsExpireAt) {
			t.Errorf("the view as of %s expires at %s, the sample's at %s", want.At.Format(time.RFC3339), got.Expires, want.AssetsExpireAt)
		}
		// Each attachment's address is in it: the sample holds what the web is to read.
		for name, b := range sampleAttachments() {
			if !strings.Contains(want.HTML, "/api/v0/assets/"+b.NodeID.String()+"/content?b="+b.ID.String()+"&amp;e=") {
				t.Errorf("the sample's view as of %s has no address of %s", want.At.Format(time.RFC3339), name)
			}
		}
	}
	if sample.Views[0].HTML == sample.Views[1].HTML {
		t.Error("the sample's two views are the same: their addresses were signed for the same hour")
	}
}
