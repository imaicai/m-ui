package main

import (
	"path/filepath"
	"testing"
)

func TestInitAndPassword(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	s, err := NewStore(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureInitialized("admin", "strong-pass-123"); err != nil {
		t.Fatal(err)
	}
	st := s.Snapshot()
	if st.Settings.AdminUser != "admin" {
		t.Fatalf("unexpected user: %s", st.Settings.AdminUser)
	}
	if !verifyPassword("strong-pass-123", st.Settings.PasswordSalt, st.Settings.PasswordHash) {
		t.Fatal("password verification failed")
	}
	if verifyPassword("wrong", st.Settings.PasswordSalt, st.Settings.PasswordHash) {
		t.Fatal("wrong password accepted")
	}
	if len(st.Subscriptions) != 1 {
		t.Fatalf("expected default subscription")
	}
}
