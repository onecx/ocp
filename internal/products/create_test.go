package products

import (
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadCreateFile(t *testing.T) {
	tests := []struct {
		name string
		data string
		want []ProductCreate
	}{
		{
			name: "default count and field inheritance",
			data: "defaults: {registry: registry.example, organization: org, product: app}\nproducts:\n- {}\n- product: other\n",
			want: []ProductCreate{{"registry.example", "org", "app", 10}, {"registry.example", "org", "other", 10}},
		},
		{
			name: "configured count and explicit overrides",
			data: `defaults: {registry: registry.example, organization: org, product: app, count: 5}
products:
  - {}
  - registry: other.example
    organization: other-org
    product: other-app
    count: 2
`,
			want: []ProductCreate{{"registry.example", "org", "app", 5}, {"other.example", "other-org", "other-app", 2}},
		},
		{name: "empty products", data: "products: []\n", want: []ProductCreate{}},
		{name: "no products", data: "{}\n", want: []ProductCreate{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadCreateFile(writeProductsFile(t, tt.data))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("products = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLoadCreateFileErrors(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{"empty file", "", "parse products file"},
		{"invalid yaml", "products: [", "parse products file"},
		{"invalid count type", "products: [{count: many}]", "parse products file"},
		{"missing fields", "products: [{}]", "products[0]: missing registry, organization, product"},
		{
			"duplicates after defaults",
			"defaults: {registry: registry.example, organization: org, product: app, count: 2}\nproducts:\n- {}\n- count: 2\n",
			"products[1]: duplicate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeProductsFile(t, tt.data)
			got, err := LoadCreateFile(path)
			if err == nil || !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), path) {
				t.Fatalf("LoadCreateFile() = %#v, %v; want error containing %q and file path", got, err, tt.want)
			}
			if got != nil {
				t.Fatalf("returned partial products on error: %#v", got)
			}
		})
	}
	t.Run("missing file", func(t *testing.T) {
		_, err := LoadCreateFile(filepath.Join(t.TempDir(), "missing.yaml"))
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("error = %v, want wrapped file-not-found error", err)
		}
	})
}

func TestProductCreateValidation(t *testing.T) {
	tests := []struct {
		name string
		ref  ProductCreate
		want string
	}{
		{"valid", ProductCreate{"registry.example", "org", "app", 1}, ""},
		{"registry", ProductCreate{"", "org", "app", 1}, "missing registry"},
		{"organization", ProductCreate{"registry.example", "", "app", 1}, "missing organization"},
		{"product", ProductCreate{"registry.example", "org", "", 1}, "missing product"},
		{"count", ProductCreate{"registry.example", "org", "app", 0}, "missing count"},
		{"all fields", ProductCreate{}, "missing registry, organization, product, count"},
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
