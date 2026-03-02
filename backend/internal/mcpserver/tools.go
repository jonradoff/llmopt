package mcpserver

import (
	"context"
	"sort"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"llmopt/internal/saas"
)

type handlers struct {
	sm *saas.Middleware
	db *mongo.Database
}

// tenantBSON returns a bson.D filter scoped to the current tenant.
func tenantBSON(ctx context.Context, extra ...bson.E) bson.D {
	filter := bson.D{}
	if tid := saas.TenantIDFromContext(ctx); tid != "" {
		filter = append(filter, bson.E{Key: "tenantId", Value: tid})
	}
	filter = append(filter, extra...)
	return filter
}

// ─── Tool Definitions ──────────────────────────────────────────────────────

func listDomainsTool() mcp.Tool {
	return mcp.NewTool("llmopt_list_domains",
		mcp.WithDescription("List all domains tracked for your LLM Optimizer account. Returns an array of domain names that have analysis data. Use this to discover which domains are available before requesting specific reports."),
	)
}

func getReportTool() mcp.Tool {
	return mcp.NewTool("llmopt_get_report",
		mcp.WithDescription("Get a specific analysis report for a domain. Returns the full report data for the requested type. Available report types: 'analysis' (site structure), 'optimizations' (answer engine scores), 'video' (video authority), 'reddit' (Reddit authority), 'search' (search visibility), 'summary' (executive summary), 'tests' (LLM knowledge tests), 'brand' (brand intelligence profile)."),
		mcp.WithString("domain",
			mcp.Required(),
			mcp.Description("The domain to get the report for (e.g. 'example.com')"),
		),
		mcp.WithString("report_type",
			mcp.Required(),
			mcp.Description("Type of report to retrieve"),
			mcp.Enum("analysis", "optimizations", "video", "reddit", "search", "summary", "tests", "brand"),
		),
	)
}

func getVisibilityScoreTool() mcp.Tool {
	return mcp.NewTool("llmopt_get_visibility_score",
		mcp.WithDescription("Get the composite AI visibility score for a domain. Returns a weighted score (0-100) computed from five components: Optimization (30%), Video Authority (20%), Reddit Authority (20%), Search Visibility (15%), and LLM Test (15%). Each component shows its individual score and availability."),
		mcp.WithString("domain",
			mcp.Required(),
			mcp.Description("The domain to score (e.g. 'example.com')"),
		),
	)
}

func listTodosTool() mcp.Tool {
	return mcp.NewTool("llmopt_list_todos",
		mcp.WithDescription("List action items (todos) generated from LLM optimization analyses. Todos are actionable recommendations for improving AI visibility. Filter by status or domain to focus on specific areas."),
		mcp.WithString("status",
			mcp.Description("Filter by status: 'todo' (open), 'completed', 'backlogged', 'archived'"),
			mcp.Enum("todo", "completed", "backlogged", "archived"),
		),
		mcp.WithString("domain",
			mcp.Description("Filter by domain (e.g. 'example.com')"),
		),
	)
}

func updateTodoTool() mcp.Tool {
	return mcp.NewTool("llmopt_update_todo",
		mcp.WithDescription("Update the status of a todo item. Requires admin or owner role. Valid transitions: 'todo' (reopen), 'completed' (mark done), 'backlogged' (defer), 'archived' (dismiss). Use llmopt_list_todos first to find the todo ID."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("The todo item ID (MongoDB ObjectID hex string)"),
		),
		mcp.WithString("status",
			mcp.Required(),
			mcp.Description("New status for the todo"),
			mcp.Enum("todo", "completed", "backlogged", "archived"),
		),
	)
}

// ─── Handlers ──────────────────────────────────────────────────────────────

