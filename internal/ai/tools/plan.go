package tools

// Plan returns the next step of an iterative plan.
func Plan() Definition {
	return Definition{
		Name:        "plan",
		Description: "Return the next command in an iterative plan when later steps depend on live output.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"commands": {
					Type:        "array",
					Description: "Exactly one next-step shell command to run now.",
					Items:       &Schema{Type: "string", Description: "One shell command."},
				},
				"message": {
					Type:        "string",
					Description: "Optional short explanation for why this step is next.",
				},
				"steps_remaining": {
					Type:        "integer",
					Description: "Estimated remaining steps after this one finishes.",
				},
			},
			Required: []string{"commands", "steps_remaining"},
		},
	}
}
