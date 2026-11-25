package store

import (
	"strconv"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

// Each entry advances the stored version by one, inside the same transaction
// as the version marker. Entry zero creates the original Foundation schema.
var migrations = [][]string{
	{
		"CREATE NODE TABLE BigBrainSchema(name STRING, version STRING, PRIMARY KEY(name));",
		"CREATE NODE TABLE Memory(id STRING, scope STRING, text STRING, source STRING, recorded_at STRING, PRIMARY KEY(id));",
		"CREATE NODE TABLE Receipt(key STRING, operation STRING, scope STRING, fingerprint STRING, result_id STRING, PRIMARY KEY(key));",
		"CREATE REL TABLE Evidence(FROM Memory TO Memory, id STRING, kind STRING, weight DOUBLE, source STRING);",
		"CREATE (:BigBrainSchema {name: 'foundation', version: '1'});",
	},
	{
		"ALTER TABLE Memory ADD memory_type STRING DEFAULT 'unknown';",
	},
	{
		"ALTER TABLE Evidence ADD scope STRING;",
		"MATCH (a:Memory)-[e:Evidence]->(:Memory) SET e.scope = a.scope;",
	},
	{
		"ALTER TABLE Memory ADD status STRING DEFAULT 'active';",
		"ALTER TABLE Memory ADD invalid_reason STRING;",
		"ALTER TABLE Memory ADD invalid_source STRING;",
		"ALTER TABLE Memory ADD invalid_at STRING;",
		"ALTER TABLE Receipt ADD deleted BOOL DEFAULT false;",
	},
}

func (s *Store) openSchema() error {
	tables, err := s.query("CALL SHOW_TABLES() RETURN name;", nil)
	if err != nil {
		return protocol.NewError("DATABASE_UNAVAILABLE", "Could not read the LadybugDB catalog: "+err.Error())
	}
	version := 0
	if len(tables) != 0 {
		rows, err := s.query("MATCH (s:BigBrainSchema {name: 'foundation'}) RETURN s.version;", nil)
		if err != nil || len(rows) != 1 {
			return protocol.NewError("DATABASE_INCOMPATIBLE", "Missing BigBrain schema version.")
		}
		value, _ := rows[0][0].(string)
		version, err = strconv.Atoi(value)
		if err != nil || version < 1 || version > len(migrations) {
			return protocol.NewError("DATABASE_INCOMPATIBLE", "Unsupported BigBrain schema version.")
		}
	}
	if version == len(migrations) {
		return s.checkSchema()
	}
	if _, err := s.query("BEGIN TRANSACTION;", nil); err != nil {
		return err
	}
	committed := false
	defer s.rollbackUnlessCommitted(&committed)
	for _, migration := range migrations[version:] {
		for _, statement := range migration {
			if _, err := s.query(statement, nil); err != nil {
				return protocol.NewError("DATABASE_INCOMPATIBLE", "Could not apply the BigBrain schema migration: "+err.Error())
			}
		}
	}
	if err := s.checkSchema(); err != nil {
		return err
	}
	if _, err := s.query("MATCH (s:BigBrainSchema {name: 'foundation'}) SET s.version = $version;",
		map[string]any{"version": strconv.Itoa(len(migrations))}); err != nil {
		return err
	}
	_, err = s.query("COMMIT;", nil)
	committed = err == nil
	return err
}

func (s *Store) checkSchema() error {
	for _, statement := range []string{
		"MATCH (m:Memory) RETURN m.id, m.scope, m.text, m.source, m.recorded_at, m.memory_type, m.status, m.invalid_reason, m.invalid_source, m.invalid_at LIMIT 0;",
		"MATCH (r:Receipt) RETURN r.key, r.operation, r.scope, r.fingerprint, r.result_id, r.deleted LIMIT 0;",
		"MATCH (:Memory)-[r:Evidence]->(:Memory) RETURN r.id, r.kind, r.weight, r.source, r.scope LIMIT 0;",
	} {
		if _, err := s.query(statement, nil); err != nil {
			return protocol.NewError("DATABASE_INCOMPATIBLE", "The database is missing part of the BigBrain schema: "+err.Error())
		}
	}
	return nil
}
