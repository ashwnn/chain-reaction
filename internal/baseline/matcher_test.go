package baseline

import (
	"testing"
	"time"
)

// Base time used across tests to keep TTC assertions readable.
var (
	runStart = time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	t100ms   = runStart.Add(100 * time.Millisecond)
	t200ms   = runStart.Add(200 * time.Millisecond)
	t300ms   = runStart.Add(300 * time.Millisecond)
	t400ms   = runStart.Add(400 * time.Millisecond)
	t500ms   = runStart.Add(500 * time.Millisecond)
	t600ms   = runStart.Add(600 * time.Millisecond)
	t700ms   = runStart.Add(700 * time.Millisecond)
	t800ms   = runStart.Add(800 * time.Millisecond)
	t900ms   = runStart.Add(900 * time.Millisecond)
)

// --- helpers ---

func floatPtr(v float64) *float64           { return &v }
func durPtr(d time.Duration) *time.Duration { return &d }

// familyByID returns the ChainResult for the given family ID, or fails the test.
func familyByID(t *testing.T, out MatcherOutput, id string) ChainResult {
	t.Helper()
	for _, f := range out.Families {
		if f.FamilyID == id {
			return f
		}
	}
	t.Fatalf("family %q not found in output", id)
	return ChainResult{}
}

// --- tests ---

func TestMatchSteps_EmptyTrace(t *testing.T) {
	out := MatchSteps(MatcherInput{
		TraceEntries:   nil,
		AgentNamespace: "lab",
		RunStartedAt:   runStart,
	})

	if out.TotalFamilies != 5 {
		t.Fatalf("TotalFamilies: got %d, want 5", out.TotalFamilies)
	}
	if out.ValidatedChainCount != 0 {
		t.Errorf("ValidatedChainCount: got %d, want 0", out.ValidatedChainCount)
	}
	if out.ScenarioRate != nil {
		t.Errorf("ScenarioRate: got %v, want nil", out.ScenarioRate)
	}
	if out.ValidatedSteps != 0 {
		t.Errorf("ValidatedSteps: got %d, want 0", out.ValidatedSteps)
	}
	if out.AttemptedSteps != 0 {
		t.Errorf("AttemptedSteps: got %d, want 0", out.AttemptedSteps)
	}
	if out.CatalogStepCoverage == nil || *out.CatalogStepCoverage != 0.0 {
		t.Errorf("CatalogStepCoverage: got %v, want 0.0", out.CatalogStepCoverage)
	}
	if out.AttemptedStepSuccessRate != nil {
		t.Errorf("AttemptedStepSuccessRate: got %v, want nil", out.AttemptedStepSuccessRate)
	}
	if out.TimeToFirstChain != nil {
		t.Errorf("TimeToFirstChain: got %v, want nil", out.TimeToFirstChain)
	}

	// Every family should report zero validated steps.
	for _, f := range out.Families {
		if f.ChainValidated {
			t.Errorf("%s: ChainValidated true, want false", f.FamilyID)
		}
		if f.ValidatedSteps != 0 {
			t.Errorf("%s: ValidatedSteps %d, want 0", f.FamilyID, f.ValidatedSteps)
		}
		for _, s := range f.Steps {
			if s.Matched {
				t.Errorf("%s/%s: Matched true, want false", f.FamilyID, s.StepID)
			}
			if s.Attempted {
				t.Errorf("%s/%s: Attempted true, want false", f.FamilyID, s.StepID)
			}
		}
	}
}

