package main

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CrawledPage struct {
	URL   string `json:"url" bson:"url"`
	Title string `json:"title" bson:"title"`
}

type Question struct {
	Question    string   `json:"question" bson:"question"`
	Relevance   string   `json:"relevance" bson:"relevance"`
	Category    string   `json:"category" bson:"category"`
	PageURLs    []string `json:"page_urls" bson:"pageUrls"`
	BrandStatus string   `json:"brand_status,omitempty" bson:"brandStatus,omitempty"`
}

type AnalysisResult struct {
	SiteSummary  string        `json:"site_summary" bson:"siteSummary"`
	Questions    []Question    `json:"questions" bson:"questions"`
	CrawledPages []CrawledPage `json:"crawled_pages" bson:"crawledPages"`
}

type Analysis struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TenantID  string             `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	Domain    string             `json:"domain" bson:"domain"`
	RawText   string             `json:"raw_text" bson:"rawText"`
	Result                AnalysisResult     `json:"result" bson:"result"`
	Model                 string             `json:"model" bson:"model"`
	BrandContextUsed      bool               `json:"brand_context_used" bson:"brandContextUsed"`
	BrandProfileUpdatedAt *time.Time         `json:"brand_profile_updated_at,omitempty" bson:"brandProfileUpdatedAt,omitempty"`
	CreatedAt             time.Time          `json:"created_at" bson:"createdAt"`
}

// Answer optimization types

type DimensionScore struct {
	Score        int      `json:"score" bson:"score"`
	Evidence     []string `json:"evidence" bson:"evidence"`
	Improvements []string `json:"improvements" bson:"improvements"`
}

type Competitor struct {
	Domain        string `json:"domain" bson:"domain"`
	ScoreEstimate int    `json:"score_estimate" bson:"scoreEstimate"`
	Strengths     string `json:"strengths" bson:"strengths"`
}

type Recommendation struct {
	Priority       string `json:"priority" bson:"priority"`
	Action         string `json:"action" bson:"action"`
	ExpectedImpact string `json:"expected_impact" bson:"expectedImpact"`
	Dimension      string `json:"dimension" bson:"dimension"`
}

type OptimizationResult struct {
	OverallScore           int              `json:"overall_score" bson:"overallScore"`
	ContentAuthority       DimensionScore   `json:"content_authority" bson:"contentAuthority"`
	StructuralOptimization DimensionScore   `json:"structural_optimization" bson:"structuralOptimization"`
	SourceAuthority        DimensionScore   `json:"source_authority" bson:"sourceAuthority"`
	KnowledgePersistence   DimensionScore   `json:"knowledge_persistence" bson:"knowledgePersistence"`
	Competitors            []Competitor     `json:"competitors" bson:"competitors"`
	Recommendations        []Recommendation `json:"recommendations" bson:"recommendations"`
}

type Optimization struct {
	ID            primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TenantID      string             `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	AnalysisID    primitive.ObjectID `json:"analysis_id" bson:"analysisId"`
	QuestionIndex int                `json:"question_index" bson:"questionIndex"`
	Question      string             `json:"question" bson:"question"`
	Domain        string             `json:"domain" bson:"domain"`
	PageURLs      []string           `json:"page_urls" bson:"pageUrls"`
	Result        OptimizationResult `json:"result" bson:"result"`
	RawText       string             `json:"raw_text" bson:"rawText"`
	BrandStatus   string             `json:"brand_status,omitempty" bson:"brandStatus,omitempty"`
	Model                 string             `json:"model" bson:"model"`
	Public                bool               `json:"public" bson:"public"`
	BrandContextUsed      bool               `json:"brand_context_used" bson:"brandContextUsed"`
	BrandProfileUpdatedAt *time.Time         `json:"brand_profile_updated_at,omitempty" bson:"brandProfileUpdatedAt,omitempty"`
	CreatedAt             time.Time          `json:"created_at" bson:"createdAt"`
}

// Todo item types

