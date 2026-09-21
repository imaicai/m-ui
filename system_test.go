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
