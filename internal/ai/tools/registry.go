package tools

import "sort"

// All returns all built-in tool definitions exposed to the model.
func All() []Definition {
	return []Definition{
		Command(),
		Chat(),
		Plan(),
		History(),
		Conversation(),
		Screen(),
		Memory(),
	}
}

// Names returns sorted tool names.
func Names(defs []Definition) []string {
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	sort.Strings(names)
	return names
}
