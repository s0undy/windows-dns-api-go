package api

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"net/http"
)

//go:embed openapi.yaml
var openAPISpec string

// DocsHandler serves the Scalar API documentation UI
func (h *Handler) DocsHandler(w http.ResponseWriter, r *http.Request) {
	// Scalar HTML template with embedded spec
	tmpl := `<!doctype html>
<html>
<head>
    <title>Windows DNS API - Documentation</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
    <script id="api-reference" data-url=""></script>
    <script>
        var configuration = {
            theme: 'default',
            spec: {
                content: {{ .Spec }}
            }
        }
        var apiReference = document.getElementById('api-reference')
        apiReference.dataset.configuration = JSON.stringify(configuration)
    </script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`

	t, err := template.New("docs").Parse(tmpl)
	if err != nil {
		WriteInternalError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// JSON-encode the OpenAPI spec to make it safe for JavaScript
	specJSON, err := json.Marshal(openAPISpec)
	if err != nil {
		WriteInternalError(w, err)
		return
	}

	// Prepare data with the properly encoded spec
	data := struct {
		Spec template.JS
	}{
		Spec: template.JS(specJSON),
	}

	if err := t.Execute(w, data); err != nil {
		WriteInternalError(w, err)
		return
	}
}
