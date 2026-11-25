package protocol

import "slices"

type ConsolidateRequest struct {
	Scope       string   `json:"scope"`
	Key         string   `json:"key"`
	EvidenceIDs []string `json:"evidence_ids"`
	// Any supplied draft field selects external mode, even if its value is empty.
	*ExternalProposal
}

type ExternalProposal struct {
	Text        string `json:"text"`
	AppliesWhen string `json:"applies_when"`
	Rationale   string `json:"rationale"`
	Producer    string `json:"producer"`
	Source      string `json:"source,omitempty"`
}

type Proposal struct {
	Origin      string `json:"origin"`
	Producer    string `json:"producer"`
	AppliesWhen string `json:"applies_when"`
	Rationale   string `json:"rationale"`
}

func (r ConsolidateRequest) Validate() error {
	if !isProjectScope(r.Scope) {
		return NewError("INVALID_SCOPE", "Consolidation requires a project scope.")
	}
	if !validText(r.Key, 256) {
		return NewError("INVALID_REQUEST", "Consolidation requires a valid write key.")
	}
	seen := make(map[string]struct{})
	for _, id := range r.EvidenceIDs {
		if !validText(id, 256) {
			return NewError("INVALID_REQUEST", "Each evidence ID must be valid and at most 256 bytes.")
		}
		seen[id] = struct{}{}
		if len(seen) > 100 {
			return NewError("INVALID_REQUEST", "Consolidation accepts at most 100 distinct evidence IDs.")
		}
	}
	if len(seen) == 0 {
		return NewError("INVALID_REQUEST", "Consolidation requires at least one evidence ID.")
	}
	if r.ExternalProposal == nil {
		return nil
	}
	if !validText(r.Text, MaxMemoryTextBytes) || !validText(r.AppliesWhen, maxSourceBytes) ||
		!validText(r.Rationale, maxSourceBytes) || !validText(r.Producer, maxSourceBytes) {
		return NewError("INVALID_REQUEST", "An external proposal requires text, applicability, rationale, and a producer.")
	}
	if r.Source != "" && !validText(r.Source, maxSourceBytes) {
		return NewError("INVALID_REQUEST", "Proposal source must be valid and at most 4096 bytes.")
	}
	return nil
}

// Canonical returns the request used for the receipt and its write.
func (r ConsolidateRequest) Canonical() ConsolidateRequest {
	r.EvidenceIDs = slices.Clone(r.EvidenceIDs)
	slices.Sort(r.EvidenceIDs)
	r.EvidenceIDs = slices.Compact(r.EvidenceIDs)
	if r.ExternalProposal != nil && r.Source == "" {
		draft := *r.ExternalProposal
		draft.Source = draft.Producer
		r.ExternalProposal = &draft
	}
	return r
}
