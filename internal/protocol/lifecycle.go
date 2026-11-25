package protocol

type MemoryWriteRequest struct {
	Scope string `json:"scope"`
	Key   string `json:"key"`
	ID    string `json:"id"`
}

type InvalidateRequest struct {
	MemoryWriteRequest
	Reason string `json:"reason"`
	Source string `json:"source"`
}

type HistoryRequest struct {
	Scope         string `json:"scope"`
	ID            string `json:"id"`
	IncludeGlobal bool   `json:"include_global,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	After         string `json:"after,omitempty"`
}

type Invalidation struct {
	Reason string `json:"reason"`
	Source string `json:"source"`
	At     string `json:"at"`
}

type HistoryResult struct {
	Memory       Memory          `json:"memory"`
	Status       string          `json:"status"`
	Proposal     *Proposal       `json:"proposal,omitempty"`
	Invalidation *Invalidation   `json:"invalidation,omitempty"`
	Related      []RelatedMemory `json:"related"`
	NextCursor   string          `json:"next_cursor,omitempty"`
}

type RelatedMemory struct {
	ID     string `json:"id"`
	Scope  string `json:"scope"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Source string `json:"source"`
	Path   Path   `json:"path"`
}

func (r MemoryWriteRequest) Validate() error {
	if err := validateScope(r.Scope); err != nil {
		return err
	}
	if !validText(r.Key, 256) || !validText(r.ID, 256) {
		return NewError("INVALID_REQUEST", "A write key and memory ID are required.")
	}
	return nil
}

func (r InvalidateRequest) Validate() error {
	if err := r.MemoryWriteRequest.Validate(); err != nil {
		return err
	}
	if !validText(r.Reason, maxSourceBytes) || !validText(r.Source, maxSourceBytes) {
		return NewError("INVALID_REQUEST", "Invalidation requires a reason and source.")
	}
	return nil
}

func (r HistoryRequest) Validate() error {
	if err := validateReadScope(r.Scope, r.IncludeGlobal); err != nil {
		return err
	}
	if !validText(r.ID, 256) || r.After != "" && !validText(r.After, 256) || r.Limit < 0 || r.Limit > MaxQueryLimit {
		return NewError("INVALID_REQUEST", "History requires a memory ID, a valid cursor and a limit of at most 100.")
	}
	return nil
}
