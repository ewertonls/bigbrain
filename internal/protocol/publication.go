package protocol

type PublishRequest struct {
	Scope      string `json:"scope"`
	Key        string `json:"key"`
	ProposalID string `json:"proposal_id"`
	Text       string `json:"text"`
	Source     string `json:"source"`
	Approve    bool   `json:"approve"`
}

func (r PublishRequest) Validate() error {
	if !isProjectScope(r.Scope) {
		return NewError("INVALID_SCOPE", "Publication requires the project that owns the proposal.")
	}
	if !r.Approve {
		return NewError("INVALID_REQUEST", "Explicit approval is required to publish a lesson.")
	}
	if !validText(r.Key, 256) || !validText(r.ProposalID, 256) ||
		!validText(r.Text, MaxMemoryTextBytes) || !validText(r.Source, maxSourceBytes) {
		return NewError("INVALID_REQUEST", "Publication requires a valid key, proposal ID, approved text, and public source.")
	}
	return nil
}