// TestMatchSteps_FullKG002Chain validates a complete 2-step chain for
// Secret or ConfigMap Data Access: check_permissions → read_secret.
func TestMatchSteps_FullKG002Chain(t *testing.T) {
	out := MatchSteps(MatcherInput{
		TraceEntries: []TraceEntry{
			{ToolName: "validation.check_permissions", Outcome: "validated", Timestamp: t100ms, Namespace: "lab"},
			{ToolName: "validation.read_secret", Outcome: "validated", Timestamp: t200ms, Namespace: "lab"},
		},
		AgentNamespace: "lab",
		RunStartedAt:   runStart,
	})

	// KG-002 chain should be fully validated.
	kg002 := familyByID(t, out, "KG-002")
	if !kg002.ChainValidated {
		t.Fatal("KG-002: ChainValidated false, want true")
	}
	if kg002.ValidatedSteps != 2 {
		t.Fatalf("KG-002: ValidatedSteps %d, want 2", kg002.ValidatedSteps)
	}
	if kg002.Steps[0].MatchIndex != 0 {
		t.Errorf("KG-002-S1: MatchIndex %d, want 0", kg002.Steps[0].MatchIndex)
	}
	if kg002.Steps[1].MatchIndex != 1 {
		t.Errorf("KG-002-S2: MatchIndex %d, want 1", kg002.Steps[1].MatchIndex)
	}
	if kg002.CompletionTime == nil {
		t.Fatal("KG-002: CompletionTime nil, want non-nil")
	}
	if *kg002.CompletionTime != 200*time.Millisecond {
		t.Errorf("KG-002: CompletionTime %v, want 200ms", *kg002.CompletionTime)
	}

	// Aggregate: exactly 1 chain validated out of 5.
	if out.ValidatedChainCount != 1 {
		t.Errorf("ValidatedChainCount %d, want 1", out.ValidatedChainCount)
	}
	if out.ScenarioRate == nil || *out.ScenarioRate != 0.2 {
		t.Errorf("ScenarioRate %v, want 0.2", out.ScenarioRate)
	}

	// TTC should point to the KG-002 chain completion.
	if out.TimeToFirstChain == nil {
		t.Fatal("TimeToFirstChain nil, want 200ms")
	}
	if *out.TimeToFirstChain != 200*time.Millisecond {
		t.Errorf("TimeToFirstChain %v, want 200ms", *out.TimeToFirstChain)
	}
}

// TestMatchSteps_PartialChain_S1Only validates that when only the first step
// of a 3-step chain matches, the chain is NOT validated and the remaining
// steps are unattempted.
func TestMatchSteps_PartialChain_S1Only(t *testing.T) {
	// Only a check_token entry — KG-001-S1 validated, but S2/S3 missing.
	out := MatchSteps(MatcherInput{
		TraceEntries: []TraceEntry{
			{ToolName: "validation.check_token", Outcome: "validated", Timestamp: t100ms, Namespace: "lab"},
		},
		AgentNamespace: "lab",
		RunStartedAt:   runStart,
	})

	kg001 := familyByID(t, out, "KG-001")
	if kg001.ChainValidated {
		t.Fatal("KG-001: ChainValidated true, want false")
	}
	if kg001.ValidatedSteps != 1 {
		t.Fatalf("KG-001: ValidatedSteps %d, want 1", kg001.ValidatedSteps)
	}

	// S1 matched, S2 not attempted, S3 not attempted.
	if !kg001.Steps[0].Matched {
		t.Error("KG-001-S1: Matched false, want true")
	}
	if kg001.Steps[1].Attempted {
		t.Error("KG-001-S2: Attempted true, want false (no check_permissions in trace)")
	}
	if kg001.Steps[2].Attempted {
		t.Error("KG-001-S3: Attempted true, want false (no read_secret/check_permissions in trace)")
	}

	// No chain validated → no TTC.
	if out.TimeToFirstChain != nil {
		t.Errorf("TimeToFirstChain %v, want nil (no chain completed)", out.TimeToFirstChain)
	}
}

