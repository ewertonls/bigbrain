package protocol

import "testing"

func TestPublishRequiresApprovalAndPublicContent(t *testing.T) {
	valid := PublishRequest{Scope: "project:A", Key: "review", ProposalID: "P", Text: "Approved lesson", Source: "Public report", Approve: true}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"scope", "key", "proposal", "text", "source", "approve"} {
		r := valid
		switch field {
		case "scope":
			r.Scope = "global"
		case "key":
			r.Key = ""
		case "proposal":
			r.ProposalID = ""
		case "text":
			r.Text = " "
		case "source":
			r.Source = " "
		case "approve":
			r.Approve = false
		}
		if err := r.Validate(); err == nil {
			t.Errorf("accepted publication with invalid %s", field)
		}
	}
	if err := (RecordRequest{Scope: "global", Key: "bypass", Text: valid.Text, Source: valid.Source, Type: "lesson"}).Validate(); err == nil {
		t.Error("record accepted the reserved lesson type")
	}
}
