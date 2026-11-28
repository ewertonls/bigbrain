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
	Version             = 1
	MaxBodyBytes        = 1 << 20
	DefaultQueryLimit   = 10
	MaxQueryLimit       = 100
	DefaultMaxDepth     = 1
	MaxGraphDepth       = 3
	DefaultMaxNeighbors = 5
	MaxGraphNeighbors   = 10

	MaxMemoryTextBytes = 64 << 10
	maxSourceBytes     = 4 << 10
	maxQueryTextBytes  = 4 << 10

	// JSON escaping can expand each input byte to six bytes. Each path entry
	// also carries IDs, type, scope, direction, timestamps and numeric fields.
	maxEncodedPathBytes = 6*maxSourceBytes + (8 << 10)
	// A match may include every step and repeat the terminal step in path.
	maxEncodedMatchBytes = 6*(MaxMemoryTextBytes+maxSourceBytes+maxQueryTextBytes) + (16 << 10) +
		(MaxGraphDepth+1)*maxEncodedPathBytes
	MaxResponseBytes = MaxQueryLimit*maxEncodedMatchBytes + 1024
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
	Scope      string   `json:"scope"`
	Key        string   `json:"key"`
	FromID     string   `json:"from_id"`
	ToID       string   `json:"to_id"`
	Kind       string   `json:"type"`
	Source     string   `json:"source"`
	Weight     float64  `json:"weight"`
	Confidence *float64 `json:"confidence,omitempty"`
}

type QueryRequest struct {
	Scope         string `json:"scope"`
	Text          string `json:"text,omitempty"`
	ID            string `json:"id,omitempty"`
	Mode          string `json:"mode,omitempty"`
	IncludeGlobal bool   `json:"include_global,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	MaxDepth      *int   `json:"max_depth,omitempty"`
	MaxNeighbors  *int   `json:"max_neighbors,omitempty"`
}

type WriteResult struct {
	ID       string `json:"id"`
	Replayed bool   `json:"replayed"`
}

// IndexProgress counts committed cache changes for one phase and visible scope.
type IndexProgress struct {
	Phase     string `json:"phase"`
	Scope     string `json:"scope"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
}

type QueryResult struct {
	Fallback string  `json:"fallback,omitempty"`
	Matches  []Match `json:"matches"`
	Count    int     `json:"count"`
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
	Score      float64  `json:"score,omitempty"`
	GraphScore float64  `json:"graph_score,omitempty"`
	BM25       float64  `json:"bm25,omitempty"`
	Cosine     float64  `json:"cosine,omitempty"`
	Kind       string   `json:"kind"`
	Terms      []string `json:"terms,omitempty"`
	Path       *Path    `json:"path,omitempty"`
	Hops       int      `json:"hops,omitempty"`
	Steps      []Path   `json:"steps,omitempty"`
}

type Path struct {
	Scope           string   `json:"scope"`
	RelationshipID  string   `json:"relationship_id"`
	FromID          string   `json:"from_id"`
	ToID            string   `json:"to_id"`
	Kind            string   `json:"type"`
	Source          string   `json:"source"`
	Direction       string   `json:"direction"`
	Weight          float64  `json:"weight"`
	Confidence      *float64 `json:"confidence,omitempty"`
	ActivationCount *int64   `json:"activation_count,omitempty"`
	LastActivatedAt string   `json:"last_activated_at,omitempty"`
	CreatedAt       string   `json:"created_at,omitempty"`
}

var projectID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func ValidProjectID(id string) bool { return projectID.MatchString(id) }

func isProjectScope(scope string) bool {
	id, project := strings.CutPrefix(scope, "project:")
	return project && ValidProjectID(id)
}

func validateScope(scope string) error {
	if scope == "global" || isProjectScope(scope) {
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
	if !validText(r.Key, 256) || !validText(r.Text, MaxMemoryTextBytes) || !validText(r.Source, maxSourceBytes) {
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
	if c := r.Confidence; c != nil && (math.IsNaN(*c) || *c < 0 || *c > 1) {
		return NewError("INVALID_REQUEST", "Link confidence must be a finite number between 0 and 1.")
	}
	return nil
}

func validateReadScope(scope string, includeGlobal bool) error {
	if err := validateScope(scope); err != nil {
		return err
	}
	if includeGlobal && scope == "global" {
		return NewError("INVALID_SCOPE", "Global inclusion requires a project scope.")
	}
	return nil
}

func (r QueryRequest) Validate() error {
	if err := validateReadScope(r.Scope, r.IncludeGlobal); err != nil {
		return err
	}
	if (r.ID == "") == (r.Text == "") || r.ID != "" && !validText(r.ID, 256) ||
		r.Text != "" && !validText(r.Text, maxQueryTextBytes) {
		return NewError("INVALID_REQUEST", "Query requires either valid search text or a memory ID.")
	}
	if r.Mode != "" && r.Mode != "text" && r.Mode != "associative" && r.Mode != "hybrid" {
		return NewError("INVALID_REQUEST", "Query mode must be text, associative or hybrid.")
	}
	if r.Limit < 0 || r.Limit > MaxQueryLimit {
		return NewError("INVALID_REQUEST", "Query limit must be between 1 and 100.")
	}
	if (r.MaxDepth != nil || r.MaxNeighbors != nil) && (r.ID != "" || r.Mode == "text") {
		return NewError("INVALID_REQUEST", "Traversal limits require a hybrid or associative text query.")
	}
	if depth := r.MaxDepth; depth != nil && (*depth < 1 || *depth > MaxGraphDepth) {
		return NewError("INVALID_REQUEST", "Maximum traversal depth must be between 1 and 3.")
	}
	if neighbors := r.MaxNeighbors; neighbors != nil && (*neighbors < 1 || *neighbors > MaxGraphNeighbors) {
		return NewError("INVALID_REQUEST", "Maximum neighbors per node must be between 1 and 10.")
	}
	return nil
}
