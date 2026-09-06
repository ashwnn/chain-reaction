package cli

import "testing"

func TestRootCommandIncludesValidateAndTheory(t *testing.T) {
	cmd := NewRootCmd("v", "c", "d")
	if cmd.PersistentFlags().Lookup("config") == nil {
		t.Fatal("expected root command to expose --config")
	}
	validateCmd, _, err := cmd.Find([]string{"validate"})
	if err != nil {
		t.Fatalf("expected validate command to be registered: %v", err)
	}
	if validateCmd == nil || validateCmd.Name() != "validate" {
		t.Fatalf("expected validate command, got %#v", validateCmd)
	}

	scanCmd, _, err := cmd.Find([]string{"scan"})
	if err != nil {
		t.Fatalf("expected scan command to remain registered: %v", err)
	}
	if scanCmd == nil || scanCmd.Name() != "scan" {
		t.Fatalf("expected scan command, got %#v", scanCmd)
	}

	theoryCmd, _, err := cmd.Find([]string{"theory"})
	if err != nil {
		t.Fatalf("expected theory command to be registered: %v", err)
	}
	if theoryCmd == nil || theoryCmd.Name() != "theory" {
		t.Fatalf("expected theory command, got %#v", theoryCmd)
	}

	verifyCmd, _, err := cmd.Find([]string{"verify", "evidence"})
	if err != nil {
		t.Fatalf("expected verify evidence command: %v", err)
	}
	if verifyCmd == nil || verifyCmd.Name() != "evidence" {
		t.Fatalf("expected evidence verifier, got %#v", verifyCmd)
	}

}
