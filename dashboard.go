package main

import "net/http"

func (a *App) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r, false) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid admin key"))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(dashboardHTML))
}

const dashboardHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Heliproxy</title>
  <style>
    :root {
      color-scheme: light dark;
      --bg: #f7f8fa;
      --surface: #ffffff;
      --line: #d8dde5;
      --text: #151922;
      --muted: #5d6675;
      --accent: #0f766e;
      --danger: #b42318;
      --warn: #a15c00;
      --ok: #087443;
      --code: #eef2f6;
    }
    @media (prefers-color-scheme: dark) {
      :root {
        --bg: #101318;
        --surface: #171b22;
        --line: #303744;
        --text: #eef2f7;
        --muted: #a7b0bf;
        --accent: #2dd4bf;
        --danger: #ff8a80;
        --warn: #fbbf24;
        --ok: #34d399;
        --code: #202631;
      }
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: var(--bg);
      color: var(--text);
      letter-spacing: 0;
    }
    header {
      border-bottom: 1px solid var(--line);
      background: var(--surface);
    }
    main, .bar {
      max-width: 1180px;
      margin: 0 auto;
      padding: 18px;
    }
    .bar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;
    }
    h1 { margin: 0; font-size: 22px; line-height: 1.2; }
    h2 { margin: 0 0 12px; font-size: 16px; }
    .sub { color: var(--muted); font-size: 13px; margin-top: 4px; }
    .actions { display: flex; gap: 8px; flex-wrap: wrap; }
    button {
      border: 1px solid var(--line);
      background: var(--surface);
      color: var(--text);
      height: 34px;
      padding: 0 12px;
      border-radius: 6px;
      cursor: pointer;
      font-weight: 600;
    }
    button.primary { background: var(--accent); border-color: var(--accent); color: #ffffff; }
    button.danger { color: var(--danger); }
    button:disabled { opacity: .55; cursor: not-allowed; }
    section {
      background: var(--surface);
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 16px;
      margin: 16px 0;
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 12px;
    }
    .metric { border: 1px solid var(--line); border-radius: 8px; padding: 12px; }
    .metric .label { color: var(--muted); font-size: 12px; }
    .metric .value { font-size: 24px; font-weight: 700; margin-top: 4px; }
    .table-wrap { overflow-x: auto; border: 1px solid var(--line); border-radius: 8px; }
    table { width: 100%; border-collapse: collapse; min-width: 820px; }
    th, td { padding: 10px; border-bottom: 1px solid var(--line); text-align: left; vertical-align: top; font-size: 13px; }
    th { color: var(--muted); font-size: 12px; font-weight: 700; background: color-mix(in srgb, var(--surface) 88%, var(--line)); position: sticky; top: 0; }
    tr:last-child td { border-bottom: 0; }
    input, textarea {
      width: 100%;
      min-height: 34px;
      border: 1px solid var(--line);
      border-radius: 6px;
      background: var(--surface);
      color: var(--text);
      padding: 7px 9px;
      font: inherit;
    }
    textarea { min-height: 72px; resize: vertical; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
    label { display: block; color: var(--muted); font-size: 12px; margin-bottom: 5px; }
    .form-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
    .mono { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
    .pill { display: inline-flex; align-items: center; height: 22px; padding: 0 8px; border-radius: 999px; font-size: 12px; border: 1px solid var(--line); }
    .ok { color: var(--ok); }
    .warn { color: var(--warn); }
    .bad { color: var(--danger); }
    .muted { color: var(--muted); }
    .row-actions { display: flex; gap: 6px; }
    pre { background: var(--code); padding: 12px; border-radius: 8px; overflow: auto; min-height: 80px; }
    @media (max-width: 760px) {
      .bar { align-items: flex-start; flex-direction: column; }
      .grid, .form-grid { grid-template-columns: 1fr; }
      main, .bar { padding: 14px; }
    }
  </style>
</head>
<body>
  <header>
    <div class="bar">
      <div>
        <h1>Heliproxy</h1>
        <div class="sub">Helius-compatible RPC proxy with sticky round-robin key rotation.</div>
      </div>
      <div class="actions">
        <button id="refreshBtn">Refresh</button>
        <button id="usageBtn">Refresh Usage</button>
        <button id="testBtn">Test RPC</button>
        <button id="saveBtn" class="primary">Save Config</button>
      </div>
    </div>
  </header>
  <main>
    <section>
      <h2>Status</h2>
      <div class="grid">
        <div class="metric"><div class="label">Configured Keys</div><div id="configuredKeys" class="value">-</div></div>
        <div class="metric"><div class="label">Enabled Keys</div><div id="enabledKeys" class="value">-</div></div>
        <div class="metric"><div class="label">Available Keys</div><div id="availableKeys" class="value">-</div></div>
        <div class="metric"><div class="label">Sticky Limit</div><div id="stickyMetric" class="value">-</div></div>
      </div>
    </section>

    <section>
      <h2>Keys</h2>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Name</th><th>Key</th><th>Project</th><th>Status</th><th>Usage</th><th>Requests</th><th>Last Error</th>
            </tr>
          </thead>
          <tbody id="keyStatusRows"></tbody>
        </table>
      </div>
    </section>

    <section>
      <h2>Config</h2>
      <div class="form-grid">
        <div><label>Listen Host</label><input id="serverHost"></div>
        <div><label>Listen Port</label><input id="serverPort" type="number" min="1" max="65535"></div>
        <div><label>Sticky Round-Robin Limit</label><input id="stickyLimit" type="number" min="1"></div>
        <div><label>Cooldown Seconds</label><input id="cooldownSeconds" type="number" min="1"></div>
        <div><label>Request Timeout Seconds</label><input id="timeoutSeconds" type="number" min="1"></div>
        <div><label>Max Body Bytes</label><input id="maxBodyBytes" type="number" min="1"></div>
      </div>
      <div class="form-grid" style="margin-top:12px">
        <div><label>Helius RPC Base URL</label><input id="rpcBaseUrl"></div>
        <div><label>Helius REST Base URL</label><input id="restBaseUrl"></div>
        <div><label>Helius Admin Base URL</label><input id="adminBaseUrl"></div>
      </div>
      <div class="form-grid" style="margin-top:12px">
        <div><label>Config Path</label><input id="configPath" disabled></div>
        <div><label>Client API Keys, one per line</label><textarea id="clientKeys"></textarea></div>
        <div><label>Admin API Keys, one per line</label><textarea id="adminKeys"></textarea></div>
      </div>
      <div class="form-grid" style="margin-top:12px">
        <div><label>RPC URL Example</label><textarea id="rpcExample" disabled></textarea></div>
        <div><label>REST URL Example</label><textarea id="restExample" disabled></textarea></div>
      </div>
    </section>

    <section>
      <h2>Edit Helius Keys</h2>
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>Enabled</th><th>Name</th><th>Project ID</th><th>API Key</th><th>Action</th></tr>
          </thead>
          <tbody id="keyEditRows"></tbody>
        </table>
      </div>
      <div class="form-grid" style="margin-top:12px">
        <div><label>New Key Name</label><input id="newKeyName" placeholder="helius-2"></div>
        <div><label>New Project ID</label><input id="newProjectId" placeholder="optional for usage"></div>
        <div><label>New Helius API Key</label><input id="newApiKey" placeholder="required to add key"></div>
      </div>
      <div class="actions" style="margin-top:12px"><button id="addKeyBtn">Add Key</button></div>
    </section>

    <section>
      <h2>Output</h2>
      <pre id="output">Loading...</pre>
    </section>
  </main>

  <script>
    const qs = new URLSearchParams(location.search);
    const adminKey = qs.get('api-key') || qs.get('api_key') || '';
    let config = null;
    let status = null;

    function withKey(path) {
      const sep = path.includes('?') ? '&' : '?';
      return path + sep + 'api-key=' + encodeURIComponent(adminKey);
    }

    async function api(path, options = {}) {
      const res = await fetch(withKey(path), {
        ...options,
        headers: { 'content-type': 'application/json', ...(options.headers || {}) }
      });
      const text = await res.text();
      let data = null;
      try { data = text ? JSON.parse(text) : null; } catch { data = { raw: text }; }
      if (!res.ok) throw new Error(data?.error || data?.Error || data?.message || text || res.statusText);
      return data;
    }

    function setOutput(value) {
      document.getElementById('output').textContent = typeof value === 'string' ? value : JSON.stringify(value, null, 2);
    }

    function lines(value) {
      return String(value || '').split('\n').map(v => v.trim()).filter(Boolean);
    }

    function fillConfig() {
      if (!config) return;
      document.getElementById('serverHost').value = config.server.host || '';
      document.getElementById('serverPort').value = config.server.port || '';
      document.getElementById('stickyLimit').value = config.routing.sticky_round_robin_limit || 1;
      document.getElementById('cooldownSeconds').value = config.routing.cooldown_seconds || 60;
      document.getElementById('timeoutSeconds').value = config.routing.request_timeout_seconds || 30;
      document.getElementById('maxBodyBytes').value = config.routing.max_body_bytes || 33554432;
      document.getElementById('rpcBaseUrl').value = config.helius.rpc_base_url || '';
      document.getElementById('restBaseUrl').value = config.helius.rest_base_url || '';
      document.getElementById('adminBaseUrl').value = config.helius.admin_base_url || '';
      document.getElementById('configPath').value = config.meta.config_path || '';
      document.getElementById('clientKeys').value = (config.auth.client_keys || []).join('\n');
      document.getElementById('adminKeys').value = (config.auth.admin_keys || []).join('\n');
      const firstClient = (config.auth.client_keys || [])[0] || '<client_key>';
      document.getElementById('rpcExample').value = location.origin + '/?api-key=' + firstClient;
      document.getElementById('restExample').value = location.origin + '/v1/wallet/<wallet>/balances?api-key=' + firstClient;
      renderKeyEditors();
    }

    function renderStatus() {
      if (!status) return;
      document.getElementById('configuredKeys').textContent = status.configured_keys ?? 0;
      document.getElementById('enabledKeys').textContent = status.enabled_keys ?? 0;
      document.getElementById('availableKeys').textContent = status.available_keys ?? 0;
      document.getElementById('stickyMetric').textContent = status.sticky_limit ?? '-';
      const rows = document.getElementById('keyStatusRows');
      rows.innerHTML = '';
      for (const key of status.keys || []) {
        const usage = key.usage || key.state?.usage;
        const credits = usage ? escapeHtml((usage.creditsRemaining ?? '-') + ' remaining / ' + (usage.creditsUsed ?? '-') + ' used') : '<span class="muted">not loaded</span>';
        const plan = usage?.subscriptionDetails?.plan ? '<div class="muted">' + escapeHtml(usage.subscriptionDetails.plan) + '</div>' : '';
        const cycle = usage?.subscriptionDetails?.billingCycle ? '<div class="muted">' + escapeHtml(usage.subscriptionDetails.billingCycle.start || '') + ' - ' + escapeHtml(usage.subscriptionDetails.billingCycle.end || '') + '</div>' : '';
        const state = key.state || {};
        const statusClass = key.available ? 'ok' : (key.enabled ? 'warn' : 'bad');
        const statusText = key.available ? 'available' : (key.enabled ? 'cooldown' : 'disabled');
        const tr = document.createElement('tr');
        tr.innerHTML =
          '<td>' + escapeHtml(key.name) + '<div class="muted mono">' + escapeHtml(key.id) + '</div></td>' +
          '<td class="mono">' + escapeHtml(key.api_key_masked || '') + '</td>' +
          '<td class="mono">' + escapeHtml(key.project_id || '') + '</td>' +
          '<td><span class="pill ' + statusClass + '">' + statusText + '</span>' + (key.cooldown_until ? '<div class="muted">until ' + escapeHtml(key.cooldown_until) + '</div>' : '') + '</td>' +
          '<td>' + credits + plan + cycle + (state.usage_error ? '<div class="bad">' + escapeHtml(state.usage_error) + '</div>' : '') + '</td>' +
          '<td>' + (state.request_count || 0) + '<div class="muted">ok ' + (state.success_count || 0) + ' / fail ' + (state.failure_count || 0) + '</div></td>' +
          '<td>' + escapeHtml(state.last_error || '') + '</td>';
        rows.appendChild(tr);
      }
    }

    function renderKeyEditors() {
      const rows = document.getElementById('keyEditRows');
      rows.innerHTML = '';
      for (const key of config.helius.keys || []) {
        const tr = document.createElement('tr');
        tr.dataset.id = key.id;
        const apiKeyValue = key.id ? '' : (key.api_key || '');
        const apiKeyPlaceholder = key.id ? (key.api_key_masked || 'leave blank to keep') : 'new key will be saved';
        tr.innerHTML =
          '<td><input type="checkbox" data-field="enabled" ' + (key.enabled ? 'checked' : '') + '></td>' +
          '<td><input data-field="name" value="' + escapeAttr(key.name || '') + '"><div class="muted mono">' + escapeHtml(key.id) + '</div></td>' +
          '<td><input data-field="project_id" value="' + escapeAttr(key.project_id || '') + '"></td>' +
          '<td><input data-field="api_key" value="' + escapeAttr(apiKeyValue) + '" placeholder="' + escapeAttr(apiKeyPlaceholder) + '"></td>' +
          '<td><button class="danger" data-action="remove">Remove</button></td>';
        rows.appendChild(tr);
      }
      rows.querySelectorAll('[data-action="remove"]').forEach(btn => {
        btn.addEventListener('click', () => btn.closest('tr').remove());
      });
    }

    function collectConfig() {
      const keys = [];
      document.querySelectorAll('#keyEditRows tr').forEach(row => {
        keys.push({
          id: row.dataset.id || '',
          name: row.querySelector('[data-field="name"]').value.trim(),
          project_id: row.querySelector('[data-field="project_id"]').value.trim(),
          api_key: row.querySelector('[data-field="api_key"]').value.trim(),
          enabled: row.querySelector('[data-field="enabled"]').checked
        });
      });
      return {
        server: {
          host: document.getElementById('serverHost').value.trim() || '0.0.0.0',
          port: Number(document.getElementById('serverPort').value || 18081)
        },
        auth: {
          client_keys: lines(document.getElementById('clientKeys').value),
          admin_keys: lines(document.getElementById('adminKeys').value)
        },
        helius: {
          rpc_base_url: document.getElementById('rpcBaseUrl').value.trim(),
          rest_base_url: document.getElementById('restBaseUrl').value.trim(),
          admin_base_url: document.getElementById('adminBaseUrl').value.trim(),
          keys
        },
        routing: {
          sticky_round_robin_limit: Number(document.getElementById('stickyLimit').value || 1),
          cooldown_seconds: Number(document.getElementById('cooldownSeconds').value || 60),
          request_timeout_seconds: Number(document.getElementById('timeoutSeconds').value || 30),
          max_body_bytes: Number(document.getElementById('maxBodyBytes').value || 33554432)
        }
      };
    }

    async function loadAll() {
      const cfg = await api('/api/admin/config');
      config = cfg.data;
      fillConfig();
      const st = await api('/api/admin/status');
      status = st.data;
      renderStatus();
      setOutput({ loaded: true, at: new Date().toISOString() });
    }

    async function saveConfig() {
      const data = await api('/api/admin/config', { method: 'PUT', body: JSON.stringify(collectConfig()) });
      config = data.data;
      fillConfig();
      await loadStatusOnly();
      setOutput(data);
    }

    async function loadStatusOnly() {
      const st = await api('/api/admin/status');
      status = st.data;
      renderStatus();
    }

    async function refreshUsage() {
      const data = await api('/api/admin/usage');
      setOutput(data);
      await loadStatusOnly();
    }

    async function testRPC() {
      const res = await fetch(withKey('/'), {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'getHealth' })
      });
      const text = await res.text();
      let data = text;
      try { data = JSON.parse(text); } catch {}
      setOutput({ status: res.status, response: data });
      await loadStatusOnly();
    }

    function addKey() {
      const name = document.getElementById('newKeyName').value.trim();
      const project = document.getElementById('newProjectId').value.trim();
      const apiKey = document.getElementById('newApiKey').value.trim();
      if (!apiKey) { setOutput('New Helius API key is required.'); return; }
      config.helius.keys.push({ id: '', name: name || 'helius-' + (config.helius.keys.length + 1), project_id: project, api_key: apiKey, api_key_masked: '', enabled: true });
      document.getElementById('newKeyName').value = '';
      document.getElementById('newProjectId').value = '';
      document.getElementById('newApiKey').value = '';
      renderKeyEditors();
    }

    function escapeHtml(value) {
      return String(value ?? '').replace(/[&<>"]/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[ch]));
    }
    function escapeAttr(value) { return escapeHtml(value).replace(/'/g, '&#39;'); }

    document.getElementById('refreshBtn').addEventListener('click', loadAll);
    document.getElementById('usageBtn').addEventListener('click', refreshUsage);
    document.getElementById('testBtn').addEventListener('click', testRPC);
    document.getElementById('saveBtn').addEventListener('click', saveConfig);
    document.getElementById('addKeyBtn').addEventListener('click', addKey);

    loadAll().catch(err => setOutput(String(err)));
  </script>
</body>
</html>`
