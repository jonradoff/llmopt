package main

import (
	"context"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/sync/errgroup"
)

// ---------- shared types for templates ----------

type sharePageData struct {
	BrandName     string
	Domain        string
	ShareID       string
	Score         int
	HasScreenshot bool

	// Availability flags for tab nav
	HasBrand  bool
	HasVideo  bool
	HasReddit bool
	HasSearch bool
	HasTodos  bool

	// Tab-specific scores for overview links
	VideoScore  int
	RedditScore int
	SearchScore int
	TodoCount   int

	// Data
	Summary       string
	Components    []vsComponent
	Optimizations []Optimization
	Brand         *BrandProfile
	Video         *VideoAnalysis
	Reddit        *RedditAnalysis
	Search        *SearchAnalysis
	Todos         []TodoItem
}

type vsComponent struct {
	Name      string
	Score     int
	Weight    float64
	Available bool
}

// ---------- data fetching ----------

// fetchShareContext loads the DomainShare + brand name. Returns nil if not found.
func fetchShareContext(ctx context.Context, mongoDB *MongoDB, shareID string) (*DomainShare, string, bool) {
	var ds DomainShare
	err := mongoDB.DomainShares().FindOne(ctx, bson.M{
		"shareId":    shareID,
		"visibility": bson.M{"$in": []string{"public", "popular"}},
	}).Decode(&ds)
	if err != nil {
		return nil, "", false
	}

	brandName := ds.Domain
	var bp BrandProfile
	if mongoDB.BrandProfiles().FindOne(ctx, bson.M{
		"tenantId": ds.TenantID,
		"domain":   ds.Domain,
	}).Decode(&bp) == nil && bp.BrandName != "" {
		brandName = bp.BrandName
	}

	// Increment view count in background
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mongoDB.DomainShares().UpdateOne(bgCtx, bson.M{"_id": ds.ID}, bson.M{"$inc": bson.M{"viewCount": 1}})
	}()

	return &ds, brandName, true
}

