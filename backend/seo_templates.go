package main

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

// seoPageTemplates maps page template name -> parsed template (base + page).
var seoPageTemplates map[string]*template.Template

var seoFuncMap = template.FuncMap{
	"scoreClass": func(score int) string {
		if score >= 70 {
			return "green"
		} else if score >= 40 {
			return "yellow"
		}
		return "red"
	},
	"barClass": func(score int) string {
		if score >= 70 {
			return "bar-green"
		} else if score >= 40 {
			return "bar-yellow"
		}
		return "bar-red"
	},
	"pillClass": func(priority string) string {
		switch strings.ToLower(priority) {
		case "high", "critical":
			return "pill-red"
		case "medium":
			return "pill-yellow"
		case "low":
			return "pill-green"
		}
		return ""
	},
	"priorityClass": func(priority string) string {
		switch strings.ToLower(priority) {
		case "high", "critical":
			return "priority-high"
		case "medium":
			return "priority-medium"
		case "low":
			return "priority-low"
		}
		return ""
	},
	"methodClass": func(method string) string {
		switch strings.ToUpper(method) {
		case "GET":
			return "method-get"
		case "POST":
			return "method-post"
		case "PATCH", "PUT":
			return "method-patch"
		case "DELETE":
			return "method-delete"
		}
		return ""
	},
	"formatDate": func(t time.Time) string {
		return t.Format("Jan 2, 2006")
	},
	"truncate": func(s string, n int) string {
		if len(s) <= n {
			return s
		}
		return s[:n] + "..."
	},
	"pct": func(w float64) int {
		return int(w * 100)
	},
	"upper": strings.ToUpper,
	"lower": strings.ToLower,
	"safe": func(s string) template.HTML {
		return template.HTML(s) //nolint:gosec // trusted content from our DB
	},
	"nl2br": func(s string) template.HTML {
		escaped := template.HTMLEscapeString(s)
		return template.HTML(strings.ReplaceAll(escaped, "\n", "<br>")) //nolint:gosec
	},
	"sub": func(a, b int) int { return a - b },
	"add": func(a, b int) int { return a + b },
}

func init() {
	// Read the base template content
	baseContent, err := templateFS.ReadFile("templates/base.html")
	if err != nil {
		log.Fatalf("Failed to read base.html: %v", err)
	}

	// Find all page templates (everything except base.html)
	entries, err := fs.ReadDir(templateFS, "templates")
	if err != nil {
		log.Fatalf("Failed to read templates dir: %v", err)
	}

	seoPageTemplates = make(map[string]*template.Template)

	for _, e := range entries {
		if e.IsDir() || e.Name() == "base.html" {
			continue
		}

		pageContent, err := templateFS.ReadFile("templates/" + e.Name())
		if err != nil {
			log.Fatalf("Failed to read template %s: %v", e.Name(), err)
		}

		// Parse base first, then page (page's define blocks override base's)
		tmpl, err := template.New("base.html").Funcs(seoFuncMap).Parse(string(baseContent))
		if err != nil {
			log.Fatalf("Failed to parse base.html for %s: %v", e.Name(), err)
		}

		_, err = tmpl.New(e.Name()).Parse(string(pageContent))
		if err != nil {
			log.Fatalf("Failed to parse %s: %v", e.Name(), err)
		}

		seoPageTemplates[e.Name()] = tmpl
	}
}

// renderPage renders a named page template (which inherits from base) and writes to w.
func renderPage(w http.ResponseWriter, name string, data any) {
	tmpl, ok := seoPageTemplates[name]
	if !ok {
		log.Printf("SEO template %q not found", name)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	// Execute the page template, which invokes base.html via {{template "base.html" .}}
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("SEO template %q render error: %v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(buf.Bytes())
}

// handleStaticAssets serves the embedded static files (CSS, etc.).
func handleStaticAssets() http.HandlerFunc {
	sub, _ := fs.Sub(staticFS, "static")
	fileServer := http.FileServer(http.FS(sub))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.StripPrefix("/static/", fileServer).ServeHTTP(w, r)
	}
}
