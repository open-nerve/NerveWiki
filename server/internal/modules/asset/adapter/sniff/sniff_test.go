package sniff_test

import (
	"bytes"
	"encoding/base64"
	"image"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/sniff"
)

// The images are written out, not encoded here: importing an encoder
// registers its format's decoder, which would hide a decoder sniff does
// not register.
const (
	png7x5  = "iVBORw0KGgoAAAANSUhEUgAAAAcAAAAFCAAAAACs8akEAAAANUlEQVR4nAAoANf/AgAAAAAAAAACAAAAAAAAAAIAAAAAAAAAAgAAAAAAAAACAAAAAAAAAAMAARgAC/l17RkAAAAASUVORK5CYII="
	jpeg7x5 = "/9j/2wCEAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDIBCQkJDAsMGA0NGDIhHCEyMjIyMjIy" +
		"MjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMv/AAAsIAAUABwEBEQD/xADSAAABBQEBAQEBAQAAAAAAAAAAAQIDBAUGBwgJCgsQAAIBAwMCBAMF" +
		"BQQEAAABfQECAwAEEQUSITFBBhNRYQcicRQygZGhCCNCscEVUtHwJDNicoIJChYXGBkaJSYnKCkqNDU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6" +
		"g4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2drh4uPk5ebn6Onq8fLz9PX29/j5+v/aAAgBAQAAPwD5/r//2Q=="
	gif7x5 = "R0lGODlhBwAFAIcAAAAAAAAARAAAiAAAzABEAABERABEiABEzACIAACIRACIiACIzADMAADMRADMiADMzADd3REREQAAVQAAmQAA3QBVAABVVQBMmQBJ3QCZAACZ" +
		"TACZmQCT3QDdAADdSQDdkwDungDu7iIiIgAAZgAAqgAA7gBmAABmZgBVqgBP7gCqAACqVQCqqgCe7gDuAADuTwD/VQD/qgD//zMzMwAAdwAAuwAA/wB3AAB3dwBduwBV" +
		"/wC7AAC7XQC7uwCq/wD/AEQAREQAiEQAzEREAEREREREiEREzESIAESIRESIiESIzETMAETMRETMiETMzEQAAFUAAFUAVUwAmUkA3VVVAFVVVUxMmUlJ3UyZAEyZTEyZ" +
		"mUmT3UndAEndSUndk0nd3U/u7mYAAGYAZlUAqk8A7mZmAGZmZlVVqk9P7lWqAFWqVVWqqk+e7k/uAE/uT0/unlX/qlX//3cAAHcAd10Au1UA/3d3AHd3d11du1VV/127" +
		"AF27XV27u1Wq/1X/AFX/VYgAiIgAzIhEAIhERIhEiIhEzIiIAIiIRIiIiIiIzIjMAIjMRIjMiIjMzIgAAIgARJkATJkAmZMA3ZlMAJlMTJlMmZNJ3ZmZAJmZTJmZmZOT" +
		"3ZPdAJPdSZPdk5Pd3ZkAAKoAAKoAVaoAqp4A7qpVAKpVVapVqp5P7qqqAKqqVaqqqp6e7p7uAJ7uT57unp7u7qr//7sAALsAXbsAu6oA/7tdALtdXbtdu6pV/7u7ALu7" +
		"Xbu7u6qq/6r/AKr/Var/qswAzMxEAMxERMxEiMxEzMyIAMyIRMyIiMyIzMzMAMzMRMzMiMzMzMwAAMwARMwAiN0Ak90A3d1JAN1JSd1Jk91J3d2TAN2TSd2Tk92T3d3d" +
		"AN3dSd3dk93d3d0AAN0ASe4AT+4Anu4A7u5PAO5PT+5Pnu5P7u6eAO6eT+6enu6e7u7uAO7uT+7unu7u7u4AAP8AAP8AVf8Aqv8A//9VAP9VVf9Vqv9V//+qAP+qVf+q" +
		"qv+q////AP//Vf//qv///ywAAAAABwAFAAAIDAABCBxIsKDBgwcDAgA7"
)

