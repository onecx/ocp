package helm

import (
	"fmt"
	"io"
	"os"
	"strings"

	"helm.sh/helm/v4/pkg/action"
	v2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/downloader"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/registry"
	ri "helm.sh/helm/v4/pkg/release"
)

func GetChart(url string) (*v2.Chart, string, error) {
	tmpDir, err := os.MkdirTemp("", "ocp-chart-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	file, err := DownloadChart(url, tmpDir)
	if err != nil {
		return nil, "", fmt.Errorf("download chart: %w", err)
	}
	chrt, err := LoadChart(file)
	if err != nil {
		return nil, "", fmt.Errorf("load chart: %w", err)
	}
	return chrt, url, nil
}

func LoadChart(chartFile string) (*v2.Chart, error) {
	chrt, err := loader.Load(chartFile)
	if err != nil {
		return nil, fmt.Errorf("load chart %s: %w", chartFile, err)
	}
	return chrt, nil
}

func TemplateChart(chrt *v2.Chart, product string, values map[string]any) ([]string, error) {
	install := action.NewInstall(action.NewConfiguration())
	install.DryRunStrategy = action.DryRunClient
	install.Replace = true // skips the release-name lookup, as `helm template` does
	install.ReleaseName = product
	install.Namespace = "default"

	rel, err := install.Run(chrt, values)
	if err != nil {
		return nil, fmt.Errorf("render chart %s: %w", chrt.Metadata.Name, err)
	}
	acc, err := ri.NewAccessor(rel)
	if err != nil {
		return nil, fmt.Errorf("read rendered chart %s: %w", chrt.Metadata.Name, err)
	}

	// Hooks (e.g. pre-install Jobs) are rendered separately from the main manifest.
	manifests := []string{acc.Manifest()}
	for _, h := range acc.Hooks() {
		hook, err := ri.NewHookAccessor(h)
		if err != nil {
			return nil, fmt.Errorf("read chart hook %s: %w", chrt.Metadata.Name, err)
		}
		manifests = append(manifests, hook.Manifest())
	}
	return manifests, nil
}

func DigestChart(chartURL string) (string, error) {

	registryConfig := cli.New().RegistryConfig

	client, err := registry.NewClient(
		registry.ClientOptEnableCache(true),
		registry.ClientOptCredentialsFile(registryConfig),
	)
	if err != nil {
		return "", err
	}

	desc, err := client.Resolve(strings.TrimPrefix(chartURL, registry.OCIScheme+"://"))
	if err != nil {
		return "", fmt.Errorf("resolve chart %s: %w", chartURL, err)
	}
	return desc.Digest.String(), nil
}

func DownloadChart(chartURL, destDir string) (string, error) {
	client, err := registry.NewClient(registry.ClientOptEnableCache(true))
	if err != nil {
		return "", err
	}
	// DownloadTo creates its temp file in destDir, so the directory must exist.
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}

	settings := cli.New()
	dl := downloader.ChartDownloader{
		Out:              io.Discard,
		Verify:           downloader.VerifyNever,
		Getters:          getter.All(settings),
		Options:          []getter.Option{getter.WithRegistryClient(client)},
		RegistryClient:   client,
		RepositoryConfig: settings.RepositoryConfig,
		RepositoryCache:  settings.RepositoryCache,
		ContentCache:     settings.ContentCache,
	}
	file, _, err := dl.DownloadTo(chartURL, "", destDir)
	if err != nil {
		return "", fmt.Errorf("download chart %s: %w", chartURL, err)
	}
	return file, nil
}
