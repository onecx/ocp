package helm

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/chart/common"
	v2 "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	ri "helm.sh/helm/v4/pkg/release"
)

func isolateHelm(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DOCKER_CONFIG", filepath.Join(home, "docker"))
	t.Setenv("HELM_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("HELM_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("HELM_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("HELM_REGISTRY_CONFIG", filepath.Join(home, "registry.json"))
	t.Setenv("HELM_PLUGINS", filepath.Join(home, "plugins"))
}

func testChart(files ...*common.File) *v2.Chart {
	return &v2.Chart{
		Metadata:  &v2.Metadata{APIVersion: "v2", Name: "app", Version: "1.0.0"},
		Values:    map[string]any{"image": "registry.example/app:1.0.0"},
		Templates: files,
	}
}

func chartArchive(t *testing.T, chart *v2.Chart) (string, []byte) {
	t.Helper()
	path, err := chartutil.Save(chart, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, data
}

func TestLoadChart(t *testing.T) {
	path, _ := chartArchive(t, testChart(&common.File{Name: "templates/pod.yaml", Data: []byte("kind: Pod\n")}))
	got, err := LoadChart(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Metadata.Name != "app" || got.Metadata.Version != "1.0.0" || len(got.Templates) != 1 {
		t.Fatalf("loaded unexpected chart: %#v", got)
	}
	missing := filepath.Join(t.TempDir(), "missing.tgz")
	if _, err := LoadChart(missing); err == nil || !strings.Contains(err.Error(), "load chart "+missing) {
		t.Fatalf("error = %v, want contextual load error", err)
	}
}

func TestGetChartHTTP(t *testing.T) {
	isolateHelm(t)
	_, archive := chartArchive(t, testChart())
	for _, tt := range []struct {
		name   string
		status int
		body   []byte
		want   string
	}{
		{"valid chart", http.StatusOK, archive, ""},
		{"missing chart", http.StatusNotFound, []byte("missing"), "download chart"},
		{"invalid archive", http.StatusOK, []byte("not a chart"), "load chart"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write(tt.body)
			}))
			defer server.Close()
			url := server.URL + "/app-1.0.0.tgz"
			got, gotURL, err := GetChart(url)
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) || got != nil || gotURL != "" {
					t.Fatalf("GetChart() = %v, %q, %v; want error containing %q", got, gotURL, err, tt.want)
				}
			} else if err != nil || got == nil || got.Metadata.Name != "app" || gotURL != url {
				t.Fatalf("GetChart() = %v, %q, %v", got, gotURL, err)
			}
			leftovers, err := filepath.Glob(filepath.Join(tmp, "ocp-chart-*"))
			if err != nil || len(leftovers) != 0 {
				t.Fatalf("temporary chart directories not cleaned up: %v, %v", leftovers, err)
			}
		})
	}
}

func TestGetChartTempDirectoryError(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if _, _, err := GetChart("unused"); err == nil || !strings.Contains(err.Error(), "create temp dir") {
		t.Fatalf("error = %v, want temporary directory error", err)
	}
}

