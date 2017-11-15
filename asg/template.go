package asg

import (
	"strings"
	"text/template"
)

func trimPrefix(prefix, s string) string { return strings.TrimPrefix(s, prefix) }

func getTemplate(name, tpl string) *template.Template {
	templateFuncs := template.FuncMap{
		"trim_prefix": trimPrefix,
	}

	return template.Must(template.New(name).Funcs(templateFuncs).Parse(tpl))
}
