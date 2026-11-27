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
    el("card-title").textContent = "Handoff review";
    el("card-summary").textContent = data.drafts.length + " pending";
    page.append(
      element(
        "p",
        "Destination · " + data.scope + " · " + data.session,
        "card-scope",
      ),
      el("review-template").content.cloneNode(true),
    );
    mountReview(panelApi, data.writable, data.drafts);
    return;
  }
  const r = data.receipt;
  el("card-title").textContent = data.preview
    ? "Context preview"
    : "Recall receipt";
  el("card-summary").textContent = r
    ? r.memories.length + " included · " + r.bytes + " B"
    : "";
  page.append(
    element(
      "p",
      data.scope + " · " +
        (data.preview ? "Preview only" : data.session || "No session pinned"),
      "card-scope",
    ),
  );
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
    head.append(
      element("span", null, "dot " + tone(item.type)),
      element("strong", short(item.text || item.id, 100)),
    );
    row.append(
      head,
      element(
        "small",
        [item.type, item.scope, item.source].filter(Boolean).join(" · "),
      ),
      facts([
        ["Why included", item.reason || "Reason unavailable for this receipt"],
        ["Source", item.source],
        ["Memory ID", item.id],
      ]),
    );
    page.append(row);
  }
  const foot = element("div", null, "receipt-foot");
  foot.append(
    element(
      "small",
      r.bytes + " UTF-8 bytes of " + r.limit + " · " + r.duplicates +
        " duplicates · " + r.budget_skipped + " skipped for budget",
    ),
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
