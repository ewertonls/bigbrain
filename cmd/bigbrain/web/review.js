function mountReview(api, writable, initial) {
  const rows = el("drafts"),
    message = el("review-message"),
    save = el("save-selected"),
    dismiss = el("dismiss-selected");
  const selected = new Map();
  let busy = false;
  save.hidden = dismiss.hidden = !writable;
  function controls() {
    save.disabled = busy || !selected.size;
    save.textContent = selected.size
      ? "Save " + selected.size +
        (selected.size === 1 ? " memory" : " memories")
      : "Save selected";
    dismiss.disabled = busy || !selected.size ||
      [...selected.values()].some((d) => d.status !== "draft");
  }
  function draw(ds) {
    if (!rows.isConnected) return;
    rows.replaceChildren();
    selected.clear();
    const pending = ds.filter((d) =>
      !["saved", "dismissed"].includes(d.status)
    );
    if (!pending.length) {
      rows.append(
        element("p", "No pending handoffs in this session.", "empty"),
      );
    }
    for (const d of pending) {
      const row = element("article", null, "draft"),
        label = element("label"),
        check = element("input"),
        m = d.content.memory;
      check.type = "checkbox";
      check.disabled = !writable;
      check.onchange = () => {
        if (check.checked) selected.set(d.id, d);
        else selected.delete(d.id);
        controls();
      };
      label.append(check, icon("check"), element("strong", short(m.text, 125)));
      const detail = element("details");
      detail.append(
        element("summary", "Preview"),
        element("p", m.text),
        facts([["Source", m.source], ["Destination", d.origin.scope], [
          "Producer",
          d.content.producer,
        ]]),
      );
      const ev = d.content.evidence;
      if (ev) {
        detail.append(
          element("h3", "Supporting observation"),
          element("p", ev.text),
          element("small", ev.source),
        );
      }
      for (
        const [name, items] of [["Open questions", d.content.open_questions], [
          "Next steps",
          d.content.next_steps,
        ]]
      ) {
        if (items?.length) {
          const list = element("ul");
          for (const item of items) list.append(element("li", item));
          detail.append(element("h3", name), list);
        }
      }
      if (writable && d.status === "draft") {
        const edit = element("details"),
          form = element("form", null, "draft-form");
        edit.append(element("summary", "Edit draft"), form);
        const content = structuredClone(d.content);
        for (
          const [name, mem] of [["Memory", content.memory], [
            "Evidence",
            content.evidence,
          ]]
        ) {
          if (!mem) continue;
          for (const key of ["text", "source", "type"]) {
            const field = element("label", name + " " + key),
              input = element(key === "text" ? "textarea" : "input");
            input.value = mem[key];
            input.oninput = () => {
              mem[key] = input.value;
            };
            field.append(input);
            form.append(field);
          }
        }
        const apply = element("button", "Apply edits");
        form.append(apply);
        form.onsubmit = async (e) => {
          e.preventDefault();
          if (busy) return;
          busy = true;
          controls();
          apply.disabled = true;
          try {
            await api("edit", { id: d.id, revision: d.revision, content });
            await refresh();
          } catch (err) {
            message.textContent = err.message;
          } finally {
            busy = false;
            controls();
            apply.disabled = false;
          }
        };
        detail.append(edit);
      }
      row.append(
        label,
        element(
          "small",
          displayType(m.type) + (d.status === "draft" ? "" : " · " + d.status),
        ),
        detail,
      );
      rows.append(row);
    }
    const closed = ds.filter((d) => ["saved", "dismissed"].includes(d.status));
    if (closed.length) {
      const history = element("details");
      history.append(
        element("summary", "Reviewed handoffs · " + closed.length),
      );
      for (const d of closed) {
        history.append(
          element("p", d.status + " · " + d.content.memory.text),
          element("small", d.content.memory.source),
        );
      }
      rows.append(history);
    }
    if (el("card-summary")) {
      el("card-summary").textContent = pending.length
        ? "Pending review"
        : "Review complete";
    }
    controls();
  }
  async function refresh() {
    const result = await api("drafts", {});
    draw(result.drafts || result);
  }
  async function submit(op) {
    if (busy || !selected.size) return;
    const approved = [...selected.values()];
    busy = true;
    controls();
    rows.inert = true;
    const outcomes = [];
    try {
      for (const d of approved) {
        try {
          const r = await api(op, { id: d.id, revision: d.revision });
          outcomes.push(r.status + " · " + short(d.content.memory.text, 70));
        } catch (e) {
          outcomes.push("Failed · " + e.message);
        }
      }
      await refresh();
      message.textContent = outcomes.join(" / ");
    } catch (e) {
      message.textContent = [...outcomes, e.message].join(" / ");
    } finally {
      busy = false;
      rows.inert = false;
      controls();
    }
  }
  save.onclick = () => submit("save");
  dismiss.onclick = () => submit("dismiss");
  draw(initial);
  if (!writable) {
    message.textContent =
      "Read-only session. Open bigbrain ui --session SESSION to edit or approve drafts.";
  }
}
