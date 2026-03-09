# Agent Architecture Specification (Draft)

## 1. Overview

This document specifies the architecture for Chain Reaction's LLM-guided Kubernetes attack chain validation agent.

**Goal:** Autonomous in-cluster agent that discovers resources, validates attack chains, and produces evidence-backed attack graphs.

**Core Pattern:** ReAct (Reasoning + Acting) with bounded execution and comprehensive evidence collection.

## 2. Component Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Agent Runner                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   Planner   │  │   State     │  │   Termination Checker   │  │
│  │  (LLM)      │  │   Manager   │  │                         │  │
│  └──────┬──────┘  └──────┬──────┘  └───────────┬─────────────┘  │
│         │                │                     │                │
│         ▼                ▼                     ▼                │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                    ReAct Loop                              │  │
│  │  Plan → Action → Observe → Update State → Repeat          │  │
│  └───────────────────────────────────────────────────────────┘  │
│         │                │                     │                │
│         ▼                ▼                     ▼                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ Tool Exec   │  │  Evidence   │  │   Attack Graph          │  │
│  │  (Registry) │  │ Collector   │  │   Builder               │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## 3. Component Specifications

### 3.1 Agent Runner (`internal/agent/runner.go`)

**Responsibility:** Orchestrates the ReAct loop with guardrails and termination checking.

**Key Types:**

```go
type Runner struct {
    planner     llm.Planner
    registry    *tools.Registry
    enforcer    *guardrails.Enforcer
    collector   *evidence.Collector
    graph       *graph.Builder
    maxIter     int
    timeout     time.Duration
}

type RunConfig struct {
    Goal          string
    MaxIterations int
    Timeout       time.Duration
    InitialContext map[string]any
}

type RunResult struct {
    RunID         string
    Graph         *graph.AttackGraph
    EvidencePath  string
    Steps         int
    Termination   string
    Duration      time.Duration
}
```

### 3.2 State Manager (`internal/agent/state.go`)

**Responsibility:** Maintains execution state across ReAct iterations.

```go
type State struct {
    mu              sync.RWMutex
    Iteration       int
    Goal            string
    History         []Step
    Context         map[string]any
    AttackGraph     *graph.AttackGraph
    StartTime       time.Time
}

type Step struct {
    Iteration   int
    Thought     string
    Action      Action
    Observation Observation
    Timestamp   time.Time
}

type Action struct {
    Thought     string         `json:"thought"`
    ToolName    string         `json:"tool_name"`
    Parameters  map[string]any `json:"parameters"`
    ActionType  ActionType     `json:"action_type"`
}

type Observation struct {
    ToolName   string         `json:"tool_name"`
    Input      map[string]any `json:"input"`
    Output     map[string]any `json:"output"`
    Success    bool           `json:"success"`
    Error      string         `json:"error,omitempty"`
    Timestamp  time.Time      `json:"timestamp"`
    Duration   time.Duration  `json:"duration_ms"`
}

type ActionType string

const (
    ActionExecute      ActionType = "execute"
    ActionFinalAnswer  ActionType = "final_answer"
)
```

### 3.3 Termination Checker (`internal/agent/termination.go`)

**Responsibility:** Determines when to stop the ReAct loop.

```go
type TerminationChecker struct {
    MaxIterations   int
    Timeout         time.Duration
    ErrorThreshold  int
}

func (tc *TerminationChecker) ShouldStop(state *State) (bool, string)

// Stop Reasons:
// - "goal_achieved" - Agent returned final answer
// - "max_iterations_reached" - Hit iteration limit
// - "timeout" - Exceeded time budget
// - "no_progress" - Stuck in repetitive loop
// - "guardrail_stop" - Guardrail enforced stop
```

## 4. Tool System (`internal/tools/`)

### 4.1 Tool Interface

