package cmd

import (
	"github.com/spf13/cobra"
)

func repositoryCmd(global *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repository",
		Short: "Repository related commands",
		Long: `Commands to build and maintain the local manifest repository.

A manifest describes one product version: its applications and their container images
(digests, types, architectures). Manifests are created from the product's Helm chart and are
stored as files in the repository directory (default ~/.ocp/repository, see --repository):

  <repository>/<registry without oci://>/<organization>/<product>/<version>.yaml

  e.g. ~/.ocp/repository/ghcr.io/onecx/onecx-test/1.14.0.yaml

Manifest creation is incremental: a manifest that already exists in the repository is never
downloaded again.`,
	}
	cmd.AddCommand(
		repositoryCreateCmd(global),
		repositoryVersionsCmd(global),
		repositoryManifestCmd(global),
		repositoryProductsCmd(global),
		repositoryCreateFileCmd(global),
		repositoryHousekeepingCmd(global),
	)
	return cmd
}
