package cmd

import (
	"io"

	"github.com/spf13/cobra"
)

type repositoryManifestOptions struct {
	global     *globalOptions
	product    *sharedProductVersionOptions
	repository *sharedRepositoryOptions
	output     *sharedOutputOptions
}

func repositoryManifestCmd(global *globalOptions) *cobra.Command {
	sharedFlags, sharedOpts := sharedProductVersionFlags()
	sharedOutputFlags, sharedOutputOpts := sharedOutputFlags()
	sharedRepositoryFlags, sharedRepositoryOpts := sharedRepositoryFlags() // Add this line to get repository flags and options
	opts := repositoryManifestOptions{
		global:     global,
		product:    sharedOpts,
		output:     sharedOutputOpts,
		repository: sharedRepositoryOpts, // Add this line to set repository options
	}
	cmd := &cobra.Command{
		Use:   "manifest",
		Short: "Display the manifest for the current product version",
		Long: `Print the manifest of one product version from the repository.

The manifest must exist in the repository (see "ocp repository create"). The output format
is yaml (default), json or plain, the output goes to stdout or to --output.`,
		Example: `  ocp repository manifest --registry oci://ghcr.io --organization onecx \
    --product onecx-test --version 1.14.0

  # as JSON into a file
  ocp repository manifest --registry oci://ghcr.io --organization onecx \
    --product onecx-test --version 1.14.0 -f json -o manifest.json`,
		RunE: commandAction(opts.run),
	}
	flags := cmd.Flags()
	flags.AddFlagSet(&sharedFlags)
	flags.AddFlagSet(&sharedOutputFlags)
	flags.AddFlagSet(&sharedRepositoryFlags)
	return cmd
}

func (opts *repositoryManifestOptions) run(args []string, stdout io.Writer) (retErr error) {
	m, err := repositoryLoadManifest(opts.repository, opts.product.registry, opts.product.organization, opts.product.product, opts.product.version)
	if err != nil {
		return err
	}
	return opts.output.writeOutput(stdout, *m)
}
