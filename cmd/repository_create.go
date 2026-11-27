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
		Long: `Create the manifest of one product version in the repository.

The Helm chart <registry>/<organization>/charts/<product>:<version> is downloaded (see
--chart-url-template), rendered and every container image it references is resolved in its
registry (digest, architectures, labels). The result is stored as
<repository>/<registry>/<organization>/<product>/<version>.yaml.

If the manifest already exists nothing is downloaded. Dependencies listed in --skip-dependency
(default helm-product) are not part of the manifest. Extra Helm values for the rendering can be
set with --helm-value.`,
		Example: `  ocp repository create --registry oci://ghcr.io --organization onecx \
    --product onecx-test --version 1.14.0

  # into a different repository directory, with extra Helm values
  ocp repository create --registry oci://ghcr.io --organization onecx \
    --product onecx-test --version 1.14.0 \
    --repository /srv/ocp --helm-value global.foo=bar,global.baz=1

  # chart stored under a different path in the registry
  ocp repository create --registry oci://registry.example.com --organization team \
    --product my-product --version 1.0.0 \
    --chart-url-template "{{.Registry}}/{{.Organization}}/{{.Product}}:{{.Version}}"`,
		RunE: commandAction(opts.run),
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
