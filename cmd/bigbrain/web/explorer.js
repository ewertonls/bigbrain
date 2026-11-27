function retrieval(m) {
  const reason = m.reason, box = section("Retrieval explanation");
  const steps = reason.steps?.length
    ? reason.steps
    : reason.path
    ? [reason.path]
    : [];
  const kinds = {
    text: "Text match",
    vector: "Vector match",
    hybrid: "Combined retrieval signals",
    relationship: "Relationship traversal",
    id: "Selected by ID",
  };
  box.append(element("strong", kinds[reason.kind] || reason.kind));
  const signals = Object.entries({
    BM25: reason.bm25,
    Cosine: reason.cosine,
    Graph: reason.graph_score,
  }).filter(([, v]) => v != null);
  if (signals.length) {
    box.append(
      facts(signals.map(([name, value]) => [name, Number(value.toFixed(3))])),
    );
  }
  if (reason.terms?.length) box.append(element("p", reason.terms.join(" · ")));
  if (!steps.length) {
    box.append(element("p", "Direct match. No relationship was traversed."));
  }
  for (const step of steps) {
    const item = element("details");
    item.append(
      element("summary", step.type + " · " + step.direction),
      facts([["Stored from", step.from_id], ["Stored to", step.to_id], [
        "Source",
        step.source,
      ], ["Relationship ID", step.relationship_id]]),
    );
    box.append(item);
  }
  return box;
}
async function mountWorkspace(api, local) {
  const page = el("page"), notice = el("notice");
  let generation = 0,
    view = "overview",
    status,
    projectGlobal,
    term = "",
    seed = "",
    edgeToInspect = "",
    back = [],
    recent = [],
    cursor = "";
  function scope() {
    const only = el("scope").value === "global";
    return {
      scope: only ? "global" : status.binding.scope,
      include_global: !only && el("global").checked,
    };
  }
  function move(name) {
    view = name;
    return render();
  }
  function openMemory(m) {
    view = "memories";
    generation++;
    notice.textContent = "";
    markTab();
    page.replaceChildren(
      action("Back to memories", () => move("memories"), "text-button"),
      heading(m.type, ""),
      element("p", m.text, "memory-body"),
    );
    page.append(
      facts([["Source", m.source], ["Scope", m.scope], [
        "Recorded",
        timeLabel(m.recorded_at),
      ], ["Memory ID", m.id]]),
      action("Explore stored neighbors", () => openGraph(m.id), "primary"),
    );
    const copies = element("div", null, "actions copy-actions");
    for (
      const [title, value] of [["Copy reference", m.id + " · " + m.source], [
        "Copy text",
        m.text,
      ], [
        "Copy context",
        m.scope + " · " + m.id + " · " + m.source + "\n" + m.text,
      ]]
    ) {
      copies.append(
        action(
          title,
          () => navigator.clipboard.writeText(value),
          "text-button",
        ),
      );
    }
    page.append(copies);
    if (m.reason) page.append(retrieval(m));
  }
  function openGraph(id, edge = "") {
    edgeToInspect = edge;
    back = [];
    seed = id;
    return move("graph");
  }
  function markTab() {
    for (const b of document.querySelectorAll("[data-page]")) {
      b.setAttribute("aria-pressed", b.dataset.page === view);
    }
  }
  function searchBox(value) {
    const form = element("form"), input = element("input");
    input.type = "search";
    input.id = "query";
    input.setAttribute("aria-label", "Search memories");
    input.placeholder = "Search memories…";
    input.value = value;
    form.append(input, element("button", "Search"));
    form.onsubmit = (e) => {
      e.preventDefault();
      term = input.value.trim();
      move("memories");
    };
    return form;
  }
  const empty = () =>
    element(
      "p",
      "No active memories in this scope. Record a memory or save a reviewed handoff to get started.",
      "empty",
    );
  async function render() {
    const stamp = ++generation, current = () => stamp === generation;
    markTab();
    document.querySelector(".scope").hidden = ["review", "settings"].includes(
      view,
    );
    page.replaceChildren();
    notice.textContent = "Loading…";
    try {
      if (!status) {
        const loaded = await api("status");
        if (!current()) return;
        status = loaded;
        el("scope").options[0].text = status.binding.scope;
        projectGlobal = el("global").checked = status.binding.include_global;
      }
      el("connection").textContent = "Local · " + status.daemon.state;
      const s = scope(), hasSession = !!status.binding.session_id;
      const read = (op, data) => api(op, { ...s, ...data });
      const browse = (after = "", limit = 12) =>
        read("browse", { after, limit });
      if (view === "settings") {
        page.append(
          heading("Settings", "Workspace and recall policy"),
          facts([["Workspace", status.binding.scope], [
            "Daemon",
            status.daemon.state,
          ], ["Session", status.binding.session_id || "No session pinned"]]),
        );
        if (!hasSession) {
          page.append(
            element(
              "p",
              "Open bigbrain ui --session SESSION to manage recall and hook settings.",
              "empty",
            ),
          );
        } else if (local) {
          page.append(el("automation-template").content.cloneNode(true));
          await mountAutomation(api);
        } else {
          const data = await api("automation", {});
          if (!current()) return;
          page.append(
            facts([
              ["Startup recall", data.policy.disable_startup ? "Off" : "On"],
              ["After compaction", data.policy.disable_compact ? "Off" : "On"],
              ["Global recall", data.policy.project_only ? "Off" : "On"],
              ["Paused", data.policy.paused ? "Yes" : "No"],
              ["Recall limit", "5 memories · 8 KiB"],
            ]),
            element(
              "p",
              "Change session policy through bigbrain ui --session SESSION.",
            ),
          );
        }
      } else if (view === "review") {
        page.append(heading("Handoff review", status.binding.scope));
        if (!hasSession) {
          page.append(
            element(
              "p",
              "Open bigbrain ui --session SESSION or pin a session with --explorer-session to review its drafts.",
              "empty",
            ),
          );
        } else {
          const result = await api("drafts", {});
          if (!current()) return;
          page.append(el("review-template").content.cloneNode(true));
          mountReview(
            api,
            local || result.writable,
            local ? result : result.drafts,
          );
        }
      } else if (view === "memories") {
        page.append(
          heading(
            "Memories",
            "Browse by recording time or search the selected scope.",
          ),
          searchBox(term),
        );
        const result = term
          ? await read("query", { text: term, mode: "hybrid", limit: 20 })
          : await browse();
        if (!current()) return;
        recent = result.matches || result.memories || [];
        cursor = result.next_cursor || "";
        const group = section(term ? "Search results" : "Recent memories"),
          rows = element("div", null, "rows");
        if (term) {
          group.append(
            element(
              "small",
              recent.length + " returned" +
                (result.fallback
                  ? " · Text and relationship fallback · " + result.fallback
                  : ""),
            ),
          );
        }
        group.append(rows);
        page.append(group);
        for (const m of recent) rows.append(memoryRow(m, () => openMemory(m)));
        if (!recent.length) {
          rows.append(
            term
              ? element("p", "No matches. Change or clear the search.", "empty")
              : empty(),
          );
        }
        const more = action("Load more memories", async () => {
          more.disabled = true;
          try {
            const batch = await browse(cursor);
            if (!current()) return;
            for (const m of batch.memories) {
              rows.append(memoryRow(m, () => openMemory(m)));
            }
            cursor = batch.next_cursor;
            more.hidden = !cursor;
          } finally {
            more.disabled = false;
          }
        });
        if (cursor) group.append(more);
      } else if (view === "graph") {
        const batch = await browse("", 8);
        if (!current()) return;
        page.append(
          heading(
            "Knowledge graph",
            "Stored relationships · one neighborhood at a time",
          ),
        );
        if (!seed) seed = batch.memories[0]?.id;
        if (seed) {
          const history = await read("history", { id: seed, limit: 6 });
          if (!current()) return;
          showGraph(history, batch.memories, read, current);
        } else page.append(empty());
      } else {
        const batch = await browse("", view === "overview" ? 6 : 20);
        if (!current()) return;
        recent = batch.memories;
        if (view === "activity") {
          page.append(
            heading("Activity", "Recent records in the selected scope"),
          );
          const timeline = section("Recorded activity"),
            events = recent.map((m) => ({
              at: m.recorded_at,
              title: "Memory recorded",
              detail: m.text,
              select: () => openMemory(m),
            }));
          if (
            hasSession && s.scope === status.binding.scope && s.include_global
          ) {
            const data = await api("automation", {});
            if (!current()) return;
            if (data.last) {
              events.push({
                at: data.last.at,
                title: "Context emitted",
                detail: data.last.memories.length + " included · " +
                  data.last.bytes + " UTF-8 bytes",
              });
            }
            for (const [hook, at] of Object.entries(data.hooks || {})) {
              events.push({ at, title: "Hook observed", detail: hook });
            }
          }
          events.sort((a, b) => Date.parse(b.at) - Date.parse(a.at));
          for (const e of events) {
            const item = element("article", null, "event");
            item.append(
              element("small", timeLabel(e.at)),
              element("strong", e.title),
              e.select
                ? action(short(e.detail, 120), e.select, "text-button")
                : element("p", e.detail),
            );
            timeline.append(item);
          }
          if (!events.length) {
            timeline.append(
              element("p", "No recorded activity in this scope.", "empty"),
            );
          }
          page.append(timeline);
        } else {
          page.append(
            heading(
              s.scope === "global" ? "Global memory" : "Project memory",
              "Knowledge accumulated across sessions.",
            ),
            searchBox(""),
          );
          const metrics = element("div", null, "metrics");
          function metric(name, value) {
            const cell = element("div");
            cell.append(element("small", name), element("strong", value));
            metrics.append(cell);
          }
          metric(
            "Active memories",
            recent.length + (batch.next_cursor ? "+" : ""),
          );
          page.insertBefore(metrics, page.querySelector("form"));
          if (recent.length) {
            const histories = await Promise.all(
              recent.slice(0, 3).map((m) =>
                read("history", { id: m.id, limit: 5 })
              ),
            );
            if (!current()) return;
            const h = histories.find((h) => h.related.length) || histories[0];
            seed = h.memory.id;
            metric("Links in preview", h.related.length);
            const preview = section(
              "Knowledge network",
              action(
                "Explore graph",
                () => openGraph(h.memory.id),
                "text-button",
              ),
            );
            preview.append(
              network(
                h,
                (kind, item) =>
                  kind === "node"
                    ? openGraph(item.id)
                    : openGraph(h.memory.id, item.relationship_id),
                true,
              ),
              element(
                "small",
                h.related.length
                  ? h.related.length + " stored relationships shown"
                  : "No stored links for this memory.",
              ),
            );
            page.append(preview);
          }
          const list = section(
            "Recent memories",
            action("Browse all", () => {
              term = "";
              return move("memories");
            }, "text-button"),
          );
          if (!recent.length) list.append(empty());
          for (const m of recent) {
            list.append(memoryRow(m, () => openMemory(m)));
          }
          page.append(list);
          if (hasSession && s.scope === status.binding.scope) {
            const ds = await api("drafts", {});
            if (!current()) return;
            const count = (ds.drafts || ds).filter((d) =>
              !["dismissed", "saved"].includes(d.status)
            ).length;
            metric("Pending review", count);
            if (count) {
              page.append(action(count + " pending · Review handoff", () =>
                move("review"), "review-link"));
            }
          }
        }
      }
      if (current()) {
        notice.textContent = "";
        if (["overview", "graph", "activity", "memories"].includes(view)) {
          status.daemon.state = "running";
          el("connection").textContent = "Local · running";
        }
      }
    } catch (e) {
      if (current()) {
        page.replaceChildren();
        notice.replaceChildren(
          element("p", e.message),
          action("Try again", render),
        );
      }
    }
  }
  function showGraph(h, choices, read, current) {
    const tools = element("div", null, "actions"), picker = element("select");
    picker.setAttribute("aria-label", "Focus memory");
    const memories = new Map([
      [h.memory.id, h.memory],
      ...choices.map((m) => [m.id, m]),
    ]);
    for (const m of memories.values()) {
      picker.add(new Option(short(m.text, 60), m.id));
    }
    picker.value = h.memory.id;
    function follow(id) {
      if (seed !== id) {
        back.push(seed);
        seed = id;
      }
      return render();
    }
    picker.onchange = () => follow(picker.value);
    tools.append(picker);
    if (back.length) {
      tools.append(action("Back", () => {
        seed = back.pop();
        return render();
      }));
    }
    page.append(tools);
    const layout = element("div", null, "graph-layout"),
      canvas = element("div", null, "graph-sheet"),
      inspector = element("aside", null, "inspector");
    layout.append(canvas, inspector);
    page.append(layout);
    function inspect(kind, obj) {
      inspector.replaceChildren();
      if (kind === "edge") {
        inspector.append(
          element("small", "Stored relationship", "category"),
          element("h2", obj.type),
          facts([
            [
              "From",
              memories.get(obj.from_id)?.text ||
              memories.get(obj.from_id)?.excerpt || obj.from_id,
            ],
            [
              "To",
              memories.get(obj.to_id)?.text ||
              memories.get(obj.to_id)?.excerpt || obj.to_id,
            ],
            ["Scope", obj.scope],
            ["Weight", obj.weight],
            ["Confidence", obj.confidence],
            ["Activations", obj.activation_count],
            ["Last activated", obj.last_activated_at || null],
            ["Source", obj.source],
            ["ID", obj.relationship_id],
          ]),
        );
        const feedback = element("div");
        async function feedbackPage(after) {
          const result = await read("feedback", {
            relationship_id: obj.relationship_id,
            after,
            limit: 5,
          });
          if (!feedback.isConnected) return;
          feedback.replaceChildren();
          for (const event of result.events) {
            feedback.append(
              element("p", event.action + " · " + timeLabel(event.recorded_at)),
              element("small", event.author + " · " + event.source),
            );
          }
          if (!result.events.length) {
            feedback.append(element("small", "No feedback recorded."));
          }
          if (result.next_cursor) {
            feedback.append(
              action("Next feedback", () => feedbackPage(result.next_cursor)),
            );
          }
        }
        inspector.append(
          action("Feedback history", () => feedbackPage(""), "text-button"),
          feedback,
        );
      } else {
        inspector.append(
          element("small", obj.type, "category " + tone(obj.type)),
          element("h2", obj.text || obj.excerpt || obj.id),
          facts([["Source", obj.source], ["Scope", obj.scope], ["ID", obj.id]]),
        );
        if (obj.id !== seed) {
          inspector.append(
            action("Explore neighbors", () => follow(obj.id), "primary"),
          );
        }
      }
    }
    function draw(batch) {
      const diagram = network(batch, inspect);
      const controls = graphControls(diagram);
      const toggle = action("List view", () => {
        const hidden = diagram.toggleAttribute("hidden");
        controls.hidden = hidden;
        toggle.textContent = hidden ? "Graph view" : "List view";
      }, "text-button");
      canvas.replaceChildren(controls, diagram, toggle);
      for (const item of batch.related) memories.set(item.id, item);
      const list = section("Relationships"),
        rows = element("div", null, "rows");
      list.append(rows);
      for (const item of batch.related) {
        rows.append(
          action(
            item.path.type + " · " + (item.excerpt || item.source),
            () => inspect("edge", item.path),
            "edge-row",
          ),
        );
      }
      if (!batch.related.length) {
        list.append(element("p", "No visible links for this memory.", "empty"));
      }
      if (batch.next_cursor) {
        const more = action("Next relationships", async () => {
          more.disabled = true;
          try {
            const next = await read("history", {
              id: seed,
              limit: 6,
              after: batch.next_cursor,
            });
            if (current()) draw(next);
          } finally {
            more.disabled = false;
          }
        });
        list.append(more);
      }
      canvas.append(list);
      const selectedEdge = batch.related.find((item) =>
        item.path.relationship_id === edgeToInspect
      );
      edgeToInspect = "";
      if (selectedEdge) inspect("edge", selectedEdge.path);
      else inspect("node", batch.memory);
    }
    draw(h);
  }
  for (const b of document.querySelectorAll("[data-page]")) {
    b.onclick = () => move(b.dataset.page);
  }
  function resetScope() {
    term = "";
    seed = "";
    back = [];
    if (view === "review" || view === "settings") view = "overview";
    render();
  }
  el("scope").onchange = () => {
    const only = el("scope").value === "global";
    el("global").checked = !only && projectGlobal;
    el("global").disabled = only;
    resetScope();
  };
  el("global").onchange = () => {
    projectGlobal = el("global").checked;
    resetScope();
  };
  await render();
}
