package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"llmopt/internal/saas"
)

// --- Fingerprint ---

func computeFingerprint(ctx context.Context, mongoDB *MongoDB, domain string, tenantCtx context.Context) ReportFingerprint {
	fp := ReportFingerprint{}
	filter := func(extra ...bson.E) bson.D {
		f := tenantFilter(tenantCtx, bson.D{{Key: "domain", Value: domain}})
		return append(f, extra...)
	}

	// Latest optimization
	var latestOpt struct {
		CreatedAt time.Time `bson:"createdAt"`
	}
	optOpts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetProjection(bson.D{{Key: "createdAt", Value: 1}})
	if err := mongoDB.Optimizations().FindOne(ctx, filter(), optOpts).Decode(&latestOpt); err == nil {
		fp.LatestOptimizationAt = &latestOpt.CreatedAt
	}
	cnt, _ := mongoDB.Optimizations().CountDocuments(ctx, filter())
	fp.OptimizationCount = int(cnt)

	// Latest analysis
	var latestAn struct {
		CreatedAt time.Time `bson:"createdAt"`
	}
	anOpts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetProjection(bson.D{{Key: "createdAt", Value: 1}})
	if err := mongoDB.Analyses().FindOne(ctx, filter(), anOpts).Decode(&latestAn); err == nil {
		fp.AnalysisCreatedAt = &latestAn.CreatedAt
		fp.AnalysisExists = true
	}

	// Video analysis
	var va struct {
		GeneratedAt time.Time `bson:"generatedAt"`
	}
	if err := mongoDB.VideoAnalyses().FindOne(ctx, filter(), options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "generatedAt", Value: 1}})).Decode(&va); err == nil {
		fp.VideoGeneratedAt = &va.GeneratedAt
		fp.VideoExists = true
	}

	// Reddit analysis
	var ra struct {
		GeneratedAt time.Time `bson:"generatedAt"`
	}
	if err := mongoDB.RedditAnalyses().FindOne(ctx, filter(), options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "generatedAt", Value: 1}})).Decode(&ra); err == nil {
		fp.RedditGeneratedAt = &ra.GeneratedAt
		fp.RedditExists = true
	}

	// Search analysis
	var sa struct {
		GeneratedAt time.Time `bson:"generatedAt"`
	}
	if err := mongoDB.SearchAnalyses().FindOne(ctx, filter(), options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "generatedAt", Value: 1}})).Decode(&sa); err == nil {
		fp.SearchGeneratedAt = &sa.GeneratedAt
		fp.SearchExists = true
	}

	// Domain summary
	var ds struct {
		GeneratedAt time.Time `bson:"generatedAt"`
	}
	if err := mongoDB.DomainSummaries().FindOne(ctx, filter(), options.FindOne().SetProjection(bson.D{{Key: "generatedAt", Value: 1}})).Decode(&ds); err == nil {
		fp.SummaryGeneratedAt = &ds.GeneratedAt
	}

	// Latest todo
	var latestTodo struct {
		CreatedAt time.Time `bson:"createdAt"`
	}
	todoFilter := tenantFilter(tenantCtx, bson.D{{Key: "domain", Value: domain}, {Key: "status", Value: "todo"}})
	todoOpts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetProjection(bson.D{{Key: "createdAt", Value: 1}})
	if err := mongoDB.Todos().FindOne(ctx, todoFilter, todoOpts).Decode(&latestTodo); err == nil {
		fp.LatestTodoAt = &latestTodo.CreatedAt
	}

	return fp
}

func fingerprintEqual(a, b ReportFingerprint) bool {
	timeEq := func(t1, t2 *time.Time) bool {
		if t1 == nil && t2 == nil {
			return true
		}
		if t1 == nil || t2 == nil {
			return false
		}
		return t1.Unix() == t2.Unix()
	}
	return timeEq(a.LatestOptimizationAt, b.LatestOptimizationAt) &&
		timeEq(a.AnalysisCreatedAt, b.AnalysisCreatedAt) &&
		timeEq(a.VideoGeneratedAt, b.VideoGeneratedAt) &&
		timeEq(a.RedditGeneratedAt, b.RedditGeneratedAt) &&
		timeEq(a.SearchGeneratedAt, b.SearchGeneratedAt) &&
		timeEq(a.SummaryGeneratedAt, b.SummaryGeneratedAt) &&
		timeEq(a.LatestTodoAt, b.LatestTodoAt) &&
		a.OptimizationCount == b.OptimizationCount &&
		a.AnalysisExists == b.AnalysisExists &&
		a.VideoExists == b.VideoExists &&
		a.RedditExists == b.RedditExists &&
		a.SearchExists == b.SearchExists
}

// --- PDF Builder ---

func scoreToRGB(score int) (int, int, int) {
	if score >= 80 {
		return 16, 185, 129 // emerald
	}
	if score >= 60 {
		return 245, 158, 11 // amber
	}
	if score >= 40 {
		return 249, 115, 22 // orange
	}
	return 239, 68, 68 // red
}

func pdfSectionHeader(pdf *gofpdf.Fpdf, title string) {
	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetTextColor(30, 30, 30)
	pdf.CellFormat(0, 10, title, "", 1, "L", false, 0, "")
	pdf.Ln(2)
	pdf.SetDrawColor(200, 200, 200)
	x := pdf.GetX()
	y := pdf.GetY()
	pdf.Line(x, y, 190, y)
	pdf.Ln(6)
}

