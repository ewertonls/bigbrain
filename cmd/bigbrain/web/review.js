import { api } from "./client.js";
import { action, el, element } from "./app.js";
const selected = new Set();
let drafts = [], busy = false;
const states = {
  draft: "Draft",
  saving: "Saving / awaiting confirmation",
  partial: "Partially saved",
  uncertain: "Save status unknown",
  saved: "Saved",
  dismissed: "Dismissed",
};
function field(label, value, multiline = false) {
  const wrapper = element("label", label);
  const input = element(multiline ? "textarea" : "input");
  input.value = value || "";
  wrapper.append(input);
  return { wrapper, input };
}
function draftView(d) {
  const section = element("article");
  section.className = "surface";
  const title = element("label");
  const check = element("input");
  check.type = "checkbox";
  check.checked = selected.has(d.id);
  check.disabled = busy || ["saved", "dismissed"].includes(d.status);
  check.onchange = () => {
    if (check.checked) selected.add(d.id);
    else selected.delete(d.id);
    buttons();
  };
  title.append(
    check,
    document.createTextNode(
      " " + (states[d.status] || d.status) + " · " +
        (d.content.memory.type === "procedural"
          ? "Procedure"
          : d.content.memory.type),
    ),
  );
  section.append(
    title,
    element("h2", d.content.memory.text),
    element("p", "Source · " + d.content.memory.source),
  );
  const ev = d.content.evidence;
  if (ev) {
    const evidence = element("div");
    evidence.className = "thread";
    evidence.append(
      element("small", "Supporting observation"),
      element("p", ev.text),
      element("p", "Source · " + ev.source),
    );
    section.append(evidence);
  }
  for (
    const [title, items] of [["Open questions", d.content.open_questions], [
      "Next steps",
      d.content.next_steps,
    ]]
  ) {
    if (items?.length) {
      section.append(element("h3", title));
      const list = element("ul");
      for (const text of items) list.append(element("li", text));
      section.append(list);
    }
  }
  section.append(element("p", "Save destination · " + d.origin.scope));
  const details = element("details");
  details.append(element("summary", "Inspect the planned writes"));
  const operations = element("ol");
  for (
    const text of [
      "Record memory",
      ...(ev
        ? [
          "Record supporting observation",
          "Link the decision to the observation · justified_by",
        ]
        : []),
    ]
  ) operations.append(element("li", text));
  details.append(operations);
  section.append(details);
  if (d.status === "draft") {
    const edit = element("details");
    edit.append(element("summary", "Edit draft"));
    const content = structuredClone(d.content), inputs = [];
    for (
      const [kind, m] of [["Memory", content.memory], [
        "Evidence",
        content.evidence,
      ]]
    ) {
      if (!m) continue;
      for (const key of ["text", "type", "source"]) {
        const { wrapper, input } = field(
          kind + " " + key,
          m[key],
          key === "text",
        );
        edit.append(wrapper);
        inputs.push(() => m[key] = input.value);
      }
    }
    edit.append(action("Apply edits", async () => {
      for (const read of inputs) read();
      await api("edit", { id: d.id, revision: d.revision, content });
      await loadReview();
    }));
    section.append(edit);
  }
  return section;
}
function buttons() {
  el("save-selected").disabled = busy || !selected.size;
  el("dismiss-selected").disabled = busy || !selected.size ||
    drafts.some((d) => selected.has(d.id) && d.status !== "draft");
}
export async function loadReview() {
  const ds = await api("drafts", {});
  drafts = ds;
  for (const id of selected) {
    if (
      !ds.some((d) => d.id === id && !["saved", "dismissed"].includes(d.status))
    ) selected.delete(id);
  }
  el("drafts").replaceChildren(...ds.map(draftView));
  buttons();
  if (!ds.length) {
    el("review-message").textContent = "No drafts in this session.";
  } else if (
    el("review-message").textContent === "No drafts in this session."
  ) el("review-message").textContent = "";
}
async function apply(actionName) {
  if (busy) return;
  busy = true;
  buttons();
  const outcomes = [];
  for (const d of drafts.filter((d) => selected.has(d.id))) {
    try {
      const item = await api(actionName, { id: d.id, revision: d.revision });
      selected.delete(d.id);
      outcomes.push(states[item.status] + " · " + d.content.memory.text);
    } catch (error) {
      outcomes.push(d.content.memory.text + " · " + error.message);
    }
  }
  busy = false;
  el("review-message").textContent = outcomes.join(" / ");
  await loadReview();
}
el("save-selected").onclick = () =>
  apply("save").catch((e) => el("review-message").textContent = e.message);
el("dismiss-selected").onclick = () =>
  apply("dismiss").catch((e) => el("review-message").textContent = e.message);