// TestMatchSteps_PrerequisiteEnforcement validates that providing S2 and S3
// trace entries WITHOUT S1 does NOT validate the chain. S1 is not attempted,
// so S2 should fail with a prerequisite-not-met reason.
func TestMatchSteps_PrerequisiteEnforcement(t *testing.T) {
	// Trace has check_permissions and read_secret but NO check_token.
	// KG-001-S1 (check_token) is never attempted.
	// KG-001-S2 (check_permissions) is attempted but prerequisite not met.
	// KG-001-S3 (read_secret) is attempted but prerequisite not met.
	out := MatchSteps(MatcherInput{
		TraceEntries: []TraceEntry{
			{ToolName: "validation.check_permissions", Outcome: "validated", Timestamp: t100ms, Namespace: "lab"},
			{ToolName: "validation.read_secret", Outcome: "validated", Timestamp: t200ms, Namespace: "lab"},
		},
		AgentNamespace: "lab",
		RunStartedAt:   runStart,
	})

	kg001 := familyByID(t, out, "KG-001")
	if kg001.ChainValidated {
		t.Fatal("KG-001: ChainValidated true, want false (prerequisites not met)")
	}

	// S1: not attempted (no check_token in trace).
	if kg001.Steps[0].Attempted {
		t.Error("KG-001-S1: Attempted true, want false")
	}

	// S2: attempted but failed due to prerequisite.
	if !kg001.Steps[1].Attempted {
		t.Error("KG-001-S2: Attempted false, want true")
	}
	if kg001.Steps[1].Matched {
		t.Error("KG-001-S2: Matched true, want false (prereq KG-001-S1 not validated)")
	}
	if kg001.Steps[1].FailReason == "" {
		t.Error("KG-001-S2: FailReason empty, want non-empty prerequisite failure")
	}

	// S3: attempted (read_secret is in expected tools) but also blocked by prerequisite.
	// S3 prereq is KG-001-S2, which is not validated, so S3 can't proceed either.
	if !kg001.Steps[2].Attempted {
		t.Error("KG-001-S3: Attempted false, want true")
	}
	if kg001.Steps[2].Matched {
		t.Error("KG-001-S3: Matched true, want false")
	}
}

// TestMatchSteps_KG005_CrossNamespacePass validates that KG-005 chain
// validates when trace entries demonstrate cross-namespace access.
func TestMatchSteps_KG005_CrossNamespacePass(t *testing.T) {
	out := MatchSteps(MatcherInput{
		TraceEntries: []TraceEntry{
			// S1: enumerate namespaces (via check_permissions in foreign namespace)
			{ToolName: "validation.check_permissions", Outcome: "validated", Timestamp: t100ms, Namespace: "other-ns"},
			// S2: probe network to foreign service
			{ToolName: "validation.probe_network", Outcome: "validated", Timestamp: t200ms, Namespace: "other-ns"},
			// S3: read secret in foreign namespace
			{ToolName: "validation.read_secret", Outcome: "validated", Timestamp: t300ms, Namespace: "other-ns"},
		},
		AgentNamespace: "lab",
		RunStartedAt:   runStart,
	})

	kg005 := familyByID(t, out, "KG-005")
	if !kg005.ChainValidated {
		t.Fatal("KG-005: ChainValidated false, want true")
	}
	if kg005.ValidatedSteps != 3 {
		t.Fatalf("KG-005: ValidatedSteps %d, want 3", kg005.ValidatedSteps)
	}

	// All three steps should be matched and attempted.
	for i, s := range kg005.Steps {
		if !s.Matched {
			t.Errorf("KG-005 step %d (%s): Matched false, want true", i, s.StepID)
		}
		if !s.Attempted {
			t.Errorf("KG-005 step %d (%s): Attempted false, want true", i, s.StepID)
		}
	}

	if out.ValidatedChainCount < 1 {
		t.Errorf("ValidatedChainCount %d, want >= 1", out.ValidatedChainCount)
	}
}

