package catalog

import (
	_ "embed"
	"encoding/json"
)

//go:embed endpoints.json
var data []byte

type Field struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Repeated    bool   `json:"repeated"`
	Description string `json:"description"`
}

type Endpoint struct {
	Section string  `json:"section"`
	Title   string  `json:"title"`
	Path    string  `json:"path"`
	Fields  []Field `json:"fields"`
}

func All() ([]Endpoint, error) {
	var endpoints []Endpoint
	err := json.Unmarshal(data, &endpoints)
	return endpoints, err
}