// images is a 7×5 image in each of the formats whose size is read.
func images(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for mime, b64 := range map[string]string{"image/png": png7x5, "image/jpeg": jpeg7x5, "image/gif": gif7x5} {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatal(err)
		}
		out[mime] = data
	}
	return out
}

// Sniff is net/http's; Size reads PNG, JPEG and GIF headers, through the
// decoders sniff registers.
func TestSniffAndSize(t *testing.T) {
	var s sniff.Sniffer
	for mime, data := range images(t) {
		if got := s.Sniff(data[:min(len(data), 512)]); got != mime {
			t.Errorf("Sniff(%s) = %q", mime, got)
		}
		if w, h, ok := s.Size(bytes.NewReader(data)); !ok || w != 7 || h != 5 {
			t.Errorf("Size(%s) = %d, %d, %v; want 7×5", mime, w, h, ok)
		}
	}
	png := images(t)["image/png"]
	for name, data := range map[string][]byte{"no image": []byte("not an image"), "a cut PNG": png[:20], "nothing": nil} {
		if _, _, ok := s.Size(bytes.NewReader(data)); ok {
			t.Errorf("Size(%s) read a size, want none", name)
		}
	}
}

// Sniff is net/http's over the whole head it is given: signatures longer
// than a few bytes, and a tag after the blanks a browser skips.
func TestSniffReadsTheWholeHead(t *testing.T) {
	for head, want := range map[string]string{
		"<html><body>":                                     "text/html; charset=utf-8",
		"RIFF\x00\x00\x00\x00WEBPVP":                       "image/webp",
		"RIFF\x00\x00\x00\x00WAVE":                         "audio/wave",
		"<!DOCTYPE html><title>x</title>":                  "text/html; charset=utf-8",
		string(bytes.Repeat([]byte{' '}, 300)) + "<html>x": "text/html; charset=utf-8",
	} {
		if got := (sniff.Sniffer{}).Sniff([]byte(head)); got != want {
			t.Errorf("Sniff(%.20q…) = %q, want %q", head, got, want)
		}
	}
}

// comments is total bytes of JPEG comment segments, each at most 65,537
// bytes: the marker, the length, the text.
func comments(total int) []byte {
	var out []byte
	for total > 0 {
		seg := min(total, 0xFFFF+2)
		if rest := total - seg; rest > 0 && rest < 4 {
			seg -= 4
		}
		n := seg - 2 // the length counts itself
		out = append(out, 0xFF, 0xFE, byte(n>>8), byte(n))
		out = append(out, bytes.Repeat([]byte{'x'}, seg-4)...)
		total -= seg
	}
	return out
}

// Size reads the first MiB, not a byte more: a JPEG whose size is read by
// its MiB-th byte, after comments, has its size read; one a byte later,
// not.
func TestSizeReadsAMiB(t *testing.T) {
	data := images(t)["image/jpeg"]
	need := 0 // the bytes of data the decoder reads for the size
	for n := 2; n <= len(data); n++ {
		if _, _, err := image.DecodeConfig(bytes.NewReader(data[:n])); err == nil {
			need = n
			break
		}
	}
	if need == 0 {
		t.Fatal("no prefix of the JPEG tells its size")
	}
	padded := func(pad int) []byte {
		return append(append(append([]byte{}, data[:2]...), comments(pad)...), data[2:]...)
	}
	if w, h, ok := (sniff.Sniffer{}).Size(bytes.NewReader(padded(1<<20 - need))); !ok || w != 7 || h != 5 {
		t.Errorf("Size(a JPEG whose size ends at the MiB) = %d, %d, %v; want 7×5", w, h, ok)
	}
	if _, _, ok := (sniff.Sniffer{}).Size(bytes.NewReader(padded(1<<20 - need + 1))); ok {
		t.Error("Size(a JPEG whose size ends a byte past the MiB) read it, want none")
	}
}
