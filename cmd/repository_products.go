package cmd

import (
	"io"

	"github.com/spf13/cobra"
	"onecx.tkit.org/ocp/internal/products"
)

type repositoryProductsOptions struct {
	global       *globalOptions
	productsFile *sharedProductsFileOptions
	manifest     *sharedManifestOptions
	helmChart    *sharedHelmChartOptions
	repository   *sharedRepositoryOptions
}

func repositoryProductsCmd(global *globalOptions) *cobra.Command {
	sharedRepoFlags, sharedRepoOpts := sharedRepositoryFlags()
	sharedManifestFlags, sharedManifestOpts := sharedManifestFlags()
	sharedHelmChartFlags, sharedHelmChartOpts := sharedHelmChartFlags()
	sharedProductsFileFlags, sharedProductsFileOpts := sharedProductsFileFlags()
	opts := repositoryProductsOptions{
		global: global,

		productsFile: sharedProductsFileOpts,
		manifest:     sharedManifestOpts,
		helmChart:    sharedHelmChartOpts,
		repository:   sharedRepoOpts,
	}
	cmd := &cobra.Command{
		Use:   "products",
		Short: "Create a repository for the current product file",
		RunE:  commandAction(opts.run),
	}
	flags := cmd.Flags()
	flags.AddFlagSet(&sharedRepoFlags)
	flags.AddFlagSet(&sharedManifestFlags)
	flags.AddFlagSet(&sharedHelmChartFlags)
	flags.AddFlagSet(&sharedProductsFileFlags)
	return cmd
}

func (opts *repositoryProductsOptions) run(args []string, stdout io.Writer) (retErr error) {
	if err := opts.repository.createRepositoryDir(); err != nil {
		return err
	}

	products, err := products.LoadProductsFile(opts.productsFile.file)
	if err != nil {
		return err
	}

	for _, product := range products {
		if err := createManifest(opts.repository, opts.helmChart, opts.manifest, product.Registry, product.Organization, product.Product, product.Version); err != nil {
			return err
		}
	}
	return nil
}
