// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resiliency

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseStatusRanges parses a comma-separated list of status codes and inclusive
// ranges (e.g. "500,502-504") into a predicate. An empty string yields a
// predicate that matches nothing.
func ParseStatusRanges(s string) (func(int) bool, error) {
	if strings.TrimSpace(s) == "" {
		return func(int) bool { return false }, nil
	}

	var ranges [][2]int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if lo, hi, ok := strings.Cut(part, "-"); ok {
			loN, err := strconv.Atoi(strings.TrimSpace(lo))
			if err != nil {
				return nil, fmt.Errorf("resiliency: invalid status range %q: %w", part, err)
			}
			hiN, err := strconv.Atoi(strings.TrimSpace(hi))
			if err != nil {
				return nil, fmt.Errorf("resiliency: invalid status range %q: %w", part, err)
			}
			ranges = append(ranges, [2]int{loN, hiN})
			continue
		}

		n, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("resiliency: invalid status code %q: %w", part, err)
		}
		ranges = append(ranges, [2]int{n, n})
	}

	return func(code int) bool {
		for _, r := range ranges {
			if code >= r[0] && code <= r[1] {
				return true
			}
		}
		return false
	}, nil
}
