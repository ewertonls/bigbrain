package protocol

type ReconcileRequest struct {
	Scope           string `json:"scope"`
	Key             string `json:"key"`
	SupportingID    string `json:"supporting_id"`
	ContraryID      string `json:"contrary_id"`
	Outcome         string `json:"outcome"`
	SupersedingSide string `json:"superseding_side,omitempty"`
	Text            string `json:"text"`
	AppliesWhen     string `json:"applies_when"`
	Rationale       string `json:"rationale"`
	Author          string `json:"author"`
	Source          string `json:"source"`
	IncludeGlobal   bool   `json:"include_global,omitempty"`
}

type Reconciliation struct {
	Outcome         string `json:"outcome"`
	SupersedingSide string `json:"superseding_side,omitempty"`
	AppliesWhen     string `json:"applies_when"`
	Rationale       string `json:"rationale"`
	Author          string `json:"author"`
}

func (r ReconcileRequest) Validate() error {
	if !isProjectScope(r.Scope) {
		return NewError("INVALID_SCOPE", "Reconciliation requires a project scope.")
	}
	if !validText(r.Key, 256) || !validText(r.SupportingID, 256) || !validText(r.ContraryID, 256) || r.SupportingID == r.ContraryID {
		return NewError("INVALID_REQUEST", "Reconciliation requires a valid key and two distinct claim IDs.")
	}
	if !validText(r.Text, MaxMemoryTextBytes) || !validText(r.AppliesWhen, maxSourceBytes) ||
		!validText(r.Rationale, maxSourceBytes) || !validText(r.Author, maxSourceBytes) || !validText(r.Source, maxSourceBytes) {
		return NewError("INVALID_REQUEST", "A review requires a conclusion, applicability, rationale, author, and source.")
	}
	switch r.Outcome {
	case "different_contexts", "disputed":
		if r.SupersedingSide != "" {
			return NewError("INVALID_REQUEST", "A superseding side applies only to the supersedes outcome.")
		}
	case "supersedes":
		if r.SupersedingSide != "supporting" && r.SupersedingSide != "contrary" {
			return NewError("INVALID_REQUEST", "Choose supporting or contrary as the superseding side.")
		}
	default:
		return NewError("INVALID_REQUEST", "Choose different_contexts, disputed, or supersedes as the outcome.")
	}
	return nil
}
