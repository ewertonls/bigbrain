package protocol

import (
	"math"
	"testing"
)

func TestFeedbackWriteRequirements(t *testing.T) {
	project := FeedbackTarget{Scope: "project:A", Key: "key", RelationshipID: "link", Author: "reviewer", Source: "review"}
	global := project
	global.Scope = "global"
	for _, r := range []interface{ Validate() error }{
		FeedbackRequest{FeedbackTarget: global},
		FeedbackRequest{FeedbackTarget: project, Confidence: new(0.7)},
		FeedbackRequest{FeedbackTarget: project, EvidenceID: "trial", Confidence: new(math.NaN())},
		ForgetRequest{FeedbackTarget: project, Before: "yesterday"},
		ForgetRequest{FeedbackTarget: global, Before: "2050-01-01T00:00:00Z"},
		ReviewWeightRequest{FeedbackTarget: project, Weight: new(2.0), Approve: true},
		ReviewWeightRequest{FeedbackTarget: global, Weight: new(2.0)},
		ReviewWeightRequest{FeedbackTarget: global, Approve: true},
		ReviewWeightRequest{FeedbackTarget: global, Approve: true, Weight: new(math.Inf(1))},
	} {
		if err := r.Validate(); err == nil {
			t.Errorf("accepted invalid request: %+v", r)
		}
	}
	for _, r := range []interface{ Validate() error }{
		FeedbackRequest{FeedbackTarget: project},
		FeedbackRequest{FeedbackTarget: project, EvidenceID: "trial", Confidence: new(0.0)},
		ReviewWeightRequest{FeedbackTarget: global, Approve: true, Weight: new(0.0)},
	} {
		if err := r.Validate(); err != nil {
			t.Errorf("valid request %+v: %v", r, err)
		}
	}
}
