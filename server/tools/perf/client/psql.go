package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Exec runs a command and answers its standard output; an error with its
// standard error when it fails.
type Exec func(ctx context.Context, name string, args ...string) ([]byte, error)

// RunCommand is Exec by os/exec.
func RunCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// PSQL reads the serve's database through psql in its container (design
// 3.4: docker exec <container> psql -XAt -v ON_ERROR_STOP=1 -c '<SQL>'),
// as User on Database.
type PSQL struct {
	Container, User, Database string
	Run                       Exec
}

// NewPSQL is psql in container as stack.sh's user on its database.
func NewPSQL(container string) PSQL {
	return PSQL{Container: container, User: "nervewiki", Database: "nervewiki", Run: RunCommand}
}

// Query runs sql: its rows, each its fields (psql's unaligned output, '|'
// between them).
func (p PSQL) Query(ctx context.Context, sql string) ([][]string, error) {
	if p.Container == "" {
		return nil, errors.New("psql: no container")
	}
	out, err := p.Run(ctx, "docker", "exec", p.Container, "psql", "-XAt", "-v", "ON_ERROR_STOP=1", "-U", p.User, "-d", p.Database,
		"-c", sql)
	if err != nil {
		return nil, err
	}
	var rows [][]string
	for line := range strings.SplitSeq(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		rows = append(rows, strings.Split(line, "|"))
	}
	return rows, nil
}

// Value is the one field sql answers.
func (p PSQL) Value(ctx context.Context, sql string) (string, error) {
	rows, err := p.Query(ctx, sql)
	if err != nil {
		return "", err
	}
	if len(rows) != 1 || len(rows[0]) != 1 {
		return "", fmt.Errorf("psql %q answered %v, want one value", sql, rows)
	}
	return rows[0][0], nil
}

// Int is the one integer sql answers.
func (p PSQL) Int(ctx context.Context, sql string) (int64, error) {
	v, err := p.Value(ctx, sql)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}

// Quiet reports whether the serve's background is quiet (design 3.4.5's
// ops, review B's M8): the search queue empty and River without a job
// running or available.
func (p PSQL) Quiet(ctx context.Context) (bool, error) {
	n, err := p.Int(ctx, "SELECT (SELECT count(*) FROM page_search_queue) + "+
		"(SELECT count(*) FROM river_job WHERE state IN ('running', 'available'))")
	return n == 0, err
}

// Literal is s as a quoted SQL literal, for the ids the drivers name.
func Literal(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
