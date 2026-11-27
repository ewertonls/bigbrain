// SVG is an isolated image in a desktop pane. This transparent native region
// sends picks from the same drawing coordinates back to the workspace.
export default function NetworkInput(props, surface) {
  let start
  const position = (event) => ({
    x: (event.fine?.x ?? event.x + .5) / Math.max(1, surface.columns),
    y: (event.fine?.y ?? event.y + .5) / Math.max(1, surface.rows),
  })
  surface.onPointer((event) => {
    if (event.type === 'down' && event.button === 'left') start = position(event)
    if (event.type !== 'up' || !start) return
    const point = position(event)
    const dx = point.x - start.x, dy = point.y - start.y
    start = undefined
    if (Math.hypot(dx * props.width, dy * props.height) > 8) {
      if (props.page === 'graph') surface.post({ frame: props.frame, pan: [dx, dy] })
      return
    }
    const x = props.left + point.x * props.width, y = props.top + point.y * props.height
    const near = props.nodes.find((n) =>
      Math.hypot(x - n.x, y - n.y) <= 12 ||
      (n.label && x >= n.label[0] && x <= n.label[2] && y >= n.label[1] && y <= n.label[3])
    )
    if (near) {
      surface.post({ frame: props.frame, node: near.id })
      return
    }
    let closest = 7, picked
    for (const edge of props.edges) {
      for (let i = 1; i < edge.points.length; i++) {
        const [ax, ay] = edge.points[i - 1], [bx, by] = edge.points[i]
        const vx = bx - ax, vy = by - ay
        const t = Math.min(1, Math.max(0, ((x - ax) * vx + (y - ay) * vy) / (vx * vx + vy * vy || 1)))
        const distance = Math.hypot(x - ax - t * vx, y - ay - t * vy)
        if (distance < closest) {
          closest = distance
          picked = edge.id
        }
      }
    }
    if (picked) surface.post({ frame: props.frame, edge: picked })
  })
  return surface.elements.Box({ width: '100%', height: '100%' })
}