```go
type Tool interface {
    Name() string
    Description() string
    Schema() ToolSchema
    Run(ctx context.Context, input map[string]any) (ToolResult, error)
}

type ToolSchema struct {
    Type       string                 `json:"type"`
    Properties map[string]Property    `json:"properties"`
    Required   []string               `json:"required"`
    Strict     bool                   `json:"strict"`
}

type Property struct {
    Type        string   `json:"type"`
    Description string   `json:"description"`
    Enum        []string `json:"enum,omitempty"`
}

type ToolResult struct {
    Success  bool              `json:"success"`
    Data     map[string]any    `json:"data,omitempty"`
    Error    *ToolError        `json:"error,omitempty"`
    Metadata ResultMetadata    `json:"metadata"`
}

type ToolError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

### 4.2 Tool Categories

**Discovery Tools:**
- `discovery.list_namespaces` - List all namespaces
- `discovery.list_pods` - List pods in namespace
- `discovery.list_serviceaccounts` - List service accounts
- `discovery.list_roles` - List RBAC roles
- `discovery.list_secrets` - List secrets (metadata only)

**Validation Tools:**
- `validation.check_permissions` - Check RBAC permissions
- `validation.probe_network` - Test network reachability
- `validation.read_secret` - Read secret content (with guardrails)

**Introspection Tools:**
- `introspection.get_current_identity` - Get current ServiceAccount
- `introspection.get_effective_permissions` - Summarize permissions

## 5. Evidence Collection System (`internal/evidence/`)

### 5.1 Evidence Event Schema

See: https://github.com/ashwnn/chain-reaction/blob/main/docs/schemas/evidence-event.json

```go
type EvidenceRecord struct {
    EvidenceID   string      `json:"evidence_id"`
    Timestamp    time.Time   `json:"timestamp"`
    EventType    EventType   `json:"event_type"`
    RunID        string      `json:"run_id"`
    Severity     Severity    `json:"severity"`
    Source       SourceInfo  `json:"source"`
    Target       *ResourceInfo `json:"target,omitempty"`
    Payload      map[string]any `json:"payload"`
    Integrity    IntegrityInfo  `json:"integrity"`
}

type EventType string

const (
    EventTypeAPICall       EventType = "api_call"
    EventTypeToolExecution EventType = "tool_execution"
    EventTypeObservation   EventType = "observation"
    EventTypeValidation    EventType = "validation_result"
    EventTypeGuardrail     EventType = "guardrail_triggered"
    EventTypeLLMDecision   EventType = "llm_decision"
)
```

### 5.2 Evidence Collector

```go
type Collector struct {
    runID        string
    outputDir    string
    evidenceFile *os.File
    encoder      *json.Encoder
    lastHash     string
    recordCount  int
}

func (c *Collector) Record(ctx context.Context, eventType EventType, severity Severity, payload map[string]any) (*EvidenceRecord, error)
func (c *Collector) RecordAPICall(ctx context.Context, method, path string, statusCode int, latencyMs int64, rbac map[string]any) (*EvidenceRecord, error)
func (c *Collector) RecordToolExecution(ctx context.Context, toolName string, input, output map[string]any, execTimeMs int64, err error) (*EvidenceRecord, error)
func (c *Collector) RecordValidation(ctx context.Context, edgeID string, result string, confidence float64, evidenceRefs []string, details map[string]any) (*EvidenceRecord, error)
func (c *Collector) Close() error
```

## 6. Attack Graph System (`internal/graph/`)

### 6.1 Graph Schema

See: https://github.com/ashwnn/chain-reaction/blob/main/docs/schemas/attack-graph.json

```go
type AttackGraph struct {
    Metadata GraphMetadata `json:"metadata"`
    Nodes    []Node        `json:"nodes"`
    Edges    []Edge        `json:"edges"`
    Paths    []AttackPath  `json:"paths"`
}

type Node struct {
    ID         string            `json:"id"`
    Kind       string            `json:"kind"`
    Name       string            `json:"name"`
    Namespace  string            `json:"namespace,omitempty"`
    Properties map[string]any    `json:"properties"`
    Compromised bool           `json:"compromised"`
}

type Edge struct {
    ID         string            `json:"id"`
    Source     string            `json:"source"`
    Target     string            `json:"target"`
    Type       string            `json:"type"`
    Status     ValidationStatus  `json:"status"`
    EvidenceID string            `json:"evidence_id,omitempty"`
    Severity   Severity          `json:"severity"`
}

type ValidationStatus string

const (
    StatusValidated  ValidationStatus = "validated"
    StatusTheoretical ValidationStatus = "theoretical"
    StatusFailed     ValidationStatus = "failed"
)

