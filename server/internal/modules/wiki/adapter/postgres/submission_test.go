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
// worst, each number is its own value, so that none is read or kept as
// another (reviews P2S2C-M4, M5, M6): the five counts 11 to 15, the
// wanted pages as the upgrade started capped, now 7; the stale pages 3,
// the links' pages worse 22; each rule's count its own, not its items'
// five, capped or not apart from its value; each boolean of an item's
// rule both ways, an item's two booleans apart; the sources' created an
// hour before the check. Its gates are the tests' to set (gateRows).
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
	counts := domain.UpgradeCounts{Sources: 11, MovedIntoSources: 12, WikiPages: 13, NewPages: 14, Changesets: 15,
		WantedBefore: domain.LintCount{Value: 10000, Capped: true}, WantedNow: domain.LintCount{Value: 7}}
	if worst {
		capped := domain.LintCount{Value: 10000, Capped: true}
		counts = domain.UpgradeCounts{Sources: 100000, MovedIntoSources: 100000, WikiPages: 100000, NewPages: 100000,
			Changesets: 100000, WantedBefore: capped, WantedNow: capped}
	}
	stale, worse, created := int64(3), 22, at.Add(-time.Hour)
	if worst {
		stale, worse, created = 0, 12, at
	}
	r := domain.UpgradeReport{
		ComputedAt: at, StalePages: stale, Passed: true,
		Gates: domain.UpgradeGates{Agents: true, SourceRoot: true, Frontmatter: true, Sources: true, Index: true,
			ReservedTitles: domain.NodesGate{NodeIDs: ids(4)}, Links: domain.LinksGate{PagesWorse: worse, NodeIDs: ids(5)},
			Preserved: domain.PreservedGate{Nodes: 100000, NodesKept: 99990, Pages: 90000, PagesKept: 89990, NodeIDs: ids(5)}},
		Counts: counts,
	}
	for i, rule := range domain.LintRules(domain.ModeLLMWiki) {
		rr := domain.ReportRule{Rule: rule, Count: domain.LintCount{Value: 10000, Capped: true}}
		if !worst {
			rr.Count = domain.LintCount{Value: int64(31 + i), Capped: i%3 == 2}
		}
		for j := range 5 {
			page, node := uuid.NewV7(), uuid.NewV7()
			item := domain.LintItem{Rule: rule}
			cut, notLink, pages := worst || j%2 == 0, worst || j%2 == 1, int64(10000)
			if !worst {
				pages = int64(41 + j)
			}
			switch rule {
			case domain.RuleWanted:
				item.ItemKey, item.Target, item.TargetCut, item.Pages = key("w:"), target, cut, pages
			case domain.RuleAmbiguous:
				item.ItemKey, item.PageID, item.Target, item.TargetCut = key("a:"), &page, target, cut
			case domain.RuleInvalidSources:
				item.ItemKey, item.PageID, item.Target, item.TargetCut, item.NotLink = key("s:"), &page, target, cut, notLink
			case domain.RuleUndigestedSources:
				item.ItemKey, item.NodeID, item.Kind, item.Bytes, item.CreatedAt = key("n:"), &node, domain.NodeAsset, 1<<40, created
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
	withGates(&want, gateRows()[0])
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
		items[name] = keys(list[0]) + keys(list[1])
		if got := keys(rule); got != "[count count_capped items rule]" {
			t.Errorf("the rule %s's members %s", name, got)
		}
	}
	// The first two items of each rule: the first's target cut, the
	// second's not; the second's not a link, the first's not.
	for name, want := range map[string]string{
		"wanted":              "[item_key pages rule target target_cut][item_key pages rule target]",
		"orphans":             "[item_key page_id rule][item_key page_id rule]",
		"ambiguous":           "[item_key page_id rule target target_cut][item_key page_id rule target]",
		"invalid_frontmatter": "[item_key page_id rule][item_key page_id rule]",
		"missing_type":        "[item_key page_id rule][item_key page_id rule]",
		"missing_summary":     "[item_key page_id rule][item_key page_id rule]",
		"undigested_sources":  "[bytes created_at item_key kind node_id rule][bytes created_at item_key kind node_id rule]",
		"invalid_sources":     "[item_key page_id rule target target_cut][item_key not_link page_id rule target]",
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
		"11":                                     string(stored.Counts["sources"]),
		"12":                                     string(stored.Counts["moved_into_sources"]),
		"13":                                     string(stored.Counts["wiki_pages"]),
		"14":                                     string(stored.Counts["new_pages"]),
		"15":                                     string(stored.Counts["changesets"]),
	} {
		if got != what {
			t.Errorf("stored %s, want %s", got, what)
		}
	}

	// The other gateRows, each kept again by the same submission once its
	// report is cleared, read back whole. Only a check that passed keeps a
	// report, so a report's gates all pass in use; kept here as any, so
	// that each gate is its own (review P2S2C-M6). The first of them has
	// the wanted counts the other way round: the pages wanted as the
	// upgrade started 5, not capped, now capped (review P2S2C-M4).
	for i, row := range gateRows()[1:] {
		other := fullReport(false)
		withGates(&other, row)
		if i == 0 {
			other.Counts.WantedBefore, other.Counts.WantedNow = domain.LintCount{Value: 5}, domain.LintCount{Value: 10000, Capped: true}
		}
		if _, err := f.pool.Exec(ctx, "UPDATE notebook_upgrades SET report = NULL WHERE id = $1", eng.ID); err != nil {
			t.Fatal(err)
		}
		if ok, err := f.store.KeepReport(ctx, eng.ID, sub, other, f.alice, at); !ok || err != nil {
			t.Fatalf("KeepReport(row %d) = %v, %v", i+1, ok, err)
		}
		got, _, err = f.store.Current(ctx, f.eng)
		if err != nil || got.Report == nil || !reflect.DeepEqual(*got.Report, other) {
			t.Fatalf("read back row %d's %+v, %v; want %+v", i+1, got.Report, err, other)
		}
	}
}

// gateRows are four patterns of a report's passed and its eight gates, in
// UpgradeGates' order. Each gate's column is its own and neither all
// passed nor all failed, so that any two gates exchanged, or a gate made
// constant, show in one row (review P2S2C-M6).
func gateRows() [][9]bool {
	return [][9]bool{
		// passed, agents, source root, reserved titles, frontmatter, sources, links, preserved, index
		{true, true, false, true, false, true, false, true, false},
		{false, true, true, false, false, false, true, true, false},
		{true, false, true, true, true, false, false, true, false},
		{true, false, false, false, true, true, true, false, true},
	}
}

// withGates sets r's passed and gates to row.
func withGates(r *domain.UpgradeReport, row [9]bool) {
	g := &r.Gates
	r.Passed, g.Agents, g.SourceRoot, g.ReservedTitles.Passed, g.Frontmatter, g.Sources, g.Links.Passed, g.Preserved.Passed, g.Index =
		row[0], row[1], row[2], row[3], row[4], row[5], row[6], row[7], row[8]
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
