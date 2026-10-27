package tools

import "testing"

func TestAll_ContainsCoreTools(t *testing.T) {
	names := Names(All())
	want := map[string]bool{
		"command": true, "chat": true, "plan": true,
		"history": true, "conversation": true, "screen": true, "memory": true,
	}
	for _, name := range names {
		delete(want, name)
	}
	if len(want) != 0 {
		t.Errorf("missing tools: %v", want)
	}
}

func TestNames_Sorted(t *testing.T) {
	names := Names(All())
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("names not sorted: %v", names)
		}
	}
}

func TestSchema_ToMap(t *testing.T) {
	def := Command()
	m := def.Parameters.ToMap()
	if m["type"] != "object" {
		t.Errorf("expected object type, got %v", m["type"])
	}
	props, ok := m["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map, got %T", m["properties"])
	}
	commands, ok := props["commands"].(map[string]any)
	if !ok {
		t.Fatalf("expected commands schema, got %T", props["commands"])
	}
	if commands["type"] != "array" {
		t.Errorf("expected commands array, got %v", commands["type"])
	}
	if _, ok := commands["items"].(map[string]any); !ok {
		t.Errorf("expected commands items schema, got %T", commands["items"])
	}
	required, ok := m["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "commands" {
		t.Errorf("expected required [commands], got %v", m["required"])
	}
}

func TestSchema_Enum(t *testing.T) {
	m := Schema{Type: "string", Enum: []string{"a", "b"}}.ToMap()
	enum, ok := m["enum"].([]string)
	if !ok || len(enum) != 2 {
		t.Errorf("expected enum [a b], got %v", m["enum"])
	}
}
