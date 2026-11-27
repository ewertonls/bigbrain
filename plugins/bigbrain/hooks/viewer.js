// Keep the launch capability inside the mod. The viewer rechecks its project
// binding and enforces draft revisions on every request.
export async function startViewer($, panel) {
  const launcher = $.plugin.root + '/scripts/bigbrain'
  const binding = ['--cwd', panel.cwd, '--session', panel.session]
  // Claude cancels the viewer's process tree when the pane closes. Let a
  // short-lived command start the shared daemon before spawning the viewer.
  const ready = await $.process.run([launcher, '--claude', 'plugin', 'doctor', '--start-daemon', ...binding], { cwd: panel.cwd })
  if (ready.exitCode !== 0) {
    if (ready.stderr.includes('start-daemon')) throw new Error('Update the BigBrain executable and run /bigbrain:setup.')
    throw new Error(ready.stderr.trim() || 'BigBrain could not start the daemon.')
  }
  if (panel.closed) return
  const child = $.process.spawn({
    argv: [launcher, '--claude', 'ui', '--connect-only', ...binding],
    cwd: panel.cwd,
  })
  panel.child = child
  let stdout = ''
  let stderr = ''
  while (!stdout.includes('\n')) {
    const chunk = await child.next()
    if (chunk.done) throw new Error(stderr.trim() || 'BigBrain viewer exited during startup.')
    if (chunk.value.stream === 'stdout') stdout += chunk.value.text
    else stderr += chunk.value.text
    if (stdout.length + stderr.length > 8192) throw new Error('Unexpected viewer startup output.')
  }
  const url = new URL(stdout.trim())
  if (url.protocol !== 'http:' || url.hostname !== '127.0.0.1' || !url.port ||
      url.username || url.password || url.pathname !== '/' || url.search || !url.hash.slice(1)) {
    throw new Error('Invalid local viewer address.')
  }
  panel.origin = url.origin
  panel.token = url.hash.slice(1)
}

export async function stopViewer(panel) {
  if (!panel) return
  panel.closed = true
  const child = panel.child
  panel.child = null
  panel.token = ''
  panel.origin = ''
  if (child) await child.return()
}

export async function request($, panel, operation, input) {
  const headers = { Authorization: 'Bearer ' + panel.token, Origin: panel.origin }
  const options = { headers }
  if (input !== undefined) {
    options.method = 'POST'
    headers['Content-Type'] = 'application/json'
    options.body = JSON.stringify(input)
  }
  const response = await $.http.fetch(panel.origin + '/api/' + operation, options)
  let envelope
  try {
    envelope = JSON.parse(response.text)
  } catch {
    throw new Error('Cannot read the local viewer response. Reopen the Explorer.')
  }
  if (!response.ok || envelope.error) {
    throw new Error(envelope.error?.message || 'The viewer rejected this request.')
  }
  return envelope.result
}
