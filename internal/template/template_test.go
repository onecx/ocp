package template

import (
	"strings"
	"testing"

	"onecx.tkit.org/ocp/internal/manifest"
)

func TestPrint(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     any
		want     string
	}{
		{"fields", "{{.Registry}}/{{.Product}}:{{.Version}}", map[string]string{"Registry": "registry.example", "Product": "app", "Version": "1.2.3"}, "registry.example/app:1.2.3"},
		{"sprig functions", `{{ default "fallback" .Name | upper }}:{{ add 2 3 }}`, map[string]string{}, "FALLBACK:5"},
		{"range", "{{range .}}{{.}}\n{{end}}", []string{"first", "second"}, "first\nsecond\n"},
		{"empty template", "", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Print(tt.template, tt.data)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("Print() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrintErrors(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     any
		want     string
	}{
		{"syntax", "{{", nil, "error parse template data"},
		{"unknown function", "{{notAFunction}}", nil, "error parse template data"},
		{"missing struct field", "{{.Missing}}", struct{}{}, "error execute template"},
		{"sprig failure", `{{fail "intentional failure"}}`, nil, "intentional failure"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Print(tt.template, tt.data)
			if got != "" || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Print() = %q, %v; want empty output and error containing %q", got, err, tt.want)
			}
		})
	}
}

func TestPreferTypes(t *testing.T) {
	jvm := &manifest.Image{Image: "ghcr.io/onecx/a:1.0", Type: "jvm"}
	native := &manifest.Image{Image: "ghcr.io/onecx/a:1.0-native", Type: "native"}
	jvmOnly := &manifest.Image{Image: "ghcr.io/onecx/b:2.0", Type: "jvm"}
	nativeOnly := &manifest.Image{Image: "ghcr.io/onecx/c:3.0-native", Type: "native"}
	other := &manifest.Image{Image: "ghcr.io/onecx/d:1.0", Type: "sidecar"}

	tests := []struct {
		name   string
		images []*manifest.Image
		types  []string
		want   []*manifest.Image
	}{
		{"native preferred", []*manifest.Image{jvm, native}, []string{"native", "jvm"}, []*manifest.Image{native}},
		{"jvm preferred", []*manifest.Image{jvm, native}, []string{"jvm", "native"}, []*manifest.Image{jvm}},
		{"fallback to jvm", []*manifest.Image{jvmOnly}, []string{"native", "jvm"}, []*manifest.Image{jvmOnly}},
		{"native only", []*manifest.Image{nativeOnly}, []string{"native", "jvm"}, []*manifest.Image{nativeOnly}},
		{"order kept, mixed", []*manifest.Image{jvm, jvmOnly, native, nativeOnly}, []string{"native", "jvm"}, []*manifest.Image{jvmOnly, native, nativeOnly}},
		{"unlisted type kept", []*manifest.Image{other, jvm, native}, []string{"native", "jvm"}, []*manifest.Image{other, native}},
		{"no types keeps all", []*manifest.Image{jvm, native}, nil, []*manifest.Image{jvm, native}},
		{"nil entries dropped", []*manifest.Image{nil, jvm}, []string{"native", "jvm"}, []*manifest.Image{jvm}},
		{"empty", nil, []string{"native"}, []*manifest.Image{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Prefer(tt.images, "Type", tt.types...)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("PreferTypes() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("PreferTypes()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPrintPreferTypes(t *testing.T) {
	data := map[string][]*manifest.Image{"Images": {
		{Image: "a:1", Type: "jvm"},
		{Image: "a:1-native", Type: "native"},
	}}
	got, err := Print(`{{ range prefer .Images "Type" "native" "jvm" }}{{ .Image }};{{ end }}`, data)
	if err != nil {
		t.Fatal(err)
	}
	if got != "a:1-native;" {
		t.Fatalf("Print() = %q", got)
	}
}

func TestPreferAttribute(t *testing.T) {
	a := &manifest.Image{Image: "x:1", Kind: "Deployment"}
	b := &manifest.Image{Image: "x:1-native", Kind: "StatefulSet"}
	got, err := Prefer([]*manifest.Image{a, b}, "Kind", "StatefulSet", "Deployment")
	if err != nil || len(got) != 1 || got[0] != b {
		t.Fatalf("Prefer() = %v, %v", got, err)
	}
	if _, err := Prefer(nil, "Unknown", "x"); err == nil {
		t.Fatal("expected error for unknown attribute")
	}
	if _, err := Print(`{{ prefer .Images "Unknown" "x" }}`, map[string][]*manifest.Image{}); err == nil {
		t.Fatal("expected template error for unknown attribute")
	}
}
