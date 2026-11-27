import { expect, test } from 'claude-code/testing'

const pane = {
  plugin: 'bigbrain',
  component: 'Pane',
  requestId: 'bigbrain-explorer',
  surface: 'terminal',
  props: { title: 'BigBrain Explorer', isFocused: true, bodyColumns: 44 },
} as const

test(
  'keeps memories available through a handoff read failure and reviews the displayed revision after retry',
  { timeoutMs: 20000 },
  async ($, on) => {
    let session = 'first'
    let draftsUnavailable = true
    const revisions: number[] = []
    const draft = {
      id: 'd1',
      revision: 4,
      status: 'draft',
      origin: { scope: 'project:demo' },
      content: {
        producer: 'handoff',
        memory: { text: 'Use sockets', type: 'decision', source: 'design.md' },
        evidence: { text: 'Observed in tests', type: 'observation', source: 'tests.md' },
      },
    }
    const memory = { ...draft.content.memory, id: 'm1', scope: draft.origin.scope }
    on('session.id', () => ({ value: session }))
    on('session.cwd', () => ({ value: '/workspace' }))
    on('ui.open', () => ({ value: { isPlaced: true } }))
    let prepared = false
    on('process.run', (_, e) => {
      expect(e.argv.slice(1)).toEqual([
        '--claude',
        'plugin',
        'doctor',
        '--start-daemon',
        '--cwd',
        '/workspace',
        '--session',
        'first',
      ])
      prepared = true
      return {
        value: {
          exitCode: 0,
          stdout: '',
          stderr: '',
          isStdoutTruncated: false,
          isStderrTruncated: false,
        },
      }
    })
    on('process.spawn', async function* (_, e) {
      expect(prepared).toBe(true)
      expect(e.argv.slice(1)).toEqual([
        '--claude',
        'ui',
        '--connect-only',
        '--cwd',
        '/workspace',
        '--session',
        'first',
      ])
      yield { stream: 'stdout', text: 'http://127.0.0.1:9101/' }
      yield { stream: 'stdout', text: '#private\n' }
      return { value: { code: 0, signal: null } }
    })
    on('http.fetch', (_, e) => {
      const path = new URL(e.url).pathname
      expect(e.init.headers.Authorization).toBe('Bearer private')
      let result: unknown
      let error
      if (path === '/api/status') result = { binding: { scope: 'project:demo', session_id: session } }
      if (path === '/api/browse') result = { memories: [memory] }
      if (path === '/api/history') result = { memory, related: [], status: 'active' }
      if (path === '/api/drafts') {
        if (draftsUnavailable) error = { message: 'Cannot read handoffs.' }
        else result = [draft]
      }
      if (path === '/api/edit') {
        const edit = JSON.parse(e.init.body as string)
        expect(edit.revision).toBe(draft.revision)
        expect(edit.content.evidence).toEqual(draft.content.evidence)
        draft.content = edit.content
        draft.revision++
        result = draft
      }
      if (path === '/api/save') {
        const action = JSON.parse(e.init.body as string)
        expect(action.id).toBe('d1')
        revisions.push(action.revision)
        if (action.revision !== draft.revision) error = { message: 'Stale draft revision.' }
        else draft.status = 'saved'
      }
      return {
        value: {
          ok: !error,
          status: error ? 400 : 200,
          headers: {},
          text: JSON.stringify({ result, error }),
        },
      }
    })
    await $.command.run({ command: 'bigbrain-explorer', args: '' })
    const ui = await $.ui.mount(pane)
    expect(await ui.find({ key: 'save' })).toBeUndefined()
    expect(await ui.find({ type: 'Text', text: 'Pending handoff count unavailable.' })).toBeDefined()
    await ui.press({ key: 'm1' })
    expect(await ui.find({ type: 'Text', text: 'design.md' })).toBeDefined()
    await ui.press({ key: 'back-detail' })
    await ui.press({ key: 'drafts' })
    expect(await ui.find({ type: 'Text', text: 'Cannot read handoffs.' })).toBeDefined()
    draftsUnavailable = false
    await ui.press({ key: 'overview' })
    expect(await ui.find({ key: 'drafts', text: 'Handoff review · 1 pending' })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: 'Pending handoff count unavailable.' })).toBeUndefined()
    await ui.press({ key: 'drafts' })
    await ui.press({ key: 'd1' })
    expect(await ui.find({ type: 'Text', text: 'design.md' })).toBeDefined()
    expect(revisions).toEqual([])
    draft.revision++
    draft.content.memory.text = 'Use local sockets'
    await ui.press({ key: 'save' })
    expect(await ui.find({ type: 'Text', text: 'Stale draft revision.' })).toBeDefined()
    await ui.press({ key: 'drafts' })
    await ui.press({ key: 'd1' })
    expect(await ui.find({ type: 'Text', text: 'Use local sockets' })).toBeDefined()
    await ui.press({ key: 'edit' })
    await ui.input({ key: 'memory-source', text: 'updated-design.md', kind: 'change' })
    await ui.press({ key: 'apply' })
    await ui.press({ key: 'd1' })
    expect(await ui.find({ type: 'Text', text: 'updated-design.md' })).toBeDefined()
    await ui.press({ key: 'save' })
    expect(revisions).toEqual([4, 6])
    expect(draft.status).toBe('saved')
    session = 'other'
    await ui.press({ key: 'overview' })
    expect(await ui.find({ key: 'd1' })).toBeUndefined()
  },
)