// fetchFullShareData loads all data for the share overview page.
func fetchFullShareData(ctx context.Context, mongoDB *MongoDB, ds *DomainShare, brandName string) *sharePageData {
	td := bson.D{{Key: "tenantId", Value: ds.TenantID}, {Key: "domain", Value: ds.Domain}}

	data := &sharePageData{
		BrandName: brandName,
		Domain:    ds.Domain,
		ShareID:   ds.ShareID,
	}

	g, gctx := errgroup.WithContext(ctx)

	// Optimizations
	g.Go(func() error {
		cur, err := mongoDB.Optimizations().Find(gctx, td,
			options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(20))
		if err != nil {
			return nil
		}
		var opts []Optimization
		if cur.All(gctx, &opts) == nil {
			data.Optimizations = opts
		}
		return nil
	})

	// Brand profile
	g.Go(func() error {
		var bp BrandProfile
		if mongoDB.BrandProfiles().FindOne(gctx, td).Decode(&bp) == nil {
			data.Brand = &bp
			data.HasBrand = true
		}
		return nil
	})

	// Video analysis
	g.Go(func() error {
		var va VideoAnalysis
		if mongoDB.VideoAnalyses().FindOne(gctx, td,
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}),
		).Decode(&va) == nil {
			data.Video = &va
			data.HasVideo = true
			if va.Result != nil {
				data.VideoScore = va.Result.OverallScore
			}
		}
		return nil
	})

	// Reddit analysis
	g.Go(func() error {
		var ra RedditAnalysis
		if mongoDB.RedditAnalyses().FindOne(gctx, td,
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}),
		).Decode(&ra) == nil {
			data.Reddit = &ra
			data.HasReddit = true
			if ra.Result != nil {
				data.RedditScore = ra.Result.OverallScore
			}
		}
		return nil
	})

	// Search analysis
	g.Go(func() error {
		var sa SearchAnalysis
		if mongoDB.SearchAnalyses().FindOne(gctx, td,
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}),
		).Decode(&sa) == nil {
			data.Search = &sa
			data.HasSearch = true
			if sa.Result != nil {
				data.SearchScore = sa.Result.OverallScore
			}
		}
		return nil
	})

	// Todos
	g.Go(func() error {
		cur, err := mongoDB.Todos().Find(gctx, td,
			options.Find().SetSort(bson.D{{Key: "priority", Value: -1}, {Key: "createdAt", Value: -1}}))
		if err != nil {
			return nil
		}
		var todos []TodoItem
		if cur.All(gctx, &todos) == nil {
			data.Todos = todos
			data.TodoCount = len(todos)
			data.HasTodos = len(todos) > 0
		}
		return nil
	})

	// Domain summary
	g.Go(func() error {
		var dsm DomainSummary
		if mongoDB.DomainSummaries().FindOne(gctx, td).Decode(&dsm) == nil {
			data.Summary = dsm.Result.ExecutiveSummary
		}
		return nil
	})

	// Visibility score
	g.Go(func() error {
		components := []vsComponent{
			{Name: "Optimization", Weight: 0.30},
			{Name: "Video Authority", Weight: 0.20},
			{Name: "Reddit Authority", Weight: 0.20},
			{Name: "Search Visibility", Weight: 0.15},
			{Name: "LLM Test", Weight: 0.15},
		}

		// Optimization average
		cur, err := mongoDB.Optimizations().Find(gctx, td,
			options.Find().SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}))
		if err == nil {
			var opts []struct {
				Result struct {
					OverallScore int `bson:"overallScore"`
				} `bson:"result"`
			}
			if cur.All(gctx, &opts) == nil && len(opts) > 0 {
				total := 0
				for _, o := range opts {
					total += o.Result.OverallScore
				}
				components[0].Score = total / len(opts)
				components[0].Available = true
			}
		}

		// Video
		var vaScore struct {
			Result *struct {
				OverallScore int `bson:"overallScore"`
			} `bson:"result"`
		}
		if mongoDB.VideoAnalyses().FindOne(gctx, td,
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
		).Decode(&vaScore) == nil && vaScore.Result != nil {
			components[1].Score = vaScore.Result.OverallScore
			components[1].Available = true
		}

		// Reddit
		var raScore struct {
			Result *struct {
				OverallScore int `bson:"overallScore"`
			} `bson:"result"`
		}
		if mongoDB.RedditAnalyses().FindOne(gctx, td,
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
		).Decode(&raScore) == nil && raScore.Result != nil {
			components[2].Score = raScore.Result.OverallScore
			components[2].Available = true
		}

		// Search
		var saScore struct {
			Result *struct {
				OverallScore int `bson:"overallScore"`
			} `bson:"result"`
		}
		if mongoDB.SearchAnalyses().FindOne(gctx, td,
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
		).Decode(&saScore) == nil && saScore.Result != nil {
			components[3].Score = saScore.Result.OverallScore
			components[3].Available = true
		}

		// LLM Test
		var ltScore struct {
			OverallScore int `bson:"overallScore"`
		}
		if mongoDB.LLMTests().FindOne(gctx,
			bson.D{
				{Key: "domain", Value: ds.Domain},
				{Key: "tenantId", Value: ds.TenantID},
				{Key: "competitorOf", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}},
			},
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "overallScore", Value: 1}}),
		).Decode(&ltScore) == nil {
			components[4].Score = ltScore.OverallScore
			components[4].Available = true
		}

		totalWeight := 0.0
		weightedSum := 0.0
		for _, c := range components {
			if c.Available {
				totalWeight += c.Weight
				weightedSum += float64(c.Score) * c.Weight
			}
		}
		if totalWeight > 0 {
			data.Score = int(weightedSum / totalWeight)
		}
		data.Components = components
		return nil
	})

	// Screenshot check
	g.Go(func() error {
		cnt, err := mongoDB.BrandScreenshots().CountDocuments(gctx, bson.M{
			"domain":    ds.Domain,
			"sizeBytes": bson.M{"$gt": 0},
		})
		if err == nil && cnt > 0 {
			data.HasScreenshot = true
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		log.Printf("fetchFullShareData error: %v", err)
	}

	return data
}

// ---------- page handlers ----------

// homeBrand is a simplified brand entry for the homepage grid.
type homeBrand struct {
	Domain      string
	BrandName   string
	ShareID     string
	AvgScore    int
	ReportCount int
	HasVideo    bool
}

// handleHomePage serves the server-rendered marketing homepage.
func handleHomePage(mongoDB *MongoDB, staticDir string) http.HandlerFunc {
	type homeData struct {
		Brands []homeBrand
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// If path is not exactly "/" let SPA handle it
		if r.URL.Path != "/" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var brands []homeBrand

		cur, err := mongoDB.DomainShares().Find(ctx, bson.M{"visibility": "popular"})
		if err == nil {
			var shares []DomainShare
			if cur.All(ctx, &shares) == nil {
				for _, s := range shares {
					brandName := s.Domain
					var bp BrandProfile
					if mongoDB.BrandProfiles().FindOne(ctx, bson.M{
						"tenantId": s.TenantID,
						"domain":   s.Domain,
					}).Decode(&bp) == nil && bp.BrandName != "" {
						brandName = bp.BrandName
					}
					brands = append(brands, homeBrand{
						Domain:    s.Domain,
						BrandName: brandName,
						ShareID:   s.ShareID,
					})
				}
			}
		}

		w.Header().Set("Cache-Control", "public, max-age=300")
		renderPage(w, "home.html", homeData{Brands: brands})
	}
}

// handleDocsPage serves the server-rendered API documentation.
func handleDocsPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		renderPage(w, "docs.html", nil)
	}
}

