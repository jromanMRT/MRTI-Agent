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
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="icon" type="image/svg+xml" href="data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCA2NCA2NCI+CiAgPGRlZnM+CiAgICA8bGluZWFyR3JhZGllbnQgaWQ9ImciIHgxPSI4IiB5MT0iOCIgeDI9IjU2IiB5Mj0iNTYiIGdyYWRpZW50VW5pdHM9InVzZXJTcGFjZU9uVXNlIj4KICAgICAgPHN0b3Agc3RvcC1jb2xvcj0iIzU1YmRmNiIvPgogICAgICA8c3RvcCBvZmZzZXQ9IjEiIHN0b3AtY29sb3I9IiM1MGQ0YmQiLz4KICAgIDwvbGluZWFyR3JhZGllbnQ+CiAgPC9kZWZzPgogIDxyZWN0IHdpZHRoPSI2NCIgaGVpZ2h0PSI2NCIgcng9IjE2IiBmaWxsPSIjMGIxYTI5Ii8+CiAgPHBhdGggZD0iTTEzIDIwIDMyIDEwbDE5IDEwdjI1TDMyIDU1IDEzIDQ1VjIwWiIgZmlsbD0ibm9uZSIgc3Ryb2tlPSJ1cmwoI2cpIiBzdHJva2Utd2lkdGg9IjQiIHN0cm9rZS1saW5lam9pbj0icm91bmQiLz4KICA8cGF0aCBkPSJtMjMgMjUgOS01IDkgNXYxNWwtOSA1LTktNVYyNVoiIGZpbGw9Im5vbmUiIHN0cm9rZT0idXJsKCNnKSIgc3Ryb2tlLXdpZHRoPSI0IiBzdHJva2UtbGluZWpvaW49InJvdW5kIi8+Cjwvc3ZnPgo=" />
<title>MRTI Agent Core</title>
<style>
  :root { --bg:#f6f2e7; --card:#fff; --soft:#f1ead9; --border:#e1d3ab; --fg:#221b12; --muted:#6c5f47;
          --faint:#705f3f; --green:#2f7d43; --red:#a1432f; --amber:#744705; --accent:#754e0d; --accent-deep:#6e4d16; }
  :root[data-theme="dark"] { --bg:#17120c; --card:#251e13; --soft:#2f2517; --border:#4a3a22; --fg:#f5ecd9;
          --muted:#d7cab1; --faint:#a8956f; --green:#6fcf8c; --red:#e08a72; --amber:#f3bf5e; --accent:#d9a63c; --accent-deep:#caa054; }
  * { box-sizing:border-box; }
  body { margin:0; background:var(--bg); color:var(--fg);
         font:14px/1.5 "IBM Plex Sans",-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif; }
  button,a { font:inherit; }
  .app-shell { display:grid; grid-template-columns:256px minmax(0,1fr); min-height:100vh; transition:grid-template-columns 200ms ease; }
  .app-shell.collapsed { grid-template-columns:64px minmax(0,1fr); }
  .sidebar { position:sticky; z-index:40; top:0; display:flex; height:100vh; padding:18px 14px; flex-direction:column;
             overflow:hidden; border-right:1px solid var(--border); background:linear-gradient(180deg,rgba(217,166,60,.055),transparent 13rem),var(--card); }
  .collapsed .sidebar { padding-inline:10px; }
  .sidebar-brand { min-height:61px; padding:0 4px 18px; border-bottom:1px solid var(--border); }
  .brand-row { display:flex; min-width:0; align-items:center; gap:12px; }
  .brand-link { display:grid; flex:0 0 auto; color:var(--accent-deep); text-decoration:none; }
  .brand-mark { display:grid; width:42px; height:42px; flex:0 0 auto; place-items:center; border:1px solid color-mix(in srgb,var(--accent) 30%,transparent); border-radius:12px; background:color-mix(in srgb,var(--accent) 10%,var(--card)); }
  .brand-mark img { width:34px; height:34px; }
  .brand-copy { display:grid; min-width:0; }
  .brand-copy strong { overflow:hidden; font-family:"Big Shoulders Display",system-ui,sans-serif; font-size:1.15rem; font-weight:800; letter-spacing:.1em; line-height:1.2; text-overflow:ellipsis; text-transform:uppercase; white-space:nowrap; }
  .brand-copy small { overflow:hidden; max-width:150px; color:var(--faint); font-size:.78rem; font-weight:600; letter-spacing:.08em; text-overflow:ellipsis; text-transform:uppercase; white-space:nowrap; }
  .collapsed .brand-copy,.collapsed .nav-label,.collapsed .section-label { display:none; }
  .collapsed .brand-row,.collapsed .sidebar-nav a,.collapsed .sidebar-section a { justify-content:center; }
  .sidebar-nav { display:grid; gap:4px; padding-top:14px; }
  .sidebar-nav a,.sidebar-section a { display:flex; min-height:42px; align-items:center; gap:11px; padding:9px 11px; border-radius:11px; color:var(--muted); text-decoration:none; }
  .sidebar-nav a:hover,.sidebar-section a:hover { color:var(--fg); background:var(--soft); }
  .sidebar-nav a.active { color:var(--fg); background:color-mix(in srgb,var(--accent) 16%,transparent); box-shadow:inset 2px 0 var(--accent); }
  .nav-icon { display:grid; width:20px; flex:0 0 20px; place-items:center; color:var(--accent-deep); }
  .sidebar-scroll { min-height:0; flex:1; overflow-y:auto; scrollbar-width:thin; }
  .sidebar-section { display:grid; gap:4px; margin-top:18px; padding-top:14px; border-top:1px solid var(--border); }
  .section-label { padding:0 10px 7px; color:var(--faint); font-size:10px; font-weight:700; letter-spacing:.12em; text-transform:uppercase; }
  .sidebar-footer { display:flex; align-items:center; justify-content:space-between; gap:7px; padding-top:14px; border-top:1px solid var(--border); }
  .collapsed .sidebar-footer { flex-direction:column; }
  .sidebar-action { display:grid; width:36px; height:36px; flex:0 0 36px; place-items:center; border:1px solid var(--border); border-radius:50%; color:var(--accent-deep); background:var(--card); cursor:pointer; }
  .sidebar-action:hover { border-color:var(--accent); color:var(--accent); background:var(--soft); }
  .sidebar-action.logout:hover { color:var(--red); }
  .workspace { min-width:0; }
  .topbar { position:sticky; z-index:20; top:0; display:flex; min-height:72px; align-items:center; gap:16px; padding:0 24px; border-bottom:1px solid var(--border); background:color-mix(in srgb,var(--bg) 92%,transparent); backdrop-filter:blur(12px); }
  .topbar-context { display:grid; }
  .topbar-context small,.pill { color:var(--faint); font-size:12px; }
  .pill { margin-left:0; }
  .notification-center { position:relative; margin-left:auto; }
  .notification-button { position:relative; display:grid; width:38px; height:38px; place-items:center; border:1px solid var(--border); border-radius:50%; color:var(--accent-deep); background:var(--card); cursor:pointer; }
  .notification-button:hover,.notification-button[aria-expanded="true"] { border-color:var(--accent); color:var(--accent); background:var(--soft); }
  .notification-button svg { width:19px; fill:none; stroke:currentColor; stroke-width:1.8; stroke-linecap:round; stroke-linejoin:round; }
  .notification-count { position:absolute; top:-6px; right:-7px; display:grid; min-width:19px; height:19px; place-items:center; padding:0 4px; border:2px solid var(--bg); border-radius:99px; color:#fff; background:#a3261b; font-size:10px; font-weight:800; }
  .notification-count[hidden],.notification-panel[hidden] { display:none; }
  .notification-panel { position:absolute; z-index:70; top:calc(100% + 10px); right:0; width:min(410px,calc(100vw - 28px)); overflow:hidden; border:1px solid var(--border); border-radius:16px; background:var(--card); box-shadow:0 20px 50px rgba(0,0,0,.28); }
  .notification-panel header { display:flex; align-items:center; justify-content:space-between; padding:14px 16px; border-bottom:1px solid var(--border); }
  .notification-panel header div { display:grid; }
  .notification-panel header small { color:var(--accent-deep); font-size:10px; font-weight:800; letter-spacing:.12em; text-transform:uppercase; }
  .notification-panel header button { display:grid; width:30px; height:30px; place-items:center; border:1px solid var(--border); border-radius:50%; color:var(--faint); background:var(--soft); cursor:pointer; }
  .notification-list { max-height:min(30rem,70vh); overflow:auto; }
  .notification-list > p { margin:0; padding:34px 16px; color:var(--faint); text-align:center; }
  .notification-list > p.error { color:var(--red); background:color-mix(in srgb,var(--red) 8%,transparent); }
  .notification-item { display:flex; align-items:flex-start; gap:11px; padding:13px 16px; border-bottom:1px solid var(--border); }
  .notification-item:last-child { border:0; }
  .notification-item > b { display:grid; width:36px; height:36px; flex:0 0 auto; place-items:center; border-radius:9px; color:var(--accent-deep); background:color-mix(in srgb,var(--accent) 12%,transparent); font-size:10px; }
  .notification-item > span { display:grid; min-width:0; flex:1; gap:3px; }
  .notification-item strong { font-size:13px; }
  .notification-item small { color:var(--muted); font-size:11px; }
  .notification-item time { color:var(--faint); font-size:10px; }
  .notification-item > a { align-self:center; color:var(--accent-deep); font-size:11px; font-weight:750; white-space:nowrap; }
  .mobile-menu,.sidebar-backdrop { display:none; }
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
  .bar { height:6px; background:var(--soft); border-radius:4px; overflow:hidden; margin-top:2px; }
  .bar > span { display:block; height:100%; background:var(--accent); }
  .empty { color:var(--muted); padding:40px; text-align:center; grid-column:1/-1; }
  dialog { background:var(--card); color:var(--fg); border:1px solid var(--border);
           border-radius:10px; width:min(900px,92vw); max-height:85vh; padding:0; }
  dialog header { position:static; display:flex; align-items:center; padding:16px 24px; border:0; }
  dialog pre { margin:0; padding:16px 24px; overflow:auto; max-height:70vh; font-size:12px; }
  dialog .x { margin-left:auto; cursor:pointer; color:var(--muted); background:none; border:0; font-size:18px; }
  a { color:var(--accent); }
  @media(max-width:820px){
    .app-shell,.app-shell.collapsed { display:block; }
    .sidebar,.collapsed .sidebar { position:fixed; width:min(280px,calc(100vw - 42px)); padding:18px 14px; transform:translateX(-105%); transition:transform 200ms ease; }
    .mobile-open .sidebar { transform:translateX(0); box-shadow:0 24px 70px rgba(0,0,0,.32); }
    .collapsed .brand-copy,.collapsed .nav-label,.collapsed .section-label { display:initial; }
    .collapsed .brand-row,.collapsed .sidebar-nav a,.collapsed .sidebar-section a { justify-content:flex-start; }
    .collapsed .sidebar-footer { flex-direction:row; }
    .collapse-action { display:none; }
    .mobile-menu { display:grid; width:38px; height:38px; place-items:center; border:1px solid var(--border); border-radius:10px; color:var(--accent-deep); background:var(--card); }
    .sidebar-backdrop { position:fixed; z-index:35; inset:0; border:0; background:rgba(0,0,0,.54); }
    .mobile-open .sidebar-backdrop { display:block; }
    main { padding:18px; grid-template-columns:1fr; }
    .alerts { margin-inline:18px; }
  }
</style>
</head>
<body>
<div class="app-shell" id="appShell">
<button class="sidebar-backdrop" id="sidebarBackdrop" type="button" aria-label="Cerrar navegación"></button>
<aside class="sidebar" aria-label="Navegación de Agent Core">
  <div class="sidebar-brand"><div class="brand-row"><a class="brand-link" id="brandLink" href="/" title="Ir a Mi espacio" aria-label="Ir a Mi espacio"><span class="brand-mark"><img id="brandLogo" src="/company-logo.svg" alt=""></span></a><span class="brand-copy"><strong>MRTI Agent Core</strong><small>Minera Río Tinto</small></span></div></div>
  <div class="sidebar-scroll">
    <nav class="sidebar-nav"><a class="active" href="/"><span class="nav-icon">⌂</span><span class="nav-label">Agentes</span></a><a href="/downloads/"><span class="nav-icon">↓</span><span class="nav-label">Descargar agente</span></a></nav>
    <div class="sidebar-section"><span class="section-label">Cambiar módulo</span><div id="appMenu"><a href="/"><span class="nav-icon">⌂</span><span class="nav-label">Mi espacio</span></a></div></div>
    <div class="sidebar-section" id="accountMenu"><span class="section-label">Mi cuenta</span><a href="/" data-portal-view="account"><span class="nav-icon">○</span><span class="nav-label">Perfil</span></a></div>
  </div>
  <div class="sidebar-footer"><button class="sidebar-action" id="themeAction" type="button" title="Cambiar tema">◐</button><button class="sidebar-action collapse-action" id="collapseAction" type="button" title="Colapsar menú">«</button><button class="sidebar-action logout" id="logoutAction" type="button" title="Cerrar sesión">↪</button></div>
</aside>
<div class="workspace">
<header class="topbar"><button class="mobile-menu" id="mobileMenu" type="button" aria-label="Abrir navegación">☰</button><span class="topbar-context"><strong>Agent Core</strong><small>Supervisión de agentes</small></span><div class="notification-center" id="notificationCenter"><button class="notification-button" id="notificationButton" type="button" aria-label="Ver notificaciones" aria-expanded="false"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4"/></svg><span class="notification-count" id="notificationCount" hidden></span></button><section class="notification-panel" id="notificationPanel" aria-label="Notificaciones" hidden><header><div><small>Novedades</small><strong>Notificaciones</strong></div><button id="notificationClose" type="button" aria-label="Cerrar notificaciones">×</button></header><div class="notification-list" id="notificationList" aria-live="polite"><p>Buscando novedades…</p></div></section></div><span class="pill" id="pill">Cargando…</span></header>
<div class="alerts" id="alerts"></div>
<main id="grid"><div class="empty">Esperando agentes…</div></main>
</div></div>

<dialog id="dlg">
  <header><strong id="dlgTitle"></strong><button class="x" onclick="dlg.close()">✕</button></header>
  <pre id="dlgBody"></pre>
</dialog>

<script>
const grid = document.getElementById('grid');
const alertsEl = document.getElementById('alerts');
const pill = document.getElementById('pill');
const dlg = document.getElementById('dlg');
const hashParams = new URLSearchParams(location.hash.slice(1));
const hashToken = hashParams.get('token');
const hashTheme = hashParams.get('theme');
if(hashTheme==='light'||hashTheme==='dark') localStorage.setItem('mrti_theme',hashTheme);
if(hashToken){ sessionStorage.setItem('mrti_portal_token', hashToken); history.replaceState({}, '', '/'); }
const portalToken = sessionStorage.getItem('mrti_portal_token');
const portalOrigin = location.protocol+'//'+location.hostname;
const sharedFontStyle = document.createElement('style');
sharedFontStyle.textContent = '@font-face{font-family:"Big Shoulders Display";font-style:normal;font-weight:700 800;font-display:swap;src:url("'+portalOrigin+'/fonts/big-shoulders-display-800.woff2") format("woff2")}@font-face{font-family:"IBM Plex Sans";font-style:normal;font-weight:400 600;font-display:swap;src:url("'+portalOrigin+'/fonts/ibm-plex-sans-400.woff2") format("woff2")}';
document.head.append(sharedFontStyle);
document.getElementById('brandLink').href = portalOrigin+'/';
document.querySelector('#appMenu a').href = portalOrigin+'/';
document.querySelectorAll('[data-portal-view]').forEach(a=>a.href=portalOrigin+'/?view='+a.dataset.portalView);
if(!portalToken) location.replace(location.protocol+'//'+location.hostname+'/?returnTo='+encodeURIComponent('/agent-core/'));

const appShell = document.getElementById('appShell');
const storedTheme = localStorage.getItem('mrti_theme');
if(storedTheme==='dark'||storedTheme==='light') document.documentElement.dataset.theme=storedTheme;
else if(matchMedia('(prefers-color-scheme: dark)').matches) document.documentElement.dataset.theme='dark';
if(localStorage.getItem('mrti_agent_sidebar_collapsed')==='1') appShell.classList.add('collapsed');
document.getElementById('collapseAction').addEventListener('click',()=>{
  const collapsed=appShell.classList.toggle('collapsed');
  localStorage.setItem('mrti_agent_sidebar_collapsed',collapsed?'1':'0');
  document.getElementById('collapseAction').textContent=collapsed?'»':'«';
});
document.getElementById('themeAction').addEventListener('click',()=>{
  const next=document.documentElement.dataset.theme==='dark'?'light':'dark';
  document.documentElement.dataset.theme=next; localStorage.setItem('mrti_theme',next);
  fetch(portalOrigin+'/api/auth/profile/preferences/theme',{method:'PATCH',headers:{'Content-Type':'application/json',Authorization:'Bearer '+portalToken},body:JSON.stringify({theme:next})}).catch(()=>{});
});
const closeMobile=()=>{appShell.classList.remove('mobile-open');document.body.style.overflow='';};
document.getElementById('mobileMenu').addEventListener('click',()=>{appShell.classList.add('mobile-open');document.body.style.overflow='hidden';});
document.getElementById('sidebarBackdrop').addEventListener('click',closeMobile);
document.addEventListener('keydown',e=>{if(e.key==='Escape')closeMobile();});
document.querySelector('.sidebar').addEventListener('click',e=>{if(e.target.closest('a'))closeMobile();});
document.getElementById('logoutAction').addEventListener('click',async()=>{
  try{await fetch(portalOrigin+'/api/auth/logout',{method:'POST',headers:{'Content-Type':'application/json',Authorization:'Bearer '+portalToken},body:'{}'});}catch(e){}
  sessionStorage.removeItem('mrti_portal_token'); localStorage.removeItem('auth_token'); localStorage.removeItem('auth_profile'); location.replace(portalOrigin+'/');
});

const notificationCenter=document.getElementById('notificationCenter');
const notificationButton=document.getElementById('notificationButton');
const notificationPanel=document.getElementById('notificationPanel');
const notificationList=document.getElementById('notificationList');
const notificationCount=document.getElementById('notificationCount');
function closeNotifications(){notificationPanel.hidden=true;notificationButton.setAttribute('aria-expanded','false');}
notificationButton.addEventListener('click',()=>{const opening=notificationPanel.hidden;notificationPanel.hidden=!opening;notificationButton.setAttribute('aria-expanded',String(opening));if(opening)loadNotifications();});
document.getElementById('notificationClose').addEventListener('click',closeNotifications);
document.addEventListener('mousedown',e=>{if(!notificationCenter.contains(e.target))closeNotifications();});
document.addEventListener('keydown',e=>{if(e.key==='Escape')closeNotifications();});
function relativeNotificationTime(value){if(!value)return '';const seconds=Math.max(0,Math.floor((Date.now()-new Date(value).getTime())/1000));if(seconds<60)return 'Ahora';if(seconds<3600)return 'Hace '+Math.floor(seconds/60)+' min';if(seconds<86400)return 'Hace '+Math.floor(seconds/3600)+' h';return 'Hace '+Math.floor(seconds/86400)+' d';}
async function loadNotifications(){
  try{
    const response=await fetch(portalOrigin+'/api/portal/v1/notifications',{headers:{Authorization:'Bearer '+portalToken}});
    if(!response.ok)throw new Error(response.status);
    const body=await response.json();const items=Array.isArray(body.data)?body.data:[];
    notificationCount.hidden=!items.length;notificationCount.textContent=items.length>9?'9+':String(items.length);
    notificationButton.setAttribute('aria-label',items.length?'Ver '+items.length+' notificaciones':'Ver notificaciones');
    notificationList.innerHTML=items.length?items.map(item=>'<article class="notification-item"><b>TK</b><span><strong>'+esc(item.title)+'</strong><small>'+esc(item.message)+'</small><time>'+esc(relativeNotificationTime(item.timestamp))+'</time></span>'+(item.href?'<a href="'+portalOrigin+esc(item.href)+'">Abrir →</a>':'')+'</article>').join(''):'<p>Sin novedades por ahora.</p>';
  }catch(e){notificationList.innerHTML='<p class="error">No fue posible consultar las notificaciones.</p>';}
}
loadNotifications();setInterval(loadNotifications,60000);

// El logo lo administra Core (Centro de control → Recursos de marca); se
// consulta en vivo (endpoint público, sin token) para que un cambio ahí se
// refleje aquí sin tocar código.
fetch(portalOrigin+'/api/portal/v1/brand-appearance',{cache:'no-store'})
  .then(r=>r.ok?r.json():Promise.reject())
  .then(({data})=>{ if(data?.portal_logo?.content_url) document.getElementById('brandLogo').src = portalOrigin+data.portal_logo.content_url; })
  .catch(()=>{});

// El puerto 8477 tiene un localStorage distinto al portal. La preferencia
// central del usuario es la autoridad y mantiene Agent/Descargas en el mismo
// tema aunque se entre directamente sin pasar por un enlace del sidebar.
fetch(portalOrigin+'/api/auth/profile/preferences',{headers:{Authorization:'Bearer '+portalToken}})
  .then(r=>r.ok?r.json():Promise.reject())
  .then(({preferences})=>{
    const theme=preferences?.theme;
    if(theme==='light'||theme==='dark') localStorage.setItem('mrti_theme',theme);
    else localStorage.removeItem('mrti_theme');
    document.documentElement.dataset.theme=theme==='light'||theme==='dark'?theme:(matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light');
  }).catch(()=>{});

fetch(portalOrigin+'/api/portal/v1/applications',{headers:{Authorization:'Bearer '+portalToken}})
  .then(r=>r.ok?r.json():Promise.reject())
  .then(({data})=>{
    const apps=(Array.isArray(data)?data:[]).filter(a=>a.code!=='agent-core');
    document.getElementById('appMenu').innerHTML='<a href="'+portalOrigin+'/"><span class="nav-icon">⌂</span><span class="nav-label">Mi espacio</span></a>'+apps.map(a=>'<a href="'+portalOrigin+esc(a.url)+'"><span class="nav-icon">◆</span><span class="nav-label">'+esc(a.name)+'</span></a>').join('');
  }).catch(()=>{});

try{
  const profile=JSON.parse(localStorage.getItem('auth_profile')||'{}');
  if(profile.role==='administrator') document.getElementById('accountMenu').insertAdjacentHTML('beforeend','<a href="'+portalOrigin+'/?view=brand-assets"><span class="nav-icon">◆</span><span class="nav-label">Recursos de marca</span></a><a href="'+portalOrigin+'/?view=control-center"><span class="nav-icon">⚙</span><span class="nav-label">Centro de control</span></a>');
}catch(e){}

function pct(v){ v = Math.max(0, Math.min(100, +v||0)); return v; }
function bar(v){ return '<div class="bar"><span style="width:'+pct(v)+'%"></span></div>'; }
function ago(ts){ if(!ts) return 'never'; const s=Math.floor(Date.now()/1000-ts);
  if(s<60) return s+'s ago'; if(s<3600) return Math.floor(s/60)+'m ago'; return Math.floor(s/3600)+'h ago'; }

async function jget(u){
  const r = await fetch(u, {headers:{Authorization:'Bearer '+portalToken}});
  if(r.status===401) location.replace(location.protocol+'//'+location.hostname+'/?returnTo='+encodeURIComponent('/agent-core/'));
  if(r.status===403) location.replace(location.protocol+'//'+location.hostname+'/?accessDenied=agent-core');
  if(!r.ok) throw new Error(r.status);
  return r.json();
}

async function render(){
  let agents=[], alerts=[];
  try { agents = await jget('/api/v1/agents') || []; alerts = await jget('/api/v1/alerts?limit=20') || []; }
  catch(e){ pill.textContent='Core unreachable'; return; }

  agents = agents.filter(a=>!a.archived);

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
