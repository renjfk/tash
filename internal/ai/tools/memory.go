package tools

// Memory saves a durable fact about the user.
func Memory() Definition {
	return Definition{
		Name:        "memory",
		Description: "Store a durable fact about the user for future sessions. Always call chat or command in the same response; the user never sees the memory call.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"message": {
					Type:        "string",
					Description: "The durable user fact to remember.",
				},
			},
			Required: []string{"message"},
		},
	}
}
