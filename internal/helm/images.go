package helm

import (
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
	v2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/strvals"
)

type HelmImage struct {
	Image string `json:"image" yaml:"image"`
	Kind  string `json:"kind" yaml:"kind"`
	Key   string `json:"key" yaml:"key"`
	App   string `json:"app" yaml:"app"`
}

func (h *HelmImage) String() string {
	return fmt.Sprintf("%v %v %v %v", h.App, h.Key, h.Image, h.Kind)
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
		r[v.App] = append(r[v.App], v)
	}

	result := &HelmImages{Images: r}
	return result, nil
}

func TemplateImagesList(manifests []string) ([]*HelmImage, error) {
	var images []*HelmImage
	err := decodeManifests(manifests, func(doc any) {
		kind, app := manifestInfo(doc)
		collectImages(doc, func(img, key string) {
			images = append(images, &HelmImage{Image: img, Kind: kind, App: app, Key: key})
		})
	})
	if err != nil {
		return nil, err
	}
	return images, nil
}

func decodeManifests(manifests []string, fn func(doc any)) error {
	for _, m := range manifests {
		dec := yaml.NewDecoder(strings.NewReader(m))
		for {
			var doc any
			if err := dec.Decode(&doc); err == io.EOF {
				break
			} else if err != nil {
				return fmt.Errorf("parse rendered manifest of document: %w", err)
			}
			fn(doc)
		}
	}
	return nil
}

func manifestInfo(doc any) (kind, app string) {
	m, ok := doc.(map[string]any)
	if !ok {
		return "", ""
	}
	kind, _ = m["kind"].(string)
	if meta, ok := m["metadata"].(map[string]any); ok {
		if labels, ok := meta["labels"].(map[string]any); ok {
			app, _ = labels["app"].(string)
		}
	}
	return kind, app
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
