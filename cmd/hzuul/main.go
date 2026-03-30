package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/vcastell/hzuul/internal/config"
	"github.com/vcastell/hzuul/internal/tui"
)

var (
	version = "dev"
	cfgFile string
	debug   bool
)

func main() {
	root := &cobra.Command{
		Use:     "hzuul",
		Short:   "Terminal UI for Zuul CI/CD",
		Long:    "HZUUL is a terminal user interface for monitoring and managing Zuul CI/CD pipelines, builds, and jobs.",
		Version: version,
		RunE:    run,
	}

	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/hzuul/config.yaml)")
	root.PersistentFlags().String("context", "", "use a specific context from config")
	root.PersistentFlags().BoolVar(&debug, "debug", false, "enable verbose debug logging")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	level := slog.LevelWarn
	if debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	ctxName, _ := cmd.Flags().GetString("context")
	if ctxName != "" {
		cfg.CurrentContext = ctxName
	}

	app, err := tui.New(cfg)
	if err != nil {
		return fmt.Errorf("initializing TUI: %w", err)
	}

	return app.Run()
}
