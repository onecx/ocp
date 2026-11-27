package cmd

import (
	"github.com/spf13/cobra"
)

func productCmd(global *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "product",
		Short: "Product related commands",
		Long:  "Commands working on the products listed in a products file, using manifests from the repository.",
	}
	cmd.AddCommand(
		productReportCmd(global),
		productCopyCmd(global),
	)
	return cmd
}
