package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

type globalOptions struct {
	debug bool // Enable debug output
}

func Main(bv BuildVersion) {
	rootCmd, _ := createApp(bv)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func createApp(bv BuildVersion) (*cobra.Command, *globalOptions) {
	opts := globalOptions{}
	rootCommand := &cobra.Command{
		Use:               "ocp",
		Short:             "Copy onecx products aretefacts from source registry to target registry.",
		TraverseChildren:  true,
		PersistentPreRunE: opts.before,
		CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Version will output the current build information",
		Long:  ``,
		Run: func(_ *cobra.Command, _ []string) {
			resp := fmt.Sprintf("Version: %s, Commit: %s, Date: %s\n", bv.Version, bv.Commit, bv.Date)
			fmt.Print(resp)
		},
	}
	rootCommand.PersistentFlags().BoolVar(&opts.debug, "debug", false, "enable debug output")
	rootCommand.AddCommand(
		productCmd(&opts),
		repositoryCmd(&opts),
		versionCmd,
	)
	return rootCommand, &opts
}

func (opts *globalOptions) before(cmd *cobra.Command, args []string) error {
	if opts.debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	return nil
}