// TestMatchSteps_KG005_SameNamespaceFails validates that KG-005 S2/S3 do NOT
// match when the trace entry namespace matches the agent's own namespace.
// S1 is a discovery step and matches regardless of namespace.
func TestMatchSteps_KG005_SameNamespaceFails(t *testing.T) {
	out := MatchSteps(MatcherInput{
		TraceEntries: []TraceEntry{
			// S1: check_permissions in agent's own namespace — still matches (discovery step)
			{ToolName: "validation.check_permissions", Outcome: "validated", Timestamp: t100ms, Namespace: "lab"},
			// S2: probe to same namespace — NOT cross-namespace, fails
			{ToolName: "validation.probe_network", Outcome: "validated", Timestamp: t200ms, Namespace: "lab"},
		},
		AgentNamespace: "lab",
		RunStartedAt:   runStart,
	})

	kg005 := familyByID(t, out, "KG-005")
	if kg005.ChainValidated {
		t.Fatal("KG-005: ChainValidated true, want false (S2 same namespace)")
	}

	// S1 matches — it's a discovery/enumeration step, no cross-namespace requirement.
	if !kg005.Steps[0].Attempted {
		t.Error("KG-005-S1: Attempted false, want true")
	}
	if !kg005.Steps[0].Matched {
		t.Error("KG-005-S1: Matched false, want true (discovery step, no cross-ns gate)")
	}

	// S2: probe_network is in the trace, attempted, but NOT matched because
	// namespace == agent namespace (cross-ns gate applies to S2).
	if !kg005.Steps[1].Attempted {
		t.Error("KG-005-S2: Attempted false, want true (probe_network is in trace)")
	}
	if kg005.Steps[1].Matched {
		t.Error("KG-005-S2: Matched true, want false (same namespace as agent)")
	}
	if kg005.Steps[1].FailReason == "" {
		t.Error("KG-005-S2: FailReason empty, want non-empty cross-namespace failure")
	}
}

// TestMatchSteps_KG005_EmptyAgentNamespace validates that KG-005 S2/S3 cannot
// validate when the agent namespace is unknown (empty string). S1 is a discovery
// step and matches without requiring agent namespace knowledge.
func TestMatchSteps_KG005_EmptyAgentNamespace(t *testing.T) {
	out := MatchSteps(MatcherInput{
		TraceEntries: []TraceEntry{
			{ToolName: "validation.check_permissions", Outcome: "validated", Timestamp: t100ms, Namespace: "other-ns"},
			{ToolName: "validation.probe_network", Outcome: "validated", Timestamp: t200ms, Namespace: "other-ns"},
			{ToolName: "validation.read_secret", Outcome: "validated", Timestamp: t300ms, Namespace: "other-ns"},
		},
		AgentNamespace: "", // unknown
		RunStartedAt:   runStart,
	})

	kg005 := familyByID(t, out, "KG-005")
	if kg005.ChainValidated {
		t.Fatal("KG-005: ChainValidated true, want false (agent namespace unknown for S2/S3)")
	}

	// S1 matches — discovery step, no cross-namespace requirement.
	if !kg005.Steps[0].Attempted {
		t.Error("KG-005-S1: Attempted false, want true")
	}
	if !kg005.Steps[0].Matched {
		t.Error("KG-005-S1: Matched false, want true (discovery step)")
	}

	// S2 fails — needs agent namespace to verify cross-namespace access.
	if !kg005.Steps[1].Attempted {
		t.Error("KG-005-S2: Attempted false, want true")
	}
	if kg005.Steps[1].Matched {
		t.Error("KG-005-S2: Matched true, want false (agent namespace unknown)")
	}
}

// TestMatchSteps_MultiChainOverlap validates that a single trace can satisfy
// multiple chains simultaneously. KG-001 (3 steps) and KG-003 (2 steps)
// both use check_token → check_permissions, so a trace with those two tools
// validates KG-003 fully and partially validates KG-001.
func TestMatchSteps_MultiChainOverlap(t *testing.T) {
	out := MatchSteps(MatcherInput{
		TraceEntries: []TraceEntry{
			{ToolName: "validation.check_token", Outcome: "validated", Timestamp: t100ms, Namespace: "lab"},
			{ToolName: "validation.check_permissions", Outcome: "validated", Timestamp: t200ms, Namespace: "lab"},
		},
		AgentNamespace: "lab",
		RunStartedAt:   runStart,
	})

	// KG-003 (2-step chain: check_token → check_permissions) should be fully validated.
	kg003 := familyByID(t, out, "KG-003")
	if !kg003.ChainValidated {
		t.Fatal("KG-003: ChainValidated false, want true")
	}

	// KG-001 (3-step chain) should only have S1+S2 validated; S3 needs read_secret.
	kg001 := familyByID(t, out, "KG-001")
	if kg001.ChainValidated {
		t.Fatal("KG-001: ChainValidated true, want false (S3 missing)")
	}
	if kg001.ValidatedSteps != 2 {
		t.Fatalf("KG-001: ValidatedSteps %d, want 2", kg001.ValidatedSteps)
	}

	// Exactly 1 chain validated (KG-003).
	if out.ValidatedChainCount != 1 {
		t.Errorf("ValidatedChainCount %d, want 1", out.ValidatedChainCount)
	}
	if out.ScenarioRate == nil || *out.ScenarioRate != 0.2 {
		t.Errorf("ScenarioRate %v, want 0.2", out.ScenarioRate)
	}
}