func (h *handlers) listDomains(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	filter := tenantBSON(ctx)
	domainSet := make(map[string]bool)

	collections := []string{
		"analyses", "optimizations", "brand_profiles",
		"video_analyses", "reddit_analyses", "search_analyses",
		"domain_summaries", "llm_tests",
	}

	for _, name := range collections {
		vals, err := h.db.Collection(name).Distinct(qctx, "domain", filter)
		if err == nil {
			for _, v := range vals {
				if s, ok := v.(string); ok && s != "" {
					domainSet[s] = true
				}
			}
		}
	}

	domains := make([]string, 0, len(domainSet))
	for d := range domainSet {
		domains = append(domains, d)
	}
	sort.Strings(domains)

	return mcp.NewToolResultJSON(map[string]any{"domains": domains, "count": len(domains)})
}

func (h *handlers) getReport(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	domain, err := req.RequireString("domain")
	if err != nil {
		return mcp.NewToolResultError("domain is required"), nil
	}
	reportType, err := req.RequireString("report_type")
	if err != nil {
		return mcp.NewToolResultError("report_type is required"), nil
	}

	qctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	noRawText := bson.D{{Key: "rawText", Value: 0}}

	switch reportType {
	case "analysis":
		return h.findLatest(qctx, "analyses", domain, "createdAt", noRawText)

	case "optimizations":
		return h.findAll(qctx, "optimizations", domain, "createdAt", noRawText)

	case "video":
		return h.findLatest(qctx, "video_analyses", domain, "generatedAt", noRawText)

	case "reddit":
		return h.findLatest(qctx, "reddit_analyses", domain, "generatedAt", nil)

	case "search":
		return h.findLatest(qctx, "search_analyses", domain, "generatedAt", nil)

	case "summary":
		filter := tenantBSON(ctx, bson.E{Key: "domain", Value: domain})
		var result bson.M
		err := h.db.Collection("domain_summaries").FindOne(qctx, filter).Decode(&result)
		if err != nil {
			return mcp.NewToolResultError("no summary found for " + domain), nil
		}
		return mcp.NewToolResultJSON(result)

	case "tests":
		filter := tenantBSON(ctx, bson.E{Key: "domain", Value: domain})
		opts := options.Find().
			SetSort(bson.D{{Key: "createdAt", Value: -1}}).
			SetLimit(10)
		cursor, err := h.db.Collection("llm_tests").Find(qctx, filter, opts)
		if err != nil {
			return mcp.NewToolResultError("database error"), nil
		}
		defer cursor.Close(qctx)
		var tests []bson.M
		if err := cursor.All(qctx, &tests); err != nil {
			return mcp.NewToolResultError("database error"), nil
		}
		if len(tests) == 0 {
			return mcp.NewToolResultError("no test results found for " + domain), nil
		}
		return mcp.NewToolResultJSON(map[string]any{"latest": tests[0], "history": tests, "count": len(tests)})

	case "brand":
		filter := tenantBSON(ctx, bson.E{Key: "domain", Value: domain})
		var result bson.M
		err := h.db.Collection("brand_profiles").FindOne(qctx, filter).Decode(&result)
		if err != nil {
			return mcp.NewToolResultError("no brand profile found for " + domain), nil
		}
		return mcp.NewToolResultJSON(result)

	default:
		return mcp.NewToolResultError("unknown report_type: " + reportType), nil
	}
}

func (h *handlers) findLatest(ctx context.Context, collection, domain, sortField string, projection bson.D) (*mcp.CallToolResult, error) {
	filter := tenantBSON(ctx, bson.E{Key: "domain", Value: domain})
	opts := options.FindOne().SetSort(bson.D{{Key: sortField, Value: -1}})
	if projection != nil {
		opts.SetProjection(projection)
	}

	var result bson.M
	err := h.db.Collection(collection).FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		return mcp.NewToolResultError("no " + collection + " data found for " + domain), nil
	}
	return mcp.NewToolResultJSON(result)
}

