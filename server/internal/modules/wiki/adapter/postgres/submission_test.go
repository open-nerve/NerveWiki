package postgresadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

// A submission's rows (M11/P2 design 3.12): Submit sets an upgrade
// upgrading reviewing, its submission who and when, which Current reads;
// one reviewing without a report is taken over only once submitted at
// takenOver or before; KeepReport and Unsubmit change only the row of
// their submission, reviewing, without a report; Unsubmit clears the
// submission's three columns. None changes an upgrade rolling back.
func TestTheSubmissionsRows(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	at := time.Date(2026, 11, 10, 9, 0, 0, 123000, time.UTC)
	eng := started(f.eng, f.alice, at)
	if err := f.store.Insert(ctx, eng, f.alice, at); err != nil {
		t.Fatal(err)
	}
	must := func(t *testing.T, what string, ok bool, err error, want bool) {
		t.Helper()
		if err != nil || ok != want {
			t.Fatalf("%s = %v, %v; want %v", what, ok, err, want)
		}
	}
	current := func(t *testing.T) domain.Upgrade {
		t.Helper()
		u, ok, err := f.store.Current(ctx, f.eng)
		if err != nil || !ok {
			t.Fatalf("Current = %v, %v", ok, err)
		}
		return u
	}
	first, second := uuid.NewV7(), uuid.NewV7()
	ok, err := f.store.Submit(ctx, eng.ID, first, f.bob, at, at.Add(-time.Minute))
	must(t, "Submit(upgrading)", ok, err, true)
	if u := current(t); u.State != domain.UpgradeReviewing || *u.SubmissionID != first || *u.SubmittedBy != f.bob ||
		!u.SubmittedAt.Equal(at) || u.Report != nil {
		t.Fatalf("submitted %+v", u)
	}
	ok, err = f.store.Submit(ctx, eng.ID, second, f.alice, at.Add(time.Minute), at.Add(-time.Microsecond))
	must(t, "Submit(reviewing, submitted a microsecond after takenOver)", ok, err, false)
	ok, err = f.store.KeepReport(ctx, eng.ID, second, domain.UpgradeReport{}, f.alice, at)
	must(t, "KeepReport(another submission)", ok, err, false)
	ok, err = f.store.Unsubmit(ctx, eng.ID, second, f.alice, at)
	must(t, "Unsubmit(another submission)", ok, err, false)
	ok, err = f.store.Submit(ctx, eng.ID, second, f.alice, at.Add(time.Minute), at)
	must(t, "Submit(reviewing, submitted at takenOver)", ok, err, true)
	if u := current(t); *u.SubmissionID != second || *u.SubmittedBy != f.alice || !u.SubmittedAt.Equal(at.Add(time.Minute)) {
		t.Fatalf("taken over %+v", u)
	}
	ok, err = f.store.KeepReport(ctx, eng.ID, first, domain.UpgradeReport{}, f.bob, at)
	must(t, "KeepReport(the submission taken over)", ok, err, false)
	report := domain.UpgradeReport{ComputedAt: at, Passed: true, Rules: []domain.ReportRule{}}
	ok, err = f.store.KeepReport(ctx, eng.ID, second, report, f.alice, at)
	must(t, "KeepReport(its submission)", ok, err, true)
	if u := current(t); u.Report == nil || !u.Report.Passed || u.State != domain.UpgradeReviewing {
		t.Fatalf("kept %+v", u)
	}
	ok, err = f.store.KeepReport(ctx, eng.ID, second, report, f.alice, at)
	must(t, "KeepReport(a report kept)", ok, err, false)
	ok, err = f.store.Unsubmit(ctx, eng.ID, second, f.alice, at)
	must(t, "Unsubmit(a report kept)", ok, err, false)
	ok, err = f.store.Submit(ctx, eng.ID, uuid.NewV7(), f.alice, at.Add(time.Hour), at.Add(time.Hour))
	must(t, "Submit(a report kept)", ok, err, false)

	ops := started(f.ops, f.bob, at)
	if err := f.store.Insert(ctx, ops, f.bob, at); err != nil {
		t.Fatal(err)
	}
	ok, err = f.store.Unsubmit(ctx, ops.ID, first, f.bob, at)
	must(t, "Unsubmit(upgrading)", ok, err, false)
	ok, err = f.store.Submit(ctx, ops.ID, first, f.bob, at, at)
	must(t, "Submit(ops)", ok, err, true)
	ok, err = f.store.Unsubmit(ctx, ops.ID, first, f.alice, at.Add(time.Second))
	must(t, "Unsubmit(its submission)", ok, err, true)
	if u, _, err := f.store.Current(ctx, f.ops); err != nil || u.State != domain.UpgradeUpgrading || u.SubmissionID != nil ||
		u.SubmittedBy != nil || u.SubmittedAt != nil || u.Report != nil {
		t.Fatalf("unsubmitted %+v, %v", u, err)
	}
	var by uuid.UUID
	var updated time.Time
	if err := f.pool.QueryRow(ctx, "SELECT updated_by_id, updated_at FROM notebook_upgrades WHERE id = $1", ops.ID).Scan(&by,
		&updated); err != nil || by != f.alice || !updated.Equal(at.Add(time.Second)) {
		t.Fatalf("unsubmitted by %s at %s, %v", by, updated, err)
	}
	ok, err = f.store.Unsubmit(ctx, ops.ID, first, f.alice, at)
	must(t, "Unsubmit(again)", ok, err, false)
	if _, err := f.pool.Exec(ctx, "UPDATE notebook_upgrades SET state = 'rolling_back', ending = 'abandoned', rollback_job_id = $2 "+
		"WHERE id = $1", ops.ID, uuid.NewV7()); err != nil {
		t.Fatal(err)
	}
	ok, err = f.store.Submit(ctx, ops.ID, second, f.bob, at, at.Add(time.Hour))
	must(t, "Submit(rolling back)", ok, err, false)
}

