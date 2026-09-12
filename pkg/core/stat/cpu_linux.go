// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build linux

package stat

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"
)

const procStat = "/proc/stat"

var (
	cpuMu     sync.Mutex
	prevIdle  uint64
	prevTotal uint64
)

// refreshCPU reads /proc/stat and returns the system-wide CPU usage in per-mille
// (0-1000) since the previous sample. idle and iowait are both treated as idle.
func refreshCPU() uint64 {
	f, err := os.Open(procStat)
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 6 || fields[0] != "cpu" {
			continue
		}

		var total, idle uint64
		for i := 1; i < len(fields); i++ {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			total += v
			// fields: user nice system idle iowait ...
			if i == 4 || i == 5 {
				idle += v
			}
		}
		if total == 0 {
			return 0
		}

		cpuMu.Lock()
		dTotal := total - prevTotal
		dIdle := idle - prevIdle
		prevTotal = total
		prevIdle = idle
		cpuMu.Unlock()

		if dTotal == 0 {
			return 0
		}
		return 1000 - dIdle*1000/dTotal
	}
	return 0
}
