// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package informer generates the informers/ subtree (factory, group/version
// interfaces, per-resource informers) in the client-go style. The naming and
// import-path helpers live in the shared templates package; this package only
// builds the per-generator view structs and delegates rendering.
package informer
