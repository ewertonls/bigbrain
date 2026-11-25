CREATE NODE TABLE Memory(id STRING, scope STRING, text STRING, source STRING, recorded_at STRING, PRIMARY KEY(id));
CREATE NODE TABLE Receipt(key STRING, operation STRING, scope STRING, fingerprint STRING, result_id STRING, PRIMARY KEY(key));
CREATE REL TABLE Evidence(FROM Memory TO Memory, kind STRING, weight DOUBLE, source STRING);

CREATE (:Memory {id: 'D1', scope: 'project-a', text: 'Orbit sends readings in batches.', source: 'Decision 12', recorded_at: '2026-11-24T00:00:00Z'});
CREATE (:Memory {id: 'E1', scope: 'project-a', text: 'The receiver accepts only one request per minute.', source: 'Test 7', recorded_at: '2026-11-24T00:00:00Z'});
CREATE (:Memory {id: 'E2', scope: 'project-a', text: 'The dashboard uses a light background.', source: 'Note 4', recorded_at: '2026-11-24T00:00:00Z'});
CREATE (:Receipt {key: 'seed-d1', operation: 'record_memory', scope: 'project-a', fingerprint: 'b9e6ffcfe1a15c11a70b362fd437941c941bfbfedba8dca30d38d0cd810ac37f', result_id: 'D1'});
CREATE (:Receipt {key: 'seed-e1', operation: 'record_memory', scope: 'project-a', fingerprint: 'fp-seed-e1', result_id: 'E1'});
CREATE (:Receipt {key: 'seed-e2', operation: 'record_memory', scope: 'project-a', fingerprint: 'fp-seed-e2', result_id: 'E2'});
CREATE (:Receipt {key: 'seed-link-d1-e1', operation: 'link_memories', scope: 'project-a', fingerprint: 'fp-seed-link-d1-e1', result_id: 'D1:E1'});
MATCH (decision:Memory {id: 'D1'}), (evidence:Memory {id: 'E1'})
CREATE (decision)-[:Evidence {kind: 'justified_by', weight: 1.0, source: 'Review of Decision 12'}]->(evidence);

EXPORT DATABASE '/work/export-v0.13.1' (format='csv', header=true);
