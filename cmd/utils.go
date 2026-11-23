package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
	"onecx.tkit.org/ocp/internal/manifest"
	"onecx.tkit.org/ocp/internal/template"
)

type sharedFilterOptions struct {
	apps     []string
	types    []string
	kinds    []string
	keys     []string
	archs    []string
	archOnly bool
	prefer   string
}

func sharedFilterFlags() (pflag.FlagSet, *sharedFilterOptions) {
	opts := sharedFilterOptions{}
	fs := pflag.FlagSet{}
	fs.StringSliceVar(&opts.apps, "app", nil, "filter by application name (e.g. onecx-parameter-svc)")
	fs.StringSliceVar(&opts.types, "type", nil, "filter by image type (jvm, native, unknown)")
	fs.StringSliceVar(&opts.kinds, "kind", nil, "filter by Kubernetes kind (e.g. Deployment, Job)")
	fs.StringSliceVar(&opts.keys, "key", nil, "filter by container key (containers, initContainers, ...)")
	fs.StringSliceVar(&opts.archs, "arch", nil, "filter images that support all given architectures (e.g. arm64)")
	fs.BoolVar(&opts.archOnly, "arch-only", false, "with --arch: image must support exactly the given architectures, nothing else")
	fs.StringVar(&opts.prefer, "prefer", "", "per application prefer this image type when both variants exist (jvm or native)")
	return fs, &opts
}

// match: values within one flag are OR-ed, different flags are AND-ed.
func (f *sharedFilterOptions) match(app string, img *manifest.Image) bool {
	return matchAny(app, f.apps) &&
		matchAny(img.Type, f.types) &&
		matchAny(img.Kind, f.kinds) &&
		matchAny(img.Key, f.keys) &&
		f.matchArch(img.Arch)
}

func (f *sharedFilterOptions) matchArch(have []string) bool {
	if len(f.archs) == 0 {
		return true
	}
	for _, want := range f.archs {
		if !containsFold(have, want) {
			return false
		}
	}
	if f.archOnly {
		for _, a := range have {
			if !containsFold(f.archs, a) {
				return false
			}
		}
	}
	return true
}

func matchAny(value string, allowed []string) bool {
	return len(allowed) == 0 || containsFold(allowed, value)
}

func containsFold(list []string, value string) bool {
	return slices.ContainsFunc(list, func(s string) bool {
		return strings.EqualFold(s, value)
	})
}

// preferType keeps, per kind/key slot, only images of the preferred type
// when such images exist; otherwise the slot is left unchanged.
func preferType(images []*manifest.Image, prefer string) []*manifest.Image {
	groups := map[string][]*manifest.Image{}
	for _, img := range images {
		slot := img.Kind + "/" + img.Key
		groups[slot] = append(groups[slot], img)
	}

	var res []*manifest.Image
	for _, group := range groups {
		hasPreferred := slices.ContainsFunc(group, func(img *manifest.Image) bool {
			return strings.EqualFold(img.Type, prefer)
		})
		for _, img := range group {
			if !hasPreferred || strings.EqualFold(img.Type, prefer) {
				res = append(res, img)
			}
		}
	}
	return res
}

type errorShouldDisplayUsage struct {
	error
}

func commandAction(handler func(args []string, stdout io.Writer) error) func(cmd *cobra.Command, args []string) error {
	return func(c *cobra.Command, args []string) error {
		err := handler(args, c.OutOrStdout())
		if _, ok := errors.AsType[errorShouldDisplayUsage](err); ok {
			c.SetOut(c.ErrOrStderr())
			_ = c.Help()
		}
		return err
	}
}

type sharedOutputOptions struct {
	format string
	file   string
}

func sharedOutputFlags() (pflag.FlagSet, *sharedOutputOptions) {
	opts := sharedOutputOptions{}
	fs := pflag.FlagSet{}
	fs.StringVarP(&opts.format, "format", "f", "", "Format the output to a specific format (json, yaml or plain)")
	fs.StringVarP(&opts.file, "output", "o", "", "Output file path")
	return fs, &opts
}

type sharedProductOptions struct {
	registry         string
	organization     string
	product          string
	version          string
	skipDependency   []string
	manifest         string
	chartUrlTemplate string
	helmValues       []string
}

