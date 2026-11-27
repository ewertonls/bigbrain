package store

import "github.com/ewertonls/bigbrain/internal/protocol"

func (s *Store) Browse(r protocol.BrowseRequest) (protocol.BrowseResult, error) {
	var result protocol.BrowseResult
	if err := r.Validate(); err != nil {
		return result, err
	}
	params := scopeParameters(r.Scope, r.IncludeGlobal)
	const visible = visibleMemory + " AND m.status='active'"
	// Stored UTC RFC3339Nano strings have variable fractional precision. Removing
	// the trailing Z makes a whole second sort before its fractional seconds.
	const sortTime = "replace(m.recorded_at,'Z','')"
	filter := ""
	if r.After != "" {
		params["after"] = r.After
		rows, err := s.query("MATCH (m:Memory {id:$after}) WHERE "+visible+" RETURN "+sortTime, params)
		if err != nil {
			return result, err
		}
		if len(rows) == 0 {
			return result, protocol.NewError("INVALID_CURSOR", "Memory cursor is no longer visible. Refresh the list.")
		}
		params["at"] = rows[0][0]
		filter = " AND (" + sortTime + " < $at OR (" + sortTime + " = $at AND m.id > $after))"
	}
	limit := r.Limit
	if limit == 0 {
		limit = protocol.DefaultQueryLimit
	}
	params["limit"] = int64(limit + 1)
	rows, err := s.query("MATCH (m:Memory) WHERE "+visible+filter+" RETURN "+memoryColumns+", "+sortTime+" AS sort_at ORDER BY sort_at DESC, m.id ASC LIMIT $limit", params)
	if err != nil {
		return result, err
	}
	result.Memories = make([]protocol.Memory, 0, min(limit, len(rows)))
	for _, row := range rows[:min(limit, len(rows))] {
		m, err := decodeMemory(row)
		if err != nil {
			return result, err
		}
		result.Memories = append(result.Memories, m)
	}
	if len(rows) > limit {
		result.NextCursor = result.Memories[limit-1].ID
	}
	return result, nil
}
