package cmd

import (
	"github.com/spf13/cobra"
)

func repositoryCmd(global *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repository",
		Short: "Repository related commands",
	}
	cmd.AddCommand(
		repositoryCreateCmd(global),
		repositoryScanCmd(global),
		repositoryManifestCmd(global),
		repositoryProductsCmd(global),
	)
	return cmd
}
