package template

import (
	"bytes"
	"fmt"
	t "text/template"

	"github.com/Masterminds/sprig/v3"
)

func Print(template string, data interface{}) (string, error) {
	temp := t.New("template")

	t, err := temp.Funcs(sprig.TxtFuncMap()).Parse(template)
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
