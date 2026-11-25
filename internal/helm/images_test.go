package helm

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/chart/common"
	v2 "helm.sh/helm/v4/pkg/chart/v2"
)

func imageSignatures(images []*HelmImage) []string {
	result := make([]string, len(images))
	for i, image := range images {
		result[i] = image.String()
	}
	slices.Sort(result)
	return result
}

func TestTemplateImagesList(t *testing.T) {
	manifests := []string{`apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: backend
spec:
  template:
    spec:
      containers:
        - {name: app, image: "registry.example/app:1"}
        - {name: duplicate, image: "registry.example/app:1"}
        - {name: empty, image: ""}
        - {name: numeric, image: 123}
        - not-an-object
      initContainers:
        - {image: "registry.example/init:1"}
      ephemeralContainers:
        - {image: "registry.example/debug:1"}
      volumes:
        - {image: {reference: "registry.example/volume:1"}}
        - {image: {reference: ""}}
        - {image: {reference: 123}}
        - {image: not-an-object}
        - not-an-object
---
apiVersion: batch/v1
kind: CronJob
metadata:
  labels:
    app: cron
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - {image: "registry.example/cron:1"}
---
kind: Pod
spec:
  containers:
    - {image: "registry.example/unlabelled:1"}
  initContainers: not-a-list
  volumes: not-a-list
---
- scalar
- 42
---
null
`, ""}
	got, err := TemplateImagesList(manifests)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		" containers registry.example/unlabelled:1 Pod",
		"backend containers registry.example/app:1 Deployment",
		"backend containers registry.example/app:1 Deployment",
		"backend ephemeralContainers registry.example/debug:1 Deployment",
		"backend initContainers registry.example/init:1 Deployment",
		"backend volumes registry.example/volume:1 Deployment",
		"cron containers registry.example/cron:1 CronJob",
	}
	if result := imageSignatures(got); !reflect.DeepEqual(result, want) {
		t.Fatalf("images = %q, want %q", result, want)
	}
}

func TestTemplateImagesListErrors(t *testing.T) {
	got, err := TemplateImagesList([]string{"kind: Pod", "spec: ["})
	if got != nil || err == nil || !strings.Contains(err.Error(), "parse rendered manifest of document") {
		t.Fatalf("TemplateImagesList() = %v, %v; want no partial images and parse error", got, err)
	}
}

func TestManifestInfo(t *testing.T) {
	for _, tt := range []struct {
		name string
		doc  any
		kind string
		app  string
	}{
		{"scalar", "value", "", ""},
		{"invalid metadata", map[string]any{"kind": "Pod", "metadata": "invalid"}, "Pod", ""},
		{"invalid labels", map[string]any{"metadata": map[string]any{"labels": []any{"invalid"}}}, "", ""},
		{"invalid app", map[string]any{"kind": 123, "metadata": map[string]any{"labels": map[string]any{"app": 123}}}, "", ""},
		{"valid labels", map[string]any{"kind": "Job", "metadata": map[string]any{"labels": map[string]any{"app": "worker"}}}, "Job", "worker"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			kind, app := manifestInfo(tt.doc)
			if kind != tt.kind || app != tt.app {
				t.Fatalf("manifestInfo() = %q, %q; want %q, %q", kind, app, tt.kind, tt.app)
			}
		})
	}
}

func TestTemplateImagesFromChartDeduplicates(t *testing.T) {
	chart := testChart(&common.File{Name: "templates/pod.yaml", Data: []byte(`apiVersion: v1
kind: Pod
metadata:
  name: app
  labels:
    app: backend
spec:
  containers:
    - {name: app, image: "{{ .Values.image }}"}
    - {name: duplicate, image: "{{ .Values.image }}"}
`)})
	got, err := TemplateImagesFromChart(chart, "app", []string{"image=registry.example/override:2"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"backend containers registry.example/override:2 Pod"}
	if len(got.Images) != 1 || !reflect.DeepEqual(imageSignatures(got.Images["backend"]), want) {
		t.Fatalf("images = %#v, want one deduplicated image", got.Images)
	}
}

func dependencyChart(alias, condition, body string) *v2.Chart {
	name := "backend"
	key := name
	if alias != "" {
		key = alias
	}
	child := &v2.Chart{
		Metadata: &v2.Metadata{APIVersion: "v2", Name: name, Version: "1.0.0"},
		Values: map[string]any{
			"app": map[string]any{"image": map[string]any{"suffix": ""}},
		},
		Templates: []*common.File{{Name: "templates/pod.yaml", Data: []byte(body)}},
	}
	parent := testChart()
	parent.Metadata.Dependencies = []*v2.Dependency{{Name: name, Alias: alias, Version: "1.0.0", Condition: condition}}
	parent.Values[key] = map[string]any{"enabled": true}
	parent.AddDependency(child)
	return parent
}

func TestTemplateImagesFromChartNativeDependencies(t *testing.T) {
	for _, alias := range []string{"", "alias"} {
		t.Run("alias="+alias, func(t *testing.T) {
			key := "backend"
			if alias != "" {
				key = alias
			}
			chart := dependencyChart(alias, key+".enabled", `apiVersion: v1
kind: Pod
metadata:
  name: backend
  labels:
    app: backend
spec:
  containers:
    - name: backend
      image: registry.example/backend:1{{ if .Values.app.image.suffix }}-{{ .Values.app.image.suffix }}{{ end }}
`)
			got, err := TemplateImagesFromChart(chart, "app", nil)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{
				"backend containers registry.example/backend:1 Pod",
				"backend containers registry.example/backend:1-native Pod",
			}
			if !reflect.DeepEqual(imageSignatures(got.Images["backend"]), want) {
				t.Fatalf("images = %q, want %q", imageSignatures(got.Images["backend"]), want)
			}
		})
	}
}

func TestTemplateImagesFromChartErrors(t *testing.T) {
	for _, tt := range []struct {
		name   string
		chart  *v2.Chart
		values []string
		want   string
	}{
		{"invalid value", testChart(), []string{"broken[not-an-index]=value"}, "parse input value"},
		{"render failure", testChart(&common.File{Name: "templates/pod.yaml", Data: []byte(`{{fail "render unavailable"}}`)}), nil, "render unavailable"},
		{"invalid dependency condition", dependencyChart("", "backend.enabled[not-an-index]", "kind: Pod\nmetadata:\n  name: backend\n"), nil, "parse condition of backend"},
		{"native render failure", dependencyChart("", "", `{{if eq .Values.app.image.suffix "native"}}{{fail "native unavailable"}}{{end}}
kind: Pod
metadata:
  name: backend
`), nil, "native unavailable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TemplateImagesFromChart(tt.chart, "app", tt.values)
			if got != nil || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("TemplateImagesFromChart() = %v, %v; want error containing %q", got, err, tt.want)
			}
		})
	}
}
