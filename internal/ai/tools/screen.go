package tools

// Screen asks tash to capture recent terminal output.
func Screen() Definition {
	return Definition{
		Name:        "screen",
		Description: "Read recent terminal screen output when the user references visible errors or output.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"count": {
					Type:        "integer",
					Description: "How many lines from the bottom of the terminal to capture.",
				},
			},
		},
	}
}