func (h *handlers) findAll(ctx context.Context, collection, domain, sortField string, projection bson.D) (*mcp.CallToolResult, error) {
	filter := tenantBSON(ctx, bson.E{Key: "domain", Value: domain})
	opts := options.Find().SetSort(bson.D{{Key: sortField, Value: -1}})
	if projection != nil {
		opts.SetProjection(projection)
	}

	cursor, err := h.db.Collection(collection).Find(ctx, filter, opts)
	if err != nil {
		return mcp.NewToolResultError("database error"), nil
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return mcp.NewToolResultError("database error"), nil
	}
	if len(results) == 0 {
		return mcp.NewToolResultError("no " + collection + " data found for " + domain), nil
	}
	return mcp.NewToolResultJSON(map[string]any{"items": results, "count": len(results)})
}

func (h *handlers) getVisibilityScore(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	domain, err := req.RequireString("domain")
	if err != nil {
		return mcp.NewToolResultError("domain is required"), nil
	}

	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	tenantID := saas.TenantIDFromContext(ctx)

	type component struct {
		Name      string  `json:"name"`
		Score     int     `json:"score"`
		Weight    float64 `json:"weight"`
		Available bool    `json:"available"`
	}

	components := []component{
		{Name: "Optimization", Weight: 0.30},
		{Name: "Video Authority", Weight: 0.20},
		{Name: "Reddit Authority", Weight: 0.20},
		{Name: "Search Visibility", Weight: 0.15},
		{Name: "LLM Test", Weight: 0.15},
	}

	// 1. Optimization average
	cursor, cerr := h.db.Collection("optimizations").Find(qctx,
		tenantBSON(ctx, bson.E{Key: "domain", Value: domain}),
		options.Find().SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
	)
	if cerr == nil {
		var opts []struct {
			Result struct {
				OverallScore int `bson:"overallScore"`
			} `bson:"result"`
		}
		if cursor.All(qctx, &opts) == nil && len(opts) > 0 {
			total := 0
			for _, o := range opts {
				total += o.Result.OverallScore
			}
			components[0].Score = total / len(opts)
			components[0].Available = true
		}
	}

	// 2. Video authority
	var va struct {
		Result *struct {
			OverallScore int `bson:"overallScore"`
		} `bson:"result"`
	}
	if h.db.Collection("video_analyses").FindOne(qctx,
		bson.D{{Key: "domain", Value: domain}, {Key: "tenantId", Value: tenantID}},
		options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
	).Decode(&va) == nil && va.Result != nil {
		components[1].Score = va.Result.OverallScore
		components[1].Available = true
	}

	// 3. Reddit authority
	var ra struct {
		Result *struct {
			OverallScore int `bson:"overallScore"`
		} `bson:"result"`
	}
	if h.db.Collection("reddit_analyses").FindOne(qctx,
		bson.D{{Key: "domain", Value: domain}, {Key: "tenantId", Value: tenantID}},
		options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
	).Decode(&ra) == nil && ra.Result != nil {
		components[2].Score = ra.Result.OverallScore
		components[2].Available = true
	}

	// 4. Search visibility
	var sa struct {
		Result *struct {
			OverallScore int `bson:"overallScore"`
		} `bson:"result"`
	}
	if h.db.Collection("search_analyses").FindOne(qctx,
		bson.D{{Key: "domain", Value: domain}, {Key: "tenantId", Value: tenantID}},
		options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
	).Decode(&sa) == nil && sa.Result != nil {
		components[3].Score = sa.Result.OverallScore
		components[3].Available = true
	}

	// 5. LLM Test
	var lt struct {
		OverallScore int `bson:"overallScore"`
	}
	if h.db.Collection("llm_tests").FindOne(qctx,
		bson.D{
			{Key: "domain", Value: domain},
			{Key: "tenantId", Value: tenantID},
			{Key: "competitorOf", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}},
		},
		options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "overallScore", Value: 1}}),
	).Decode(&lt) == nil {
		components[4].Score = lt.OverallScore
		components[4].Available = true
	}

	totalWeight := 0.0
	weightedSum := 0.0
	available := 0
	for _, c := range components {
		if c.Available {
			totalWeight += c.Weight
			weightedSum += float64(c.Score) * c.Weight
			available++
		}
	}

	score := 0
	if totalWeight > 0 {
		score = int(weightedSum / totalWeight)
	}

	return mcp.NewToolResultJSON(map[string]any{
		"domain":     domain,
		"score":      score,
		"components": components,
		"available":  available,
		"total":      len(components),
	})
}

