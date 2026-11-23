package products

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type ProductCreate struct {
	Registry     string `yaml:"registry"`
	Organization string `yaml:"organization"`
	Product      string `yaml:"product"`
	Count        int    `yaml:"count"`
}

type CreateFile struct {
	Defaults ProductCreate   `yaml:"defaults"`
	Products []ProductCreate `yaml:"products"`
}

func LoadCreateFile(path string) ([]ProductCreate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read products file %s: %w", path, err)
	}
	var pf CreateFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&pf); err != nil {
		return nil, fmt.Errorf("parse products file %s: %w", path, err)
	}

	if pf.Defaults.Count == 0 {
		pf.Defaults.Count = 10
	}

	result := make([]ProductCreate, 0, len(pf.Products))
	seen := map[ProductCreate]bool{}
	for i, p := range pf.Products {
		p = p.withDefaults(pf.Defaults)
		if err := p.validate(); err != nil {
			return nil, fmt.Errorf("%s: products[%d]: %w", path, i, err)
		}
		if seen[p] {
			return nil, fmt.Errorf("%s: products[%d]: duplicate %v", path, i, p)
		}
		seen[p] = true
		result = append(result, p)
	}
	return result, nil
}

func (p ProductCreate) withDefaults(d ProductCreate) ProductCreate {
	if p.Registry == "" {
		p.Registry = d.Registry
	}
	if p.Organization == "" {
		p.Organization = d.Organization
	}
	if p.Product == "" {
		p.Product = d.Product
	}
	if p.Count == 0 {
		p.Count = d.Count
	}
	return p
}

func (p ProductCreate) validate() error {
	var missing []string
	if p.Registry == "" {
		missing = append(missing, "registry")
	}
	if p.Organization == "" {
		missing = append(missing, "organization")
	}
	if p.Product == "" {
		missing = append(missing, "product")
	}
	if p.Count == 0 {
		missing = append(missing, "count")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing %s", strings.Join(missing, ", "))
	}
	return nil
}
