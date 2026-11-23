package cmd

import "github.com/spf13/cobra"

func helmCmd(global *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "helm",
		Short: "Helm chart related commands",
	}
	cmd.AddCommand(
		helmChartCmd(global),
		helmListCmd(global),
	)
	return cmd
}
