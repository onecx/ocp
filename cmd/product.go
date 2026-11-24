package cmd

import (
	"github.com/spf13/cobra"
)

func productCmd(global *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "product",
		Short: "Product related commands",
	}
	cmd.AddCommand(
		productReportCmd(global),
		productCopyCmd(global),
	)
	return cmd
}
