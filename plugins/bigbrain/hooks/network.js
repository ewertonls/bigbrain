// Graph data is the union of the visible histories. A stored edge may occur
// twice (or several edges may join the same pair), so use relationship IDs.
export function neighborhood(histories) {
  const nodes = new Map(), edges = new Map()
  for (const { memory, related } of histories) {
    nodes.set(memory.id, memory)
    for (const item of related) {
      if (!nodes.has(item.id)) nodes.set(item.id, item)
      edges.set(item.path.relationship_id, item.path)
    }
  }
  return { nodes, edges }
}

export function color(type) {
  return ({ decision: '#d97706', procedural: '#8b5cf6', semantic: '#14b8a6', preference: '#14b8a6' })[
    type
  ] || '#71829b'
}

export function short(value, limit) {
  const chars = Array.from(String(value ?? ''))
  return chars.length > limit ? chars.slice(0, limit - 1).join('') + '…' : chars.join('')
}

function escapeXML(value) {
  return String(value).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(
    /"/g,
    '&quot;',
  )
}

export const brand =
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24"><style>g{stroke:#282828}@media(prefers-color-scheme:dark){g{stroke:#eee}}</style><g fill="none" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M10 4a3 3 0 0 0-5 2 4 4 0 0 0-2 6 4 4 0 0 0 2 6 3 3 0 0 0 5 2V4Zm0 4H8m2 7H7m7-12v4l3 2h3m-6 4h6m-6 7v-3l3-1"/><circle cx="20" cy="8.5" r="1.5"/><circle cx="20.5" cy="13" r="1.5"/><circle cx="18.5" cy="16" r="1.5"/></g></svg>'

