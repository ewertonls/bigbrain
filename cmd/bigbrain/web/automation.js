import { api } from "./client.js";
import { el, element } from "./explorer.js";
const size = (n) =>
  n < 1024 ? n + " B" : Number((n / 1024).toFixed(2)) + " KiB";
function draw({ last, policy, hooks }) {
  const panel = el("last-recall");
  panel.replaceChildren();
  el("preview-text").textContent = "";
  if (last) {
    panel.className = "surface";
    panel.append(element("h2", "Context size"));
    panel.append(
      element(
        "p",
        size(last.bytes) + (last.limit ? " of " + size(last.limit) : ""),
      ),
    );
    if (last.limit) {
      const bar = element("progress");
      bar.max = last.limit;
      bar.value = last.bytes;
      bar.setAttribute(
        "aria-label",
        "Emitted UTF-8 bytes out of the recall limit",
      );
      panel.append(bar);
    }
    panel.append(
      element(
        "p",
        last.memories.length + " included · " + last.duplicates +
          " duplicates · " + last.budget_skipped + " skipped for budget",
      ),
    );
    const details = element("details");
    details.append(
      element("summary", "Inspect recall receipt"),
      element(
        "p",
        last.bytes + " UTF-8 bytes · " + new Date(last.at).toLocaleString(),
      ),
    );
    if (last.fallback) {
      details.append(
        element("p", "Text and relationship fallback · " + last.fallback),
      );
    }
    for (const m of last.memories) details.append(element("p", m.id));
    details.append(element("code", last.hash));
    panel.append(details);
  } else panel.append(element("p", "No recall receipt for this session."));
  const area = el("policies");
  area.replaceChildren();
  for (
    const [key, name, value] of [
      ["startup", "Startup and resume", !policy.disable_startup],
      ["compact", "After compaction", !policy.disable_compact],
      ["global", "Include global in recall", !policy.project_only],
      ["bootstrap", "Use latest reviewed handoff", !policy.disable_bootstrap],
    ]
  ) {
    const label = element("label"), input = element("input");
    input.type = "checkbox";
    input.checked = value;
    input.onchange = () => change(key, input.checked);
    label.append(input, document.createTextNode(name));
    area.append(label);
  }
  area.append(
    element(
      "p",
      "Up to 5 memories · 8 KiB · Per-prompt and subagent recall off",
    ),
  );
  const seen = Object.keys(hooks || {});
  el("hook-state").textContent =
    (seen.length
      ? "Observed in this session: " + seen.join(", ")
      : "No hook delivery observed in this session.") +
    " · Review host authorization in /hooks.";
  el("pause").textContent = policy.paused
    ? "Resume for this session"
    : "Pause for this session";
  el("pause").onclick = () => change("paused", !policy.paused);
}
async function change(setting, enabled) {
  try {
    draw(await api("policy", { setting, enabled }));
  } catch (e) {
    el("connection").textContent = e.message;
    await loadAutomation();
  }
}
export async function loadAutomation() {
  draw(await api("automation", {}));
}
el("preview").onclick = async () => {
  try {
    const packet = await api("preview", {});
    el("preview-text").textContent = packet.text ||
      "No active memories selected.";
  } catch (e) {
    el("preview-text").textContent = e.message;
  }
};
