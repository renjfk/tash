package tools

// Conversation asks tash to load older conversation entries.
func Conversation() Definition {
	return Definition{
		Name:        "conversation",
		Description: "Load older conversation entries when the current window is not enough.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"count": {
					Type:        "integer",
					Description: "How many additional older conversation entries to load.",
				},
			},
		},
	}
}
