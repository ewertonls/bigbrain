import { api } from "./client.js";
import { el, mountExplorer } from "./explorer.js";

await mountExplorer(api);
async function navigate(name) {
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
