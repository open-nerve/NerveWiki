package migrations_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The four conditions of 00046's filter as Go reads them (M9/P4 design
// 3.2.2), for the floors of the random pages: the bound's size, a long run
// as written, a long data: string in any case, a run or a data: string
// made long by the eleven runes that fold to ASCII.
var (
	partialRun   = regexp.MustCompile(`[0-9A-Za-z]{65}`)
	partialURL   = regexp.MustCompile("(?i)data:[^\t\n\v\f\r )\"'<>\\]`]{60}")
	partialFolds = regexp.MustCompile(`[\x{DF}\x{17F}\x{1E9E}\x{212A}\x{FB00}-\x{FB06}]`)
	partialFold  = regexp.MustCompile(`[0-9A-Za-z\x{DF}\x{17F}\x{1E9E}\x{212A}\x{FB00}-\x{FB06}]{22}`)
	partialData  = regexp.MustCompile(`(?i)data:`)
)

// filtered are the conditions of 00046's filter content meets.
func filtered(content string) [4]bool {
	return [4]bool{len(content) > domain.BoundFreeContentBytes, partialRun.MatchString(content), partialURL.MatchString(content),
		partialFolds.MatchString(content) && (partialFold.MatchString(content) || partialData.MatchString(content))}
}

// randomCut is a random content of the pieces the cuts are made of: words,
// runs of ASCII letters and digits of 60 to 70, runs of 18 to 26 of the
// eleven that fold to ASCII and of ASCII letters, data: strings in any
// case of 55 to 66 characters after "data:", CJK; one in 40 past the
// threshold, of prose.
func randomCut(r *rand.Rand) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	folds := []rune{0xDF, 0x17F, 0x1E9E, 0x212A, 0xFB00, 0xFB01, 0xFB02, 0xFB03, 0xFB04, 0xFB05, 0xFB06}
	var b strings.Builder
	if r.IntN(40) == 0 {
		for b.Len() <= domain.BoundFreeContentBytes+r.IntN(4000) {
			b.WriteString("the quick brown fox jumps over the lazy dog 1234 ")
		}
		return b.String()
	}
	for range 1 + r.IntN(6) {
		switch r.IntN(6) {
		case 0:
			for range 1 + r.IntN(12) {
				b.WriteByte(letters[r.IntN(len(letters))])
			}
		case 1:
			for range 60 + r.IntN(11) {
				b.WriteByte(letters[r.IntN(len(letters))])
			}
		case 2:
			for range 18 + r.IntN(9) {
				if r.IntN(2) == 0 {
					b.WriteRune(folds[r.IntN(len(folds))])
				} else {
					b.WriteByte(letters[r.IntN(52)])
				}
			}
		case 3:
			b.WriteString([]string{"data:", "DATA:", "Data:"}[r.IntN(3)])
			for range 55 + r.IntN(12) {
				b.WriteByte("abcXYZ019/+=;,.-_%"[r.IntN(18)])
			}
		case 4:
			b.WriteString("数据库设计")
		default:
			b.WriteString("plain words")
		}
		b.WriteString([]string{" ", "\n", ")", "，", ""}[r.IntN(5)])
	}
	return b.String()
}

