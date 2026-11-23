package products

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type ProductRef struct {
	Registry     string `yaml:"registry"`
	Organization string `yaml:"organization"`
	Product      string `yaml:"product"`
	Version      string `yaml:"version"`
}

type ProductsFile struct {
	Defaults ProductRef   `yaml:"defaults"`
	Products []ProductRef `yaml:"products"`
}

func LoadProductsFile(path string) ([]ProductRef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read products file %s: %w", path, err)
	}
	var pf ProductsFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&pf); err != nil {
		return nil, fmt.Errorf("parse products file %s: %w", path, err)
	}

	result := make([]ProductRef, 0, len(pf.Products))
	seen := map[ProductRef]bool{}
	for i, p := range pf.Products {
		p = p.withDefaults(pf.Defaults)
		if err := p.validate(); err != nil {
			return nil, fmt.Errorf("%s: products[%d]: %w", path, i, err)
		}
		if seen[p] {
			return nil, fmt.Errorf("%s: products[%d]: duplicate %s", path, i, p)
		}
		seen[p] = true
		result = append(result, p)
	}
	return result, nil
}

func (p ProductRef) withDefaults(d ProductRef) ProductRef {
	if p.Registry == "" {
		p.Registry = d.Registry
	}
	if p.Organization == "" {
		p.Organization = d.Organization
	}
	if p.Product == "" {
		p.Product = d.Product
	}
	if p.Version == "" {
		p.Version = d.Version
	}
	return p
}

func (p ProductRef) validate() error {
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
	if p.Version == "" {
		missing = append(missing, "version")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing %s", strings.Join(missing, ", "))
	}
	return nil
}
