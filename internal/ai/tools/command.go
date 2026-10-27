package tools

// Command returns shell commands for the user.
func Command() Definition {
	return Definition{
		Name:        "command",
		Description: "Return the final shell command or command sequence for the user to run.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"commands": {
					Type:        "array",
					Description: "Shell commands to place into the user's workflow in execution order.",
					Items:       &Schema{Type: "string", Description: "One shell command."},
				},
				"message": {
					Type:        "string",
					Description: "Optional short explanation shown to the user before the commands.",
				},
			},
			Required: []string{"commands"},
		},
	}
}
