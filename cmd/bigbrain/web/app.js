import {api} from './client.js';
const status = document.querySelector('#connection');
try {
  const {binding, daemon} = await api('status');
  status.textContent = binding.scope + ' · Daemon ' + daemon.state;
} catch (error) { status.textContent = error.message; }
