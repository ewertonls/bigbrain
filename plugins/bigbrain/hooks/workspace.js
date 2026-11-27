import { request } from './viewer.js'
import { neighborhood } from './network.js'

// Each controller belongs to one open pane. All reads use the same selection;
// draft and policy operations stay pinned to the originating session.
export function workspace($, p) {
  const api = (op, args) => request($, p, op, args)
  const read = (op, args) =>
    api(op, {
      scope: p.scope === 'global' ? 'global' : p.binding.scope,
      include_global: p.scope === 'combined',
      ...args,
    })
  const recent = (limit, after = '') => read('browse', { limit, after })
  const history = (id, limit, after = '') => read('history', { id, limit, after })

  async function overview() {
    const [batch, drafts] = await Promise.all([recent(6), p.scope === 'global' ? [] : api('drafts', {})])
    const histories = await Promise.all(batch.memories.slice(0, 3).map((m) => history(m.id, 5)))
    p.recent = batch.memories
    p.hasMore = !!batch.next_cursor
    p.network = neighborhood(histories)
    p.previewRoots = new Map(
      histories.flatMap((h) => h.related.map((item) => [item.path.relationship_id, h.memory.id])),
    )
    p.seed = (histories.find((h) => h.related.length) || histories[0])?.memory.id
    p.pending = drafts.filter((d) => !['saved', 'dismissed'].includes(d.status)).length
    p.page = 'overview'
  }
  async function memories(term = '', after = '') {
    const batch = term
      ? await read('query', { text: term, mode: 'hybrid', limit: 20 })
      : await recent(12, after)
    p.memories = batch.matches || batch.memories || []
    p.term = term
    p.after = after
    p.next = batch.next_cursor
    p.fallback = batch.fallback
    p.page = 'memories'
  }
  async function graph(id, after = '') {
    const choices = await recent(8)
    const focus = id || choices.memories[0]?.id
    const h = focus ? await history(focus, 6, after) : null
    p.network = neighborhood(h ? [h] : [])
    p.choices = new Map(choices.memories.map((m) => [m.id, m]))
    for (const [key, node] of p.network.nodes) p.choices.set(key, node)
    p.seed = focus
    p.history = h
    p.edge = null
    p.selected = focus
    p.feedback = null
    p.zoom = 1
    p.panX = p.panY = 0
    p.page = 'graph'
  }
  async function activity() {
    const [batch, automation] = await Promise.all([
      recent(20),
      p.scope === 'combined' ? api('automation', {}) : null,
    ])
    const events = batch.memories.map((m) => ({
      at: m.recorded_at,
      title: 'Memory recorded',
      memory: m,
    }))
    if (automation?.last) {
      const last = automation.last
      events.push({
        at: last.at,
        title: 'Context emitted',
        detail: last.memories.length + ' included · ' + last.bytes + ' bytes',
      })
    }
    for (const [hook, at] of Object.entries(automation?.hooks || {})) {
      events.push({ at, title: 'Hook observed', detail: hook })
    }
    p.events = events.sort((a, b) => Date.parse(b.at) - Date.parse(a.at))
    p.page = 'activity'
  }
  async function settings() {
    p.automation = await api('automation', {})
    p.preview = ''
    p.page = 'settings'
  }
  async function drafts() {
    p.drafts = await api('drafts', {})
    p.page = 'drafts'
  }
  return {
    overview,
    memories,
    graph,
    activity,
    settings,
    drafts,
    async scope(value) {
      if (value === p.scope) return
      const page = p.page
      p.scope = value
      p.trail = []
      p.term = ''
      // Remove the previous scope's rows even when the next read fails.
      p.page = 'loading'
      if (page === 'memories' || page === 'detail') return memories()
      if (page === 'graph') return graph()
      if (page === 'activity') return activity()
      return overview()
    },
    detail(memory) {
      p.detailFrom = p.page
      p.detail = memory
      p.copyText = ''
      p.page = 'detail'
    },
    async follow(id) {
      const previous = p.seed
      await graph(id)
      if (previous && previous !== id) p.trail.push(previous)
    },
    async backGraph() {
      const id = p.trail.at(-1)
      await graph(id)
      p.trail.pop()
    },
    selectNode(id) {
      p.selected = id
      p.edge = null
      p.feedback = null
    },
    selectEdge(edge) {
      p.edge = edge
      p.feedback = null
    },
    async feedback(after = '') {
      p.feedback = await read('feedback', { relationship_id: p.edge.relationship_id, limit: 5, after })
    },
    async policy(setting, enabled) {
      p.automation = await api('policy', { setting, enabled })
      p.preview = ''
    },
    async preview() {
      const result = await api('preview', {})
      p.preview = result.text || 'No memories selected.'
    },
    async review(op, draft, content) {
      await api(op, { id: draft.id, revision: draft.revision, content })
      await drafts()
    },
    async copy(value, surface) {
      const result = await $.ui.copy({ text: value, surface })
      p.copyText = result.isCopied ? '' : value
    },
  }
}
