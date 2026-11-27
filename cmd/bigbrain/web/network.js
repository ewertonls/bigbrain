// One visible neighborhood per canvas. Arrowheads always point to the stored target.
function network(history, select, compact = false) {
  const svg = (tag, attrs = {}) => {
    const node = document.createElementNS("http://www.w3.org/2000/svg", tag);
    for (const [key, value] of Object.entries(attrs)) {
      node.setAttribute(key, value);
    }
    return node;
  };
  const root = history.memory, neighbors = new Map();
  for (const item of history.related) {
    if (item.id !== root.id) neighbors.set(item.id, item);
  }
  const width = 480, height = compact ? 190 : 272, cy = height / 2;
  const diagram = svg("svg", {
    viewBox: "0 0 " + width + " " + height,
    class: "network",
    role: "group",
    "aria-label": "Stored relationships around " + root.text,
  });
  const title = svg("title");
  title.textContent = "Stored memory neighborhood";
  const defs = svg("defs"),
    marker = svg("marker", {
      id: "edge-tip",
      viewBox: "0 0 8 8",
      refX: 7,
      refY: 4,
      markerWidth: 6,
      markerHeight: 6,
      orient: "auto",
    });
  marker.append(svg("path", { d: "M0,0 L8,4 L0,8", class: "arrow" }));
  defs.append(marker);
  diagram.append(title, defs);
  const points = new Map([[root.id, { x: 240, y: cy, item: root }]]);
  const others = [...neighbors.values()];
  others.forEach((item, i) => {
    const angle = 2 * Math.PI * i / others.length +
      (others.length > 2 ? Math.PI / 6 : 0);
    points.set(item.id, {
      x: 240 + 147 * Math.cos(angle),
      y: cy + (cy - 40) * Math.sin(angle),
      item,
    });
  });
  function activate(node, name, run) {
    node.setAttribute("role", "button");
    node.setAttribute("tabindex", "0");
    node.setAttribute("aria-label", name);
    node.onclick = run;
    node.onkeydown = (e) => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        run();
      }
    };
  }
  const seen = new Map();
  for (const { path: p } of history.related) {
    const a = points.get(p.from_id), b = points.get(p.to_id);
    const key = [p.from_id, p.to_id].sort().join("|"), n = seen.get(key) || 0;
    seen.set(key, n + 1);
    const dx = b.x - a.x, dy = b.y - a.y, length = Math.hypot(dx, dy);
    let d;
    if (!length) {
      d = "M" + (a.x - 6) + " " + (a.y - 7) + " c-30 -32 38 -32 13 0";
    } else {
      const bend = n * 24 * (p.from_id < p.to_id ? 1 : -1);
      d = "M" + (a.x + dx / length * 9) + " " + (a.y + dy / length * 9) + " Q" +
        ((a.x + b.x) / 2 - dy / length * bend) + " " +
        ((a.y + b.y) / 2 + dx / length * bend) + " " +
        (b.x - dx / length * 11) + " " + (b.y - dy / length * 11);
    }
    const g = svg("g", { class: "graph-edge" });
    g.append(
      svg("path", { d, class: "edge-hit" }),
      svg("path", { d, class: "edge-line", "marker-end": "url(#edge-tip)" }),
    );
    activate(g, p.type + " · " + p.source, () => select("edge", p));
    diagram.append(g);
  }
  for (const [id, { x, y, item }] of points) {
    const g = svg("g", {
      class: "graph-node " + tone(item.type),
      transform: "translate(" + x + " " + y + ")",
    });
    const full = item.text || item.excerpt || item.source || id;
    const name = svg("title");
    name.textContent = full;
    const label = svg("text", { y: 26, "text-anchor": "middle" });
    label.textContent = short(full, compact ? 19 : 23);
    if (id === root.id) g.append(svg("circle", { r: 13, class: "halo" }));
    g.append(name, svg("circle", { r: id === root.id ? 7 : 5 }), label);
    activate(g, full, () => select("node", item));
    diagram.append(g);
  }
  return diagram;
}

// Scale within the original SVG coordinate system. Drag the background to pan.
function graphControls(diagram) {
  const original = diagram.viewBox.baseVal,
    width = original.width,
    height = original.height;
  let zoom = 1, x = width / 2, y = height / 2, drag;
  function update() {
    const w = width / zoom, h = height / zoom;
    diagram.setAttribute("viewBox", [x - w / 2, y - h / 2, w, h].join(" "));
  }
  diagram.onpointerdown = (e) => {
    if (e.target !== diagram || e.button !== 0) return;
    drag = { x: e.clientX, y: e.clientY };
    diagram.setPointerCapture(e.pointerId);
  };
  diagram.onpointermove = (e) => {
    if (!drag) return;
    const scale = diagram.getScreenCTM().a;
    x -= (e.clientX - drag.x) / scale;
    y -= (e.clientY - drag.y) / scale;
    drag = { x: e.clientX, y: e.clientY };
    update();
  };
  diagram.onlostpointercapture = () => {
    drag = null;
  };
  const buttons = element("div", null, "graph-controls");
  for (
    const [name, ratio] of [["Zoom out", .8], ["Zoom in", 1.25], ["Fit", 0]]
  ) {
    buttons.append(action(name, () => {
      if (ratio) zoom = Math.min(3, Math.max(.6, zoom * ratio));
      else {
        zoom = 1;
        x = width / 2;
        y = height / 2;
      }
      update();
    }));
  }
  return buttons;
}
