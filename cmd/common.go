package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
	"onecx.tkit.org/ocp/internal/manifest"
	"onecx.tkit.org/ocp/internal/template"
)

type sharedCreateFileOptions struct {
	file string
}

func sharedCreateFileFlags() (pflag.FlagSet, *sharedCreateFileOptions) {
	opts := sharedCreateFileOptions{}
	fs := pflag.FlagSet{}
	fs.StringVar(&opts.file, "file", "", "YAML file listing products; see command help for its structure and example")
	markRequired(&fs, "file")
	return fs, &opts
}

type sharedProductsFileOptions struct {
	file string
}

func sharedProductsFileFlags() (pflag.FlagSet, *sharedProductsFileOptions) {
	opts := sharedProductsFileOptions{}
	fs := pflag.FlagSet{}
	fs.StringVar(&opts.file, "products-file", "", "YAML file listing products; see command help for its structure and example")
	markRequired(&fs, "products-file")
	return fs, &opts
}

type sharedHelmChartOptions struct {
	chartUrlTemplate string
}

func sharedHelmChartFlags() (pflag.FlagSet, *sharedHelmChartOptions) {
	opts := sharedHelmChartOptions{}
	fs := pflag.FlagSet{}
	fs.StringVar(&opts.chartUrlTemplate, "chart-url-template", "{{.Registry}}/{{.Organization}}/charts/{{.Product}}:{{ .Version }}", "chart URL template. Variables [Registry, Organization, Product, Version]")
	return fs, &opts
}

func (opts *sharedHelmChartOptions) helmChartUrl(registry, organization, product, version string) (string, error) {
	data := struct {
		Registry     string
		Organization string
		Product      string
		Version      string
	}{
		Registry:     registry,
		Organization: organization,
		Product:      product,
		Version:      version,
	}
	chartURL, err := template.Print(opts.chartUrlTemplate, data)
	if err != nil {
		return "", fmt.Errorf("error print template: %w", err)
	}
	return chartURL, nil
}

type sharedManifestOptions struct {
	skipDependency []string
	helmValues     []string
}

func sharedManifestFlags() (pflag.FlagSet, *sharedManifestOptions) {
	opts := sharedManifestOptions{}
	fs := pflag.FlagSet{}
	fs.StringSliceVar(&opts.skipDependency, "skip-dependency", []string{"helm-product"}, "comma-separated dependency names to exclude from the result")
	fs.StringSliceVar(&opts.helmValues, "helm-value", []string{}, "comma-separated Helm values")
	return fs, &opts
}

type sharedRepositoryOptions struct {
	dir                   string
	repositoryUrlTemplate string
}

func sharedRepositoryFlags() (pflag.FlagSet, *sharedRepositoryOptions) {

	homeDir, err := os.UserHomeDir()
	if err != nil {
		cobra.CheckErr(fmt.Errorf("resolve home directory: %w", err))
	}

	var opts sharedRepositoryOptions
	var flags pflag.FlagSet
	flags.StringVarP(&opts.dir, "dir", "d", filepath.Join(homeDir, ".ocp", "repository"), "Directory for the repository")
	flags.StringVar(&opts.repositoryUrlTemplate, "repository-url-template", "{{.Registry}}/{{.Organization}}/charts/{{.Product}}", "repository URL template. Variables [Registry, Organization, Product]")
	return flags, &opts
}

func (repo *sharedRepositoryOptions) createRepositoryDir() error {
	dir := repo.dir
	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	case err != nil:
		return fmt.Errorf("check directory %s: %w", dir, err)
	case !info.IsDir():
		return fmt.Errorf("%s exists but is not a directory", dir)
	}
	return nil
}

func (repo *sharedRepositoryOptions) repositoryManifestFile(registry, organization, product, version string) string {
	return filepath.Join(repo.dir, strings.TrimPrefix(registry, "oci://"), organization, product, version+".yaml")
}

func (repo *sharedRepositoryOptions) repositoryExistsManifest(manifestFile string) (bool, error) {
	if _, err := os.Stat(manifestFile); err == nil {
		return true, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("check manifest %s: %w", manifestFile, err)
	}
	return false, nil
}

func (repo *sharedRepositoryOptions) repositoryWriteManifestFile(registry string, m *manifest.Manifest) error {
	manifestFile := repo.repositoryManifestFile(registry, m.Organization, m.Product, m.Version)
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(manifestFile), 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", filepath.Dir(manifestFile), err)
	}
	if err := os.WriteFile(manifestFile, data, 0o644); err != nil {
		return fmt.Errorf("write manifest %s: %w", manifestFile, err)
	}
	return nil
}

