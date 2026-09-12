// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package stat

import "testing"

func TestCpuUsageInRange(t *testing.T) {
	v := CpuUsage()
	if v < 0 || v > 1000 {
		t.Fatalf("CpuUsage() = %d, want within [0, 1000]", v)
	}
}
