package main

import "testing"

func TestNextCountryNodeName(t *testing.T) {
	nodes := []Node{{Name: "US01"}, {Name: "US03"}, {Name: "JP01"}, {Name: "custom"}}
	if got := nextCountryNodeName("us", nodes); got != "US04" {
		t.Fatalf("got %q, want US04", got)
	}
	if got := nextCountryNodeName("JP", nodes); got != "JP02" {
		t.Fatalf("got %q, want JP02", got)
	}
	if got := nextCountryNodeName("", nodes); got != "NODE01" {
		t.Fatalf("got %q, want NODE01", got)
	}
}

func TestValidServerIdentityRejectsPrivateIP(t *testing.T) {
	if validServerIdentity(serverIdentity{IP: "10.0.0.8", Country: "US"}) {
		t.Fatal("private IP must never be used as a node address")
	}
	if !validServerIdentity(serverIdentity{IP: "8.8.8.8", Country: "US"}) {
		t.Fatal("public IP with valid country should be accepted")
	}
	if validServerIdentity(serverIdentity{IP: "8.8.8.8", Country: "XX"}) {
		t.Fatal("unknown country code must be rejected")
	}
}

func TestApplyNodeCountryBackfill(t *testing.T) {
	st := defaultState()
	st.Nodes = []Node{
		{ID: "n1", Name: "US01"},
		{ID: "n2", Name: "US02", Country: "JP"},
	}
	if !applyNodeCountryBackfill(&st, "us") {
		t.Fatal("expected missing country to be backfilled")
	}
	if st.Nodes[0].Country != "US" {
		t.Fatalf("unexpected backfilled country: %q", st.Nodes[0].Country)
	}
	if st.Nodes[1].Country != "JP" {
		t.Fatalf("existing country must not be overwritten: %q", st.Nodes[1].Country)
	}
	if normalizeCountryCode("xx") != "" || normalizeCountryCode("USA") != "" || normalizeCountryCode(" jp ") != "JP" {
		t.Fatal("country code normalization failed")
	}
}



func TestCPUPercentBetween(t *testing.T) {
	prev := cpuSample{idle: 700, total: 1000}
	current := cpuSample{idle: 770, total: 1100}
	if got := cpuPercentBetween(prev, current); got != 30 {
		t.Fatalf("got %.2f%%, want 30%%", got)
	}
	if got := cpuPercentBetween(current, prev); got != 0 {
		t.Fatalf("counter rollback must return 0, got %.2f%%", got)
	}
}
