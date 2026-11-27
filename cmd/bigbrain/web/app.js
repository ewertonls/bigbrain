import { api } from "./client.js";
export const el = (id) => document.getElementById(id);
export function element(tag, text) {
  const node = document.createElement(tag);
  if (text != null) node.textContent = text;
  return node;
}
export function action(label, run) {
  const b = element("button", label);
  b.type = "button";
  b.onclick = async () => {
    try {
      await run();
    } catch (error) {
      el("connection").textContent = error.message;
    }
  };
  return b;
}
export function label(m) {
  const text = (m?.text || "").split("\n").find((line) => line.trim()) ||
    m?.id || "Unavailable";
  return text.length > 90 ? text.slice(0, 89) + "…" : text;
}
export const app = { binding: null, page: "explorer" };
export function scope() {
  const globalOnly = el("scope").value === "global";
  return {
    scope: globalOnly ? "global" : app.binding.scope,
    include_global: !globalOnly && el("global").checked,
  };
}
let generation = 0,
  inspectSerial = 0,
  matches = [],
  selected = null,
  focused = null,
  next = "",
  truncated = false;
const nodes = new Map(), edges = new Map();
function clear() {
  el("explorer").classList.remove("details-open");
  generation++;
  inspectSerial++;
  matches = [];
  selected = focused = null;
  edges.clear();
  nodes.clear();
  next = "";
  el("selection").hidden = el("results").hidden = true;
  el("history").replaceChildren();
  el("search-status").textContent = "";
}
function addEdge(edge) {
  if (edges.has(edge.relationship_id)) return;
  const ids = new Set(
    [edge.from_id, edge.to_id].filter((id) => !nodes.has(id)),
  );
  if (nodes.size + ids.size > 50 || edges.size >= 100) {
    truncated = true;
    return;
  }
  for (const id of ids) {
    nodes.set(
      id,
      matches.find((m) => m.id === id) || { id },
    );
  }
  edges.set(edge.relationship_id, edge);
}
function nodeButton(id) {
  const b = action(label(nodes.get(id)), () => inspect(id));
  b.className = "node";
  b.setAttribute("aria-pressed", focused?.id === id);
  return b;
}
function drawPath() {
  const area = el("paths");
  area.replaceChildren();
  if (!edges.size) area.append(nodeButton(selected.id));
  else if (el("mode").value === "Graph") {
    for (const edge of edges.values()) {
      const row = element("div");
      row.className = "edge";
      row.append(
        nodeButton(edge.from_id),
        element("small", "— " + edge.type + " →"),
        nodeButton(edge.to_id),
      );
      area.append(row);
      if (edge.direction === "reverse") {
        area.append(
          element("small", "Retrieval traversed this edge in reverse."),
        );
      }
    }
  } else {
    const table = element("table"), head = table.createTHead().insertRow();
    for (const h of ["From", "Relationship", "To", "Traversal"]) {
      head.append(element("th", h));
    }
    const body = table.createTBody();
    for (const e of edges.values()) {
      const row = body.insertRow();
      row.insertCell().append(nodeButton(e.from_id));
      row.insertCell().textContent = e.type;
      row.insertCell().append(nodeButton(e.to_id));
      row.insertCell().textContent = e.direction;
    }
    area.append(table);
  }
  el("counts").textContent = nodes.size +
    (nodes.size === 1 ? " node · " : " nodes · ") + edges.size +
    (edges.size === 1 ? " link shown" : " links shown") +
    (truncated ? " · Display limit reached" : "");
}
async function inspect(id) {
  const stamp = generation, serial = ++inspectSerial;
  let m = nodes.get(id);
  el("history").replaceChildren();
  next = "";
  if (!m?.text) {
    el("inspector").textContent = "Loading memory…";
    const q = await api("query", { ...scope(), id, limit: 1 });
    if (stamp !== generation || serial !== inspectSerial) return;
    m = q.matches?.[0] || { id };
    nodes.set(id, m);
  }
  focused = m;
  drawPath();
  const panel = el("inspector");
  panel.replaceChildren(
    element("h2", m.type === "procedural" ? "Procedure" : m.type || "Memory"),
    element("small", m.scope || id),
  );
  const p = element("p", m.text || "Memory text unavailable.");
  p.className = "memory";
  panel.append(p);
  if (m.source) panel.append(element("p", "Source · " + m.source));
  const buttons = element("div");
  buttons.className = "actions";
  if (m.text) {
    for (
      const [name, value] of [["Copy reference", m.id + " · " + m.source], [
        "Copy text",
        m.text,
      ], [
        "Copy context",
        "Retrieved evidence · " + m.scope + " · " + m.id + " · " + m.source +
        "\n" + m.text,
      ]]
    ) {
      buttons.append(action(name, () => navigator.clipboard.writeText(value)));
    }
  }
  buttons.append(
    action("View history", () => historyPage(id, stamp, serial, "")),
  );
  panel.append(buttons);
}
async function historyPage(id, stamp, serial, after) {
  const h = await api("history", { ...scope(), id, after, limit: 10 });
  if (
    stamp !== generation || serial !== inspectSerial ||
    (after && next !== after)
  ) return;
  for (const item of h.related || []) addEdge(item.path);
  next = h.next_cursor || "";
  drawPath();
  el("history").replaceChildren(element("p", "History · " + h.status));
  if (next && !truncated) {
    el("history").append(
      action(
        "Load more relationships",
        () => historyPage(id, stamp, serial, next),
      ),
    );
  }
}
function select(m) {
  selected = m;
  nodes.clear();
  edges.clear();
  truncated = false;
  nodes.set(m.id, m);
  for (
    const e of [
      ...(m.reason.steps || []),
      ...(m.reason.path ? [m.reason.path] : []),
    ]
  ) addEdge(e);
  el("selection").hidden = false;
  el("reason").textContent = [
    ({
      text: "Matched the search text",
      id: "Selected by memory ID",
      vector: "Matched by meaning",
      hybrid: "Matched by combined retrieval signals",
      relationship: "Reached through a related memory",
    })[m.reason.kind] || m.reason.kind,
    (m.reason.terms || []).join(", "),
    m.reason.hops
      ? m.reason.hops + (m.reason.hops === 1 ? " hop" : " hops")
      : "",
  ].filter(Boolean).join(" · ");
  drawResults();
  return inspect(m.id);
}
function drawResults() {
  const needle = el("filter").value.toLowerCase(), area = el("matches");
  area.replaceChildren();
  for (
    const m of matches.filter((m) => m.text.toLowerCase().includes(needle))
  ) {
    const b = action(label(m), () => {
      el("explorer").classList.add("details-open");
      return select(m);
    });
    b.setAttribute("aria-pressed", selected?.id === m.id);
    area.append(b);
  }
}
async function search() {
  clear();
  const text = el("query").value.trim();
  if (!text || !app.binding) return;
  const stamp = generation;
  el("search-status").textContent = "Searching…";
  try {
    const q = await api("query", {
      ...scope(),
      text,
      mode: "hybrid",
      limit: 20,
    });
    if (stamp !== generation) return;
    el("connection").textContent = "Local · daemon running";
    matches = q.matches || [];
    el("filter").value = "";
    el("results").hidden = !matches.length;
    el("search-status").textContent = matches.length + " results loaded" +
      (q.fallback
        ? " · Using text and relationships (" + q.fallback + ")"
        : "");
    if (matches.length) await select(matches[0]);
  } catch (e) {
    if (stamp === generation) el("search-status").textContent = e.message;
  }
}
el("search").onsubmit = (e) => {
  e.preventDefault();
  search();
};
let projectGlobal = true;
el("scope").onchange = () => {
  const onlyGlobal = el("scope").value === "global";
  if (onlyGlobal) {
    projectGlobal = el("global").checked;
    el("global").checked = false;
  } else el("global").checked = projectGlobal;
  el("global").disabled = onlyGlobal;
  search();
};
el("back-results").onclick = () => {
  el("explorer").classList.remove("details-open");
  el("filter").focus();
};
el("global").onchange = search;
el("filter").oninput = drawResults;
el("mode").onchange = drawPath;
try {
  const { binding, daemon } = await api("status");
  app.binding = binding;
  el("scope").options[0].text = "Project · " + binding.scope.slice(8);
  el("global").checked = binding.include_global;
  el("connection").textContent = "Local · daemon " + daemon.state;
} catch (e) {
  el("connection").textContent = e.message;
}
async function navigate(name) {
  app.page = name;
  for (const b of document.querySelectorAll("nav [data-page]")) {
    const selected = b.dataset.page === name;
    b.setAttribute("aria-pressed", selected);
    el(b.dataset.page).hidden = !selected;
  }
  document.querySelector(".scope").hidden = name !== "explorer";
  if (name === "automation") {
    const { loadAutomation } = await import("./automation.js");
    await loadAutomation();
  }
  if (name === "review") {
    const { loadReview } = await import("./review.js");
    await loadReview();
  }
}
document.querySelector("nav").onclick = (e) => {
  const b = e.target.closest("[data-page]");
  if (b) {
    navigate(b.dataset.page).catch((error) => {
      el("connection").textContent = error.message;
    });
  }
};
