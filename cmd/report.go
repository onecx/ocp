package cmd

import (
	"io"
	"os"

	"github.com/spf13/cobra"
	"onecx.tkit.org/ocp/internal/manifest"
	"onecx.tkit.org/ocp/internal/template"
)

type reportOptions struct {
	global   *globalOptions
	output   *sharedOutputOptions
	manifest []string
	template string
}

func reportCmd(global *globalOptions) *cobra.Command {
	sharedOutputFlags, sharedOutputOpts := sharedOutputFlags()
	opts := reportOptions{
		global:   global,
		manifest: []string{},
		template: "",
		output:   sharedOutputOpts,
	}
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Display the report for the current product version",
		RunE:  commandAction(opts.run),
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.template, "template", "t", "", "Path to the template file")
	if err := cmd.MarkFlagRequired("template"); err != nil {
		panic(err)
	}
	flags.StringSliceVar(&opts.manifest, "manifest", []string{}, "Paths to the manifest files")
	if err := cmd.MarkFlagRequired("manifest"); err != nil {
		panic(err)
	}
	flags.AddFlagSet(&sharedOutputFlags)
	return cmd
}

func (opts *reportOptions) run(args []string, stdout io.Writer) (retErr error) {
	manifests, err := getManifests(opts.manifest)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(opts.template)
	if err != nil {
		return err
	}

	report := Report{
		Manifests: manifests,
	}

	tmp, err := template.Print(string(data), report)
	if err != nil {
		return err
	}

	return write(opts.output.file, []byte(tmp), stdout)
}

type Report struct {
	Manifests []*manifest.Manifest
}