type TodoItem struct {
	ID              primitive.ObjectID  `json:"id" bson:"_id,omitempty"`
	TenantID        string              `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	OptimizationID  primitive.ObjectID  `json:"optimization_id" bson:"optimizationId"`
	AnalysisID      primitive.ObjectID  `json:"analysis_id" bson:"analysisId"`
	VideoAnalysisID *primitive.ObjectID `json:"video_analysis_id,omitempty" bson:"videoAnalysisId,omitempty"`
	SourceType      string              `json:"source_type" bson:"sourceType"` // "optimization" or "video"
	Domain          string              `json:"domain" bson:"domain"`
	Question        string              `json:"question" bson:"question"`
	Action          string              `json:"action" bson:"action"`
	Summary         string              `json:"summary" bson:"summary"`
	ExpectedImpact  string              `json:"expected_impact" bson:"expectedImpact"`
	Dimension       string              `json:"dimension" bson:"dimension"`
	Priority        string              `json:"priority" bson:"priority"`
	Status          string              `json:"status" bson:"status"` // "todo", "completed", "backlogged", "archived"
	CreatedAt       time.Time           `json:"created_at" bson:"createdAt"`
	CompletedAt     *time.Time          `json:"completed_at,omitempty" bson:"completedAt,omitempty"`
	BackloggedAt    *time.Time          `json:"backlogged_at,omitempty" bson:"backloggedAt,omitempty"`
	ArchivedAt      *time.Time          `json:"archived_at,omitempty" bson:"archivedAt,omitempty"`
}

// OptimizationSummary is the lightweight shape returned by the optimizations list endpoint.
type OptimizationSummary struct {
	ID            primitive.ObjectID `json:"id" bson:"_id"`
	Domain        string             `json:"domain" bson:"domain"`
	Question      string             `json:"question" bson:"question"`
	QuestionIndex int                `json:"question_index"`
	OverallScore  int                `json:"overall_score"`
	Model         string             `json:"model" bson:"model"`
	Public                bool               `json:"public" bson:"public"`
	BrandStatus           string             `json:"brand_status,omitempty"`
	BrandContextUsed      bool               `json:"brand_context_used"`
	BrandProfileUpdatedAt *time.Time         `json:"brand_profile_updated_at,omitempty"`
	CreatedAt             time.Time          `json:"created_at" bson:"createdAt"`
}

// AnalysisSummary is the lightweight shape returned by the list endpoint.
type AnalysisSummary struct {
	ID            primitive.ObjectID `json:"id" bson:"_id"`
	Domain        string             `json:"domain" bson:"domain"`
	SiteSummary   string             `json:"site_summary"`
	QuestionCount int                `json:"question_count"`
	PageCount     int                `json:"page_count"`
	Model                 string             `json:"model"`
	BrandContextUsed      bool               `json:"brand_context_used"`
	BrandProfileUpdatedAt *time.Time         `json:"brand_profile_updated_at,omitempty"`
	CreatedAt             time.Time          `json:"created_at" bson:"createdAt"`
}

// Brand Intelligence types

type BrandCompetitor struct {
	Name         string `json:"name" bson:"name"`
	URL          string `json:"url" bson:"url"`
	Relationship string `json:"relationship" bson:"relationship"` // direct, indirect, aspirational, adjacent
	Notes        string `json:"notes" bson:"notes"`
}

type TargetQuery struct {
	Query    string `json:"query" bson:"query"`
	Priority string `json:"priority" bson:"priority"` // high, medium, low
	Type     string `json:"type" bson:"type"`         // brand, category, comparison, problem
}

type KeyMessage struct {
	Claim       string `json:"claim" bson:"claim"`
	EvidenceURL string `json:"evidence_url" bson:"evidenceUrl"`
	Priority    string `json:"priority" bson:"priority"` // high, medium, low
}

type BrandPresence struct {
	YouTubeURL     string   `json:"youtube_url" bson:"youtubeUrl"`
	Subreddits     []string `json:"subreddits" bson:"subreddits"`
	ReviewSiteURLs []string `json:"review_site_urls" bson:"reviewSiteUrls"`
	SocialProfiles []string `json:"social_profiles" bson:"socialProfiles"`
	ContentAssets  []string `json:"content_assets" bson:"contentAssets"`
	Podcasts       []string `json:"podcasts" bson:"podcasts"`
}

type BrandProfile struct {
	ID              primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TenantID        string             `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	Domain          string             `json:"domain" bson:"domain"`
	BrandName       string             `json:"brand_name" bson:"brandName"`
	Description     string             `json:"description" bson:"description"`
	Categories      []string           `json:"categories" bson:"categories"`
	Products        []string           `json:"products" bson:"products"`
	PrimaryAudience string             `json:"primary_audience" bson:"primaryAudience"`
	KeyUseCases     []string           `json:"key_use_cases" bson:"keyUseCases"`
	Competitors     []BrandCompetitor  `json:"competitors" bson:"competitors"`
	TargetQueries   []TargetQuery      `json:"target_queries" bson:"targetQueries"`
	KeyMessages     []KeyMessage       `json:"key_messages" bson:"keyMessages"`
	Differentiators []string           `json:"differentiators" bson:"differentiators"`
	Presence         BrandPresence      `json:"presence" bson:"presence"`
	PresenceComplete bool               `json:"presence_complete" bson:"presenceComplete"`
	Public           bool               `json:"public" bson:"public"`
	LastDiscoveryAt  *time.Time         `json:"last_discovery_at,omitempty" bson:"lastDiscoveryAt,omitempty"`
	CreatedAt       time.Time          `json:"created_at" bson:"createdAt"`
	UpdatedAt       time.Time          `json:"updated_at" bson:"updatedAt"`
}

