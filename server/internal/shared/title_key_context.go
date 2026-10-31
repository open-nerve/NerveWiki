package shared

import (
	"context"
	"io"

	"golang.org/x/text/cases"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// TitleKeyContext applies TitleKey's NFC, full Unicode fold and NFC pipeline
// while observing cancellation during long inputs. One stateful transform
// chain spans all chunks, including combining sequences and UTF-8 boundaries.
func TitleKeyContext(ctx context.Context, s string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// Small titles already fit one cancellation interval; keep the original
	// implementation and avoid a streaming buffer for each short segment.
	if len(s) <= 4096 {
		key := TitleKey(s)
		if err := ctx.Err(); err != nil {
			return "", err
		}
		return key, nil
	}
	reader := &titleKeyReader{ctx: ctx, text: s}
	transformed := transform.NewReader(reader, transform.Chain(norm.NFC, cases.Fold(), norm.NFC))
	out, err := io.ReadAll(transformed)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return string(out), nil
}

type titleKeyReader struct {
	ctx  context.Context
	text string
}

func (r *titleKeyReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if r.text == "" {
		return 0, io.EOF
	}
	n := copy(p[:min(len(p), 4096)], r.text)
	r.text = r.text[n:]
	return n, nil
}