test('unbound workspace can retry from the pane', async ($, on) => {
  on('session.id', () => ({ value: 's' }))
  on('session.cwd', () => ({ value: '/unbound' }))
  on('ui.open', () => ({ value: { isPlaced: true } }))
  on('process.run', () => {
    return {
      value: {
        exitCode: 1,
        stdout: '',
        stderr: 'Run bigbrain project init PROJECT_ID.',
        isStdoutTruncated: false,
        isStderrTruncated: false,
      },
    }
  })
  await $.command.run({ command: 'bigbrain-explorer', args: '' })
  const ui = await $.ui.mount(pane)
  expect(await ui.find({ type: 'Text', text: /project init/ })).toBeDefined()
  expect(await ui.find({ key: 'retry' })).toBeDefined()
})

test(
  'desktop overview, scoped search, graph inspector and session settings',
  { timeoutMs: 30000 },
  async ($, on) => {
    const m = {
      id: 'm',
      type: 'decision',
      text: 'Use <local> transport',
      source: 'design.md',
      scope: 'project:demo',
      recorded_at: '2026-01-01T12:00:00Z',
    }
    const n = { ...m, id: 'n', text: 'Measure feedback', source: 'notes.md' }
    const invalidation = { reason: 'Metric retired', source: 'review-log.md', at: m.recorded_at }
    const path = {
      from_id: 'm',
      to_id: 'n',
      relationship_id: 'r',
      type: 'justified_by',
      source: 'link.md',
      direction: 'forward',
      weight: 1,
    }
    const automation = {
      policy: { paused: false },
      hooks: { 'SessionStart/startup': m.recorded_at },
      last: null,
    }
    const requests: string[] = []
    on('session.id', () => ({ value: 'desktop-chat' }))
    on('session.cwd', () => ({ value: '/workspace' }))
    on('ui.open', () => ({ value: { isPlaced: true } }))
    on(
      'process.run',
      () => ({
        value: {
          stdout: '',
          stderr: '',
          exitCode: 0,
          isStdoutTruncated: false,
          isStderrTruncated: false,
        },
      }),
    )
    on('process.spawn', async function* () {
      yield { stream: 'stdout', text: 'http://127.0.0.1:9101/#secret\n' }
      return { value: { code: 0, signal: null } }
    })
    on('http.fetch', (_, e) => {
      const op = new URL(e.url).pathname.slice(5), data = JSON.parse((e.init.body as string) || '{}')
      requests.push(op + ':' + (data.scope || '') + ':' + String(data.include_global))
      let result: unknown
      switch (op) {
        case 'status':
          result = {
            binding: { scope: 'project:demo', include_global: true, session_id: 'desktop-chat' },
            daemon: { state: 'running' },
          }
          break
        case 'drafts':
          result = []
          break
        case 'browse':
          result = { memories: data.scope === 'global' ? [] : [m] }
          break
        case 'history': {
          const focusedOnActive = data.id === m.id
          const neighbor = focusedOnActive ? n : m
          result = {
            memory: focusedOnActive ? m : n,
            status: focusedOnActive ? 'active' : 'invalidated',
            invalidation: focusedOnActive ? undefined : invalidation,
            related: [{
              id: neighbor.id,
              type: neighbor.type,
              scope: neighbor.scope,
              source: neighbor.source,
              excerpt: neighbor.text,
              status: focusedOnActive ? 'invalidated' : 'active',
              path: { ...path, direction: focusedOnActive ? 'forward' : 'reverse' },
            }],
          }
          break
        }
        case 'query':
          result = {
            matches: [{
              ...m,
              reason: { kind: 'relationship', steps: [{ ...path, direction: 'reverse' }] },
            }],
          }
          break
        case 'feedback':
          result = {
            events: [{
              action: 'used',
              author: 'person',
              source: 'review.md',
              recorded_at: m.recorded_at,
            }],
          }
          break
        case 'automation':
          result = automation
          break
        case 'policy':
          expect(data).toEqual({ setting: 'paused', enabled: true })
          automation.policy.paused = true
          result = automation
          break
        default:
          throw new Error(op)
      }
      return { value: { ok: true, status: 200, headers: {}, text: JSON.stringify({ result }) } }
    })
    await $.command.run({ command: 'bigbrain-explorer', args: '' })
    const ui = await $.ui.mount({ ...pane, surface: 'desktop' })
    const tree = JSON.stringify(await ui.drawn())
    expect(tree).toContain('Knowledge network · 2 memories · 1 relationship')
    expect(tree).toContain('&lt;local&gt;')
    expect(tree).not.toContain('#secret')
    // A click on the visible center node in the desktop preview opens its graph.
    await ui.resize({ columns: 40, rows: 10, in: 'network-input' })
    await ui.pointer({ type: 'down', button: 'left', x: 19, y: 4, in: 'network-input' })
    await ui.pointer({ type: 'up', button: 'left', x: 19, y: 4, in: 'network-input' })
    expect(await ui.find({ key: 'focus-memory' })).toBeDefined()
    await ui.press({ key: 'edge-r' })
    expect(await ui.find({ type: 'Text', text: 'link.md' })).toBeDefined()
    await ui.press({ key: 'feedback' })
    expect(await ui.find({ type: 'Text', text: /review.md/ })).toBeDefined()
    await ui.select({ key: 'node', value: 'n' })
    expect(await ui.find({ type: 'Text', text: 'Invalidated' })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: invalidation.reason })).toBeDefined()
    await ui.press({ key: 'follow' })
    expect(await ui.find({ key: 'back-graph' })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: 'Invalidated' })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: invalidation.reason })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: invalidation.source })).toBeDefined()
    await ui.select({ key: 'node', value: 'm' })
    expect(await ui.find({ type: 'Text', text: 'Active' })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: invalidation.reason })).toBeUndefined()
    await ui.press({ key: 'memories' })
    await ui.select({ key: 'scope', value: 'project' })
    await ui.input({ key: 'search', text: 'transport', kind: 'submit' })
    expect(requests).toContain('query:project:demo:false')
    await ui.press({ key: 'm' })
    expect(await ui.find({ type: 'Text', text: 'Retrieval explanation' })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: 'n → m' })).toBeDefined()
    await ui.select({ key: 'scope', value: 'global' })
    expect(await ui.find({ key: 'm' })).toBeUndefined()
    expect(requests).toContain('browse:global:false')
    await ui.press({ key: 'overview' })
    expect(await ui.find({ key: 'drafts' })).toBeUndefined()
    await ui.select({ key: 'scope', value: 'combined' })
    await ui.press({ key: 'activity' })
    expect(await ui.find({ type: 'Text', text: 'SessionStart/startup' })).toBeDefined()
    await ui.press({ key: 'settings' })
    await ui.press({ key: 'pause' })
    expect(await ui.find({ key: 'pause', text: 'Resume recall' })).toBeDefined()
  },
)
