const el = (id) => document.getElementById(id);
function element(tag, text, cls) {
  const n = document.createElement(tag);
  if (text != null) n.textContent = text;
  if (cls) n.className = cls;
  return n;
}
function action(label, run, cls) {
  const b = element("button", label, cls);
  b.type = "button";
  b.onclick = async () => {
    try {
      await run();
    } catch (e) {
      el("notice").textContent = e.message;
    }
  };
  return b;
}
const short = (text, n) => text.length > n ? text.slice(0, n - 1) + "…" : text;
const tone = (type) =>
  ({
    decision: "amber",
    procedural: "violet",
    semantic: "teal",
    preference: "teal",
  })[type] || "slate";
const timeLabel = (at) =>
  Number.isFinite(Date.parse(at))
    ? new Date(at).toLocaleString(undefined, {
      dateStyle: "short",
      timeStyle: "short",
    })
    : at;
function heading(title, subtitle) {
  const block = element("div", null, "page-title");
  block.append(element("h1", title));
  if (subtitle) block.append(element("p", subtitle));
  return block;
}
function section(title, button) {
  const box = element("section", null, "group"),
    head = element("div", null, "heading");
  head.append(element("h2", title));
  if (button) head.append(button);
  box.append(head);
  return box;
}
function memoryRow(m, select) {
  const row = action("", select, "memory-row " + tone(m.type)),
    body = element("span", null, "row-copy");
  body.append(
    element("strong", short(m.text, 125)),
    element("small", [m.type, m.scope, timeLabel(m.recorded_at)].join(" · ")),
  );
  row.append(
    element("span", null, "dot"),
    body,
    element("span", "›", "chevron"),
  );
  return row;
}
function facts(pairs) {
  const list = element("dl");
  for (const [label, value] of pairs) {
    list.append(element("dt", label), element("dd", value ?? "Not recorded"));
  }
  return list;
}
