import { request, startViewer, stopViewer } from './viewer.js'
import { workspace } from './workspace.js'
import { draw } from './views.js'

const paneId = 'bigbrain-explorer'
let active

async function disconnect(panel = active) {
  if (panel === active) active = undefined
  await stopViewer(panel)
}

async function sameSession($, panel) {
  return panel.session === await $.session.id() && panel.cwd === await $.session.cwd()
}

// A button keeps the specific panel and draft revision that the person saw.
// A late response from a closed pane must never become another pane's state.
async function act($, panel, action) {
  if (active !== panel || panel.busy) return
  panel.busy = true
  panel.error = ''
  $.ui.invalidate('ui.render')
  try {
    if (!await sameSession($, panel)) {
      await disconnect(panel)
      return
    }
    if (active === panel) await action()
  } catch (err) {
    panel.error = err.message || String(err)
  } finally {
    panel.busy = false
    $.ui.invalidate('ui.render')
  }
}

async function open($) {
  const previous = active
  const panel = {
    session: '',
    cwd: '',
    page: 'setup',
    term: '',
    error: '',
    busy: false,
    trail: [],
  }
  active = panel
  await stopViewer(previous)
  const [session, cwd] = await Promise.all([$.session.id(), $.session.cwd()])
  panel.session = session
  panel.cwd = cwd
  if (active !== panel) return
  await $.ui.open({
    id: paneId,
    title: 'BigBrain Explorer',
    focus: true,
    closeOnEscape: true,
    columns: 56,
    rows: 26,
  })
  await act($, panel, async () => {
    try {
      await startViewer($, panel)
      if (panel !== active) return
      const status = await request($, panel, 'status')
      if (status.binding.session_id !== panel.session) throw new Error('Viewer session changed.')
      panel.binding = status.binding
      panel.daemon = status.daemon?.state || 'unknown'
      panel.scope = status.binding.include_global === false ? 'project' : 'combined'
      panel.actions = workspace($, panel)
      await panel.actions.overview()
    } catch (err) {
      await stopViewer(panel)
      throw err
    }
  })
}

export function register(on) {
  on('session.start', async ($, e, next) => {
    await disconnect()
    await $.command.register({
      name: 'bigbrain-explorer',
      description: 'Browse project memories and review handoffs',
    })
    return next(e)
  })
  on('command.run', { command: 'bigbrain-explorer' }, async ($) => {
    await open($)
    return {}
  })
  on('ui.render', { component: 'Pane' }, async ($, e, next) => {
    if (e.requestId !== paneId) return next(e)
    const panel = active
    if (panel?.session && !await sameSession($, panel)) await disconnect(panel)
    const current = active
    return draw($, e, current, (action) => act($, current, action), () => open($))
  })
  on('ui.message', async ($, e, next) => {
    if (e.requestId !== paneId || e.element !== 'network-input') return next(e)
    const p = active, pick = e.data
    if (!p || !pick || !['overview', 'graph'].includes(p.page) || pick.frame !== p.graphFrame) return {}
    await act($, p, async () => {
      if (
        p.page === 'graph' && Array.isArray(pick.pan) && pick.pan.length === 2 &&
        pick.pan.every(Number.isFinite)
      ) {
        p.panX = Math.max(-.8, Math.min(.8, p.panX - pick.pan[0] / p.zoom))
        p.panY = Math.max(-.8, Math.min(.8, p.panY - pick.pan[1] / p.zoom))
      } else if (p.network.nodes.has(pick.node)) {
        if (p.page === 'overview') {
          p.trail = []
          await p.actions.graph(pick.node)
        } else await p.actions.selectNode(pick.node)
      } else if (p.network.edges.has(pick.edge)) {
        const edge = p.network.edges.get(pick.edge)
        if (p.page === 'overview') {
          p.trail = []
          await p.actions.graph(p.previewRoots.get(pick.edge))
        }
        if (p.network.edges.has(pick.edge)) p.actions.selectEdge(edge)
      }
    })
    return {}
  })
  on('ui.close', async ($, e, next) => {
    if (e.id === paneId) await disconnect()
    return next(e)
  })
  on('session.end', async ($, e, next) => {
    await disconnect()
    return next(e)
  })
}
