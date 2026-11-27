package main

import (
	"encoding/json"
	"os"
	"strings"
)

func verifyReviewedHandoff(c cli) {
	defer shutdown(c)
	check(os.Mkdir(c.dir, 0700))
	c.env = []string{"BIGBRAIN_CODEX_STATE=" + c.db + "-state", "CODEX_THREAD_ID=session-A"}
	decode[json.RawMessage](c.run("project", "init", "A"))
	const draft = `{"producer":"acceptance","memory":{"type":"decision","text":"Use a socket.","source":"fixture/transport"},"evidence":{"type":"observation","text":"Only one daemon owns the database.","source":"fixture/daemon"}}`
	type staged struct {
		ID       string
		Status   string
		Manifest struct {
			IDs    []string
			LinkID string `json:"link_id"`
		}
	}
	stage := func(input string) staged {
		return decode[staged](c.invokeInput(strings.NewReader(input), "--json", "handoff", "stage"))
	}
	discard := stage(strings.Replace(draft, "Use a socket.", "Abandoned proposal.", 1))
	decode[staged](c.run("handoff", "dismiss", "--id", discard.ID, "--revision", "1"))
	require(len(decode[matches](c.run("query", "--mode", "text", "--text", "socket")).Items) == 0, "unreviewed draft became searchable")
	d := stage(draft)
	// The first record committed, but its acknowledgement did not reach the
	// local manifest. Replay must recover its ID before submitting the link.
	first := decode[writeResult](c.run("record", "--key", d.ID+"/decision", "--type", "decision", "--text", "Use a socket.", "--source", "fixture/transport"))
	save := func() staged { return decode[staged](c.run("handoff", "save", "--id", d.ID, "--revision", "1")) }
	saved := save()
	require(saved.Status == "saved" && len(saved.Manifest.IDs) == 2 && saved.Manifest.IDs[0] == first.ID && saved.Manifest.LinkID != "", "incomplete reviewed handoff")
	require(save().Manifest.LinkID == saved.Manifest.LinkID, "replay created another relationship")
	require(queryText(c, "Abandoned", "text", "5").Count == 0, "dismissed proposal became searchable")
	type packet struct {
		Text    string
		Receipt struct{ Bytes int }
	}
	recovered := decode[packet](c.run("context", "--session", "session-B", "--preview"))
	require(strings.Contains(recovered.Text, "fixture/transport") && strings.Contains(recovered.Text, first.ID) && len(recovered.Text) == recovered.Receipt.Bytes && recovered.Receipt.Bytes <= 8192, "cross-session context missing or oversized")
}
