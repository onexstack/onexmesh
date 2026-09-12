// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package app provides a CLI application bootstrap: config loading, flag
// binding, options validation, slog init, signal-aware RunFunc and graceful
// shutdown hooks, following the onex app framework.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	_ "go.uber.org/automaxprocs"

	"github.com/onexstack/onexmesh/pkg/options"
	"github.com/onexstack/onexmesh/pkg/version"
)

// RunFunc is the application startup callback. ctx is signal-aware.
type RunFunc func(ctx context.Context) error

// LifecycleHook is called at a lifecycle stage.
type LifecycleHook func(ctx context.Context) error

// OptionsValidator validates options, returning a single error.
type OptionsValidator interface {
	Validate() error
}

// OptionsCompleter post-processes options after flag binding.
type OptionsCompleter interface {
	Complete() error
}

// FlagSetOptions adds flags and validates, without a prefix.
type FlagSetOptions interface {
	AddFlags(fs *pflag.FlagSet)
	OptionsValidator
}

// Option configures an App.
type Option func(*App)

// App is a CLI application instance.
type App struct {
	name        string
	shortDesc   string
	description string
	run         RunFunc
	cmd         *cobra.Command
	options     any
	slogOpts    *options.SlogOptions
	versionInfo *version.Info
	signals     []os.Signal

	shutdownTimeout time.Duration
	silence         bool
	noConfig        bool

	cfgFile           string
	v                 *viper.Viper
	configSearchPaths []string
	configName        string
	dirInHome         string
	envPrefix         string

	preRunHooks      []LifecycleHook
	afterStartHooks  []LifecycleHook
	beforeStopHooks  []LifecycleHook
	preShutdownHooks []LifecycleHook
}

// WithRun sets the startup callback.
func WithRun(run RunFunc) Option {
	return func(a *App) { a.run = run }
}

// WithOptions sets the options struct (FlagSetOptions or NamedFlagSetOptions).
func WithOptions(opts any) Option {
	return func(a *App) { a.options = opts }
}

// WithSlogOptions configures structured logging.
func WithSlogOptions(opts *options.SlogOptions) Option {
	return func(a *App) { a.slogOpts = opts }
}

// WithVersionInfo sets version info for cobra's --version.
func WithVersionInfo(info *version.Info) Option {
	return func(a *App) { a.versionInfo = info }
}

// WithDescription sets the long description.
func WithDescription(desc string) Option {
	return func(a *App) { a.description = desc }
}

// WithSignals overrides the shutdown signals.
func WithSignals(sigs ...os.Signal) Option {
	return func(a *App) { a.signals = sigs }
}

// WithShutdownTimeout sets the shutdown timeout.
func WithShutdownTimeout(d time.Duration) Option {
	return func(a *App) { a.shutdownTimeout = d }
}

// WithSilence suppresses startup info output.
func WithSilence() Option {
	return func(a *App) { a.silence = true }
}

// WithNoConfig disables config file loading.
func WithNoConfig() Option {
	return func(a *App) { a.noConfig = true }
}

// WithConfigSearchPaths sets config search directories.
func WithConfigSearchPaths(paths []string) Option {
	return func(a *App) { a.configSearchPaths = paths }
}

// WithConfigName overrides the config file name. Defaults to the app name.
func WithConfigName(name string) Option {
	return func(a *App) { a.configName = name }
}

// WithDirInHome sets the directory (relative to the user's home directory) in
// which the app searches for its config file. For example,
// WithDirInHome(".onexmesh") searches $HOME/.onexmesh. It overrides the default
// ".<app-name>" home directory.
func WithDirInHome(dir string) Option {
	return func(a *App) { a.dirInHome = dir }
}

// WithEnvPrefix overrides the environment variable prefix. Defaults to the
// app name upper-cased with "-" replaced by "_".
func WithEnvPrefix(prefix string) Option {
	return func(a *App) { a.envPrefix = prefix }
}

// WithPreRunHook adds a hook run before RunFunc.
func WithPreRunHook(hook LifecycleHook) Option {
	return func(a *App) { a.preRunHooks = append(a.preRunHooks, hook) }
}

// WithPreShutdownHook adds a hook always run after RunFunc.
func WithPreShutdownHook(hook LifecycleHook) Option {
	return func(a *App) { a.preShutdownHooks = append(a.preShutdownHooks, hook) }
}

// NewApp creates an App.
func NewApp(name, shortDesc string, opts ...Option) *App {
	a := &App{
		name:            name,
		shortDesc:       shortDesc,
		description:     shortDesc,
		shutdownTimeout: 10 * time.Second,
		signals:         []os.Signal{os.Interrupt, syscall.SIGTERM},
		envPrefix:       strings.ReplaceAll(strings.ToUpper(name), "-", "_"),
		v:               viper.New(),
		run:             func(ctx context.Context) error { return nil },
	}
	for _, o := range opts {
		o(a)
	}
	a.buildCommand()
	return a
}

