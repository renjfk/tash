package tools

// History asks tash to search shell history for more context.
func History() Definition {
	return Definition{
		Name:        "history",
		Description: "Search recent shell history for more context before answering.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"filter": {
					Type:        "string",
					Description: "Optional grep-style filter for narrowing history matches.",
				},
				"count": {
					Type:        "integer",
					Description: "Maximum number of history entries to inspect or return.",
				},
			},
		},
	}
}