// Center the focused memory and spread its neighbors around it. A second
// ring keeps an overview with several neighborhoods legible.
export function diagram(ui, surface, graph, focus, zoom, compact, columns, panel) {
  const nodes = [...graph.nodes.values()]
  if (!nodes.length) return ui.Text({ dimColor: true, children: 'No memories in this scope.' })
  const center = graph.nodes.get(focus) || nodes[0]
  const others = nodes.filter((m) => m.id !== center.id)
  const caption = (m) => m.text || m.excerpt || m.id
  if (surface === 'terminal') {
    // The terminal has no SVG element. Show the same topology and direction
    // with the actual endpoints (including isolated nodes in the preview).
    const lines = [], linked = new Set()
    for (const edge of graph.edges.values()) {
      linked.add(edge.from_id)
      linked.add(edge.to_id)
      const from = graph.nodes.get(edge.from_id), to = graph.nodes.get(edge.to_id)
      const cap = Math.max(7, Math.floor((columns - 7) / 2))
      lines.push(
        ui.Text({
          children: [
            ui.Text({ color: color(from.type), children: '● ' }),
            short(caption(from), cap),
            ' → ',
            ui.Text({ color: color(to.type), children: '● ' }),
            short(caption(to), cap),
          ],
        }),
      )
    }
    for (const m of nodes) {
      if (!linked.has(m.id)) {
        lines.push(
          ui.Text({
            children: [
              ui.Text({ color: color(m.type), children: '● ' }),
              short(caption(m), columns - 4),
            ],
          }),
        )
      }
    }
    return ui.Box({
      flexDirection: 'column',
      justifyContent: 'center',
      alignItems: 'center',
      paddingY: 1,
      children: lines,
    })
  }
  const width = Math.min(500, Math.max(245, columns * 7)), height = compact ? 155 : 205
  const points = new Map([[center.id, { x: width / 2, y: height / 2 }]])
  others.forEach((m, i) => {
    const ring = Math.floor(i / 8), n = Math.min(8, others.length - ring * 8)
    const angle = (i % 8) * Math.PI * 2 / n - Math.PI / 2 + ring * .38
    points.set(m.id, {
      x: width / 2 + Math.cos(angle) * width * (ring ? .46 : .34),
      y: height / 2 + Math.sin(angle) * height * (ring ? .44 : .32),
    })
  })
  const w = width / zoom, h = height / zoom
  const left = (width - w) / 2 + (compact ? 0 : panel.panX * width)
  const top = (height - h) / 2 + (compact ? 0 : panel.panY * height)
  const frame = [panel.page, panel.scope, focus, zoom, left, top, ...graph.edges.keys()].join('|')
  const hit = { page: panel.page, frame, width: w, height: h, left, top, nodes: [], edges: [] }
  const svg = [
    '<svg xmlns="http://www.w3.org/2000/svg" width="' + width + '" height="' + height + '" viewBox="' +
    [left, top, w, h].join(' ') +
    '"><style>text{font:12px system-ui;fill:#222}@media(prefers-color-scheme:dark){text{fill:#eee}}</style>',
    '<defs><marker id="arrow" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="5" markerHeight="5" orient="auto"><path d="M0 0L8 4 0 8" fill="#849097"/></marker></defs>',
  ]
  const pairs = new Map()
  for (const edge of graph.edges.values()) {
    const a = points.get(edge.from_id), b = points.get(edge.to_id)
    const pair = [edge.from_id, edge.to_id].sort().join('/'), n = pairs.get(pair) || 0
    pairs.set(pair, n + 1)
    const dx = b.x - a.x, dy = b.y - a.y, len = Math.hypot(dx, dy)
    const bend = 14 * n * (edge.from_id < edge.to_id ? 1 : -1)
    const x1 = len ? a.x + 7 * dx / len : a.x - 6, y1 = len ? a.y + 7 * dy / len : a.y - 5
    const x2 = len ? b.x - 9 * dx / len : a.x + 6, y2 = len ? b.y - 9 * dy / len : a.y - 5
    const cx = len ? (a.x + b.x) / 2 - bend * dy / len : a.x
    const cy = len ? (a.y + b.y) / 2 + bend * dx / len : a.y - 55
    const d = 'M' + x1 + ' ' + y1 + ' Q' + cx + ' ' + cy + ' ' + x2 + ' ' + y2
    const curve = []
    for (let i = 0; i <= 8; i++) {
      const t = i / 8, u = 1 - t
      curve.push([u * u * x1 + 2 * u * t * cx + t * t * x2, u * u * y1 + 2 * u * t * cy + t * t * y2])
    }
    hit.edges.push({ id: edge.relationship_id, points: curve })
    svg.push(
      '<path d="' + d + '" fill="none" stroke="' +
        (!compact && edge.relationship_id === panel.edge?.relationship_id ? '#14b8a6' : '#849097') +
        '" stroke-width="1.6" marker-end="url(#arrow)"><title>' +
        escapeXML(edge.type) + '</title></path>',
    )
  }
  const labels = []
  for (const [id, { x, y }] of points) {
    const target = { id, x, y }
    hit.nodes.push(target)
    const m = graph.nodes.get(id), tint = color(m.type), name = caption(m)
    svg.push(
      '<circle cx="' + x + '" cy="' + y + '" r="' + (id === center.id ? 7 : 5) + '" fill="' + tint +
        '"><title>' + escapeXML(short(name, 160)) + '</title></circle>',
    )
    if (!compact && id === panel.selected && !panel.edge) {
      svg.push(
        '<circle cx="' + x + '" cy="' + y + '" r="11" fill="none" stroke="' + tint +
          '" stroke-width="1.5"/>',
      )
    }
    if (labels.length >= (compact ? 3 : 6)) continue
    const label = short(name, compact ? 22 : 26), lw = Array.from(label).length * 6.8 + 12
    const left = Math.max(1, Math.min(width - lw - 1, x - lw / 2)), top = y + 10
    if (
      labels.some((r) => left < r.left + r.width && left + lw > r.left && Math.abs(top - r.top) < 19)
    ) continue
    labels.push({ left, top, width: lw })
    target.label = [left, top, left + lw, top + 18]
    svg.push(
      '<rect x="' + left + '" y="' + top + '" width="' + lw + '" height="18" rx="4" fill="' + tint +
        '" fill-opacity=".17"/><text text-anchor="middle" x="' + (left + lw / 2) + '" y="' + (top + 13) +
        '">' + escapeXML(label) + '</text>',
    )
  }
  svg.push('</svg>')
  const image = ui.Svg({
    source: svg.join(''),
    alt: 'Knowledge network · ' + nodes.length + (nodes.length === 1 ? ' memory · ' : ' memories · ') +
      graph.edges.size + (graph.edges.size === 1 ? ' relationship' : ' relationships'),
  })
  if (surface !== 'desktop') return image
  panel.graphFrame = frame
  return ui.Box({
    position: 'relative',
    flexDirection: 'column',
    alignSelf: 'center',
    children: [
      image,
      ui.Box({
        position: 'absolute',
        left: 0,
        top: 0,
        right: 0,
        bottom: 0,
        children: [
          ui.Client({
            key: 'network-input',
            module: './network-input.js',
            width: '100%',
            height: '100%',
            props: hit,
          }),
        ],
      }),
    ],
  })
}
