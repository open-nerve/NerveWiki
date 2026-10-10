package archiveadapter

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
)

// An import's archive's directory, read before archive/zip reads it (M7/P6
// design 3.10). archive/zip reads the directory's records from its start
// until one's signature is not a record's, whatever the end says it holds,
// one *zip.File each: a 512 MiB archive can hold some eleven million. So
// the end is found as archive/zip finds it (Go 1.27's readDirectoryEnd,
// whose rules this follows), and the records are counted as it reads
// them, nothing kept, before it does.

// The zip format's signatures and lengths (APPNOTE 4.3).
const (
	directoryEndSignature    = 0x06054b50
	directory64LocSignature  = 0x07064b50
	directory64EndSignature  = 0x06064b50
	directoryHeaderSignature = 0x02014b50
	directoryEndLen          = 22
	directory64LocLen        = 20
	directory64EndLen        = 56
	directoryHeaderLen       = 46
)

// MaxDirectory is the largest directory an import reads (M7/P6 design
// 3.10): 50,000 entries of a long path and its extra fields each, with
// room to spare.
const MaxDirectory = 64 << 20

// errNotZip is an archive without an end, or whose end points outside it.
var errNotZip = errors.New("zip: no directory's end, or one pointing outside the archive")

// directory is an archive's directory as its end tells it: how many
// records it holds and how many bytes, and where it starts in the file.
type directory struct {
	records uint64
	size    uint64
	start   int64
}

// readEnd finds the directory's end in the last 1 KiB of r, then its last
// 65 KiB, the zip64 end when the end says so, and where the directory
// starts, as archive/zip does: errNotZip for none.
func readEnd(r io.ReaderAt, size int64) (directory, error) {
	var buf []byte
	var at int64
	for i, n := range []int64{1024, 65 * 1024} {
		n = min(n, size)
		buf = make([]byte, n)
		if _, err := r.ReadAt(buf, size-n); err != nil && !errors.Is(err, io.EOF) {
			return directory{}, err
		}
		if p := endIn(buf); p >= 0 {
			buf, at = buf[p:], size-n+int64(p)
			break
		}
		if i == 1 || n == size {
			return directory{}, errNotZip
		}
	}
	d := directory{
		records: uint64(binary.LittleEndian.Uint16(buf[10:])),
		size:    uint64(binary.LittleEndian.Uint32(buf[12:])),
	}
	offset := uint64(binary.LittleEndian.Uint32(buf[16:]))
	if d.records == 0xffff || d.size == 0xffffffff || offset == 0xffffffff {
		p, ok, err := find64End(r, at)
		if err != nil {
			return directory{}, err
		}
		if ok {
			at = p
			if d, offset, err = read64End(r, size, p); err != nil {
				return directory{}, err
			}
		}
	}
	const maxInt64 = 1<<63 - 1
	if d.size > maxInt64 || offset > maxInt64 {
		return directory{}, errNotZip
	}
	base := at - int64(d.size) - int64(offset)
	if start := base + int64(offset); start < 0 || start >= size {
		return directory{}, errNotZip
	}
	// An end that names a base other than 0 though a record starts at its
	// offset from 0: archive/zip takes 0.
	if base > 0 && recordAt(r, size, int64(offset)) {
		base = 0
	}
	d.start = base + int64(offset)
	return d, nil
}

// endIn is where the directory's end starts in b, the last one whose
// comment fits b; -1 for none.
func endIn(b []byte) int {
	for i := len(b) - directoryEndLen; i >= 0; i-- {
		if binary.LittleEndian.Uint32(b[i:]) == directoryEndSignature {
			n := int(binary.LittleEndian.Uint16(b[i+directoryEndLen-2:]))
			if n+directoryEndLen+i > len(b) {
				return -1
			}
			return i
		}
	}
	return -1
}

// find64End reads the zip64 locator right before the end at: where the
// zip64 end is; false for no valid locator.
func find64End(r io.ReaderAt, at int64) (int64, bool, error) {
	loc := at - directory64LocLen
	if loc < 0 {
		return 0, false, nil
	}
	buf := make([]byte, directory64LocLen)
	if _, err := r.ReadAt(buf, loc); err != nil {
		return 0, false, err
	}
	if binary.LittleEndian.Uint32(buf) != directory64LocSignature || binary.LittleEndian.Uint32(buf[4:]) != 0 ||
		binary.LittleEndian.Uint32(buf[16:]) != 1 {
		return 0, false, nil
	}
	p := int64(binary.LittleEndian.Uint64(buf[8:])) //nolint:gosec // archive/zip takes it so
	// archive/zip ignores a locator that points before the file's start.
	return p, p >= 0, nil
}

// read64End reads the zip64 end at p in r of size bytes: the directory's
// records and size, and its offset; errNotZip for an end outside it.
func read64End(r io.ReaderAt, size, p int64) (directory, uint64, error) {
	if p > size-directory64EndLen {
		return directory{}, 0, errNotZip
	}
	buf := make([]byte, directory64EndLen)
	if _, err := r.ReadAt(buf, p); err != nil {
		return directory{}, 0, err
	}
	if binary.LittleEndian.Uint32(buf) != directory64EndSignature {
		return directory{}, 0, errNotZip
	}
	return directory{records: binary.LittleEndian.Uint64(buf[32:]), size: binary.LittleEndian.Uint64(buf[40:])},
		binary.LittleEndian.Uint64(buf[48:]), nil
}

// recordAt reports whether a directory's record starts at offset.
func recordAt(r io.ReaderAt, size, offset int64) bool {
	if offset < 0 || offset >= size {
		return false
	}
	_, ok := nextRecord(bufio.NewReader(io.NewSectionReader(r, offset, size-offset)))
	return ok
}

// countRecords counts the directory's records from start as archive/zip
// reads them, until one's signature is not a record's or the file ends;
// it stops once more than most are counted, or more than limit bytes
// read. It answers the records and their bytes.
func countRecords(r io.ReaderAt, size, start int64, most int, limit int64) (int, int64) {
	in := bufio.NewReader(io.NewSectionReader(r, start, size-start))
	n, read := 0, int64(0)
	for n <= most && read <= limit {
		length, ok := nextRecord(in)
		if !ok {
			break
		}
		n++
		read += length
	}
	return n, read
}

// nextRecord reads a directory's record from in: its header, name, extra
// field and comment; false when its signature is not a record's or the
// file ends within it, as archive/zip stops there.
func nextRecord(in *bufio.Reader) (int64, bool) {
	var head [directoryHeaderLen]byte
	if _, err := io.ReadFull(in, head[:]); err != nil || binary.LittleEndian.Uint32(head[:]) != directoryHeaderSignature {
		return 0, false
	}
	rest := int64(binary.LittleEndian.Uint16(head[28:])) + int64(binary.LittleEndian.Uint16(head[30:])) +
		int64(binary.LittleEndian.Uint16(head[32:]))
	if n, err := in.Discard(int(rest)); err != nil || int64(n) != rest {
		return 0, false
	}
	return directoryHeaderLen + rest, true
}
