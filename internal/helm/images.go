package helm

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	v2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/strvals"
)

type HelmImage struct {
	Image string `json:"image" yaml:"image"`
	Kind  string `json:"kind" yaml:"kind"`
	Key   string `json:"key" yaml:"key"`
	Alias string `json:"alias" yaml:"alias"`
}

func (h *HelmImage) String() string {
	return fmt.Sprintf("%v %v %v %v", h.Alias, h.Key, h.Image, h.Kind)
}

type HelmImages struct {
	Images map[string][]*HelmImage `json:"images" yaml:"images"`
}

func TemplateImagesFromChart(chrt *v2.Chart, product string, inputValues []string) (*HelmImages, error) {

	values := map[string]any{}

	for _, v := range inputValues {
		if err := strvals.ParseInto(v, values); err != nil {
			return nil, fmt.Errorf("parse input value %s: %w", v, err)
		}
	}

	manifests, err := TemplateChart(chrt, product, values)
	if err != nil {
		return nil, err
	}
	images := []*HelmImage{}
	tmp, err := TemplateImagesList(manifests)
	if err != nil {
		return nil, fmt.Errorf("template images: %w", err)
	}
	images = append(images, tmp...)

	for _, dep := range chrt.Metadata.Dependencies {
		name := dep.Alias
		if name == "" {
			name = dep.Name
		}
		if dep.Condition != "" {
			if err := strvals.ParseInto(dep.Condition+"=true", values); err != nil {
				return nil, fmt.Errorf("parse condition of %s: %w", name, err)
			}
		}
		if err := strvals.ParseInto(name+".app.image.suffix=native", values); err != nil {
			return nil, fmt.Errorf("set native suffix of %s: %w", name, err)
		}
	}

	manifests, err = TemplateChart(chrt, product, values)
	if err != nil {
		return nil, err
	}
	tmp, err = TemplateImagesList(manifests)
	if err != nil {
		return nil, fmt.Errorf("template images: %w", err)
	}
	images = append(images, tmp...)

	mi := map[string]*HelmImage{}
	for _, img := range images {
		mi[img.String()] = img
	}

	r := map[string][]*HelmImage{}
	for _, v := range mi {
		r[v.Alias] = append(r[v.Alias], v)
	}

	result := &HelmImages{Images: r}
	return result, nil
}

func TemplateImagesList(manifests []string) ([]*HelmImage, error) {
	var images []*HelmImage
	err := decodeManifests(manifests, func(doc any, app string) {
		kind := manifestInfo(doc)
		collectImages(doc, func(img, key string) {
			images = append(images, &HelmImage{Image: img, Kind: kind, Alias: app, Key: key})
		})
	})
	if err != nil {
		return nil, err
	}
	return images, nil
}

func decodeManifests(manifests []string, fn func(doc any, app string)) error {
	for _, m := range manifests {
		for _, item := range splitDocs(m) {
			file := manifestFile(item)
			charts := manifestSubcharts(file)
			alias := charts[0]
			dec := yaml.NewDecoder(strings.NewReader(item))
			var doc any
			if err := dec.Decode(&doc); err != nil {
				return fmt.Errorf("parse rendered manifest of document: %w", err)
			}
			fn(doc, alias)
		}
	}
	return nil
}

var docSep = regexp.MustCompile(`(?m)^---[ \t]*$`)

// splitDocs splits a multi-document YAML string into non-empty documents.
func splitDocs(manifest string) []string {
	var docs []string
	for _, d := range docSep.Split(manifest, -1) {
		if strings.TrimSpace(d) == "" {
			continue // leading "---" yields an empty first part
		}
		docs = append(docs, strings.TrimSpace(d))
	}
	return docs
}

func manifestFile(m string) string {
	sc := bufio.NewScanner(strings.NewReader(m))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line == "---" {
			continue
		}
		if p, ok := strings.CutPrefix(line, "# Source:"); ok {
			return strings.TrimSpace(p)
		}
		return ""
	}
	return ""
}

func manifestSubcharts(path string) []string {
	parts := strings.Split(path, "/")
	var out []string
	for i := 1; i+1 < len(parts); i++ {
		if parts[i] == "charts" {
			out = append(out, parts[i+1])
			i++
		}
	}
	return out
}

func manifestInfo(doc any) (kind string) {
	m, ok := doc.(map[string]any)
	if !ok {
		return ""
	}
	kind, _ = m["kind"].(string)
	return kind
}

// collectImages walks any object so Pods, Deployments, StatefulSets, Jobs and CronJobs are all covered.
func collectImages(node any, add func(img, key string)) {
	switch n := node.(type) {
	case map[string]any:
		for key, val := range n {
			if key == "containers" || key == "initContainers" || key == "ephemeralContainers" {
				if list, ok := val.([]any); ok {
					for _, c := range list {
						if m, ok := c.(map[string]any); ok {
							if img, ok := m["image"].(string); ok && img != "" {
								add(img, key)
							}
						}
					}
				}
			}
			if key == "volumes" {
				if list, ok := val.([]any); ok {
					for _, v := range list {
						if vol, ok := v.(map[string]any); ok {
							if img, ok := vol["image"].(map[string]any); ok {
								if ref, ok := img["reference"].(string); ok && ref != "" {
									add(ref, key)
								}
							}
						}
					}
				}
			}
			collectImages(val, add)
		}
	case []any:
		for _, v := range n {
			collectImages(v, add)
		}
	}
}
