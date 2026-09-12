// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build !linux

package stat

// refreshCPU returns 0 on non-Linux platforms where /proc/stat is unavailable,
// effectively disabling CPU-based overload protection.
func refreshCPU() uint64 {
	return 0
}
