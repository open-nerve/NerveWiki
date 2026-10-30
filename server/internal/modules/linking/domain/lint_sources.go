package domain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
	"uuid"
)

// SourceJudgment keeps the full L target or stable J JSON. Target eligibility
// belongs to the caller, after every matching persisted edge has been counted.
type SourceJudgment struct {
	NotLink       bool
	FullText      string
	ResolvedID    *uuid.UUID
	ResolvedAsset bool
}

type sourceSlot struct {
	value  any
	string bool
	count  uint8
	unique SourceJudgment
}

// SourceSlots owns one first-key JSON value through all of its edge batches.
// It does not know area/current/target eligibility and cannot prefilter them.
type SourceSlots struct {
	key      string
	array    bool
	hasSlots bool
	finished bool
	slots    []sourceSlot
}

func NewSourceSlots(ctx context.Context, actualKey string, jsonValue []byte) (*SourceSlots, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(actualKey) != 7 || !strings.EqualFold(actualKey, "sources") {
		return nil, errLintValue
	}
	decoder := json.NewDecoder(&sourceJSONReader{ctx: ctx, data: jsonValue})
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		if canceled := ctx.Err(); canceled != nil {
			return nil, canceled
		}
		return nil, errLintValue
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if canceled := ctx.Err(); canceled != nil {
			return nil, canceled
		}
		return nil, errLintValue
	}
	s := &SourceSlots{key: actualKey}
	values := []any{value}
	if array, ok := value.([]any); ok {
		s.array, values = true, array
	}
	if len(values) > 10000 {
		return nil, errLintValue
	}
	s.slots = make([]sourceSlot, len(values))
	for i, value := range values {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		_, isString := value.(string)
		s.slots[i] = sourceSlot{value: value, string: isString}
		s.hasSlots = s.hasSlots || isString
	}
	return s, nil
}

// QueryShape matches the existing SourceSpec: Items is zero for a nonarray.
func (s *SourceSlots) QueryShape() (array bool, items int32, hasSlots bool) {
	if s.array {
		return true, int32(len(s.slots)), s.hasSlots
	}
	return false, 0, s.hasSlots
}

func (s *SourceSlots) Observe(propertyKey *string, target string, resolvedID *uuid.UUID, resolvedAsset bool) error {
	if s.finished {
		return errLintValue
	}
	if propertyKey == nil {
		return nil
	}
	index := 0
	if !s.array {
		if *propertyKey != s.key {
			return nil
		}
	} else {
		if len(*propertyKey) < 9 || len(*propertyKey) > 12 || !strings.HasPrefix(*propertyKey, s.key+".") {
			return nil
		}
		digits := (*propertyKey)[8:]
		if len(digits) > 1 && digits[0] == '0' {
			return nil
		}
		for _, digit := range digits {
			if digit < '0' || digit > '9' {
				return nil
			}
		}
		parsed, err := strconv.Atoi(digits)
		if err != nil || parsed >= len(s.slots) {
			return nil
		}
		index = parsed
	}
	if index >= len(s.slots) || !s.slots[index].string {
		return nil
	}
	if target == "" || resolvedAsset && resolvedID == nil {
		return errLintValue
	}
	slot := &s.slots[index]
	if slot.count == 0 {
		slot.count = 1
		slot.unique = SourceJudgment{FullText: target, ResolvedAsset: resolvedAsset}
		if resolvedID != nil {
			id := *resolvedID
			slot.unique.ResolvedID = &id
		}
	} else {
		slot.count = 2
		slot.unique = SourceJudgment{}
	}
	return nil
}

// Finish is called even for a page without matching edges. Only exactly one
// edge makes L; zero/multiple edges or a nonstring value make J. It is single use.
func (s *SourceSlots) Finish(ctx context.Context) ([]SourceJudgment, error) {
	if s.finished {
		return nil, errLintValue
	}
	s.finished = true
	defer func() { s.slots = nil }()
	out := make([]SourceJudgment, 0, len(s.slots))
	for _, slot := range s.slots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if slot.string && slot.count == 1 {
			out = append(out, slot.unique)
			continue
		}
		var text strings.Builder
		if err := sourceStableJSON(ctx, &text, slot.value); err != nil {
			return nil, err
		}
		out = append(out, SourceJudgment{NotLink: true, FullText: text.String()})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type sourceJSONReader struct {
	ctx  context.Context
	data []byte
}

func (r *sourceJSONReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p[:min(len(p), 4096)], r.data)
	r.data = r.data[n:]
	return n, nil
}

// Stable JSON has Go's deterministic map key/escaping semantics, while long
// strings and comparisons poll ctx. Numbers retain the decoder's UseNumber
// spelling instead of passing through an inexact float64.
func sourceStableJSON(ctx context.Context, out *strings.Builder, value any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch v := value.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		out.WriteString(strconv.FormatBool(v))
	case json.Number:
		for text := v.String(); len(text) > 0; {
			if err := ctx.Err(); err != nil {
				return err
			}
			n := min(len(text), 4096)
			out.WriteString(text[:n])
			text = text[n:]
		}
	case string:
		out.WriteByte('"')
		for len(v) > 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			n := min(len(v), 4096)
			for n < len(v) && !utf8.RuneStart(v[n]) {
				n--
			}
			encoded, err := json.Marshal(v[:n])
			if err != nil {
				return errLintValue
			}
			out.Write(encoded[1 : len(encoded)-1])
			v = v[n:]
		}
		out.WriteByte('"')
	case []any:
		out.WriteByte('[')
		for i, item := range v {
			if i != 0 {
				out.WriteByte(',')
			}
			if err := sourceStableJSON(ctx, out, item); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			if err := ctx.Err(); err != nil {
				return err
			}
			keys = append(keys, key)
		}
		var canceled error
		slices.SortFunc(keys, func(a, b string) int {
			for len(a) > 0 && len(b) > 0 {
				if canceled != nil {
					return 0
				}
				if canceled = ctx.Err(); canceled != nil {
					return 0
				}
				n := min(len(a), len(b), 4096)
				if c := bytes.Compare([]byte(a[:n]), []byte(b[:n])); c != 0 {
					return c
				}
				a, b = a[n:], b[n:]
			}
			return len(a) - len(b)
		})
		if canceled != nil {
			return canceled
		}
		out.WriteByte('{')
		for i, key := range keys {
			if i != 0 {
				out.WriteByte(',')
			}
			if err := sourceStableJSON(ctx, out, key); err != nil {
				return err
			}
			out.WriteByte(':')
			if err := sourceStableJSON(ctx, out, v[key]); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	default:
		return errLintValue
	}
	return nil
}
