package tools

// Tool represents a function that can be called by the agent.
type Tool interface {
	// Name is the name of the tool, as it would be called by the model.
	Name() string
	// Description is a description of the tool's purpose, inputs, and outputs.
	Description() string
	// Parameters returns the JSON schema for the tool's arguments.
	Parameters() any
	// Execute runs the tool with the given arguments and returns the output.
	// The args are expected to be a JSON string.
	Execute(args string) (string, error)
	// RequiresConfirmation indicates whether the tool requires user confirmation before execution.
	RequiresConfirmation() bool
}
