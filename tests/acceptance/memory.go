package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

type evaluationMemory struct{ Key, Scope, Text, Source string }
type evaluationQuestion struct {
	Kind, Question, Search, Expected string
	Relevant                         []string
}
type memoryFixture struct {
	Project   string
	Limit     int
	Memories  []evaluationMemory
	Links     [][2]string
	Questions []evaluationQuestion
}

type evaluationQuery struct {
	text, mode string
	global     bool
}

type questionScore struct {
	Search   string   `json:"search"`
	Hit      bool     `json:"hit"`
	Returned []string `json:"returned_keys"`
}

type modeScore struct {
	Hits       map[string]int  `json:"hits"`
	Irrelevant int             `json:"irrelevant"`
	MedianUS   int64           `json:"median_us"`
	P95US      int64           `json:"p95_us"`
	Questions  []questionScore `json:"questions"`
}

func evaluationScope(scope string) []string {
	if scope == "global" {
		return []string{"--global"}
	}
	return []string{"--project", scope}
}

func (f memoryFixture) query(c cli, q evaluationQuery) protocol.QueryResult {
	args := []string{"query", "--project", f.Project, "--limit", strconv.Itoa(f.Limit), "--text", q.text, "--mode", q.mode}
	if q.global {
		args = append(args, "--include-global")
	}
	return decode[protocol.QueryResult](c.run(args...))
}

func (f memoryFixture) seed(c cli) (map[string]evaluationMemory, map[string]string) {
	memories := make(map[string]evaluationMemory)
	ids := make(map[string]string)
	for _, m := range f.Memories {
		require(ids[m.Key] == "", "duplicate fixture key %q", m.Key)
		args := recordArgs(m.Key, m.Text, m.Source, evaluationScope(m.Scope)...)
		ids[m.Key] = write(c, args...).ID
		memories[m.Key] = m
	}
	for _, edge := range f.Links {
		a, b := edge[0], edge[1]
		require(ids[a] != "" && ids[b] != "", "invalid fixture link %v", edge)
		owner := memories[a].Scope
		if owner == "global" {
			owner = memories[b].Scope
		}
		args := []string{"link", "--key", a + "-" + b, "--from", ids[a], "--to", ids[b], "--type", "related", "--source", "Fixture review of " + a + " and " + b}
		write(c, append(args, evaluationScope(owner)...)...)
	}
	return memories, ids
}

func wordsInCommon(a, b string) bool {
	tokens := func(s string) []string {
		return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	}
	second := tokens(b)
	for _, w := range tokens(a) {
		if slices.Contains(second, w) {
			return true
		}
	}
	return false
}

func (f memoryFixture) checkRanking(q evaluationQuery, result protocol.QueryResult, byID map[string]evaluationMemory) {
	require(len(result.Matches) == min(f.Limit, result.Count), "wrong count or limit for %+v", q)
	seen := make(map[string]bool)
	for _, m := range result.Matches {
		source, ok := byID[m.ID]
		require(ok && source.Source == m.Source, "unknown memory or incorrect source for %+v", q)
		require(source.Scope == f.Project || q.global && source.Scope == "global", "private memory for %+v", q)
		require(!seen[m.ID], "duplicate result for %+v", q)
		seen[m.ID] = true
	}
}

