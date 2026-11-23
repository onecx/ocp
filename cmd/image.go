package cmd

import "github.com/spf13/cobra"

func imageCmd(global *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Image related commands",
	}
	cmd.AddCommand(
		imageListCmd(global),
	)
	return cmd
}