func TestTemplateChartIncludesHooksAndValues(t *testing.T) {
	chart := testChart(
		&common.File{Name: "templates/pod.yaml", Data: []byte(`apiVersion: v1
kind: Pod
metadata:
  name: {{ .Release.Name }}
  namespace: {{ .Release.Namespace }}
spec:
  containers:
    - name: app
      image: {{ .Values.image }}
`)},
		&common.File{Name: "templates/hook.yaml", Data: []byte(`apiVersion: batch/v1
kind: Job
metadata:
  name: setup
  annotations:
    helm.sh/hook: pre-install
spec:
  template:
    spec:
      containers:
        - name: setup
          image: registry.example/setup:1
      restartPolicy: Never
`)},
	)
	got, err := TemplateChart(chart, "release-name", map[string]any{"image": "registry.example/override:2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !strings.Contains(got[0], "name: release-name") ||
		!strings.Contains(got[0], "namespace: default") || !strings.Contains(got[0], "image: registry.example/override:2") ||
		!strings.Contains(got[1], "image: registry.example/setup:1") {
		t.Fatalf("rendered manifests = %q", got)
	}
}

func TestTemplateChartErrors(t *testing.T) {
	for _, tt := range []struct {
		name     string
		product  string
		template string
	}{
		{"invalid release name", "INVALID_NAME", "kind: Pod\n"},
		{"invalid template", "app", "{{"},
		{"invalid yaml", "app", "kind: [\n"},
		{"template failure", "app", `{{fail "render failed"}}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			chart := testChart(&common.File{Name: "templates/pod.yaml", Data: []byte(tt.template)})
			if got, err := TemplateChart(chart, tt.product, nil); err == nil || got != nil || !strings.Contains(err.Error(), "render chart app") {
				t.Fatalf("TemplateChart() = %v, %v; want contextual render error", got, err)
			}
		})
	}
}

func TestTemplateChartAccessorErrors(t *testing.T) {
	sentinel := errors.New("accessor unavailable")
	t.Run("release accessor", func(t *testing.T) {
		original := ri.NewAccessor
		t.Cleanup(func() { ri.NewAccessor = original })
		ri.NewAccessor = func(ri.Releaser) (ri.Accessor, error) { return nil, sentinel }
		if _, err := TemplateChart(testChart(), "app", nil); !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "read rendered chart app") {
			t.Fatalf("error = %v, want wrapped accessor error", err)
		}
	})
	t.Run("hook accessor", func(t *testing.T) {
		original := ri.NewHookAccessor
		t.Cleanup(func() { ri.NewHookAccessor = original })
		ri.NewHookAccessor = func(ri.Hook) (ri.HookAccessor, error) { return nil, sentinel }
		chart := testChart(&common.File{Name: "templates/hook.yaml", Data: []byte("apiVersion: v1\nkind: Pod\nmetadata:\n  name: hook\n  annotations:\n    helm.sh/hook: pre-install\n")})
		if _, err := TemplateChart(chart, "app", nil); !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "read chart hook app") {
			t.Fatalf("error = %v, want wrapped hook accessor error", err)
		}
	})
}

func TestRegistryClientConfigurationErrors(t *testing.T) {
	isolateHelm(t)
	if err := os.WriteFile(os.Getenv("HELM_REGISTRY_CONFIG"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		run  func() error
	}{
		{"client", func() error { _, err := createClient(); return err }},
		{"versions", func() error { _, err := GetVersions("unused"); return err }},
		{"digest", func() error { _, err := DigestChart("unused"); return err }},
		{"download", func() error { _, err := DownloadChart("unused", t.TempDir()); return err }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); err == nil {
				t.Fatal("invalid credentials configuration accepted")
			}
		})
	}
}

func TestRegistryOperationErrors(t *testing.T) {
	isolateHelm(t)
	if _, err := GetVersions("oci://invalid reference !"); err == nil || !strings.Contains(err.Error(), `list versions for "invalid reference !"`) {
		t.Fatalf("error = %v, want contextual versions error", err)
	}
	if _, err := DigestChart("oci://invalid reference !"); err == nil || !strings.Contains(err.Error(), "resolve chart") {
		t.Fatalf("error = %v, want contextual resolve error", err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := DownloadChart("unused", filepath.Join(file, "child")); err == nil {
		t.Fatal("download destination beneath regular file accepted")
	}
	if _, err := DownloadChart("unsupported://chart", t.TempDir()); err == nil || !strings.Contains(err.Error(), "download chart") {
		t.Fatalf("error = %v, want contextual download error", err)
	}
}

func TestDownloadChartCreatesDestination(t *testing.T) {
	isolateHelm(t)
	_, archive := chartArchive(t, testChart())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	dest := filepath.Join(t.TempDir(), "nested", "charts")
	path, err := DownloadChart(server.URL+"/app-1.0.0.tgz", dest)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close downloaded chart: %v", err)
		}
	}()
	data, err := io.ReadAll(file)
	if err != nil || string(data) != string(archive) || filepath.Dir(path) != dest {
		t.Fatalf("downloaded chart differs: path=%q, error=%v", path, err)
	}
}
