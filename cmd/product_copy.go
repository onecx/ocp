package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"onecx.tkit.org/ocp/internal/products"
)

type productCopyOptions struct {
	global       *globalOptions
	repository   *sharedRepositoryOptions
	source       *sharedRepositoryOptions
	productsFile *sharedProductsFileOptions
}

func productCopyCmd(global *globalOptions) *cobra.Command {

	sharedRepositoryFlags, sharedRepositoryOpts := sharedRepositoryFlags()
	sharedSourceRepositoryFlags, sharedSourceRepositoryOpts := sharedRepositoryFlagsWithPrefix("source-")
	sharedProductsFileFlags, sharedProductsFileOpts := sharedProductsFileFlags()
	opts := productCopyOptions{
		global:       global,
		repository:   sharedRepositoryOpts,
		source:       sharedSourceRepositoryOpts,
		productsFile: sharedProductsFileOpts,
	}
	cmd := &cobra.Command{
		Use:   "copy",
		Short: "Copy the list of products' versions from source to target repository",
		Long: `Copy the manifest files of the products listed in the products file from the source
repository (--source-repository) to the target repository (--repository).

The manifests are copied byte for byte and the target directories are created as needed.
A manifest that already exists in the target repository is skipped. A manifest that is
missing in the source repository is reported and skipped.

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
		Example: `  # copy the manifests listed in products.yaml from one repository to another
  ocp product copy --products-file products.yaml \
    --source-repository /path/to/source --repository /path/to/target

  # copy from the default repository (~/.ocp/repository) to a shared directory
  ocp product copy --products-file products.yaml --repository /srv/ocp`,
		RunE: commandAction(opts.run),
	}
	flags := cmd.Flags()
	flags.AddFlagSet(&sharedRepositoryFlags)
	flags.AddFlagSet(&sharedSourceRepositoryFlags)
	flags.AddFlagSet(&sharedProductsFileFlags)
	return cmd
}

func (opts *productCopyOptions) run(args []string, stdout io.Writer) (retErr error) {

	products, err := products.LoadProductsFile(opts.productsFile.file)
	if err != nil {
		return err
	}

	for _, product := range products {
		targetManifestFile := opts.repository.repositoryManifestFile(product.Registry, product.Organization, product.Product, product.Version)
		exists, err := opts.repository.repositoryExistsManifest(targetManifestFile)
		if err != nil {
			return fmt.Errorf("check manifest file in repository: %w", err)
		}
		if exists {
			continue
		}

		sourceManifestFile := opts.source.repositoryManifestFile(product.Registry, product.Organization, product.Product, product.Version)
		exists, err = opts.source.repositoryExistsManifest(sourceManifestFile)
		if err != nil {
			return fmt.Errorf("check manifest file in source repository: %w", err)
		}
		if !exists {
			if _, err := fmt.Fprintf(stdout, "Manifest file does not exist in source repository: %s\n", sourceManifestFile); err != nil {
				return err
			}
			continue
		}

		data, err := os.ReadFile(sourceManifestFile)
		if err != nil {
			return fmt.Errorf("read source manifest %s: %w", sourceManifestFile, err)
		}
		if err := os.MkdirAll(filepath.Dir(targetManifestFile), 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", filepath.Dir(targetManifestFile), err)
		}
		if err := os.WriteFile(targetManifestFile, data, 0o644); err != nil {
			return fmt.Errorf("write manifest %s: %w", targetManifestFile, err)
		}
	}
	return nil
}
