package cmd

import (
	"io"
	"slices"

	"github.com/spf13/cobra"
	"onecx.tkit.org/ocp/internal/manifest"
)

type imageListOptions struct {
	global  *globalOptions
	product *sharedProductOptions
	output  *sharedOutputOptions
	filter  *sharedFilterOptions
	digest  bool
}

func imageListCmd(global *globalOptions) *cobra.Command {
	sharedFlags, sharedOpts := sharedProductFlags()
	sharedOutputFlags, sharedOutputOpts := sharedOutputFlags()
	sharedFilterFlags, sharedFilterOpts := sharedFilterFlags()
	opts := imageListOptions{
		global:  global,
		product: sharedOpts,
		output:  sharedOutputOpts,
		filter:  sharedFilterOpts,
	}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show the Docker image list for the current product version",
		Example: `  ocp image list -m manifest.yaml --type jvm
  ocp image list -m manifest.yaml --type native --kind deployment
  ocp image list -m manifest.yaml --arch arm64 --arch-only
  ocp image list -m manifest.yaml --key containers,initContainers`,
		RunE: commandAction(opts.run),
	}
	flags := cmd.Flags()
	flags.AddFlagSet(&sharedFlags)
	flags.StringVarP(&sharedOpts.manifest, "manifest", "m", "", "Path to the manifest file")
	flags.AddFlagSet(&sharedOutputFlags)
	flags.AddFlagSet(&sharedFilterFlags)
	flags.BoolVar(&opts.digest, "digest", false, "Add digests to image names")
	markProductFlagsRequiredUnlessManifest(cmd)
	return cmd
}

func (opts *imageListOptions) run(args []string, stdout io.Writer) (retErr error) {
	mf, err := getManifest(opts.product)
	if err != nil {
		return err
	}

	tmp := map[string]struct{}{}
	for appName, app := range mf.Applications {

		var matched []*manifest.Image
		for _, img := range app.Images {
			if opts.filter.match(appName, img) {
				matched = append(matched, img)
			}
		}
		if opts.filter.prefer != "" {
			matched = preferType(matched, opts.filter.prefer)
		}
		for _, img := range matched {
			key := img.Image
			if opts.digest && img.Digest != "" {
				key += "@" + img.Digest
			}
			tmp[key] = struct{}{}
		}
	}

	items := []string{}
	for i := range tmp {
		items = append(items, i)
	}
	slices.Sort(items)
	return writeOutput(opts.output, stdout, items)
}
