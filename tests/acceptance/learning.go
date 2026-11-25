package main

import "github.com/ewertonls/bigbrain/internal/protocol"

const learningCheck = "cross_project_learning"

func verifyLearning(c cli) {
	defer shutdown(c)
	original := write(c, recordArgs("trial-a", "Quota errors stopped after batching requests.", "Private A incident 41", "--project", "A")...).ID
	proposal := write(c, "consolidate", "--project", "A", "--key", "a-proposal", "--evidence", original).ID
	queryIDs(query(c, "--project", "B", "--include-global", "--id", proposal))
	const approvedText = "Batch requests to receivers with shared quotas."
	const publicSource = "Approved receiver guidance"
	publish := []string{"publish", "--project", "A", "--key", "publish", "--proposal", proposal, "--text", approvedText, "--source", publicSource}
	expectError(c.run(publish...), "INVALID_REQUEST")
	queryIDs(queryText(c, "receivers", "text", "5", "--project", "B", "--include-global"))
	lesson := write(c, append(publish, "--approve")...).ID
	public := query(c, "--project", "B", "--include-global", "--id", lesson)
	queryIDs(public, lesson)
	require(public.Items[0].Text == approvedText && public.Items[0].Source == publicSource && public.Items[0].Scope == "global",
		"published content differs from approval: %+v", public)
	history := func(id string, scope ...string) protocol.HistoryResult {
		return decode[protocol.HistoryResult](c.run(append([]string{"history", "--id", id}, scope...)...))
	}
	require(len(history(lesson, "--project", "B", "--include-global").Related) == 0, "B saw private publication evidence")

	application := write(c, recordArgs("apply", "B enabled request batching on its dedicated receiver.", "B rollout", "--project", "B")...).ID
	appliedLink := write(c, "link", "--project", "B", "--key", "apply-link", "--from", application, "--to", lesson,
		"--type", "applies", "--source", "B rollout").ID
	const trialSource = "B latency trial"
	counter := write(c, recordArgs("trial-b", "Batching increased latency on dedicated receivers.", trialSource, "--project", "B")...).ID
	feedback := write(c, "feedback", "--project", "B", "--include-global", "--key", "feedback", "--relationship", appliedLink,
		"--author", "B operator", "--source", "B trial review", "--evidence", counter, "--confidence", "0.25")
	events := decode[protocol.FeedbackHistoryResult](c.run("feedback-history", "--project", "B", "--include-global", "--relationship", appliedLink)).Events
	require(len(events) == 1, "expected one recorded use, got %+v", events)
	event := events[0]
	require(event.ID == feedback.ID && event.RelationshipID == appliedLink && event.Scope == "project:B" &&
		event.Action == "feedback" && event.Author == "B operator" && event.Source == "B trial review" && event.EvidenceID == counter &&
		event.ActivationCount != nil && *event.ActivationCount == 1 && event.Confidence != nil && *event.Confidence == 0.25,
		"feedback lost its evidence or attribution: %+v", event)
	applicationHistory := history(application, "--project", "B", "--include-global")
	require(len(applicationHistory.Related) == 1, "application lost its link to the lesson: %+v", applicationHistory)
	applied := applicationHistory.Related[0].Path
	require(applied.RelationshipID == appliedLink && applied.ToID == lesson && applied.Weight == 2 &&
		applied.ActivationCount != nil && *applied.ActivationCount == 1 && applied.Confidence != nil && *applied.Confidence == 0.25,
		"feedback did not update the application relationship: %+v", applied)

	const appliesWhen = "Check whether the destination enforces a shared quota."
	const rationale = "The shared quota motivated batching in A. The dedicated receiver in B had no such limit."
	review := write(c, "reconcile", "--project", "B", "--include-global", "--key", "review", "--supporting", lesson,
		"--contrary", counter, "--outcome", "different_contexts", "--text", "Batch only on shared quotas.",
		"--applies-when", appliesWhen, "--rationale", rationale, "--author", "B maintainer", "--source", "Review 12").ID
	shutdown(c)
	reopened := history(review, "--project", "B", "--include-global")
	require(reopened.Memory.Scope == "project:B" && reopened.Memory.Text == "Batch only on shared quotas." &&
		reopened.Memory.Source == "Review 12" && reopened.Reconciliation != nil &&
		reopened.Reconciliation.Outcome == "different_contexts" && reopened.Reconciliation.AppliesWhen == appliesWhen &&
		reopened.Reconciliation.Rationale == rationale && reopened.Reconciliation.Author == "B maintainer",
		"review lost its conclusion, conditions or author: %+v", reopened)
	require(len(reopened.Related) == 2, "expected both sides of the review: %+v", reopened.Related)
	claims := map[string]string{"supporting_claim": lesson, "contrary_claim": counter}
	for _, claim := range reopened.Related {
		source := publicSource
		if claim.Path.Kind == "contrary_claim" {
			source = trialSource
		}
		require(claim.ID == claims[claim.Path.Kind] && claim.Source == source && claim.Path.Scope == "project:B" &&
			claim.Path.FromID == review && claim.Path.ToID == claim.ID && claim.Path.Direction == "forward" && claim.Path.Source == "Review 12",
			"review claim lost its identity or source: %+v", claim)
		delete(claims, claim.Path.Kind)
	}
	local := history(review, "--project", "B")
	require(len(local.Related) == 1 && local.Related[0].ID == counter, "local history included a global claim: %+v", local.Related)
	queryIDs(query(c, "--project", "A", "--include-global", "--id", review))
	queryIDs(query(c, "--global", "--id", counter))
	require(len(history(lesson, "--global").Related) == 0, "global history exposed a private application, claim or proposal")
}