func (a *App) buildCommand() {
	cmd := &cobra.Command{
		Use:           a.name,
		Short:         a.shortDesc,
		Long:          a.description,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return a.setupCommand(cmd, args)
		},
		RunE: a.runCommand,
	}

	if a.versionInfo != nil {
		cmd.Version = a.versionInfo.String()
		cmd.SetVersionTemplate(`{{.Version}}`)
	}

	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	// Register option flags.
	if fso, ok := a.options.(FlagSetOptions); ok {
		fso.AddFlags(cmd.Flags())
	}

	fs := cmd.PersistentFlags()
	if !a.noConfig {
		fs.StringVarP(&a.cfgFile, "config", "c", "", "Config file path.")
	}

	a.cmd = cmd
}

func (a *App) setupCommand(cmd *cobra.Command, args []string) error {
	if err := a.loadConfig(); err != nil {
		return err
	}
	_ = a.v.BindPFlags(cmd.Flags())
	_ = a.v.BindPFlags(cmd.PersistentFlags())

	if err := a.applyOptions(); err != nil {
		return err
	}

	if a.slogOpts != nil {
		if err := a.slogOpts.Apply(); err != nil {
			return fmt.Errorf("apply slog options: %w", err)
		}
	}

	if !a.silence {
		slog.Info("starting application", "name", a.name, "version", a.versionInfo)
	}
	return nil
}

func (a *App) runCommand(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(cmd.Context(), a.signals...)
	defer cancel()

	for _, hook := range a.preRunHooks {
		if err := hook(ctx); err != nil {
			return fmt.Errorf("pre-run hook: %w", err)
		}
	}

	runErr := a.run(ctx)

	// BeforeStop hooks run after the RunFunc returns but before shutdown cleanup,
	// the natural deregistration point.
	if len(a.beforeStopHooks) > 0 {
		stopCtx, sc := context.WithTimeout(context.Background(), a.shutdownTimeout)
		for _, hook := range a.beforeStopHooks {
			if err := hook(stopCtx); err != nil {
				slog.Error("before-stop hook failed", "err", err)
			}
		}
		sc()
	}

	if len(a.preShutdownHooks) > 0 {
		shutdownCtx, sc := context.WithTimeout(context.Background(), a.shutdownTimeout)
		for _, hook := range a.preShutdownHooks {
			if err := hook(shutdownCtx); err != nil {
				slog.Error("pre-shutdown hook failed", "err", err)
			}
		}
		sc()
	}
	return runErr
}

func (a *App) loadConfig() error {
	if a.noConfig {
		return nil
	}
	if a.cfgFile != "" {
		a.v.SetConfigFile(a.cfgFile)
	} else {
		for _, p := range a.effectiveConfigSearchPaths() {
			a.v.AddConfigPath(p)
		}
		a.v.SetConfigType("yaml")
		name := a.configName
		if name == "" {
			name = a.name
		}
		a.v.SetConfigName(name)
	}
	a.v.AutomaticEnv()
	a.v.SetEnvPrefix(a.envPrefix)
	a.v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	if err := a.v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("parse config: %w", err)
		}
	}
	return nil
}

func (a *App) applyOptions() error {
	if a.options == nil {
		return nil
	}
	if err := a.v.Unmarshal(a.options); err != nil {
		return fmt.Errorf("unmarshal options: %w", err)
	}
	if c, ok := a.options.(OptionsCompleter); ok {
		if err := c.Complete(); err != nil {
			return fmt.Errorf("complete options: %w", err)
		}
	}
	if v, ok := a.options.(OptionsValidator); ok {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("invalid options: %w", err)
		}
	}
	return nil
}

func (a *App) effectiveConfigSearchPaths() []string {
	if len(a.configSearchPaths) > 0 {
		return a.configSearchPaths
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	homeConfigDir := "." + a.name
	if a.dirInHome != "" {
		homeConfigDir = a.dirInHome
	}

	return []string{
		".",
		filepath.Join(homeDir, homeConfigDir),
		filepath.Join("/etc", a.name),
	}
}

// Run launches the app and exits on error.
func (a *App) Run() {
	if err := a.RunContext(context.Background()); err != nil {
		slog.Error("application exited with error", "err", err)
		os.Exit(1)
	}
}

// RunContext launches the app with a parent context.
func (a *App) RunContext(ctx context.Context) error {
	return a.cmd.ExecuteContext(ctx)
}

// Command returns the underlying cobra command.
func (a *App) Command() *cobra.Command {
	return a.cmd
}