func sharedProductFlags() (pflag.FlagSet, *sharedProductOptions) {
	opts := sharedProductOptions{}
	fs := pflag.FlagSet{}
	fs.StringVar(&opts.registry, "registry", "oci://ghcr.io", "helm chart OCI registry URL")
	fs.StringVar(&opts.organization, "organization", "onecx", "organization name")
	fs.StringVar(&opts.product, "product", "", "product name")
	fs.StringVar(&opts.version, "version", "", "product version")
	fs.StringVar(&opts.chartUrlTemplate, "chart-url-template", "{{.Registry}}/{{.Organization}}/charts/{{.Product}}:{{ .Version }}", "chart URL template. Variables [Registry, Organization, Product, Version]")
	fs.StringSliceVar(&opts.skipDependency, "skip-dependency", []string{"helm-product"}, "comma-separated dependency names to exclude from the result")
	fs.StringSliceVar(&opts.helmValues, "helm-value", []string{}, "comma-separated Helm values")
	return fs, &opts
}

func markProductFlagsRequired(cmd *cobra.Command) {
	for _, name := range []string{"product", "version"} {
		if err := cmd.MarkFlagRequired(name); err != nil {
			panic(err)
		}
	}
}

func markProductFlagsRequiredUnlessManifest(cmd *cobra.Command) {
	for _, name := range []string{"product", "version"} {
		cmd.MarkFlagsOneRequired("manifest", name)
		cmd.MarkFlagsMutuallyExclusive("manifest", name)
	}
}

func getManifests(files []string) ([]*manifest.Manifest, error) {
	manifests := make([]*manifest.Manifest, len(files))
	for i, file := range files {
		m, err := manifest.LoadManifestFromFile(file)
		if err != nil {
			return nil, err
		}
		manifests[i] = m
	}
	return manifests, nil
}

func getManifest(opts *sharedProductOptions) (*manifest.Manifest, error) {
	if opts.manifest != "" {
		return manifest.LoadManifestFromFile(opts.manifest)
	}
	url, err := chartURL(opts)
	if err != nil {
		return nil, err
	}
	cfg := &manifest.Config{
		Organization: opts.organization,
		Product:      opts.product,
		Version:      opts.version,
		ExcludeApps:  opts.skipDependency,
		Url:          url,
		HelmValues:   opts.helmValues,
	}
	return manifest.CreateManifest(cfg)
}

func writeOutput(cfg *sharedOutputOptions, stdout io.Writer, data any) error {
	format := cfg.format
	if format == "json" {
		out, err := json.MarshalIndent(data, "", "    ")
		if err != nil {
			return err
		}
		return write(cfg.file, out, stdout)
	}

	if format == "yaml" || format == "" {
		out, err := yaml.Marshal(data)
		if err != nil {
			return err
		}
		return write(cfg.file, out, stdout)
	}

	if format == "plain" {
		out := plain(data)
		return write(cfg.file, out, stdout)
	}

	return fmt.Errorf("unsupported format: %s", format)
}

func write(file string, out []byte, stdout io.Writer) error {
	out = append(bytes.TrimRight(out, "\r\n"), '\n')
	if file != "" {
		err := os.WriteFile(file, out, 0644)
		if err != nil {
			return err
		}
		return nil
	}
	fmt.Fprintf(stdout, "%s", string(out))
	return nil
}

func plain(data any) []byte {
	if s, ok := data.(fmt.Stringer); ok {
		return []byte(s.String())
	}
	rv := reflect.ValueOf(data)
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		lines := make([]string, rv.Len())
		for i := range rv.Len() {
			lines[i] = fmt.Sprint(rv.Index(i).Interface())
		}
		return []byte(strings.Join(lines, "\n"))
	}
	return fmt.Appendf(nil, "%v", data)
}

type templateData struct {
	Registry     string
	Organization string
	Product      string
	Version      string
}

func chartURL(opts *sharedProductOptions) (string, error) {
	data := templateData{
		Registry:     opts.registry,
		Organization: opts.organization,
		Product:      opts.product,
		Version:      opts.version,
	}
	chartURL, err := template.Print(opts.chartUrlTemplate, data)
	if err != nil {
		return "", fmt.Errorf("error print template: %w", err)
	}
	return chartURL, nil
}
