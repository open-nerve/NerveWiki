package drive

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/open-nerve/NerveWiki/server/tools/perf/client"
)

// Ops is B4's (design 3.6, 3.4.5): on the imported vault, the page Root
// at the notebook's root, Reps each of its deletion, restored by a
// background restore that must succeed, its move under another page of
// the root, its rename, and the rename of the page most linked to; then
// the deletion of its first folder of the first level, recorded alone.
// Before each, the background quiet (psql: the search queue empty, River
// without a job running or available, review B's M8), then Idle.
type Ops struct {
	Client   *client.Client
	PSQL     client.PSQL
	Notebook string
	Root     string
	Reps     int
	Idle     time.Duration
	Poll     time.Duration
	Out      io.Writer // a line an operation
}

// quiet waits for the background to be quiet, then o.Idle.
func (o Ops) quiet(ctx context.Context) error {
	for {
		q, err := o.PSQL.Quiet(ctx)
		if err != nil {
			return err
		}
		if q {
			return sleep(ctx, o.Idle)
		}
		if err := sleep(ctx, o.Poll); err != nil {
			return err
		}
	}
}

// id is the one id sql answers.
func (o Ops) id(ctx context.Context, what, sql string) (string, error) {
	v, err := o.PSQL.Value(ctx, sql)
	if err != nil {
		return "", fmt.Errorf("%s: %w", what, err)
	}
	return v, nil
}

// timed does op after the quiet, and writes its time and status.
func (o Ops) timed(ctx context.Context, op string, do func() (client.Record, error)) error {
	if err := o.quiet(ctx); err != nil {
		return err
	}
	rec, err := do()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	_, err = fmt.Fprintf(o.Out, "%s status=%d took_ms=%.1f\n", op, rec.Status, float64(rec.Took())/float64(time.Millisecond))
	return err
}

// deleteAndRestore deletes id, then restores it in the background and
// waits for the restore to succeed.
func (o Ops) deleteAndRestore(ctx context.Context, op, id string) error {
	if err := o.timed(ctx, op, func() (client.Record, error) {
		return o.Client.JSON(ctx, op, http.MethodDelete, "/api/v0/nodes/"+url.PathEscape(id), nil, nil, http.StatusNoContent)
	}); err != nil {
		return err
	}
	var j job
	if _, err := o.Client.JSON(ctx, op+"-restore", http.MethodPost, "/api/v0/notebooks/"+url.PathEscape(o.Notebook)+"/trash-restores",
		map[string]string{"node_id": id}, &j, http.StatusAccepted); err != nil {
		return err
	}
	_, err := awaitJob(ctx, o.Client, op+"-restore-job", j.ID, o.Poll)
	return err
}

// Run runs the operations.
func (o Ops) Run(ctx context.Context) error {
	nb := client.Literal(o.Notebook)
	root, err := o.id(ctx, "the vault's page", "SELECT id FROM nodes WHERE notebook_id = "+nb+" AND parent_id IS NULL AND name = "+
		client.Literal(o.Root)+" AND deleted_at IS NULL")
	if err != nil {
		return err
	}
	folder, err := o.id(ctx, "its first folder", "SELECT id FROM nodes WHERE notebook_id = "+nb+" AND parent_id = "+client.Literal(root)+
		" AND name = 'f1' AND deleted_at IS NULL")
	if err != nil {
		return err
	}
	linked, err := o.PSQL.Query(ctx, "SELECT l.resolved_id, n.name, count(DISTINCT l.source_id) FROM page_links l JOIN nodes n ON n.id = l.resolved_id "+
		"WHERE l.notebook_id = "+nb+" AND n.deleted_at IS NULL GROUP BY 1, 2 ORDER BY 3 DESC, 1 LIMIT 1")
	if err != nil || len(linked) != 1 || len(linked[0]) != 3 {
		return fmt.Errorf("the page most linked to: %v, %w", linked, err)
	}
	if _, err := fmt.Fprintf(o.Out, "most-linked id=%s name=%q linking_pages=%s\n", linked[0][0], linked[0][1], linked[0][2]); err != nil {
		return err
	}
	run := time.Now().UnixMilli()
	var homes [2]page
	for i := range homes {
		if homes[i], err = createPage(ctx, o.Client, "setup-page", o.Notebook, "", fmt.Sprintf("elsewhere %d %c", run, 'a'+i), ""); err != nil {
			return err
		}
	}
	for range o.Reps {
		if err := o.deleteAndRestore(ctx, "delete-vault", root); err != nil {
			return err
		}
	}
	// The moves: under one home, the other, then back at the root, and
	// so on; the renames, the last back to the name, so that the vault is
	// as it was for the export after it.
	for rep := range o.Reps {
		var parent any
		if rep < o.Reps-1 {
			parent = homes[rep%2].ID
		}
		if err := o.timed(ctx, "move-vault", func() (client.Record, error) {
			return o.Client.JSON(ctx, "move-vault", http.MethodPost, "/api/v0/nodes/"+url.PathEscape(root)+"/move",
				map[string]any{"parent_id": parent}, nil, http.StatusOK)
		}); err != nil {
			return err
		}
	}
	for _, r := range []struct{ op, id, name string }{{"rename-vault", root, o.Root}, {"rename-most-linked", linked[0][0], linked[0][1]}} {
		for rep := range o.Reps {
			name := r.name
			if rep < o.Reps-1 {
				name = fmt.Sprintf("%s r%d", r.name, rep+1)
			}
			if err := o.timed(ctx, r.op, func() (client.Record, error) {
				return o.Client.JSON(ctx, r.op, http.MethodPatch, "/api/v0/nodes/"+url.PathEscape(r.id), map[string]string{"name": name}, nil,
					http.StatusOK)
			}); err != nil {
				return err
			}
		}
	}
	for range o.Reps {
		if err := o.deleteAndRestore(ctx, "delete-folder", folder); err != nil {
			return err
		}
	}
	return o.quiet(ctx)
}
