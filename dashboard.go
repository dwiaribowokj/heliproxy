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
      --bg: #f5f7fb;
      --bg-soft: #eef7f5;
      --surface: rgba(255, 255, 255, .86);
      --surface-strong: #ffffff;
      --line: #dce3ee;
      --text: #111827;
      --muted: #64748b;
      --accent: #0f766e;
      --accent-2: #14b8a6;
      --danger: #b42318;
      --warn: #a15c00;
      --ok: #087443;
      --code: #eef2f7;
      --shadow: 0 18px 55px rgba(15, 23, 42, .12);
      --shadow-soft: 0 10px 30px rgba(15, 23, 42, .07);
      --radius: 18px;
    }
    @media (prefers-color-scheme: dark) {
      :root {
        --bg: #090d14;
        --bg-soft: #0b1718;
        --surface: rgba(17, 24, 39, .78);
        --surface-strong: #111827;
        --line: #263244;
        --text: #eef2f7;
        --muted: #9aa8bd;
        --accent: #2dd4bf;
        --accent-2: #5eead4;
        --danger: #ff8a80;
        --warn: #fbbf24;
        --ok: #34d399;
        --code: #151d2a;
        --shadow: 0 18px 55px rgba(0, 0, 0, .42);
        --shadow-soft: 0 10px 30px rgba(0, 0, 0, .25);
      }
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background:
        radial-gradient(circle at top left, color-mix(in srgb, var(--accent-2) 20%, transparent), transparent 34rem),
        radial-gradient(circle at top right, rgba(20, 184, 166, .13), transparent 30rem),
        linear-gradient(180deg, var(--bg-soft), var(--bg) 34rem);
      color: var(--text);
      letter-spacing: 0;
    }
    header {
      position: sticky;
      top: 0;
      z-index: 10;
      border-bottom: 1px solid color-mix(in srgb, var(--line) 75%, transparent);
      background: color-mix(in srgb, var(--surface) 92%, transparent);
      backdrop-filter: blur(16px);
    }
    main, .bar {
      max-width: 1240px;
      margin: 0 auto;
      padding: 18px;
    }
    main { padding-bottom: 108px; }
    .bar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 18px;
    }
    h1 { margin: 0; font-size: 23px; line-height: 1.15; letter-spacing: -.03em; }
    h2 { margin: 0 0 12px; font-size: 16px; letter-spacing: -.01em; }
    .sub { color: var(--muted); font-size: 13px; margin-top: 5px; }
    .brand-row { display: flex; align-items: center; gap: 11px; }
    .logo-mark {
      width: 40px;
      height: 40px;
      display: grid;
      place-items: center;
      border-radius: 14px;
      background: linear-gradient(135deg, var(--accent), var(--accent-2));
      color: white;
      font-weight: 900;
      box-shadow: var(--shadow-soft);
    }
    .actions { display: flex; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
    button {
      border: 1px solid var(--line);
      background: var(--surface-strong);
      color: var(--text);
      height: 36px;
      padding: 0 13px;
      border-radius: 10px;
      cursor: pointer;
      font-weight: 700;
      transition: transform .15s ease, border-color .15s ease, background .15s ease, box-shadow .15s ease;
    }
    button:hover:not(:disabled) { transform: translateY(-1px); border-color: color-mix(in srgb, var(--accent) 42%, var(--line)); box-shadow: var(--shadow-soft); }
    button.primary { background: linear-gradient(135deg, var(--accent), var(--accent-2)); border-color: transparent; color: #ffffff; }
    button.danger { color: var(--danger); }
    button.big { height: 42px; padding: 0 18px; border-radius: 999px; }
    button:disabled { opacity: .52; cursor: not-allowed; }
    section {
      background: var(--surface);
      border: 1px solid color-mix(in srgb, var(--line) 84%, transparent);
      border-radius: var(--radius);
      padding: 18px;
      margin: 16px 0;
      box-shadow: var(--shadow-soft);
      backdrop-filter: blur(12px);
    }
    .section-head { display: flex; justify-content: space-between; gap: 12px; align-items: flex-start; margin-bottom: 14px; }
    .section-head h2 { margin-bottom: 5px; }
    .hint { color: var(--muted); font-size: 13px; line-height: 1.45; }
    .dirty-card { border-color: color-mix(in srgb, var(--accent) 42%, var(--line)); box-shadow: inset 4px 0 0 var(--accent), var(--shadow-soft); }
    .grid {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 12px;
    }
    .metric {
      position: relative;
      overflow: hidden;
      border: 1px solid var(--line);
      border-radius: 16px;
      padding: 14px;
      background: linear-gradient(180deg, color-mix(in srgb, var(--surface-strong) 90%, transparent), color-mix(in srgb, var(--surface) 86%, transparent));
    }
    .metric:before {
      content: "";
      position: absolute;
      inset: 0 auto 0 0;
      width: 4px;
      background: linear-gradient(180deg, var(--accent), var(--accent-2));
      opacity: .78;
    }
    .metric .label { color: var(--muted); font-size: 12px; font-weight: 700; text-transform: uppercase; letter-spacing: .06em; }
    .metric .value { font-size: 27px; font-weight: 800; margin-top: 6px; letter-spacing: -.04em; }
    .table-wrap { overflow-x: auto; border: 1px solid var(--line); border-radius: 14px; background: var(--surface-strong); }
    table { width: 100%; border-collapse: collapse; min-width: 820px; }
    th, td { padding: 12px; border-bottom: 1px solid var(--line); text-align: left; vertical-align: top; font-size: 13px; }
    th { color: var(--muted); font-size: 11px; font-weight: 800; text-transform: uppercase; letter-spacing: .06em; background: color-mix(in srgb, var(--surface-strong) 88%, var(--line)); position: sticky; top: 0; }
    tr:last-child td { border-bottom: 0; }
    tbody tr:hover { background: color-mix(in srgb, var(--accent) 5%, transparent); }
    input, textarea {
      width: 100%;
      min-height: 36px;
      border: 1px solid var(--line);
      border-radius: 10px;
      background: var(--surface-strong);
      color: var(--text);
      padding: 8px 10px;
      font: inherit;
      outline: none;
      transition: border-color .15s ease, box-shadow .15s ease;
    }
    input:focus, textarea:focus { border-color: var(--accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 18%, transparent); }
    textarea { min-height: 76px; resize: vertical; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
    label { display: block; color: var(--muted); font-size: 12px; font-weight: 700; margin-bottom: 6px; }
    .form-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; }
    .mono { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
    .pill { display: inline-flex; align-items: center; min-height: 24px; padding: 0 9px; border-radius: 999px; font-size: 12px; font-weight: 800; border: 1px solid currentColor; background: color-mix(in srgb, currentColor 9%, transparent); }
    .ok { color: var(--ok); }
    .warn { color: var(--warn); }
    .bad { color: var(--danger); }
    .muted { color: var(--muted); }
    .row-actions { display: flex; gap: 6px; }
    pre { background: var(--code); padding: 13px; border-radius: 14px; overflow: auto; min-height: 80px; border: 1px solid var(--line); }
    .save-dock {
      position: fixed;
      left: 50%;
      bottom: 18px;
      transform: translate(-50%, 120%);
      display: flex;
      align-items: center;
      gap: 12px;
      width: min(92vw, 590px);
      padding: 12px 14px;
      border: 1px solid var(--line);
      border-radius: 999px;
      background: color-mix(in srgb, var(--surface-strong) 92%, transparent);
      box-shadow: var(--shadow);
      backdrop-filter: blur(16px);
      opacity: 0;
      pointer-events: none;
      transition: transform .2s ease, opacity .2s ease;
      z-index: 20;
    }
    .save-dock.visible { transform: translate(-50%, 0); opacity: 1; pointer-events: auto; }
    .save-dock .save-copy { flex: 1; min-width: 0; }
    .save-title { font-size: 13px; font-weight: 800; }
    .save-detail { font-size: 12px; color: var(--muted); margin-top: 2px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .unsaved-badge { display: none; margin-left: 8px; color: var(--warn); font-size: 12px; font-weight: 800; }
    .unsaved-badge.visible { display: inline; }
    @media (max-width: 900px) {
      .grid, .form-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    }
    @media (max-width: 760px) {
      .bar { align-items: flex-start; flex-direction: column; }
      .actions { justify-content: flex-start; }
      .grid, .form-grid { grid-template-columns: 1fr; }
      main, .bar { padding: 14px; }
      .save-dock { border-radius: 18px; align-items: stretch; flex-direction: column; }
      .save-dock button { width: 100%; }
    }
  </style>
</head>
<body>
  <header>
    <div class="bar">
      <div class="brand-row">
        <div class="logo-mark">H</div>
        <div>
          <h1>Heliproxy</h1>
          <div class="sub">Helius-compatible RPC proxy with sticky round-robin key rotation.</div>
        </div>
      </div>
      <div class="actions">
        <button id="refreshBtn">Refresh</button>
        <button id="usageBtn">Refresh Usage</button>
        <button id="testBtn">Test RPC</button>
        <button id="saveBtn" class="primary" disabled>Saved</button>
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

    <section id="configSection">
      <div class="section-head">
        <div>
          <h2>Config <span id="configDirtyBadge" class="unsaved-badge">Unsaved changes</span></h2>
          <div class="hint">Ubah setting di sini. Tombol save akan aktif dan muncul mengambang hanya kalau ada perubahan yang perlu disimpan.</div>
        </div>
      </div>
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

    <section id="keysSection">
      <div class="section-head">
        <div>
          <h2>Edit Helius Keys <span id="keysDirtyBadge" class="unsaved-badge">Unsaved changes</span></h2>
          <div class="hint">Untuk key lama, kosongkan kolom API Key kalau tidak ingin mengganti secret. Project ID opsional, tapi diperlukan untuk usage/billing display.</div>
        </div>
      </div>
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
  <div id="saveDock" class="save-dock" role="status" aria-live="polite">
    <div class="save-copy">
      <div class="save-title">Ada perubahan config belum disimpan</div>
      <div id="saveDetail" class="save-detail">Review perubahan, lalu simpan.</div>
    </div>
    <button id="discardBtn">Discard</button>
    <button id="floatingSaveBtn" class="primary big">Save Config</button>
  </div>

  <script>
    const qs = new URLSearchParams(location.search);
    const adminKey = qs.get('api-key') || qs.get('api_key') || '';
    let config = null;
    let status = null;
    let baselineConfigJSON = '';
    let isHydrating = false;

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
      baselineConfigJSON = stableStringify(collectConfig());
      updateDirtyState();
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
        const credits = renderUsageSummary(usage);
        const plan = usage?.subscriptionDetails?.plan ? '<div class="muted">' + escapeHtml(usage.subscriptionDetails.plan) + (usage.estimate ? ' estimate' : '') + '</div>' : '';
        const cycle = usage?.subscriptionDetails?.billingCycle?.start || usage?.subscriptionDetails?.billingCycle?.end ? '<div class="muted">' + escapeHtml(usage.subscriptionDetails.billingCycle.start || '') + ' - ' + escapeHtml(usage.subscriptionDetails.billingCycle.end || '') + '</div>' : '';
        const note = usage?.note ? '<div class="warn">' + escapeHtml(usage.note) + '</div>' : '';
        const state = key.state || {};
        const statusClass = key.available ? 'ok' : (key.enabled ? 'warn' : 'bad');
        const statusText = key.available ? 'available' : (key.enabled ? 'cooldown' : 'disabled');
        const tr = document.createElement('tr');
        tr.innerHTML =
          '<td>' + escapeHtml(key.name) + '<div class="muted mono">' + escapeHtml(key.id) + '</div></td>' +
          '<td class="mono">' + escapeHtml(key.api_key_masked || '') + '</td>' +
          '<td class="mono">' + escapeHtml(key.project_id || '') + '</td>' +
          '<td><span class="pill ' + statusClass + '">' + statusText + '</span>' + (key.cooldown_until ? '<div class="muted">until ' + escapeHtml(key.cooldown_until) + '</div>' : '') + '</td>' +
          '<td>' + credits + plan + cycle + note + (state.usage_error ? '<div class="bad">' + escapeHtml(state.usage_error) + '</div>' : '') + '</td>' +
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
        btn.addEventListener('click', () => { btn.closest('tr').remove(); updateDirtyState(); });
      });
      rows.querySelectorAll('input').forEach(input => input.addEventListener('input', updateDirtyState));
      rows.querySelectorAll('input[type="checkbox"]').forEach(input => input.addEventListener('change', updateDirtyState));
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
      updateDirtyState();
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
      updateDirtyState();
    }

    function renderUsageSummary(usage) {
      if (!usage) return '<span class="muted">not loaded</span>';
      const limit = usage.subscriptionDetails?.creditsLimit;
      const used = usage.creditsUsed;
      const remaining = usage.creditsRemaining;
      if (usage.estimate) {
        return '<span class="pill warn">Free plan limit ' + escapeHtml(formatNumber(limit || 1000000)) + ' credits/month</span><div class="muted">Used/remaining unavailable from Helius Admin API.</div>';
      }
      return escapeHtml(formatNumber(remaining ?? '-') + ' remaining / ' + formatNumber(used ?? '-') + ' used');
    }

    function formatNumber(value) {
      if (typeof value !== 'number' || !isFinite(value) || value < 0) return '-';
      return new Intl.NumberFormat().format(value);
    }

    function stableStringify(value) {
      return JSON.stringify(sortValue(value));
    }

    function sortValue(value) {
      if (Array.isArray(value)) return value.map(sortValue);
      if (!value || typeof value !== 'object') return value;
      return Object.keys(value).sort().reduce((acc, key) => {
        acc[key] = sortValue(value[key]);
        return acc;
      }, {});
    }

    function updateDirtyState() {
      if (!config || isHydrating) return;
      const current = stableStringify(collectConfig());
      const dirty = current !== baselineConfigJSON;
      document.getElementById('saveBtn').disabled = !dirty;
      document.getElementById('saveBtn').textContent = dirty ? 'Save Config' : 'Saved';
      document.getElementById('floatingSaveBtn').disabled = !dirty;
      document.getElementById('saveDock').classList.toggle('visible', dirty);
      document.getElementById('configSection').classList.toggle('dirty-card', dirty);
      document.getElementById('keysSection').classList.toggle('dirty-card', dirty);
      document.getElementById('configDirtyBadge').classList.toggle('visible', dirty);
      document.getElementById('keysDirtyBadge').classList.toggle('visible', dirty);
      document.getElementById('saveDetail').textContent = dirty ? 'Klik Save Config untuk menulis ke config.yaml.' : 'Tidak ada perubahan.';
    }

    function watchFormChanges() {
      document.querySelectorAll('input, textarea').forEach(el => {
        if (el.disabled) return;
        el.addEventListener('input', updateDirtyState);
        el.addEventListener('change', updateDirtyState);
      });
    }

    function discardChanges() {
      if (!config) return;
      fillConfig();
      setOutput('Perubahan lokal dibatalkan. Config di server tidak diubah.');
    }

    function escapeHtml(value) {
      return String(value ?? '').replace(/[&<>"]/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[ch]));
    }
    function escapeAttr(value) { return escapeHtml(value).replace(/'/g, '&#39;'); }

    document.getElementById('refreshBtn').addEventListener('click', loadAll);
    document.getElementById('usageBtn').addEventListener('click', refreshUsage);
    document.getElementById('testBtn').addEventListener('click', testRPC);
    document.getElementById('saveBtn').addEventListener('click', saveConfig);
    document.getElementById('floatingSaveBtn').addEventListener('click', saveConfig);
    document.getElementById('discardBtn').addEventListener('click', discardChanges);
    document.getElementById('addKeyBtn').addEventListener('click', addKey);
    watchFormChanges();

    loadAll().catch(err => setOutput(String(err)));
  </script>
</body>
</html>`
