package cmd

import (
	"io"

	"github.com/spf13/cobra"
)

type manifestOptions struct {
	global  *globalOptions
	product *sharedProductOptions
	output  *sharedOutputOptions
}

func manifestCmd(global *globalOptions) *cobra.Command {
	sharedFlags, sharedOpts := sharedProductFlags()
	sharedOutputFlags, sharedOutputOpts := sharedOutputFlags()
	opts := manifestOptions{
		global:  global,
		product: sharedOpts,
		output:  sharedOutputOpts,
	}
	cmd := &cobra.Command{
		Use:   "manifest",
		Short: "Display the manifest for the current product version",
		RunE:  commandAction(opts.run),
	}
	flags := cmd.Flags()
	flags.AddFlagSet(&sharedFlags)
	flags.StringVarP(&sharedOpts.manifest, "manifest", "m", "", "Path to the manifest file")
	flags.AddFlagSet(&sharedOutputFlags)
	markProductFlagsRequiredUnlessManifest(cmd)
	return cmd
}

func (opts *manifestOptions) run(args []string, stdout io.Writer) (retErr error) {
	manifest, err := getManifest(opts.product)
	if err != nil {
		return err
	}
	return writeOutput(opts.output, stdout, *manifest)
}