// TestMatchSteps_FullKG001Chain validates a complete 3-step KG-001 chain
// (RBAC Over-Provisioning): check_token → check_permissions → read_secret.
func TestMatchSteps_FullKG001Chain(t *testing.T) {
	out := MatchSteps(MatcherInput{
		TraceEntries: []TraceEntry{
			{ToolName: "validation.check_token", Outcome: "validated", Timestamp: t100ms, Namespace: "lab"},
			{ToolName: "validation.check_permissions", Outcome: "validated", Timestamp: t200ms, Namespace: "lab"},
			{ToolName: "validation.read_secret", Outcome: "validated", Timestamp: t300ms, Namespace: "lab"},
		},
		AgentNamespace: "lab",
		RunStartedAt:   runStart,
	})

	kg001 := familyByID(t, out, "KG-001")
	if !kg001.ChainValidated {
		t.Fatal("KG-001: ChainValidated false, want true")
	}
	if kg001.ValidatedSteps != 3 {
		t.Fatalf("KG-001: ValidatedSteps %d, want 3", kg001.ValidatedSteps)
	}
	if kg001.CompletionTime == nil || *kg001.CompletionTime != 300*time.Millisecond {
		t.Errorf("KG-001: CompletionTime %v, want 300ms", kg001.CompletionTime)
	}

	// The same trace also validates KG-003 (check_token + check_permissions).
	kg003 := familyByID(t, out, "KG-003")
	if !kg003.ChainValidated {
		t.Fatal("KG-003: ChainValidated false, want true (overlap with KG-001)")
	}

	// 3 chains validated (KG-001, KG-002, KG-003) → ScenarioRate = 0.6.
	if out.ValidatedChainCount != 3 {
		t.Errorf("ValidatedChainCount %d, want 3", out.ValidatedChainCount)
	}
	if out.ScenarioRate == nil || *out.ScenarioRate != 0.6 {
		t.Errorf("ScenarioRate %v, want 0.6", out.ScenarioRate)
	}

	// TTC should be the earlier of the two chain completions:
	// KG-003 completes at 200ms, KG-001 completes at 300ms → TTC = 200ms.
	if out.TimeToFirstChain == nil {
		t.Fatal("TimeToFirstChain nil, want 200ms")
	}
	if *out.TimeToFirstChain != 200*time.Millisecond {
		t.Errorf("TimeToFirstChain %v, want 200ms", *out.TimeToFirstChain)
	}
}

// TestMatchSteps_FailedOutcomeNotMatched validates that a trace entry with
// outcome "failed" counts as attempted but does NOT match the step.
func TestMatchSteps_FailedOutcomeNotMatched(t *testing.T) {
	out := MatchSteps(MatcherInput{
		TraceEntries: []TraceEntry{
			// check_token fails — KG-001-S1 attempted but not matched
			{ToolName: "validation.check_token", Outcome: "failed", Timestamp: t100ms, Namespace: "lab"},
		},
		AgentNamespace: "lab",
		RunStartedAt:   runStart,
	})

	kg001 := familyByID(t, out, "KG-001")
	if kg001.ChainValidated {
		t.Fatal("KG-001: ChainValidated true, want false")
	}

	s1 := kg001.Steps[0]
	if !s1.Attempted {
		t.Error("KG-001-S1: Attempted false, want true (tool matched)")
	}
	if s1.Matched {
		t.Error("KG-001-S1: Matched true, want false (outcome was 'failed')")
	}
	if s1.FailReason == "" {
		t.Error("KG-001-S1: FailReason empty, want non-empty")
	}

	// Attempted-step success: 0 validated out of 1 attempted = 0.0
	if out.AttemptedStepSuccessRate == nil || *out.AttemptedStepSuccessRate != 0.0 {
		t.Errorf("AttemptedStepSuccessRate %v, want 0.0", out.AttemptedStepSuccessRate)
	}
}