func (repo *sharedRepositoryOptions) repositoryURL(registry, organization, product string) (string, error) {
	data := struct {
		Registry     string
		Organization string
		Product      string
	}{
		Registry:     registry,
		Organization: organization,
		Product:      product,
	}
	repositoryURL, err := template.Print(repo.repositoryUrlTemplate, data)
	if err != nil {
		return "", fmt.Errorf("error print template: %w", err)
	}
	return repositoryURL, nil
}

type sharedProductOptions struct {
	registry     string
	organization string
	product      string
}

func sharedProductFlags() (pflag.FlagSet, *sharedProductOptions) {
	opts := sharedProductOptions{}
	fs := pflag.FlagSet{}
	fs.StringVar(&opts.registry, "registry", "", "helm chart OCI registry URL")
	fs.StringVar(&opts.organization, "organization", "", "organization name")
	fs.StringVar(&opts.product, "product", "", "product name")
	markRequired(&fs, "registry", "organization", "product")
	return fs, &opts
}

type sharedProductVersionOptions struct {
	*sharedProductOptions
	version string
}

func sharedProductVersionFlags() (pflag.FlagSet, *sharedProductVersionOptions) {
	fs, opts := sharedProductFlags()
	popts := &sharedProductVersionOptions{
		sharedProductOptions: opts,
	}
	fs.StringVar(&popts.version, "version", "", "product version")
	markRequired(&fs, "version")
	return fs, popts
}

type sharedOutputOptions struct {
	format string
	file   string
}

func sharedOutputFlags() (pflag.FlagSet, *sharedOutputOptions) {
	opts := sharedOutputOptions{}
	fs := pflag.FlagSet{}
	fs.StringVarP(&opts.format, "format", "f", "yaml", "Format the output to a specific format (json, yaml or plain)")
	fs.StringVarP(&opts.file, "output", "o", "", "Output file path")
	return fs, &opts
}

func (o *sharedOutputOptions) write(out []byte, stdout io.Writer) error {
	out = append(bytes.TrimRight(out, "\r\n"), '\n')
	if o.file != "" {
		err := os.WriteFile(o.file, out, 0644)
		if err != nil {
			return err
		}
		return nil
	}
	_, err := fmt.Fprintf(stdout, "%s", string(out))
	if err != nil {
		return err
	}
	return nil
}

func (o *sharedOutputOptions) writeOutput(stdout io.Writer, data any) error {
	format := o.format
	if format == "json" {
		out, err := json.MarshalIndent(data, "", "    ")
		if err != nil {
			return err
		}
		return o.write(out, stdout)
	}

	if format == "yaml" || format == "" {
		out, err := yaml.Marshal(data)
		if err != nil {
			return err
		}
		return o.write(out, stdout)
	}

	if format == "plain" {
		out := plain(data)
		return o.write(out, stdout)
	}

	return fmt.Errorf("unsupported format: %s", format)
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

func markRequired(flags *pflag.FlagSet, names ...string) {
	for _, name := range names {
		if err := cobra.MarkFlagRequired(flags, name); err != nil {
			panic(err)
		}
	}
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

func createManifest(repo *sharedRepositoryOptions, helmChart *sharedHelmChartOptions, mf *sharedManifestOptions, registry, organization, product, version string) error {
	manifestFile := repo.repositoryManifestFile(registry, organization, product, version)

	exists, err := repo.repositoryExistsManifest(manifestFile)
	if err != nil {
		return fmt.Errorf("check manifest file in repository: %w", err)
	}
	if exists {
		return nil
	}

	slog.Info("create manifest", "file", manifestFile)
	url, err := helmChart.helmChartUrl(registry, organization, product, version)
	if err != nil {
		return err
	}
	cfg := &manifest.Config{
		Organization: organization,
		Product:      product,
		Version:      version,
		ExcludeApps:  mf.skipDependency,
		Url:          url,
		HelmValues:   mf.helmValues,
	}
	m, err := manifest.CreateManifest(cfg)
	if err != nil {
		return fmt.Errorf("download manifest: %w", err)
	}

	return repo.repositoryWriteManifestFile(registry, m)
}

func repositoryLoadManifest(repo *sharedRepositoryOptions, registry, organization, product, version string) (*manifest.Manifest, error) {
	manifestFile := repo.repositoryManifestFile(registry, organization, product, version)
	exists, err := repo.repositoryExistsManifest(manifestFile)
	if err != nil {
		return nil, fmt.Errorf("check manifest file in repository: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("manifest file does not exist in repository: %s", manifestFile)
	}

	m, err := manifest.LoadManifestFromFile(manifestFile)
	if err != nil {
		return nil, fmt.Errorf("load manifest from file: %w", err)
	}

	return m, nil
}
