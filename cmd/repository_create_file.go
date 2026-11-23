package cmd

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/spf13/cobra"
	"onecx.tkit.org/ocp/internal/helm"
	"onecx.tkit.org/ocp/internal/manifest"
	"onecx.tkit.org/ocp/internal/products"
)

type repositoryCreateFileOptions struct {
	global     *globalOptions
	repository *sharedRepositoryOptions
	manifest   *sharedManifestOptions
	helmChart  *sharedHelmChartOptions
	createFile *sharedCreateFileOptions
}

func repositoryCreateFileCmd(global *globalOptions) *cobra.Command {
	sharedRepoFlags, sharedRepoOpts := sharedRepositoryFlags()
	sharedManifestFlags, sharedManifestOpts := sharedManifestFlags()
	sharedHelmChartFlags, sharedHelmChartOpts := sharedHelmChartFlags()
	sharedCreateFileFlags, sharedCreateFileOpts := sharedCreateFileFlags()
	opts := repositoryCreateFileOptions{
		global:     global,
		createFile: sharedCreateFileOpts,
		repository: sharedRepoOpts,
		manifest:   sharedManifestOpts,
		helmChart:  sharedHelmChartOpts,
	}
	cmd := &cobra.Command{
		Use:   "creates",
		Short: "Creates repository from the YAML create-file",
		RunE:  commandAction(opts.run),
	}
	flags := cmd.Flags()
	flags.AddFlagSet(&sharedRepoFlags)
	flags.AddFlagSet(&sharedManifestFlags)
	flags.AddFlagSet(&sharedHelmChartFlags)
	flags.AddFlagSet(&sharedCreateFileFlags)
	return cmd
}

func (opts *repositoryCreateFileOptions) run(args []string, stdout io.Writer) (retErr error) {
	if err := opts.repository.createRepositoryDir(); err != nil {
		return err
	}

	products, err := products.LoadCreateFile(opts.createFile.file)
	if err != nil {
		return err
	}

	for _, product := range products {
		// Implement the logic to create repository for each product here
		slog.Info("Creating repository for product", "registry", product.Registry, "organization", product.Organization, "product", product.Product)
		if err := opts.createRepositoryForProduct(product); err != nil {
			return err
		}
	}
	return nil
}

func (opts *repositoryCreateFileOptions) createRepositoryForProduct(product products.ProductCreate) error {
	repositoryUrl, err := opts.repository.repositoryURL(product.Registry, product.Organization, product.Product)
	if err != nil {
		return fmt.Errorf("get repository URL: %w", err)
	}
	versions, err := helm.GetVersions(repositoryUrl)
	if err != nil {
		return fmt.Errorf("get chart versions: %w", err)
	}

	if len(versions) > product.Count {
		versions = versions[:product.Count]
	}

	for _, v := range versions {

		manifestFile := opts.repository.repositoryManifestFile(product.Registry, product.Organization, product.Product, v)

		exists, err := opts.repository.repositoryExistsManifest(manifestFile)
		if err != nil {
			return fmt.Errorf("check manifest file in repository: %w", err)
		}
		if exists {
			continue
		}
		slog.Info("Processing version", "registry", product.Registry, "organization", product.Organization, "product", product.Product, "version", v)
		url, err := opts.helmChart.helmChartUrl(product.Registry, product.Organization, product.Product, v)
		if err != nil {
			return err
		}

		cfg := &manifest.Config{
			Organization: product.Organization,
			Product:      product.Product,
			Version:      v,
			ExcludeApps:  opts.manifest.skipDependency,
			Url:          url,
			HelmValues:   opts.manifest.helmValues,
		}
		m, err := manifest.CreateManifest(cfg)
		if err != nil {
			return fmt.Errorf("download manifest: %w", err)
		}
		if err := opts.repository.repositoryWriteManifestFile(product.Registry, m); err != nil {
			return fmt.Errorf("write manifest file: %w", err)
		}
	}
	return nil
}
