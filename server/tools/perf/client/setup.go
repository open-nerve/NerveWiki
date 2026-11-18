package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Prepared is what a round's drivers share (design 3.4.4): the workspace
// perf, its notebooks by name, and the personal access tokens, one a
// reader, writer, editor, tab or agent.
type Prepared struct {
	Base      string            `json:"base"`
	Workspace string            `json:"workspace"`
	Notebooks map[string]string `json:"notebooks"`
	PATs      []string          `json:"pats"`
}

// Prepare signs in as email, makes the workspace perf and the notebooks
// named (a standard notebook each) unless they are there, a round kept
// across a restart having them, and pats new tokens, which need the
// password.
func Prepare(ctx context.Context, base, email, password string, notebooks []string, pats int) (Prepared, error) {
	token, err := Login(ctx, base, email, password)
	if err != nil {
		return Prepared{}, err
	}
	c := New(base, token, -1, nil)
	p := Prepared{Base: base, Workspace: "perf", Notebooks: map[string]string{}}
	var spaces struct {
		Data []struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	if _, err := c.JSON(ctx, "workspaces", http.MethodGet, "/api/v0/workspaces", nil, &spaces, http.StatusOK); err != nil {
		return Prepared{}, err
	}
	found := false
	for _, w := range spaces.Data {
		found = found || w.Slug == p.Workspace
	}
	if !found {
		if _, err := c.JSON(ctx, "workspace", http.MethodPost, "/api/v0/workspaces", map[string]string{"name": "Perf", "slug": p.Workspace},
			nil, http.StatusCreated); err != nil {
			return Prepared{}, err
		}
	}
	var books struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	path := "/api/v0/workspaces/" + url.PathEscape(p.Workspace) + "/notebooks"
	if _, err := c.JSON(ctx, "notebooks", http.MethodGet, path, nil, &books, http.StatusOK); err != nil {
		return Prepared{}, err
	}
	for _, b := range books.Data {
		p.Notebooks[b.Name] = b.ID
	}
	for _, name := range notebooks {
		if p.Notebooks[name] != "" {
			continue
		}
		var made struct {
			ID string `json:"id"`
		}
		if _, err := c.JSON(ctx, "notebook", http.MethodPost, path, map[string]string{"name": name}, &made, http.StatusCreated); err != nil {
			return Prepared{}, err
		}
		p.Notebooks[name] = made.ID
	}
	for i := range pats {
		var made struct {
			Token string `json:"token"`
		}
		// The password's bucket keeps its burst of 5 at 600 a minute
		// (decision 460): past the burst a token is refused until it
		// refills, a tenth of a second; Retry-After is a second at least.
		for wait := 0; ; wait++ {
			_, err := c.JSON(ctx, "pat", http.MethodPost, "/api/v0/me/api-tokens",
				map[string]string{"name": fmt.Sprintf("perf %d", i), "current_password": password}, &made, http.StatusCreated)
			var se *StatusError
			if errors.As(err, &se) && se.Status == http.StatusTooManyRequests && wait < patRetries {
				if err := pause(ctx, time.Second); err != nil {
					return Prepared{}, err
				}
				continue
			}
			if err != nil {
				return Prepared{}, err
			}
			break
		}
		p.PATs = append(p.PATs, made.Token)
	}
	return p, nil
}

// patRetries is how often a token refused by the rate limit is asked for
// again.
const patRetries = 60

// pause waits d, or until ctx ends.
func pause(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Save writes p to path, readable by its owner alone.
func (p Prepared) Save(path string) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

// LoadPrepared reads what Save wrote.
func LoadPrepared(path string) (Prepared, error) {
	var p Prepared
	b, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	return p, json.Unmarshal(b, &p)
}

// Clients are a client a token of p, from the first, as many as want: an
// error when p has fewer.
func (p Prepared) Clients(want int, log *Log) ([]*Client, error) {
	if want > len(p.PATs) {
		return nil, fmt.Errorf("%d credentials wanted, %d prepared", want, len(p.PATs))
	}
	out := make([]*Client, want)
	for i := range out {
		out[i] = New(p.Base, p.PATs[i], i, log)
	}
	return out, nil
}

// Notebook is the notebook name's id: an error when p has none.
func (p Prepared) Notebook(name string) (string, error) {
	id := p.Notebooks[name]
	if id == "" {
		return "", fmt.Errorf("no notebook %q prepared", name)
	}
	return id, nil
}
