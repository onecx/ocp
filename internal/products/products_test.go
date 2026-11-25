package products

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeProductsFile(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "products.yaml")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadProductsFile(t *testing.T) {
	tests := []struct {
		name string
		data string
		want []ProductRef
	}{
		{
			name: "defaults and overrides",
			data: `defaults:
  registry: oci://registry.example
  organization: default-org
  product: default-product
  version: 1.0.0
products:
  - {}
  - registry: oci://other.example
    organization: other-org
    product: other-product
    version: 2.0.0
  - product: third-product
`,
			want: []ProductRef{
				{"oci://registry.example", "default-org", "default-product", "1.0.0"},
				{"oci://other.example", "other-org", "other-product", "2.0.0"},
				{"oci://registry.example", "default-org", "third-product", "1.0.0"},
			},
		},
		{name: "empty products", data: "products: []\n", want: []ProductRef{}},
		{name: "no products", data: "{}\n", want: []ProductRef{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadProductsFile(writeProductsFile(t, tt.data))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("products = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLoadProductsFileErrors(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{"empty file", "", "parse products file"},
		{"invalid yaml", "products: [", "parse products file"},
		{"invalid product type", "products: [not-an-object]", "parse products file"},
		{"missing fields", "products: [{}]", "products[0]: missing registry, organization, product, version"},
		{
			"duplicates after defaults",
			"defaults: {registry: registry.example, organization: org, product: app, version: 1.0.0}\nproducts:\n- {}\n- product: app\n",
			"products[1]: duplicate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeProductsFile(t, tt.data)
			got, err := LoadProductsFile(path)
			if err == nil || !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), path) {
				t.Fatalf("LoadProductsFile() = %#v, %v; want error containing %q and file path", got, err, tt.want)
			}
			if got != nil {
				t.Fatalf("returned partial products on error: %#v", got)
			}
		})
	}
	t.Run("missing file", func(t *testing.T) {
		_, err := LoadProductsFile(filepath.Join(t.TempDir(), "missing.yaml"))
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("error = %v, want wrapped file-not-found error", err)
		}
	})
}

func TestProductRefValidation(t *testing.T) {
	valid := ProductRef{"registry.example", "org", "app", "1.0.0"}
	tests := []struct {
		name string
		ref  ProductRef
		want string
	}{
		{"valid", valid, ""},
		{"registry", ProductRef{"", "org", "app", "1.0.0"}, "missing registry"},
		{"organization", ProductRef{"registry.example", "", "app", "1.0.0"}, "missing organization"},
		{"product", ProductRef{"registry.example", "org", "", "1.0.0"}, "missing product"},
		{"version", ProductRef{"registry.example", "org", "app", ""}, "missing version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ref.validate()
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}