// 00046 adds page_search.partial, false for the rows there, refused true
// without a body, and queues in bulk every live page SearchTokens marks
// partial, a superset of them (M9/P4 design 3.2.2, 6.1): the schema goes
// down to 00045, a notebook's pages are written as the program before
// wrote them: 2,000 of random contents, 3,000 that no cut reaches, and,
// built, a page each condition alone picks (240 KiB of random CJK, partial
// by the bound alone; 65 ASCII letters; "data:" and 60 characters; 22 ﬃ),
// the threshold's two (74,001 and 74,000 bytes of prose, the first queued,
// the second not, which pins the SQL's number to BoundFreeContentBytes), a
// page queued exact before, which stays exact, a cut page in the trash and
// an attachment, neither queued; then up. Each condition picks 1% of the
// random pages at least. Down, the column and its check go; up again.
func TestTheMigrationQueuesEveryPartialPage(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewDatabase(t))
	m, err := postgres.NewMigrator(pool, migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	downTo(t, m, 46)

	const (
		user = "0199a2b4-0000-7000-8000-000000000001"
		ws   = "0199a2b4-0000-7000-8000-000000000002"
		nb   = "0199a2b4-0000-7000-8000-000000000003"
	)
	exec := func(stmt string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, stmt, args...); err != nil {
			t.Fatalf("%.120s: %v", stmt, err)
		}
	}
	exec(`INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'a@corp.com', 'x', 'a', now(), now())`, user)
	exec(`INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at)
		VALUES ($1, 'acme', 'Acme', $2, $2, now(), now())`, ws, user)
	exec(`INSERT INTO notebooks (id, workspace_id, name, workspace_access, created_by_id, updated_by_id, created_at, updated_at)
		VALUES ($1, $2, 'Eng', 'editor', $3, $3, now(), now())`, nb, ws, user)

	var pages []seededPage
	contentOf := map[uuid.UUID]string{}
	add := func(content string) uuid.UUID {
		p := seededPage{id: uuid.NewV7(), content: content}
		pages, contentOf[p.id] = append(pages, p), content
		return p.id
	}
	r := rand.New(rand.NewPCG(46, 0))
	var random []uuid.UUID
	for range 2_000 {
		random = append(random, add(randomCut(r)))
	}
	var plain []uuid.UUID
	for i := range 3_000 {
		plain = append(plain, add(fmt.Sprintf("plain words of page %d, [[a link]] and **bold** 数据库", i)))
	}
	var cjk strings.Builder
	for cjk.Len() < 240<<10 {
		cjk.WriteRune(rune(0x4E00 + r.IntN(20_000)))
	}
	prose := func(n int) string { return strings.Repeat("the quick brown fox jumps over ", n/31+1)[:n] }
	built := map[string]uuid.UUID{
		"the bound alone":     add(cjk.String()),
		"a long run":          add("a " + strings.Repeat("q", 65) + " b"),
		"a long data: string": add("![](data:" + strings.Repeat("Q/", 30) + ")"),
		"22 ﬃ":                add(strings.Repeat(string(rune(0xFB03)), 22)),
		"74,001 bytes":        add(prose(domain.BoundFreeContentBytes + 1)),
	}
	under := add(prose(domain.BoundFreeContentBytes))
	exact := add(strings.Repeat("e", 70))
	gone := add(strings.Repeat("g", 70))
	pages[len(pages)-1].deleted = true
	ids, names, contents, hashes, sizes, deleted := []uuid.UUID{}, []string{}, []string{}, [][]byte{}, []int32{}, []bool{}
	for i, p := range pages {
		sum := sha256.Sum256([]byte(p.content))
		ids, names, contents = append(ids, p.id), append(names, fmt.Sprint("p", i)), append(contents, p.content)
		hashes, sizes, deleted = append(hashes, sum[:]), append(sizes, int32(len(p.content))), append(deleted, p.deleted)
	}
	exec(`INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
			updated_at, deleted_at)
		SELECT x.id, $2, NULL, 'page', x.name, x.name, 0, $3, $3, now(), now(), CASE WHEN x.deleted THEN now() END
		FROM unnest($1::uuid[], $4::text[], $5::boolean[]) x (id, name, deleted)`, ids, nb, user, names, deleted)
	exec(`INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at, deleted_at)
		SELECT x.id, x.content, 1, x.hash, x.size, $5, now(), CASE WHEN x.deleted THEN now() END
		FROM unnest($1::uuid[], $2::text[], $3::bytea[], $4::integer[], $6::boolean[]) x (id, content, hash, size, deleted)`,
		ids, contents, hashes, sizes, user, deleted)
	asset := uuid.NewV7()
	exec(`INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
		updated_at) VALUES ($1, $2, NULL, 'asset', 'x.png', 'x.png', 0, $3, $3, now(), now())`, asset, nb, user)
	exec(`INSERT INTO page_search (node_id, revision, name, tokenizer, built_at, title, body)
		VALUES ($1, 1, 'p0', '4/17.0.0/17.0.0', now(), 'p0', 'p0')`, plain[0])
	exec(`INSERT INTO page_search_queue (node_id, notebook_id, bulk) VALUES ($1, $2, false)`, exact, nb)

	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up(): %v", err)
	}
	rows, err := pool.Query(ctx, `SELECT node_id, notebook_id = $1, bulk FROM page_search_queue`, nb)
	if err != nil {
		t.Fatal(err)
	}
	type queued struct{ ofNotebook, bulk bool }
	queue := map[uuid.UUID]queued{}
	for rows.Next() {
		var id uuid.UUID
		var q queued
		if err := rows.Scan(&id, &q.ofNotebook, &q.bulk); err != nil {
			t.Fatal(err)
		}
		queue[id] = q
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	partials, conditions := 0, [4]int{}
	for _, p := range pages {
		if p.deleted {
			continue
		}
		_, _, partial := domain.SearchTokens("p", &p.content)
		q, ok := queue[p.id]
		switch {
		case partial && !ok:
			t.Errorf("a partial page is not queued: %.80q (%d bytes)", p.content, len(p.content))
		case ok && (!q.ofNotebook || q.bulk == (p.id == exact)):
			t.Errorf("%s queued %+v, want of its notebook, in bulk but the one queued exact before", p.id, q)
		}
		if partial {
			partials++
		}
	}
	for _, id := range random {
		for i, met := range filtered(contentOf[id]) {
			if met {
				conditions[i]++
			}
		}
	}
	for i, n := range conditions {
		if n < len(random)/100 {
			t.Errorf("condition %d picks %d of the %d random pages, want 1%% at least", i+1, n, len(random))
		}
	}
	for what, id := range built {
		content := contentOf[id]
		_, _, partial := domain.SearchTokens("p", &content)
		if _, ok := queue[id]; !ok || (!partial && what != "74,001 bytes") {
			t.Errorf("the page of %s: queued %v, partial %v; want it queued, and partial unless the threshold's", what, ok, partial)
		}
	}
	if picked := filtered(contentOf[built["the bound alone"]]); picked != [4]bool{true, false, false, false} {
		t.Errorf("the 240 KiB of CJK meet the conditions %v, want the bound's alone", picked)
	}
	for what, id := range map[string]uuid.UUID{"74,000 bytes": under, "a cut page in the trash": gone, "an attachment": asset} {
		if _, ok := queue[id]; ok {
			t.Errorf("the page of %s is queued, want it not", what)
		}
	}
	for _, id := range plain {
		if _, ok := queue[id]; ok {
			t.Fatalf("a page no cut reaches is queued: %s", id)
		}
	}
	t.Logf("%d pages queued, %d of them partial; the random pages' conditions %v", len(queue), partials, conditions)

	var partial bool
	if err := pool.QueryRow(ctx, `SELECT partial FROM page_search WHERE node_id = $1`, plain[0]).Scan(&partial); err != nil || partial {
		t.Errorf("the row there before is partial %v, %v; want false", partial, err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO page_search (node_id, revision, name, tokenizer, built_at, title, body, partial)
		VALUES ($1, NULL, 'x.png', '5/17.0.0/17.0.0', now(), 'png', NULL, true)`, asset)
	if pgErr := (*pgconn.PgError)(nil); !errors.As(err, &pgErr) || pgErr.ConstraintName != "page_search_partial_check" {
		t.Errorf("a partial row without a body = %v, want page_search_partial_check's refusal", err)
	}

	downTo(t, m, 46)
	if column(t, pool, "page_search", "partial") || constraint(t, pool, "page_search_partial_check") {
		t.Error("page_search.partial or its check left after Down")
	}
	if _, err := m.Up(ctx); err != nil || !column(t, pool, "page_search", "partial") {
		t.Fatalf("Up() again = %v, the column there %v", err, column(t, pool, "page_search", "partial"))
	}
}

// seededPage is a page the test writes as the program before 00046 wrote
// it: its content, and whether it is in the trash.
type seededPage struct {
	id      uuid.UUID
	content string
	deleted bool
}

// column reports whether table has the column.
func column(t *testing.T, pool *pgxpool.Pool, table, name string) bool {
	t.Helper()
	var ok bool
	if err := pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2)`, table, name).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	return ok
}

// constraint reports whether the schema has a constraint of that name.
func constraint(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var ok bool
	if err := pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = $1)`, name).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	return ok
}
