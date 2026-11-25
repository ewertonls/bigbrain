package protocol

import "testing"

func TestReconcileRequiresAnExplicitSupersedingSide(t *testing.T) {
	request := ReconcileRequest{Scope: "project:B", Key: "review", SupportingID: "A", ContraryID: "B",
		Text: "Conclusion", AppliesWhen: "Under load", Rationale: "Trial comparisons", Author: "Reviewer", Source: "Review 2"}
	for _, tc := range []struct {
		outcome, side string
		valid         bool
	}{
		{"different_contexts", "", true}, {"disputed", "", true}, {"supersedes", "supporting", true}, {"supersedes", "contrary", true},
		{"supersedes", "", false}, {"supersedes", "both", false}, {"different_contexts", "supporting", false},
		{"disputed", "contrary", false}, {"unknown", "", false},
	} {
		request.Outcome, request.SupersedingSide = tc.outcome, tc.side
		if err := request.Validate(); (err == nil) != tc.valid {
			t.Errorf("outcome %q and side %q: %v", tc.outcome, tc.side, err)
		}
	}
	request.Outcome, request.SupersedingSide = "different_contexts", ""
	request.ContraryID = request.SupportingID
	if err := request.Validate(); err == nil {
		t.Fatal("reconciliation accepted the same claim on both sides")
	}
}
