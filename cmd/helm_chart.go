package cmd

import (
	"io"

	"github.com/spf13/cobra"
	v2 "helm.sh/helm/v4/pkg/chart/v2"
	"onecx.tkit.org/ocp/internal/helm"
)

type helmChartOptions struct {
	global  *globalOptions
	product *sharedProductOptions
	output  *sharedOutputOptions
}

func helmChartCmd(global *globalOptions) *cobra.Command {
	sharedFlags, sharedOpts := sharedProductFlags()
	sharedOutputFlags, sharedOutputOpts := sharedOutputFlags()
	opts := helmChartOptions{
		global:  global,
		product: sharedOpts,
		output:  sharedOutputOpts,
	}
	cmd := &cobra.Command{
		Use:   "chart",
		Short: "Show the Helm chart for the current product version",
		RunE:  commandAction(opts.run),
	}
	flags := cmd.Flags()
	flags.AddFlagSet(&sharedFlags)
	flags.AddFlagSet(&sharedOutputFlags)
	markProductFlagsRequired(cmd)
	return cmd
}

func (opts *helmChartOptions) run(args []string, stdout io.Writer) (retErr error) {
	url, err := chartURL(opts.product)
	if err != nil {
		return err
	}
	chrt, _, err := helm.GetChart(url)
	if err != nil {
		return err
	}
	info := HelmInfo{
		Organization: opts.product.organization,
		Product:      opts.product.product,
		Version:      opts.product.version,
		Lock:         chrt.Lock,
		Chart:        chrt.Metadata,
	}
	return writeOutput(opts.output, stdout, info)
}

type HelmInfo struct {
	Organization string       `json:"organization" yaml:"organization"`
	Product      string       `json:"product" yaml:"product"`
	Version      string       `json:"version" yaml:"version"`
	Chart        *v2.Metadata `json:"chart" yaml:"chart"`
	Lock         *v2.Lock     `json:"lock" yaml:"lock"`
}
