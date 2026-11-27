const svg = (tag, attrs = {}) => {
  const node = document.createElementNS("http://www.w3.org/2000/svg", tag);
  for (const [key, value] of Object.entries(attrs)) {
    node.setAttribute(key, value);
  }
  return node;
};

// Use the same stored edge once when two visible neighborhoods overlap.
function neighborhood(histories) {
  const nodes = new Map(), edges = new Map();
  for (const h of histories) {
    nodes.set(h.memory.id, h.memory);
    for (const item of h.related) {
      if (!nodes.has(item.id)) nodes.set(item.id, item);
      edges.set(item.path.relationship_id, item.path);
    }
  }
  return { nodes, edges };
}

// A bounded spring layout keeps linked memories together. Initial positions
// are deterministic and the simulation finishes before the first paint.
function positionNetwork(nodes, edges, width, height) {
  const points = new Map(), all = [];
  for (const item of nodes.values()) {
    const angle = all.length * 2.4, radius = .22 + .16 * (all.length % 3);
    const p = {
      item,
      x: width * (.5 + Math.cos(angle) * radius * .76),
      y: height * (.5 + Math.sin(angle) * radius * .76),
      degree: 0,
    };
    points.set(item.id, p);
    all.push(p);
  }
  for (const e of edges.values()) {
    points.get(e.from_id).degree++;
    points.get(e.to_id).degree++;
  }
  for (let pass = 0; pass < 55; pass++) {
    for (const p of all) {
      p.dx = (width / 2 - p.x) * .014;
      p.dy = (height / 2 - p.y) * .023;
    }
    for (let i = 0; i < all.length; i++) {
      for (let j = 0; j < i; j++) {
        const a = all[i], b = all[j], x = a.x - b.x, y = a.y - b.y;
        const force = 415 / Math.max(90, x * x + y * y);
        a.dx += x * force;
        a.dy += y * force;
        b.dx -= x * force;
        b.dy -= y * force;
      }
    }
    for (const e of edges.values()) {
      const a = points.get(e.from_id),
        b = points.get(e.to_id),
        x = b.x - a.x,
        y = b.y - a.y;
      const force = .065 * (1 - 86 / Math.max(1, Math.hypot(x, y)));
      a.dx += x * force;
      a.dy += y * force;
      b.dx -= x * force;
      b.dy -= y * force;
    }
    for (const p of all) {
      p.x += p.dx;
      p.y += p.dy;
    }
  }
  // Fit after relaxing. Clamping at every iteration stacks leaves on the edges.
  for (const [key, size, margin] of [["x", width, 32], ["y", height, 32]]) {
    const values = all.map((p) => p[key]),
      min = Math.min(...values),
      max = Math.max(...values);
    const scale = Math.min(1.65, (size - 2 * margin) / Math.max(1, max - min));
    for (const p of all) p[key] = size / 2 + (p[key] - (min + max) / 2) * scale;
  }
  return points;
}
function network(graph, root, select, compact = false) {
  const width = 560, height = compact ? 210 : 264;
  const diagram = svg("svg", {
    viewBox: "0 0 " + width + " " + height,
    class: "network",
    role: "group",
    "aria-label": "Stored memory network",
  });
  const defs = svg("defs"),
    marker = svg("marker", {
      id: "edge-tip",
      viewBox: "0 0 8 8",
      refX: 7,
      refY: 4,
      markerWidth: 5,
      markerHeight: 5,
      orient: "auto",
    });
  marker.append(svg("path", { d: "M0 0 8 4 0 8", class: "arrow" }));
  defs.append(marker);
  diagram.append(defs);
  const points = positionNetwork(graph.nodes, graph.edges, width, height);
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
  for (const p of graph.edges.values()) {
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
  const labeled = new Set(
    [...points.entries()].sort((a, b) => b[1].degree - a[1].degree).slice(
      0,
      compact ? 3 : 5,
    ).map(([id]) => id),
  );
  const labels = [];
  for (const [id, { x, y, item, degree }] of points) {
    const g = svg("g", {
      class: "graph-node " + tone(item.type),
      transform: "translate(" + x + " " + y + ")",
    });
    const full = item.text || item.excerpt || item.source || id;
    const name = svg("title");
    name.textContent = full;
    const radius = id === root.id ? 8 : Math.min(8, 3.5 + degree);
    g.append(name, svg("circle", { r: radius }));
    if (labeled.has(id)) {
      const text = short(full, compact ? 21 : 25), w = text.length * 7 + 10;
      const left = Math.max(2, Math.min(width - w - 2, x - w / 2));
      for (const top of [y + radius + 3, y - radius - 22]) {
        if (
          top < 0 || top + 20 > height ||
          labels.some((r) =>
            left < r.right && left + w > r.left && top < r.bottom &&
            top + 20 > r.top
          )
        ) continue;
        labels.push({ left, right: left + w, top, bottom: top + 20 });
        const label = svg("text", {
          x: left - x + w / 2,
          y: top - y + 14,
          "text-anchor": "middle",
        });
        label.textContent = text;
        g.append(
          svg("rect", {
            x: left - x,
            y: top - y,
            width: w,
            height: 20,
            rx: 4,
            class: "node-label",
          }),
          label,
        );
        break;
      }
    }
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

// Retrieval arrows follow the recorded walk, which can oppose stored direction.
function retrievalDiagram(match, steps, matches) {
  const ids = [];
  if (steps.length) {
    const first = steps[0];
    ids.push(first.direction === "reverse" ? first.to_id : first.from_id);
    for (const step of steps) {
      ids.push(step.direction === "reverse" ? step.from_id : step.to_id);
    }
  } else ids.push(match.id);
  const width = Math.max(230, ids.length * 152), height = 108;
  const diagram = svg("svg", {
    class: "retrieval-diagram",
    viewBox: "0 0 " + width + " " + height,
    role: "img",
    "aria-label": steps.length + " traversed relationships for " + match.text,
  });
  const x = (i) => width * (i + .5) / ids.length;
  ids.forEach((id, i) => {
    const m = matches.find((m) => m.id === id), y = 45;
    if (i) {
      const a = x(i - 1) + 9, b = x(i) - 10;
      diagram.append(
        svg("path", {
          d: "M" + a + " " + y + "H" + b + "m-6 -4 6 4-6 4",
          class: "traversed",
        }),
      );
    }
    const node = svg("g", { class: "graph-node " + tone(m?.type) }),
      title = svg("title"),
      label = svg("text", { x: x(i), y: 70, "text-anchor": "middle" });
    title.textContent = m?.text || id;
    label.textContent = short(m?.text || id, 16);
    node.append(
      title,
      svg("circle", { cx: x(i), cy: y, r: id === match.id ? 9 : 6 }),
      label,
    );
    diagram.append(node);
  });
  return diagram;
}