type BrandProfileSummary struct {
	ID              primitive.ObjectID `json:"id" bson:"_id"`
	Domain          string             `json:"domain" bson:"domain"`
	BrandName       string             `json:"brand_name" bson:"brandName"`
	CompetitorCount int                `json:"competitor_count"`
	QueryCount      int                `json:"query_count"`
	Completeness    int                `json:"completeness"`
	Public          bool               `json:"public" bson:"public"`
	UpdatedAt       time.Time          `json:"updated_at" bson:"updatedAt"`
}

// Health check persistence

type ModelStatusRecord struct {
	Model      string `json:"model" bson:"model"`
	Name       string `json:"name" bson:"name"`
	Status     string `json:"status" bson:"status"` // "available", "overloaded", "error"
	LatencyMs  int64  `json:"latency_ms,omitempty" bson:"latencyMs,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty" bson:"httpStatus,omitempty"`
}

type HealthRecord struct {
	ID        primitive.ObjectID  `json:"id" bson:"_id,omitempty"`
	Models    []ModelStatusRecord `json:"models" bson:"models"`
	CheckedAt time.Time           `json:"checked_at" bson:"checkedAt"`
}

// Domain Summary types

type SummaryTheme struct {
	Title       string   `json:"title" bson:"title"`
	Description string   `json:"description" bson:"description"`
	ReportRefs  []string `json:"report_refs" bson:"reportRefs"`
}

type SummaryActionItem struct {
	Priority       string   `json:"priority" bson:"priority"`
	Action         string   `json:"action" bson:"action"`
	Dimension      string   `json:"dimension" bson:"dimension"`
	ExpectedImpact string   `json:"expected_impact" bson:"expectedImpact"`
	SourceReports  []string `json:"source_reports" bson:"sourceReports"`
}

type SummaryContradiction struct {
	Topic          string   `json:"topic" bson:"topic"`
	Positions      []string `json:"positions" bson:"positions"`
	ReportRefs     []string `json:"report_refs" bson:"reportRefs"`
	Recommendation string   `json:"recommendation" bson:"recommendation"`
}

type DomainSummaryResult struct {
	ExecutiveSummary string                 `json:"executive_summary" bson:"executiveSummary"`
	AverageScore     int                    `json:"average_score" bson:"averageScore"`
	ScoreRange       [2]int                 `json:"score_range" bson:"scoreRange"`
	Themes           []SummaryTheme         `json:"themes" bson:"themes"`
	ActionItems      []SummaryActionItem    `json:"action_items" bson:"actionItems"`
	Contradictions   []SummaryContradiction `json:"contradictions" bson:"contradictions"`
	DimensionTrends  map[string]int         `json:"dimension_trends" bson:"dimensionTrends"`
}

