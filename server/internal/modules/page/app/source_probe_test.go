package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

type sourceProbeFixture struct {
	rows        map[SourceKey][]uuid.UUID
	heads       map[uuid.UUID]SourceValue
	calls       [][]SourceProbe
	returned    int
	snapshot    bool
	headBatches []int
}

func probeID(n int) uuid.UUID {
	var id uuid.UUID
	binary.BigEndian.PutUint64(id[8:], uint64(n))
	return id
}
func probeKey(kind domain.Kind, body string) SourceKey {
	return SourceKey{Kind: kind, Hash: sha256.Sum256([]byte(body))}
}
func (f *sourceProbeFixture) SourceHeads(ctx context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]SourceValue, error) {
	if f.snapshot && ctx.Value(sourceSnapshotKey{}) != true {
		return nil, fmt.Errorf("heads outside snapshot")
	}
	f.headBatches = append(f.headBatches, len(ids))
	if len(ids) > domain.MaxUnitNodes {
		return nil, fmt.Errorf("unbounded heads")
	}
	out := map[uuid.UUID]SourceValue{}
	for _, id := range ids {
		if v, ok := f.heads[id]; ok {
			out[id] = v
		}
	}
	return out, nil
}
func (f *sourceProbeFixture) ProbeSources(ctx context.Context, _ uuid.UUID, ps []SourceProbe) ([]SourceMatch, error) {
	if f.snapshot && ctx.Value(sourceSnapshotKey{}) != true {
		return nil, fmt.Errorf("probe outside snapshot")
	}
	f.calls = append(f.calls, slices.Clone(ps))
	total := 0
	var out []SourceMatch
	for i, p := range ps {
		total += p.Take
		if p.Take <= 0 || total > domain.MaxUnitNodes {
			return nil, fmt.Errorf("unbounded take %d", total)
		}
		count := 0
		for _, id := range f.rows[p.Key] {
			if p.After != nil && bytes.Compare(id[:], p.After[:]) <= 0 {
				continue
			}
			out = append(out, SourceMatch{Probe: i, ID: id})
			count++
			if count == p.Take {
				break
			}
		}
	}
	f.returned += len(out)
	return out, nil
}
func TestSourceProbesBoundExcludedRowsAndAdvanceCursors(t *testing.T) {
	for _, n := range []int{0, 1, 3, 9999, 10000, 10001, 60000} {
		for _, external := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/%v", n, external), func(t *testing.T) {
				key := probeKey(domain.KindPage, "same")
				f := &sourceProbeFixture{rows: map[SourceKey][]uuid.UUID{}}
				initial := map[uuid.UUID]SourceValue{}
				for i := range n {
					id := probeID(i)
					f.rows[key] = append(f.rows[key], id)
					initial[id] = SourceValue{ID: id, Kind: key.Kind, BeforeSourceHash: key.Hash[:]}
				}
				want := probeID(n)
				if external {
					f.rows[key] = append(f.rows[key], want)
					for i := range 30 {
						f.rows[key] = append(f.rows[key], probeID(n+i+1))
					}
				}
				got, err := probeSourceWitnesses(t.Context(), f, probeID(999999), []SourceKey{key}, initial)
				if err != nil {
					t.Fatal(err)
				}
				id, found := got[key]
				if found != external || external && id != want {
					t.Errorf("external witness = %s/%v, want %s/%v", id, found, want, external)
				}
				expected := n
				if external {
					expected++
				}
				if f.returned != expected {
					t.Errorf("actual returned rows=%d want %d", f.returned, expected)
				}
				maxCalls := 2 + (n+domain.MaxUnitNodes-1)/domain.MaxUnitNodes
				if len(f.calls) == 0 || len(f.calls) > maxCalls {
					t.Errorf("calls=%d, must batch excluded rows within %d", len(f.calls), maxCalls)
				}
				if len(f.calls) > 0 && (len(f.calls[0]) != 1 || f.calls[0][0].Take != 1 || f.calls[0][0].After != nil) {
					t.Errorf("first probe=%+v", f.calls[0])
				}
			})
		}
	}
}
func TestSourceProbesBatchKeysAndKeepKindsIndependent(t *testing.T) {
	f := &sourceProbeFixture{rows: map[SourceKey][]uuid.UUID{}}
	var keys []SourceKey
	for i := range 10002 {
		key := probeKey(domain.KindPage, fmt.Sprint(i))
		keys = append(keys, key)
		f.rows[key] = []uuid.UUID{probeID(i)}
	}
	asset := keys[0]
	asset.Kind = domain.KindAsset
	keys = append(keys, asset)
	f.rows[asset] = []uuid.UUID{probeID(10009)}
	got, err := probeSourceWitnesses(t.Context(), f, probeID(999999), keys, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(keys) || len(f.calls) != 2 || f.returned != len(keys) {
		t.Errorf("witnesses/calls/rows=%d/%d/%d want %d/2/%d", len(got), len(f.calls), f.returned, len(keys), len(keys))
	}
	if got[asset] != probeID(10009) {
		t.Errorf("asset confused with page: %v", got[asset])
	}
}
