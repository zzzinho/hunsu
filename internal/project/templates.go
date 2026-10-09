package project

import (
	"embed"
	"slices"
)

//go:embed templates
var templates embed.FS

func templateFor(name string) []byte {
	if !slices.Contains(designSystem, name) {
		name = "page.html"
	}
	data, _ := templates.ReadFile("templates/" + name)
	return data
}