// fullReport is a report of every rule, each with five items of its
// shape, at the bounds the contract sets when worst (M11/P2 design 3.10):
// targets of 256 control characters, each written \u00XX; item keys of
// 140 bytes; every gate naming its most nodes; counts at their caps. Not
// worst, the wanted pages now are 7, not capped, so that the two wanted
// counts are not the same (review P2S2C-M4).
func fullReport(worst bool) domain.UpgradeReport {
	at := time.Date(2026, 11, 10, 9, 0, 0, 123456000, time.UTC)
	target, key := "Plan", func(prefix string) string { return prefix + uuid.NewV7().String() }
	if worst {
		target = strings.Repeat("\x01", 256)
		key = func(prefix string) string { return prefix + strings.Repeat("f", 140-len(prefix)) }
	}
	ids := func(n int) []uuid.UUID {
		out := make([]uuid.UUID, n)
		for i := range out {
			out[i] = uuid.NewV7()
		}
		return out
	}
	now := domain.LintCount{Value: 10000, Capped: true}
	if !worst {
		now = domain.LintCount{Value: 7}
	}
	r := domain.UpgradeReport{
		ComputedAt: at, StalePages: 0, Passed: true,
		Gates: domain.UpgradeGates{Agents: true, SourceRoot: true, Frontmatter: true, Sources: true, Index: true,
			ReservedTitles: domain.NodesGate{NodeIDs: ids(4)}, Links: domain.LinksGate{PagesWorse: 12, NodeIDs: ids(5)},
			Preserved: domain.PreservedGate{Nodes: 100000, NodesKept: 99990, Pages: 90000, PagesKept: 89990, NodeIDs: ids(5)}},
		Counts: domain.UpgradeCounts{Sources: 100000, MovedIntoSources: 100000, WikiPages: 100000, NewPages: 100000, Changesets: 100000,
			WantedBefore: domain.LintCount{Value: 10000, Capped: true}, WantedNow: now},
	}
	for _, rule := range domain.LintRules(domain.ModeLLMWiki) {
		rr := domain.ReportRule{Rule: rule, Count: domain.LintCount{Value: 10000, Capped: true}}
		for range 5 {
			page, node := uuid.NewV7(), uuid.NewV7()
			item := domain.LintItem{Rule: rule}
			switch rule {
			case domain.RuleWanted:
				item.ItemKey, item.Target, item.TargetCut, item.Pages = key("w:"), target, worst, 10000
			case domain.RuleAmbiguous:
				item.ItemKey, item.PageID, item.Target, item.TargetCut = key("a:"), &page, target, worst
			case domain.RuleInvalidSources:
				item.ItemKey, item.PageID, item.Target, item.TargetCut, item.NotLink = key("s:"), &page, target, worst, true
			case domain.RuleUndigestedSources:
				item.ItemKey, item.NodeID, item.Kind, item.Bytes, item.CreatedAt = key("n:"), &node, domain.NodeAsset, 1<<40, at
			default:
				item.ItemKey, item.PageID = key("p:"), &page
			}
			rr.Items = append(rr.Items, item)
		}
		r.Rules = append(r.Rules, rr)
	}
	return r
}

