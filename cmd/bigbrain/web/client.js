const token = location.hash.slice(1);
history.replaceState(null, '', location.pathname);
export async function api(operation, input) {
  const options = {headers: {Authorization: 'Bearer ' + token}};
  if (input !== undefined) {
    options.method = 'POST';
    options.headers['Content-Type'] = 'application/json';
    options.body = JSON.stringify(input);
  }
  const response = await fetch('/api/' + operation, options);
  if (!response.headers.get('content-type')?.includes('application/json')) {
    throw new Error(await response.text() || 'Viewer disconnected.');
  }
  const envelope = await response.json();
  if (envelope.error) throw new Error(envelope.error.message);
  return envelope.result;
}