type AttackPath struct {
    ID         string   `json:"id"`
    Nodes      []string `json:"nodes"`
    Edges      []string `json:"edges"`
    Validated  bool     `json:"validated"`
    Severity   Severity `json:"severity"`
}
```

### 6.2 Graph Builder

```go
type Builder struct {
    graph *AttackGraph
    mu    sync.RWMutex
}

func (b *Builder) AddNode(node Node) string
func (b *Builder) AddEdge(edge Edge) string
func (b *Builder) ValidateEdge(edgeID string, evidenceID string) error
func (b *Builder) Export(format ExportFormat) ([]byte, error)
```

### 6.3 Export Formats

- **JSON** - Native graph format
- **DOT** - Graphviz for static rendering
- **Cytoscape** - Web visualization
- **Mermaid** - Markdown embedding

## 7. LLM Integration (`internal/llm/`)

### 7.1 Planner Interface

```go
type Planner interface {
    SuggestNextTool(ctx context.Context, state State, availableTools []tools.Tool) (Action, error)
}

type OpenAIPlanner struct {
    client *openai.Client
    model  string
}

type AnthropicPlanner struct {
    client *anthropic.Client
    model  string
}
```

### 7.2 ReAct Prompt Template

```
You are a Kubernetes security agent using the ReAct pattern.

Goal: {{.Goal}}

Available Tools:
{{range .Tools}}
- {{.Name}}: {{.Description}}
{{end}}

Execution History:
{{range .History}}
Step {{.Iteration}}:
Thought: {{.Thought}}
Action: {{.Action.ToolName}}({{.Action.Parameters}})
Observation: {{.Observation.Output}}
{{end}}

Respond with JSON:
{
    "thought": "Your reasoning about next action",
    "tool_name": "tool to call or 'final_answer'",
    "parameters": { /* tool inputs */ },
    "action_type": "execute" | "final_answer"
}
```

## 8. Guardrails Integration

The guardrails system (defined in https://github.com/ashwnn/chain-reaction/blob/main/docs/draft/guardrails-spec.md) integrates at multiple points:

1. **Pre-execution:** Check if action is in allow-list
2. **Rate limiting:** Enforce QPS/burst limits
3. **Stop conditions:** Monitor iteration count, time budget
4. **Evidence:** Log all guardrail triggers

## 9. Data Flow

```
1. Runner initializes with goal and context
2. ReAct Loop:
   a. Planner suggests next action (LLM call)
   b. Guardrails validate action
   c. Tool executes with evidence collection
   d. Observation recorded
   e. State updated
   f. Attack graph updated
   g. Termination checker evaluated
3. Loop continues until termination
4. Final artifacts exported (graph + evidence bundle)
```

## 10. File Organization

```
internal/
├── agent/
│   ├── runner.go         # Main orchestration
│   ├── state.go          # State management
│   ├── termination.go    # Stop conditions
│   └── react.go          # ReAct loop logic
├── tools/
│   ├── interface.go      # Tool interface
│   ├── registry.go       # Tool registration
│   ├── executor.go       # Tool execution
│   ├── discovery/        # Discovery tools
│   └── validation/       # Validation tools
├── evidence/
│   ├── collector.go      # Evidence collection
│   ├── types.go          # Evidence types
│   └── export.go         # Bundle export
├── graph/
│   ├── builder.go        # Graph construction
│   ├── types.go          # Graph types
│   └── export.go         # Format exporters
└── llm/
    ├── planner.go        # Planner interface
    ├── openai.go         # OpenAI implementation
    └── prompts.go        # Prompt templates
```

## 11. Acceptance Criteria

- [x] Architecture diagram + component responsibilities (this document)
- [x] Initial JSON schemas for evidence events
- [x] Initial JSON schemas for graph nodes/edges
- [x] Data contracts between components defined
- [x] Integration points with guardrails specified

## 12. References

- ReAct Pattern: Reasoning + Acting loop for LLM agents
- Guardrails Spec: https://github.com/ashwnn/chain-reaction/blob/main/docs/draft/guardrails-spec.md
- Threat Model: https://github.com/ashwnn/chain-reaction/blob/main/docs/draft/threat-model.md