// handleSharePageSSR serves the full server-rendered share overview.
func handleSharePageSSR(mongoDB *MongoDB, staticDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareID := r.PathValue("shareId")
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		ds, brandName, ok := fetchShareContext(ctx, mongoDB, shareID)
		if !ok {
			// Not found — serve SPA for potential private share
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}

		data := fetchFullShareData(ctx, mongoDB, ds, brandName)
		w.Header().Set("Cache-Control", "public, max-age=300")
		renderPage(w, "share.html", data)
	}
}

// handleShareBrandPage serves the brand intelligence sub-page.
func handleShareBrandPage(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareID := r.PathValue("shareId")
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		ds, brandName, ok := fetchShareContext(ctx, mongoDB, shareID)
		if !ok {
			http.NotFound(w, r)
			return
		}

		data := fetchSubPageData(ctx, mongoDB, ds, brandName)
		w.Header().Set("Cache-Control", "public, max-age=300")
		renderPage(w, "share_brand.html", data)
	}
}

// handleShareVideoPage serves the video authority sub-page.
func handleShareVideoPage(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareID := r.PathValue("shareId")
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		ds, brandName, ok := fetchShareContext(ctx, mongoDB, shareID)
		if !ok {
			http.NotFound(w, r)
			return
		}

		data := fetchSubPageData(ctx, mongoDB, ds, brandName)
		w.Header().Set("Cache-Control", "public, max-age=300")
		renderPage(w, "share_video.html", data)
	}
}

// handleShareRedditPage serves the Reddit authority sub-page.
func handleShareRedditPage(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareID := r.PathValue("shareId")
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		ds, brandName, ok := fetchShareContext(ctx, mongoDB, shareID)
		if !ok {
			http.NotFound(w, r)
			return
		}

		data := fetchSubPageData(ctx, mongoDB, ds, brandName)
		w.Header().Set("Cache-Control", "public, max-age=300")
		renderPage(w, "share_reddit.html", data)
	}
}

// handleShareSearchPage serves the search visibility sub-page.
func handleShareSearchPage(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareID := r.PathValue("shareId")
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		ds, brandName, ok := fetchShareContext(ctx, mongoDB, shareID)
		if !ok {
			http.NotFound(w, r)
			return
		}

		data := fetchSubPageData(ctx, mongoDB, ds, brandName)
		w.Header().Set("Cache-Control", "public, max-age=300")
		renderPage(w, "share_search.html", data)
	}
}

// handleShareTodosPage serves the action items sub-page.
func handleShareTodosPage(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareID := r.PathValue("shareId")
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		ds, brandName, ok := fetchShareContext(ctx, mongoDB, shareID)
		if !ok {
			http.NotFound(w, r)
			return
		}

		data := fetchSubPageData(ctx, mongoDB, ds, brandName)
		w.Header().Set("Cache-Control", "public, max-age=300")
		renderPage(w, "share_todos.html", data)
	}
}

// fetchSubPageData loads the share data needed for sub-page templates.
// Loads all data since sub-pages need tab availability flags.
func fetchSubPageData(ctx context.Context, mongoDB *MongoDB, ds *DomainShare, brandName string) *sharePageData {
	return fetchFullShareData(ctx, mongoDB, ds, brandName)
}
