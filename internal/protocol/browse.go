package protocol

// Browse pages active memories from newest to oldest. Ties use the memory ID.
type BrowseRequest struct {
	Scope         string `json:"scope"`
	IncludeGlobal bool   `json:"include_global,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	After         string `json:"after,omitempty"`
}

type BrowseResult struct {
	Memories   []Memory `json:"memories"`
	NextCursor string   `json:"next_cursor,omitempty"`
}

func (r BrowseRequest) Validate() error {
	if err := validateReadScope(r.Scope, r.IncludeGlobal); err != nil {
		return err
	}
	if r.Limit < 0 || r.Limit > MaxQueryLimit || r.After != "" && !validText(r.After, 256) {
		return NewError("INVALID_REQUEST", "Choose a valid memory cursor and a page of at most 100 memories.")
	}
	return nil
}
