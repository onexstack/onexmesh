// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// serviceKinds maps the onexmeshctl `new` kind to the onexai `create`
// subcommand and any extra flags.
var serviceKinds = map[string]struct {
	subcommand string
	extraFlags map[string]string
}{
	"webserver": {subcommand: "webserver"},
	"grpc":      {subcommand: "webserver", extraFlags: map[string]string{"web-framework": "grpc"}},
	"mcpserver": {subcommand: "mcpserver"},
	"jobserver": {subcommand: "jobserver"},
	"mqserver":  {subcommand: "mqserver"},
	"clitool":   {subcommand: "clitool"},
}

func newNewCommand() *cobra.Command {
	var (
		dir          string
		binaryName   string
		registry     string
		webFramework string
		modulePath   string
	)

	cmd := &cobra.Command{
		Use:   "new KIND SERVICE",
		Short: "Scaffold a new onexmesh service via onexai create",
		Long: `Scaffold a new service in the onexstack layout by delegating to onexai create.

KIND is one of: webserver, grpc, mcpserver, jobserver, mqserver, clitool.
SERVICE is the logical service name, e.g. "edu.course.student-api". Its final
dot-separated segment becomes the binary name and default output directory.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind := args[0]
			service := args[1]

			kindSpec, ok := serviceKinds[kind]
			if !ok {
				return fmt.Errorf("unknown kind %q (supported: webserver, grpc, mcpserver, jobserver, mqserver, clitool)", kind)
			}

			seg := finalSegment(service)
			if dir == "" {
				dir = "./" + seg
			}
			if binaryName == "" {
				binaryName = seg
			}
			if modulePath == "" {
				modulePath = "github.com/onexstack/" + seg
			}

			argv := []string{"create", kindSpec.subcommand, dir,
				"--binary-name", binaryName,
				"--module-path", modulePath,
			}
			if registry != "" {
				argv = append(argv, "--service-registry", registry)
			}
			if wf := webFramework; wf != "" {
				argv = append(argv, "--web-framework", wf)
			} else if ewf, ok := kindSpec.extraFlags["web-framework"]; ok {
				argv = append(argv, "--web-framework", ewf)
			}

			return runOnexai(argv...)
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Output directory (default ./<binary-name>).")
	cmd.Flags().StringVar(&binaryName, "binary-name", "", "Binary name (default: last segment of SERVICE).")
	cmd.Flags().StringVar(&registry, "registry", "", "Service registry: none, polaris.")
	cmd.Flags().StringVar(&webFramework, "web-framework", "", "Web framework: gin, grpc, ...")
	cmd.Flags().StringVar(&modulePath, "module-path", "", "Go module path (default github.com/onexstack/<binary-name>).")

	return cmd
}

// runOnexai invokes the onexai CLI, streaming its output to the terminal.
func runOnexai(args ...string) error {
	path, err := exec.LookPath("onexai")
	if err != nil {
		return fmt.Errorf("onexai not found in PATH: %w", err)
	}

	c := exec.Command(path, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin

	if err := c.Run(); err != nil {
		return fmt.Errorf("onexai %s failed: %w", strings.Join(args, " "), err)
	}
	return nil
}

// finalSegment returns the substring after the last dot, e.g.
// "edu.course.student-api" -> "student-api".
func finalSegment(service string) string {
	if i := strings.LastIndexByte(service, '.'); i >= 0 {
		return service[i+1:]
	}
	return service
}