// A report kept reads back as it was kept, one encoding both ways (M11/P2
// design 3.12), its members the contract's UpgradeReport's: no cursor,
// an item's members its rule's.
func TestTheReportRoundTrips(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	at := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	eng := started(f.eng, f.alice, at)
	if err := f.store.Insert(ctx, eng, f.alice, at); err != nil {
		t.Fatal(err)
	}
	sub := uuid.NewV7()
	if ok, err := f.store.Submit(ctx, eng.ID, sub, f.alice, at, at); !ok || err != nil {
		t.Fatalf("Submit = %v, %v", ok, err)
	}
	want := fullReport(false)
	want.Passed, want.Gates.Links.Passed = false, false
	want.Gates.Preserved.NodeIDs = nil // none read back as none
	if ok, err := f.store.KeepReport(ctx, eng.ID, sub, want, f.alice, at); !ok || err != nil {
		t.Fatalf("KeepReport = %v, %v", ok, err)
	}
	got, _, err := f.store.Current(ctx, f.eng)
	if err != nil || got.Report == nil || !reflect.DeepEqual(*got.Report, want) {
		t.Fatalf("read back %+v, %v; want %+v", got.Report, err, want)
	}
	var raw []byte
	if err := f.pool.QueryRow(ctx, "SELECT report FROM notebook_upgrades WHERE id = $1", eng.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var stored struct {
		Members map[string]json.RawMessage
		Gates   map[string]map[string]json.RawMessage `json:"gates"`
		Rules   []map[string]json.RawMessage          `json:"rules"`
		Counts  map[string]json.RawMessage            `json:"counts"`
	}
	if err := json.Unmarshal(raw, &stored.Members); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	keys := func(m any) string {
		v := reflect.ValueOf(m)
		var out []string
		for _, k := range v.MapKeys() {
			out = append(out, k.String())
		}
		slices.Sort(out)
		return fmt.Sprint(out)
	}
	items := map[string]string{}
	for _, rule := range stored.Rules {
		var list []map[string]json.RawMessage
		if err := json.Unmarshal(rule["items"], &list); err != nil {
			t.Fatal(err)
		}
		var name string
		if err := json.Unmarshal(rule["rule"], &name); err != nil {
			t.Fatal(err)
		}
		items[name] = keys(list[0])
		if got := keys(rule); got != "[count count_capped items rule]" {
			t.Errorf("the rule %s's members %s", name, got)
		}
	}
	for name, want := range map[string]string{
		"wanted":              "[item_key pages rule target]",
		"orphans":             "[item_key page_id rule]",
		"ambiguous":           "[item_key page_id rule target]",
		"invalid_frontmatter": "[item_key page_id rule]",
		"missing_type":        "[item_key page_id rule]",
		"missing_summary":     "[item_key page_id rule]",
		"undigested_sources":  "[bytes created_at item_key kind node_id rule]",
		"invalid_sources":     "[item_key not_link page_id rule target]",
	} {
		if items[name] != want {
			t.Errorf("an item of %s has %s, want %s (false and zero members left out)", name, items[name], want)
		}
	}
	for what, got := range map[string]string{
		"[computed_at counts gates passed rules stale_pages]":                                   keys(stored.Members),
		"[agents frontmatter index links preserved reserved_titles source_root sources]":        keys(stored.Gates),
		"[node_ids nodes nodes_kept pages pages_kept passed]":                                   keys(stored.Gates["preserved"]),
		"[node_ids pages_worse passed]":                                                         keys(stored.Gates["links"]),
		"[changesets moved_into_sources new_pages sources wanted_before wanted_now wiki_pages]": keys(stored.Counts),
		"[node_ids passed]":                      keys(stored.Gates["reserved_titles"]),
		"[passed]":                               keys(stored.Gates["agents"]),
		`[]`:                                     string(stored.Gates["preserved"]["node_ids"]),
		`{"count": 10000, "count_capped": true}`: string(stored.Counts["wanted_before"]),
		`{"count": 7, "count_capped": false}`:    string(stored.Counts["wanted_now"]),
	} {
		if got != what {
			t.Errorf("stored %s, want %s", got, what)
		}
	}

	// The wanted counts the other way round, kept again by the same
	// submission once its report is cleared: the pages wanted as the
	// upgrade started 5, not capped, now capped (review P2S2C-M4).
	other := fullReport(false)
	other.Counts.WantedBefore, other.Counts.WantedNow = domain.LintCount{Value: 5}, domain.LintCount{Value: 10000, Capped: true}
	if _, err := f.pool.Exec(ctx, "UPDATE notebook_upgrades SET report = NULL WHERE id = $1", eng.ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.store.KeepReport(ctx, eng.ID, sub, other, f.alice, at); !ok || err != nil {
		t.Fatalf("KeepReport(the other way round) = %v, %v", ok, err)
	}
	got, _, err = f.store.Current(ctx, f.eng)
	if err != nil || got.Report == nil || !reflect.DeepEqual(got.Report.Counts, other.Counts) {
		t.Fatalf("read back the counts %+v, %v; want %+v", got.Report, err, other.Counts)
	}
}

// The worst report fits notebook_upgrades_report_check's 65,536 bytes of
// text (decision 263): kept through the table, read back as kept. Its
// size, as jsonb writes it, goes to the record (-v).
func TestTheWorstReportFits(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	at := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	eng := started(f.eng, f.alice, at)
	if err := f.store.Insert(ctx, eng, f.alice, at); err != nil {
		t.Fatal(err)
	}
	sub := uuid.NewV7()
	if ok, err := f.store.Submit(ctx, eng.ID, sub, f.alice, at, at); !ok || err != nil {
		t.Fatalf("Submit = %v, %v", ok, err)
	}
	worst := fullReport(true)
	if ok, err := f.store.KeepReport(ctx, eng.ID, sub, worst, f.alice, at); !ok || err != nil {
		t.Fatalf("KeepReport(the worst) = %v, %v", ok, err)
	}
	got, _, err := f.store.Current(ctx, f.eng)
	if err != nil || got.Report == nil || !reflect.DeepEqual(*got.Report, worst) {
		t.Fatalf("read back %+v, %v", got.Report, err)
	}
	var text int
	if err := f.pool.QueryRow(ctx, "SELECT octet_length(report::text) FROM notebook_upgrades WHERE id = $1", eng.ID).Scan(&text); err != nil {
		t.Fatal(err)
	}
	if text > 65536 {
		t.Fatalf("the worst report is %d bytes of text, past 65,536", text)
	}
	t.Logf("the worst report: %d bytes of jsonb text, of 65,536", text)
}
