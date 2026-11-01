package main

import (
	"testing"
	"time"
)

func TestWatchdogWaitsForConsecutiveFailures(t *testing.T) {
	var w watchdogState
	now := time.Unix(1_700_000_000, 0)
	if w.observe(false, now) {
		t.Fatal("a single failed check must not trigger recovery")
	}
	if !w.observe(false, now.Add(watchdogInterval)) {
		t.Fatal("second consecutive failure must trigger recovery")
	}
}

func TestWatchdogHealthyCheckResetsCounters(t *testing.T) {
	var w watchdogState
	now := time.Unix(1_700_000_000, 0)
	w.observe(false, now)
	w.observe(true, now.Add(watchdogInterval))
	if w.observe(false, now.Add(2*watchdogInterval)) {
		t.Fatal("failure count must restart after a healthy check")
	}
}

func TestWatchdogBacksOffExponentiallyAndIsCapped(t *testing.T) {
	var w watchdogState
	now := time.Unix(1_700_000_000, 0)
	w.observe(false, now)

	// First recovery as soon as the failure is confirmed.
	now = now.Add(watchdogInterval)
	if !w.observe(false, now) {
		t.Fatal("first recovery expected")
	}
	last := now

	gap := watchdogBaseGap
	for i := 0; i < 8; i++ {
		// Just inside the gap: no action.
		if w.observe(false, last.Add(gap-time.Second)) {
			t.Fatalf("recovery #%d fired %v before the back-off gap %v elapsed", i+2, time.Second, gap)
		}
		// Gap elapsed: act again.
		last = last.Add(gap)
		if !w.observe(false, last) {
			t.Fatalf("recovery #%d expected after %v", i+2, gap)
		}
		gap *= 2
		if gap > watchdogMaxGap {
			gap = watchdogMaxGap
		}
	}
	if gap != watchdogMaxGap {
		t.Fatalf("back-off never reached the cap: %v", gap)
	}
}

func TestWatchdogRecoveryResetsBackoffOnceHealthy(t *testing.T) {
	var w watchdogState
	now := time.Unix(1_700_000_000, 0)
	w.observe(false, now)
	w.observe(false, now.Add(watchdogInterval)) // recovery #1
	w.observe(true, now.Add(2*watchdogInterval))

	later := now.Add(10 * time.Minute)
	w.observe(false, later)
	if !w.observe(false, later.Add(watchdogInterval)) {
		t.Fatal("after recovery the next incident must be handled promptly, not with a stale back-off")
	}
}

func TestHasEnabledNode(t *testing.T) {
	if hasEnabledNode(AppState{}) {
		t.Fatal("empty state has no enabled node")
	}
	if hasEnabledNode(AppState{Nodes: []Node{{ID: "a"}}}) {
		t.Fatal("disabled node must not count")
	}
	if !hasEnabledNode(AppState{Nodes: []Node{{ID: "a"}, {ID: "b", Enabled: true}}}) {
		t.Fatal("enabled node must count")
	}
}
