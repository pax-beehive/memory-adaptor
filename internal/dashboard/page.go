package dashboard

// pageHTML is the single-page dashboard UI. It is intentionally framework-free:
// one HTML document with inline CSS and vanilla JS that polls the JSON APIs.
const pageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>paxm dashboard</title>
<style>
  :root {
    --bg: #0b0e14;
    --panel: #12161f;
    --panel-2: #171c27;
    --border: rgba(255, 255, 255, 0.07);
    --border-strong: rgba(255, 255, 255, 0.12);
    --text: #e6e9ef;
    --muted: #8b93a5;
    --faint: #5c6373;
    --accent: #7c6cff;
    --accent-2: #4f8dff;
    --ok: #3fb68b;
    --warn: #d9a03f;
    --bad: #e5606c;
    --mono: "SF Mono", ui-monospace, Menlo, Consolas, monospace;
  }
  * { box-sizing: border-box; }
  html { color-scheme: dark; }
  body {
    margin: 0; color: var(--text); background: var(--bg);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
    font-size: 14px; line-height: 1.5;
    background-image: radial-gradient(ellipse 80% 50% at 50% -10%, rgba(124, 108, 255, 0.08), transparent);
  }
  a { color: var(--accent-2); text-decoration: none; }

  header {
    position: sticky; top: 0; z-index: 10;
    display: flex; align-items: center; gap: 12px;
    padding: 14px 24px;
    background: rgba(11, 14, 20, 0.85);
    backdrop-filter: blur(12px); -webkit-backdrop-filter: blur(12px);
    border-bottom: 1px solid var(--border);
  }
  .logo {
    width: 22px; height: 22px; border-radius: 7px; flex: none;
    background: linear-gradient(135deg, var(--accent), var(--accent-2));
    box-shadow: 0 0 14px rgba(124, 108, 255, 0.45);
  }
  header h1 { font-size: 15px; font-weight: 600; margin: 0; letter-spacing: 0.2px; }
  header .hint { color: var(--faint); font-size: 12px; flex: 1; }
  .btn {
    padding: 6px 14px; border-radius: 8px; font-size: 13px; cursor: pointer;
    color: var(--text); background: var(--panel-2); border: 1px solid var(--border-strong);
    transition: border-color 0.15s, background 0.15s;
  }
  .btn:hover { border-color: var(--accent); background: #1c2230; }

  nav { display: flex; gap: 6px; padding: 10px 24px 0; }
  nav button {
    border: 1px solid transparent; background: none; color: var(--muted);
    padding: 7px 16px; font-size: 13px; font-weight: 500; cursor: pointer; border-radius: 999px;
    transition: color 0.15s, background 0.15s;
  }
  nav button:hover { color: var(--text); }
  nav button.active { color: var(--text); background: var(--panel-2); border-color: var(--border-strong); }

  main { padding: 22px 24px 40px; max-width: 1240px; margin: 0 auto; }

  .cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 12px; margin-bottom: 18px; }
  .card {
    background: var(--panel); border: 1px solid var(--border); border-radius: 12px;
    padding: 14px 16px 12px; position: relative; overflow: hidden;
  }
  .card::before {
    content: ""; position: absolute; top: 0; left: 0; right: 0; height: 2px;
    background: linear-gradient(90deg, var(--accent), transparent);
    opacity: 0.55;
  }
  .card .num { font-size: 26px; font-weight: 650; font-variant-numeric: tabular-nums; letter-spacing: -0.5px; }
  .card .label { color: var(--muted); font-size: 12px; margin-top: 2px; }

  .panel {
    background: var(--panel); border: 1px solid var(--border); border-radius: 12px;
    padding: 18px; margin-bottom: 18px;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.35);
  }
  .panel h2 { font-size: 13px; font-weight: 600; margin: 0 0 14px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.6px; }
  .grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 18px; }
  @media (max-width: 900px) { .grid-2 { grid-template-columns: 1fr; } }

  table { border-collapse: collapse; width: 100%; font-size: 13px; }
  th {
    text-align: left; padding: 8px 12px; color: var(--faint);
    font-weight: 600; font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px;
    border-bottom: 1px solid var(--border-strong); white-space: nowrap;
  }
  td { padding: 9px 12px; border-bottom: 1px solid var(--border); vertical-align: top; }
  tbody tr:last-child td { border-bottom: 0; }
  td.num, th.num { text-align: right; font-variant-numeric: tabular-nums; }
  tr.clickable { cursor: pointer; transition: background 0.12s; }
  tr.clickable:hover { background: rgba(124, 108, 255, 0.05); }
  tr.detail td { background: var(--panel-2); border-bottom: 1px solid var(--border-strong); }

  .bars { display: flex; align-items: flex-end; gap: 5px; height: 90px; padding-top: 6px; }
  .bar {
    flex: 1; border-radius: 4px 4px 0 0; min-height: 3px; position: relative;
    background: linear-gradient(180deg, var(--accent), rgba(124, 108, 255, 0.35));
    transition: opacity 0.15s;
  }
  .bar:hover { opacity: 0.75; }
  .bar span {
    position: absolute; bottom: -20px; left: 0; right: 0; text-align: center;
    font-size: 10px; color: var(--faint); white-space: nowrap; font-variant-numeric: tabular-nums;
  }
  .barspace { height: 26px; }

  .tag {
    display: inline-flex; align-items: center; padding: 2px 9px; border-radius: 999px;
    font-size: 11px; font-weight: 600; letter-spacing: 0.2px;
  }
  .tag.active { background: rgba(63, 182, 139, 0.14); color: var(--ok); }
  .tag.passive { background: rgba(217, 160, 63, 0.14); color: var(--warn); }
  .tag.err { background: rgba(229, 96, 108, 0.14); color: var(--bad); }
  .tag.mcp { background: rgba(139, 147, 165, 0.16); color: var(--muted); }

  .controls { display: flex; gap: 10px; margin-bottom: 14px; align-items: center; flex-wrap: wrap; }
  .controls label { font-size: 12px; color: var(--muted); display: inline-flex; align-items: center; gap: 6px; }
  .controls input, .controls select {
    padding: 6px 10px; font-size: 13px; color: var(--text);
    background: var(--panel); border: 1px solid var(--border-strong); border-radius: 8px;
    outline: none; transition: border-color 0.15s, box-shadow 0.15s;
  }
  .controls input:focus, .controls select:focus { border-color: var(--accent); box-shadow: 0 0 0 3px rgba(124, 108, 255, 0.18); }
  .controls input::placeholder { color: var(--faint); }

  .pager { display: flex; gap: 12px; align-items: center; margin-top: 14px; font-size: 12px; color: var(--muted); font-variant-numeric: tabular-nums; }
  .pager button {
    padding: 4px 12px; border-radius: 7px; font-size: 12px; cursor: pointer;
    color: var(--text); background: var(--panel-2); border: 1px solid var(--border-strong);
  }
  .pager button:hover:not(:disabled) { border-color: var(--accent); }
  .pager button:disabled { opacity: 0.35; cursor: default; }
  .pager select {
    padding: 4px 8px; font-size: 12px; color: var(--text);
    background: var(--panel-2); border: 1px solid var(--border-strong); border-radius: 7px;
  }

  .muted { color: var(--muted); }
  .faint { color: var(--faint); }
  .hits { margin-top: 4px; }
  .hits td { font-size: 12px; border-bottom: 1px solid var(--border); }
  .err-text { color: var(--bad); font-size: 12px; }
  code {
    font-family: var(--mono); font-size: 11.5px; color: #b7bdd0;
    background: rgba(255, 255, 255, 0.05); border-radius: 5px; padding: 2px 6px;
    word-break: break-all;
  }
  .scorebar {
    display: inline-block; width: 72px; height: 5px; border-radius: 3px;
    background: rgba(255, 255, 255, 0.08); vertical-align: middle; margin-right: 8px; overflow: hidden;
  }
  .scorebar i { display: block; height: 100%; border-radius: 3px; background: linear-gradient(90deg, var(--accent-2), var(--accent)); }
  .scoreval { font-variant-numeric: tabular-nums; color: var(--muted); font-size: 12px; }
  .empty { padding: 28px 0; text-align: center; color: var(--faint); font-size: 13px; }
  .kv { display: flex; gap: 8px; align-items: baseline; font-size: 12px; color: var(--muted); margin-top: 6px; }