func (h *handlers) listTodos(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	qctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filter := tenantBSON(ctx)
	if status := req.GetString("status", ""); status != "" {
		filter = append(filter, bson.E{Key: "status", Value: status})
	}
	if domain := req.GetString("domain", ""); domain != "" {
		filter = append(filter, bson.E{Key: "domain", Value: domain})
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(200)

	cursor, err := h.db.Collection("todos").Find(qctx, filter, opts)
	if err != nil {
		return mcp.NewToolResultError("database error"), nil
	}
	defer cursor.Close(qctx)

	var todos []bson.M
	if err := cursor.All(qctx, &todos); err != nil {
		return mcp.NewToolResultError("database error"), nil
	}
	if todos == nil {
		todos = []bson.M{}
	}

	return mcp.NewToolResultJSON(map[string]any{"todos": todos, "count": len(todos)})
}

func (h *handlers) updateTodo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check role
	role := saas.MemberRoleFromContext(ctx)
	if role != "admin" && role != "owner" {
		return mcp.NewToolResultError("insufficient permissions: admin or owner role required"), nil
	}

	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required"), nil
	}
	status, err := req.RequireString("status")
	if err != nil {
		return mcp.NewToolResultError("status is required"), nil
	}

	if status != "todo" && status != "completed" && status != "backlogged" && status != "archived" {
		return mcp.NewToolResultError("status must be 'todo', 'completed', 'backlogged', or 'archived'"), nil
	}

	oid, oerr := primitive.ObjectIDFromHex(id)
	if oerr != nil {
		return mcp.NewToolResultError("invalid todo ID format"), nil
	}

	qctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	now := time.Now()
	var update bson.D
	switch status {
	case "completed":
		update = bson.D{
			{Key: "$set", Value: bson.D{{Key: "status", Value: status}, {Key: "completedAt", Value: now}}},
			{Key: "$unset", Value: bson.D{{Key: "backloggedAt", Value: ""}, {Key: "archivedAt", Value: ""}}},
		}
	case "backlogged":
		update = bson.D{
			{Key: "$set", Value: bson.D{{Key: "status", Value: status}, {Key: "backloggedAt", Value: now}}},
			{Key: "$unset", Value: bson.D{{Key: "completedAt", Value: ""}, {Key: "archivedAt", Value: ""}}},
		}
	case "archived":
		update = bson.D{
			{Key: "$set", Value: bson.D{{Key: "status", Value: status}, {Key: "archivedAt", Value: now}}},
			{Key: "$unset", Value: bson.D{{Key: "completedAt", Value: ""}, {Key: "backloggedAt", Value: ""}}},
		}
	default: // "todo"
		update = bson.D{
			{Key: "$set", Value: bson.D{{Key: "status", Value: status}}},
			{Key: "$unset", Value: bson.D{{Key: "completedAt", Value: ""}, {Key: "backloggedAt", Value: ""}, {Key: "archivedAt", Value: ""}}},
		}
	}

	filter := tenantBSON(ctx, bson.E{Key: "_id", Value: oid})
	result, uerr := h.db.Collection("todos").UpdateOne(qctx, filter, update)
	if uerr != nil {
		return mcp.NewToolResultError("database error"), nil
	}
	if result.MatchedCount == 0 {
		return mcp.NewToolResultError("todo not found"), nil
	}

	return mcp.NewToolResultJSON(map[string]any{"updated": true, "id": id, "status": status})
}
