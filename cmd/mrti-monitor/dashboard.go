package main

import (
	"net/http"
	"strings"
)

func (s *server) dashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	basePath, _ := r.Context().Value(basePathKey).(string)
	html := strings.ReplaceAll(dashboardHTML, "__AGENT_BASE__", basePath)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

const dashboardHTML = `<!doctype html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="icon" type="image/svg+xml" href="__AGENT_BASE__/portal-assets/favicon.svg" />
<title>MRTI | Agent Core</title>
<style>
  @font-face { font-family:"Big Shoulders Display"; font-style:normal; font-weight:700 800; font-display:swap; src:url("__AGENT_BASE__/portal-assets/big-shoulders-display-800.woff2") format("woff2"); }
  @font-face { font-family:"IBM Plex Sans"; font-style:normal; font-weight:400 600; font-display:swap; src:url("__AGENT_BASE__/portal-assets/ibm-plex-sans-400.woff2") format("woff2"); }
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
  .brand-link { display:flex; width:100%; align-items:center; gap:12px; padding:3px; border:1px solid transparent; border-radius:14px; color:var(--accent-deep); text-decoration:none; transition:border-color 160ms ease,background 160ms ease; }
  .brand-link:hover { border-color:var(--border); background:var(--soft); }
  .brand-mark { display:grid; width:42px; height:42px; flex:0 0 auto; place-items:center; border:1px solid color-mix(in srgb,var(--accent) 30%,transparent); border-radius:12px; background:color-mix(in srgb,var(--accent) 10%,var(--card)); }
  .brand-mark img { width:34px; height:34px; }
  .brand-copy { display:grid; min-width:0; }
  .brand-copy strong { display:flex; align-items:baseline; gap:7px; overflow:hidden; font-family:"Big Shoulders Display",system-ui,sans-serif; font-size:1.15rem; font-weight:800; letter-spacing:.1em; line-height:1.2; text-overflow:ellipsis; text-transform:uppercase; white-space:nowrap; }
  .brand-module { color:var(--accent-deep); font-size:.74em; letter-spacing:.04em; }
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
  .page-footer { display:flex; min-height:72px; align-items:center; gap:12px; padding:0 24px; border-top:1px solid var(--border); color:var(--faint); font-size:.7rem; letter-spacing:.035em; }
  .footer-separator { width:3px; height:3px; border-radius:50%; background:var(--faint); }
  .topbar { position:sticky; z-index:25; top:0; display:flex; min-height:72px; align-items:center; gap:16px; padding:0 24px; border-bottom:1px solid var(--border); background:color-mix(in srgb,var(--bg) 92%,transparent); backdrop-filter:blur(12px); }
  .topbar-context { display:grid; }
  .topbar-context small,.pill { color:var(--faint); font-size:12px; }
  .pill { margin-left:0; }
  .notification-center { position:relative; }
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
  .account-menu { position:relative; }
  .account-trigger { display:flex; align-items:center; gap:9px; padding:0; border:0; color:var(--fg); background:transparent; cursor:pointer; text-align:left; }
  .account-avatar { display:grid; width:34px; height:34px; place-items:center; border:1px solid var(--border); border-radius:50%; color:var(--accent-deep); background:var(--card); font-size:11px; font-weight:800; }
  .account-identity { display:grid; max-width:180px; }.account-identity strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px}.account-identity small{color:var(--faint);font-size:10px;text-transform:capitalize}.account-chevron{color:var(--faint)}
  .account-trigger[aria-expanded="true"] .account-chevron { transform:rotate(180deg); }
  .account-panel { position:absolute; z-index:70; top:calc(100% + 10px); right:0; width:220px; padding:7px; border:1px solid var(--border); border-radius:12px; background:var(--card); box-shadow:0 18px 45px rgba(0,0,0,.22); }
  .account-panel[hidden]{display:none}.account-panel a,.account-panel button{display:flex;width:100%;align-items:center;gap:10px;padding:10px 11px;border:0;border-radius:8px;color:var(--muted);background:transparent;cursor:pointer;font:inherit;font-size:12px;font-weight:650;text-align:left;text-decoration:none}.account-panel a:hover,.account-panel button:hover{color:var(--fg);background:var(--soft)}.account-panel .account-logout{margin-top:5px;border-top:1px solid var(--border);border-radius:0 0 8px 8px;color:var(--red)}
  .mobile-menu,.sidebar-backdrop { display:none; }
  .dashboard-content { width:min(1480px,100%); margin:0 auto; padding:28px 30px 42px; }
  .page-heading { display:flex; align-items:flex-end; justify-content:space-between; gap:24px; margin-bottom:22px; }
  .eyebrow { margin:0 0 5px; color:var(--accent-deep); font-size:10px; font-weight:800; letter-spacing:.18em; text-transform:uppercase; }
  .page-heading h1 { margin:0; font-family:"Big Shoulders Display",system-ui,sans-serif; font-size:clamp(2rem,4vw,3rem); font-weight:800; letter-spacing:.025em; line-height:1; text-transform:uppercase; }
  .page-heading p:last-child { max-width:650px; margin:9px 0 0; color:var(--muted); }
  .pill { margin-left:auto; padding:7px 10px; border:1px solid var(--border); border-radius:999px; background:var(--card); white-space:nowrap; }
  .metrics { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:12px; margin-bottom:22px; }
  .metric { min-width:0; padding:16px 18px; border:1px solid var(--border); border-radius:14px; background:var(--card); box-shadow:0 10px 30px rgba(70,48,13,.035); }
  .metric span { color:var(--muted); font-size:11px; font-weight:700; letter-spacing:.04em; text-transform:uppercase; }
  .metric strong { display:block; margin-top:3px; color:var(--accent-deep); font-family:"Big Shoulders Display",system-ui,sans-serif; font-size:2rem; line-height:1; }
  .metric small { display:block; margin-top:7px; color:var(--faint); font-size:11px; }
  .panel { margin-top:16px; overflow:hidden; border:1px solid var(--border); border-radius:16px; background:color-mix(in srgb,var(--card) 80%,transparent); }
  .panel-heading { display:flex; align-items:center; justify-content:space-between; gap:16px; padding:17px 19px; border-bottom:1px solid var(--border); }
  .panel-heading h2 { margin:0; font-size:16px; }.panel-heading p{margin:3px 0 0;color:var(--faint);font-size:11px}.panel-count{color:var(--accent-deep);font-size:12px;font-weight:800}
  .agent-grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(290px,1fr)); gap:14px; padding:16px; }
  .card { display:block; width:100%; padding:0; overflow:hidden; border:1px solid var(--border); border-radius:14px; color:var(--fg); background:var(--card); cursor:pointer; text-align:left; transition:border-color 150ms ease,transform 150ms ease,box-shadow 150ms ease; }
  .card:hover,.card:focus-visible { border-color:var(--accent); transform:translateY(-2px); box-shadow:0 14px 35px rgba(70,48,13,.10); outline:none; }
  .card-head { display:flex; align-items:flex-start; justify-content:space-between; gap:12px; padding:16px 17px 13px; }
  .agent-title { display:flex; min-width:0; align-items:center; gap:10px; }.agent-title div{min-width:0}.agent-title h3{overflow:hidden;margin:0;text-overflow:ellipsis;white-space:nowrap;font-size:15px}.agent-title small{display:block;margin-top:2px;color:var(--faint);font-size:10px}
  .device-icon { display:grid; width:38px; height:38px; flex:0 0 auto; place-items:center; border-radius:10px; color:var(--accent-deep); background:color-mix(in srgb,var(--accent) 12%,transparent); font-weight:800; }
  .status-badge { display:inline-flex; align-items:center; gap:6px; padding:5px 8px; border-radius:999px; font-size:10px; font-weight:800; }
  .status-badge.online { color:var(--green); background:color-mix(in srgb,var(--green) 12%,transparent); }.status-badge.offline{color:var(--red);background:color-mix(in srgb,var(--red) 10%,transparent)}
  .dot { width:7px; height:7px; border-radius:50%; display:inline-block; background:currentColor; box-shadow:0 0 0 3px color-mix(in srgb,currentColor 12%,transparent); }
  .card-body { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); border-top:1px solid var(--border); }
  .agent-stat { min-width:0; padding:12px 16px; }.agent-stat:nth-child(odd){border-right:1px solid var(--border)}.agent-stat:nth-child(n+3){border-top:1px solid var(--border)}
  .agent-stat b { display:block; overflow:hidden; color:var(--fg); font-size:12px; font-weight:650; text-overflow:ellipsis; white-space:nowrap; }.agent-stat span{display:block;margin-bottom:3px;color:var(--faint);font-size:9px;font-weight:750;letter-spacing:.07em;text-transform:uppercase}
  .alerts { display:grid; gap:8px; padding:16px; }
  .alert { border:1px solid color-mix(in srgb,var(--amber) 30%,var(--border)); border-left:3px solid var(--amber); border-radius:10px;
           padding:10px 12px; background:var(--card); font-size:12px; }
  .alert.critical { border-left-color:var(--red); }
  .bar { height:6px; background:var(--soft); border-radius:4px; overflow:hidden; margin-top:2px; }
  .bar > span { display:block; height:100%; background:var(--accent); }
  .empty { color:var(--muted); padding:42px 20px; text-align:center; grid-column:1/-1; }.empty strong{display:block;margin-bottom:5px;color:var(--fg)}
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
    .module-tabs { max-width:46vw; }
    .topbar-context { display:none; }
    .account-identity { display:none; }
    .sidebar-backdrop { position:fixed; z-index:35; inset:0; border:0; background:rgba(0,0,0,.54); }
    .mobile-open .sidebar-backdrop { display:block; }
    .dashboard-content { padding:20px 16px 32px; }
    .page-heading { align-items:flex-start; flex-direction:column; gap:10px; }
    .metrics { grid-template-columns:repeat(2,minmax(0,1fr)); }
    .agent-grid { grid-template-columns:1fr; padding:12px; }
    .panel-heading { padding:15px; }
  }
  @media(max-width:420px){.metrics{grid-template-columns:1fr 1fr}.metric{padding:14px}.metric strong{font-size:1.65rem}.page-footer{align-items:flex-start;flex-wrap:wrap;padding:18px}.copyright{width:100%}}
  @media(max-width:359px){.pill{display:none}.module-tabs{max-width:38vw}}
/* Navegación global compartida visualmente; copia local por frontend. */
.portal-header-navigation { display: flex; flex: 0 1 auto; align-items: center; gap: 10px; min-width: 0; }
.module-tabs { display: flex; gap: 4px; overflow-x: auto; scrollbar-width: none; }
.module-tabs::-webkit-scrollbar { display: none; }
.module-tab { flex-shrink: 0; padding: 8px 12px; border-radius: 8px; color: var(--muted); background: transparent; font-size: .74rem; font-weight: 700; text-decoration: none; white-space: nowrap; transition: background 120ms ease, color 120ms ease; }
.module-tab:hover { color: var(--fg); background: var(--soft); }
.module-tab.is-current { color: var(--accent-deep); background: color-mix(in srgb, var(--accent) 14%, transparent); font-weight: 800; box-shadow: inset 0 -2px var(--accent); }
.module-tab.is-current:hover { background: color-mix(in srgb, var(--accent) 14%, transparent); }
@media (max-width: 767px) {
  .portal-header-navigation { gap: 6px; }
  .topbar:has(.portal-header-navigation), .portal-module-topbar:has(.portal-header-navigation) { gap: 6px; padding-inline: 10px; }
  .topbar:has(.portal-header-navigation) > span:first-of-type, .portal-module-topbar:has(.portal-header-navigation) .portal-module-context { display: none; }
  .topbar:has(.portal-header-navigation) .mobile-menu-button { margin-right: 0; }
  .portal-module-topbar:has(.portal-header-navigation) .portal-module-actions { margin-left: auto; gap: 6px; }
}

</style>
</head>
<body>
<div class="app-shell" id="appShell">
<button class="sidebar-backdrop" id="sidebarBackdrop" type="button" aria-label="Cerrar navegación"></button>
<aside class="sidebar" aria-label="Navegación de Agent Core">
  <div class="sidebar-brand"><div class="brand-row"><a class="brand-link" id="brandLink" href="/" title="Ir al home" aria-label="Ir al home"><span class="brand-mark"><img id="brandLogo" src="__AGENT_BASE__/portal-assets/company-logo.svg" alt=""></span><span class="brand-copy"><strong><span>MRTI</span><span class="brand-module">Agent Core</span></strong><small>Minera Río Tinto</small></span></a></div></div>
  <div class="sidebar-scroll">
    <nav class="sidebar-nav"><a class="active" href="__AGENT_BASE__/"><span class="nav-icon">⌂</span><span class="nav-label">Agentes</span></a><a href="__AGENT_BASE__/downloads/"><span class="nav-icon">↓</span><span class="nav-label">Descargar agente</span></a></nav>
  </div>
  <div class="sidebar-footer"><button class="sidebar-action" id="themeAction" type="button" title="Cambiar tema">◐</button><button class="sidebar-action collapse-action" id="collapseAction" type="button" title="Colapsar menú">«</button></div>
</aside>
<div class="workspace">
<header class="topbar"><button class="mobile-menu" id="mobileMenu" type="button" aria-label="Abrir navegación">☰</button><nav class="portal-header-navigation module-tabs" id="moduleTabsNav" aria-label="Navegación de la plataforma"><a class="module-tab" id="dashboardLink" href="/dashboard">Dashboard</a><a class="module-tab is-current" aria-current="page">Agent Core</a></nav><span class="topbar-context"><strong>Agent Core</strong><small>Supervisión de agentes</small></span><span class="pill" id="pill">Actualizando…</span><div class="notification-center" id="notificationCenter"><button class="notification-button" id="notificationButton" type="button" aria-label="Ver notificaciones" aria-expanded="false"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4"/></svg><span class="notification-count" id="notificationCount" hidden></span></button><section class="notification-panel" id="notificationPanel" aria-label="Notificaciones" hidden><header><div><small>Novedades</small><strong>Notificaciones</strong></div><button id="notificationClose" type="button" aria-label="Cerrar notificaciones">×</button></header><div class="notification-list" id="notificationList" aria-live="polite"><p>Buscando novedades…</p></div></section></div><div class="account-menu" id="accountMenu"><button class="account-trigger" id="accountTrigger" type="button" aria-label="Abrir menú de usuario" aria-expanded="false"><span class="account-avatar" id="accountAvatar">U</span><span class="account-identity"><strong id="accountName">Usuario</strong><small id="accountRole">Sesión activa</small></span><span class="account-chevron">⌄</span></button><div class="account-panel" id="accountPanel" role="menu" hidden><a href="/" data-portal-view="account" role="menuitem">○ Perfil</a><a href="/" data-portal-view="brand-assets" role="menuitem">◆ Recursos de marca</a><a href="/" data-portal-view="company-news" role="menuitem">▤ Noticias</a><span id="adminAccountLinks"></span><button class="account-logout" id="logoutAction" type="button" role="menuitem">↪ Cerrar sesión</button></div></div></header>
<main class="dashboard-content">
  <section class="page-heading"><div><p class="eyebrow">MRTI Agent Core</p><h1>Supervisión de agentes</h1><p>Estado operativo de los equipos que reportan telemetría, consumo del agente y alertas recientes.</p></div></section>
  <section class="metrics" aria-label="Resumen operativo"><article class="metric"><span>Agentes registrados</span><strong id="metricTotal">—</strong><small>Equipos activos en inventario</small></article><article class="metric"><span>En línea</span><strong id="metricOnline">—</strong><small>Reportando actualmente</small></article><article class="metric"><span>Sin conexión</span><strong id="metricOffline">—</strong><small>Requieren revisión</small></article><article class="metric"><span>Alertas recientes</span><strong id="metricAlerts">—</strong><small>Últimos eventos recibidos</small></article></section>
  <section class="panel"><header class="panel-heading"><div><h2>Agentes monitoreados</h2><p>Selecciona un equipo para consultar su último reporte completo.</p></div><span class="panel-count" id="agentCount">0 equipos</span></header><div class="agent-grid" id="grid"><div class="empty"><strong>Esperando agentes</strong>Los equipos aparecerán aquí cuando envíen telemetría.</div></div></section>
  <section class="panel"><header class="panel-heading"><div><h2>Alertas recientes</h2><p>Señales emitidas por las reglas de supervisión.</p></div><span class="panel-count" id="alertCount">0 alertas</span></header><div class="alerts" id="alerts"><div class="empty"><strong>Sin alertas recientes</strong>La operación se encuentra estable.</div></div></section>
</main>
<footer class="page-footer"><span>MRTI</span><span class="footer-separator"></span><span>La puerta de entrada digital de Minera Río Tinto</span><span class="copyright" id="footerYear"></span></footer>
</div></div>

<dialog id="dlg">
  <header><strong id="dlgTitle"></strong><button class="x" onclick="dlg.close()">✕</button></header>
  <pre id="dlgBody"></pre>
</dialog>

<script>
const AGENT_BASE = '__AGENT_BASE__';
const grid = document.getElementById('grid');
const alertsEl = document.getElementById('alerts');
const pill = document.getElementById('pill');
const dlg = document.getElementById('dlg');
const hashParams = new URLSearchParams(location.hash.slice(1));
const hashToken = hashParams.get('token');
const hashTheme = hashParams.get('theme');
if(hashTheme==='light'||hashTheme==='dark') localStorage.setItem('mrti_theme',hashTheme);
if(hashToken){ sessionStorage.setItem('mrti_portal_token', hashToken); history.replaceState({}, '', location.pathname); }
// Mismo origen que Core cuando se sirve bajo /agent-core/ (vía nginx): la
// sesión de localStorage ya está compartida, sin necesidad del token por
// hash. Ese mecanismo queda solo como respaldo para acceder directo al
// puerto del servicio, fuera de nginx.
const portalToken = localStorage.getItem('auth_token') || sessionStorage.getItem('mrti_portal_token');
const portalOrigin = location.protocol+'//'+location.hostname;
document.getElementById('dashboardLink').href=portalOrigin+'/dashboard';
document.getElementById('brandLink').href = portalOrigin+'/';
document.querySelectorAll('[data-portal-view]').forEach(a=>a.href=portalOrigin+'/?view='+a.dataset.portalView);
if(!portalToken) location.replace(location.protocol+'//'+location.hostname+'/?returnTo='+encodeURIComponent('/agent-core/'));

const appShell = document.getElementById('appShell');
const storedTheme = localStorage.getItem('mrti_theme');
if(storedTheme==='dark'||storedTheme==='light') document.documentElement.dataset.theme=storedTheme;
else if(matchMedia('(prefers-color-scheme: dark)').matches) document.documentElement.dataset.theme='dark';
document.getElementById('footerYear').textContent = '© '+new Date().getFullYear()+' MRTI';
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
const accountMenu=document.getElementById('accountMenu'),accountTrigger=document.getElementById('accountTrigger'),accountPanel=document.getElementById('accountPanel');
const closeAccount=()=>{accountPanel.hidden=true;accountTrigger.setAttribute('aria-expanded','false')};
accountTrigger.addEventListener('click',()=>{const opening=accountPanel.hidden;accountPanel.hidden=!opening;accountTrigger.setAttribute('aria-expanded',String(opening))});
document.addEventListener('mousedown',e=>{if(!accountMenu.contains(e.target))closeAccount()});
document.addEventListener('keydown',e=>{if(e.key==='Escape')closeAccount()});

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
    const moduleIcons={'mrti-legal':'LG',rh:'RH',activos:'AT','agent-core':'AG',tickets:'TK'};
    notificationList.innerHTML=items.length?items.map(item=>'<article class="notification-item"><b>'+esc(moduleIcons[item.module_code]||'MRTI')+'</b><span><strong>'+esc(item.title)+'</strong><small>'+esc(item.message)+'</small><time>'+esc(relativeNotificationTime(item.timestamp))+'</time></span>'+(item.href?'<a href="'+portalOrigin+esc(item.href)+'">Abrir →</a>':'')+'</article>').join(''):'<p>Sin novedades por ahora.</p>';
  }catch(e){notificationList.innerHTML='<p class="error">No fue posible consultar las notificaciones.</p>';}
}
loadNotifications();setInterval(loadNotifications,60000);

// El logo lo administra Core (Centro de control → Recursos de marca); se
// consulta en vivo (endpoint público, sin token) para que un cambio ahí se
// refleje aquí sin tocar código.
const sharesPortalOrigin=!location.port||location.port==='80'||location.port==='443';
if(sharesPortalOrigin){
  fetch(portalOrigin+'/api/portal/v1/brand-appearance',{cache:'no-store'})
    .then(r=>r.ok?r.json():Promise.reject())
    .then(({data})=>{ if(data?.portal_logo?.content_url){ const candidate=new Image(); candidate.onload=()=>{ document.getElementById('brandLogo').src=candidate.src; }; candidate.src=portalOrigin+data.portal_logo.content_url; } })
    .catch(()=>{});
}

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
    document.getElementById('moduleTabsNav').insertAdjacentHTML('beforeend',apps.map(a=>'<a class="module-tab" href="'+portalOrigin+esc(a.url)+'">'+esc(a.name.replace(/^MRTI\s*/i,''))+'</a>').join(''));
  }).catch(()=>{});

fetch(portalOrigin+'/api/auth/me',{headers:{Authorization:'Bearer '+portalToken}}).then(r=>r.ok?r.json():Promise.reject()).then(({profile})=>{const name=profile?.full_name||'Usuario';document.getElementById('accountName').textContent=name;document.getElementById('accountRole').textContent=profile?.role||'Sesión activa';document.getElementById('accountAvatar').textContent=name.split(/\s+/).slice(0,2).map(part=>part[0]).join('').toUpperCase()||'U';if(profile?.role==='administrator')document.getElementById('adminAccountLinks').innerHTML='<a href="'+portalOrigin+'/?view=control-center" role="menuitem">⚙ Centro de control</a>'}).catch(()=>{});

function pct(v){ v = Math.max(0, Math.min(100, +v||0)); return v; }
function bar(v){ return '<div class="bar"><span style="width:'+pct(v)+'%"></span></div>'; }
function ago(ts){ if(!ts) return 'Sin reporte'; const s=Math.max(0,Math.floor(Date.now()/1000-ts));
  if(s<60) return 'Hace '+s+' s'; if(s<3600) return 'Hace '+Math.floor(s/60)+' min'; if(s<86400)return 'Hace '+Math.floor(s/3600)+' h';return 'Hace '+Math.floor(s/86400)+' d'; }

async function jget(u){
  const r = await fetch(u, {headers:{Authorization:'Bearer '+portalToken}});
  if(r.status===401) location.replace(location.protocol+'//'+location.hostname+'/?returnTo='+encodeURIComponent('/agent-core/'));
  if(r.status===403) location.replace(location.protocol+'//'+location.hostname+'/?accessDenied=agent-core');
  if(!r.ok) throw new Error(r.status);
  return r.json();
}

async function render(){
  let agents=[], alerts=[];
  try { agents = await jget(AGENT_BASE+'/api/v1/agents') || []; alerts = await jget(AGENT_BASE+'/api/v1/alerts?limit=20') || []; }
  catch(e){ pill.textContent='Sin conexión'; return; }

  agents = agents.filter(a=>!a.archived);

  const online = agents.filter(a=>a.online).length;
  const offline=agents.length-online;
  pill.textContent = 'Actualizado '+new Date().toLocaleTimeString('es-MX',{hour:'2-digit',minute:'2-digit'});
  document.getElementById('metricTotal').textContent=agents.length;
  document.getElementById('metricOnline').textContent=online;
  document.getElementById('metricOffline').textContent=offline;
  document.getElementById('metricAlerts').textContent=alerts.length;
  document.getElementById('agentCount').textContent=agents.length+' '+(agents.length===1?'equipo':'equipos');
  document.getElementById('alertCount').textContent=alerts.length+' '+(alerts.length===1?'alerta':'alertas');

  if(!agents.length){ grid.innerHTML='<div class="empty"><strong>Sin agentes registrados</strong>Configura un equipo para que reporte a este servidor.</div>'; }
  else {
    grid.innerHTML = agents.map(a=>{
      return '<button class="card" type="button" data-agent-id="'+esc(a.id)+'">'+
        '<span class="card-head"><span class="agent-title"><span class="device-icon">PC</span><span><h3>'+esc(a.hostname||a.name||a.id)+'</h3><small>'+esc(a.os||'Sistema sin identificar')+' · '+esc(a.arch||'Arquitectura desconocida')+'</small></span></span><span class="status-badge '+(a.online?'online':'offline')+'"><span class="dot"></span>'+(a.online?'En línea':'Sin conexión')+'</span></span>'+
        '<span class="card-body">'+
          '<span class="agent-stat"><span>Último reporte</span><b>'+esc(ago(a.last_seen))+'</b></span>'+
          '<span class="agent-stat"><span>Versión</span><b>'+esc(a.version||'—')+'</b></span>'+
          '<span class="agent-stat"><span>Memoria del agente</span><b>'+esc(a.self_mem_mb||0)+' MB</b></span>'+
          '<span class="agent-stat"><span>CPU del agente</span><b>'+esc(a.self_cpu_percent||0)+'%</b></span>'+
        '</span></button>';
    }).join('');
  }

  alertsEl.innerHTML = alerts.length
    ? alerts.map(a=>'<div class="alert '+esc(a.severity)+'"><b>'+esc(a.hostname||a.agent_id)+
        '</b> · '+esc(a.rule)+'<div>'+esc(a.message)+'</div></div>').join('')
    : '<div class="empty"><strong>Sin alertas recientes</strong>La operación se encuentra estable.</div>';
}

grid.addEventListener('click',event=>{const card=event.target.closest('[data-agent-id]');if(!card)return;const id=card.dataset.agentId;const title=card.querySelector('h3')?.textContent||id;showAgent(id,title);});

async function showAgent(id, title){
  try {
    const env = await jget(AGENT_BASE+'/api/v1/agents/'+id);
    document.getElementById('dlgTitle').textContent = title+' · Último reporte completo';
    document.getElementById('dlgBody').textContent = JSON.stringify(env, null, 2);
    dlg.showModal();
  } catch(e){ alert('No fue posible consultar el agente: '+e.message); }
}

function esc(s){ return String(s==null?'':s).replace(/[&<>"']/g, c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c])); }

render();
setInterval(render, 5000);
</script>
</body>
</html>`
