package cmd

import (
	"io"

	"github.com/spf13/cobra"
)

type repositoryCreateOptions struct {
	global     *globalOptions
	product    *sharedProductVersionOptions
	manifest   *sharedManifestOptions
	helmChart  *sharedHelmChartOptions
	repository *sharedRepositoryOptions
}

func repositoryCreateCmd(global *globalOptions) *cobra.Command {
	sharedProductFlags, sharedProductOpts := sharedProductVersionFlags()
	sharedRepoFlags, sharedRepoOpts := sharedRepositoryFlags()
	sharedManifestFlags, sharedManifestOpts := sharedManifestFlags()
	sharedHelmChartFlags, sharedHelmChartOpts := sharedHelmChartFlags()
	opts := repositoryCreateOptions{
		global:     global,
		product:    sharedProductOpts,
		manifest:   sharedManifestOpts,
		helmChart:  sharedHelmChartOpts,
		repository: sharedRepoOpts,
	}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a repository for the current product version",
		RunE:  commandAction(opts.run),
	}
	flags := cmd.Flags()
	flags.AddFlagSet(&sharedProductFlags)
	flags.AddFlagSet(&sharedRepoFlags)
	flags.AddFlagSet(&sharedManifestFlags)
	flags.AddFlagSet(&sharedHelmChartFlags)
	return cmd
}

func (opts *repositoryCreateOptions) run(args []string, stdout io.Writer) (retErr error) {
	if err := opts.repository.createRepositoryDir(); err != nil {
		return err
	}
	return createManifest(opts.repository, opts.helmChart, opts.manifest, opts.product.registry, opts.product.organization, opts.product.product, opts.product.version)
}
