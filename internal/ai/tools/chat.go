package tools

// Chat returns a short non-command reply.
func Chat() Definition {
	return Definition{
		Name:        "chat",
		Description: "Reply with a short chat or explanation instead of commands.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"message": {
					Type:        "string",
					Description: "The short reply to show to the user.",
				},
			},
			Required: []string{"message"},
		},
	}
}
