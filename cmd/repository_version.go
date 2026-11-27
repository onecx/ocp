package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"onecx.tkit.org/ocp/internal/helm"
	"onecx.tkit.org/ocp/internal/manifest"
)

type repositoryVersionsOptions struct {
	global     *globalOptions
	product    *sharedProductOptions
	repository *sharedRepositoryOptions
	manifest   *sharedManifestOptions
	helmChart  *sharedHelmChartOptions
	latest     int
}

func repositoryVersionsCmd(global *globalOptions) *cobra.Command {
	sharedFlags, sharedOpts := sharedProductFlags()
	sharedRepoFlags, sharedRepoOpts := sharedRepositoryFlags()
	sharedManifestFlags, sharedManifestOpts := sharedManifestFlags()
	sharedHelmChartFlags, sharedHelmChartOpts := sharedHelmChartFlags()
	opts := repositoryVersionsOptions{
		global:     global,
		product:    sharedOpts,
		repository: sharedRepoOpts,
		manifest:   sharedManifestOpts,
		helmChart:  sharedHelmChartOpts,
	}
	cmd := &cobra.Command{
		Use:   "versions",
		Short: "Create last n-versions of the current product",
		Long: `Create the manifests of the latest versions of one product.

The chart versions are read from the registry and the manifests of the first --latest versions
(the order returned by the registry, default 20) are created. Versions that already exist in
the repository are skipped.`,
		Example: `  ocp repository versions --registry oci://ghcr.io --organization onecx --product onecx-test

  # only the latest 5 versions
  ocp repository versions --registry oci://ghcr.io --organization onecx \
    --product onecx-test --latest 5`,
		RunE: commandAction(opts.run),
	}
	flags := cmd.Flags()
	flags.AddFlagSet(&sharedFlags)
	flags.AddFlagSet(&sharedRepoFlags)
	flags.AddFlagSet(&sharedManifestFlags)
	flags.AddFlagSet(&sharedHelmChartFlags)
	flags.IntVar(&opts.latest, "latest", 20, "Number of last chart versions to store in the repository")
	return cmd
}

func (opts *repositoryVersionsOptions) run(args []string, stdout io.Writer) (retErr error) {
	if err := opts.repository.createRepositoryDir(); err != nil {
		return err
	}

	repositoryUrl, err := opts.repository.repositoryURL(opts.product.registry, opts.product.organization, opts.product.product)
	if err != nil {
		return fmt.Errorf("get repository URL: %w", err)
	}
	versions, err := helm.GetVersions(repositoryUrl)
	if err != nil {
		return fmt.Errorf("get chart versions: %w", err)
	}

	if len(versions) > opts.latest {
		versions = versions[:opts.latest]
	}

	for _, v := range versions {

		manifestFile := opts.repository.repositoryManifestFile(opts.product.registry, opts.product.organization, opts.product.product, v)

		exists, err := opts.repository.repositoryExistsManifest(manifestFile)
		if err != nil {
			return fmt.Errorf("check manifest file in repository: %w", err)
		}
		if exists {
			continue
		}

		url, err := opts.helmChart.helmChartUrl(opts.product.registry, opts.product.organization, opts.product.product, v)
		if err != nil {
			return err
		}

		cfg := &manifest.Config{
			Organization: opts.product.organization,
			Product:      opts.product.product,
			Version:      v,
			ExcludeApps:  opts.manifest.skipDependency,
			Url:          url,
			HelmValues:   opts.manifest.helmValues,
		}
		m, err := manifest.CreateManifest(cfg)
		if err != nil {
			return fmt.Errorf("download manifest: %w", err)
		}
		if err := opts.repository.repositoryWriteManifestFile(opts.product.registry, m); err != nil {
			return fmt.Errorf("write manifest file: %w", err)
		}
	}
	return nil
}