func pdfSubHeader(pdf *gofpdf.Fpdf, title string) {
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(50, 50, 50)
	pdf.CellFormat(0, 8, title, "", 1, "L", false, 0, "")
	pdf.Ln(2)
}

func pdfScoreBox(pdf *gofpdf.Fpdf, label string, score int) {
	r, g, b := scoreToRGB(score)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(r, g, b)
	pdf.CellFormat(25, 7, fmt.Sprintf("%d/100", score), "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(80, 80, 80)
	pdf.CellFormat(0, 7, label, "", 1, "L", false, 0, "")
}

func pdfCleanText(text string) string {
	text = strings.ReplaceAll(text, "\u2022", "-")
	text = strings.ReplaceAll(text, "\u2013", "-")
	text = strings.ReplaceAll(text, "\u2014", "--")
	text = strings.ReplaceAll(text, "\u2018", "'")
	text = strings.ReplaceAll(text, "\u2019", "'")
	text = strings.ReplaceAll(text, "\u201c", "\"")
	text = strings.ReplaceAll(text, "\u201d", "\"")
	return text
}

func pdfBodyText(pdf *gofpdf.Fpdf, text string) {
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(60, 60, 60)
	pdf.MultiCell(0, 5, pdfCleanText(text), "", "L", false)
	pdf.Ln(3)
}

func pdfBullet(pdf *gofpdf.Fpdf, text string) {
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(60, 60, 60)
	text = pdfCleanText(text)
	w, _ := pdf.GetPageSize()
	_, _, mr, _ := pdf.GetMargins()
	maxW := w - pdf.GetX() - mr - 2
	pdf.CellFormat(5, 5, "-", "", 0, "L", false, 0, "")
	pdf.MultiCell(maxW, 5, text, "", "L", false)
}

func pdfCheckPageBreak(pdf *gofpdf.Fpdf, neededMM float64) {
	_, h := pdf.GetPageSize()
	_, _, _, mb := pdf.GetMargins()
	if pdf.GetY()+neededMM > h-mb {
		pdf.AddPage()
	}
}

// tocEntry tracks a section name and its starting page.
type tocEntry struct {
	name    string
	page    int
	missing bool // true if the section data is not yet generated
}

func buildReportPDF(
	domain string,
	analysis *Analysis,
	optimizations []Optimization,
	videoAnalysis *VideoAnalysis,
	redditAnalysis *RedditAnalysis,
	summary *DomainSummary,
	todos []TodoItem,
) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)

	displayDomain := strings.TrimPrefix(domain, "https://")
	displayDomain = strings.TrimPrefix(displayDomain, "http://")

	// --- Cover Page ---
	pdf.AddPage()
	pdf.Ln(40)
	pdf.SetFont("Helvetica", "B", 28)
	pdf.SetTextColor(30, 30, 30)
	pdf.CellFormat(0, 14, "LLM Optimization Report", "", 1, "C", false, 0, "")
	pdf.Ln(8)

	pdf.SetFont("Helvetica", "", 16)
	pdf.SetTextColor(80, 80, 80)
	pdf.CellFormat(0, 10, displayDomain, "", 1, "C", false, 0, "")
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "", 11)
	pdf.SetTextColor(120, 120, 120)
	pdf.CellFormat(0, 7, "Generated "+time.Now().Format("January 2, 2006"), "", 1, "C", false, 0, "")

	// AI Models used
	modelSet := map[string]bool{}
	if analysis != nil && analysis.Model != "" {
		modelSet[analysis.Model] = true
	}
	for _, opt := range optimizations {
		if opt.Model != "" {
			modelSet[opt.Model] = true
		}
	}
	if videoAnalysis != nil && videoAnalysis.Model != "" {
		modelSet[videoAnalysis.Model] = true
	}
	if redditAnalysis != nil && redditAnalysis.Model != "" {
		modelSet[redditAnalysis.Model] = true
	}
	if len(modelSet) > 0 {
		modelNames := make([]string, 0, len(modelSet))
		for m := range modelSet {
			modelNames = append(modelNames, m)
		}
		sort.Strings(modelNames)
		pdf.SetFont("Helvetica", "I", 10)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 6, "AI Models: "+strings.Join(modelNames, ", "), "", 1, "C", false, 0, "")
	}

	pdf.Ln(12)

	// Branding
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(140, 140, 140)
	pdf.CellFormat(0, 6, "Powered by LLM Optimizer", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(100, 130, 200)
	pdf.CellFormat(0, 5, "llmopt.metavert.io", "", 1, "C", false, 0, "")

	// --- Table of Contents Page ---
	pdf.AddPage()
	tocPageNum := pdf.PageNo()
	pdf.SetFont("Helvetica", "B", 20)
	pdf.SetTextColor(30, 30, 30)
	pdf.CellFormat(0, 12, "Table of Contents", "", 1, "L", false, 0, "")
	pdf.Ln(4)
	pdf.SetDrawColor(200, 200, 200)
	x := pdf.GetX()
	y := pdf.GetY()
	pdf.Line(x, y, 190, y)
	pdf.Ln(10)
	// We'll come back to fill this in after rendering all sections.
	// Reserve vertical space — will be overwritten in second pass.

	// Track TOC entries as we render
	toc := []tocEntry{}

	// Count active todos for TOC
	todoCount := 0
	for _, t := range todos {
		if t.Status == "todo" {
			todoCount++
		}
	}

	// --- 1. Executive Summary ---
	if summary != nil {
		pdf.AddPage()
		toc = append(toc, tocEntry{name: "Executive Summary", page: pdf.PageNo()})
		pdfSectionHeader(pdf, "Executive Summary")

		if summary.Result.ExecutiveSummary != "" {
			pdfBodyText(pdf, summary.Result.ExecutiveSummary)
		}

		if summary.Result.AverageScore > 0 {
			pdf.Ln(2)
			pdf.SetFont("Helvetica", "B", 11)
			pdf.SetTextColor(50, 50, 50)
			r, g, b := scoreToRGB(summary.Result.AverageScore)
			pdf.CellFormat(0, 7, "Average Score: ", "", 0, "L", false, 0, "")
			pdf.SetTextColor(r, g, b)
			scoreStr := fmt.Sprintf("%d/100", summary.Result.AverageScore)
			if summary.Result.ScoreRange[0] > 0 && summary.Result.ScoreRange[1] > 0 {
				scoreStr += fmt.Sprintf(" (range: %d-%d)", summary.Result.ScoreRange[0], summary.Result.ScoreRange[1])
			}
			pdf.CellFormat(0, 7, scoreStr, "", 1, "L", false, 0, "")
			pdf.Ln(4)
		}

		if len(summary.Result.Themes) > 0 {
			pdfSubHeader(pdf, "Key Themes")
			for _, theme := range summary.Result.Themes {
				pdfCheckPageBreak(pdf, 15)
				pdf.SetFont("Helvetica", "B", 10)
				pdf.SetTextColor(50, 50, 50)
				pdf.CellFormat(0, 6, theme.Title, "", 1, "L", false, 0, "")
				if theme.Description != "" {
					pdfBodyText(pdf, theme.Description)
				}
			}
		}

	} else {
		toc = append(toc, tocEntry{name: "Executive Summary", missing: true})
	}

	// --- 2. Site Analysis ---
	if analysis != nil {
		pdf.AddPage()
		toc = append(toc, tocEntry{name: "Site Analysis", page: pdf.PageNo()})
		pdfSectionHeader(pdf, "Site Analysis")

		if analysis.Result.SiteSummary != "" {
			pdfSubHeader(pdf, "Site Summary")
			pdfBodyText(pdf, analysis.Result.SiteSummary)
		}

		if len(analysis.Result.Questions) > 0 {
			pdfCheckPageBreak(pdf, 20)
			pdfSubHeader(pdf, fmt.Sprintf("Questions Discovered (%d)", len(analysis.Result.Questions)))

			// Table header
			pdf.SetFont("Helvetica", "B", 9)
			pdf.SetTextColor(80, 80, 80)
			pdf.SetFillColor(245, 245, 245)
			pdf.CellFormat(90, 6, "Question", "B", 0, "L", true, 0, "")
			pdf.CellFormat(35, 6, "Category", "B", 0, "L", true, 0, "")
			pdf.CellFormat(0, 6, "Relevance", "B", 1, "L", true, 0, "")

			pdf.SetFont("Helvetica", "", 8)
			pdf.SetTextColor(60, 60, 60)
			for _, q := range analysis.Result.Questions {
				pdfCheckPageBreak(pdf, 6)
				question := pdfCleanText(q.Question)
				if len(question) > 70 {
					question = question[:67] + "..."
				}
				cat := q.Category
				if len(cat) > 20 {
					cat = cat[:17] + "..."
				}
				pdf.CellFormat(90, 5, question, "", 0, "L", false, 0, "")
				pdf.CellFormat(35, 5, cat, "", 0, "L", false, 0, "")
				pdf.CellFormat(0, 5, q.Relevance, "", 1, "L", false, 0, "")
			}
			pdf.Ln(3)
		}

		if len(analysis.Result.CrawledPages) > 0 {
			pdfCheckPageBreak(pdf, 10)
			pdf.SetFont("Helvetica", "", 10)
			pdf.SetTextColor(100, 100, 100)
			pdf.CellFormat(0, 6, fmt.Sprintf("Pages crawled: %d", len(analysis.Result.CrawledPages)), "", 1, "L", false, 0, "")
		}
	} else {
		toc = append(toc, tocEntry{name: "Site Analysis", missing: true})
	}

	// --- 3. Optimization Reports ---
	if len(optimizations) > 0 {
		pdf.AddPage()
		toc = append(toc, tocEntry{name: fmt.Sprintf("Optimization Reports (%d)", len(optimizations)), page: pdf.PageNo()})
		pdfSectionHeader(pdf, fmt.Sprintf("Optimization Reports (%d)", len(optimizations)))

		for i, opt := range optimizations {
			if i > 0 {
				pdfCheckPageBreak(pdf, 50)
				pdf.Ln(4)
				pdf.SetDrawColor(220, 220, 220)
				lx := pdf.GetX()
				ly := pdf.GetY()
				pdf.Line(lx, ly, 190, ly)
				pdf.Ln(4)
			}

			pdfCheckPageBreak(pdf, 40)
			pdf.SetFont("Helvetica", "B", 11)
			pdf.SetTextColor(40, 40, 40)
			q := pdfCleanText(opt.Question)
			if len(q) > 120 {
				q = q[:117] + "..."
			}
			pdf.MultiCell(0, 6, q, "", "L", false)
			pdf.Ln(2)

			r, g, b := scoreToRGB(opt.Result.OverallScore)
			pdf.SetFont("Helvetica", "B", 14)
			pdf.SetTextColor(r, g, b)
			pdf.CellFormat(30, 8, fmt.Sprintf("%d/100", opt.Result.OverallScore), "", 0, "L", false, 0, "")
			pdf.SetFont("Helvetica", "", 10)
			pdf.SetTextColor(120, 120, 120)
			pdf.CellFormat(0, 8, "Overall Score", "", 1, "L", false, 0, "")
			pdf.Ln(2)

			pdfScoreBox(pdf, "Content Authority", opt.Result.ContentAuthority.Score)
			pdfScoreBox(pdf, "Structural Optimization", opt.Result.StructuralOptimization.Score)
			pdfScoreBox(pdf, "Source Authority", opt.Result.SourceAuthority.Score)
			pdfScoreBox(pdf, "Knowledge Persistence", opt.Result.KnowledgePersistence.Score)
			pdf.Ln(2)

			if len(opt.Result.Competitors) > 0 {
				pdfCheckPageBreak(pdf, 20)
				pdf.Ln(2)
				pdf.SetFont("Helvetica", "B", 10)
				pdf.SetTextColor(80, 80, 80)
				pdf.CellFormat(0, 6, "Competitive Landscape:", "", 1, "L", false, 0, "")
				pdf.SetFont("Helvetica", "", 9)
				for _, comp := range opt.Result.Competitors {
					pdfCheckPageBreak(pdf, 6)
					cr, cg, cb := scoreToRGB(comp.ScoreEstimate)
					pdf.SetTextColor(cr, cg, cb)
					pdf.CellFormat(20, 5, fmt.Sprintf("%d/100", comp.ScoreEstimate), "", 0, "L", false, 0, "")
					pdf.SetTextColor(60, 60, 60)
					name := comp.Domain
					if len(name) > 30 {
						name = name[:27] + "..."
					}
					pdf.CellFormat(50, 5, name, "", 0, "L", false, 0, "")
					strengths := comp.Strengths
					if len(strengths) > 60 {
						strengths = strengths[:57] + "..."
					}
					pdf.SetTextColor(100, 100, 100)
					pdf.CellFormat(0, 5, pdfCleanText(strengths), "", 1, "L", false, 0, "")
				}
			}
		}
	} else {
		toc = append(toc, tocEntry{name: "Optimization Reports", missing: true})
	}

	// --- 4. Video Authority ---
	if videoAnalysis != nil && videoAnalysis.Result != nil {
		pdf.AddPage()
		toc = append(toc, tocEntry{name: "YouTube Video Authority", page: pdf.PageNo()})
		pdfSectionHeader(pdf, "YouTube Video Authority")

		vr := videoAnalysis.Result

		r, g, b := scoreToRGB(vr.OverallScore)
		pdf.SetFont("Helvetica", "B", 18)
		pdf.SetTextColor(r, g, b)
		pdf.CellFormat(30, 10, fmt.Sprintf("%d", vr.OverallScore), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 11)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 10, "/ 100  Overall Video Authority Score", "", 1, "L", false, 0, "")
		pdf.Ln(4)

		pdfScoreBox(pdf, "Transcript Authority", vr.TranscriptAuthority.Score)
		pdfScoreBox(pdf, "Topical Dominance", vr.TopicalDominance.Score)
		pdfScoreBox(pdf, "Citation Network", vr.CitationNetwork.Score)
		pdfScoreBox(pdf, "Brand Narrative", vr.BrandNarrative.Score)
		pdf.Ln(4)

		if vr.ExecutiveSummary != "" {
			pdfSubHeader(pdf, "Summary")
			pdfBodyText(pdf, vr.ExecutiveSummary)
		}

		if len(vr.VideoScorecards) > 0 {
			pdfCheckPageBreak(pdf, 20)
			pdfSubHeader(pdf, "Video Scorecards")
			limit := len(vr.VideoScorecards)
			if limit > 10 {
				limit = 10
			}
			for _, sc := range vr.VideoScorecards[:limit] {
				pdfCheckPageBreak(pdf, 10)
				cr, cg, cb := scoreToRGB(sc.OverallScore)
				pdf.SetTextColor(cr, cg, cb)
				pdf.SetFont("Helvetica", "B", 10)
				pdf.CellFormat(20, 5, fmt.Sprintf("%d/100", sc.OverallScore), "", 0, "L", false, 0, "")
				pdf.SetFont("Helvetica", "", 9)
				pdf.SetTextColor(60, 60, 60)
				title := sc.Title
				if len(title) > 80 {
					title = title[:77] + "..."
				}
				pdf.CellFormat(0, 5, pdfCleanText(title), "", 1, "L", false, 0, "")
			}
			pdf.Ln(2)
		}

	} else {
		toc = append(toc, tocEntry{name: "YouTube Video Authority", missing: true})
	}

	// --- 5. Reddit Authority ---
	if redditAnalysis != nil && redditAnalysis.Result != nil {
		pdf.AddPage()
		toc = append(toc, tocEntry{name: "Reddit Authority", page: pdf.PageNo()})
		pdfSectionHeader(pdf, "Reddit Authority")

		rr := redditAnalysis.Result

		r, g, b := scoreToRGB(rr.OverallScore)
		pdf.SetFont("Helvetica", "B", 18)
		pdf.SetTextColor(r, g, b)
		pdf.CellFormat(30, 10, fmt.Sprintf("%d", rr.OverallScore), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 11)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 10, "/ 100  Overall Reddit Authority Score", "", 1, "L", false, 0, "")
		pdf.Ln(4)

		pdfScoreBox(pdf, "Presence", rr.Presence.Score)
		pdfScoreBox(pdf, "Sentiment & Recommendations", rr.Sentiment.Score)
		pdfScoreBox(pdf, "Competitive Positioning", rr.Competitive.Score)
		pdfScoreBox(pdf, "Training Signal Strength", rr.TrainingSignal.Score)
		pdf.Ln(4)

		if rr.ExecutiveSummary != "" {
			pdfSubHeader(pdf, "Summary")
			pdfBodyText(pdf, rr.ExecutiveSummary)
		}

		if len(rr.Presence.ShareOfVoice) > 0 {
			pdfCheckPageBreak(pdf, 20)
			pdfSubHeader(pdf, "Share of Voice")
			for _, sov := range rr.Presence.ShareOfVoice {
				pdfCheckPageBreak(pdf, 6)
				pdf.SetFont("Helvetica", "B", 10)
				pdf.SetTextColor(80, 80, 80)
				pdf.CellFormat(20, 5, fmt.Sprintf("%.0f%%", sov.Percentage), "", 0, "L", false, 0, "")
				pdf.SetFont("Helvetica", "", 10)
				pdf.SetTextColor(60, 60, 60)
				pdf.CellFormat(0, 5, fmt.Sprintf("%s (%d mentions)", sov.BrandName, sov.MentionCount), "", 1, "L", false, 0, "")
			}
			pdf.Ln(2)
		}

	} else {
		toc = append(toc, tocEntry{name: "Reddit Authority", missing: true})
	}

	// --- 6. Recommendations (consolidated) ---
	type pdfRec struct {
		action   string
		priority string
		source   string // "General", "YouTube", "Reddit"
		impact   string
	}
	var allRecs []pdfRec
	seen := map[string]bool{}

	addRec := func(action, priority, source, impact string) {
		key := strings.ToLower(strings.TrimSpace(action))
		if seen[key] || key == "" {
			return
		}
		seen[key] = true
		if priority == "" {
			priority = "medium"
		}
		allRecs = append(allRecs, pdfRec{action: action, priority: priority, source: source, impact: impact})
	}

	// Active todos first (user-curated, highest importance)
	for _, t := range todos {
		if t.Status != "todo" {
			continue
		}
		source := "General"
		if t.SourceType == "video" {
			source = "YouTube"
		} else if t.SourceType == "reddit" {
			source = "Reddit"
		}
		addRec(t.Action, t.Priority, source, t.ExpectedImpact)
	}

	// Optimization report recommendations
	for _, opt := range optimizations {
		recs := opt.Result.Recommendations
		if len(recs) > 3 {
			recs = recs[:3]
		}
		for _, rec := range recs {
			addRec(rec.Action, rec.Priority, "General", rec.ExpectedImpact)
		}
	}

	// Video recommendations
	if videoAnalysis != nil && videoAnalysis.Result != nil {
		for _, rec := range videoAnalysis.Result.Recommendations {
			addRec(rec.Action, rec.Priority, "YouTube", rec.ExpectedImpact)
		}
	}

	// Reddit recommendations
	if redditAnalysis != nil && redditAnalysis.Result != nil {
		for _, rec := range redditAnalysis.Result.Recommendations {
			addRec(rec.Action, rec.Priority, "Reddit", rec.ExpectedImpact)
		}
	}

	if len(allRecs) > 0 {
		priorityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		sort.Slice(allRecs, func(i, j int) bool {
			pi := priorityOrder[allRecs[i].priority]
			pj := priorityOrder[allRecs[j].priority]
			if pi != pj {
				return pi < pj
			}
			// Within same priority, group by source for readability
			return allRecs[i].source < allRecs[j].source
		})

		pdf.AddPage()
		toc = append(toc, tocEntry{name: "Recommendations", page: pdf.PageNo()})
		pdfSectionHeader(pdf, "Recommendations")

		// Disclaimer
		pdf.SetFont("Helvetica", "I", 9)
		pdf.SetTextColor(100, 100, 100)
		pdf.MultiCell(0, 4.5, pdfCleanText("The following recommendations are intended to improve visibility and authority specifically within large language models and AI-powered search systems. They do not necessarily reflect broader business constraints, priorities, or objectives, and should be considered alongside your overall strategy for digital presence and brand development."), "", "L", false)
		pdf.Ln(6)

		currentPriority := ""
		for _, rec := range allRecs {
			if rec.priority != currentPriority {
				currentPriority = rec.priority
				pdfCheckPageBreak(pdf, 15)
				pdf.Ln(3)
				pdf.SetFont("Helvetica", "B", 11)
				priorityColor := map[string][3]int{
					"high":   {239, 68, 68},
					"medium": {245, 158, 11},
					"low":    {107, 114, 128},
				}
				c := priorityColor[currentPriority]
				if c == [3]int{} {
					c = [3]int{107, 114, 128}
				}
				pdf.SetTextColor(c[0], c[1], c[2])
				pdf.CellFormat(0, 7, strings.ToUpper(currentPriority)+" PRIORITY", "", 1, "L", false, 0, "")
				pdf.Ln(1)
			}

			pdfCheckPageBreak(pdf, 12)
			// Source tag + action
			pdf.SetFont("Helvetica", "B", 9)
			pdf.SetTextColor(100, 100, 100)
			tagW := pdf.GetStringWidth("["+rec.source+"] ") + 2
			pdf.CellFormat(tagW, 5, "["+rec.source+"]", "", 0, "L", false, 0, "")
			pdf.SetFont("Helvetica", "", 10)
			pdf.SetTextColor(60, 60, 60)
			w, _ := pdf.GetPageSize()
			_, _, mr, _ := pdf.GetMargins()
			maxW := w - pdf.GetX() - mr
			pdf.MultiCell(maxW, 5, pdfCleanText(rec.action), "", "L", false)
			if rec.impact != "" {
				pdf.SetFont("Helvetica", "I", 9)
				pdf.SetTextColor(140, 140, 140)
				pdf.CellFormat(tagW, 4, "", "", 0, "L", false, 0, "")
				impact := rec.impact
				if len(impact) > 100 {
					impact = impact[:97] + "..."
				}
				pdf.CellFormat(0, 4, "Impact: "+pdfCleanText(impact), "", 1, "L", false, 0, "")
			}
		}
	} else {
		toc = append(toc, tocEntry{name: "Recommendations", missing: true})
	}

	// --- 7. Supporting Research ---
	pdf.AddPage()
	toc = append(toc, tocEntry{name: "Supporting Research", page: pdf.PageNo()})
	pdfSectionHeader(pdf, "Supporting Research")

	pdfSubHeader(pdf, "Research Digest")
	researchParagraphs := []string{
		"The emerging science of LLM visibility reveals a fundamental shift in how information gains authority online. The most significant recent finding comes from NanoKnow (2026), which demonstrates that content appearing frequently in training data more than doubles a model's accuracy on related questions -- and that the advantage compounds when content is both memorized during training and retrievable at inference time. This means the traditional SEO playbook of optimizing for a single ranking algorithm is being replaced by a dual imperative: getting into training corpora through widespread, high-quality publication, while simultaneously remaining citable through structured, authoritative web presence.",
		"Across the research, a consistent pattern emerges: AI search engines overwhelmingly favor earned media over brand-owned content, citing third-party sources 72-92% of the time. Content that includes quotations from authoritative sources gains +41% visibility -- the single most effective optimization technique identified. Meanwhile, YouTube has rapidly become the dominant social citation source for LLMs, with its share doubling to 39% between August and December 2024. Critically, video LLMs process content through transcripts, not visual analysis -- a 7B model trained on YouTube transcripts outperformed 72B models, proving that transcript quality matters far more than production value.",
		"Reddit has emerged as the #2 social citation source for LLMs, with unique authority dynamics. Reddit was foundational in LLM training through datasets like WebText and the Common Crawl, and continues through $60M (Google) and $70M (OpenAI) annual licensing deals. Unlike YouTube's channel-centric authority, Reddit's influence comes from multi-user validation -- upvoted comment consensus, especially in \"best X for Y\" recommendation threads, creates credibility signals that LLMs weight heavily.",
		"However, this new landscape comes with important caveats. Citation accuracy across AI answer engines remains surprisingly poor (49-68%), with nearly a third of claims lacking any source backing. Citation concentration follows power-law dynamics, where the top 20 sources capture 28-67% of all citations. And LLMs exhibit strong positional bias, reliably attending to content at the beginning and end of context while ignoring the middle. Together, these findings inform LLM Optimizer's scoring frameworks across answer optimization, video authority, and Reddit authority analysis.",
	}
	for _, para := range researchParagraphs {
		pdfCheckPageBreak(pdf, 25)
		pdfBodyText(pdf, para)
	}

	pdf.Ln(4)
	pdfSubHeader(pdf, "Bibliography")

	type bibEntry struct {
		ref   string
		title string
		venue string
		url   string
	}
	bibliography := []bibEntry{
		{"[1]", "Lost in the Middle: How Language Models Use Long Contexts", "TACL 2024", "https://arxiv.org/abs/2307.03172"},
		{"[2]", "GEO: Generative Engine Optimization", "Princeton / KDD 2024", "https://arxiv.org/abs/2311.09735"},
		{"[3]", "NanoKnow: Probing LLM Knowledge by Linking Training Data to Answers", "2026", "https://arxiv.org/abs/2602.20122"},
		{"[4]", "GEO: How to Dominate AI Search -- Source Preferences", "U of Toronto 2025", "https://arxiv.org/abs/2509.08919"},
		{"[5]", "YouTube vs Reddit AI Citations", "Adweek / Bluefish / Emberos / Goodie AI, 2025", "https://www.adweek.com/media/youtube-reddit-ai-search-engine-citations"},
		{"[6]", "News Source Citing Patterns in AI Search Systems", "2025", "https://arxiv.org/abs/2507.05301"},
		{"[7]", "LiveCC: Learning Video LLM with Streaming Speech Transcription", "CVPR 2025", "https://arxiv.org/abs/2504.16030"},
		{"[8]", "The False Promise of Factual and Verifiable Source-Cited Responses", "2024", "https://arxiv.org/abs/2410.22349"},
		{"[9]", "Language Models are Unsupervised Multitask Learners", "OpenAI, 2019 (Radford et al.)", "https://cdn.openai.com/better-language-models/language_models_are_unsupervised_multitask_learners.pdf"},
		{"[10]", "Consent in Crisis: The Rapid Decline of the AI Data Commons", "ACM FAccT 2024 (Longpre et al.)", "https://dl.acm.org/doi/10.1145/3630106.3659033"},
		{"[11]", "Reddit Data Licensing: Google and OpenAI Deals", "Reuters / The Verge, 2024", "https://www.reuters.com/technology/reddit-ai-content-licensing-deal-google-2024-02-22/"},
		{"[12]", "Community Consensus as LLM Authority Signal", "Bluefish Labs / Emberos Research, 2025", "https://www.adweek.com/media/youtube-reddit-ai-search-engine-citations"},
	}

	for _, bib := range bibliography {
		pdfCheckPageBreak(pdf, 14)
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(80, 80, 80)
		pdf.CellFormat(10, 5, bib.ref, "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(40, 40, 40)
		w, _ := pdf.GetPageSize()
		_, _, mr, _ := pdf.GetMargins()
		maxW := w - pdf.GetX() - mr
		pdf.MultiCell(maxW, 4.5, fmt.Sprintf("%s. %s.", pdfCleanText(bib.title), bib.venue), "", "L", false)
		pdf.SetX(pdf.GetX() + 10)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(100, 130, 200)
		pdf.CellFormat(0, 4, bib.url, "", 1, "L", false, 0, "")
		pdf.Ln(1.5)
	}

	// --- Fill in Table of Contents (second pass) ---
	pdf.SetPage(tocPageNum)
	// Position after the header area
	pdf.SetY(42)

	for _, entry := range toc {
		pdf.SetFont("Helvetica", "", 11)
		if entry.missing {
			// Missing section: amber italic
			pdf.SetTextColor(160, 120, 40)
			pdf.SetFont("Helvetica", "I", 11)
			pdf.CellFormat(0, 7, fmt.Sprintf("    %s  -  Not yet generated", entry.name), "", 1, "L", false, 0, "")
		} else {
			pdf.SetTextColor(40, 40, 40)
			// Section name on left, page number on right
			nameW := pdf.GetStringWidth(entry.name)
			pageStr := fmt.Sprintf("%d", entry.page)
			pageW := pdf.GetStringWidth(pageStr)
			w, _ := pdf.GetPageSize()
			ml, _, mr, _ := pdf.GetMargins()
			available := w - ml - mr
			dotW := available - nameW - pageW - 8 // 8mm padding
			dots := ""
			if dotW > 0 {
				dotCount := int(dotW / pdf.GetStringWidth("."))
				for i := 0; i < dotCount; i++ {
					dots += "."
				}
			}
			pdf.CellFormat(0, 7, fmt.Sprintf("    %s %s %s", entry.name, dots, pageStr), "", 1, "L", false, 0, "")
		}
	}

	// --- Footer on all pages (with AutoPageBreak disabled to prevent blank pages) ---
	pdf.SetAutoPageBreak(false, 0)
	totalPages := pdf.PageCount()
	for i := 1; i <= totalPages; i++ {
		pdf.SetPage(i)
		_, h := pdf.GetPageSize()
		pdf.SetY(h - 12)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(160, 160, 160)
		pdf.CellFormat(0, 5, fmt.Sprintf("Page %d of %d  |  LLM Optimizer  |  %s", i, totalPages, displayDomain), "", 0, "C", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	if pdf.Err() {
		return nil, fmt.Errorf("PDF error: %s", pdf.Error())
	}
	return buf.Bytes(), nil
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// --- Handlers ---

func handleGeneratePDF(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		sendSSE(w, flusher, "status", map[string]string{"message": "Checking for cached report..."})

		// Compute current fingerprint
		fp := computeFingerprint(ctx, mongoDB, domain, r.Context())

		if fp.OptimizationCount == 0 && !fp.AnalysisExists && !fp.VideoExists && !fp.RedditExists {
			sendSSE(w, flusher, "error", map[string]string{"message": "No reports found for this domain"})
			return
		}

		// Check cache
		var cached ReportPDF
		cacheFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		err := mongoDB.ReportPDFs().FindOne(ctx, cacheFilter).Decode(&cached)
		if err == nil && fingerprintEqual(cached.Fingerprint, fp) {
			sendSSE(w, flusher, "done", map[string]any{
				"pdf_id":     cached.ID.Hex(),
				"cached":     true,
				"size_bytes": cached.SizeBytes,
			})
			return
		}

		// Gather data — Analysis
		sendSSE(w, flusher, "status", map[string]string{"message": "Loading site analysis..."})
		var analysis *Analysis
		var an Analysis
		anFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		anOpts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetProjection(bson.D{{Key: "rawText", Value: 0}})
		if err := mongoDB.Analyses().FindOne(ctx, anFilter, anOpts).Decode(&an); err == nil {
			analysis = &an
		}

		// Gather data — Optimizations
		sendSSE(w, flusher, "status", map[string]string{"message": "Gathering optimization reports..."})
		cursor, err := mongoDB.Optimizations().Find(ctx, tenantFilter(r.Context(), bson.D{
			{Key: "domain", Value: domain},
		}), options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(50).SetProjection(bson.D{
			{Key: "rawText", Value: 0},
		}))
		var optimizations []Optimization
		if err == nil {
			_ = cursor.All(ctx, &optimizations)
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Found %d optimization reports", len(optimizations)),
		})

		// Video analysis
		sendSSE(w, flusher, "status", map[string]string{"message": "Loading video analysis..."})
		var videoAnalysis *VideoAnalysis
		var va VideoAnalysis
		vaFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		vaOpts := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "rawText", Value: 0}})
		if err := mongoDB.VideoAnalyses().FindOne(ctx, vaFilter, vaOpts).Decode(&va); err == nil {
			videoAnalysis = &va
		}

		// Reddit analysis
		sendSSE(w, flusher, "status", map[string]string{"message": "Loading Reddit analysis..."})
		var redditAnalysis *RedditAnalysis
		var ra RedditAnalysis
		raFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		raOpts := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "rawText", Value: 0}})
		if err := mongoDB.RedditAnalyses().FindOne(ctx, raFilter, raOpts).Decode(&ra); err == nil {
			redditAnalysis = &ra
		}

		// Domain summary
		sendSSE(w, flusher, "status", map[string]string{"message": "Loading executive summary..."})
		var summary *DomainSummary
		var ds DomainSummary
		dsFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		if err := mongoDB.DomainSummaries().FindOne(ctx, dsFilter).Decode(&ds); err == nil {
			summary = &ds
		}

		// Todos
		sendSSE(w, flusher, "status", map[string]string{"message": "Loading action items..."})
		todoFilter := tenantFilter(r.Context(), bson.D{
			{Key: "domain", Value: domain},
			{Key: "status", Value: "todo"},
		})
		todoCursor, err := mongoDB.Todos().Find(ctx, todoFilter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(200))
		var todos []TodoItem
		if err == nil {
			_ = todoCursor.All(ctx, &todos)
		}

		// Build PDF
		sendSSE(w, flusher, "status", map[string]string{"message": "Generating PDF report..."})
		pdfBytes, err := buildReportPDF(domain, analysis, optimizations, videoAnalysis, redditAnalysis, summary, todos)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Failed to generate PDF: " + err.Error()})
			return
		}

		// Upsert into cache
		sendSSE(w, flusher, "status", map[string]string{"message": "Saving report..."})
		now := time.Now()
		tenantID := saas.TenantIDFromContext(r.Context())
		doc := ReportPDF{
			TenantID:    tenantID,
			Domain:      domain,
			PDFData:     pdfBytes,
			SizeBytes:   len(pdfBytes),
			Fingerprint: fp,
			GeneratedAt: now,
		}

		replaceFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		replaceUpsert := &options.ReplaceOptions{}
		replaceUpsert.SetUpsert(true)
		result, err := mongoDB.ReportPDFs().ReplaceOne(ctx, replaceFilter, doc, replaceUpsert)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Failed to cache PDF: " + err.Error()})
			return
		}

		// Determine the ID
		pdfID := ""
		if result.UpsertedID != nil {
			if oid, ok := result.UpsertedID.(primitive.ObjectID); ok {
				pdfID = oid.Hex()
			}
		}
		if pdfID == "" {
			var existing ReportPDF
			if err := mongoDB.ReportPDFs().FindOne(ctx, replaceFilter, options.FindOne().SetProjection(bson.D{{Key: "_id", Value: 1}})).Decode(&existing); err == nil {
				pdfID = existing.ID.Hex()
			}
		}

		sendSSE(w, flusher, "done", map[string]any{
			"pdf_id":     pdfID,
			"cached":     false,
			"size_bytes": len(pdfBytes),
		})
	}
}

func handleServePDF(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		idStr := r.PathValue("id")

		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, "Invalid PDF ID", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{
			{Key: "_id", Value: oid},
			{Key: "domain", Value: domain},
		})

		var pdf ReportPDF
		if err := mongoDB.ReportPDFs().FindOne(ctx, filter).Decode(&pdf); err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, "PDF not found", http.StatusNotFound)
			} else {
				http.Error(w, "Database error", http.StatusInternalServerError)
			}
			return
		}

		displayDomain := strings.TrimPrefix(domain, "https://")
		displayDomain = strings.TrimPrefix(displayDomain, "http://")
		filename := displayDomain + "-llm-report.pdf"

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Header().Set("Content-Length", strconv.Itoa(len(pdf.PDFData)))
		w.Write(pdf.PDFData)
	}
}

// Ensure imports are used
var _ = json.Marshal
