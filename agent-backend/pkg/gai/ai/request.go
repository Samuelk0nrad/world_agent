package ai

type AIRequest struct {
	Prompt       string
	SystemPrompt string
	MaxTokens    int
}

func (r AIRequest) CombinedPrompt() string {
	systemPrompt := r.SystemPrompt
	prompt := r.Prompt

	if systemPrompt == "" {
		return prompt
	}
	if prompt == "" {
		return systemPrompt
	}

	return systemPrompt + "\n\n" + prompt
}