</style>
</head>
<body>
<header>
  <div class="logo"></div>
  <h1>paxm dashboard</h1>
  <span class="hint">read-only local telemetry</span>
  <button class="btn" id="refresh">Refresh</button>
</header>
<nav>
  <button data-tab="overview" class="active">Overview</button>
  <button data-tab="recalls">Recalls</button>
  <button data-tab="sessions">Sessions</button>
  <button data-tab="logs">Logs</button>
</nav>
<main id="root"></main>
<script>
(function () {
  var root = document.getElementById('root');
  var tabs = ['overview', 'recalls', 'sessions', 'logs'];
  var tab = tabs.indexOf(location.hash.slice(1)) >= 0 ? location.hash.slice(1) : 'overview';
  var expanded = {};
  var recalls = { mode: 'all', state: '', profile: '', q: '', hitProvider: null, hitQ: '', limit: 50, offset: 0 };
  var logs = { kind: '', limit: 50, offset: 0 };
  var sessionTimeline = null;
  var providerNames = null;

  function esc(value) {
    return String(value == null ? '' : value)
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }
  function fmtTime(iso) {
    if (!iso) return '';
    var d = new Date(iso);
    return isNaN(d) ? iso : d.toLocaleString();
  }
  function relTime(iso) {
    var d = new Date(iso);
    if (isNaN(d)) return '';
    var s = Math.max(0, (Date.now() - d.getTime()) / 1000);
    if (s < 60) return Math.floor(s) + 's ago';
    if (s < 3600) return Math.floor(s / 60) + 'm ago';
    if (s < 86400) return Math.floor(s / 3600) + 'h ago';
    return Math.floor(s / 86400) + 'd ago';
  }
  function fmtNum(n) { return n == null ? '0' : String(n); }
  function fetchJSON(url) {
    return fetch(url).then(function (r) {
      if (!r.ok) throw new Error(url + ': HTTP ' + r.status);
      return r.json();
    });
  }
  function recallModeOf(e) {
    if (e.source === 'hook') return 'passive';
    if (e.source === 'mcp') return 'mcp';
    return 'active';
  }
  function eventKey(e) { return e.time + '|' + e.kind + '|' + (e.query_hash || ''); }
  function showError(err) {
    root.innerHTML = '<div class="panel err-text">' + esc(err.message || err) + '</div>';
  }
  function scoreBar(score) {
    var pct = Math.max(0, Math.min(100, Math.round((score || 0) * 100)));
    return '<span class="scorebar"><i style="width:' + pct + '%"></i></span><span class="scoreval">' + (score || 0).toFixed(3) + '</span>';
  }
  function pagerHTML(total, st) {
    var from = total === 0 ? 0 : st.offset + 1;
    var to = Math.min(st.offset + st.limit, total);
    return '<div class="pager">' +
      '<button data-page="prev" ' + (st.offset === 0 ? 'disabled' : '') + '>&larr; Prev</button>' +
      '<span>' + from + '&ndash;' + to + ' of ' + total + '</span>' +
      '<button data-page="next" ' + (to >= total ? 'disabled' : '') + '>Next &rarr;</button>' +
      '<select data-pagesize>' + [20, 50, 100].map(function (n) {
        return '<option value="' + n + '"' + (st.limit === n ? ' selected' : '') + '>' + n + ' / page</option>';
      }).join('') + '</select></div>';
  }
  function bindPager(st, rerender) {
    Array.prototype.forEach.call(document.querySelectorAll('.pager button'), function (btn) {
      btn.onclick = function () {
        st.offset += btn.getAttribute('data-page') === 'next' ? st.limit : -st.limit;
        if (st.offset < 0) st.offset = 0;
        rerender();
      };
    });
    var size = document.querySelector('[data-pagesize]');
    if (size) size.onchange = function () { st.limit = parseInt(this.value, 10); st.offset = 0; rerender(); };
  }

  function renderOverview() {
    fetchJSON('/api/summary').then(function (s) {
      var t = s.totals || {};
      var cards = [
        ['Recalls', t.recalls], ['Hits', t.hits], ['Inserted', t.inserted],
        ['Writes', t.writes], ['Items', t.items], ['Recall timeouts', t.recall_timeouts],
        ['Provider errors', t.provider_errors], ['Events', t.events]
      ].map(function (c) {
        return '<div class="card"><div class="num">' + fmtNum(c[1]) + '</div><div class="label">' + esc(c[0]) + '</div></div>';
      }).join('');
      var maxDaily = 1;
      (s.daily || []).forEach(function (d) { if (d.counter.events > maxDaily) maxDaily = d.counter.events; });
      var bars = (s.daily || []).map(function (d) {
        var h = Math.max(3, Math.round(84 * d.counter.events / maxDaily));
        return '<div class="bar" style="height:' + h + 'px" title="' + esc(d.date) + ': ' + d.counter.events + ' events"><span>' + esc(d.date.slice(5)) + '</span></div>';
      }).join('') || '<div class="empty">no events in this window</div>';
      function namedRows(rows, pick) {
        return (rows || []).map(function (r) {
          return '<tr><td>' + esc(r.name) + '</td>' + pick(r.counter).map(function (v) { return '<td class="num">' + fmtNum(v) + '</td>'; }).join('') + '</tr>';
        }).join('') || '<tr><td colspan="5" class="muted">no data</td></tr>';
      }
      var pick = function (c) { return [c.recalls, c.hits, c.writes, c.errors]; };
      root.innerHTML =
        '<div class="cards">' + cards + '</div>' +
        '<div class="panel"><h2>Events per day &middot; ' + s.days + 'd window</h2><div class="bars">' + bars + '</div><div class="barspace"></div></div>' +
        '<div class="grid-2">' +
        '<div class="panel"><h2>By agent</h2><table><thead><tr><th>agent</th><th class="num">recalls</th><th class="num">hits</th><th class="num">writes</th><th class="num">errors</th></tr></thead><tbody>' + namedRows(s.agents, pick) + '</tbody></table></div>' +
        '<div class="panel"><h2>By provider</h2><table><thead><tr><th>provider</th><th class="num">recalls</th><th class="num">hits</th><th class="num">writes</th><th class="num">errors</th></tr></thead><tbody>' + namedRows(s.providers, pick) + '</tbody></table></div>' +
        '</div>' +
        '<div class="panel"><h2>Storage</h2><div class="muted"><code>' + esc((s.storage || {}).dir || '') + '</code> &middot; ' + fmtNum((s.storage || {}).event_bytes) + ' bytes, max ' + fmtNum((s.storage || {}).max_event_files) + ' files</div></div>';
    }).catch(showError);
  }

  function recallDetailHTML(e) {
    var html = '';
    if (e.error) html += '<div class="err-text">' + esc(e.error) + '</div>';
    if (e.recall_hits && e.recall_hits.length) {
      html += '<table class="hits"><thead><tr><th>#</th><th>provider</th><th>score</th><th>relevance</th><th>tier</th><th>text preview</th></tr></thead><tbody>';
      e.recall_hits.forEach(function (h, i) {
        html += '<tr><td class="faint">' + (i + 1) + '</td><td>' + esc(h.provider) + '</td><td>' + scoreBar(h.score) + '</td><td class="scoreval">' + (h.relevance || 0).toFixed(3) + '</td><td>' + esc(h.tier) + '</td><td>' + (h.text_preview ? esc(h.text_preview) : '<span class="faint">(text capture off)</span>') + '</td></tr>';
      });
      html += '</tbody></table>';
    } else {
      html += '<div class="faint">no hit details recorded on this event</div>';
    }
    if (e.provider_recall_details && e.provider_recall_details.length) {
      html += '<div class="kv">providers&nbsp;' + e.provider_recall_details.map(function (p) {
        return '<code>' + esc(p.provider) + ' &middot; ' + esc(p.outcome) + ' &middot; ' + fmtNum(p.duration_ms) + 'ms</code>';
      }).join(' ') + '</div>';
    }
    if (e.session_key) html += '<div class="kv">session&nbsp;<code>' + esc(e.session_key) + '</code></div>';
    return html;
  }

  function recallParams() {
    var params = 'kind=recall,hook_recall&limit=' + recalls.limit + '&offset=' + recalls.offset;
    if (recalls.mode === 'active') params += '&source=cli';
    if (recalls.mode === 'passive') params += '&source=hook';
    if (recalls.mode === 'mcp') params += '&source=mcp';
    if (recalls.state) params += '&state=' + encodeURIComponent(recalls.state);
    if (recalls.profile) params += '&profile=' + encodeURIComponent(recalls.profile);
    if (recalls.q) params += '&q=' + encodeURIComponent(recalls.q);
    if (recalls.hitProvider) params += '&hit_provider=' + encodeURIComponent(recalls.hitProvider);
    if (recalls.hitQ) params += '&hit_q=' + encodeURIComponent(recalls.hitQ);
    return params;
  }

  function renderRecalls() {
    var loaded = providerNames
      ? Promise.resolve(providerNames)
      : fetchJSON('/api/summary').then(function (s) {
          providerNames = (s.providers || []).map(function (p) { return p.name; });
          return providerNames;
        });
    loaded.then(function (names) {
      if (recalls.hitProvider === null) recalls.hitProvider = names.indexOf('sqlite') >= 0 ? 'sqlite' : '';
      fetchJSON('/api/events?' + recallParams()).then(function (data) {
        var events = data.events || [];
        var rows = events.map(function (e) {
          var mode = recallModeOf(e);
          var badge = mode === 'passive'
            ? '<span class="tag passive">passive ' + esc(e.target || '') + '/' + esc(e.hook_event || '') + '</span>'
            : '<span class="tag ' + (mode === 'mcp' ? 'mcp' : 'active') + '">' + mode + '</span>';
          var timeout = e.recall_timed_out ? ' <span class="tag err">timeout</span>' : '';
          var status = e.success ? '' : ' <span class="tag err">error</span>';
          var key = eventKey(e);
          var main = '<tr class="clickable" data-key="' + esc(key) + '">' +
            '<td><span title="' + esc(fmtTime(e.time)) + '">' + esc(relTime(e.time)) + '</span></td><td>' + badge + '</td>' +
            '<td>' + (e.query_preview ? esc(e.query_preview) : '<span class="faint">hash ' + esc((e.query_hash || '').slice(0, 8)) + '</span>') + '</td>' +
            '<td class="num">' + fmtNum(e.hit_count) + '</td><td class="num">' + fmtNum(e.inserted_count) + '</td><td>' + esc(e.profile || '') + '</td>' +
            '<td class="num">' + fmtNum(e.duration_ms) + 'ms' + timeout + status + '</td></tr>';
          var open = expanded[key] ? '' : ' style="display:none"';
          return main + '<tr class="detail" data-detail="' + esc(key) + '"' + open + '><td colspan="7">' + recallDetailHTML(e) + '</td></tr>';
        }).join('') || '<tr><td colspan="7"><div class="empty">no recall events match — adjust filters or run <code>paxm recall</code></div></td></tr>';
        root.innerHTML =
          '<div class="controls">' +
          '<label>mode <select id="mode"><option value="all">all</option><option value="active">active</option><option value="passive">passive</option><option value="mcp">mcp</option></select></label>' +
          '<label>state <select id="state"><option value="">all</option><option value="success">success</option><option value="error">error</option><option value="timeout">timeout</option></select></label>' +
          '<label>profile <input id="profile" size="8" placeholder="any" value="' + esc(recalls.profile) + '"></label>' +
          '<input id="search" size="18" placeholder="filter query preview" value="' + esc(recalls.q) + '">' +
          '<label>hit provider <select id="hitprovider"><option value="">all</option>' + names.map(function (n) { return '<option value="' + esc(n) + '">' + esc(n) + '</option>'; }).join('') + '</select></label>' +
          '<input id="hitsearch" size="16" placeholder="filter hit text" value="' + esc(recalls.hitQ) + '">' +
          '<span class="faint">click a row for hit details</span></div>' +
          '<div class="panel"><table><thead><tr><th>time</th><th>mode</th><th>query</th><th class="num">hits</th><th class="num">inserted</th><th>profile</th><th class="num">duration</th></tr></thead><tbody>' + rows + '</tbody></table>' +
          pagerHTML(data.total || 0, recalls) + '</div>';
        document.getElementById('mode').value = recalls.mode;
        document.getElementById('state').value = recalls.state;
        document.getElementById('hitprovider').value = recalls.hitProvider;
        document.getElementById('mode').onchange = function () { recalls.mode = this.value; recalls.offset = 0; renderRecalls(); };
        document.getElementById('state').onchange = function () { recalls.state = this.value; recalls.offset = 0; renderRecalls(); };
        document.getElementById('hitprovider').onchange = function () { recalls.hitProvider = this.value; recalls.offset = 0; renderRecalls(); };
        document.getElementById('hitsearch').onchange = function () { recalls.hitQ = this.value.trim(); recalls.offset = 0; renderRecalls(); };
        document.getElementById('profile').onchange = function () { recalls.profile = this.value.trim(); recalls.offset = 0; renderRecalls(); };
        document.getElementById('search').onchange = function () { recalls.q = this.value.trim(); recalls.offset = 0; renderRecalls(); };
        bindPager(recalls, renderRecalls);
        Array.prototype.forEach.call(document.querySelectorAll('tr.clickable'), function (row) {
          row.onclick = function () {
            var key = row.getAttribute('data-key');
            expanded[key] = !expanded[key];
            var detail = document.querySelector('tr[data-detail="' + key + '"]');
            if (detail) detail.style.display = expanded[key] ? '' : 'none';
          };
        });
      }).catch(showError);
    }).catch(showError);
  }

  function renderSessions() {
    fetchJSON('/api/sessions').then(function (data) {
      var rows = (data.sessions || []).map(function (s) {
        return '<tr class="clickable" data-key="' + esc(s.key) + '">' +
          '<td><span title="' + esc(fmtTime(s.last_seen)) + '">' + esc(relTime(s.last_seen)) + '</span></td>' +
          '<td>' + esc(s.target || '') + '</td><td><code>' + esc(s.key) + '</code></td>' +
          '<td class="num">' + fmtNum(s.recalls) + '</td><td class="num">' + fmtNum(s.writes) + '</td><td class="num">' + fmtNum(s.deliveries) + '</td>' +
          '<td>' + (s.last_query ? esc(s.last_query) : '') + '</td></tr>';
      }).join('') || '<tr><td colspan="7"><div class="empty">no sessions with capture activity yet</div></td></tr>';
      var timeline = '';
      if (sessionTimeline) {
        var lines = sessionTimeline.events.map(function (e) {
          return '<tr><td><span title="' + esc(fmtTime(e.time)) + '">' + esc(relTime(e.time)) + '</span></td><td>' + esc(e.kind) + '</td>' +
            '<td>' + (e.success ? '<span class="tag active">ok</span>' : '<span class="tag err">error</span>') + '</td>' +
            '<td>' + esc(e.query_preview || '') + '</td><td class="num">' + fmtNum(e.hit_count || e.item_count || 0) + '</td></tr>';
        }).join('');
        timeline = '<div class="panel"><h2>Timeline &middot; <code>' + esc(sessionTimeline.key) + '</code></h2><table><thead><tr><th>time</th><th>kind</th><th>status</th><th>query</th><th class="num">hits/items</th></tr></thead><tbody>' + lines + '</tbody></table></div>';
      }
      root.innerHTML =
        '<div class="panel"><h2>Sessions &middot; from capture session keys</h2><table><thead><tr><th>last seen</th><th>target</th><th>session key</th><th class="num">recalls</th><th class="num">writes</th><th class="num">deliveries</th><th>last query</th></tr></thead><tbody>' + rows + '</tbody></table></div>' +
        timeline;
      Array.prototype.forEach.call(document.querySelectorAll('tr.clickable'), function (row) {
        row.onclick = function () {
          var key = row.getAttribute('data-key');
          fetchJSON('/api/events?limit=500&session=' + encodeURIComponent(key)).then(function (data) {
            sessionTimeline = { key: key, events: data.events || [] };
            renderSessions();
          }).catch(showError);
        };
      });
    }).catch(showError);
  }

  function renderLogs() {
    var url = '/api/events?limit=' + logs.limit + '&offset=' + logs.offset + (logs.kind ? '&kind=' + encodeURIComponent(logs.kind) : '');
    fetchJSON(url).then(function (data) {
      var rows = (data.events || []).map(function (e) {
        var status = e.success ? '<span class="tag active">ok</span>' : (e.skipped ? '<span class="tag passive">skipped</span>' : '<span class="tag err">error</span>');
        return '<tr><td><span title="' + esc(fmtTime(e.time)) + '">' + esc(relTime(e.time)) + '</span></td><td>' + esc(e.kind) + '</td><td>' + esc(e.source || '') + '</td>' +
          '<td>' + status + '</td><td class="num">' + fmtNum(e.duration_ms) + 'ms</td>' +
          '<td>' + esc(e.query_preview || e.error || '') + '</td></tr>';
      }).join('') || '<tr><td colspan="6"><div class="empty">no events</div></td></tr>';
      var kinds = ['', 'recall', 'hook_recall', 'remember', 'hook_write', 'hook_delivery'];
      var options = kinds.map(function (k) { return '<option value="' + k + '">' + (k || 'all kinds') + '</option>'; }).join('');
      root.innerHTML =
        '<div class="controls"><label>kind <select id="kind">' + options + '</select></label></div>' +
        '<div class="panel"><table><thead><tr><th>time</th><th>kind</th><th>source</th><th>status</th><th class="num">duration</th><th>detail</th></tr></thead><tbody>' + rows + '</tbody></table>' +
        pagerHTML(data.total || 0, logs) + '</div>';
      document.getElementById('kind').value = logs.kind;
      document.getElementById('kind').onchange = function () { logs.kind = this.value; logs.offset = 0; renderLogs(); };
      bindPager(logs, renderLogs);
    }).catch(showError);
  }

  function render() {
    if (tab === 'overview') renderOverview();
    else if (tab === 'recalls') renderRecalls();
    else if (tab === 'sessions') renderSessions();
    else renderLogs();
  }

  document.getElementById('refresh').onclick = render;
  Array.prototype.forEach.call(document.querySelectorAll('nav button'), function (btn) {
    btn.onclick = function () {
      tab = btn.getAttribute('data-tab');
      location.hash = tab;
      Array.prototype.forEach.call(document.querySelectorAll('nav button'), function (b) { b.classList.toggle('active', b === btn); });
      render();
    };
  });
  Array.prototype.forEach.call(document.querySelectorAll('nav button'), function (b) { b.classList.toggle('active', b.getAttribute('data-tab') === tab); });
  render();
})();
</script>
</body>
</html>
`
