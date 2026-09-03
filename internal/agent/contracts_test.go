package agent

import "testing"

func TestTypedActionValidation(t *testing.T) {
	action := plannerAction{
		ActionType: actionTypeExecute,
		ToolName:   "discovery.list_namespaces",
		Parameters: map[string]any{"namespace": "team-a"},
	}

	if err := action.Validate([]string{"discovery.list_namespaces", "discovery.list_pods"}); err != nil {
		t.Fatalf("expected valid action, got error: %v", err)
	}
}

func TestUnknownToolRejected(t *testing.T) {
	action := plannerAction{
		ActionType: actionTypeExecute,
		ToolName:   "validation.read_secret",
	}

	err := action.Validate([]string{"discovery.list_namespaces"})
	if err == nil {
		t.Fatal("expected unknown tool to be rejected")
	}
	if err.Error() != "unknown tool \"validation.read_secret\"" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInvalidActionKindRejected(t *testing.T) {
	action := plannerAction{ActionType: actionType("plan")}

	err := action.Validate([]string{"discovery.list_namespaces"})
	if err == nil {
		t.Fatal("expected invalid action type to be rejected")
	}
	if err.Error() != "invalid action type \"plan\"" {
		t.Fatalf("unexpected error: %v", err)
	}
}
