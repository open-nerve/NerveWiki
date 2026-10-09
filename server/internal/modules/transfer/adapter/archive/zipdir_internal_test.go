package archiveadapter

import (
	"bytes"
	"errors"
	"testing"
)

// bounded lets left bytes be read, a read past them failing whole, until
// lifted.
func TestBoundedReadsNoMoreThanCounted(t *testing.T) {
	b := &bounded{r: bytes.NewReader(make([]byte, 100)), left: 10}
	if n, err := b.ReadAt(make([]byte, 6), 0); n != 6 || err != nil {
		t.Fatalf("ReadAt(6) = %d, %v", n, err)
	}
	if _, err := b.ReadAt(make([]byte, 5), 6); !errors.Is(err, errBounded) {
		t.Errorf("ReadAt(5) past the bound = %v, want errBounded", err)
	}
	if n, err := b.ReadAt(make([]byte, 4), 6); n != 4 || err != nil {
		t.Errorf("ReadAt(4) up to the bound = %d, %v", n, err)
	}
	b.lift()
	if n, err := b.ReadAt(make([]byte, 50), 10); n != 50 || err != nil {
		t.Errorf("ReadAt(50) lifted = %d, %v", n, err)
	}
}
