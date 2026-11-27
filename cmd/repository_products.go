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
		Long: `Create the manifests of all product versions listed in the products file.

Every product version is handled like "ocp repository create". Manifests that already exist in
the repository are skipped, so the command can be repeated to pick up newly added versions.

The products file is a YAML file with optional defaults that every product inherits:

  defaults:
    registry: oci://ghcr.io
    organization: onecx
  products:
    - product: onecx-test
      version: 1.14.0
    - product: onecx-workspace
      version: 2.0.0
    - product: onecx-test
      version: 1.13.0
      organization: other-org   # overrides the default

Every product needs registry, organization, product and version, either set directly or
inherited from defaults. Duplicate products are rejected.`,
		Example: `  ocp repository products --products-file products.yaml

  # into a different repository directory, without the default dependency exclusion
  ocp repository products --products-file products.yaml \
    --repository /srv/ocp --skip-dependency ""`,
		RunE: commandAction(opts.run),
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
