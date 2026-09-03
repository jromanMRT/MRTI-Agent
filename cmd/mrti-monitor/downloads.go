package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type downloadArtifact struct {
	OS           string
	Architecture string
	Filename     string
	Icon         string
	Available    bool
	Size         string
}

var downloadCatalog = []downloadArtifact{
	{OS: "Windows", Architecture: "64 bits (Intel/AMD)", Filename: "mrti-agent-windows-amd64.zip", Icon: "⊞"},
	{OS: "Linux", Architecture: "64 bits (Intel/AMD)", Filename: "mrti-agent-linux-amd64.tar.gz", Icon: "◆"},
	{OS: "Linux", Architecture: "ARM64", Filename: "mrti-agent-linux-arm64.tar.gz", Icon: "◆"},
	{OS: "macOS", Architecture: "Apple Silicon", Filename: "mrti-agent-macos-arm64.tar.gz", Icon: "●"},
	{OS: "macOS", Architecture: "Intel", Filename: "mrti-agent-macos-amd64.tar.gz", Icon: "●"},
}

func (s *server) downloadsPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/downloads/" {
		http.NotFound(w, r)
		return
	}
	artifacts := make([]downloadArtifact, len(downloadCatalog))
	copy(artifacts, downloadCatalog)
	for i := range artifacts {
		if info, err := os.Stat(filepath.Join(s.downloadsDir, artifacts[i].Filename)); err == nil && !info.IsDir() {
			artifacts[i].Available = true
			artifacts[i].Size = humanSize(info.Size())
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := downloadsTemplate.Execute(w, map[string]any{"Artifacts": artifacts}); err != nil {
		http.Error(w, "could not render downloads page", http.StatusInternalServerError)
	}
}

func (s *server) downloadFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, `/\\`) || !knownDownload(name) {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(s.downloadsDir, name)
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, name))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, name, info.ModTime(), f)
}

func knownDownload(name string) bool {
	for _, artifact := range downloadCatalog {
		if artifact.Filename == name {
			return true
		}
	}
	return false
}

func humanSize(bytes int64) string {
	const mb = 1024 * 1024
	if bytes >= mb {
		return fmt.Sprintf("%.1f MB", float64(bytes)/mb)
	}
	return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
}

