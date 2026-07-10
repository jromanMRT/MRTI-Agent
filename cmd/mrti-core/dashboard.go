package main

import "net/http"

func (s *server) dashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboardHTML))
}

const dashboardHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>MRTI Core</title>
<style>
  :root { --bg:#0d1117; --card:#161b22; --border:#30363d; --fg:#e6edf3; --muted:#8b949e;
          --green:#3fb950; --red:#f85149; --amber:#d29922; --accent:#58a6ff; }
  * { box-sizing:border-box; }
  body { margin:0; background:var(--bg); color:var(--fg);
         font:14px/1.5 -apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif; }
  header { padding:16px 24px; border-bottom:1px solid var(--border); display:flex;
           align-items:center; gap:16px; position:sticky; top:0; background:var(--bg); z-index:5; }
  header h1 { font-size:18px; margin:0; } header .sub { color:var(--muted); }
  .pill { margin-left:auto; color:var(--muted); font-size:12px; }
  main { padding:24px; display:grid; grid-template-columns:repeat(auto-fill,minmax(320px,1fr)); gap:16px; }
  .card { background:var(--card); border:1px solid var(--border); border-radius:10px; padding:16px; cursor:pointer; }
  .card:hover { border-color:var(--accent); }
  .card h2 { margin:0 0 4px; font-size:16px; display:flex; align-items:center; gap:8px; }
  .dot { width:9px; height:9px; border-radius:50%; display:inline-block; }
  .on { background:var(--green); } .off { background:var(--red); }
  .meta { color:var(--muted); font-size:12px; margin-bottom:10px; }
  .kv { display:grid; grid-template-columns:auto 1fr; gap:2px 10px; font-size:13px; }
  .kv b { color:var(--muted); font-weight:500; }
  .alerts { margin:0 24px 24px; }
  .alerts h3 { margin:0 0 8px; }
  .alert { background:var(--card); border-left:3px solid var(--amber); border-radius:6px;
           padding:8px 12px; margin-bottom:6px; font-size:13px; }
  .alert.critical { border-left-color:var(--red); }
  .bar { height:6px; background:#21262d; border-radius:4px; overflow:hidden; margin-top:2px; }
  .bar > span { display:block; height:100%; background:var(--accent); }
  .empty { color:var(--muted); padding:40px; text-align:center; grid-column:1/-1; }
  dialog { background:var(--card); color:var(--fg); border:1px solid var(--border);
           border-radius:10px; width:min(900px,92vw); max-height:85vh; padding:0; }
  dialog header { position:static; border:0; }
  dialog pre { margin:0; padding:16px 24px; overflow:auto; max-height:70vh; font-size:12px; }
  dialog .x { margin-left:auto; cursor:pointer; color:var(--muted); background:none; border:0; font-size:18px; }
  a { color:var(--accent); }
</style>
</head>
<body>
<header>
  <h1>MRTI Core</h1>
  <span class="sub">fleet dashboard</span>
  <span class="pill" id="pill">loading…</span>
</header>
<div class="alerts" id="alerts"></div>
<main id="grid"><div class="empty">Waiting for agents…</div></main>

<dialog id="dlg">
  <header><strong id="dlgTitle"></strong><button class="x" onclick="dlg.close()">✕</button></header>
  <pre id="dlgBody"></pre>
</dialog>

<script>
const grid = document.getElementById('grid');
const alertsEl = document.getElementById('alerts');
const pill = document.getElementById('pill');
const dlg = document.getElementById('dlg');

function pct(v){ v = Math.max(0, Math.min(100, +v||0)); return v; }
function bar(v){ return '<div class="bar"><span style="width:'+pct(v)+'%"></span></div>'; }
function ago(ts){ if(!ts) return 'never'; const s=Math.floor(Date.now()/1000-ts);
  if(s<60) return s+'s ago'; if(s<3600) return Math.floor(s/60)+'m ago'; return Math.floor(s/3600)+'h ago'; }

async function jget(u){ const r = await fetch(u); if(!r.ok) throw new Error(r.status); return r.json(); }

async function render(){
  let agents=[], alerts=[];
  try { agents = await jget('/api/v1/agents') || []; alerts = await jget('/api/v1/alerts?limit=20') || []; }
  catch(e){ pill.textContent='Core unreachable'; return; }

  const online = agents.filter(a=>a.online).length;
  pill.textContent = agents.length+' agents · '+online+' online · updated '+new Date().toLocaleTimeString();

  if(!agents.length){ grid.innerHTML='<div class="empty">No agents yet. Point an agent at this server.</div>'; }
  else {
    grid.innerHTML = agents.map(a=>{
      return '<div class="card" onclick="showAgent(\''+a.id+'\',\''+(a.name||a.hostname)+'\')">'+
        '<h2><span class="dot '+(a.online?'on':'off')+'"></span>'+esc(a.hostname||a.id)+'</h2>'+
        '<div class="meta">'+esc(a.os)+'/'+esc(a.arch)+' · v'+esc(a.version)+' · seen '+ago(a.last_seen)+'</div>'+
        '<div class="kv">'+
          '<b>agent id</b><span>'+esc((a.id||'').slice(0,18))+'…</span>'+
          '<b>seq</b><span>#'+(a.sequence||0)+'</span>'+
          '<b>self</b><span>'+(a.self_mem_mb||0)+' MB · '+(a.self_cpu_percent||0)+'% CPU</span>'+
        '</div></div>';
    }).join('');
  }

  alertsEl.innerHTML = alerts.length
    ? '<h3>Recent alerts</h3>'+alerts.map(a=>'<div class="alert '+esc(a.severity)+'"><b>'+esc(a.hostname||a.agent_id)+
        '</b> — ['+esc(a.severity)+'] '+esc(a.rule)+': '+esc(a.message)+'</div>').join('')
    : '';
}

async function showAgent(id, title){
  try {
    const env = await jget('/api/v1/agents/'+id);
    document.getElementById('dlgTitle').textContent = title+'  ·  full latest envelope';
    document.getElementById('dlgBody').textContent = JSON.stringify(env, null, 2);
    dlg.showModal();
  } catch(e){ alert('Failed to load agent: '+e.message); }
}

function esc(s){ return String(s==null?'':s).replace(/[&<>"']/g, c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c])); }

render();
setInterval(render, 5000);
</script>
</body>
</html>`
