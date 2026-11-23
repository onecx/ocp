package cmd

import (
	"io"
	"os"

	"github.com/spf13/cobra"
	"onecx.tkit.org/ocp/internal/manifest"
	"onecx.tkit.org/ocp/internal/products"
	"onecx.tkit.org/ocp/internal/template"
)

type reportOptions struct {
	global       *globalOptions
	repository   *sharedRepositoryOptions
	output       *sharedOutputOptions
	productsFile *sharedProductsFileOptions
	template     string
}

func reportCmd(global *globalOptions) *cobra.Command {
	sharedOutputFlags, sharedOutputOpts := sharedOutputFlags()
	sharedRepositoryFlags, sharedRepositoryOpts := sharedRepositoryFlags()
	sharedProductsFileFlags, sharedProductsFileOpts := sharedProductsFileFlags()
	opts := reportOptions{
		global:       global,
		repository:   sharedRepositoryOpts,
		productsFile: sharedProductsFileOpts,
		template:     "",
		output:       sharedOutputOpts,
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
	flags.AddFlagSet(&sharedOutputFlags)
	flags.AddFlagSet(&sharedRepositoryFlags)
	flags.AddFlagSet(&sharedProductsFileFlags)
	return cmd
}

func (opts *reportOptions) run(args []string, stdout io.Writer) (retErr error) {

	products, err := products.LoadProductsFile(opts.productsFile.file)
	if err != nil {
		return err
	}

	var manifests []*manifest.Manifest
	for _, product := range products {
		m, err := repositoryLoadManifest(opts.repository, product.Registry, product.Organization, product.Product, product.Version)
		if err != nil {
			return err
		}
		manifests = append(manifests, m)
	}

	data, err := os.ReadFile(opts.template)
	if err != nil {
		return err
	}

	report := struct {
		Manifests []*manifest.Manifest
	}{
		Manifests: manifests,
	}

	tmp, err := template.Print(string(data), report)
	if err != nil {
		return err
	}

	return opts.output.write([]byte(tmp), stdout)
}