var downloadsTemplate = template.Must(template.New("downloads").Parse(`<!doctype html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Descargar MRTI Agent</title>
<style>
  @font-face { font-family:"Big Shoulders Display"; font-style:normal; font-weight:700 800; font-display:swap; src:url("/portal-assets/big-shoulders-display-800.woff2") format("woff2"); }
  @font-face { font-family:"IBM Plex Sans"; font-style:normal; font-weight:400 600; font-display:swap; src:url("/portal-assets/ibm-plex-sans-400.woff2") format("woff2"); }
  :root { --bg:#f6f2e7; --card:#fff; --border:#e1d3ab; --fg:#221b12;
          --muted:#6c5f47; --accent:#6e4d16; --accent2:#a9781f; color-scheme:light; }
  :root[data-theme=dark] { --bg:#17120c; --card:#251e13; --border:#4a3a22; --fg:#f5ecd9; --muted:#d7cab1; --accent:#d9a63c; --accent2:#f3d68d; color-scheme:dark; }
  @media(prefers-color-scheme:dark){:root:not([data-theme=light]){--bg:#17120c;--card:#251e13;--border:#4a3a22;--fg:#f5ecd9;--muted:#d7cab1;--accent:#d9a63c;--accent2:#f3d68d;color-scheme:dark}}
  * { box-sizing:border-box; }
  body { margin:0; min-height:100vh; color:var(--fg); background:
    radial-gradient(circle at 80% 5%,color-mix(in srgb,var(--accent) 12%,transparent) 0,transparent 30%),var(--bg);
    font:15px/1.5 "IBM Plex Sans",-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif; }
  .shell { display:grid; grid-template-columns:256px minmax(0,1fr); min-height:100vh; }
  .sidebar { position:sticky; z-index:40; top:0; display:flex; height:100vh; padding:18px 14px; flex-direction:column; border-right:1px solid var(--border); background:var(--card); }
  .brand { display:flex; align-items:center; gap:12px; min-height:61px; padding:0 4px 18px; border-bottom:1px solid var(--border); }
  .brand-mark { display:grid; width:42px; height:42px; flex:0 0 auto; place-items:center; border:1px solid color-mix(in srgb,var(--accent) 30%,transparent); border-radius:12px; color:var(--accent); background:color-mix(in srgb,var(--accent) 10%,var(--card)); text-decoration:none; }
  .brand-mark img { width:34px; height:34px; }
  .brand-copy { display:grid; min-width:0; }
  .brand-copy strong { overflow:hidden; font-family:"Big Shoulders Display",system-ui,sans-serif; font-size:1.15rem; font-weight:800; letter-spacing:.1em; line-height:1.2; text-overflow:ellipsis; text-transform:uppercase; white-space:nowrap; }
  .brand-copy small { overflow:hidden; max-width:150px; color:var(--muted); font-size:.78rem; font-weight:600; letter-spacing:.08em; text-overflow:ellipsis; text-transform:uppercase; white-space:nowrap; }
  .topbar small { display:block; color:var(--muted); font-size:11px; }
  .sidebar nav { display:grid; gap:4px; padding-top:14px; }
  .sidebar nav a,.account-link { display:flex; min-height:42px; align-items:center; gap:11px; padding:9px 11px; border-radius:11px; color:var(--muted); text-decoration:none; }
  .sidebar nav a:hover,.sidebar nav a.active,.account-link:hover { color:var(--fg); background:color-mix(in srgb,var(--accent) 13%,var(--card)); }
  .sidebar-section { display:grid; gap:4px; margin-top:18px; padding-top:14px; border-top:1px solid var(--border); }
  .section-label { padding:0 10px 7px; color:var(--muted); font-size:10px; letter-spacing:.12em; text-transform:uppercase; }
  .sidebar-footer { display:flex; gap:7px; margin-top:auto; padding-top:14px; border-top:1px solid var(--border); }
  .sidebar-action,.mobile-menu { display:grid; width:36px; height:36px; place-items:center; border:1px solid var(--border); border-radius:50%; color:var(--accent); background:var(--card); cursor:pointer; }
  .workspace { min-width:0; }
  .topbar { position:sticky; z-index:20; top:0; display:flex; min-height:72px; align-items:center; gap:16px; padding:0 24px; border-bottom:1px solid var(--border); background:color-mix(in srgb,var(--bg) 92%,transparent); backdrop-filter:blur(12px); }
  .topbar-context { display:grid; margin-right:auto; }
  .header-module-switcher { display:grid; flex:0 0 auto; gap:2px; }
  .header-module-switcher > span { color:var(--muted); font-size:9px; font-weight:700; letter-spacing:.08em; text-transform:uppercase; }
  .header-module-switcher select { min-width:150px; max-width:210px; height:34px; padding:0 30px 0 10px; border:1px solid var(--border); border-radius:9px; color:var(--fg); background:var(--card); font-size:12px; font-weight:700; cursor:pointer; }
  .notification-center { position:relative; }
  .notification-button { position:relative; display:grid; width:38px; height:38px; place-items:center; border:1px solid var(--border); border-radius:50%; color:var(--accent); background:var(--card); cursor:pointer; }
  .notification-count { position:absolute; top:-6px; right:-7px; min-width:19px; height:19px; padding:2px 4px; border:2px solid var(--bg); border-radius:999px; color:#fff; background:#a3261b; font-size:10px; font-weight:800; }
  .notification-panel { position:absolute; z-index:60; top:calc(100% + 10px); right:0; width:min(410px,calc(100vw - 28px)); overflow:hidden; border:1px solid var(--border); border-radius:16px; color:var(--fg); background:var(--card); box-shadow:0 20px 50px rgba(0,0,0,.28); }
  .notification-panel[hidden] { display:none; }
  .notification-panel header { display:flex; align-items:center; justify-content:space-between; padding:14px 16px; border-bottom:1px solid var(--border); }
  .notification-panel header button { width:30px; height:30px; border:1px solid var(--border); border-radius:50%; color:var(--muted); background:var(--card); cursor:pointer; }
  .notification-list { max-height:min(30rem,70vh); overflow-y:auto; }
  .notification-list p,.notification-list article { margin:0; padding:13px 16px; border-bottom:1px solid var(--border); color:var(--muted); }
  .notification-list article { display:grid; gap:3px; }.notification-list article strong{color:var(--fg)}.notification-list article a{color:var(--accent);font-size:12px;font-weight:700}
  .mobile-menu,.backdrop { display:none; }
  .hero { max-width:820px; margin:50px auto 36px; padding:0 24px; text-align:center; }
  .eyebrow { color:var(--accent2); letter-spacing:.12em; text-transform:uppercase; font-size:12px; font-weight:700; }
  h1 { margin:12px 0; font-size:clamp(36px,7vw,64px); line-height:1.05; letter-spacing:-.045em; }
  .hero p { color:var(--muted); font-size:18px; max-width:670px; margin:18px auto; }
  main { max-width:1100px; margin:auto; padding:20px 24px 70px; display:grid;
         grid-template-columns:repeat(3,1fr); gap:18px; }
  .card { background:var(--card);
          border:1px solid var(--border); border-radius:18px; padding:24px; min-height:260px;
          display:flex; flex-direction:column; box-shadow:0 18px 50px rgba(0,0,0,.18); }
  .icon { width:48px; height:48px; border-radius:13px; display:grid; place-items:center;
          background:color-mix(in srgb,var(--accent) 13%,var(--card)); color:var(--accent); font-size:25px; }
  h2 { margin:18px 0 2px; font-size:23px; } .arch { color:var(--muted); margin-bottom:20px; }
  .file { color:var(--muted); font-size:12px; overflow-wrap:anywhere; margin-top:auto; }
  .button { display:block; margin-top:14px; border-radius:10px; padding:11px 14px; text-align:center;
            color:#04101b; background:linear-gradient(90deg,var(--accent),#73c6ff); text-decoration:none; font-weight:750; }
  .button:hover { filter:brightness(1.1); transform:translateY(-1px); }
  .missing { background:#223044; color:#a3b1c2; cursor:not-allowed; }
  footer { text-align:center; color:var(--muted); padding:0 24px 38px; }
  @media(max-width:800px){ .shell{display:block}.sidebar{position:fixed;left:0;width:min(280px,calc(100vw - 42px));transform:translateX(-105%);transition:transform 200ms ease}.shell.mobile-open .sidebar{transform:translateX(0)}.mobile-open .backdrop{display:block;position:fixed;z-index:35;inset:0;border:0;background:rgba(0,0,0,.54)}.mobile-menu{display:grid;border-radius:10px}.header-module-switcher>span{display:none}.header-module-switcher select{min-width:0;max-width:132px}.topbar-context{display:none}main{grid-template-columns:1fr}.hero{margin-top:25px} }
</style>
</head>
<body><div class="shell" id="appShell">
<button class="backdrop" id="backdrop" type="button" aria-label="Cerrar navegación"></button>
<aside class="sidebar"><div class="brand"><a class="brand-mark" id="brandLink" href="/" title="Ir a Mi espacio" aria-label="Ir a Mi espacio"><img id="brandLogo" src="/portal-assets/company-logo.svg" alt=""></a><span class="brand-copy"><strong>MRTI</strong><small>Minera Río Tinto</small></span></div><nav><a href="/">⌂ <span>Agentes</span></a><a class="active" href="/downloads/">↓ <span>Descargar agente</span></a></nav><div class="sidebar-section"><span class="section-label">Mi cuenta</span><a class="account-link" id="profileLink" href="#">○ <span>Perfil</span></a></div><div class="sidebar-footer"><button class="sidebar-action" id="themeAction" type="button" title="Cambiar tema">◐</button><button class="sidebar-action" id="logoutAction" type="button" title="Cerrar sesión">↪</button></div></aside>
<div class="workspace"><div class="topbar"><button class="mobile-menu" id="mobileMenu" type="button" aria-label="Abrir navegación">☰</button><label class="header-module-switcher"><span>Cambiar módulo</span><select id="moduleSelect" aria-label="Cambiar de módulo"><option value="" selected disabled>MRTI Agent Core</option></select></label><span class="topbar-context"><strong>Centro de descargas</strong><small>MRTI Agent Core</small></span><div class="notification-center" id="notificationCenter"><button class="notification-button" id="notificationButton" type="button" aria-label="Ver notificaciones">♢<span class="notification-count" id="notificationCount" hidden></span></button><section class="notification-panel" id="notificationPanel" hidden><header><strong>Notificaciones</strong><button id="notificationClose" type="button" aria-label="Cerrar notificaciones">×</button></header><div class="notification-list" id="notificationList"><p>Buscando novedades…</p></div></section></div></div>
<header class="hero">
  <div class="eyebrow">Monitoreo sin interrupciones</div>
  <h1>Instala MRTI Agent</h1>
  <p>Selecciona tu sistema operativo. El agente se instala como servicio, inicia automáticamente y trabaja en segundo plano.</p>
</header>
<main>
{{range .Artifacts}}
  <section class="card">
    <div class="icon">{{.Icon}}</div>
    <h2>{{.OS}}</h2>
    <div class="arch">{{.Architecture}}</div>
    <div class="file">{{.Filename}}{{if .Available}} · {{.Size}}{{end}}</div>
    {{if .Available}}<a class="button" href="/downloads/files/{{.Filename}}">Descargar</a>
    {{else}}<span class="button missing">No disponible</span>{{end}}
  </section>
{{end}}
</main>
<footer>Configura la URL y la clave de tu MRTI Monitor antes de ejecutar el instalador.</footer>
</div></div><script>
const shell=document.getElementById('appShell'), portalOrigin=location.protocol+'//'+location.hostname, portalToken=sessionStorage.getItem('mrti_portal_token');
document.getElementById('brandLink').href=portalOrigin+'/'; document.getElementById('profileLink').href=portalOrigin+'/?view=account';
fetch(portalOrigin+'/api/portal/v1/brand-appearance',{cache:'no-store'}).then(response=>response.ok?response.json():Promise.reject()).then(({data})=>{if(data?.portal_logo?.content_url){const candidate=new Image();candidate.onload=()=>{document.getElementById('brandLogo').src=candidate.src};candidate.src=portalOrigin+data.portal_logo.content_url}}).catch(()=>{});
const esc=value=>String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
const savedTheme=localStorage.getItem('mrti_theme'); if(savedTheme) document.documentElement.dataset.theme=savedTheme;
document.getElementById('themeAction').onclick=()=>{const next=document.documentElement.dataset.theme==='light'?'dark':'light';document.documentElement.dataset.theme=next;localStorage.setItem('mrti_theme',next)};
document.getElementById('mobileMenu').onclick=()=>shell.classList.add('mobile-open'); document.getElementById('backdrop').onclick=()=>shell.classList.remove('mobile-open');
document.getElementById('logoutAction').onclick=async()=>{try{if(portalToken)await fetch(portalOrigin+'/api/auth/logout',{method:'POST',headers:{'Content-Type':'application/json',Authorization:'Bearer '+portalToken},body:'{}'})}catch(e){}sessionStorage.removeItem('mrti_portal_token');location.replace(portalOrigin+'/')};
if(portalToken){
  fetch(portalOrigin+'/api/portal/v1/applications',{headers:{Authorization:'Bearer '+portalToken}}).then(r=>r.ok?r.json():Promise.reject()).then(({data})=>{const apps=(Array.isArray(data)?data:[]).filter(a=>a.code!=='agent-core');document.getElementById('moduleSelect').insertAdjacentHTML('beforeend','<option value="'+portalOrigin+'/">Mi espacio</option>'+apps.map(a=>'<option value="'+portalOrigin+esc(a.url)+'">'+esc(a.name)+'</option>').join(''))}).catch(()=>{});
  fetch(portalOrigin+'/api/portal/v1/notifications',{headers:{Authorization:'Bearer '+portalToken}}).then(r=>r.ok?r.json():Promise.reject()).then(({data})=>{const items=Array.isArray(data)?data:[];const count=document.getElementById('notificationCount');if(items.length){count.hidden=false;count.textContent=items.length>9?'9+':items.length}document.getElementById('notificationList').innerHTML=items.length?items.map(item=>'<article><strong>'+esc(item.title)+'</strong><span>'+esc(item.message)+'</span>'+(item.href?'<a href="'+portalOrigin+esc(item.href)+'">Abrir →</a>':'')+'</article>').join(''):'<p>Sin novedades por ahora.</p>'}).catch(()=>{document.getElementById('notificationList').innerHTML='<p>No fue posible consultar las notificaciones.</p>'});
}
document.getElementById('moduleSelect').onchange=event=>{if(event.target.value)location.assign(event.target.value)};
const notificationPanel=document.getElementById('notificationPanel');document.getElementById('notificationButton').onclick=()=>notificationPanel.hidden=!notificationPanel.hidden;document.getElementById('notificationClose').onclick=()=>notificationPanel.hidden=true;document.addEventListener('keydown',event=>{if(event.key==='Escape')notificationPanel.hidden=true});document.addEventListener('click',event=>{if(!event.target.closest('#notificationCenter'))notificationPanel.hidden=true});
</script></body>
</html>`))
