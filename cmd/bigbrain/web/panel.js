// The host loads this module and the shared Explorer in one sandboxed document.
const pending = new Map();
let sequence = 0;
let hostOrigin = "*";

function send(message) {
  window.parent.postMessage({ jsonrpc: "2.0", ...message }, hostOrigin);
}

function request(method, params) {
  const id = ++sequence;
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      pending.delete(id);
      reject(new Error("The desktop host did not respond. Reopen the panel."));
    }, 30000);
    pending.set(id, { resolve, reject, timer });
    send({ id, method, params });
  });
}

window.addEventListener("message", ({ source, origin, data }) => {
  if (
    source !== window.parent || data?.jsonrpc !== "2.0" ||
    (hostOrigin !== "*" && origin !== hostOrigin)
  ) return;
  if (data.method === "ui/resource-teardown") {
    send({ id: data.id, result: {} });
    return;
  }
  const call = pending.get(data.id);
  if (!call || data.method || (!data.error && !("result" in data))) return;
  if (origin && origin !== "null") hostOrigin = origin;
  pending.delete(data.id);
  clearTimeout(call.timer);
  if (data.error) call.reject(new Error(data.error.message));
  else call.resolve(data.result);
});

try {
  await request("ui/initialize", {
    appInfo: { name: "BigBrain Explorer", version: "1.0.0" },
    appCapabilities: {},
    protocolVersion: "2026-01-26",
  });
  send({ method: "ui/notifications/initialized" });
  await mountExplorer(async (operation, args = {}) => {
    const name = operation === "status" ? "explorer" : "explorer_" + operation;
    const result = await request("tools/call", { name, arguments: args });
    if (result.isError) {
      throw new Error(result.content?.[0]?.text || "Explorer request failed.");
    }
    return result._meta.bigbrain;
  });
} catch (error) {
  document.getElementById("connection").textContent = error.message;
}