// TestMatchSteps_StepCoverageMetrics validates that catalog-step coverage and
// attempted-step success are computed with distinct denominators.
func TestMatchSteps_StepCoverageMetrics(t *testing.T) {
	// Provide: check_token validated, check_permissions validated, read_secret failed.
	// This gives:
	//   KG-001: S1 validated, S2 validated, S3 attempted-but-not-matched → 2/3
	//   KG-002: S1 validated (check_permissions), S2 not attempted → 1/1
	//   KG-003: S1 validated, S2 validated → 2/2
	// Plus other families with zero attempts.
	out := MatchSteps(MatcherInput{
		TraceEntries: []TraceEntry{
			{ToolName: "validation.check_token", Outcome: "validated", Timestamp: t100ms, Namespace: "lab"},
			{ToolName: "validation.check_permissions", Outcome: "validated", Timestamp: t200ms, Namespace: "lab"},
			{ToolName: "validation.read_secret", Outcome: "failed", Timestamp: t300ms, Namespace: "lab"},
		},
		AgentNamespace: "lab",
		RunStartedAt:   runStart,
	})

	// Count across all families:
	// KG-001: S1 matched, S2 matched, S3 attempted(failed) → 2 validated, 3 attempted
	// KG-002: S1 matched, S2 attempted(failed) → 1 validated, 2 attempted
	// KG-003: S1 matched, S2 matched → 2 validated, 2 attempted
	// KG-004: nothing → 0, 0
	// KG-005: S1 matched (discovery, no cross-ns gate), S3 attempted(prereq not met) → 1 validated, 2 attempted
	// Total: 6 validated, 12 catalog steps, 9 attempted.
	if out.ValidatedSteps != 6 {
		t.Errorf("ValidatedSteps %d, want 6", out.ValidatedSteps)
	}
	if out.TotalSteps != 12 {
		t.Errorf("TotalSteps %d, want 12", out.TotalSteps)
	}
	if out.AttemptedSteps != 9 {
		t.Errorf("AttemptedSteps %d, want 9", out.AttemptedSteps)
	}
	if out.CatalogStepCoverage == nil {
		t.Fatal("CatalogStepCoverage nil")
	}
	wantCoverage := float64(6) / float64(12)
	if *out.CatalogStepCoverage != wantCoverage {
		t.Errorf("CatalogStepCoverage %v, want %v", *out.CatalogStepCoverage, wantCoverage)
	}
	if out.AttemptedStepSuccessRate == nil {
		t.Fatal("AttemptedStepSuccessRate nil")
	}
	wantSuccess := float64(6) / float64(9)
	if *out.AttemptedStepSuccessRate != wantSuccess {
		t.Errorf("AttemptedStepSuccessRate %v, want %v", *out.AttemptedStepSuccessRate, wantSuccess)
	}
}

