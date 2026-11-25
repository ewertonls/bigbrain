package protocol

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	Version           = 1
	MaxBodyBytes      = 1 << 20
	DefaultQueryLimit = 10
	MaxQueryLimit     = 100

	maxMemoryTextBytes = 64 << 10
	maxSourceBytes     = 4 << 10
	maxQueryTextBytes  = 4 << 10

	// A byte may take six bytes after JSON escaping. Allow additional space
	// per match for IDs, terms, and the explanation, plus the envelope.
	MaxResponseBytes = MaxQueryLimit*(6*(maxMemoryTextBytes+2*maxSourceBytes+maxQueryTextBytes)+(16<<10)) + 1024
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func NewError(code, message string) *Error { return &Error{Code: code, Message: message} }

type Response struct {
	Version int             `json:"version"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

func Encode(w io.Writer, result any, err error) error {
	response := Response{Version: Version}
	if err != nil {
		if !errors.As(err, &response.Error) {
			response.Error = NewError("INTERNAL", "The operation failed. Check the daemon log for details.")
		}
	} else {
		value, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return marshalErr
		}
		response.Result = value
	}
	return json.NewEncoder(w).Encode(response)
}

type StatusResult struct {
	Running    bool   `json:"running"`
	DatabaseID string `json:"database_id,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
	PID        int    `json:"pid,omitempty"`
	State      string `json:"state"`
}

type RecordRequest struct {
	Scope  string `json:"scope"`
	Key    string `json:"key"`
	Text   string `json:"text"`
	Source string `json:"source"`
	Type   string `json:"type,omitempty"`
}

type LinkRequest struct {
	Scope  string  `json:"scope"`
	Key    string  `json:"key"`
	FromID string  `json:"from_id"`
	ToID   string  `json:"to_id"`
	Kind   string  `json:"type"`
	Source string  `json:"source"`
	Weight float64 `json:"weight"`
}

type QueryRequest struct {
	Scope         string `json:"scope"`
	Text          string `json:"text,omitempty"`
	ID            string `json:"id,omitempty"`
	Mode          string `json:"mode,omitempty"`
	IncludeGlobal bool   `json:"include_global,omitempty"`
	Limit         int    `json:"limit,omitempty"`
}

type WriteResult struct {
	ID       string `json:"id"`
	Replayed bool   `json:"replayed"`
}

type QueryResult struct {
	Matches []Match `json:"matches"`
	Count   int     `json:"count"`
}

type Memory struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Scope      string `json:"scope"`
	Text       string `json:"text"`
	Source     string `json:"source"`
	RecordedAt string `json:"recorded_at"`
}

type Match struct {
	Memory
	Reason Reason `json:"reason"`
}

type Reason struct {
	Kind  string   `json:"kind"`
	Terms []string `json:"terms,omitempty"`
	Path  *Path    `json:"path,omitempty"`
}

type Path struct {
	RelationshipID string  `json:"relationship_id"`
	FromID         string  `json:"from_id"`
	ToID           string  `json:"to_id"`
	Kind           string  `json:"type"`
	Source         string  `json:"source"`
	Direction      string  `json:"direction"`
	Weight         float64 `json:"weight"`
}

var projectID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func ValidProjectID(id string) bool { return projectID.MatchString(id) }

func validateScope(scope string) error {
	id, project := strings.CutPrefix(scope, "project:")
	if scope == "global" || project && ValidProjectID(id) {
		return nil
	}
	return NewError("INVALID_SCOPE", "Choose an explicit project or global scope.")
}

func validText(value string, maxBytes int) bool {
	return len(value) <= maxBytes && strings.TrimSpace(value) != "" &&
		utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}

func (r RecordRequest) Validate() error {
	if err := validateScope(r.Scope); err != nil {
		return err
	}
	if !validText(r.Key, 256) || !validText(r.Text, maxMemoryTextBytes) || !validText(r.Source, maxSourceBytes) {
		return NewError("INVALID_REQUEST", "Record requires a valid key, text, and source.")
	}
	switch r.Type {
	case "", "semantic", "episodic", "procedural", "decision", "preference", "entity", "observation":
	default:
		return NewError("INVALID_REQUEST", "Unknown memory type.")
	}
	return nil
}

func (r LinkRequest) Validate() error {
	if err := validateScope(r.Scope); err != nil {
		return err
	}
	if !validText(r.Key, 256) || !validText(r.FromID, 256) || !validText(r.ToID, 256) ||
		!validText(r.Kind, 256) || !validText(r.Source, maxSourceBytes) ||
		math.IsNaN(r.Weight) || math.IsInf(r.Weight, 0) || r.Weight < 0 {
		return NewError("INVALID_REQUEST", "Link requires a valid key, both endpoint IDs, a type, a source, and a nonnegative finite weight.")
	}
	return nil
}

func (r QueryRequest) Validate() error {
	if err := validateScope(r.Scope); err != nil {
		return err
	}
	if r.IncludeGlobal && r.Scope == "global" {
		return NewError("INVALID_SCOPE", "Global inclusion requires a project scope.")
	}
	if (r.ID == "") == (r.Text == "") || r.ID != "" && !validText(r.ID, 256) ||
		r.Text != "" && !validText(r.Text, maxQueryTextBytes) {
		return NewError("INVALID_REQUEST", "Query requires either valid search text or a memory ID.")
	}
	if r.Mode != "" && r.Mode != "text" && r.Mode != "associative" {
		return NewError("INVALID_REQUEST", "Query mode must be text or associative.")
	}
	if r.Limit < 0 || r.Limit > MaxQueryLimit {
		return NewError("INVALID_REQUEST", "Query limit must be between 1 and 100.")
	}
	return nil
}
