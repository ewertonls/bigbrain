function drawCard({ detail: result }) {
  const page = el("page");
  page.replaceChildren();
  const data = result?._meta?.bigbrain;
  if (!data) {
    el("notice").textContent = result?.content?.[0]?.text ||
      "No BigBrain result available.";
    return;
  }
  el("notice").textContent = "";
  if (data.kind === "review") {
    el("card-title").textContent = "New knowledge";
    el("card-summary").textContent = "Pending review";
    page.append(el("review-template").content.cloneNode(true));
    const destination = element(
      "small",
      "Destination: " + data.scope,
      "destination",
    );
    destination.title = "Session · " + data.session;
    el("review-actions").prepend(destination);
    mountReview(panelApi, data.writable, data.drafts);
    return;
  }
  const r = data.receipt;
  el("card-title").textContent = data.preview
    ? "Context preview"
    : "Context recalled";
  el("card-summary").textContent = r
    ? r.memories.length + (r.memories.length === 1 ? " memory" : " memories") +
      " · " + r.bytes + " B"
    : "";
  if (!r) {
    page.append(
      element(
        "p",
        "No receipt available. Request a preview with search text or pin the session that emitted context.",
        "empty",
      ),
    );
    return;
  }
  for (const item of data.items || []) {
    const row = element("details", null, "receipt-row"),
      head = element("summary");
    const text = item.text || item.id;
    const [first, ...rest] = text.split(/\n+|(?<=[.!?])\s+/);
    head.append(
      icon(
        item.scope === "global"
          ? "globe"
          : item.type === "procedural"
          ? "repeat"
          : "branch",
      ),
    );
    const body = element("span", null, "receipt-copy");
    body.append(element("strong", short(first, 110)));
    const description = rest.join(" ") || item.source;
    if (description) body.append(element("span", short(description, 135)));
    body.append(
      element(
        "small",
        [displayType(item.type), item.scope].filter(Boolean).join(" · "),
      ),
    );
    head.append(body);
    row.append(
      head,
      facts([
        ["Why included", item.reason || "Reason unavailable for this receipt"],
        ["Source", item.source],
        ["Memory ID", item.id],
        ["Excerpt", item.text],
      ]),
    );
    page.append(row);
  }
  const foot = element("details", null, "receipt-foot");
  foot.append(
    element("summary", "Receipt details"),
    facts([
      ["Scope", data.scope],
      ["Session", data.session || "No session pinned"],
      ["Context size", r.bytes + " UTF-8 bytes of " + r.limit],
      ["Duplicates", r.duplicates],
      ["Skipped for budget", r.budget_skipped],
    ]),
  );
  if (r.fallback) {
    foot.append(
      element("small", "Text and relationship fallback · " + r.fallback),
    );
  }
  page.append(foot);
}
window.addEventListener("bigbrain-result", drawCard);
try {
  await connectHost();
  new ResizeObserver(() => {
    send({
      method: "ui/notifications/size-changed",
      params: {
        height: Math.ceil(document.body.getBoundingClientRect().height),
      },
    });
  }).observe(document.body);
} catch (e) {
  el("notice").textContent = e.message;
}
