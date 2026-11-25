package protocol

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestConsolidationRejectsEmptyExternalInput(t *testing.T) {
	for _, field := range []string{"text", "applies_when", "rationale", "producer", "source"} {
		var r ConsolidateRequest
		if err := json.Unmarshal([]byte(`{"scope":"project:A","key":"attempt","evidence_ids":["a"],"`+field+`":""}`), &r); err != nil {
			t.Fatal(err)
		}
		if err := r.Validate(); err == nil {
			t.Errorf("empty external field %q selected local synthesis", field)
		}
	}
}

func TestConsolidationReceiptEncoding(t *testing.T) {
	// Existing receipt fingerprints hash these JSON bytes, including field order.
	for _, original := range []string{
		`{"scope":"project:A","key":"trial","evidence_ids":["a"]}`,
		`{"scope":"project:A","key":"trial","evidence_ids":["a"],"text":"Retry.","applies_when":"Quota resets","rationale":"Two trials","producer":"agent","source":"agent"}`,
	} {
		var r ConsolidateRequest
		if err := json.Unmarshal([]byte(original), &r); err != nil {
			t.Fatal(err)
		}
		if err := r.Validate(); err != nil {
			t.Fatal(err)
		}
		got, err := json.Marshal(r.Canonical())
		if err != nil || string(got) != original {
			t.Fatalf("receipt payload = %s, %v; want %s", got, err, original)
		}
	}
}

func TestConsolidationEvidenceSet(t *testing.T) {
	ids := make([]string, 100)
	for i := range ids {
		ids[i] = strconv.Itoa(i)
	}
	r := ConsolidateRequest{Scope: "project:A", Key: "trial", EvidenceIDs: append(ids, "1", "2")}
	if err := r.Validate(); err != nil {
		t.Fatalf("100 distinct IDs plus duplicates: %v", err)
	}
	for _, invalid := range [][]string{append(ids, "100"), nil, {" "}, {strings.Repeat("x", 257)}} {
		r.EvidenceIDs = invalid
		if err := r.Validate(); err == nil {
			t.Errorf("accepted invalid evidence set of length %d", len(invalid))
		}
	}
	r.EvidenceIDs = []string{"b", "a", "a"}
	want := ConsolidateRequest{Scope: r.Scope, Key: r.Key, EvidenceIDs: []string{"a", "b"}}
	if got := r.Canonical(); !reflect.DeepEqual(got, want) || !reflect.DeepEqual(r.EvidenceIDs, []string{"b", "a", "a"}) {
		t.Fatalf("canonical = %+v, original = %+v", got, r)
	}
}

func TestExternalProposalFields(t *testing.T) {
	r := ConsolidateRequest{
		Scope: "project:A", Key: "submit", EvidenceIDs: []string{"a"},
		ExternalProposal: &ExternalProposal{Text: "lesson", AppliesWhen: "a retry fails", Rationale: "the trial reproduced it", Producer: "reviewer"},
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	draft := *r.ExternalProposal
	explicit := r
	explicit.ExternalProposal = &draft
	explicit.Source = r.Producer
	if !reflect.DeepEqual(r.Canonical(), explicit.Canonical()) || r.Source != "" {
		t.Fatal("omitted source did not default to producer")
	}
	for _, field := range []*string{&r.Text, &r.AppliesWhen, &r.Rationale, &r.Producer} {
		saved := *field
		*field = ""
		if err := r.Validate(); err == nil {
			t.Errorf("accepted partial external proposal %+v", r)
		}
		*field = saved
	}
	r.Source = " "
	if err := r.Validate(); err == nil {
		t.Error("accepted a blank external source")
	}
	r.Source, r.Scope = "", "global"
	if err := r.Validate(); err == nil {
		t.Error("accepted a global proposal")
	}
	record := RecordRequest{Scope: "project:A", Key: "bypass", Text: r.Text, Type: "lesson_proposal", Source: "review"}
	if err := record.Validate(); err == nil {
		t.Error("record accepted the reserved lesson_proposal type")
	}
}
