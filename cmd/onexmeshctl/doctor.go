// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/onexstack/onexmesh/pkg/version"
)

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the environment for onexmesh compatibility",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("onexmeshctl version: %s\n", version.Get().GitVersion)
			fmt.Printf("go version:          %s\n", runtime.Version())
			fmt.Printf("platform:            %s/%s\n", runtime.GOOS, runtime.GOARCH)
			fmt.Println("environment ok")
		},
	}
}
