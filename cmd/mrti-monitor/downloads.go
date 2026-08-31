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
  :root { --bg:#f6f2e7; --card:#fff; --border:#e1d3ab; --fg:#221b12;
          --muted:#6c5f47; --accent:#6e4d16; --accent2:#a9781f; color-scheme:light; }
  :root[data-theme=dark] { --bg:#17120c; --card:#251e13; --border:#4a3a22; --fg:#f5ecd9; --muted:#d7cab1; --accent:#d9a63c; --accent2:#f3d68d; color-scheme:dark; }
  @media(prefers-color-scheme:dark){:root:not([data-theme=light]){--bg:#17120c;--card:#251e13;--border:#4a3a22;--fg:#f5ecd9;--muted:#d7cab1;--accent:#d9a63c;--accent2:#f3d68d;color-scheme:dark}}
  * { box-sizing:border-box; }
  body { margin:0; min-height:100vh; color:var(--fg); background:
    radial-gradient(circle at 80% 5%,color-mix(in srgb,var(--accent) 12%,transparent) 0,transparent 30%),var(--bg);
    font:15px/1.5 Inter,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif; }
  .shell { display:grid; grid-template-columns:256px minmax(0,1fr); min-height:100vh; }
  .sidebar { position:sticky; z-index:40; top:0; display:flex; height:100vh; padding:18px 14px; flex-direction:column; border-right:1px solid var(--border); background:var(--card); }
  .brand { display:flex; align-items:center; gap:11px; min-height:61px; padding:0 4px 18px; border-bottom:1px solid var(--border); color:var(--fg); font-weight:750; text-decoration:none; }
  .brand-mark { display:grid; width:42px; height:42px; place-items:center; border:1px solid var(--accent); border-radius:12px; color:var(--accent); }
  .brand small,.topbar small { display:block; color:var(--muted); font-size:11px; }
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
  @media(max-width:800px){ .shell{display:block}.sidebar{position:fixed;left:0;width:min(280px,calc(100vw - 42px));transform:translateX(-105%);transition:transform 200ms ease}.shell.mobile-open .sidebar{transform:translateX(0)}.mobile-open .backdrop{display:block;position:fixed;z-index:35;inset:0;border:0;background:rgba(0,0,0,.54)}.mobile-menu{display:grid;border-radius:10px}main{grid-template-columns:1fr}.hero{margin-top:25px} }
</style>
</head>
<body><div class="shell" id="appShell">
<button class="backdrop" id="backdrop" type="button" aria-label="Cerrar navegación"></button>
<aside class="sidebar"><a class="brand" href="/"><span class="brand-mark">A</span><span>MRTI Agent Core<small>Supervisión de agentes</small></span></a><nav><a href="/">⌂ <span>Agentes</span></a><a class="active" href="/downloads/">↓ <span>Descargar agente</span></a></nav><div class="sidebar-section"><span class="section-label">Mi cuenta</span><a class="account-link" id="profileLink" href="#">○ <span>Perfil</span></a></div><div class="sidebar-footer"><button class="sidebar-action" id="themeAction" type="button" title="Cambiar tema">◐</button><a class="sidebar-action" id="coreLink" href="#" title="Volver a Mi espacio">⌂</a></div></aside>
<div class="workspace"><div class="topbar"><button class="mobile-menu" id="mobileMenu" type="button" aria-label="Abrir navegación">☰</button><span class="topbar-context"><strong>Centro de descargas</strong><small>MRTI Agent Core</small></span></div>
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
const shell=document.getElementById('appShell'), portalOrigin=location.protocol+'//'+location.hostname;
document.getElementById('coreLink').href=portalOrigin+'/'; document.getElementById('profileLink').href=portalOrigin+'/?view=account';
const savedTheme=localStorage.getItem('mrti_theme'); if(savedTheme) document.documentElement.dataset.theme=savedTheme;
document.getElementById('themeAction').onclick=()=>{const next=document.documentElement.dataset.theme==='light'?'dark':'light';document.documentElement.dataset.theme=next;localStorage.setItem('mrti_theme',next)};
document.getElementById('mobileMenu').onclick=()=>shell.classList.add('mobile-open'); document.getElementById('backdrop').onclick=()=>shell.classList.remove('mobile-open');
</script></body>
</html>`))
