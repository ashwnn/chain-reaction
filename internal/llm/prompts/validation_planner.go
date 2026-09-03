package prompts

import (
	"fmt"
	"strings"
)

// Provider identifies the prompt overlay style to render.
type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderGroq      Provider = "groq"
)

// Message is a provider-agnostic chat message for prompt assembly.
type Message struct {
	Role    string
	Content string
}

// ValidationPlannerInput contains the typed fields needed to build validation-planner messages.
type ValidationPlannerInput struct {
	Goal      string
	Mode      string
	Iteration int
	History   string
}

// SelectProvider returns the deterministic overlay choice for a provider name.
// Unknown names fall back to the OpenAI-compatible overlay for local safety.
func SelectProvider(name string) Provider {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", string(ProviderOpenAI):
		return ProviderOpenAI
	case string(ProviderAnthropic):
		return ProviderAnthropic
	case string(ProviderGroq):
		return ProviderGroq
	default:
		return ProviderOpenAI
	}
}

// RenderValidationPlannerSystemPrompt renders the shared validation-planner system prompt plus
// the provider-specific overlay guidance.
func RenderValidationPlannerSystemPrompt(providerName string) string {
	selected := SelectProvider(providerName)
	return sharedCorePrompt + "\n\n" + providerOverlayPrompt(selected)
}

// RenderValidationPlannerUserPrompt renders the typed user prompt for the validation planner.
func RenderValidationPlannerUserPrompt(input ValidationPlannerInput) string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "Goal: %s\nMode: %s\nIteration: %d", input.Goal, input.Mode, input.Iteration)

	history := strings.TrimSpace(input.History)
	if history != "" {
		b.WriteString("\n\nHistory of executed tools and their results:\n")
		b.WriteString(history)
	}

	return b.String()
}

// RenderValidationPlannerMessages renders the validation-planner messages in provider-agnostic form.
func RenderValidationPlannerMessages(providerName string, input ValidationPlannerInput) []Message {
	return []Message{
		{
			Role:    "system",
			Content: RenderValidationPlannerSystemPrompt(providerName),
		},
		{
			Role:    "user",
			Content: RenderValidationPlannerUserPrompt(input),
		},
	}
}

const sharedCorePrompt = `You are an automated security testing agent operating from an assumed-breach pod identity inside a Kubernetes cluster.
Operate only as the assigned ServiceAccount for this pod.
Assume ordinary cluster networking only.
Do not assume node access.
Do not rely on out-of-band credentials.
Prefer evidence over inference.
Use bounded probes only.
Prefer validation.* tools over discovery.* tools when you already have a concrete target.
Use discovery.* tools only to resolve a missing namespace, service, secret, or service account name before a bounded validation action.
Use discovery.list_secrets before validation.read_secret when you need a concrete secret target.
Do not treat ConfigMaps as Secrets; objects such as kube-root-ca.crt ConfigMaps do not satisfy secret-read validation.
Avoid spending multiple consecutive steps on broad enumeration if a direct validation action can test the current hypothesis.
Respect guardrails as enforced execution boundaries even when the evidence suggests a tempting next step.
Gather evidence with tools instead of guessing.
If you have verified the goal or reached a dead end, conclude concisely from the evidence you have.`

func providerOverlayPrompt(p Provider) string {
	switch p {
	case ProviderAnthropic:
		return `Anthropic overlay: make one tool_use decision at a time. When concluding, keep the final text concise and evidence-backed.`
	case ProviderGroq:
		return `Groq overlay: follow OpenAI-compatible function/tool calling discipline. Prefer bounded tool calls over speculative prose; act directly when a clear evidence-gathering probe is available.`
	case ProviderOpenAI:
		fallthrough
	default:
		return `OpenAI/ChatGPT overlay: prefer function/tool calling when the next bounded probe is clear. Keep prose minimal before tool calls.`
	}
}
