package oauth

import (
	"html/template"
	"net/http"
)

// callbackPageHTML is the page the browser tab is left on once the callback is
// handled, so the outcome is visible where the user's attention already is.
// Rendered through html/template, which escapes the server-supplied error.
const callbackPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>k6 — {{if .Error}}authentication failed{{else}}authentication complete{{end}}</title>
<style>
  body { background: #111217; color: #ccccdc; font-family: -apple-system, BlinkMacSystemFont,
         "Segoe UI", Roboto, sans-serif; display: flex; align-items: center;
         justify-content: center; height: 100vh; margin: 0; }
  main { text-align: center; max-width: 30rem; padding: 2rem; }
  h1 { font-size: 1.5rem; font-weight: 500; margin: 0 0 0.75rem; }
  h1.ok { color: #6ccf8e; }
  h1.err { color: #ff5286; }
  p { line-height: 1.5; margin: 0; color: #8e8e8e; }
  p + p { margin-top: 0.75rem; }
</style>
</head>
<body>
<main>
{{if .Error}}
  <h1 class="err">Authentication failed</h1>
  <p>{{.Error}}</p>
  <p>Go back to the CLI and try again.</p>
{{else}}
  <h1 class="ok">Authentication complete</h1>
  <p>Go back to the CLI.</p>
{{end}}
</main>
</body>
</html>
`

func writeSuccessPage(w http.ResponseWriter) {
	writePage(w, http.StatusOK, "")
}

func writeErrorPage(w http.ResponseWriter, message string) {
	writePage(w, http.StatusBadRequest, stripControlChars(message))
}

// writePage renders the callback page. The template is parsed per call rather
// than once at package level: it is a compile-time constant parsed in
// microseconds, and this runs at most once per login.
func writePage(w http.ResponseWriter, status int, errMessage string) {
	tmpl := template.Must(template.New("callback").Parse(callbackPageHTML))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	// A failed render or write has no useful recovery: the login's outcome is
	// already decided and reported on the flow's channels, and only this
	// cosmetic page is lost.
	_ = tmpl.Execute(w, struct{ Error string }{Error: errMessage})
}