func runMemoryEvaluation(c cli, fixturePath string) {
	data, err := os.ReadFile(fixturePath)
	check(err)
	var f memoryFixture
	check(json.Unmarshal(data, &f))
	model := os.Getenv("BIGBRAIN_EMBEDDINGS")
	require(model != "", "set BIGBRAIN_EMBEDDINGS to the local evaluation model")
	model, err = filepath.Abs(model)
	check(err)
	file, err := os.Open(model)
	check(err)
	digest := sha256.New()
	_, err = io.Copy(digest, file)
	check(err)
	check(file.Close())
	c.env = []string{"BIGBRAIN_EMBEDDINGS=" + model, "BIGBRAIN_VECTOR_INDEX="}
	defer func() { c.run("shutdown") }()
	memories, ids := f.seed(c)
	byID := make(map[string]evaluationMemory, len(ids))
	for label, id := range ids {
		byID[id] = memories[label]
	}
	require(len(f.Questions) > 0 && f.Limit > 0, "evaluation requires questions and a positive limit")
	for _, q := range f.Questions {
		require(ids[q.Expected] != "" && slices.Contains(q.Relevant, q.Expected), "invalid expected answer for %q", q.Search)
		if q.Kind == "paraphrase" {
			require(!wordsInCommon(q.Search, memories[q.Expected].Text), "paraphrase shares a term with its target: %s", q.Search)
		}
	}
	start := time.Now()
	warm := f.query(c, evaluationQuery{f.Questions[0].Search, "hybrid", true})
	firstHybrid := time.Since(start).Microseconds()
	require(warm.Fallback == "", "configured model fell back: %s", warm.Fallback)

	const repetitions = 5
	original := make(map[evaluationQuery]protocol.QueryResult)
	scores := make(map[string]modeScore)
	for _, mode := range []string{"text", "associative", "hybrid"} {
		score := modeScore{Hits: map[string]int{"direct": 0, "graph": 0, "paraphrase": 0}}
		var latency []int64
		for _, q := range f.Questions {
			key := evaluationQuery{q.Search, mode, false}
			original[key] = f.query(c, key)
			f.checkRanking(key, original[key], byID)
			key.global = true
			got := f.query(c, key)
			original[key] = got
			f.checkRanking(key, got, byID)
			require(got.Fallback == "", "unexpected fallback for %q: %s", q.Search, got.Fallback)
			row := questionScore{Search: q.Search, Returned: []string{}}
			for _, m := range got.Matches {
				label := byID[m.ID].Key
				row.Returned = append(row.Returned, label)
				if m.ID == ids[q.Expected] {
					row.Hit = true
				}
				if !slices.Contains(q.Relevant, label) {
					score.Irrelevant++
				}
			}
			if row.Hit {
				score.Hits[q.Kind]++
			}
			score.Questions = append(score.Questions, row)
			for range repetitions {
				start := time.Now()
				again := f.query(c, key)
				latency = append(latency, time.Since(start).Microseconds())
				require(reflect.DeepEqual(again, got), "unstable ranking for %+v", key)
			}
		}
		slices.Sort(latency)
		score.MedianUS = latency[(len(latency)-1)/2]
		score.P95US = latency[(95*len(latency)+99)/100-1]
		scores[mode] = score
	}

	// Add perfect lexical and semantic competitors in a different project.
	// All queries keep the same database, model, scopes and request limits.
	for i, q := range f.Questions {
		for j, text := range []string{q.Search, memories[q.Expected].Text} {
			key := fmt.Sprintf("outside-%d-%d", i, j)
			write(c, recordArgs(key, text, "Private distractor", "--project", "other")...)
		}
	}
	for _, state := range []string{"warm", "reopened"} {
		if state == "reopened" {
			shutdown(c)
		}
		for key, before := range original {
			require(reflect.DeepEqual(f.query(c, key), before), "private records changed %s ranking for %+v", state, key)
		}
	}
	shutdown(c)
	c.env = []string{"BIGBRAIN_EMBEDDINGS=" + filepath.Join(c.db, "absent-model"), "BIGBRAIN_VECTOR_INDEX="}
	for key, before := range original {
		after := f.query(c, key)
		if key.mode == "hybrid" {
			require(after.Fallback == "model_unavailable", "unavailable model returned %q", after.Fallback)
			after.Fallback = ""
			key.mode = "associative"
			before = original[key]
		}
		require(reflect.DeepEqual(after, before), "unavailable model changed the %s control for %q", key.mode, key.text)
	}
	shutdown(c)

	check(json.NewEncoder(os.Stdout).Encode(map[string]any{
		"model_sha256": hex.EncodeToString(digest.Sum(nil)), "limit": f.Limit, "warm_queries_per_mode": len(f.Questions) * repetitions,
		"first_hybrid_us": firstHybrid, "scores": scores, "scope_comparisons": 2 * len(original), "fallback_comparisons": len(original),
	}))
	for i, q := range f.Questions {
		if q.Kind == "direct" && scores["text"].Questions[i].Hit {
			require(scores["hybrid"].Questions[i].Hit, "lost direct hit for %q", q.Search)
		}
	}
	require(scores["hybrid"].Hits["paraphrase"] > max(scores["text"].Hits["paraphrase"], scores["associative"].Hits["paraphrase"]), "hybrid did not gain paraphrase hits")
}
