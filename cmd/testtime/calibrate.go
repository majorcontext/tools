package main

import (
	"os"
	"path/filepath"
	"time"
)

const (
	calibrationRuns  = 2
	cpuRounds        = 20_000_000
	fileRounds       = 600
	calibrationBytes = 4096
)

// calibrate runs a fixed workload, a CPU loop and a loop of file create, write, stat and remove, and returns the
// seconds of the fastest of a few runs. The workload is the same on every machine, so the ratio of two results
// measures the speed of the machines.
func calibrate() float64 {
	best := time.Duration(1<<63 - 1)
	for range calibrationRuns {
		start := time.Now()
		cpuWork()
		fileWork()
		best = min(best, time.Since(start))
	}
	return best.Seconds()
}

var sink uint64

func cpuWork() {
	x := uint64(88172645463325252)
	for range cpuRounds {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
	}
	sink = x
}

func fileWork() {
	dir, err := os.MkdirTemp("", "testtime-calibrate")
	if err != nil {
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()
	data := make([]byte, calibrationBytes)
	path := filepath.Join(dir, "f")
	for range fileRounds {
		if os.WriteFile(path, data, 0o600) != nil {
			return
		}
		if _, err := os.Stat(path); err != nil {
			return
		}
		if os.Remove(path) != nil {
			return
		}
	}
}
