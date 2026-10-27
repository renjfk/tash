package tools

// Definition describes one model-callable tool.
type Definition struct {
	Name        string
	Description string
	Parameters  Schema
}

// Schema is a small JSON Schema subset used for tool definitions.
type Schema struct {
	Type        string
	Description string
	Properties  map[string]Schema
	Items       *Schema
	Required    []string
	Enum        []string
}

// ToMap converts the schema to the OpenAI-compatible JSON shape.
func (s Schema) ToMap() map[string]any {
	m := map[string]any{}
	if s.Type != "" {
		m["type"] = s.Type
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if len(s.Properties) > 0 {
		props := make(map[string]any, len(s.Properties))
		for name, prop := range s.Properties {
			props[name] = prop.ToMap()
		}
		m["properties"] = props
	}
	if s.Items != nil {
		m["items"] = s.Items.ToMap()
	}
	if len(s.Required) > 0 {
		m["required"] = s.Required
	}
	if len(s.Enum) > 0 {
		m["enum"] = s.Enum
	}
	return m
}
