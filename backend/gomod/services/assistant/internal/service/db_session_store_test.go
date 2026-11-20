package service

import (
	"context"
	"testing"
)

// The guards are the only part of the store that can be checked without a
// database, and they are the security-relevant part: without a user id there
// is no owner, so every method has to refuse rather than read the whole table.
func TestDBSessionStoreRefusesAnonymousCalls(t *testing.T) {
	const sid = "11111111-1111-4111-8111-111111111111"
	store := NewDBSessionStore(nil, 12)
	ctx := context.Background()

	if msgs, err := store.Load(ctx, 0, sid); err != nil || msgs != nil {
		t.Fatalf("Load(anon) = %v, %v; want nil, nil", msgs, err)
	}
	if err := store.Append(ctx, 0, sid, Turn{Role: "user", Content: "hi"}); err != nil {
		t.Fatalf("Append(anon) = %v; want nil", err)
	}
	if conv, err := store.Latest(ctx, 0, 40); err != nil || conv.SessionID != "" {
		t.Fatalf("Latest(anon) = %+v, %v; want zero, nil", conv, err)
	}
}
