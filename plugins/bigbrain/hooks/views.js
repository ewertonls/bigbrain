import { brand, color, diagram, short } from './network.js'
import { draftDetail } from './review.js'

const titleCase = (value) => value ? value[0].toUpperCase() + value.slice(1) : 'Memory'
const caption = (item) => item.text || item.excerpt || item.id
function time(at) {
  if (!at) return ''
  const d = new Date(at)
  return Number.isFinite(d.getTime())
    ? d.toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' })
    : at
}

export function draw($, event, p, run, reopen) {
  const ui = $.ui.resolve(event)
  const { Box, Text, Button, Input, Select } = ui
  const terminal = event.surface === 'terminal'
  const columns = event.props.bodyColumns || event.viewport?.columns || 52
  const txt = (children, props = {}) => Text({ ...props, children })
  const muted = (value) => txt(value, { size: 'caption', dimColor: true })
  const heading = (value) => txt(value, { size: 'heading', weight: 'semibold' })
  const col = (children, props = {}) => Box({ flexDirection: 'column', ...props, children })
  const row = (children, props = {}) =>
    Box({ flexDirection: 'row', alignItems: 'center', flexWrap: 'wrap', gap: 'xs', ...props, children })
  const between = (children) => row(children, { justifyContent: 'space-between', gap: 0 })
  const button = (key, label, action, props = {}) =>
    Button({
      key,
      ...(props.children ? {} : { label }),
      plain: true,
      ...props,
      onPress: () => run(action),
    })
  const select = (key, value, options, choose, label) =>
    Select({ key, value, options, label, onSelect: (v) => run(() => choose(v)) })
  const section = (title, children, action) =>
    col([between([heading(title), ...(action ? [action] : [])]), ...children], { gap: 'sm' })
  const divider = () =>
    terminal ? txt('─'.repeat(Math.min(80, Math.max(10, columns - 4))), { dimColor: true }) : ui.Svg({
      source:
        '<svg xmlns="http://www.w3.org/2000/svg" width="800" height="1"><path d="M0 .5H800" stroke="#888" stroke-opacity=".30"/></svg>',
      alt: 'Section divider',
      height: 1,
    })
  const fact = (label, value) => col([muted(label), txt(String(value ?? 'Not recorded'))])
  if (!p?.binding || p.page === 'setup') {
    return col([
      heading('BigBrain'),
      txt(
        p?.error || (p?.busy ? 'Opening Explorer…' : 'Run /bigbrain:setup to configure this workspace.'),
      ),
      ...(!p?.busy ? [Button({ key: 'retry', label: 'Try again', onPress: reopen })] : []),
    ], { gap: 'sm', padding: 1 })
  }
  const a = p.actions
  const project = p.binding.scope.replace(/^project:/, '')
  const scopeTitle = p.scope === 'global' ? 'Global' : project
  const tab = ({ detail: 'memories' })[p.page] || p.page
  const goGraph = (id) => {
    p.trail = []
    return a.graph(id)
  }
  const search = () =>
    Input({
      key: 'search',
      placeholder: 'Search memories…',
      value: p.term,
      onSubmit: (term) => run(() => a.memories(term.trim())),
    })
  const memoryRow = (m) =>
    row([
      txt('●', { color: color(m.type) }),
      col([
        txt(short(m.text, 125), { weight: 'medium' }),
        muted([titleCase(m.type), m.scope, time(m.recorded_at)].filter(Boolean).join(' · ')),
      ], { flexGrow: 1, flexShrink: 1, minWidth: 0 }),
      button(m.id, '›', () => a.detail(m)),
    ], { flexWrap: 'nowrap', gap: 'sm' })
  const body = []
  const top = between([
    row([
      terminal
        ? txt('◈', { color: '#14b8a6' })
        : ui.Svg({ source: brand, alt: 'BigBrain', width: 21, height: 21 }),
      txt('BigBrain', { size: 'heading', weight: 'semibold' }),
    ]),
    row([
      txt('●', { color: p.daemon === 'running' ? '#41c77b' : 'inactive' }),
      muted(p.daemon === 'running' ? 'Local' : 'Local · ' + p.daemon),
      button('settings', '⚙', a.settings),
    ]),
  ])
  const scope = between([
    row([txt('▱', { dimColor: true }), txt(scopeTitle, { weight: 'semibold' })], { flexShrink: 1 }),
    select('scope', p.scope, [{ value: 'combined', label: 'Project + global' }, {
      value: 'project',
      label: 'Project only',
    }, { value: 'global', label: 'Global only' }], a.scope),
  ])
  const tabs = between(
    ['overview', 'memories', 'graph', 'activity'].map((name) =>
      button(name, titleCase(name), () => name === 'graph' ? goGraph() : a[name](), {
        dimColor: tab !== name,
        children: txt(titleCase(name), { weight: tab === name ? 'semibold' : 'regular' }),
      })
    ),
  )
  if (p.page === 'overview') {
    const metric = (name, value) =>
      col([muted(name), txt(String(value), { size: 'title', weight: 'semibold', tabular: true })], {
        flexGrow: 1,
      })
    body.push(
      row([
        metric('Active memories', p.recent.length + (p.hasMore ? '+' : '')),
        metric('Links in preview', p.network.edges.size),
      ]),
      search(),
    )
    if (p.recent.length) {
      body.push(
        section('Knowledge network', [
          diagram(ui, event.surface, p.network, p.seed, 1, true, columns, p),
          ...(!p.network.edges.size ? [muted('No visible links in this sample.')] : []),
        ], button('explore-graph', 'Explore graph', () => goGraph(p.seed))),
        divider(),
      )
    }
    body.push(
      section(
        'Recent memories',
        p.recent.length ? p.recent.map(memoryRow) : [muted('No active memories in this scope.')],
        button('browse-all', 'Browse all', () => a.memories()),
      ),
    )
    if (p.scope !== 'global') {
      body.push(
        button('drafts', 'Handoff review' + (p.pending ? ' · ' + p.pending + ' pending' : ''), a.drafts),
      )
    }
  } else if (p.page === 'memories') {
    body.push(
      heading('Memories'),
      search(),
      section(p.term ? 'Search results' : 'Recent memories', [
        ...(p.fallback ? [muted('Search fallback · ' + p.fallback)] : []),
        ...p.memories.map(memoryRow),
        ...(!p.memories.length
          ? [
            muted(
              p.term ? 'No matches. Change or clear the search.' : 'No active memories in this scope.',
            ),
          ]
          : []),
      ]),
    )
    if (p.after || p.next) {
      body.push(row([
        ...(p.after ? [button('first', 'First page', () => a.memories())] : []),
        ...(p.next ? [button('more', 'Next memories', () => a.memories('', p.next))] : []),
      ]))
    }
  } else if (p.page === 'detail') {
    const m = p.detail
    body.push(
      button('back-detail', '‹ ' + titleCase(p.detailFrom), () => {
        p.page = p.detailFrom
      }),
      muted(titleCase(m.type)),
      heading(m.text),
      fact('Source', m.source),
      fact('Scope', m.scope),
      fact('Recorded', time(m.recorded_at)),
      fact('Memory ID', m.id),
      button('neighbors', 'Explore stored neighbors', () => goGraph(m.id)),
    )
    body.push(
      row(
        [['reference', m.id + ' · ' + m.source], ['text', m.text], [
          'context',
          m.scope + ' · ' + m.id + ' · ' + m.source + '\n' + m.text,
        ]]
          .map(([name, value]) =>
            button('copy-' + name, 'Copy ' + name, () => a.copy(value, event.surface))
          ),
      ),
    )
    if (p.copyText) {
      body.push(Input({
        key: 'copy-value',
        label: 'Select and copy',
        value: p.copyText,
        onSubmit: () => {
          p.copyText = ''
          $.ui.invalidate('ui.render')
        },
      }))
    }
    if (m.reason) {
      const r = m.reason, steps = r.steps?.length ? r.steps : r.path ? [r.path] : []
      const lines = [
        txt(
          ({
            text: 'Text match',
            vector: 'Vector match',
            hybrid: 'Combined retrieval signals',
            relationship: 'Relationship traversal',
            id: 'Selected by ID',
          })[r.kind] || r.kind,
        ),
      ]
      if (r.terms?.length) lines.push(muted(r.terms.join(' · ')))
      for (
        const [label, n] of [['BM25', r.bm25], ['Cosine', r.cosine], ['Graph score', r.graph_score]]
      ) if (n != null) lines.push(fact(label, n.toFixed(3)))
      if (!steps.length) lines.push(muted('Direct match. No relationship was traversed.'))
      for (const s of steps) {
        lines.push(
          txt(s.type + ' · ' + s.direction),
          muted(s.direction === 'reverse' ? s.to_id + ' → ' + s.from_id : s.from_id + ' → ' + s.to_id),
          fact('Stored direction', s.from_id + ' → ' + s.to_id),
          fact('Source', s.source),
          fact('Relationship ID', s.relationship_id),
        )
      }
      body.push(divider(), section('Retrieval explanation', lines))
    }
  } else if (p.page === 'graph') {
    body.push(heading('Knowledge graph'))
    if (!p.history) body.push(muted('No active memories in this scope.'))
    else {
      const h = p.history
      body.push(
        select(
          'focus-memory',
          p.seed,
          [...p.choices.values()].map((m) => ({ value: m.id, label: short(caption(m), 55) })),
          a.follow,
          'Focus memory',
        ),
      )
      if (p.trail.length) body.push(button('back-graph', '‹ Previous neighborhood', a.backGraph))
      if (!p.listOnly) {
        body.push(diagram(ui, event.surface, p.network, p.seed, p.zoom, false, columns, p))
      }
      body.push(row([
        button('list-view', p.listOnly ? 'Graph view' : 'List view', () => {
          p.listOnly = !p.listOnly
        }),
        ...(!terminal && !p.listOnly
          ? [
            button('minus', '−', () => {
              p.zoom = Math.max(.6, p.zoom / 1.25)
            }),
            button('plus', '+', () => {
              p.zoom = Math.min(3, p.zoom * 1.25)
            }),
            button('fit', 'Fit', () => {
              p.zoom = 1
              p.panX = p.panY = 0
            }),
          ]
          : []),
      ]))
      body.push(
        section('Memories', [
          select(
            'node',
            p.selected,
            [...p.network.nodes.values()].map((m) => ({ value: m.id, label: short(caption(m), 55) })),
            a.selectNode,
            'Inspect',
          ),
        ]),
      )
      const edges = [...p.network.edges.values()]
      body.push(
        section(
          'Relationships',
          edges.length
            ? edges.map((s) =>
              button(
                'edge-' + s.relationship_id,
                s.type + ' · ' +
                  short(
                    caption(p.network.nodes.get(s.from_id === h.memory.id ? s.to_id : s.from_id)),
                    46,
                  ),
                () => a.selectEdge(s),
              )
            )
            : [muted('No visible links for this memory.')],
        ),
      )
      if (h.next_cursor) {
        body.push(button('more-edges', 'Next relationships', () => a.graph(p.seed, h.next_cursor)))
      }
      if (p.edge) {
        const e = p.edge
        body.push(divider(), heading(e.type))
        for (
          const [k, v] of [
            ['From', caption(p.network.nodes.get(e.from_id))],
            ['To', caption(p.network.nodes.get(e.to_id))],
            ['Scope', e.scope],
            ['Weight', e.weight],
            ['Confidence', e.confidence],
            ['Activations', e.activation_count],
            ['Last activated', time(e.last_activated_at)],
            ['Source', e.source],
            ['Relationship ID', e.relationship_id],
          ]
        ) if (v != null && v !== '') body.push(fact(k, v))
        body.push(button('feedback', 'Feedback history', () => a.feedback()))
        if (p.feedback) {
          for (const item of p.feedback.events || []) {
            body.push(
              txt(item.action + ' · ' + time(item.recorded_at)),
              muted(item.author + ' · ' + item.source),
            )
          }
          if (!p.feedback.events?.length) body.push(muted('No feedback recorded.'))
          if (p.feedback.next_cursor) {
            body.push(button('more-feedback', 'Next feedback', () => a.feedback(p.feedback.next_cursor)))
          }
        }
      } else {
        const node = p.network.nodes.get(p.selected)
        body.push(
          divider(),
          muted(titleCase(node.type)),
          heading(caption(node)),
          fact('Source', node.source),
          fact('Scope', node.scope),
          fact('Memory ID', node.id),
        )
        if (node.id !== p.seed) body.push(button('follow', 'Explore neighbors', () => a.follow(node.id)))
      }
    }
  } else if (p.page === 'activity') {
    body.push(heading('Brain activity'))
    for (const item of p.events) {
      body.push(
        col([
          muted(item.title + ' · ' + time(item.at)),
          item.memory ? memoryRow(item.memory) : txt(item.detail),
        ]),
      )
    }
    if (!p.events.length) body.push(muted('No recorded activity in this scope.'))
  } else if (p.page === 'settings') {
    const { policy, last, hooks } = p.automation
    body.push(
      heading('Settings'),
      fact('Workspace', p.binding.scope),
      fact('Session', p.session),
      button('drafts', 'Handoff review', a.drafts),
      divider(),
    )
    const switches = [
      ['startup', 'Startup and resume', !policy.disable_startup],
      ['compact', 'After compaction', !policy.disable_compact],
      ['global', 'Include global in recall', !policy.project_only],
      ['bootstrap', 'Use latest reviewed handoff', !policy.disable_bootstrap],
    ]
    body.push(
      section(
        'Session policy',
        switches.map(([id, name, on]) =>
          between([txt(name), button('policy-' + id, on ? 'On' : 'Off', () => a.policy(id, !on))])
        ),
      ),
    )
    body.push(
      button(
        'pause',
        policy.paused ? 'Resume recall' : 'Pause recall',
        () => a.policy('paused', !policy.paused),
      ),
      divider(),
    )
    body.push(
      section(
        'Last recall',
        last
          ? [
            txt(last.bytes + ' / ' + last.limit + ' bytes'),
            muted(
              last.memories.length + ' included · ' + (last.duplicates || 0) + ' duplicates · ' +
                (last.budget_skipped || 0) + ' skipped',
            ),
            ...last.memories.map((m) => txt(m.id)),
          ]
          : [muted('No recall receipt for this session.')],
      ),
    )
    body.push(button('preview', 'Preview next context', a.preview))
    if (p.preview) body.push(txt(p.preview))
    for (const [hook, at] of Object.entries(hooks || {})) body.push(fact(hook, time(at)))
  } else if (p.page === 'drafts') {
    body.push(heading('Handoff review'), muted(p.binding.scope))
    if (!p.drafts.length) body.push(muted('No handoffs in this session.'))
    for (const d of p.drafts) {
      body.push(button(d.id, d.status + ' · ' + short(d.content.memory.text, 110), () => {
        p.draft = d
        p.page = 'review'
      }))
    }
  } else if (p.page === 'review' || p.page === 'edit') {
    const draft = p.draft
    body.push(
      button('drafts', '‹ Handoff review', a.drafts),
      ...draftDetail(
        ui,
        p,
        () => $.ui.invalidate('ui.render'),
        (op, content) => run(() => a.review(op, draft, content)),
      ),
    )
  } else body.push(button('retry', 'Try again', a.overview))
  const hiddenScope = ['settings', 'drafts', 'review', 'edit'].includes(p.page)
  return col([
    top,
    ...(!hiddenScope
      ? [col([scope], { borderStyle: 'round', borderColor: '#555555', paddingX: 1 })]
      : []),
    tabs,
    divider(),
    ...(p.error ? [txt(p.error, { color: 'error' })] : []),
    ...(p.busy ? [muted('Loading…')] : []),
    ...body,
  ], { gap: terminal ? 0 : 'sm', paddingX: terminal ? 1 : 0, paddingY: terminal ? 0 : 'xs' })
}
