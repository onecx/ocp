package manifest

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"onecx.tkit.org/ocp/internal/registry"
)

func TestLoadManifestFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	data := `apiVersion: v1
organization: org
product: product
version: 1.0.0
createdAt: "2026-01-02T03:04:05Z"
revision: revision
buildVersion: build
helm: oci://registry.example/org/charts/product:1.0.0
digest: sha256:chart
applications:
  backend:
    helm: oci://registry.example/org/charts/backend:2.0.0
    version: 2.0.0
    digest: sha256:backend
    images:
      - image: registry.example/org/backend:2.0.0
        digest: sha256:image
        type: jvm
        kind: Deployment
        architecture: [amd64, arm64/v8]
        key: containers
        createdAt: created
        revision: image-revision
        buildVersion: image-build
        source: https://example.test/source
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadManifestFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := &Manifest{
		ApiVersion: "v1", Organization: "org", Product: "product", Version: "1.0.0",
		CreatedAt: "2026-01-02T03:04:05Z", Revision: "revision", BuildVersion: "build",
		Helm: "oci://registry.example/org/charts/product:1.0.0", Digest: "sha256:chart",
		Applications: map[string]*Application{
			"backend": {
				Helm: "oci://registry.example/org/charts/backend:2.0.0", Version: "2.0.0", Digest: "sha256:backend",
				Images: []*Image{{
					Image: "registry.example/org/backend:2.0.0", Digest: "sha256:image",
					Type: "jvm", Kind: "Deployment", Arch: []string{"amd64", "arm64/v8"}, Key: "containers",
					CreatedAt: "created", Revision: "image-revision", BuildVersion: "image-build", Source: "https://example.test/source",
				}},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("manifest = %#v, want %#v", got, want)
	}
}

func TestLoadManifestFromFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	if _, err := LoadManifestFromFile(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want wrapped file-not-found error", err)
	}
	if err := os.WriteFile(path, []byte("applications: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifestFromFile(path); err == nil || !strings.Contains(err.Error(), "parse manifest "+path) {
		t.Fatalf("error = %v, want parse error including path", err)
	}
}

func TestGetArchs(t *testing.T) {
	tests := []struct {
		name string
		desc *registry.ImageDescriptor
		want []string
	}{
		{"nil descriptor", nil, []string{}},
		{"no platforms", &registry.ImageDescriptor{}, []string{}},
		{"architectures and variants", &registry.ImageDescriptor{Platforms: []*v1.Platform{
			{OS: "linux", Architecture: "amd64"},
			{OS: "linux", Architecture: "arm64", Variant: "v8"},
		}}, []string{"amd64", "arm64/v8"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getArchs(tt.desc); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("architectures = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectType(t *testing.T) {
	tests := []struct {
		name string
		desc *registry.ImageDescriptor
		want ImageType
	}{
		{"nil descriptor", nil, TypeUnknown},
		{"missing config", &registry.ImageDescriptor{}, TypeUnknown},
		{"native", &registry.ImageDescriptor{Config: &v1.ConfigFile{}}, TypeNative},
		{"java environment", &registry.ImageDescriptor{Config: &v1.ConfigFile{Config: v1.Config{Env: []string{"PATH=/bin", "JAVA_APP_JAR=app.jar"}}}}, TypeJVM},
		{"empty java environment", &registry.ImageDescriptor{Config: &v1.ConfigFile{Config: v1.Config{Env: []string{"JAVA_APP_JAR="}}}}, TypeJVM},
		{"java entrypoint", &registry.ImageDescriptor{Config: &v1.ConfigFile{Config: v1.Config{Entrypoint: []string{"/bin/sh", "/deployments/run-java.sh"}}}}, TypeJVM},
		{"unrelated environment", &registry.ImageDescriptor{Config: &v1.ConfigFile{Config: v1.Config{Env: []string{"NOT_JAVA_APP_JAR=app.jar"}, Entrypoint: []string{"/app"}}}}, TypeNative},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectType(tt.desc); got != tt.want {
				t.Fatalf("type = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValue(t *testing.T) {
	tests := []struct {
		name string
		data map[string]string
		keys []string
		want string
	}{
		{"preferred annotation", map[string]string{"preferred": "first", "fallback": "second"}, []string{"preferred", "fallback"}, "first"},
		{"empty preferred annotation", map[string]string{"preferred": "", "fallback": "second"}, []string{"missing", "preferred", "fallback"}, "second"},
		{"no match", map[string]string{"other": "value"}, []string{"missing"}, ""},
		{"nil annotations", nil, []string{"missing"}, ""},
		{"no keys", map[string]string{"key": "value"}, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := value(tt.data, tt.keys...); got != tt.want {
				t.Fatalf("value = %q, want %q", got, tt.want)
			}
		})
	}
}
