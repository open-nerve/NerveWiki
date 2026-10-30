package mcpadapter

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

func TestLintPreservesIdentityExtremes(t *testing.T) {
	data := lintDataSet(t, 1, 1)
	// The cursor's allowed int64 count reaches its full decimal width. This
	// is a boundary value of the actual codec, not a claim to seed 2^63 pages.
	data.rows[0].Items[0].Pages = math.MaxInt64
	data.positions[domain.RuleWanted][0].Pages = math.MaxInt64
	data.rows[0].Count = domain.LintCount{Value: 10000, Capped: true}
	invalid := &data.rows[7].Items[0]
	invalid.Target, invalid.TargetCut, invalid.NotLink = "42", false, true
	var header [9]byte
	header[0] = 'J'
	binary.BigEndian.PutUint64(header[1:], 2)
	sum := sha256.Sum256(append(header[:], []byte("42")...))
	digest := hex.EncodeToString(sum[:])
	invalid.ItemKey = "s:" + invalid.PageID.String() + ":" + digest
	data.positions[domain.RuleInvalidSources][0].Hash = digest
	source := &data.rows[6].Items[0]
	source.Bytes = 0
	source.CreatedAt = lintTime().Add(999999999 * time.Nanosecond)
	path := lintLongPath(t, 10, 600, true)
	source.Path = &path
	data.positions[domain.RuleUndigestedSources][0].CreatedAt = source.CreatedAt
	for i := range data.rows {
		row := &data.rows[i]
		cursor, err := (lintCodec{}).Encode(app.LintScope{NotebookID: lintID(10)}, row.Rule, data.positions[row.Rule][0])
		if err != nil {
			t.Fatal(err)
		}
		row.Items[0].Cursor = cursor
	}
	f := newLintRuntime(t, data)
	server := lintServer(t, f.lint)
	for _, index := range []int{0, 6, 7} {
		row := data.rows[index]
		answer, size := lintCall(t, server, true, "lint", map[string]any{"rule": string(row.Rule)})
		var page lintReadPage
		if err := json.Unmarshal(answer.Structured, &page); err != nil {
			t.Fatal(err)
		}
		if len(page.Data) != 1 || page.Next != nil {
			t.Fatal("identity extreme lost the first complete item")
		}
		lintAssertItem(t, page.Data[0], row.Items[0])
		if index == 0 {
			var exact struct {
				Data []struct{ Pages json.Number } `json:"data"`
			}
			if err := json.Unmarshal(answer.Structured, &exact); err != nil {
				t.Fatal(err)
			}
			if exact.Data[0].Pages.String() != strconv.FormatInt(math.MaxInt64, 10) {
				t.Fatal("full-width wanted page count lost decimal identity")
			}
		}
		t.Logf("identity extreme %s SDK=%d canonical implicit cursor=%d", row.Rule, size, len(row.Items[0].Cursor))
	}
	answer, _ := lintCall(t, server, false, "lint", nil)
	var summary lintReadSummary
	if err := json.Unmarshal(answer.Structured, &summary); err != nil {
		t.Fatal(err)
	}
	if len(summary.Rules) != 8 || summary.Rules[0].Count != 10000 || !summary.Rules[0].Capped {
		t.Fatal("summary lost the true capped lower-bound fields")
	}
}
