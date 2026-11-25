package store

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

const originalEvidence = `
UNWIND CAST($ids AS STRING[]) AS evidenceID
MATCH (e:Memory {id: evidenceID, scope: $scope, status: 'active'})
WHERE NOT (e.memory_type IN ['lesson_proposal', 'reconciliation'])`

func unavailableEvidence() error {
	return protocol.NewError("NOT_FOUND", "One or more original memories are unavailable in the selected project.")
}

func (s *Store) Consolidate(r protocol.ConsolidateRequest) (protocol.WriteResult, error) {
	if err := r.Validate(); err != nil {
		return protocol.WriteResult{}, err
	}
	r = r.Canonical()
	return s.write("consolidate", r.Scope, r.Key, "", r, func(id string) error {
		var text, source string
		var proposal protocol.Proposal
		if draft := r.ExternalProposal; draft != nil {
			text, source = draft.Text, draft.Source
			proposal = protocol.Proposal{Origin: "external", Producer: draft.Producer, AppliesWhen: draft.AppliesWhen, Rationale: draft.Rationale}
		} else {
			var err error
			text, err = s.synthesize(r.Scope, r.EvidenceIDs)
			if err != nil {
				return err
			}
			source = "bigbrain:extractive-v1"
			proposal = protocol.Proposal{
				Origin: "extractive", Producer: source,
				AppliesWhen: "In project " + strings.TrimPrefix(r.Scope, "project:") + ", when the situations described in the linked evidence recur.",
				Rationale:   fmt.Sprintf("This synthesis groups excerpts from %d of the project's memories by their wording.", len(r.EvidenceIDs)),
			}
		}

		now := time.Now().UTC().Format(time.RFC3339Nano)
		_, err := s.query(`CREATE (:Memory {id: $id, scope: $scope, text: $text, source: $source,
  recorded_at: $at, memory_type: 'lesson_proposal', status: 'active', proposal_origin: $origin,
  proposal_producer: $producer, proposal_applies_when: $appliesWhen, proposal_rationale: $rationale});`,
			map[string]any{"id": id, "scope": r.Scope, "text": text, "source": source, "at": now,
				"origin": proposal.Origin, "producer": proposal.Producer, "appliesWhen": proposal.AppliesWhen, "rationale": proposal.Rationale})
		if err != nil {
			return err
		}
		rows, err := s.query(originalEvidence+`
WITH e MATCH (p:Memory {id: $id})
CREATE (p)-[:Evidence {id: CAST(gen_random_uuid() AS STRING), scope: $scope, kind: 'derived_from',
  weight: 1.0, source: $source, activation_count: 0, created_at: $at}]->(e)
RETURN count(*);`, map[string]any{"ids": r.EvidenceIDs, "scope": r.Scope, "id": id, "source": source, "at": now})
		if err != nil {
			return err
		}
		if len(rows) != 1 || rows[0][0] != int64(len(r.EvidenceIDs)) {
			return unavailableEvidence()
		}
		return nil
	})
}

func (s *Store) synthesize(scope string, ids []string) (string, error) {
	rows, err := s.query(originalEvidence+" RETURN e.text ORDER BY e.id;", map[string]any{"scope": scope, "ids": ids})
	if err != nil {
		return "", err
	}
	if len(rows) != len(ids) {
		return "", unavailableEvidence()
	}

	type excerptGroup struct {
		text  string
		count int
	}
	var groups []excerptGroup
	positions := make(map[string]int)
	for _, row := range rows {
		text, ok := row[0].(string)
		if !ok {
			return "", errors.New("invalid evidence text")
		}
		excerpt := firstExcerpt(text)
		key := strings.ToLower(strings.Join(strings.Fields(excerpt), " "))
		if i, found := positions[key]; found {
			groups[i].count++
		} else {
			positions[key] = len(groups)
			groups = append(groups, excerptGroup{text: excerpt, count: 1})
		}
	}
	// The ID order above determines the original wording and breaks frequency ties.
	slices.SortStableFunc(groups, func(a, b excerptGroup) int { return cmp.Compare(b.count, a.count) })
	var summary strings.Builder
	for i, g := range groups {
		if i > 0 {
			summary.WriteByte('\n')
		}
		summary.WriteString(g.text)
		if g.count > 1 {
			fmt.Fprintf(&summary, " (%d memories)", g.count)
		}
	}
	return summary.String(), nil
}

func firstExcerpt(text string) string {
	const maxBytes = 240
	text = strings.TrimSpace(text)
	if len(text) > maxBytes {
		end := maxBytes
		for !utf8.RuneStart(text[end]) {
			end--
		}
		text = strings.TrimRightFunc(text[:end], unicode.IsSpace)
	}
	for i, r := range text {
		if strings.ContainsRune("\r\n\u0085\u2028\u2029", r) {
			return strings.TrimSpace(text[:i])
		}
		standalone := strings.ContainsRune("。！？", r)
		if !standalone && !strings.ContainsRune(".!?", r) {
			continue
		}
		end := i + utf8.RuneLen(r)
		next, size := utf8.DecodeRuneInString(text[end:])
		for next == '"' || next == '\'' || unicode.In(next, unicode.Pe, unicode.Pf) {
			end += size
			next, size = utf8.DecodeRuneInString(text[end:])
		}
		if standalone || size == 0 || unicode.IsSpace(next) {
			return text[:end]
		}
	}
	return text
}
