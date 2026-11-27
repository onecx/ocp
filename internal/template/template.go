package template

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"strings"
	t "text/template"

	"github.com/Masterminds/sprig/v3"
	"onecx.tkit.org/ocp/internal/manifest"
)

// nativeSuffix marks the tag of a native image; the matching jvm image has the same reference without it.
const nativeSuffix = "-native"

// funcMap returns the template functions: sprig plus the ocp specific ones.
func funcMap() t.FuncMap {
	fm := sprig.TxtFuncMap()
	fm["prefer"] = Prefer
	return fm
}

// Prefer filters images so that, for every image available in several variants
// (for example the native image "app:1.0-native" and the jvm image "app:1.0"),
// only the variant with the highest priority remains.
//
// The attribute is the name of an Image field ("Type") whose value is compared with values.
// The priority is the order of values: Prefer(images, "Type", "native", "jvm") keeps native
// if available, otherwise jvm. Variants are linked by the Image reference without the "-native" suffix.
//
// Images whose attribute value is not in the list are not part of the selection and are kept as they are.
// The order of the images is preserved. Usage in a template: {{ range prefer .Images "Type" "native" "jvm" }}
func Prefer(images []*manifest.Image, attribute string, values ...string) ([]*manifest.Image, error) {
	if _, ok := reflect.TypeFor[manifest.Image]().FieldByName(attribute); !ok {
		return nil, fmt.Errorf("prefer: image has no attribute %q", attribute)
	}
	value := func(img *manifest.Image) string {
		return fmt.Sprint(reflect.ValueOf(img).Elem().FieldByName(attribute).Interface())
	}
	rank := func(img *manifest.Image) int {
		return slices.Index(values, value(img))
	}
	key := func(img *manifest.Image) string {
		return strings.TrimSuffix(img.Image, nativeSuffix)
	}

	best := map[string]*manifest.Image{}
	for _, img := range images {
		if img == nil || rank(img) < 0 {
			continue
		}
		if cur, ok := best[key(img)]; !ok || rank(img) < rank(cur) {
			best[key(img)] = img
		}
	}

	result := make([]*manifest.Image, 0, len(images))
	for _, img := range images {
		if img == nil {
			continue
		}
		if rank(img) < 0 || best[key(img)] == img {
			result = append(result, img)
		}
	}
	return result, nil
}

func Print(template string, data any) (string, error) {
	temp := t.New("template")

	t, err := temp.Funcs(funcMap()).Parse(template)
	if err != nil {
		return "", fmt.Errorf("error parse template data: %w", err)
	}

	var tpl bytes.Buffer
	err = t.Execute(&tpl, data)
	if err != nil {
		return "", fmt.Errorf("error execute template: %w", err)
	}
	return tpl.String(), nil
}