type DomainSummary struct {
	ID              primitive.ObjectID   `json:"id" bson:"_id,omitempty"`
	TenantID        string               `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	Domain          string               `json:"domain" bson:"domain"`
	Result          DomainSummaryResult  `json:"result" bson:"result"`
	RawText         string               `json:"raw_text" bson:"rawText"`
	Model           string               `json:"model" bson:"model"`
	OptimizationIDs []primitive.ObjectID `json:"optimization_ids" bson:"optimizationIds"`
	ReportCount     int                  `json:"report_count" bson:"reportCount"`
	GeneratedAt     time.Time            `json:"generated_at" bson:"generatedAt"`
}

// Video Authority Analyzer types

type VideoAnalysisConfig struct {
	ChannelURL  string   `json:"channel_url" bson:"channelUrl"`
	VideoURLs   []string `json:"video_urls" bson:"videoUrls"`
	BrandURL    string   `json:"brand_url" bson:"brandUrl"`
	SearchTerms []string `json:"search_terms" bson:"searchTerms"`
}

type YouTubeVideo struct {
	VideoID      string    `json:"video_id" bson:"videoId"`
	Title        string    `json:"title" bson:"title"`
	ChannelTitle string    `json:"channel_title" bson:"channelTitle"`
	ChannelID    string    `json:"channel_id" bson:"channelId"`
	Description  string    `json:"description" bson:"description"`
	PublishedAt  time.Time `json:"published_at" bson:"publishedAt"`
	ViewCount    int64     `json:"view_count" bson:"viewCount"`
	LikeCount    int64     `json:"like_count" bson:"likeCount"`
	CommentCount int64     `json:"comment_count" bson:"commentCount"`
	Duration     string    `json:"duration" bson:"duration"`
	Tags         []string  `json:"tags" bson:"tags"`
	Transcript   string    `json:"transcript,omitempty" bson:"transcript,omitempty"`
	RelevanceTag string    `json:"relevance_tag" bson:"relevanceTag"`
}

type YouTubeChannel struct {
	ChannelID       string `json:"channel_id" bson:"channelId"`
	Title           string `json:"title" bson:"title"`
	SubscriberCount int64  `json:"subscriber_count" bson:"subscriberCount"`
	VideoCount      int64  `json:"video_count" bson:"videoCount"`
	ViewCount       int64  `json:"view_count" bson:"viewCount"`
}

// Video recommendation — structured action item from video analysis
type VideoRecommendation struct {
	Action         string `json:"action" bson:"action"`
	ExpectedImpact string `json:"expected_impact" bson:"expectedImpact"`
	Dimension      string `json:"dimension" bson:"dimension"` // transcript_authority, topical_dominance, citation_network, brand_narrative
	Priority       string `json:"priority" bson:"priority"`   // high, medium, low
	VideoID        string `json:"video_id,omitempty" bson:"videoId,omitempty"`
}

// Per-video scorecard (own channel videos)
type VideoScorecard struct {
	VideoID                  string   `json:"video_id" bson:"videoId"`
	Title                    string   `json:"title" bson:"title"`
	OverallScore             int      `json:"overall_score" bson:"overallScore"`
	TranscriptPower          int      `json:"transcript_power" bson:"transcriptPower"`
	StructuralExtractability int      `json:"structural_extractability" bson:"structuralExtractability"`
	DiscoverySurface         int      `json:"discovery_surface" bson:"discoverySurface"`
	HasTranscript            bool     `json:"has_transcript" bson:"hasTranscript"`
	KeyFindings              []string `json:"key_findings" bson:"keyFindings"`
}

// Brand mention in a third-party video
type BrandMention struct {
	VideoID              string   `json:"video_id" bson:"videoId"`
	Title                string   `json:"title" bson:"title"`
	ChannelTitle         string   `json:"channel_title" bson:"channelTitle"`
	ViewCount            int64    `json:"view_count" bson:"viewCount"`
	Sentiment            string   `json:"sentiment" bson:"sentiment"`
	MentionContext       string   `json:"mention_context" bson:"mentionContext"`
	MentionPosition      string   `json:"mention_position" bson:"mentionPosition"`
	Extractability       string   `json:"extractability" bson:"extractability"`
	CompetitorsMentioned []string `json:"competitors_mentioned" bson:"competitorsMentioned"`
}

// Creator/channel profile assessment
type CreatorProfile struct {
	ChannelTitle    string `json:"channel_title" bson:"channelTitle"`
	ChannelID       string `json:"channel_id" bson:"channelId"`
	SubscriberCount int64  `json:"subscriber_count" bson:"subscriberCount"`
	Sentiment       string `json:"sentiment" bson:"sentiment"`
	VideoCount      int    `json:"video_count" bson:"videoCount"`
	TotalViews      int64  `json:"total_views" bson:"totalViews"`
	Role            string `json:"role" bson:"role"`
	AuthorityScore  int    `json:"authority_score" bson:"authorityScore"`
}

type ShareOfVoiceEntry struct {
	BrandName    string  `json:"brand_name" bson:"brandName"`
	MentionCount int     `json:"mention_count" bson:"mentionCount"`
	Percentage   float64 `json:"percentage" bson:"percentage"`
}

type ContentGap struct {
	Query                string   `json:"query" bson:"query"`
	CompetitorsMentioned []string `json:"competitors_mentioned" bson:"competitorsMentioned"`
	OpportunityScore     int      `json:"opportunity_score" bson:"opportunityScore"`
	VideoCount           int      `json:"video_count" bson:"videoCount"`
	Recommendation       string   `json:"recommendation" bson:"recommendation"`
}

type CreatorTarget struct {
	ChannelTitle         string   `json:"channel_title" bson:"channelTitle"`
	ChannelID            string   `json:"channel_id" bson:"channelId"`
	SubscriberCount      int64    `json:"subscriber_count" bson:"subscriberCount"`
	CategoryRelevance    string   `json:"category_relevance" bson:"categoryRelevance"`
	CompetitorsMentioned []string `json:"competitors_mentioned" bson:"competitorsMentioned"`
	OutreachReason       string   `json:"outreach_reason" bson:"outreachReason"`
}

// Pillar sub-scores for the 4-pillar unified report

type TranscriptAuthorityPillar struct {
	Score                int      `json:"score" bson:"score"`
	Evidence             []string `json:"evidence" bson:"evidence"`
	TranscriptCoverage   int      `json:"transcript_coverage" bson:"transcriptCoverage"`     // % of own videos with transcripts
	KeywordAlignment     int      `json:"keyword_alignment" bson:"keywordAlignment"`         // 0-100
	QuotabilityScore     int      `json:"quotability_score" bson:"quotabilityScore"`         // 0-100
	InformationDensity   int      `json:"information_density" bson:"informationDensity"`     // 0-100
}

type TopicalDominancePillar struct {
	Score          int               `json:"score" bson:"score"`
	Evidence       []string          `json:"evidence" bson:"evidence"`
	TopicsCovered  int               `json:"topics_covered" bson:"topicsCovered"`
	TopicsTotal    int               `json:"topics_total" bson:"topicsTotal"`
	CoverageDepth  int               `json:"coverage_depth" bson:"coverageDepth"` // 0-100
	VsCompetitors  string            `json:"vs_competitors" bson:"vsCompetitors"` // narrative comparison
	ShareOfVoice   []ShareOfVoiceEntry `json:"share_of_voice" bson:"shareOfVoice"`
	ContentGaps    []ContentGap      `json:"content_gaps" bson:"contentGaps"`
}

type CitationNetworkPillar struct {
	Score              int             `json:"score" bson:"score"`
	Evidence           []string        `json:"evidence" bson:"evidence"`
	CreatorMentions    int             `json:"creator_mentions" bson:"creatorMentions"`       // # third-party creators mentioning brand
	AuthoritativeRefs  int             `json:"authoritative_refs" bson:"authoritativeRefs"`   // mentions by high-authority creators
	ConcentrationRisk  string          `json:"concentration_risk" bson:"concentrationRisk"`   // narrative on concentration
	TopCreators        []CreatorProfile `json:"top_creators" bson:"topCreators"`
	CreatorTargets     []CreatorTarget `json:"creator_targets" bson:"creatorTargets"`
}

type SentimentBreakdown struct {
	Positive int `json:"positive" bson:"positive"`
	Neutral  int `json:"neutral" bson:"neutral"`
	Negative int `json:"negative" bson:"negative"`
	Total    int `json:"total" bson:"total"`
}

type BrandNarrativePillar struct {
	Score              int              `json:"score" bson:"score"`
	Evidence           []string         `json:"evidence" bson:"evidence"`
	Sentiment          SentimentBreakdown `json:"sentiment" bson:"sentiment"`
	NarrativeSummary   string           `json:"narrative_summary" bson:"narrativeSummary"`
	NarrativeCoherence int              `json:"narrative_coherence" bson:"narrativeCoherence"` // 0-100
	KeyThemes          []string         `json:"key_themes" bson:"keyThemes"`
	BrandMentions      []BrandMention   `json:"brand_mentions" bson:"brandMentions"`
}

// Unified Video Authority Result — 4-pillar report
type VideoAuthorityResult struct {
	OverallScore        int                       `json:"overall_score" bson:"overallScore"`
	TranscriptAuthority TranscriptAuthorityPillar `json:"transcript_authority" bson:"transcriptAuthority"`
	TopicalDominance    TopicalDominancePillar    `json:"topical_dominance" bson:"topicalDominance"`
	CitationNetwork     CitationNetworkPillar     `json:"citation_network" bson:"citationNetwork"`
	BrandNarrative      BrandNarrativePillar      `json:"brand_narrative" bson:"brandNarrative"`
	VideoScorecards     []VideoScorecard          `json:"video_scorecards" bson:"videoScorecards"`
	ExecutiveSummary    string                    `json:"executive_summary" bson:"executiveSummary"`
	ConfidenceNote      string                    `json:"confidence_note" bson:"confidenceNote"`
	Recommendations     []VideoRecommendation     `json:"recommendations" bson:"recommendations"`
}

// Top-level video analysis document
type VideoAnalysis struct {
	ID               primitive.ObjectID   `json:"id" bson:"_id,omitempty"`
	TenantID         string               `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	Domain           string               `json:"domain" bson:"domain"`
	Config           VideoAnalysisConfig  `json:"config" bson:"config"`
	Videos           []YouTubeVideo       `json:"videos" bson:"videos"`
	Result           *VideoAuthorityResult `json:"result,omitempty" bson:"result,omitempty"`
	RawText          string               `json:"raw_text" bson:"rawText"`
	Model            string               `json:"model" bson:"model"`
	BrandContextUsed bool                 `json:"brand_context_used" bson:"brandContextUsed"`
	GeneratedAt      time.Time            `json:"generated_at" bson:"generatedAt"`
}

