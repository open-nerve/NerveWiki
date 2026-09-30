package app_test

import (
	"bytes"
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

const tokenIDText = "0199a2b4-0000-7000-8000-00000000000a"

func testTokenID() uuid.UUID { return uuid.MustParse(tokenIDText) }

// testPAT is a token whose hash fakeAPITokens knows.
func testPAT() domain.PAT {
	var p domain.PAT
	for i := range p {
		p[i] = byte(i)
	}
	return p
}

// validToken is testPAT's row: the test account's, live, never used.
func validToken() app.APITokenCredential {
	return app.APITokenCredential{ID: testTokenID(), UserID: testUserID(), UserActive: true}
}

// fakeAPITokens is one personal access token, in memory.
type fakeAPITokens struct {
	token    app.APITokenCredential
	hash     []byte // the token's hash
	readErr  error  // the error of every read
	touchErr error
	touches  []time.Time // the now of each TouchAPIToken
	stale    []time.Time // its staleBefore
	byID     []uuid.UUID // ids read
}

func newFakeAPITokens(token app.APITokenCredential) *fakeAPITokens {
	return &fakeAPITokens{token: token, hash: testPAT().Hash()}
}

func (f *fakeAPITokens) APITokenByHash(_ context.Context, hash []byte) (app.APITokenCredential, error) {
	if f.readErr != nil {
		return app.APITokenCredential{}, f.readErr
	}
	if !bytes.Equal(hash, f.hash) {
		return app.APITokenCredential{}, app.ErrNotFound
	}
	return f.token, nil
}

func (f *fakeAPITokens) APITokenByID(_ context.Context, id uuid.UUID) (app.APITokenCredential, error) {
	f.byID = append(f.byID, id)
	if f.readErr != nil {
		return app.APITokenCredential{}, f.readErr
	}
	if id != f.token.ID {
		return app.APITokenCredential{}, app.ErrNotFound
	}
	return f.token, nil
}

func (f *fakeAPITokens) TouchAPIToken(_ context.Context, id uuid.UUID, now, staleBefore time.Time) error {
	if id == f.token.ID {
		f.touches = append(f.touches, now)
		f.stale = append(f.stale, staleBefore)
	}
	return f.touchErr
}
