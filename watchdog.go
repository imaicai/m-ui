package main

import (
	"errors"
	"log"
	"os"
	"time"
)

// The watchdog exists for long-running servers: systemd already restarts an
// Xray that *crashes*, but nothing notices one that is stopped, stuck in a
// start-limit state or running with an unresponsive API. Only a *global*
// failure (service inactive / API not answering) triggers recovery. A single
// node whose port is unavailable never causes restarts of the healthy nodes.
const (
	watchdogInterval    = 45 * time.Second
	watchdogFailsNeeded = 2 // consecutive failed checks before acting (~90 s)
	watchdogBaseGap     = 2 * time.Minute
	watchdogMaxGap      = 30 * time.Minute
)

type watchdogState struct {
	failures   int
	recoveries int // recovery attempts since the last healthy check
	lastAction time.Time
}

func (w *watchdogState) reset() { *w = watchdogState{} }

// observe records one health check and reports whether a recovery should run
// now. Repeated recoveries back off exponentially (2, 4, 8 ... capped at 30
// minutes) so a persistent fault can never turn into a restart loop.
func (w *watchdogState) observe(healthy bool, now time.Time) bool {
	if healthy {
		w.failures, w.recoveries = 0, 0
		return false
	}
	w.failures++
	if w.failures < watchdogFailsNeeded {
		return false
	}
	// After recovery attempt n the next one waits base * 2^(n-1): 2, 4, 8, 16, 30, 30 ... minutes.
	gap := watchdogBaseGap
	for i := 1; i < w.recoveries && gap < watchdogMaxGap; i++ {
		gap *= 2
	}
	if gap > watchdogMaxGap {
		gap = watchdogMaxGap
	}
	if !w.lastAction.IsZero() && now.Sub(w.lastAction) < gap {
		return false
	}
	w.lastAction = now
	w.recoveries++
	return true
}

func hasEnabledNode(st AppState) bool {
	for _, n := range st.Nodes {
		if n.Enabled {
			return true
		}
	}
	return false
}

// xrayServing reports whether the Xray service is active and its local API
// answers.
func (x *XrayManager) xrayServing() error {
	if !serviceActive("xray") {
		return errors.New("xray 服务未运行")
	}
	if _, err := x.QueryUserTraffic(false); err != nil {
		return err
	}
	return nil
}

func (s *Server) xrayWatchdog() {
	ticker := time.NewTicker(watchdogInterval)
	defer ticker.Stop()
	var w watchdogState
	for now := range ticker.C {
		s.watchdogTick(&w, now)
	}
}

func (s *Server) watchdogTick(w *watchdogState, now time.Time) {
	// Held for the whole check so it can never interleave with a node change that is
	// being applied (which legitimately restarts or hot-swaps inbounds).
	s.trafficMu.Lock()
	defer s.trafficMu.Unlock()

	st := s.store.Snapshot()
	if !hasEnabledNode(st) {
		w.reset()
		return
	}
	if _, err := os.Stat(s.cfg.XrayBin); err != nil {
		return
	}
	err := s.xray.xrayServing()
	if !w.observe(err == nil, now) {
		return
	}
	log.Printf("m-ui: 看门狗检测到 Xray 异常（%v），正在尝试恢复", err)
	if rerr := s.xray.EnsureRunningConfig(st); rerr != nil {
		log.Printf("m-ui: 看门狗恢复失败，将按退避间隔重试: %v", rerr)
		return
	}
	log.Printf("m-ui: 看门狗已恢复 Xray")
}
