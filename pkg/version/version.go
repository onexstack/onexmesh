// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package version provides build version information, injected via ldflags.
package version

import (
	"fmt"
	"runtime"
)

// Build-time variables, injected via -ldflags "-X ...".
var (
	gitVersion = "v0.0.0-master"
	gitCommit  = "unknown"
	buildDate  = "unknown"
)

// Info holds version metadata.
type Info struct {
	GitVersion string
	GitCommit  string
	BuildDate  string
	GoVersion  string
	Compiler   string
	Platform   string
}

// Get returns the current build info.
func Get() *Info {
	return &Info{
		GitVersion: gitVersion,
		GitCommit:  gitCommit,
		BuildDate:  buildDate,
		GoVersion:  runtime.Version(),
		Compiler:   runtime.Compiler,
		Platform:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a single-line version string carrying all build metadata.
func (i *Info) String() string {
	return fmt.Sprintf("%s (commit %s, built %s, %s, %s)",
		i.GitVersion, i.GitCommit, i.BuildDate, i.GoVersion, i.Platform)
}

// ToJSON returns the info as JSON.
func (i *Info) ToJSON() string {
	return fmt.Sprintf(
		`{"gitVersion":%q,"gitCommit":%q,"buildDate":%q,"goVersion":%q,"compiler":%q,"platform":%q}`,
		i.GitVersion, i.GitCommit, i.BuildDate, i.GoVersion, i.Compiler, i.Platform,
	)
}