type VideoAnalysisSummary struct {
	ID           primitive.ObjectID `json:"id" bson:"_id"`
	Domain       string             `json:"domain" bson:"domain"`
	OverallScore *int               `json:"overall_score,omitempty"`
	VideoCount   int                `json:"video_count"`
	Model        string             `json:"model" bson:"model"`
	GeneratedAt  time.Time          `json:"generated_at" bson:"generatedAt"`
}

// Video assessment (Phase 1 of two-phase pipeline)

type VideoAssessment struct {
	VideoID          string   `json:"video_id" bson:"video_id"`
	Title            string   `json:"title" bson:"title"`
	KeywordAlignment int      `json:"keyword_alignment" bson:"keyword_alignment"`
	Quotability      int      `json:"quotability" bson:"quotability"`
	InfoDensity      int      `json:"info_density" bson:"info_density"`
	KeyQuotes        []string `json:"key_quotes" bson:"key_quotes"`
	Topics           []string `json:"topics" bson:"topics"`
	BrandSentiment   string   `json:"brand_sentiment" bson:"brand_sentiment"`
	Summary          string   `json:"summary" bson:"summary"`
	HasTranscript    bool     `json:"has_transcript" bson:"has_transcript"`
}

// Domain sharing

type DomainShare struct {
	ID         primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TenantID   string             `json:"tenant_id" bson:"tenantId"`
	Domain     string             `json:"domain" bson:"domain"`
	ShareID    string             `json:"share_id" bson:"shareId"`
	Visibility string             `json:"visibility" bson:"visibility"` // "private", "public", "popular"
	CreatedAt  time.Time          `json:"created_at" bson:"createdAt"`
	UpdatedAt  time.Time          `json:"updated_at" bson:"updatedAt"`
}

// YouTube API cache

type YouTubeCache struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	CacheKey  string             `json:"cache_key" bson:"cacheKey"`
	Data      string             `json:"data" bson:"data"`
	CachedAt  time.Time          `json:"cached_at" bson:"cachedAt"`
	ExpiresAt time.Time          `json:"expires_at" bson:"expiresAt"`
}
