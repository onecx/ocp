package cmd

import (
	"io"
	"os"

	"github.com/spf13/cobra"
	"onecx.tkit.org/ocp/internal/manifest"
	"onecx.tkit.org/ocp/internal/products"
	"onecx.tkit.org/ocp/internal/template"
)

type productReportOptions struct {
	global       *globalOptions
	repository   *sharedRepositoryOptions
	output       *sharedOutputOptions
	productsFile *sharedProductsFileOptions
	template     string
}

func productReportCmd(global *globalOptions) *cobra.Command {
	sharedOutputFlags, sharedOutputOpts := sharedOutputFlags()
	sharedRepositoryFlags, sharedRepositoryOpts := sharedRepositoryFlags()
	sharedProductsFileFlags, sharedProductsFileOpts := sharedProductsFileFlags()
	opts := productReportOptions{
		global:       global,
		repository:   sharedRepositoryOpts,
		productsFile: sharedProductsFileOpts,
		template:     "",
		output:       sharedOutputOpts,
	}
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Display the report for the current product version",
		Long: `Render a Go text/template with the manifests of the products listed in the products file.

The manifests must exist in the repository (see "ocp repository products"). The template
receives the list of manifests in .Manifests; every manifest has the fields of the manifest
file (Organization, Product, Version, Digest, Applications, ...), where Applications maps the
application name to its Images (Image, Digest, Type, Kind, Arch, Key, ...).

Available functions are all sprig functions (https://masterminds.github.io/sprig/) and:

  prefer <images> <attribute> <value>...
      Keep, for every image available in several variants, only the variant whose attribute
      value comes first in the list of values. The native image (tag with the suffix "-native")
      is linked with the jvm image (same tag without the suffix). Images with other attribute
      values are kept. The attribute is the Go field name of the image, e.g. "Type".

The rendered text is written to stdout or to --output. The --format flag is not used, the
template defines the format.

The products file is a YAML file with optional defaults that every product inherits:

  defaults:
    registry: oci://ghcr.io
    organization: onecx
  products:
    - product: onecx-test
      version: 1.14.0
    - product: onecx-workspace
      version: 2.0.0
    - product: onecx-test
      version: 1.13.0
      organization: other-org   # overrides the default

Every product needs registry, organization, product and version, either set directly or
inherited from defaults. Duplicate products are rejected.`,
		Example: `  # report.tpl
  {{- range .Manifests }}
  {{ .Product }} {{ .Version }}
  {{- range $name, $app := .Applications }}
    {{ $name }}
  {{- range prefer $app.Images "Type" "native" "jvm" }}
      {{ .Image }}@{{ .Digest }}
  {{- end }}
  {{- end }}
  {{- end }}

  # print the report, native images preferred, jvm as fallback
  ocp product report --products-file products.yaml --template report.tpl

  # write the report to a file
  ocp product report --products-file products.yaml -t report.tpl -o report.txt`,
		RunE: commandAction(opts.run),
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

func (opts *productReportOptions) run(args []string, stdout io.Writer) (retErr error) {

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
