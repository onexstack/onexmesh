// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// value wraps a decoded configuration value and provides typed access.
type value struct {
	data any
}

func newValue(data any) Value { return &value{data: data} }

func (v *value) String() string {
	switch t := v.data.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

func (v *value) Int() int {
	switch t := v.data.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case uint64:
		return int(t)
	case float64:
		return int(t)
	case bool:
		if t {
			return 1
		}
		return 0
	case string:
		n, _ := strconv.Atoi(t)
		return n
	default:
		return 0
	}
}

func (v *value) Int64() int64 {
	switch t := v.data.(type) {
	case int:
		return int64(t)
	case int64:
		return t
	case uint64:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	default:
		return 0
	}
}

func (v *value) Float64() float64 {
	switch t := v.data.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	default:
		return 0
	}
}

func (v *value) Bool() bool {
	switch t := v.data.(type) {
	case bool:
		return t
	case string:
		b, _ := strconv.ParseBool(t)
		return b
	case int:
		return t != 0
	default:
		return false
	}
}

func (v *value) Duration() time.Duration {
	if t, ok := v.data.(time.Duration); ok {
		return t
	}
	if s, ok := v.data.(string); ok {
		if d, err := time.ParseDuration(s); err == nil {
			return d
		}
	}
	return time.Duration(v.Int64())
}

func (v *value) Bytes() []byte {
	if b, ok := v.data.([]byte); ok {
		return b
	}
	b, _ := json.Marshal(v.data)
	return b
}

func (v *value) Scan(out interface{}) error {
	b, err := json.Marshal(v.data)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}
