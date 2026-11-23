package cmd

import (
	"io"
	"slices"

	"github.com/spf13/cobra"
)

type helmListOptions struct {
	global  *globalOptions
	product *sharedProductOptions
	output  *sharedOutputOptions
	all     bool
}

func helmListCmd(global *globalOptions) *cobra.Command {
	sharedFlags, sharedOpts := sharedProductFlags()
	sharedOutputFlags, sharedOutputOpts := sharedOutputFlags()
	opts := helmListOptions{
		global:  global,
		product: sharedOpts,
		output:  sharedOutputOpts,
	}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show the Helm list for the current product version",
		RunE:  commandAction(opts.run),
	}
	flags := cmd.Flags()
	flags.AddFlagSet(&sharedFlags)
	flags.StringVarP(&sharedOpts.manifest, "manifest", "m", "", "Path to the manifest file")
	flags.AddFlagSet(&sharedOutputFlags)
	flags.BoolVar(&opts.all, "all", false, "Show all helm chart including the dependencies, not just the main ones")
	markProductFlagsRequiredUnlessManifest(cmd)
	return cmd
}

func (opts *helmListOptions) run(args []string, stdout io.Writer) (retErr error) {
	mf, err := getManifest(opts.product)
	if err != nil {
		return err
	}

	tmp := map[string]struct{}{}
	tmp[mf.Helm] = struct{}{}
	if opts.all {
		for _, app := range mf.Applications {
			tmp[app.Helm] = struct{}{}
		}
	}

	items := []string{}
	for i := range tmp {
		items = append(items, i)
	}
	slices.Sort(items)
	return writeOutput(opts.output, stdout, items)
}