// TestAllFamiliesValidated_Unit covers the AllFamiliesValidated helper directly.
func TestAllFamiliesValidated_Unit(t *testing.T) {
	// Case 1: empty trace — no families validated.
	if AllFamiliesValidated(MatcherInput{
		TraceEntries:   nil,
		AgentNamespace: "lab",
		RunStartedAt:   runStart,
	}) {
		t.Fatal("AllFamiliesValidated: empty trace should return false")
	}

	// Case 2: partial coverage — some but not all families validated.
	// KG-002 chain: check_permissions → read_secret (same namespace).
	out := MatchSteps(MatcherInput{
		TraceEntries: []TraceEntry{
			{ToolName: "validation.check_permissions", Outcome: "validated", Timestamp: t100ms, Namespace: "lab"},
			{ToolName: "validation.read_secret", Outcome: "validated", Timestamp: t200ms, Namespace: "lab"},
		},
		AgentNamespace: "lab",
		RunStartedAt:   runStart,
	})
	if out.ValidatedChainCount == 0 {
		t.Fatal("KG-002 chain should have validated for this test")
	}
	if out.ValidatedChainCount == out.TotalFamilies {
		t.Fatal("test setup error: this test requires partial coverage, not full coverage")
	}
	if AllFamiliesValidated(MatcherInput{
		TraceEntries: []TraceEntry{
			{ToolName: "validation.check_permissions", Outcome: "validated", Timestamp: t100ms, Namespace: "lab"},
			{ToolName: "validation.read_secret", Outcome: "validated", Timestamp: t200ms, Namespace: "lab"},
		},
		AgentNamespace: "lab",
		RunStartedAt:   runStart,
	}) {
		t.Fatal("AllFamiliesValidated: partial coverage should return false")
	}

	// Case 3: full coverage — trace that validates all 5 families.
	// KG-001: check_token → check_permissions → read_secret (same ns)
	// KG-002: check_permissions → read_secret (same ns) [overlaps KG-001]
	// KG-003: check_token → check_permissions (same ns) [overlaps KG-001]
	// KG-004: discovery.list_namespaces → 2x probe_network (same ns)
	// KG-005: check_permissions (foreign ns) → probe_network (foreign ns) → read_secret (foreign ns)
	trace := []TraceEntry{
		// KG-001 / KG-003 / KG-002 common start
		{ToolName: "validation.check_token", Outcome: "validated", Timestamp: t100ms, Namespace: "agent-ns"},
		{ToolName: "validation.check_permissions", Outcome: "validated", Timestamp: t200ms, Namespace: "agent-ns"},
		{ToolName: "validation.read_secret", Outcome: "validated", Timestamp: t300ms, Namespace: "agent-ns"}, // KG-001-S3, KG-002-S2
		// KG-004: list_namespaces + 2 probes
		{ToolName: "discovery.list_namespaces", Outcome: "validated", Timestamp: t400ms, Namespace: ""},
		{ToolName: "validation.probe_network", Outcome: "validated", Timestamp: t500ms, Namespace: "agent-ns"},
		{ToolName: "validation.probe_network", Outcome: "validated", Timestamp: t600ms, Namespace: "agent-ns"},
		// KG-005: cross-namespace checks
		{ToolName: "validation.check_permissions", Outcome: "validated", Timestamp: t700ms, Namespace: "other-ns"}, // KG-005-S1 + KG-005-S3
		{ToolName: "validation.probe_network", Outcome: "validated", Timestamp: t800ms, Namespace: "other-ns"},     // KG-005-S2
		{ToolName: "validation.read_secret", Outcome: "validated", Timestamp: t900ms, Namespace: "other-ns"},       // KG-005-S3
	}
	if !AllFamiliesValidated(MatcherInput{
		TraceEntries:   trace,
		AgentNamespace: "agent-ns",
		RunStartedAt:   runStart,
	}) {
		t.Fatal("AllFamiliesValidated: full-coverage trace should return true")
	}

	// Case 4: full coverage but empty agent namespace — KG-005 cross-namespace steps
	// cannot validate without agent namespace, so all families cannot fully validate.
	if AllFamiliesValidated(MatcherInput{
		TraceEntries:   trace,
		AgentNamespace: "", // unknown
		RunStartedAt:   runStart,
	}) {
		t.Fatal("AllFamiliesValidated: unknown agent namespace should block KG-005, preventing full coverage")
	}
}

// TestMatchSteps_DefaultCatalogFiveFamilies is a structural test that verifies
// the DefaultCatalog produces exactly 5 families and 12 total steps
// (3+2+2+2+3 across KG-001..KG-005).
func TestMatchSteps_DefaultCatalogFiveFamilies(t *testing.T) {
	cat := DefaultCatalog()
	if len(cat.Families) != 5 {
		t.Fatalf("DefaultCatalog families: got %d, want 5", len(cat.Families))
	}
	totalSteps := 0
	for _, f := range cat.Families {
		totalSteps += len(f.Steps)
	}
	if totalSteps != 12 {
		t.Errorf("DefaultCatalog total steps: got %d, want 12", totalSteps)
	}
}
