package prompts

import (
	"strings"
	"testing"
)

func TestSelectProvider(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Provider
	}{
		{name: "empty defaults to openai", input: "", want: ProviderOpenAI},
		{name: "openai selected", input: "openai", want: ProviderOpenAI},
		{name: "openai selected case insensitive", input: " OpenAI ", want: ProviderOpenAI},
		{name: "anthropic selected", input: "anthropic", want: ProviderAnthropic},
		{name: "groq selected", input: "groq", want: ProviderGroq},
		{name: "unknown falls back to openai", input: "gemini", want: ProviderOpenAI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SelectProvider(tt.input); got != tt.want {
				t.Fatalf("SelectProvider(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRenderValidationPlannerSystemPromptSharedCoreClauses(t *testing.T) {
	providers := []string{"", "openai", "anthropic", "groq", "unknown-provider"}
	requiredClauses := []string{
		"assumed-breach pod identity",
		"assigned ServiceAccount",
		"ordinary cluster networking",
		"Do not assume node access.",
		"Do not rely on out-of-band credentials.",
		"Prefer evidence over inference.",
		"Use bounded probes only.",
		"Prefer validation.* tools over discovery.* tools",
		"Use discovery.* tools only to resolve a missing namespace, service, secret, or service account name",
		"Use discovery.list_secrets before validation.read_secret",
		"Do not treat ConfigMaps as Secrets",
		"Avoid spending multiple consecutive steps on broad enumeration",
		"Respect guardrails as enforced execution boundaries",
	}

	for _, provider := range providers {
		t.Run(providerOrFallbackName(provider), func(t *testing.T) {
			systemPrompt := RenderValidationPlannerSystemPrompt(provider)
			for _, clause := range requiredClauses {
				if !strings.Contains(systemPrompt, clause) {
					t.Fatalf("system prompt for provider %q missing clause %q:\n%s", provider, clause, systemPrompt)
				}
			}
		})
	}
}

func TestRenderValidationPlannerSystemPromptProviderOverlays(t *testing.T) {
	openAI := RenderValidationPlannerSystemPrompt("openai")
	anthropic := RenderValidationPlannerSystemPrompt("anthropic")
	groq := RenderValidationPlannerSystemPrompt("groq")
	unknown := RenderValidationPlannerSystemPrompt("unknown")

	if !strings.Contains(openAI, "OpenAI/ChatGPT overlay") {
		t.Fatalf("openai prompt missing openai overlay:\n%s", openAI)
	}
	if !strings.Contains(openAI, "Keep prose minimal before tool calls.") {
		t.Fatalf("openai prompt missing minimal prose guidance:\n%s", openAI)
	}
	if strings.Contains(openAI, "one tool_use decision at a time") {
		t.Fatalf("openai prompt should not include anthropic overlay:\n%s", openAI)
	}
	if strings.Contains(openAI, "Prefer bounded tool calls over speculative prose") {
		t.Fatalf("openai prompt should not include groq-only wording:\n%s", openAI)
	}

	if !strings.Contains(anthropic, "Anthropic overlay") {
		t.Fatalf("anthropic prompt missing anthropic overlay:\n%s", anthropic)
	}
	if !strings.Contains(anthropic, "one tool_use decision at a time") {
		t.Fatalf("anthropic prompt missing tool_use guidance:\n%s", anthropic)
	}
	if !strings.Contains(anthropic, "keep the final text concise and evidence-backed") {
		t.Fatalf("anthropic prompt missing concluding text guidance:\n%s", anthropic)
	}
	if strings.Contains(anthropic, "Prefer bounded tool calls over speculative prose") {
		t.Fatalf("anthropic prompt should not include groq-only wording:\n%s", anthropic)
	}

	if !strings.Contains(groq, "Groq overlay") {
		t.Fatalf("groq prompt missing groq overlay:\n%s", groq)
	}
	if !strings.Contains(groq, "OpenAI-compatible function/tool calling discipline") {
		t.Fatalf("groq prompt missing openai-compatible tool guidance:\n%s", groq)
	}
	if !strings.Contains(groq, "Prefer bounded tool calls over speculative prose") {
		t.Fatalf("groq prompt missing guidance about bounded tool calls:\n%s", groq)
	}
	if strings.Contains(groq, "one tool_use decision at a time") {
		t.Fatalf("groq prompt should not include anthropic overlay:\n%s", groq)
	}

	if unknown != openAI {
		t.Fatalf("unknown provider should render same prompt as openai fallback\nunknown:\n%s\nopenai:\n%s", unknown, openAI)
	}
}

func TestRenderValidationPlannerMessages(t *testing.T) {
	messages := RenderValidationPlannerMessages("anthropic", ValidationPlannerInput{
		Goal:      "validate whether the assigned service account can list secrets",
		Mode:      "validation",
		Iteration: 3,
		History:   "- Iteration 1: validation.check_permissions({\"verb\":\"list\",\"resource\":\"secrets\"}) => {\"allowed\":false}",
	})

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "system" {
		t.Fatalf("expected first message role system, got %q", messages[0].Role)
	}
	if messages[1].Role != "user" {
		t.Fatalf("expected second message role user, got %q", messages[1].Role)
	}
	if !strings.Contains(messages[1].Content, "Goal: validate whether the assigned service account can list secrets") {
		t.Fatalf("user prompt missing goal:\n%s", messages[1].Content)
	}
	if !strings.Contains(messages[1].Content, "Mode: validation") {
		t.Fatalf("user prompt missing mode:\n%s", messages[1].Content)
	}
	if !strings.Contains(messages[1].Content, "Iteration: 3") {
		t.Fatalf("user prompt missing iteration:\n%s", messages[1].Content)
	}
	if !strings.Contains(messages[1].Content, "History of executed tools and their results:") {
		t.Fatalf("user prompt missing history header:\n%s", messages[1].Content)
	}
	if !strings.Contains(messages[1].Content, "validation.check_permissions") {
		t.Fatalf("user prompt missing history content:\n%s", messages[1].Content)
	}
}

func providerOrFallbackName(provider string) string {
	if provider == "" {
		return "empty"
	}
	return provider
}
