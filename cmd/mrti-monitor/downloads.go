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
  :root { --bg:#08111f; --card:#101c2d; --border:#26374c; --fg:#edf5ff;
          --muted:#91a4bb; --accent:#35a7ff; --accent2:#37d6a2; }
  * { box-sizing:border-box; }
  body { margin:0; min-height:100vh; color:var(--fg); background:
    radial-gradient(circle at 80% 5%,#12395a 0,transparent 30%),var(--bg);
    font:15px/1.5 Inter,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif; }
  nav { max-width:1100px; margin:auto; padding:24px; display:flex; justify-content:space-between; }
  nav a { color:var(--muted); text-decoration:none; } .brand { color:var(--fg); font-weight:750; }
  header { max-width:820px; margin:50px auto 36px; padding:0 24px; text-align:center; }
  .eyebrow { color:var(--accent2); letter-spacing:.12em; text-transform:uppercase; font-size:12px; font-weight:700; }
  h1 { margin:12px 0; font-size:clamp(36px,7vw,64px); line-height:1.05; letter-spacing:-.045em; }
  header p { color:var(--muted); font-size:18px; max-width:670px; margin:18px auto; }
  main { max-width:1100px; margin:auto; padding:20px 24px 70px; display:grid;
         grid-template-columns:repeat(3,1fr); gap:18px; }
  .card { background:linear-gradient(145deg,rgba(20,36,56,.96),rgba(12,25,41,.96));
          border:1px solid var(--border); border-radius:18px; padding:24px; min-height:260px;
          display:flex; flex-direction:column; box-shadow:0 18px 50px rgba(0,0,0,.18); }
  .icon { width:48px; height:48px; border-radius:13px; display:grid; place-items:center;
          background:#172b43; color:var(--accent); font-size:25px; }
  h2 { margin:18px 0 2px; font-size:23px; } .arch { color:var(--muted); margin-bottom:20px; }
  .file { color:var(--muted); font-size:12px; overflow-wrap:anywhere; margin-top:auto; }
  .button { display:block; margin-top:14px; border-radius:10px; padding:11px 14px; text-align:center;
            color:#04101b; background:linear-gradient(90deg,var(--accent),#73c6ff); text-decoration:none; font-weight:750; }
  .button:hover { filter:brightness(1.1); transform:translateY(-1px); }
  .missing { background:#223044; color:#a3b1c2; cursor:not-allowed; }
  footer { text-align:center; color:var(--muted); padding:0 24px 38px; }
  @media(max-width:800px){ main { grid-template-columns:1fr; } header { margin-top:25px; } }
</style>
</head>
<body>
<nav><span class="brand">MRTI</span><a href="/">Panel de monitoreo →</a></nav>
<header>
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
</body>
</html>`))
