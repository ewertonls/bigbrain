---
kadai: task
id: "BB-0051"
title: "Retain a paginated activity history across sessions"
status: "todo"
depends_on: [BB-0048]
---
Extend Activity beyond recent active records, latest hook deliveries and the last recall receipt. Record metadata-only events for emitted context and reviewed handoff outcomes at their owning operations. Preserve explicit project and session filters, stable ordering, retention limits and idempotency across retries. Keep prompts, transcript text and full memory bodies out of the event log.

Use the [current activity view](images/redesign/activity.png) as the starting point. Verify that reading a page never creates a new event and that one project's private events are invisible to other projects.
