package protocol

import (
	"math"
	"time"
)

type FeedbackTarget struct {
	Scope          string `json:"scope"`
	Key            string `json:"key"`
	RelationshipID string `json:"relationship_id"`
	Author         string `json:"author"`
	Source         string `json:"source"`
	IncludeGlobal  bool   `json:"include_global,omitempty"`
}

type FeedbackRequest struct {
	FeedbackTarget
	EvidenceID string   `json:"evidence_id,omitempty"`
	Confidence *float64 `json:"confidence,omitempty"`
}

type ForgetRequest struct {
	FeedbackTarget
	Before string `json:"before"`
}

type ReviewWeightRequest struct {
	FeedbackTarget
	Weight  *float64 `json:"weight"`
	Approve bool     `json:"approve"`
}

type FeedbackHistoryRequest struct {
	Scope          string `json:"scope"`
	RelationshipID string `json:"relationship_id"`
	IncludeGlobal  bool   `json:"include_global,omitempty"`
	After          string `json:"after,omitempty"`
	Limit          int    `json:"limit,omitempty"`
}

type FeedbackHistoryResult struct {
	Events     []FeedbackEvent `json:"events"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

type FeedbackEvent struct {
	ID              string   `json:"id"`
	Scope           string   `json:"scope"`
	RelationshipID  string   `json:"relationship_id"`
	Action          string   `json:"action"`
	Author          string   `json:"author"`
	Source          string   `json:"source"`
	EvidenceID      string   `json:"evidence_id,omitempty"`
	RecordedAt      string   `json:"recorded_at"`
	Weight          float64  `json:"weight"`
	Confidence      *float64 `json:"confidence,omitempty"`
	ActivationCount *int64   `json:"activation_count,omitempty"`
	LastActivatedAt string   `json:"last_activated_at,omitempty"`
	Cutoff          string   `json:"cutoff,omitempty"`
}

func (r FeedbackTarget) validate(global bool) error {
	if err := validateReadScope(r.Scope, r.IncludeGlobal); err != nil {
		return err
	}
	if global != (r.Scope == "global") {
		return NewError("INVALID_SCOPE", "Feedback and forgetting require a project. Reviewed global weights require the global scope.")
	}
	if !validText(r.Key, 256) || !validText(r.RelationshipID, 256) || !validText(r.Author, maxSourceBytes) || !validText(r.Source, maxSourceBytes) {
		return NewError("INVALID_REQUEST", "Provide a write key, relationship ID, author and source.")
	}
	return nil
}

func (r FeedbackRequest) Validate() error {
	if err := r.FeedbackTarget.validate(false); err != nil {
		return err
	}
	if r.EvidenceID != "" && !validText(r.EvidenceID, 256) {
		return NewError("INVALID_REQUEST", "Provide a valid evidence memory ID.")
	}
	if c := r.Confidence; c != nil && (math.IsNaN(*c) || *c < 0 || *c > 1 || r.EvidenceID == "") {
		return NewError("INVALID_REQUEST", "Changing confidence requires an evidence memory ID and a confidence between 0 and 1.")
	}
	return nil
}

func (r ForgetRequest) Validate() error {
	if err := r.FeedbackTarget.validate(false); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339Nano, r.Before); err != nil {
		return NewError("INVALID_REQUEST", "Forgetting requires a cutoff timestamp in RFC3339 format.")
	}
	return nil
}

func (r ReviewWeightRequest) Validate() error {
	if err := r.FeedbackTarget.validate(true); err != nil {
		return err
	}
	if !r.Approve || r.Weight == nil || *r.Weight < 0 || math.IsNaN(*r.Weight) || math.IsInf(*r.Weight, 0) {
		return NewError("INVALID_REQUEST", "A global weight review requires approval and a finite nonnegative weight.")
	}
	return nil
}

func (r FeedbackHistoryRequest) Validate() error {
	if err := validateReadScope(r.Scope, r.IncludeGlobal); err != nil {
		return err
	}
	if !validText(r.RelationshipID, 256) || r.After != "" && !validText(r.After, 256) || r.Limit < 0 || r.Limit > MaxQueryLimit {
		return NewError("INVALID_REQUEST", "Feedback history requires a relationship ID, a valid cursor and a limit of at most 100.")
	}
	return nil
}
