package project

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// ProjectTemplate is the structure read from project-template.json.
// It defines the scaffold applied to a newly created GitHub Project.
type ProjectTemplate struct {
	ShortDescription string            `json:"shortDescription"`
	Readme           string            `json:"readme"`
	StatusField      StatusFieldConfig `json:"statusField"`
	Views            []ViewConfig      `json:"views"`
}

// StatusFieldConfig holds the desired options for the project's Status field.
type StatusFieldConfig struct {
	Options []StatusOption `json:"options"`
}

// StatusOption is a single option in the Status single-select field.
type StatusOption struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// ViewConfig describes a view to be created in the GitHub Project.
type ViewConfig struct {
	Name   string `json:"name"`
	Layout string `json:"layout"`
	Filter string `json:"filter"`
}

//go:embed assets/project-template.json
var embeddedTemplate []byte

// ReadProjectTemplate parses the embedded project template.
func ReadProjectTemplate() (*ProjectTemplate, error) {
	var tpl ProjectTemplate
	if err := json.Unmarshal(embeddedTemplate, &tpl); err != nil {
		return nil, fmt.Errorf("parsing project template: %w", err)
	}
	return &tpl, nil
}
