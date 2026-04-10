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

var seoTemplates *template.Template

func init() {
	funcMap := template.FuncMap{
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
		"weightPct": func(w float64) string {
			return strings.TrimRight(strings.TrimRight(
				strings.Replace(
					template.HTMLEscapeString(
						strings.TrimRight(strings.TrimRight(
							func() string { return "" }(), "0"), ".")),
					"", "", 0),
				"0"), ".")
			// Simplified: just format as percentage
		},
		"pct": func(w float64) int {
			return int(w * 100)
		},
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": strings.Title,
		"safe": func(s string) template.HTML {
			return template.HTML(s) //nolint:gosec // trusted content from our DB
		},
		"nl2br": func(s string) template.HTML {
			escaped := template.HTMLEscapeString(s)
			return template.HTML(strings.ReplaceAll(escaped, "\n", "<br>")) //nolint:gosec
		},
		"sub": func(a, b int) int { return a - b },
		"add": func(a, b int) int { return a + b },
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
	}

	var err error
	seoTemplates, err = template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse SEO templates: %v", err)
	}
}

// renderPage renders a named template into the base layout and writes to w.
func renderPage(w http.ResponseWriter, name string, data any) {
	// Each page template defines blocks that override the base.
	// We need to execute the page template (which inherits from base).
	tmpl := seoTemplates.Lookup(name)
	if tmpl == nil {
		log.Printf("SEO template %q not found", name)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
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
