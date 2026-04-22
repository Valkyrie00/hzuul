package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/Valkyrie00/hzuul/internal/config"
	"github.com/Valkyrie00/hzuul/internal/tui"
	"github.com/Valkyrie00/hzuul/internal/updater"
	"github.com/spf13/cobra"
)

var (
	version     = "dev"
	cfgFile     string
	debug       bool
	selfUpdate  bool
	showVersion bool
)

func main() {
	root := &cobra.Command{
		Use:          "hzuul",
		Short:        "Terminal UI for Zuul CI/CD",
		Long:         "HZUUL is a terminal user interface for monitoring and managing Zuul CI/CD pipelines, builds, and jobs.",
		RunE:         run,
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.hzuul/config.yaml)")
	root.PersistentFlags().String("context", "", "use a specific context from config")
	root.PersistentFlags().BoolVar(&debug, "debug", false, "enable verbose debug logging")
	root.Flags().BoolVarP(&selfUpdate, "selfupdate", "U", false, "update hzuul to the latest version")
	root.Flags().BoolVarP(&showVersion, "version", "v", false, "print version and check for updates")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if showVersion {
		fmt.Printf("hzuul version %s\n", version)
		res, err := updater.Check(version)
		if err == nil && res.Available {
			fmt.Printf("\nA new version is available: %s → %s\nRun 'hzuul -U' to update.\n", res.Current, res.Latest)
		}
		return nil
	}

	if selfUpdate {
		return updater.SelfUpdate(version)
	}

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

	app, err := tui.New(cfg, version)
	if err != nil {
		return err
	}

	return app.Run()
}
