import { useState, useRef, useCallback, useEffect, useMemo } from 'react'
import { apiFetch } from './apiFetch'
import { fetchCurrentUser, fetchUnreadCount, clearAuth, type UserInfo } from './auth'

interface CrawledPage {
  url: string
  title: string
}

interface Question {
  question: string
  relevance: string
  category: string
  page_urls?: string[]
  brand_status?: string
}

interface AnalysisResult {
  site_summary: string
  questions: Question[]
  crawled_pages?: CrawledPage[]
}

interface AnalysisSummary {
  id: string
  domain: string
  site_summary: string
  question_count: number
  page_count: number
  model: string
  brand_context_used: boolean
  brand_profile_updated_at?: string
  created_at: string
}

interface ResultMeta {
  id: string
  model: string
  createdAt: string
  cached: boolean
  brandContextUsed: boolean
  brandProfileUpdatedAt?: string
}

interface DimensionScore {
  score: number
  evidence: string[]
  improvements: string[]
}

interface Competitor {
  domain: string
  score_estimate: number
  strengths: string
}

interface OptRecommendation {
  priority: string
  action: string
  expected_impact: string
  dimension: string
}

interface OptimizationResult {
  overall_score: number
  content_authority: DimensionScore
  structural_optimization: DimensionScore
  source_authority: DimensionScore
  knowledge_persistence: DimensionScore
  competitors: Competitor[]
  recommendations: OptRecommendation[]
}

interface OptimizationMeta {
  id: string
  model: string
  createdAt: string
  cached: boolean
  question: string
  questionIndex: number
  brandStatus?: string
  brandContextUsed: boolean
  brandProfileUpdatedAt?: string
}

interface TodoItem {
  id: string
  optimization_id: string
  analysis_id: string
  video_analysis_id?: string
  source_type?: string
  domain: string
  question: string
  action: string
  summary?: string
  expected_impact: string
  dimension: string
  priority: string
  status: 'todo' | 'completed' | 'backlogged' | 'archived'
  created_at: string
  completed_at?: string
  backlogged_at?: string
  archived_at?: string
}

type TodoSubTab = 'todo' | 'completed' | 'backlogged' | 'archived'
type TodoSortMode = 'priority' | 'question'

interface OptimizationListItem {
  id: string
  domain: string
  question: string
  question_index: number
  overall_score: number
  model: string
  public: boolean
  brand_status?: string
  brand_context_used: boolean
  brand_profile_updated_at?: string
  created_at: string
}

interface FullOptimization {
  id: string
  analysis_id: string
  question_index: number
  question: string
  domain: string
  page_urls: string[]
  result: OptimizationResult
  raw_text: string
  model: string
  public: boolean
  brand_status?: string
  brand_context_used: boolean
  brand_profile_updated_at?: string
  created_at: string
}

interface SummaryTheme {
  title: string
  description: string
  report_refs: string[]
}
interface SummaryActionItem {
  priority: string
  action: string
  dimension: string
  expected_impact: string
  source_reports: string[]
}
interface SummaryContradiction {
  topic: string
  positions: string[]
  report_refs: string[]
  recommendation: string
}
interface DomainSummaryResult {
  executive_summary: string
  average_score: number
  score_range: [number, number]
  themes: SummaryTheme[]
  action_items: SummaryActionItem[]
  contradictions: SummaryContradiction[]
  dimension_trends: Record<string, number>
}
interface DomainSummary {
  id: string
  domain: string
  result: DomainSummaryResult
  model: string
  optimization_ids: string[]
  report_count: number
  generated_at: string
}
interface DomainSummaryStatus {
  exists: boolean
  generated_at?: string
  included_count?: number
  total_report_count: number
  newer_report_count?: number
  stale?: boolean
}

interface HealthTimelineRecord {
  id: string
  models: { model: string; name: string; status: string; latency_ms?: number }[]
  checked_at: string
}

// Brand Intelligence types

interface BrandCompetitor {
  name: string
  url: string
  relationship: string
  notes: string
}

interface TargetQuery {
  query: string
  priority: string
  type: string
}

interface KeyMessage {
  claim: string
  evidence_url: string
  priority: string
}

interface BrandPresence {
  youtube_url: string
  subreddits: string[]
  review_site_urls: string[]
  social_profiles: string[]
  content_assets: string[]
  podcasts: string[]
}

interface BrandProfile {
  id: string
  domain: string
  brand_name: string
  description: string
  categories: string[]
  products: string[]
  primary_audience: string
  key_use_cases: string[]
  competitors: BrandCompetitor[]
  target_queries: TargetQuery[]
  key_messages: KeyMessage[]
  differentiators: string[]
  presence: BrandPresence
  presence_complete: boolean
  public: boolean
  last_discovery_at?: string
  created_at: string
  updated_at: string
}

interface BrandProfileSummary {
  id: string
  domain: string
  brand_name: string
  competitor_count: number
  query_count: number
  completeness: number
  public: boolean
  updated_at: string
}

interface DiscoveredCompetitor {
  name: string
  url: string
  relationship: string
  source: string
  confidence: number
  notes?: string
}

interface SuggestedQuery {
  query: string
  priority: string
  type: string
}

interface SuggestedClaim {
  claim: string
  evidence_url: string
  priority: string
}

type SuggestedDifferentiator = string

// Video Authority types

interface YouTubeVideo {
  video_id: string
  title: string
  channel_title: string
  channel_id: string
  description: string
  published_at: string
  view_count: number
  like_count: number
  comment_count: number
  duration: string
  tags: string[]
  transcript?: string
  relevance_tag: string
}

interface VideoRecommendation {
  action: string
  expected_impact: string
  dimension: string
  priority: string
  video_id?: string
}

interface VideoScorecard {
  video_id: string
  title: string
  overall_score: number
  transcript_power: number
  structural_extractability: number
  discovery_surface: number
  has_transcript: boolean
  key_findings: string[]
}

interface BrandMention {
  video_id: string
  title: string
  channel_title: string
  view_count: number
  sentiment: string
  mention_context: string
  mention_position: string
  extractability: string
  competitors_mentioned: string[]
}

interface CreatorProfile {
  channel_title: string
  channel_id: string
  subscriber_count: number
  sentiment: string
  video_count: number
  total_views: number
  role: string
  authority_score: number
}

interface ShareOfVoiceEntry {
  brand_name: string
  mention_count: number
  percentage: number
}

interface ContentGap {
  query: string
  competitors_mentioned: string[]
  opportunity_score: number
  video_count: number
  recommendation: string
}

interface CreatorTarget {
  channel_title: string
  channel_id: string
  subscriber_count: number
  category_relevance: string
  competitors_mentioned: string[]
  outreach_reason: string
}

interface TranscriptAuthorityPillar {
  score: number
  evidence: string[]
  transcript_coverage: number
  keyword_alignment: number
  quotability_score: number
  information_density: number
}

interface TopicalDominancePillar {
  score: number
  evidence: string[]
  topics_covered: number
  topics_total: number
  coverage_depth: number
  vs_competitors: string
  share_of_voice: ShareOfVoiceEntry[]
  content_gaps: ContentGap[]
}

interface CitationNetworkPillar {
  score: number
  evidence: string[]
  creator_mentions: number
  authoritative_refs: number
  concentration_risk: string
  top_creators: CreatorProfile[]
  creator_targets: CreatorTarget[]
}

interface SentimentBreakdown {
  positive: number
  neutral: number
  negative: number
  total: number
}

interface BrandNarrativePillar {
  score: number
  evidence: string[]
  sentiment: SentimentBreakdown
  narrative_summary: string
  narrative_coherence: number
  key_themes: string[]
  brand_mentions: BrandMention[]
}

interface VideoAuthorityResult {
  overall_score: number
  transcript_authority: TranscriptAuthorityPillar
  topical_dominance: TopicalDominancePillar
  citation_network: CitationNetworkPillar
  brand_narrative: BrandNarrativePillar
  video_scorecards: VideoScorecard[]
  executive_summary: string
  confidence_note: string
  recommendations: VideoRecommendation[]
}

interface VideoAnalysis {
  id: string
  domain: string
  config: { channel_url: string; video_urls: string[]; brand_url: string; search_terms: string[] }
  videos: YouTubeVideo[]
  result?: VideoAuthorityResult
  model: string
  brand_context_used: boolean
  generated_at: string
}

interface VideoAnalysisSummary {
  id: string
  domain: string
  overall_score?: number
  video_count: number
  model: string
  generated_at: string
}

interface VideoDetail {
  video_id: string
  title: string
  transcript: string
  transcript_length: number
  assessment: {
    keyword_alignment: number
    quotability: number
    info_density: number
    key_quotes: string[]
    topics: string[]
    brand_sentiment: string
    summary: string
  } | null
}

type VideoView = 'input' | 'discovering' | 'review' | 'running' | 'results' | 'transcripts'

type OptimizeView = 'list' | 'detail' | 'running' | 'summary' | 'generating-summary'

interface ModelStatus {
  model: string
  name: string
  status: 'available' | 'overloaded' | 'error'
  latency_ms?: number
  http_status?: number
  error?: string
}

interface HealthCheck {
  models: ModelStatus[]
  checked_at: string
}

type AppState = 'idle' | 'analyzing' | 'done' | 'error'
type ActiveTab = 'analyze' | 'status' | 'optimize' | 'todos' | 'brand' | 'video'

const CATEGORY_COLORS = [
  'bg-primary-500/20 text-primary-300 border-primary-500/30',
  'bg-accent-purple/20 text-purple-300 border-accent-purple/30',
  'bg-accent-cyan/20 text-cyan-300 border-accent-cyan/30',
  'bg-accent-emerald/20 text-emerald-300 border-accent-emerald/30',
  'bg-amber-500/20 text-amber-300 border-amber-500/30',
  'bg-accent-pink/20 text-pink-300 border-accent-pink/30',
  'bg-sky-500/20 text-sky-300 border-sky-500/30',
  'bg-rose-500/20 text-rose-300 border-rose-500/30',
]

function safePathname(urlStr: string): string {
  try {
    const parsed = new URL(urlStr)
    return parsed.pathname === '/' ? urlStr : parsed.pathname
  } catch {
    return urlStr
  }
}

/** Format a date string as "Mon DD, h:mm AM" */
function fmtDate(d: string) {
  return new Date(d).toLocaleDateString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' })
}

/** Extract the hostname from a domain/URL string for grouping and comparison */
function domainKey(input: string): string {
  if (!input) return ''
  let s = input.trim().toLowerCase()
  s = s.replace(/^https?:\/\//, '').replace(/\/.*$/, '').replace(/:\d+$/, '')
  return s
}

/** Normalize a URL: add https:// if missing, strip trailing slash */
function normalizeUrl(input: string): string {
  let s = input.trim().toLowerCase()
  if (!s) return ''
  s = s.replace(/^https?:\/\//, '')
  s = s.replace(/\/+$/, '')
  return s
}

export default function App() {
  const [url, setUrl] = useState('')
  const [state, setState] = useState<AppState>('idle')
  const [statusMessages, setStatusMessages] = useState<string[]>([])
  const [result, setResult] = useState<AnalysisResult | null>(null)
  const [error, setError] = useState('')
  const abortRef = useRef<AbortController | null>(null)

  const [activeTab, setActiveTab] = useState<ActiveTab>(() => {
    const params = new URLSearchParams(window.location.search)
    const tab = params.get('tab')
    const validTabs: ActiveTab[] = ['analyze', 'status', 'optimize', 'todos', 'brand', 'video']
    if (tab && validTabs.includes(tab as ActiveTab)) return tab as ActiveTab
    return 'analyze'
  })
  const [selectedDomain, setSelectedDomain] = useState('')
  const [history, setHistory] = useState<AnalysisSummary[]>([])

  const [resultMeta, setResultMeta] = useState<ResultMeta | null>(null)
  const [forceAnalyze, setForceAnalyze] = useState(false)

  const [optimization, setOptimization] = useState<OptimizationResult | null>(null)
  const [optimizationMeta, setOptimizationMeta] = useState<OptimizationMeta | null>(null)
  const [optimizing, setOptimizing] = useState(false)
  const [optimizeAutoArchive, setOptimizeAutoArchive] = useState(true)
  const [optimizeMessages, setOptimizeMessages] = useState<string[]>([])
  const [optimizeError, setOptimizeError] = useState('')
  const [optScores, setOptScores] = useState<Record<number, number>>({})
  const optimizeAbortRef = useRef<AbortController | null>(null)

  const [optList, setOptList] = useState<OptimizationListItem[]>([])
  const [optListLoading, setOptListLoading] = useState(false)
  const [optimizeView, setOptimizeView] = useState<OptimizeView>('list')
  const [selectedOpt, setSelectedOpt] = useState<FullOptimization | null>(null)
  const [optTodos, setOptTodos] = useState<TodoItem[]>([])

  // Domain summary state
  const [summaryStatuses, setSummaryStatuses] = useState<Record<string, DomainSummaryStatus>>({})
  const [activeSummary, setActiveSummary] = useState<DomainSummary | null>(null)
  const [activeSummaryStale, setActiveSummaryStale] = useState(false)
  const [activeSummaryDomain, setActiveSummaryDomain] = useState('')
  const [generatingSummary, setGeneratingSummary] = useState(false)
  const [summaryMessages, setSummaryMessages] = useState<string[]>([])

  const [healthHistory, setHealthHistory] = useState<HealthCheck[]>([])
  const [healthChecking, setHealthChecking] = useState(false)

  const [todos, setTodos] = useState<TodoItem[]>([])
  const [todosLoading, setTodosLoading] = useState(false)
  const [todoSubTab, setTodoSubTab] = useState<TodoSubTab>('todo')
  const [todoSortMode, setTodoSortMode] = useState<TodoSortMode>('priority')
  const [expandedTodos, setExpandedTodos] = useState<Set<string>>(new Set())

  const [healthTimeline, setHealthTimeline] = useState<HealthTimelineRecord[]>([])
  const [healthTimelineLoading, setHealthTimelineLoading] = useState(false)

  // Domain sharing state
  const [shareModalDomain, setShareModalDomain] = useState<string | null>(null)
  const [domainShareState, setDomainShareState] = useState<{ visibility: string; share_id: string; share_url: string } | null>(null)
  const [shareLoading, setShareLoading] = useState(false)
  const [shareCopied, setShareCopied] = useState(false)
  const [popularDomains, setPopularDomains] = useState<{ domain: string; brand_name: string; share_id: string; avg_score: number; report_count: number }[]>([])

  // Shared view mode (read-only /share/{shareId} URL)
  const [sharedMode, setSharedMode] = useState(false)
  const [sharedLoading, setSharedLoading] = useState(false)
  const [sharedNotFound, setSharedNotFound] = useState(false)
  const sharedModeRef = useRef(false)
  const [sharedOptimizations, setSharedOptimizations] = useState<FullOptimization[]>([])

  // Modals for optimize question click + auth/subscription gating
  const [optimizeConfirmQ, setOptimizeConfirmQ] = useState<number | null>(null) // question index for confirmation
  const [readOnlyOptModal, setReadOnlyOptModal] = useState<string | null>(null) // brand name for "not ready" modal
  const [subscriptionModal, setSubscriptionModal] = useState(false)
  const [loginModal, setLoginModal] = useState(false)

  // Brand Intelligence state
  const [brandProfile, setBrandProfile] = useState<BrandProfile | null>(null)
  const [brandList, setBrandList] = useState<BrandProfileSummary[]>([])
  const [, setBrandLoading] = useState(false)
  const [brandSaving, setBrandSaving] = useState(false)
  const [brandDomain, setBrandDomain] = useState('')
  const [brandEditing, setBrandEditing] = useState(false)
  // Brand form state (editable fields)
  const [brandForm, setBrandForm] = useState({
    brand_name: '',
    description: '',
    categories: '' as string,
    products: '' as string,
    primary_audience: '',
    key_use_cases: '' as string,
    differentiators: '' as string,
  })
  const [brandCompetitors, setBrandCompetitors] = useState<BrandCompetitor[]>([])
  const [brandQueries, setBrandQueries] = useState<TargetQuery[]>([])
  const [brandMessages, setBrandMessages] = useState<KeyMessage[]>([])
  const [brandPresenceForm, setBrandPresenceForm] = useState({
    youtube_url: '', subreddits: '', review_site_urls: '', social_profiles: '', content_assets: '', podcasts: '',
  })
  // Brand auto-discovery state
  const [discovering, setDiscovering] = useState(false)
  const [discoverMessages, setDiscoverMessages] = useState<string[]>([])
  const [discoveredCompetitors, setDiscoveredCompetitors] = useState<DiscoveredCompetitor[]>([])
  const [discoverSelected, setDiscoverSelected] = useState<Set<number>>(new Set())
  const [suggestingQueries, setSuggestingQueries] = useState(false)
  const [suggestMessages, setSuggestMessages] = useState<string[]>([])
  const [suggestedQueries, setSuggestedQueries] = useState<SuggestedQuery[]>([])
  const [suggestSelected, setSuggestSelected] = useState<Set<number>>(new Set())
  const [generatingDesc, setGeneratingDesc] = useState(false)
  const [generateMessages, setGenerateMessages] = useState<string[]>([])
  const [predictingAudience, setPredictingAudience] = useState(false)
  const [predictAudienceMessages, setPredictAudienceMessages] = useState<string[]>([])
  const [suggestingClaims, setSuggestingClaims] = useState(false)
  const [suggestClaimMessages, setSuggestClaimMessages] = useState<string[]>([])
  const [suggestedClaims, setSuggestedClaims] = useState<SuggestedClaim[]>([])
  const [suggestClaimSelected, setSuggestClaimSelected] = useState<Set<number>>(new Set())
  const [predictingDiffs, setPredictingDiffs] = useState(false)
  const [predictDiffMessages, setPredictDiffMessages] = useState<string[]>([])
  const [suggestedDiffs, setSuggestedDiffs] = useState<SuggestedDifferentiator[]>([])
  const [suggestDiffSelected, setSuggestDiffSelected] = useState<Set<number>>(new Set())
  const [brandPresenceComplete, setBrandPresenceComplete] = useState(false)
  const [brandSections, setBrandSections] = useState({ audience: false, competitors: true, queries: false, presence: false })

  const [confirmDeleteBrand, setConfirmDeleteBrand] = useState(false)
  const [confirmDeleteAnalysis, setConfirmDeleteAnalysis] = useState<string | null>(null)
  const [confirmDeleteOptimization, setConfirmDeleteOptimization] = useState<string | null>(null)

  // Video Authority state
  const [videoView, setVideoView] = useState<VideoView>('input')
  const [videoChannelURL, setVideoChannelURL] = useState('')
  const [videoURLs, setVideoURLs] = useState<string[]>([''])
  const [videoBrandURL, setVideoBrandURL] = useState('')
  const [videoSearchTerms, setVideoSearchTerms] = useState<string[]>([])
  const [videoSearchTermInput, setVideoSearchTermInput] = useState('')
  const [videoDiscovering, setVideoDiscovering] = useState(false)
  const [discoveredVideos, setDiscoveredVideos] = useState<YouTubeVideo[]>([])
  const [selectedVideoIds, setSelectedVideoIds] = useState<Set<string>>(new Set())
  const [videoQuotaEstimate, setVideoQuotaEstimate] = useState(0)
  const [videoAnalyzing, setVideoAnalyzing] = useState(false)
  const [videoAutoArchive, setVideoAutoArchive] = useState(true)
  const videoAbortRef = useRef<AbortController | null>(null)
  const [videoMessages, setVideoMessages] = useState<string[]>([])
  const [videoAnalysis, setVideoAnalysis] = useState<VideoAnalysis | null>(null)
  const [videoAnalysisList, setVideoAnalysisList] = useState<VideoAnalysisSummary[]>([])
  // videoResultTab removed — unified report has no tabs
  const [videoDomain, setVideoDomain] = useState('')
  const [confirmDeleteVideoAnalysis, setConfirmDeleteVideoAnalysis] = useState(false)
  const [videoExpandedCards, setVideoExpandedCards] = useState<Set<string>>(new Set())
  const [videoDetails, setVideoDetails] = useState<VideoDetail[]>([])
  const [videoDetailsExpanded, setVideoDetailsExpanded] = useState<Set<string>>(new Set())
  const [videoSettingsOpen, setVideoSettingsOpen] = useState(false)
  // Video review filters
  const [videoFilterTags, setVideoFilterTags] = useState<Set<string>>(new Set(['own', 'direct_mention', 'competitor_comparison', 'category_content']))
  const [videoFilterMinViews, setVideoFilterMinViews] = useState(0)
  const [videoFilterRecency, setVideoFilterRecency] = useState<'30d' | '90d' | '1y' | '3y' | 'all'>('all')

  // SaaS auth state
  const [saasEnabled, setSaasEnabled] = useState(false)
  const [user, setUser] = useState<UserInfo | null>(null)
  const [unreadCount, setUnreadCount] = useState(0)
  const [showCredits, setShowCredits] = useState(false)
  const [tenantCredits, setTenantCredits] = useState(0)
  const [hasActivePlan, setHasActivePlan] = useState(false)

  useEffect(() => {
    apiFetch('/api/config').then(r => r.ok ? r.json() : null).then(data => {
      if (data?.saas_enabled) {
        setSaasEnabled(true)
        fetchCurrentUser().then(setUser)
        fetchUnreadCount().then(setUnreadCount)
        // Fetch credits info
        apiFetch('/api/plans').then(r => r.ok ? r.json() : null).then(plans => {
          if (plans?.plans) {
            const hasCredits = plans.plans.some((p: { usageCreditsPerMonth: number; bonusCredits: number }) => p.usageCreditsPerMonth > 0 || p.bonusCredits > 0)
            setShowCredits(hasCredits)
            setTenantCredits((plans.tenantSubscriptionCredits || 0) + (plans.tenantPurchasedCredits || 0))
            setHasActivePlan(plans.billingStatus === 'active' || plans.billingWaived === true)
          }
        }).catch(() => {})
      }
    }).catch(() => {})
  }, [])

  // Brand dirty tracking — flag-based, set only on user/AI changes after load
  const brandDirtyRef = useRef(false)
  const brandLoadingRef = useRef(false)
  const [pendingNavAction, setPendingNavAction] = useState<(() => void) | null>(null)

  // Effect: detect state changes after loading. If brandLoadingRef is set, this is
  // from a load/reset (not user edits), so clear dirty. Otherwise mark dirty.
  useEffect(() => {
    if (!brandEditing) return
    if (brandLoadingRef.current) {
      brandLoadingRef.current = false
      brandDirtyRef.current = false
      return
    }
    brandDirtyRef.current = true
  }, [brandEditing, brandForm, brandCompetitors, brandQueries, brandMessages, brandPresenceForm, brandPresenceComplete])

  const isBrandDirty = useCallback(() => {
    return brandEditing && brandDirtyRef.current
  }, [brandEditing])

  // Navigation guard — wraps any action that would leave the brand editor
  // Skip in shared mode — brand is read-only, no unsaved changes possible
  const guardBrandNav = useCallback((action: () => void) => {
    if (sharedModeRef.current || !isBrandDirty()) {
      action()
    } else {
      setPendingNavAction(() => action)
    }
  }, [isBrandDirty])

  const fetchHistory = useCallback(async () => {
    try {
      const response = await apiFetch('/api/analyses')
      if (response.ok) {
        const data = await response.json()
        setHistory(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch history:', err)
    }
  }, [])

  const checkHealth = useCallback(async () => {
    setHealthChecking(true)
    try {
      const response = await apiFetch('/api/health/claude')
      if (response.ok) {
        const data = await response.json() as HealthCheck
        setHealthHistory(prev => [data, ...prev].slice(0, 50))
      }
    } catch (err) {
      setHealthHistory(prev => [{
        models: [
          { model: 'claude-sonnet-4-6', name: 'Sonnet 4.6', status: 'error' as const, error: (err as Error).message },
          { model: 'claude-haiku-4-5-20251001', name: 'Haiku 4.5', status: 'error' as const, error: (err as Error).message },
        ],
        checked_at: new Date().toISOString(),
      }, ...prev].slice(0, 50))
    } finally {
      setHealthChecking(false)
    }
  }, [])

  const fetchTodos = useCallback(async () => {
    setTodosLoading(true)
    try {
      const response = await apiFetch('/api/todos')
      if (response.ok) {
        const data = await response.json()
        setTodos(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch todos:', err)
    } finally {
      setTodosLoading(false)
    }
  }, [])

  const fetchOptList = useCallback(async () => {
    setOptListLoading(true)
    try {
      const response = await apiFetch('/api/optimizations')
      if (response.ok) {
        const data = await response.json()
        setOptList(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch optimizations:', err)
    } finally {
      setOptListLoading(false)
    }
  }, [])

  // All unique domains across the app, sorted by most recent activity
  const allDomains = useMemo((): { domain: string; latestDate: string }[] => {
    // Keyed by domainKey() to deduplicate variations like https://x.com vs x.com
    const domainMap: Record<string, { display: string; latestDate: string }> = {}
    const track = (domain: string, date: string) => {
      const key = domainKey(domain)
      if (!key) return
      const existing = domainMap[key]
      if (!existing || new Date(date) > new Date(existing.latestDate)) {
        domainMap[key] = { display: domain, latestDate: date }
      }
    }
    for (const h of history) track(h.domain, h.created_at)
    for (const o of optList) track(o.domain, o.created_at)
    for (const b of brandList) track(b.domain, b.updated_at)
    for (const v of videoAnalysisList) track(v.domain, v.generated_at)
    for (const t of todos) track(t.domain, t.created_at)
    return Object.entries(domainMap)
      .map(([, v]) => ({ domain: v.display, latestDate: v.latestDate }))
      .sort((a, b) => new Date(b.latestDate).getTime() - new Date(a.latestDate).getTime())
  }, [history, optList, brandList, videoAnalysisList, todos])

  const fetchSummaryStatuses = useCallback(async (domains: string[]) => {
    const statuses: Record<string, DomainSummaryStatus> = {}
    await Promise.all(domains.map(async (domain) => {
      try {
        const res = await apiFetch(`/api/domains/${encodeURIComponent(domain)}/summary/status`)
        if (res.ok) statuses[domain] = await res.json()
      } catch { /* ignore */ }
    }))
    setSummaryStatuses(statuses)
  }, [])

  // Fetch summary statuses when optList changes
  useEffect(() => {
    const domains = [...new Set(optList.map(o => o.domain))]
    if (domains.length > 0) fetchSummaryStatuses(domains)
  }, [optList, fetchSummaryStatuses])

  const handleSummaryClick = useCallback(async (domain: string) => {
    setActiveSummaryDomain(domain)
    const status = summaryStatuses[domain]
    if (status?.exists) {
      try {
        const res = await apiFetch(`/api/domains/${encodeURIComponent(domain)}/summary`)
        if (res.ok) {
          const data = await res.json()
          setActiveSummary(data.summary)
          setActiveSummaryStale(data.stale)
          setOptimizeView('summary')
          return
        }
      } catch { /* fall through */ }
    }
    setActiveSummary(null)
    setActiveSummaryStale(false)
    setOptimizeView('summary')
  }, [summaryStatuses])

  const deleteAnalysis = useCallback(async (id: string) => {
    try {
      const response = await apiFetch(`/api/analyses/${id}`, { method: 'DELETE' })
      if (response.ok) {
        fetchHistory()
        fetchOptList()
        fetchTodos()
        if (resultMeta?.id === id) {
          setResultMeta(null)
          setResult(null)
          setState('idle')
        }
      }
    } catch (err) {
      console.error('Failed to delete analysis:', err)
    }
    setConfirmDeleteAnalysis(null)
  }, [fetchHistory, fetchOptList, fetchTodos, resultMeta])

  const deleteOptimization = useCallback(async (id: string) => {
    try {
      const response = await apiFetch(`/api/optimizations/${id}`, { method: 'DELETE' })
      if (response.ok) {
        fetchOptList()
        fetchTodos()
        setSelectedOpt(null)
        setOptimizeView('list')
      }
    } catch (err) {
      console.error('Failed to delete optimization:', err)
    }
    setConfirmDeleteOptimization(null)
  }, [fetchOptList, fetchTodos])

  const loadOptimizationDetail = useCallback(async (id: string) => {
    // In shared mode, use cached full optimization data instead of authenticated API call
    if (sharedModeRef.current) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cached = sharedOptimizations.find((o: any) => o.id === id)
      if (cached) {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const data = cached as any
        setSelectedOpt({
          id: data.id,
          analysis_id: data.analysis_id || '',
          question_index: data.question_index ?? -1,
          question: data.question || '',
          domain: data.domain || '',
          page_urls: data.page_urls || [],
          result: data.result || {},
          raw_text: '',
          model: data.model || '',
          public: data.public || false,
          brand_status: data.brand_status || '',
          brand_context_used: data.brand_context_used || false,
          brand_profile_updated_at: data.brand_profile_updated_at,
          created_at: data.created_at || '',
        } as FullOptimization)
        setOptimization(data.result)
        setOptimizationMeta({
          id: data.id,
          model: data.model || '',
          createdAt: data.created_at || '',
          cached: false,
          question: data.question || '',
          questionIndex: data.question_index ?? -1,
          brandStatus: data.brand_status || undefined,
          brandContextUsed: data.brand_context_used || false,
          brandProfileUpdatedAt: data.brand_profile_updated_at || undefined,
        })
        // Use todos from shared data for this optimization
        setOptTodos(todos.filter(t => t.optimization_id === id))
        setOptimizeView('detail')
      }
      return
    }
    try {
      const [optRes, todosRes] = await Promise.all([
        apiFetch(`/api/optimizations/${id}`),
        apiFetch(`/api/todos?optimization_id=${id}`),
      ])
      if (optRes.ok) {
        const data = await optRes.json() as FullOptimization
        setSelectedOpt(data)
        setOptimization(data.result)
        setOptimizationMeta({
          id: data.id,
          model: data.model,
          createdAt: data.created_at,
          cached: false,
          question: data.question,
          questionIndex: data.question_index,
          brandStatus: data.brand_status || undefined,
          brandContextUsed: data.brand_context_used || false,
          brandProfileUpdatedAt: data.brand_profile_updated_at || undefined,
        })
      }
      if (todosRes.ok) {
        const todosData = await todosRes.json()
        setOptTodos(todosData || [])
      }
      setOptimizeView('detail')
    } catch (err) {
      console.error('Failed to load optimization:', err)
    }
  }, [sharedOptimizations, todos])

  const fetchPopularDomains = useCallback(async () => {
    try {
      const response = await apiFetch('/api/share/popular')
      if (response.ok) {
        const data = await response.json()
        setPopularDomains(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch popular domains:', err)
    }
  }, [])

  const fetchDomainShare = useCallback(async (domain: string) => {
    setShareLoading(true)
    setShareCopied(false)
    try {
      const response = await apiFetch(`/api/domains/${encodeURIComponent(domain)}/share`)
      if (response.ok) {
        const data = await response.json()
        setDomainShareState(data)
      }
    } catch (err) {
      console.error('Failed to fetch domain share:', err)
    } finally {
      setShareLoading(false)
    }
  }, [])

  const setDomainVisibility = useCallback(async (domain: string, visibility: string) => {
    setShareLoading(true)
    try {
      const response = await apiFetch(`/api/domains/${encodeURIComponent(domain)}/share`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ visibility }),
      })
      if (response.ok) {
        const data = await response.json()
        setDomainShareState(data)
        setShareCopied(false)
        if (visibility === 'popular' || visibility === 'private') {
          fetchPopularDomains()
        }
      }
    } catch (err) {
      console.error('Failed to set domain visibility:', err)
    } finally {
      setShareLoading(false)
    }
  }, [fetchPopularDomains])

  const fetchHealthTimeline = useCallback(async (hours = 24) => {
    setHealthTimelineLoading(true)
    try {
      const response = await apiFetch(`/api/health/history?hours=${hours}`)
      if (response.ok) {
        const data = await response.json()
        setHealthTimeline(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch health timeline:', err)
    } finally {
      setHealthTimelineLoading(false)
    }
  }, [])

  // Brand Intelligence callbacks
  const emptyPresence: BrandPresence = { youtube_url: '', subreddits: [], review_site_urls: [], social_profiles: [], content_assets: [], podcasts: [] }

  const fetchBrandList = useCallback(async () => {
    setBrandLoading(true)
    try {
      const response = await apiFetch('/api/brands')
      if (response.ok) {
        const data = await response.json()
        setBrandList(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch brands:', err)
    } finally {
      setBrandLoading(false)
    }
  }, [])

  const loadBrandProfile = useCallback(async (domain: string) => {
    setBrandLoading(true)
    try {
      const response = await apiFetch(`/api/brands/${encodeURIComponent(domain)}`)
      if (response.ok) {
        const data = await response.json() as BrandProfile
        setBrandProfile(data)
        setBrandDomain(data.domain)
        setBrandForm({
          brand_name: data.brand_name || '',
          description: data.description || '',
          categories: (data.categories || []).join(', '),
          products: (data.products || []).join(', '),
          primary_audience: data.primary_audience || '',
          key_use_cases: (data.key_use_cases || []).join(', '),
          differentiators: (data.differentiators || []).join(', '),
        })
        setBrandCompetitors(data.competitors || [])
        setBrandQueries(data.target_queries || [])
        setBrandMessages(data.key_messages || [])
        const p = data.presence || emptyPresence
        setBrandPresenceForm({
          youtube_url: p.youtube_url || '',
          subreddits: (p.subreddits || []).join(', '),
          review_site_urls: (p.review_site_urls || []).join(', '),
          social_profiles: (p.social_profiles || []).join(', '),
          content_assets: (p.content_assets || []).join(', '),
          podcasts: (p.podcasts || []).join(', '),
        })
        setBrandPresenceComplete(data.presence_complete || false)
        brandLoadingRef.current = true
        setBrandEditing(true)
        return true
      } else if (response.status === 404) {
        return false
      }
    } catch (err) {
      console.error('Failed to load brand:', err)
    } finally {
      setBrandLoading(false)
    }
    return false
  }, [])

  const saveBrandProfile = useCallback(async () => {
    if (!brandDomain.trim()) return
    setBrandSaving(true)
    try {
      const splitTrim = (s: string) => s.split(',').map(x => x.trim()).filter(Boolean)
      const body: Partial<BrandProfile> = {
        brand_name: brandForm.brand_name,
        description: brandForm.description,
        categories: splitTrim(brandForm.categories),
        products: splitTrim(brandForm.products),
        primary_audience: brandForm.primary_audience,
        key_use_cases: splitTrim(brandForm.key_use_cases),
        competitors: brandCompetitors,
        target_queries: brandQueries,
        key_messages: brandMessages,
        differentiators: splitTrim(brandForm.differentiators),
        presence: {
          youtube_url: brandPresenceForm.youtube_url,
          subreddits: splitTrim(brandPresenceForm.subreddits),
          review_site_urls: splitTrim(brandPresenceForm.review_site_urls),
          social_profiles: splitTrim(brandPresenceForm.social_profiles),
          content_assets: splitTrim(brandPresenceForm.content_assets),
          podcasts: splitTrim(brandPresenceForm.podcasts),
        },
        presence_complete: brandPresenceComplete,
      }
      const response = await apiFetch(`/api/brands/${encodeURIComponent(brandDomain.trim())}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (response.ok) {
        const saved = await response.json() as BrandProfile
        setBrandProfile(saved)
        fetchBrandList()
        brandDirtyRef.current = false
      }
    } catch (err) {
      console.error('Failed to save brand:', err)
    } finally {
      setBrandSaving(false)
    }
  }, [brandDomain, brandForm, brandCompetitors, brandQueries, brandMessages, brandPresenceForm, brandPresenceComplete, fetchBrandList])

  const startNewBrand = useCallback((domain?: string) => {
    brandLoadingRef.current = true
    setBrandProfile(null)
    setBrandDomain(domain || url || '')
    setBrandForm({ brand_name: '', description: '', categories: '', products: '', primary_audience: '', key_use_cases: '', differentiators: '' })
    setBrandCompetitors([])
    setBrandQueries([])
    setBrandMessages([])
    setBrandPresenceForm({ youtube_url: '', subreddits: '', review_site_urls: '', social_profiles: '', content_assets: '', podcasts: '' })
    setBrandPresenceComplete(false)
    setDiscoveredCompetitors([])
    setSuggestedQueries([])
    setSuggestedClaims([])
    setSuggestedDiffs([])
    setBrandEditing(true)
  }, [url])

  // SSE helper for brand streaming endpoints
  const brandSSE = useCallback(async (
    url: string,
    setMessages: React.Dispatch<React.SetStateAction<string[]>>,
    setRunning: React.Dispatch<React.SetStateAction<boolean>>,
    onDone: (result: string) => void,
  ) => {
    setMessages([])
    setRunning(true)
    try {
      const response = await apiFetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({}),
      })
      if (!response.ok || !response.body) throw new Error(`HTTP ${response.status}`)

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        while (true) {
          const idx = buffer.indexOf('\n\n')
          if (idx === -1) break
          const message = buffer.slice(0, idx)
          buffer = buffer.slice(idx + 2)
          let eventType = '', data = ''
          for (const line of message.split('\n')) {
            if (line.startsWith('event: ')) eventType = line.slice(7)
            else if (line.startsWith('data: ')) data = line.slice(6)
          }
          if (!data || !eventType) continue
          try {
            const parsed = JSON.parse(data)
            if (eventType === 'status') setMessages(prev => [...prev, parsed.message])
            else if (eventType === 'done') { onDone(parsed.result); setRunning(false); return }
            else if (eventType === 'error') { setMessages(prev => [...prev, 'Error: ' + parsed.message]); setRunning(false); return }
          } catch { /* skip malformed */ }
        }
      }
    } catch (err) {
      setMessages(prev => [...prev, 'Connection failed: ' + (err as Error).message])
    } finally {
      setRunning(false)
    }
  }, [])

  const generateSummary = useCallback(async (domain: string) => {
    setActiveSummaryDomain(domain)
    setGeneratingSummary(true)
    setSummaryMessages([])
    setOptimizeView('generating-summary')

    await brandSSE(
      `/api/domains/${encodeURIComponent(domain)}/summary`,
      setSummaryMessages,
      setGeneratingSummary,
      async (resultStr: string) => {
        try {
          const parsed = JSON.parse(resultStr) as DomainSummaryResult
          const res = await apiFetch(`/api/domains/${encodeURIComponent(domain)}/summary`)
          if (res.ok) {
            const data = await res.json()
            setActiveSummary(data.summary)
            setActiveSummaryStale(false)
          } else {
            setActiveSummary({ id: '', domain, result: parsed, model: '', optimization_ids: [], report_count: 0, generated_at: new Date().toISOString() })
            setActiveSummaryStale(false)
          }
        } catch {
          setActiveSummary(null)
        }
        setOptimizeView('summary')
        fetchSummaryStatuses([...new Set(optList.map(o => o.domain))])
      },
    )
  }, [brandSSE, fetchSummaryStatuses, optList])

  // ── Video Authority callbacks ──

  const fetchVideoAnalysisList = useCallback(async () => {
    try {
      const res = await apiFetch('/api/video/analyses')
      if (res.ok) {
        const data = await res.json()
        setVideoAnalysisList(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch video analyses:', err)
    }
  }, [])

  const loadVideoAnalysis = useCallback(async (domain: string) => {
    try {
      const res = await apiFetch(`/api/video/analyses/${encodeURIComponent(domain)}`)
      if (res.ok) {
        const data = await res.json() as VideoAnalysis
        setVideoAnalysis(data)
        setVideoDomain(domain)
        setVideoView('results')
      }
    } catch (err) {
      console.error('Failed to load video analysis:', err)
    }
  }, [])

  const prepopulateFromBrand = useCallback((domain: string) => {
    setVideoDomain(domain)
    setVideoBrandURL(domain)
    // Find brand profile
    const brand = brandList.find(b => b.domain === domain)
    if (brand) {
      // Load full profile for presence data
      apiFetch(`/api/brands/${encodeURIComponent(domain)}`)
        .then(res => res.ok ? res.json() : null)
        .then((profile: BrandProfile | null) => {
          if (profile) {
            if (profile.presence?.youtube_url) setVideoChannelURL(profile.presence.youtube_url)
            if (profile.target_queries?.length) {
              setVideoSearchTerms(profile.target_queries.map(q => q.query))
            }
            // Collapse settings since they're now populated
            setVideoSettingsOpen(false)
          }
        })
        .catch(() => {})
    }
  }, [brandList])

  // Filtered video list for review view
  const filteredDiscoveredVideos = useMemo(() => {
    const now = Date.now()
    const recencyMs: Record<string, number> = {
      '30d': 30 * 86400000,
      '90d': 90 * 86400000,
      '1y': 365 * 86400000,
      '3y': 3 * 365 * 86400000,
    }
    return discoveredVideos.filter(v => {
      if (!videoFilterTags.has(v.relevance_tag)) return false
      if ((v.view_count || 0) < videoFilterMinViews) return false
      if (videoFilterRecency !== 'all') {
        const age = now - new Date(v.published_at).getTime()
        if (age > (recencyMs[videoFilterRecency] || Infinity)) return false
      }
      return true
    })
  }, [discoveredVideos, videoFilterTags, videoFilterMinViews, videoFilterRecency])

  // Auto-select all filtered videos when filters change
  useEffect(() => {
    if (filteredDiscoveredVideos.length > 0) {
      setSelectedVideoIds(new Set(filteredDiscoveredVideos.map(v => v.video_id)))
    }
  }, [filteredDiscoveredVideos])

  // Computed stats for filter UI
  const videoViewCountMax = useMemo(() => {
    if (discoveredVideos.length === 0) return 0
    return Math.max(...discoveredVideos.map(v => v.view_count || 0))
  }, [discoveredVideos])

  const videoTagCounts = useMemo(() => {
    const counts: Record<string, number> = {}
    for (const v of discoveredVideos) {
      counts[v.relevance_tag] = (counts[v.relevance_tag] || 0) + 1
    }
    return counts
  }, [discoveredVideos])

  const videoDiscover = useCallback(async () => {
    if (!videoDomain.trim()) return
    setVideoDiscovering(true)
    setVideoMessages([])
    setDiscoveredVideos([])
    setSelectedVideoIds(new Set())
    setVideoFilterTags(new Set(['own', 'direct_mention', 'competitor_comparison', 'category_content']))
    setVideoFilterMinViews(0)
    setVideoFilterRecency('all')
    setVideoView('discovering')

    try {
      // Gather competitor names from brand profile
      let competitors: string[] = []
      let brandName = videoDomain
      try {
        const brandRes = await apiFetch(`/api/brands/${encodeURIComponent(videoDomain.trim())}`)
        if (brandRes.ok) {
          const brand = await brandRes.json() as BrandProfile
          brandName = brand.brand_name || videoDomain
          competitors = brand.competitors?.map(c => c.name) || []
        }
      } catch { /* ignore */ }

      const res = await apiFetch('/api/video/discover', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain: videoDomain.trim(),
          brand_name: brandName,
          channel_url: videoChannelURL.trim(),
          search_terms: videoSearchTerms,
          competitors,
        }),
      })

      if (!res.ok) {
        // SSE error before stream starts — fall back to text
        const text = await res.text()
        setVideoMessages([text || 'Discovery failed'])
        setVideoView('input')
        setVideoDiscovering(false)
        return
      }

      // Parse SSE stream
      const reader = res.body?.getReader()
      if (!reader) {
        setVideoMessages(['Streaming not supported'])
        setVideoView('input')
        setVideoDiscovering(false)
        return
      }

      const decoder = new TextDecoder()
      let buffer = ''
      let currentEvent = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })

        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          if (line.startsWith('event: ')) {
            currentEvent = line.slice(7).trim()
          } else if (line.startsWith('data: ') && currentEvent) {
            try {
              const data = JSON.parse(line.slice(6))
              if (currentEvent === 'status') {
                setVideoMessages(prev => [...prev, data.message])
              } else if (currentEvent === 'done') {
                const videos = (data.videos || []) as YouTubeVideo[]
                setDiscoveredVideos(videos)
                setSelectedVideoIds(new Set(videos.map((v: YouTubeVideo) => v.video_id)))
                setVideoQuotaEstimate(data.quota_estimate || 0)
                setVideoView('review')
              } else if (currentEvent === 'error') {
                setVideoMessages(prev => [...prev, data.message || 'Discovery failed'])
                setVideoView('input')
              }
            } catch { /* ignore malformed JSON */ }
            currentEvent = ''
          }
        }
      }
    } catch (err) {
      setVideoMessages(['Connection failed: ' + (err as Error).message])
      setVideoView('input')
    } finally {
      setVideoDiscovering(false)
    }
  }, [videoDomain, videoChannelURL, videoSearchTerms])

  const videoAnalyze = useCallback(async () => {
    if (selectedVideoIds.size === 0) return
    const controller = new AbortController()
    videoAbortRef.current = controller
    setVideoAnalyzing(true)
    setVideoMessages([])
    setVideoView('running')

    // Auto-archive incomplete video todos for this domain
    if (videoAutoArchive && videoDomain.trim()) {
      try {
        await apiFetch('/api/todos/archive', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ source_type: 'video', domain: videoDomain.trim() }),
        })
      } catch { /* best effort */ }
    }

    try {
      const response = await apiFetch('/api/video/analyze', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain: videoDomain.trim(),
          config: {
            channel_url: videoChannelURL.trim(),
            video_urls: videoURLs.filter(u => u.trim()),
            brand_url: videoBrandURL.trim(),
            search_terms: videoSearchTerms,
          },
          selected_video_ids: [...selectedVideoIds],
        }),
        signal: controller.signal,
      })

      if (!response.ok || !response.body) {
        setVideoMessages(['Analysis request failed'])
        setVideoAnalyzing(false)
        return
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })

        while (true) {
          const idx = buffer.indexOf('\n\n')
          if (idx === -1) break
          const message = buffer.slice(0, idx)
          buffer = buffer.slice(idx + 2)

          let eventType = '', data = ''
          for (const line of message.split('\n')) {
            if (line.startsWith('event: ')) eventType = line.slice(7)
            else if (line.startsWith('data: ')) data = line.slice(6)
          }
          if (!data || !eventType) continue

          try {
            const parsed = JSON.parse(data)
            if (eventType === 'status') {
              setVideoMessages(prev => [...prev, parsed.message])
            } else if (eventType === 'progress') {
              // Replace last message in-place (e.g. transcript extraction counter)
              setVideoMessages(prev => prev.length > 0 ? [...prev.slice(0, -1), parsed.message] : [parsed.message])
            } else if (eventType === 'done') {
              // Parse the nested result JSON
              try {
                const result = typeof parsed.result === 'string' ? JSON.parse(parsed.result) : parsed.result
                const analysis: VideoAnalysis = {
                  id: '',
                  domain: result.domain || videoDomain,
                  config: result.config || { channel_url: videoChannelURL, video_urls: [], brand_url: videoBrandURL, search_terms: videoSearchTerms },
                  videos: result.videos || [],
                  result: result.result,
                  model: result.model || '',
                  brand_context_used: result.brand_context_used || false,
                  generated_at: result.generated_at || new Date().toISOString(),
                }
                setVideoAnalysis(analysis)
                setVideoView('results')
              } catch {
                setVideoMessages(prev => [...prev, 'Failed to parse analysis results'])
              }
              setVideoAnalyzing(false)
              fetchVideoAnalysisList()
              fetchTodos()
              return
            } else if (eventType === 'error') {
              setVideoMessages(prev => [...prev, 'Error: ' + parsed.message])
              setVideoAnalyzing(false)
              return
            }
          } catch { /* skip malformed */ }
        }
      }
    } catch (err) {
      if (!controller.signal.aborted) {
        setVideoMessages(prev => [...prev, 'Connection failed: ' + (err as Error).message])
      }
    } finally {
      setVideoAnalyzing(false)
      videoAbortRef.current = null
    }
  }, [selectedVideoIds, videoDomain, videoChannelURL, videoURLs, videoBrandURL, videoSearchTerms, videoAutoArchive, fetchVideoAnalysisList])

  const videoAnalyzeStop = useCallback(() => {
    videoAbortRef.current?.abort()
    setVideoAnalyzing(false)
    setVideoMessages(prev => [...prev, 'Analysis stopped by user'])
  }, [])

  const deleteVideoAnalysis = useCallback(async () => {
    if (!videoDomain) return
    try {
      await apiFetch(`/api/video/analyses/${encodeURIComponent(videoDomain)}`, { method: 'DELETE' })
      setVideoAnalysis(null)
      setVideoView('input')
      fetchVideoAnalysisList()
    } catch (err) {
      console.error('Failed to delete video analysis:', err)
    }
    setConfirmDeleteVideoAnalysis(false)
  }, [videoDomain, fetchVideoAnalysisList])

  // Fetch video analyses list on tab switch
  useEffect(() => {
    if (activeTab === 'video') {
      fetchVideoAnalysisList()
    }
  }, [activeTab, fetchVideoAnalysisList])

  const discoverCompetitors = useCallback(async () => {
    if (!brandDomain.trim()) return
    setDiscoveredCompetitors([])
    setDiscoverSelected(new Set())
    await brandSSE(
      `/api/brands/${encodeURIComponent(brandDomain.trim())}/discover-competitors`,
      setDiscoverMessages,
      setDiscovering,
      (result) => {
        try {
          const parsed = JSON.parse(result)
          const comps = (parsed.competitors || []) as DiscoveredCompetitor[]
          setDiscoveredCompetitors(comps)
          setDiscoverSelected(new Set(comps.map((_: DiscoveredCompetitor, i: number) => i)))
        } catch { /* ignore */ }
      },
    )
  }, [brandDomain, brandSSE])

  const addSelectedCompetitors = useCallback(() => {
    const newComps = discoveredCompetitors
      .filter((_, i) => discoverSelected.has(i))
      .map(c => ({ name: c.name, url: c.url, relationship: c.relationship, notes: c.notes || '' }))
    setBrandCompetitors(prev => {
      const existing = new Set(prev.map(c => c.name.toLowerCase()))
      return [...prev, ...newComps.filter(c => !existing.has(c.name.toLowerCase()))]
    })
    setDiscoveredCompetitors([])
  }, [discoveredCompetitors, discoverSelected])

  const suggestQueries = useCallback(async () => {
    if (!brandDomain.trim()) return
    setSuggestedQueries([])
    setSuggestSelected(new Set())
    await brandSSE(
      `/api/brands/${encodeURIComponent(brandDomain.trim())}/suggest-queries`,
      setSuggestMessages,
      setSuggestingQueries,
      (result) => {
        try {
          const parsed = JSON.parse(result)
          const queries = (parsed.queries || []) as SuggestedQuery[]
          setSuggestedQueries(queries)
          setSuggestSelected(new Set(queries.map((_: SuggestedQuery, i: number) => i)))
        } catch { /* ignore */ }
      },
    )
  }, [brandDomain, brandSSE])

  const addSelectedQueries = useCallback(() => {
    const newQueries = suggestedQueries
      .filter((_, i) => suggestSelected.has(i))
    setBrandQueries(prev => {
      const existing = new Set(prev.map(q => q.query.toLowerCase()))
      return [...prev, ...newQueries.filter(q => !existing.has(q.query.toLowerCase()))]
    })
    setSuggestedQueries([])
  }, [suggestedQueries, suggestSelected])

  const generateDescription = useCallback(async () => {
    if (!brandDomain.trim()) return
    await brandSSE(
      `/api/brands/${encodeURIComponent(brandDomain.trim())}/generate-description`,
      setGenerateMessages,
      setGeneratingDesc,
      (result) => {
        try {
          const parsed = JSON.parse(result)
          if (parsed.description) setBrandForm(prev => ({ ...prev, description: parsed.description }))
          if (parsed.brand_name && !brandForm.brand_name) setBrandForm(prev => ({ ...prev, brand_name: parsed.brand_name }))
          if (parsed.categories?.length && !brandForm.categories) setBrandForm(prev => ({ ...prev, categories: parsed.categories.join(', ') }))
          if (parsed.products?.length && !brandForm.products) setBrandForm(prev => ({ ...prev, products: parsed.products.join(', ') }))
        } catch { /* ignore */ }
      },
    )
  }, [brandDomain, brandSSE, brandForm.brand_name, brandForm.categories, brandForm.products])

  const predictAudience = useCallback(async () => {
    if (!brandDomain.trim()) return
    await brandSSE(
      `/api/brands/${encodeURIComponent(brandDomain.trim())}/predict-audience`,
      setPredictAudienceMessages,
      setPredictingAudience,
      (result) => {
        try {
          const parsed = JSON.parse(result)
          if (parsed.primary_audience && !brandForm.primary_audience)
            setBrandForm(prev => ({ ...prev, primary_audience: parsed.primary_audience }))
          if (parsed.key_use_cases?.length && !brandForm.key_use_cases)
            setBrandForm(prev => ({ ...prev, key_use_cases: parsed.key_use_cases.join(', ') }))
        } catch { /* ignore */ }
      },
    )
  }, [brandDomain, brandSSE, brandForm.primary_audience, brandForm.key_use_cases])

  const suggestClaims = useCallback(async () => {
    if (!brandDomain.trim()) return
    setSuggestedClaims([])
    setSuggestClaimSelected(new Set())
    await brandSSE(
      `/api/brands/${encodeURIComponent(brandDomain.trim())}/suggest-claims`,
      setSuggestClaimMessages,
      setSuggestingClaims,
      (result) => {
        try {
          const parsed = JSON.parse(result)
          const claims = (parsed.claims || []) as SuggestedClaim[]
          setSuggestedClaims(claims)
          setSuggestClaimSelected(new Set(claims.map((_: SuggestedClaim, i: number) => i)))
        } catch { /* ignore */ }
      },
    )
  }, [brandDomain, brandSSE])

  const addSelectedClaims = useCallback(() => {
    const newClaims = suggestedClaims
      .filter((_: SuggestedClaim, i: number) => suggestClaimSelected.has(i))
      .map((c: SuggestedClaim) => ({ claim: c.claim, evidence_url: c.evidence_url || '', priority: c.priority || 'medium' }))
    setBrandMessages(prev => {
      const existing = new Set(prev.map(m => m.claim.toLowerCase()))
      return [...prev, ...newClaims.filter(c => !existing.has(c.claim.toLowerCase()))]
    })
    setSuggestedClaims([])
  }, [suggestedClaims, suggestClaimSelected])

  const predictDifferentiators = useCallback(async () => {
    if (!brandDomain.trim()) return
    setSuggestedDiffs([])
    setSuggestDiffSelected(new Set())
    await brandSSE(
      `/api/brands/${encodeURIComponent(brandDomain.trim())}/predict-differentiators`,
      setPredictDiffMessages,
      setPredictingDiffs,
      (result) => {
        try {
          const parsed = JSON.parse(result)
          const raw = parsed.differentiators || []
          // Handle both string[] and legacy {differentiator, reasoning}[] formats
          const diffs: string[] = raw.map((d: string | { differentiator: string }) =>
            typeof d === 'string' ? d : d.differentiator
          ).filter(Boolean)
          setSuggestedDiffs(diffs)
          setSuggestDiffSelected(new Set(diffs.map((_: string, i: number) => i)))
        } catch { /* ignore */ }
      },
    )
  }, [brandDomain, brandSSE])

  const addSelectedDifferentiators = useCallback(() => {
    const newDiffs = suggestedDiffs
      .filter((_: string, i: number) => suggestDiffSelected.has(i))
    const currentDiffs = brandForm.differentiators ? brandForm.differentiators.split(',').map(s => s.trim()).filter(Boolean) : []
    const existingLower = new Set(currentDiffs.map(d => d.toLowerCase()))
    const merged = [...currentDiffs, ...newDiffs.filter(d => !existingLower.has(d.toLowerCase()))]
    setBrandForm(prev => ({ ...prev, differentiators: merged.join(', ') }))
    setSuggestedDiffs([])
  }, [suggestedDiffs, suggestDiffSelected, brandForm.differentiators])

  // Brand completeness score
  const brandCompleteness = (() => {
    let score = 0
    // Core (30%): name, description, categories, products
    if (brandForm.brand_name) score += 8
    if (brandForm.description) score += 10
    if (brandForm.categories) score += 6
    if (brandForm.products) score += 6
    // Audience (15%): audience, use cases
    if (brandForm.primary_audience) score += 8
    if (brandForm.key_use_cases) score += 7
    // Competitors (25%)
    if (brandCompetitors.length > 0) score += 15
    if (brandCompetitors.length >= 3) score += 10
    // Queries/Voice (20%)
    if (brandQueries.length > 0) score += 8
    if (brandQueries.length >= 5) score += 4
    if (brandMessages.length > 0) score += 4
    if (brandForm.differentiators) score += 4
    // Presence (10%)
    if (brandPresenceComplete || brandPresenceForm.youtube_url || brandPresenceForm.subreddits || brandPresenceForm.review_site_urls) score += 10
    return Math.min(100, score)
  })()

  const updateTodoStatus = useCallback(async (id: string, status: 'todo' | 'completed' | 'backlogged' | 'archived') => {
    try {
      const response = await apiFetch(`/api/todos/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status }),
      })
      if (response.ok) {
        const updater = (t: TodoItem) =>
          t.id === id ? {
            ...t, status,
            completed_at: status === 'completed' ? new Date().toISOString() : undefined,
            archived_at: status === 'archived' ? new Date().toISOString() : undefined,
          } : t
        setTodos(prev => prev.map(updater))
        setOptTodos(prev => prev.map(updater))
      }
    } catch (err) {
      console.error('Failed to update todo:', err)
    }
  }, [])

  // Tab-switch effects — split per tab to avoid cross-tab dependency cycles
  // In shared mode, data is pre-loaded from the share endpoint — skip authenticated fetches.
  useEffect(() => {
    if (sharedModeRef.current) return
    if (activeTab === 'todos') {
      fetchTodos()
      fetchBrandList()
      fetchOptList()
      fetchHistory()
    }
  }, [activeTab, selectedDomain, fetchTodos, fetchBrandList, fetchOptList, fetchHistory])

  useEffect(() => {
    if (sharedModeRef.current) return
    if (activeTab === 'optimize' && optimizeView === 'list') {
      fetchOptList()
    }
  }, [activeTab, optimizeView, fetchOptList])

  useEffect(() => {
    if (activeTab === 'status') {
      fetchHealthTimeline()
    }
  }, [activeTab, fetchHealthTimeline])

  useEffect(() => {
    if (sharedModeRef.current) return
    if (activeTab === 'brand' && !brandEditing && selectedDomain) {
      fetchBrandList()
      const existing = brandList.find(b => domainKey(b.domain) === domainKey(selectedDomain))
      if (existing) {
        loadBrandProfile(selectedDomain).then(found => {
          if (!found) {
            startNewBrand(selectedDomain)
            setTimeout(() => generateDescription(), 100)
          }
        })
      } else {
        startNewBrand(selectedDomain)
        // Auto-trigger description generation for new domains
        setTimeout(() => generateDescription(), 100)
      }
    }
  }, [activeTab, brandEditing, selectedDomain, fetchBrandList])

  useEffect(() => {
    if (sharedModeRef.current) return
    if (activeTab === 'video') {
      fetchVideoAnalysisList()
      if (selectedDomain) {
        // Always prepopulate settings from brand profile (channel URL, search terms)
        prepopulateFromBrand(selectedDomain)
        // Auto-load existing report if one exists for this domain
        const existing = videoAnalysisList.find(a => domainKey(a.domain) === domainKey(selectedDomain))
        if (existing) {
          loadVideoAnalysis(existing.domain)
        }
      }
    }
  }, [activeTab, selectedDomain, fetchVideoAnalysisList, prepopulateFromBrand])

  // Fetch all data on mount for domain dropdown (skip in shared mode)
  useEffect(() => {
    if (sharedModeRef.current) return
    fetchPopularDomains()
    fetchBrandList()
    fetchHistory()
    fetchOptList()
    fetchTodos()
    fetchVideoAnalysisList()
  }, [fetchPopularDomains, fetchBrandList, fetchHistory, fetchOptList, fetchTodos, fetchVideoAnalysisList])

  // Detect /share/{shareId} URL and load shared data
  useEffect(() => {
    const match = window.location.pathname.match(/^\/share\/([A-Za-z0-9]+)$/)
    if (!match) return
    const shareId = match[1]
    const shareParams = new URLSearchParams(window.location.search)
    const shareTab = shareParams.get('tab')
    const validShareTabs: ActiveTab[] = ['analyze', 'optimize', 'todos', 'brand', 'video']
    if (shareTab && validShareTabs.includes(shareTab as ActiveTab)) {
      setActiveTab(shareTab as ActiveTab)
    }
    setSharedMode(true)
    sharedModeRef.current = true
    setSharedLoading(true)
    fetch(`/api/share/${shareId}`)
      .then(r => {
        if (!r.ok) { setSharedNotFound(true); setSharedLoading(false); return null }
        return r.json()
      })
      .then(data => {
        if (!data) return
        setSelectedDomain(data.domain)
        setUrl(data.domain)
        // Map analyses
        if (data.analyses?.length > 0) {
          const latest = data.analyses[0]
          setResult(latest.result)
          setResultMeta({ id: latest.id, model: latest.model, createdAt: latest.created_at, cached: false, brandContextUsed: latest.brand_context_used, brandProfileUpdatedAt: latest.brand_profile_updated_at || null })
          setState('done')
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          setHistory(data.analyses.map((a: any) => ({
            id: a.id, domain: a.domain,
            site_summary: a.result?.site_summary || a.result?.siteSummary || '',
            question_count: a.result?.questions?.length || 0,
            page_count: a.result?.crawled_pages?.length || a.result?.crawledPages?.length || 0,
            model: a.model, brand_context_used: a.brand_context_used,
            brand_profile_updated_at: a.brand_profile_updated_at,
            created_at: a.created_at,
          })))
        }
        // Map optimizations
        if (data.optimizations?.length > 0) {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          setOptList(data.optimizations.map((o: any) => ({
            id: o.id, domain: o.domain, question: o.question,
            question_index: o.question_index ?? -1,
            overall_score: o.result?.overallScore ?? 0,
            model: o.model, public: false,
            brand_status: o.brand_status || '',
            brand_context_used: o.brand_context_used,
            brand_profile_updated_at: o.brand_profile_updated_at,
            created_at: o.created_at,
          })))
          // Store full optimization objects for detail view in shared mode
          setSharedOptimizations(data.optimizations)
        }
        // Map brand profile — populate form fields so Brand tab renders properly
        if (data.brand_profile) {
          const bp = data.brand_profile
          setBrandProfile(bp)
          setBrandDomain(data.domain)
          setBrandForm({
            brand_name: bp.brand_name || '',
            description: bp.description || '',
            categories: (bp.categories || []).join(', '),
            products: (bp.products || []).join(', '),
            primary_audience: bp.primary_audience || '',
            key_use_cases: (bp.key_use_cases || []).join(', '),
            differentiators: (bp.differentiators || []).join(', '),
          })
          setBrandCompetitors(bp.competitors || [])
          setBrandQueries(bp.target_queries || [])
          setBrandMessages(bp.key_messages || [])
          const p = bp.presence || { youtube_url: '', subreddits: [], review_site_urls: [], social_profiles: [], content_assets: [], podcasts: [] }
          setBrandPresenceForm({
            youtube_url: p.youtube_url || '',
            subreddits: (p.subreddits || []).join(', '),
            review_site_urls: (p.review_site_urls || []).join(', '),
            social_profiles: (p.social_profiles || []).join(', '),
            content_assets: (p.content_assets || []).join(', '),
            podcasts: (p.podcasts || []).join(', '),
          })
          setBrandPresenceComplete(bp.presence_complete || false)
          brandLoadingRef.current = true
          setBrandEditing(true)
        }
        // Map video analysis
        if (data.video_analysis) {
          setVideoAnalysis(data.video_analysis)
          setVideoDomain(data.domain)
          setVideoView('results')
        }
        // Map todos
        if (data.todos?.length > 0) {
          setTodos(data.todos)
        }
        // Map domain summary
        if (data.domain_summary) {
          setActiveSummary(data.domain_summary)
          setActiveSummaryDomain(data.domain)
        }
        setSharedLoading(false)
      })
      .catch(() => { setSharedNotFound(true); setSharedLoading(false) })
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Propagate selectedDomain to per-tab domain states
  useEffect(() => {
    if (!selectedDomain) return
    setBrandDomain(selectedDomain)
    setVideoDomain(selectedDomain)
    // In shared mode, don't reset views — share data load already set them
    if (!sharedModeRef.current) {
      setOptimizeView('list')
      setVideoView('input')
      setBrandEditing(false)
    }
  }, [selectedDomain])

  // Populate optScores from optList when analysis result or optList changes
  useEffect(() => {
    if (!result || !optList.length || !selectedDomain) {
      setOptScores({})
      return
    }
    const domainOpts = optList.filter(o => domainKey(o.domain) === domainKey(selectedDomain))
    const scores: Record<number, number> = {}
    // For each question in the analysis, find the latest optimization by question_index
    for (const opt of domainOpts) {
      if (opt.question_index >= 0 && opt.question_index < result.questions.length) {
        // Only keep the highest score (latest is first since list is sorted by date desc,
        // but question_index may have multiple runs — keep the latest/first)
        if (scores[opt.question_index] === undefined) {
          scores[opt.question_index] = opt.overall_score
        }
      }
    }
    setOptScores(scores)
  }, [result, optList, selectedDomain])

  // Clear selectedDomain if it no longer exists in allDomains
  // (but only if idle — don't clear during active analysis which adds new domains)
  // Skip in shared mode — domain is set from share data, not from allDomains
  useEffect(() => {
    if (sharedModeRef.current) return
    if (selectedDomain && allDomains.length > 0 && state !== 'analyzing' &&
        !allDomains.some(d => domainKey(d.domain) === domainKey(selectedDomain))) {
      setSelectedDomain('')
      setActiveTab('analyze')
    }
  }, [allDomains, selectedDomain, state])

  // Global health check polling — runs always, not just on Status tab
  useEffect(() => {
    checkHealth()
    const interval = setInterval(checkHealth, 5 * 60 * 1000)
    return () => clearInterval(interval)
  }, [checkHealth])


  const loadAnalysis = useCallback(async (id: string) => {
    try {
      const response = await apiFetch(`/api/analyses/${id}`)
      if (response.ok) {
        const data = await response.json()
        setResult(data.result)
        setResultMeta({
          id: data.id,
          model: data.model || '',
          createdAt: data.created_at || '',
          cached: false,
          brandContextUsed: data.brand_context_used || false,
          brandProfileUpdatedAt: data.brand_profile_updated_at || undefined,
        })
        setUrl(data.domain)
        setActiveTab('analyze')
        setState('done')
      }
    } catch (err) {
      console.error('Failed to load analysis:', err)
    }
  }, [])

  const loadBestAnalysis = useCallback(async (domain: string) => {
    try {
      const res = await apiFetch(`/api/analyses?domain=${encodeURIComponent(domainKey(domain))}`)
      if (!res.ok) return
      const list: AnalysisSummary[] = await res.json()
      // Backend sorts brand-intel reports first, then by date
      const best = list[0]
      if (best) loadAnalysis(best.id)
    } catch { /* ignore */ }
  }, [loadAnalysis])

  const analyze = useCallback(async (force?: boolean) => {
    const targetUrl = normalizeUrl(url)
    if (!targetUrl) return
    setUrl(targetUrl)

    // If the entered domain already exists in the dropdown, select it instead of re-analyzing
    const existingDomain = allDomains.find(d => domainKey(d.domain) === domainKey(targetUrl))
    if (existingDomain && !force && !forceAnalyze) {
      setSelectedDomain(existingDomain.domain)
      setUrl(existingDomain.domain)
      loadBestAnalysis(existingDomain.domain)
      return
    }

    // If the domain matches a popular brand, navigate to its shared view
    const popularMatch = popularDomains.find(pd => domainKey(pd.domain) === domainKey(targetUrl))
    if (popularMatch && !force && !forceAnalyze) {
      window.location.href = `/share/${popularMatch.share_id}`
      return
    }

    // SaaS gating: require login and active subscription for new analyses
    if (saasEnabled && !user) {
      window.location.href = '/login'
      return
    }
    if (saasEnabled && user && !hasActivePlan) {
      setSubscriptionModal(true)
      return
    }

    const shouldForce = force ?? forceAnalyze

    setState('analyzing')
    setStatusMessages([])
    setResult(null)
    setResultMeta(null)
    setError('')
    setForceAnalyze(false)

    const controller = new AbortController()
    abortRef.current = controller

    try {
      const response = await apiFetch('/api/analyze', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: targetUrl, force: shouldForce }),
        signal: controller.signal,
      })

      if (!response.ok || !response.body) {
        throw new Error(`HTTP ${response.status}`)
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let finished = false

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })

        while (true) {
          const idx = buffer.indexOf('\n\n')
          if (idx === -1) break

          const message = buffer.slice(0, idx)
          buffer = buffer.slice(idx + 2)

          let eventType = ''
          let data = ''

          for (const line of message.split('\n')) {
            if (line.startsWith('event: ')) eventType = line.slice(7)
            else if (line.startsWith('data: ')) data = line.slice(6)
          }

          if (!data || !eventType) continue

          try {
            const parsed = JSON.parse(data)

            switch (eventType) {
              case 'status':
                setStatusMessages(prev => [...prev, parsed.message])
                break
              case 'text':
                break
              case 'done':
                try {
                  const analysisResult = JSON.parse(parsed.result) as AnalysisResult
                  setResult(analysisResult)
                  setResultMeta({
                    id: parsed.id || '',
                    model: parsed.model || '',
                    createdAt: parsed.created_at || '',
                    cached: parsed.cached || false,
                    brandContextUsed: parsed.brand_context_used || false,
                    brandProfileUpdatedAt: parsed.brand_profile_updated_at || undefined,
                  })
                  setState('done')
                  setSelectedDomain(targetUrl)
                  fetchHistory()
                  // Load the best report for this domain (may be a better one with brand intel)
                  loadBestAnalysis(targetUrl)
                  finished = true
                } catch {
                  setError('Failed to parse analysis results')
                  setState('error')
                  finished = true
                }
                break
              case 'error':
                setError(parsed.message)
                setState('error')
                finished = true
                break
            }
          } catch {
            // Malformed SSE data, skip
          }
        }
      }

      if (!finished) {
        setError('Analysis stream ended unexpectedly')
        setState('error')
      }
    } catch (err) {
      if (!controller.signal.aborted) {
        setError('Connection failed: ' + (err as Error).message)
        setState('error')
      }
    }
  }, [url, forceAnalyze, fetchHistory, allDomains, loadBestAnalysis, saasEnabled, user, hasActivePlan, popularDomains])

  const cancel = useCallback(() => {
    abortRef.current?.abort()
    setState('idle')
  }, [])

  const optimizeQuestion = useCallback(async (questionIdx: number, force?: boolean) => {
    if (!resultMeta?.id) return

    setOptimizing(true)
    setOptimizeMessages([])
    setOptimization(null)
    setOptimizationMeta(null)
    setOptimizeError('')
    setOptTodos([])
    setSelectedOpt(null)
    setOptimizeView('running')
    setActiveTab('optimize')

    // Auto-archive incomplete optimization todos for this question
    const questionText = result?.questions[questionIdx]?.question
    if (optimizeAutoArchive && url && questionText) {
      try {
        await apiFetch('/api/todos/archive', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ source_type: 'optimization', domain: url, question: questionText }),
        })
      } catch { /* best effort */ }
    }

    const controller = new AbortController()
    optimizeAbortRef.current = controller

    try {
      const response = await apiFetch(`/api/analyses/${resultMeta.id}/questions/${questionIdx}/optimize`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ force: force || false }),
        signal: controller.signal,
      })

      if (!response.ok || !response.body) {
        throw new Error(`HTTP ${response.status}`)
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let finished = false

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })

        while (true) {
          const idx = buffer.indexOf('\n\n')
          if (idx === -1) break

          const message = buffer.slice(0, idx)
          buffer = buffer.slice(idx + 2)

          let eventType = ''
          let data = ''

          for (const line of message.split('\n')) {
            if (line.startsWith('event: ')) eventType = line.slice(7)
            else if (line.startsWith('data: ')) data = line.slice(6)
          }

          if (!data || !eventType) continue

          try {
            const parsed = JSON.parse(data)

            switch (eventType) {
              case 'status':
                setOptimizeMessages(prev => [...prev, parsed.message])
                break
              case 'text':
                break
              case 'done':
                try {
                  const optResult = JSON.parse(parsed.result) as OptimizationResult
                  setOptimization(optResult)
                  setOptimizationMeta({
                    id: parsed.id || '',
                    model: parsed.model || '',
                    createdAt: parsed.created_at || '',
                    cached: parsed.cached || false,
                    question: result?.questions[questionIdx]?.question || '',
                    questionIndex: questionIdx,
                    brandStatus: parsed.brand_status || result?.questions[questionIdx]?.brand_status || undefined,
                    brandContextUsed: parsed.brand_context_used || false,
                    brandProfileUpdatedAt: parsed.brand_profile_updated_at || undefined,
                  })
                  setOptScores(prev => ({ ...prev, [questionIdx]: optResult.overall_score }))
                  setOptimizing(false)
                  setOptimizeView('detail')
                  // Fetch todos after a short delay (they're created async)
                  if (parsed.id) {
                    setTimeout(async () => {
                      try {
                        const todosRes = await apiFetch(`/api/todos?optimization_id=${parsed.id}`)
                        if (todosRes.ok) {
                          const todosData = await todosRes.json()
                          setOptTodos(todosData || [])
                        }
                      } catch { /* ignore */ }
                    }, 1000)
                  }
                  finished = true
                } catch {
                  setOptimizeError('Failed to parse optimization results')
                  setOptimizing(false)
                  finished = true
                }
                break
              case 'error':
                setOptimizeError(parsed.message)
                setOptimizing(false)
                finished = true
                break
            }
          } catch {
            // Malformed SSE data, skip
          }
        }
      }

      if (!finished) {
        setOptimizeError('Optimization stream ended unexpectedly')
        setOptimizing(false)
      }
    } catch (err) {
      if (!controller.signal.aborted) {
        setOptimizeError('Connection failed: ' + (err as Error).message)
        setOptimizing(false)
      }
    }
  }, [resultMeta, result, optimizeAutoArchive, url])

  const cancelOptimize = useCallback(() => {
    optimizeAbortRef.current?.abort()
    setOptimizing(false)
    setOptimizeView('list')
  }, [])

  // Build category color map
  const categories = result?.questions
    ? [...new Set(result.questions.map(q => q.category))]
    : []
  const categoryColors: Record<string, string> = {}
  categories.forEach((cat, i) => {
    categoryColors[cat] = CATEGORY_COLORS[i % CATEGORY_COLORS.length]
  })

  const readOnly = sharedMode

  // Handle question click from Analyze tab — shows confirmation or jumps to existing report
  const handleQuestionClick = (questionIdx: number) => {
    if (readOnly) {
      // Check if optimization report exists for this question
      const existing = optList.find(o =>
        domainKey(o.domain) === domainKey(selectedDomain) && o.question_index === questionIdx
      )
      if (existing) {
        // Jump to existing report
        loadOptimizationDetail(existing.id)
        setActiveTab('optimize')
      } else {
        // Show "not ready" modal
        const brandName = brandList.find(b => domainKey(b.domain) === domainKey(selectedDomain))?.brand_name || selectedDomain
        setReadOnlyOptModal(brandName)
      }
      return
    }
    // Normal (authenticated) mode
    if (!saasEnabled || !user) {
      // Non-SaaS mode: just optimize directly
      optimizeQuestion(questionIdx)
      return
    }
    // Check if optimization report already exists
    const existing = optList.find(o =>
      domainKey(o.domain) === domainKey(selectedDomain) && o.question_index === questionIdx
    )
    if (existing) {
      // Jump directly to existing report
      loadOptimizationDetail(existing.id)
      setActiveTab('optimize')
      return
    }
    // Show confirmation modal
    setOptimizeConfirmQ(questionIdx)
  }

  // Shared mode: loading / not-found states
  if (sharedLoading) {
    return (
      <div className="min-h-screen bg-dark-950 flex items-center justify-center">
        <div className="text-center">
          <div className="w-8 h-8 border-2 border-primary-500 border-t-transparent rounded-full animate-spin mx-auto mb-4" />
          <p className="text-dark-400">Loading shared report...</p>
        </div>
      </div>
    )
  }

  if (sharedNotFound) {
    return (
      <div className="min-h-screen bg-dark-950 flex items-center justify-center">
        <div className="text-center max-w-md mx-auto px-4">
          <div className="w-16 h-16 rounded-full bg-dark-800 flex items-center justify-center mx-auto mb-4">
            <svg className="w-8 h-8 text-dark-500" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636" /></svg>
          </div>
          <h2 className="text-white text-xl font-semibold mb-2">Report Not Available</h2>
          <p className="text-dark-400 text-sm mb-6">This shared report is no longer available. The owner may have made it private.</p>
          <a href="/" className="inline-flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-500 transition-colors text-sm font-medium">
            Go to LLM Optimizer
          </a>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="sticky top-0 z-50 bg-dark-900/80 backdrop-blur-xl border-b border-dark-800">
        <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <a href="/" className="flex items-center gap-3">
              <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary-500 to-accent-purple flex items-center justify-center">
                <svg className="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.455 2.456L21.75 6l-1.036.259a3.375 3.375 0 00-2.455 2.456z" />
                </svg>
              </div>
              <h1 className="text-xl font-semibold tracking-tight text-white">
                LLM Optimizer
              </h1>
            </a>
            {readOnly && selectedDomain && (
              <span className="text-dark-400 text-sm ml-2">· {selectedDomain}</span>
            )}
          </div>
          <div className="flex items-center gap-2">
            {!readOnly && saasEnabled && user && (
              <>
                {user.isRootTenant && (user.role === 'owner' || user.role === 'admin') && (
                  <a href="/last" className="px-3 py-1.5 text-sm text-dark-400 hover:text-white hover:bg-dark-800/50 rounded-lg transition-colors">Admin</a>
                )}
                <a href="/last/team" className="flex items-center gap-1.5 px-3 py-1.5 text-sm text-dark-400 hover:text-white hover:bg-dark-800/50 rounded-lg transition-colors">
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z" />
                  </svg>
                  Team
                </a>
                <a href="/last/plan" className="flex items-center gap-1.5 px-3 py-1.5 text-sm text-dark-400 hover:text-white hover:bg-dark-800/50 rounded-lg transition-colors">
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 002.25-2.25V6.75A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25v10.5A2.25 2.25 0 004.5 19.5z" />
                  </svg>
                  Plan
                </a>
                {showCredits && (
                  <a href="/last/plan" className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-dark-800 border border-dark-700 text-sm text-dark-300 hover:text-white hover:border-primary-500/30 transition-colors" title="Usage credits">
                    <svg className="w-4 h-4 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
                    </svg>
                    <span className="font-medium">{tenantCredits.toLocaleString()}</span>
                  </a>
                )}
                <a href="/last/messages" className="relative px-2 py-1.5 text-dark-400 hover:text-white hover:bg-dark-800/50 rounded-lg transition-colors">
                  <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M14.857 17.082a23.848 23.848 0 005.454-1.31A8.967 8.967 0 0118 9.75v-.7V9A6 6 0 006 9v.75a8.967 8.967 0 01-2.312 6.022c1.733.64 3.56 1.085 5.455 1.31m5.714 0a24.255 24.255 0 01-5.714 0m5.714 0a3 3 0 11-5.714 0" />
                  </svg>
                  {unreadCount > 0 && (
                    <span className="absolute -top-0.5 -right-0.5 w-4 h-4 bg-primary-500 text-white text-[10px] font-bold rounded-full flex items-center justify-center">{unreadCount > 9 ? '9+' : unreadCount}</span>
                  )}
                </a>
                <a href="/last/settings" className="px-3 py-1.5 text-sm text-dark-400 hover:text-white hover:bg-dark-800/50 rounded-lg transition-colors truncate max-w-[120px]">{user.displayName || user.email}</a>
                <button
                  onClick={() => clearAuth()}
                  className="text-dark-400 hover:text-white transition-colors"
                  title="Sign out"
                >
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6a2.25 2.25 0 00-2.25 2.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15m3 0l3-3m0 0l-3-3m3 3H9" />
                  </svg>
                </button>
              </>
            )}
            {saasEnabled && !user && (
              <>
                <a href="/login" className="px-3 py-1.5 text-sm text-dark-400 hover:text-white hover:bg-dark-800/50 rounded-lg transition-colors">Sign In</a>
                <a href="/signup" className="px-3 py-1.5 text-sm text-white bg-primary-500 hover:bg-primary-600 rounded-lg transition-colors">Sign Up</a>
              </>
            )}
            <button
              onClick={() => setActiveTab('status')}
              className={`flex items-center gap-2 px-3 py-1.5 rounded-lg transition-colors cursor-pointer ${
                activeTab === 'status' ? 'bg-dark-800 text-white' : 'hover:bg-dark-800/50 text-dark-400'
              }`}
            >
              {healthHistory.length > 0 ? (() => {
                const latest = healthHistory[0]
                const anyAvailable = latest.models.some(m => m.status === 'available')
                const allError = latest.models.every(m => m.status === 'error')
                return <span className={`w-2.5 h-2.5 rounded-full ${anyAvailable ? 'bg-emerald-400' : allError ? 'bg-red-400' : 'bg-amber-400'}`} />
              })() : <span className="w-2.5 h-2.5 rounded-full bg-dark-600" />}
              <span className="text-sm">Status</span>
            </button>
          </div>
        </div>
      </header>

      <main className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        {/* Hero */}
        {!readOnly && (
        <div className="text-center mb-8">
          <h2 className="text-4xl sm:text-5xl font-bold text-white mb-4 leading-tight">
            Be the Brand AI Recommends
          </h2>
          <p className="text-dark-400 text-lg max-w-2xl mx-auto leading-relaxed">
            ChatGPT, Claude, Gemini, and Perplexity are replacing search for millions of people.
            We help you see how AI sees your brand — and optimize so it recommends you.
          </p>
        </div>
        )}

        {/* Tab Navigation — Domain List + tab pills */}
        <div className="flex justify-center mb-6">
          <div className="inline-flex items-center bg-dark-900/50 border border-dark-800 rounded-lg p-1 gap-1">
            {/* Back button — shown when viewing a report, navigates to default home view */}
            {selectedDomain && (
              <button
                onClick={() => {
                  if (sharedMode) {
                    window.location.href = '/'
                  } else {
                    setSelectedDomain('')
                    setActiveTab('analyze')
                    setUrl('')
                    setResult(null)
                    setResultMeta(null)
                    setState('idle')
                  }
                }}
                className="px-2 py-2 rounded-md text-dark-400 hover:text-white transition-all cursor-pointer"
                title="Back to home"
              >
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" />
                </svg>
              </button>
            )}
            {!readOnly && <select
              value={selectedDomain}
              onChange={e => {
                const domain = e.target.value
                const doSwitch = () => {
                  setSelectedDomain(domain)
                  if (domain) {
                    setUrl(domain)
                    if (activeTab === 'analyze') {
                      loadBestAnalysis(domain)
                    }
                  } else {
                    // (New Domain) selected — go to Analyze tab
                    setActiveTab('analyze')
                    setUrl('')
                    setResult(null)
                    setResultMeta(null)
                    setState('idle')
                  }
                }
                if (activeTab === 'brand' && brandEditing) {
                  guardBrandNav(doSwitch)
                } else {
                  doSwitch()
                }
              }}
              className="px-3 py-2 bg-dark-800 border border-dark-700 rounded-md text-white focus:outline-none focus:border-primary-500/50 cursor-pointer text-sm max-w-[200px] truncate"
            >
              <option value="">(New Domain)</option>
              {allDomains.map(d => (
                <option key={d.domain} value={d.domain}>{d.domain.replace(/^https?:\/\//, '')}</option>
              ))}
            </select>}
            <button
              onClick={() => {
                const doSwitch = () => {
                  setActiveTab('analyze')
                  if (selectedDomain) loadBestAnalysis(selectedDomain)
                }
                activeTab === 'brand' && brandEditing ? guardBrandNav(doSwitch) : doSwitch()
              }}
              className={`px-5 py-2 rounded-md text-sm font-medium transition-all cursor-pointer ${
                activeTab === 'analyze'
                  ? 'bg-primary-600 text-white'
                  : 'text-dark-400 hover:text-white'
              }`}
            >
              Analyze
            </button>
            {(['todos', 'brand', 'video', 'optimize'] as const).map(tab => {
              const labels: Record<string, string> = { todos: 'To-Do', brand: 'Brand', video: 'Video', optimize: 'Optimize' }
              const disabled = !selectedDomain
              return (
                <button
                  key={tab}
                  onClick={() => {
                    if (disabled) return
                    if (activeTab === 'brand' && brandEditing) {
                      guardBrandNav(() => setActiveTab(tab))
                    } else {
                      setActiveTab(tab)
                    }
                  }}
                  className={`px-5 py-2 rounded-md text-sm font-medium transition-all relative flex items-center gap-2 ${
                    disabled
                      ? 'text-dark-600 cursor-not-allowed'
                      : activeTab === tab
                        ? 'bg-primary-600 text-white cursor-pointer'
                        : 'text-dark-400 hover:text-white cursor-pointer'
                  }`}
                >
                  {labels[tab]}
                  {tab === 'todos' && selectedDomain && (() => {
                    const count = todos.filter(t => t.status === 'todo' && domainKey(t.domain) === domainKey(selectedDomain)).length
                    return count > 0 ? (
                      <span className="absolute -top-1.5 -right-1.5 min-w-[18px] h-[18px] flex items-center justify-center px-1 text-[10px] font-bold bg-red-500 text-white rounded-full leading-none">
                        {count > 99 ? '99+' : count}
                      </span>
                    ) : null
                  })()}
                  {tab === 'brand' && selectedDomain && (() => {
                    // In shared mode, use brandCompleteness (computed from form state) since brandList isn't populated
                    const pct = sharedMode
                      ? (brandEditing ? brandCompleteness : 0)
                      : (brandList.find(b => domainKey(b.domain) === domainKey(selectedDomain))?.completeness ?? 0)
                    if (pct >= 100) return null
                    if (pct > 0) return (
                      <span className="absolute -top-1.5 -right-1.5 min-w-[18px] h-[18px] flex items-center justify-center px-1 text-[10px] font-bold bg-amber-500 text-white rounded-full leading-none">
                        {pct}%
                      </span>
                    )
                    // Don't show empty dot if brand profile exists (shared mode with full profile)
                    if (sharedMode && brandProfile) return null
                    return <span className="absolute -top-1 -right-1 w-2.5 h-2.5 bg-amber-500 rounded-full" />
                  })()}
                  {tab === 'video' && videoAnalyzing && <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse" />}
                  {tab === 'optimize' && optimizing && <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse" />}
                </button>
              )
            })}
          </div>
        </div>

        {/* URL entry + Go button — always on Analyze tab */}
        {activeTab === 'analyze' && <div className="max-w-xl mx-auto mb-8 flex items-center gap-2">
          <input
            type="text"
            value={url}
            onChange={e => setUrl(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter' && state !== 'analyzing') {
                if (saasEnabled && !user) { setLoginModal(true) }
                else if (sharedMode) { window.location.href = '/' }
                else { analyze() }
              }
            }}
            placeholder="example.com"
            disabled={state === 'analyzing'}
            className="flex-1 px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors disabled:opacity-50 text-center"
          />
          {state === 'analyzing' ? (
            <button
              onClick={cancel}
              className="px-6 py-2.5 bg-dark-800 border border-dark-700 text-white font-medium rounded-lg hover:bg-dark-700 transition-all cursor-pointer shrink-0"
            >
              Cancel
            </button>
          ) : (
            <button
              onClick={() => {
                if (saasEnabled && !user) { setLoginModal(true) }
                else if (sharedMode) { window.location.href = '/' }
                else { analyze() }
              }}
              disabled={!url.trim()}
              className="px-6 py-2.5 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer shrink-0"
            >
              Go
            </button>
          )}
        </div>}

        {/* Alert Banner — shown on all tabs when models are degraded */}
        {healthHistory.length > 0 && healthHistory[0].models.some(m => m.status !== 'available') && (
          <div className="max-w-2xl mx-auto mb-6 animate-fade-in">
            <div className={`border rounded-xl p-4 flex items-start gap-3 ${
              healthHistory[0].models.every(m => m.status !== 'available')
                ? 'bg-red-500/10 border-red-500/20'
                : 'bg-amber-500/10 border-amber-500/20'
            }`}>
              <svg className={`w-5 h-5 shrink-0 mt-0.5 ${
                healthHistory[0].models.every(m => m.status !== 'available') ? 'text-red-400' : 'text-amber-400'
              }`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
              </svg>
              <p className={`text-sm ${
                healthHistory[0].models.every(m => m.status !== 'available') ? 'text-red-300' : 'text-amber-300'
              }`}>
                {healthHistory[0].models.every(m => m.status !== 'available')
                  ? 'All Claude models are currently unavailable. Analysis requests will fail until service is restored.'
                  : (() => {
                      const degraded = healthHistory[0].models.filter(m => m.status !== 'available')
                      const available = healthHistory[0].models.filter(m => m.status === 'available')
                      const primary = healthHistory[0].models[0]
                      if (primary.status !== 'available') {
                        return `Primary model (${primary.name}) is ${primary.status}. Analysis will use ${available[0]?.name || 'fallback'} as fallback.`
                      }
                      return `${degraded.map(m => m.name).join(', ')} ${degraded.length > 1 ? 'are' : 'is'} currently ${degraded[0]?.status || 'unavailable'}.`
                    })()
                }
              </p>
            </div>
          </div>
        )}

        {/* ===== ANALYZE TAB ===== */}
        {activeTab === 'analyze' && (
          <>
            {/* Popular Brands */}
            {state !== 'done' && popularDomains.length > 0 && (
              <div className="max-w-2xl mx-auto mb-12 animate-fade-in">
                <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4 text-center">
                  Popular Brands
                </h3>
                <div className="grid gap-2">
                  {popularDomains.map(pd => (
                    <a
                      key={pd.share_id}
                      href={`/share/${pd.share_id}`}
                      className="w-full text-left bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-xl p-4 hover:border-primary-500/50 transition-all cursor-pointer group block"
                    >
                      <div className="flex items-center gap-3">
                        <div className={`w-10 h-10 rounded-lg flex items-center justify-center text-sm font-bold shrink-0 ${
                          pd.avg_score >= 80 ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30' :
                          pd.avg_score >= 60 ? 'bg-amber-500/15 text-amber-400 border border-amber-500/30' :
                          pd.avg_score >= 40 ? 'bg-orange-500/15 text-orange-400 border border-orange-500/30' :
                          'bg-red-500/15 text-red-400 border border-red-500/30'
                        }`}>
                          {pd.avg_score}
                        </div>
                        <div className="flex-1 min-w-0">
                          <p className="text-white text-sm font-medium group-hover:text-primary-300 transition-colors truncate">
                            {pd.brand_name || pd.domain}
                          </p>
                          <p className="text-dark-500 text-xs">{pd.domain} · {pd.report_count} {pd.report_count === 1 ? 'report' : 'reports'}</p>
                        </div>
                        <svg className="w-4 h-4 text-dark-600 group-hover:text-primary-400 transition-colors shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                          <path strokeLinecap="round" strokeLinejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" />
                        </svg>
                      </div>
                    </a>
                  ))}
                </div>
              </div>
            )}

            {/* Progress */}
            {state === 'analyzing' && statusMessages.length > 0 && (
              <div className="max-w-2xl mx-auto mb-12 animate-fade-in">
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                  <div className="space-y-3">
                    {statusMessages.map((msg, i) => (
                      <div key={i} className="flex items-center gap-3 text-sm">
                        {i < statusMessages.length - 1 ? (
                          <svg className="w-4 h-4 text-accent-emerald shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                          </svg>
                        ) : (
                          <div className="w-4 h-4 shrink-0 flex items-center justify-center">
                            <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse" />
                          </div>
                        )}
                        <span className={i < statusMessages.length - 1 ? 'text-dark-500' : 'text-dark-300'}>
                          {msg}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            )}

            {/* Error */}
            {state === 'error' && (
              <div className="max-w-2xl mx-auto mb-12 animate-fade-in">
                <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-4">
                  <p className="text-red-400 text-sm">{error}</p>
                </div>
              </div>
            )}

            {/* Results */}
            {result && (
              <div className="space-y-8 animate-slide-up">
                {/* Result metadata bar */}
                {resultMeta && (
                  <div className="flex items-center justify-between flex-wrap gap-3">
                    <div className="flex items-center gap-3 text-sm text-dark-400">
                      {resultMeta.createdAt && (
                        <span>
                          Analyzed {new Date(resultMeta.createdAt).toLocaleDateString(undefined, {
                            month: 'short', day: 'numeric', year: 'numeric',
                            hour: '2-digit', minute: '2-digit',
                          })}
                        </span>
                      )}
                      {resultMeta.model && (
                        <>
                          <span className="text-dark-600">·</span>
                          <span>using {resultMeta.model}</span>
                        </>
                      )}
                      {resultMeta.cached && (
                        <span className="text-xs px-2 py-0.5 rounded-full bg-primary-500/15 text-primary-400 border border-primary-500/20 font-medium">
                          Cached result
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      {saasEnabled && user && (user.role === 'owner' || user.role === 'admin') && selectedDomain && (
                        <button
                          onClick={() => { setDomainShareState(null); setShareModalDomain(selectedDomain); fetchDomainShare(selectedDomain) }}
                          className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all cursor-pointer flex items-center gap-1.5"
                          title="Share this domain"
                        >
                          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M7.217 10.907a2.25 2.25 0 1 0 0 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186 9.566-5.314m-9.566 7.5 9.566 5.314m0-12.814a2.25 2.25 0 1 0 0-2.186m0 2.186a2.25 2.25 0 1 0 0 2.186" /></svg>
                          Share
                        </button>
                      )}
                      {!readOnly && (
                        <button
                          onClick={() => analyze(true)}
                          disabled={state === 'analyzing'}
                          className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all disabled:opacity-50 cursor-pointer"
                        >
                          Re-analyze
                        </button>
                      )}
                    </div>
                  </div>
                )}

                {/* Brand Intelligence Indicator */}
                {resultMeta && (() => {
                  const domain = url.trim()
                  const brandEntry = brandList.find(b => b.domain === domain)
                  const brandNewer = brandEntry && resultMeta.brandProfileUpdatedAt && new Date(brandEntry.updated_at) > new Date(resultMeta.brandProfileUpdatedAt)
                  const noBrandUsed = !resultMeta.brandContextUsed
                  if (resultMeta.brandContextUsed && !brandNewer) {
                    return (
                      <div className="flex items-center gap-2 px-4 py-2.5 bg-emerald-500/10 border border-emerald-500/20 rounded-xl">
                        <svg className="w-4 h-4 text-emerald-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" /></svg>
                        <span className="text-emerald-400 text-xs font-medium">Brand Intelligence Active</span>
                      </div>
                    )
                  }
                  if (brandEntry && (noBrandUsed || brandNewer)) {
                    return (
                      <div className="flex items-center justify-between gap-3 px-4 py-2.5 bg-amber-500/10 border border-amber-500/20 rounded-xl">
                        <div className="flex items-center gap-2">
                          <svg className="w-4 h-4 text-amber-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" /></svg>
                          <span className="text-amber-400 text-xs font-medium">
                            {brandNewer ? 'Brand Intelligence has been updated since this analysis' : 'Brand Intelligence available but not used in this analysis'}
                          </span>
                        </div>
                        {!readOnly && (
                          <button
                            onClick={() => analyze(true)}
                            disabled={state === 'analyzing'}
                            className="text-xs px-3 py-1.5 bg-amber-500/10 border border-amber-500/30 text-amber-400 rounded-lg hover:bg-amber-500/20 transition-all disabled:opacity-50 cursor-pointer whitespace-nowrap shrink-0"
                          >
                            Re-run with Brand Intel
                          </button>
                        )}
                      </div>
                    )
                  }
                  if (!brandEntry && !readOnly) {
                    return (
                      <div className="flex items-center justify-between gap-3 px-4 py-2.5 bg-dark-900/50 border border-dark-800 rounded-xl">
                        <span className="text-dark-500 text-xs">No Brand Intelligence set up for this domain</span>
                        <button
                          onClick={() => { startNewBrand(domain); setActiveTab('brand') }}
                          className="text-xs text-primary-400 hover:text-primary-300 transition-colors cursor-pointer whitespace-nowrap shrink-0"
                        >
                          Set up Brand Intel
                        </button>
                      </div>
                    )
                  }
                  return null
                })()}

                {/* Summary Card */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                  <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-3">
                    Site Summary
                  </h3>
                  <p className="text-dark-200 text-lg leading-relaxed">{result.site_summary}</p>
                </div>

                {/* Questions */}
                <div>
                  <div className="flex items-center justify-between mb-5">
                    <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest">
                      Questions People Ask ({result.questions.length})
                    </h3>
                    <div className="flex gap-2 flex-wrap justify-end">
                      {categories.map(cat => (
                        <span
                          key={cat}
                          className={`text-xs px-2 py-0.5 rounded-full border font-medium ${categoryColors[cat]}`}
                        >
                          {cat}
                        </span>
                      ))}
                    </div>
                  </div>

                  {!readOnly && (
                    <label className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={optimizeAutoArchive}
                        onChange={e => setOptimizeAutoArchive(e.target.checked)}
                        className="w-3.5 h-3.5 rounded border-dark-600 bg-dark-800 text-primary-500 focus:ring-primary-500/30 cursor-pointer"
                      />
                      <span className="text-dark-400 text-xs">Automatically archive incomplete optimization recommendations when re-running a question</span>
                    </label>
                  )}

                  <div className="grid gap-3">
                    {result.questions.map((q, i) => (
                      <div
                        key={i}
                        className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-5 hover:border-dark-700 transition-colors group"
                      >
                        <div className="flex items-start justify-between gap-4 mb-2">
                          <p className="text-white font-medium leading-snug group-hover:text-primary-300 transition-colors">
                            {q.question}
                          </p>
                          <div className="flex items-center gap-2 shrink-0">
                            {optScores[i] !== undefined && (
                              <span className={`text-xs px-1.5 py-0.5 rounded font-semibold ${
                                optScores[i] >= 80 ? 'bg-emerald-500/20 text-emerald-400' :
                                optScores[i] >= 60 ? 'bg-amber-500/20 text-amber-400' :
                                optScores[i] >= 40 ? 'bg-orange-500/20 text-orange-400' :
                                'bg-red-500/20 text-red-400'
                              }`}>
                                {optScores[i]}
                              </span>
                            )}
                            {q.brand_status === 'aspirational' && (
                              <span className="text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider bg-amber-500/20 text-amber-400 border border-amber-500/30">
                                Aspirational
                              </span>
                            )}
                            {q.brand_status === 'missing' && (
                              <span className="text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider bg-rose-500/20 text-rose-400 border border-rose-500/30">
                                Missing?
                              </span>
                            )}
                            <span
                              className={`text-xs px-2 py-0.5 rounded-full border font-medium whitespace-nowrap ${
                                categoryColors[q.category] || 'bg-dark-700/50 text-dark-400 border-dark-600'
                              }`}
                            >
                              {q.category}
                            </span>
                          </div>
                        </div>
                        <p className="text-dark-400 text-sm leading-relaxed">{q.relevance}</p>
                        <div className="mt-2.5 flex items-center justify-between gap-3">
                          <div className="flex flex-wrap gap-1.5">
                            {q.page_urls && q.page_urls.map((pageUrl, j) => (
                              <a
                                key={j}
                                href={pageUrl}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-xs text-primary-400 hover:text-primary-300 bg-primary-500/10 border border-primary-500/20 rounded-full px-2.5 py-0.5 transition-colors"
                              >
                                {safePathname(pageUrl)}
                              </a>
                            ))}
                          </div>
                          {resultMeta?.id && (
                            <button
                              onClick={() => handleQuestionClick(i)}
                              disabled={optimizing}
                              className={`text-xs px-3 py-1 rounded-lg transition-all disabled:opacity-50 cursor-pointer whitespace-nowrap shrink-0 ${
                                readOnly
                                  ? optScores[i] !== undefined
                                    ? 'bg-primary-500/10 border border-primary-500/20 text-primary-300 hover:bg-primary-500/20'
                                    : 'bg-dark-800 border border-dark-700 text-dark-400 hover:bg-dark-700'
                                  : 'bg-accent-purple/10 border border-accent-purple/20 text-purple-300 hover:bg-accent-purple/20 hover:text-purple-200'
                              }`}
                            >
                              {optimizing && optimizationMeta?.questionIndex === i
                                ? 'Optimizing...'
                                : readOnly
                                  ? optScores[i] !== undefined ? 'View Report' : 'View'
                                  : optScores[i] !== undefined ? 'View Report' : 'Optimize'}
                            </button>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                {/* Crawled Pages */}
                {result.crawled_pages && result.crawled_pages.length > 0 && (
                  <div>
                    <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                      Pages Crawled ({result.crawled_pages.length})
                    </h3>
                    <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-5">
                      <div className="space-y-2">
                        {result.crawled_pages.map((page, i) => (
                          <div key={i} className="flex items-center gap-3">
                            <a
                              href={page.url}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-sm text-primary-400 hover:text-primary-300 transition-colors truncate"
                            >
                              {page.url}
                            </a>
                            {page.title && (
                              <span className="text-dark-500 text-xs shrink-0">
                                {page.title}
                              </span>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )}
          </>
        )}

        {/* ===== OPTIMIZE TAB ===== */}
        {activeTab === 'optimize' && (
          <div className="max-w-2xl mx-auto animate-fade-in space-y-6">

            {/* === Running view: SSE progress === */}
            {optimizeView === 'running' && (
              <>
                {optimizing && optimizeMessages.length > 0 && (
                  <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                    <div className="flex items-center justify-between mb-4">
                      <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest">
                        Analyzing LLM Visibility
                      </h3>
                      <button
                        onClick={cancelOptimize}
                        className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all cursor-pointer"
                      >
                        Cancel
                      </button>
                    </div>
                    <div className="space-y-3">
                      {optimizeMessages.map((msg, i) => (
                        <div key={i} className="flex items-center gap-3 text-sm">
                          {i < optimizeMessages.length - 1 ? (
                            <svg className="w-4 h-4 text-accent-emerald shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                            </svg>
                          ) : (
                            <div className="w-4 h-4 shrink-0 flex items-center justify-center">
                              <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse" />
                            </div>
                          )}
                          <span className={i < optimizeMessages.length - 1 ? 'text-dark-500' : 'text-dark-300'}>
                            {msg}
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                {optimizeError && (
                  <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-4">
                    <p className="text-red-400 text-sm">{optimizeError}</p>
                    <button
                      onClick={() => setOptimizeView('list')}
                      className="text-xs text-dark-400 hover:text-white mt-2 cursor-pointer"
                    >
                      Back to reports
                    </button>
                  </div>
                )}
              </>
            )}

            {/* === List view: past optimization reports === */}
            {optimizeView === 'list' && (
              <>
                {optListLoading ? (
                  <div className="flex justify-center py-16">
                    <div className="w-6 h-6 border-2 border-dark-700 border-t-primary-500 rounded-full animate-spin" />
                  </div>
                ) : (() => {
                  const filtered = optList.filter(o => domainKey(o.domain) === domainKey(selectedDomain))
                  const avgScore = filtered.length > 0 ? Math.round(filtered.reduce((s, o) => s + o.overall_score, 0) / filtered.length) : 0
                  const canonicalDomain = filtered[0]?.domain || selectedDomain
                  const status = summaryStatuses[canonicalDomain]
                  return filtered.length === 0 ? (
                    <div className="text-center text-dark-500 py-16">
                      No optimization reports for this domain yet.
                    </div>
                  ) : (
                    <div className="space-y-4">
                      {/* Domain header with summary */}
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3">
                          <div className={`w-10 h-10 rounded-xl flex items-center justify-center text-sm font-bold shrink-0 border ${
                            avgScore >= 80 ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' :
                            avgScore >= 60 ? 'bg-amber-500/15 text-amber-400 border-amber-500/30' :
                            avgScore >= 40 ? 'bg-orange-500/15 text-orange-400 border-orange-500/30' :
                            'bg-red-500/15 text-red-400 border-red-500/30'
                          }`}>
                            {avgScore}
                          </div>
                          <div>
                            <h4 className="text-white font-medium text-sm">{selectedDomain}</h4>
                            <span className="text-xs text-dark-500">{filtered.length} {filtered.length === 1 ? 'report' : 'reports'}</span>
                          </div>
                        </div>
                        {(readOnly ? activeSummary : true) && (
                          <button
                            onClick={() => readOnly ? setOptimizeView('summary') : handleSummaryClick(canonicalDomain)}
                            className={`text-xs px-3 py-1.5 rounded-lg border transition-all cursor-pointer flex items-center gap-1.5 ${
                              readOnly || (status?.exists && !status?.stale)
                                ? 'bg-primary-500/10 border-primary-500/30 text-primary-400 hover:bg-primary-500/20'
                                : status?.stale
                                ? 'bg-amber-500/10 border-amber-500/30 text-amber-400 hover:bg-amber-500/20'
                                : 'bg-dark-800 border-dark-700 text-dark-400 hover:bg-dark-700 hover:text-white'
                            }`}
                          >
                            Summary
                            {!readOnly && status?.stale && (
                              <span className="w-1.5 h-1.5 rounded-full bg-amber-400" />
                            )}
                          </button>
                        )}
                      </div>
                      {/* Report cards */}
                      <div className="space-y-2">
                        {filtered.map(item => (
                          <button
                            key={item.id}
                            onClick={() => loadOptimizationDetail(item.id)}
                            className="w-full text-left bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-xl p-4 hover:border-primary-500/40 transition-all cursor-pointer group"
                          >
                            <div className="flex items-start gap-3">
                              <div className={`w-9 h-9 rounded-lg flex items-center justify-center text-sm font-bold shrink-0 ${
                                item.overall_score >= 80 ? 'bg-emerald-500/15 text-emerald-400' :
                                item.overall_score >= 60 ? 'bg-amber-500/15 text-amber-400' :
                                item.overall_score >= 40 ? 'bg-orange-500/15 text-orange-400' :
                                'bg-red-500/15 text-red-400'
                              }`}>
                                {item.overall_score}
                              </div>
                              <div className="flex-1 min-w-0">
                                <p className="text-white text-sm leading-snug group-hover:text-primary-300 transition-colors line-clamp-2">
                                  {item.question}
                                </p>
                                <div className="flex items-center gap-2 mt-1 text-xs text-dark-500">
                                  <span>{fmtDate(item.created_at)}</span>
                                  {item.model && (
                                    <>
                                      <span className="text-dark-600">·</span>
                                      <span>via {item.model}</span>
                                    </>
                                  )}
                                  {item.brand_context_used ? (
                                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-emerald-500/15 text-emerald-400 border border-emerald-500/20 font-medium">Brand Intel</span>
                                  ) : (
                                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-dark-700/50 text-dark-500 border border-dark-600 font-medium">No Brand Intel</span>
                                  )}
                                </div>
                              </div>
                            </div>
                          </button>
                        ))}
                      </div>
                    </div>
                  )
                })()}
              </>
            )}

            {/* === Generating summary view === */}
            {optimizeView === 'generating-summary' && (
              <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest">
                    Generating Domain Summary
                  </h3>
                </div>
                {summaryMessages.length > 0 && (
                  <div className="space-y-3">
                    {summaryMessages.map((msg, i) => (
                      <div key={i} className="flex items-center gap-3 text-sm">
                        {i < summaryMessages.length - 1 ? (
                          <svg className="w-4 h-4 text-accent-emerald shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                          </svg>
                        ) : (
                          <div className="w-4 h-4 shrink-0 flex items-center justify-center">
                            <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse" />
                          </div>
                        )}
                        <span className={i < summaryMessages.length - 1 ? 'text-dark-500' : 'text-dark-300'}>{msg}</span>
                      </div>
                    ))}
                  </div>
                )}
                {!generatingSummary && summaryMessages.some(m => m.startsWith('Error')) && (
                  <button onClick={() => setOptimizeView('list')} className="text-xs text-dark-400 hover:text-white mt-4 cursor-pointer">
                    Back to reports
                  </button>
                )}
              </div>
            )}

            {/* === Summary detail view === */}
            {optimizeView === 'summary' && (
              <>
                <button
                  onClick={() => { setOptimizeView('list'); fetchOptList() }}
                  className="text-xs text-dark-400 hover:text-white transition-colors cursor-pointer flex items-center gap-1"
                >
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" />
                  </svg>
                  All reports
                </button>

                {!activeSummary ? (
                  /* No summary exists */
                  readOnly ? (
                    <div className="text-center text-dark-500 py-16">
                      No summary report available for this domain yet.
                    </div>
                  ) : (
                    <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-8 text-center">
                      <h3 className="text-white font-semibold text-lg mb-2">Domain Summary for {activeSummaryDomain}</h3>
                      <p className="text-dark-400 text-sm mb-6 max-w-md mx-auto">
                        Generate a comprehensive summary that synthesizes all optimization reports and brand intelligence for this domain into a unified strategic overview.
                      </p>
                      <button
                        onClick={() => generateSummary(activeSummaryDomain)}
                        className="px-5 py-2.5 bg-gradient-to-r from-primary-600 to-primary-500 text-white text-sm font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all cursor-pointer"
                      >
                        Generate Summary Report
                      </button>
                    </div>
                  )
                ) : (
                  /* Summary exists — full detail */
                  <div className="space-y-4">
                    {/* Header */}
                    <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                      <div className="flex items-center justify-between mb-3">
                        <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest">
                          Domain Summary
                        </h3>
                        {!readOnly && (
                          <div className="flex items-center gap-2">
                            <button
                              onClick={() => generateSummary(activeSummary.domain)}
                              className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all cursor-pointer"
                            >
                              Regenerate
                            </button>
                          </div>
                        )}
                      </div>
                      <h2 className="text-white font-medium text-lg mb-2">{activeSummary.domain}</h2>
                      <div className="flex items-center gap-3 text-xs text-dark-500">
                        <span>{activeSummary.report_count} reports analyzed</span>
                        <span className="text-dark-600">·</span>
                        <span>Generated {new Date(activeSummary.generated_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>
                        {activeSummary.model && (
                          <>
                            <span className="text-dark-600">·</span>
                            <span>via {activeSummary.model}</span>
                          </>
                        )}
                      </div>
                    </div>

                    {/* Stale banner */}
                    {activeSummaryStale && !readOnly && (
                      <div className="flex items-center justify-between bg-amber-500/10 border border-amber-500/20 rounded-xl px-4 py-3">
                        <span className="text-amber-400 text-sm">New reports have been added since this summary was generated.</span>
                        <button
                          onClick={() => generateSummary(activeSummary.domain)}
                          className="text-xs px-3 py-1.5 bg-amber-500/20 border border-amber-500/30 text-amber-400 rounded-lg hover:bg-amber-500/30 transition-all cursor-pointer whitespace-nowrap ml-3"
                        >
                          Regenerate
                        </button>
                      </div>
                    )}

                    {/* Score overview */}
                    <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                      <div className="flex items-center gap-4 mb-4">
                        <div className={`w-14 h-14 rounded-xl flex items-center justify-center text-xl font-bold border ${
                          activeSummary.result.average_score >= 80 ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' :
                          activeSummary.result.average_score >= 60 ? 'bg-amber-500/15 text-amber-400 border-amber-500/30' :
                          activeSummary.result.average_score >= 40 ? 'bg-orange-500/15 text-orange-400 border-orange-500/30' :
                          'bg-red-500/15 text-red-400 border-red-500/30'
                        }`}>
                          {activeSummary.result.average_score}
                        </div>
                        <div>
                          <div className="text-white font-medium">Average Score</div>
                          <div className="text-dark-500 text-xs">Range: {activeSummary.result.score_range[0]} – {activeSummary.result.score_range[1]}</div>
                        </div>
                      </div>
                      {activeSummary.result.dimension_trends && (
                        <div className="grid grid-cols-2 gap-3">
                          {[
                            { key: 'content_authority', label: 'Content Authority' },
                            { key: 'structural_optimization', label: 'Structural Optimization' },
                            { key: 'source_authority', label: 'Source Authority' },
                            { key: 'knowledge_persistence', label: 'Knowledge Persistence' },
                          ].map(dim => {
                            const score = activeSummary.result.dimension_trends[dim.key] ?? 0
                            return (
                              <div key={dim.key} className="bg-dark-800/50 rounded-lg p-3">
                                <div className="flex items-center justify-between mb-1.5">
                                  <span className="text-dark-400 text-xs">{dim.label}</span>
                                  <span className={`text-xs font-bold ${score >= 70 ? 'text-emerald-400' : score >= 50 ? 'text-amber-400' : 'text-red-400'}`}>{score}</span>
                                </div>
                                <div className="w-full h-1.5 bg-dark-700 rounded-full overflow-hidden">
                                  <div className={`h-full rounded-full transition-all ${score >= 70 ? 'bg-emerald-500' : score >= 50 ? 'bg-amber-500' : 'bg-red-500'}`} style={{ width: `${score}%` }} />
                                </div>
                              </div>
                            )
                          })}
                        </div>
                      )}
                    </div>

                    {/* Executive Summary */}
                    <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                      <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-3">Executive Summary</h3>
                      <div className="text-dark-300 text-sm leading-relaxed whitespace-pre-line">
                        {activeSummary.result.executive_summary}
                      </div>
                    </div>

                    {/* Themes */}
                    {activeSummary.result.themes?.length > 0 && (
                      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                        <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">Key Themes</h3>
                        <div className="space-y-3">
                          {activeSummary.result.themes.map((theme, i) => (
                            <div key={i} className="bg-dark-800/50 rounded-xl p-4">
                              <h4 className="text-white font-medium text-sm mb-1">{theme.title}</h4>
                              <p className="text-dark-400 text-sm leading-relaxed">{theme.description}</p>
                              {theme.report_refs?.length > 0 && (
                                <div className="mt-2 text-xs text-dark-500">From reports: {theme.report_refs.join(', ')}</div>
                              )}
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                    {/* Priority Action Items */}
                    {activeSummary.result.action_items?.length > 0 && (
                      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                        <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">Priority Actions</h3>
                        <div className="space-y-2">
                          {activeSummary.result.action_items.map((item, i) => (
                            <div key={i} className="flex items-start gap-3 bg-dark-800/50 rounded-xl p-4">
                              <span className={`text-[10px] px-2 py-0.5 rounded font-bold uppercase shrink-0 mt-0.5 ${
                                item.priority === 'high' ? 'bg-red-500/15 text-red-400 border border-red-500/20' :
                                item.priority === 'medium' ? 'bg-amber-500/15 text-amber-400 border border-amber-500/20' :
                                'bg-emerald-500/15 text-emerald-400 border border-emerald-500/20'
                              }`}>
                                {item.priority}
                              </span>
                              <div className="flex-1 min-w-0">
                                <p className="text-white text-sm">{item.action}</p>
                                {item.expected_impact && (
                                  <p className="text-dark-500 text-xs mt-1">{item.expected_impact}</p>
                                )}
                                <div className="flex items-center gap-2 mt-1 text-xs text-dark-600">
                                  {item.dimension && <span>{item.dimension.replace(/_/g, ' ')}</span>}
                                  {item.source_reports?.length > 0 && (
                                    <>
                                      <span>·</span>
                                      <span>From reports: {item.source_reports.join(', ')}</span>
                                    </>
                                  )}
                                </div>
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                    {/* Contradictions */}
                    {activeSummary.result.contradictions?.length > 0 && (
                      <div className="bg-dark-900/50 backdrop-blur-sm border border-amber-500/20 rounded-2xl p-6">
                        <h3 className="text-xs font-semibold text-amber-400 uppercase tracking-widest mb-4">Contradictions to Resolve</h3>
                        <div className="space-y-3">
                          {activeSummary.result.contradictions.map((c, i) => (
                            <div key={i} className="bg-amber-500/5 border border-amber-500/10 rounded-xl p-4">
                              <h4 className="text-white font-medium text-sm mb-2">{c.topic}</h4>
                              <div className="space-y-1 mb-2">
                                {c.positions.map((pos, j) => (
                                  <p key={j} className="text-dark-400 text-sm pl-3 border-l-2 border-amber-500/30">{pos}</p>
                                ))}
                              </div>
                              <p className="text-dark-300 text-sm"><span className="text-amber-400 font-medium">Recommendation:</span> {c.recommendation}</p>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                )}
              </>
            )}

            {/* === Detail view: full optimization report === */}
            {optimizeView === 'detail' && optimization && optimizationMeta && (
              <>
                {/* Back button */}
                <button
                  onClick={() => { setOptimizeView('list'); fetchOptList() }}
                  className="text-xs text-dark-400 hover:text-white transition-colors cursor-pointer flex items-center gap-1"
                >
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" />
                  </svg>
                  All reports
                </button>

                {/* Header */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                  <div className="flex items-center justify-between mb-3">
                    <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest">
                      LLM Visibility Analysis
                    </h3>
                    <div className="flex items-center gap-2">
                      {saasEnabled && user && (user.role === 'owner' || user.role === 'admin') && selectedDomain && (
                        <button
                          onClick={() => { setDomainShareState(null); setShareModalDomain(selectedDomain); fetchDomainShare(selectedDomain) }}
                          className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all cursor-pointer flex items-center gap-1.5"
                          title="Share this domain"
                        >
                          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M7.217 10.907a2.25 2.25 0 1 0 0 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186 9.566-5.314m-9.566 7.5 9.566 5.314m0-12.814a2.25 2.25 0 1 0 0-2.186m0 2.186a2.25 2.25 0 1 0 0 2.186" /></svg>
                          Share
                        </button>
                      )}
                      {!readOnly && resultMeta?.id && (
                        <button
                          onClick={() => optimizeQuestion(optimizationMeta.questionIndex, true)}
                          disabled={optimizing}
                          className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all disabled:opacity-50 cursor-pointer"
                        >
                          Re-analyze
                        </button>
                      )}
                      {!readOnly && (
                        <button
                          onClick={() => setConfirmDeleteOptimization(optimizationMeta.id)}
                          className="text-xs px-3 py-1.5 text-red-400 border border-red-500/30 rounded-lg hover:bg-red-500/10 transition-all cursor-pointer"
                        >
                          Delete
                        </button>
                      )}
                    </div>
                  </div>
                  <p className="text-white font-medium text-lg leading-snug mb-2">
                    {optimizationMeta.question}
                    {optimizationMeta.brandStatus === 'aspirational' && (
                      <span className="ml-2 text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider bg-amber-500/20 text-amber-400 border border-amber-500/30 align-middle">Aspirational</span>
                    )}
                    {optimizationMeta.brandStatus === 'missing' && (
                      <span className="ml-2 text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider bg-rose-500/20 text-rose-400 border border-rose-500/30 align-middle">Missing?</span>
                    )}
                  </p>
                  <div className="flex items-center gap-3 text-sm text-dark-400 flex-wrap">
                    <span>{selectedOpt?.domain || url}</span>
                    {optimizationMeta.createdAt && (
                      <>
                        <span className="text-dark-600">·</span>
                        <span>{new Date(optimizationMeta.createdAt).toLocaleDateString(undefined, {
                          month: 'short', day: 'numeric', year: 'numeric',
                          hour: '2-digit', minute: '2-digit',
                        })}</span>
                      </>
                    )}
                    {optimizationMeta.model && (
                      <>
                        <span className="text-dark-600">·</span>
                        <span>via {optimizationMeta.model}</span>
                      </>
                    )}
                    {optimizationMeta.cached && (
                      <span className="text-xs px-2 py-0.5 rounded-full bg-primary-500/15 text-primary-400 border border-primary-500/20 font-medium">
                        Cached
                      </span>
                    )}
                  </div>
                </div>

                {/* Brand Intelligence Warning */}
                {(() => {
                  const domain = selectedOpt?.domain || url
                  const brandEntry = brandList.find(b => b.domain === domain)
                  const brandNewer = brandEntry && optimizationMeta.brandProfileUpdatedAt && new Date(brandEntry.updated_at) > new Date(optimizationMeta.brandProfileUpdatedAt)
                  if (optimizationMeta.brandContextUsed && !brandNewer) {
                    return (
                      <div className="flex items-center gap-2 px-4 py-2.5 bg-emerald-500/10 border border-emerald-500/20 rounded-xl">
                        <svg className="w-4 h-4 text-emerald-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" /></svg>
                        <span className="text-emerald-400 text-xs font-medium">Brand Intelligence Active</span>
                      </div>
                    )
                  }
                  if (!optimizationMeta.brandContextUsed || brandNewer) {
                    return (
                      <div className="flex items-center justify-between gap-3 px-4 py-2.5 bg-amber-500/10 border border-amber-500/20 rounded-xl">
                        <div className="flex items-center gap-2 min-w-0">
                          <svg className="w-4 h-4 text-amber-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" /></svg>
                          <span className="text-amber-400 text-xs font-medium">
                            {brandNewer ? 'Brand Intelligence has been updated since this report was generated' : 'This report was generated without Brand Intelligence'}
                          </span>
                        </div>
                        {!readOnly && (
                          <div className="flex items-center gap-2 shrink-0">
                            {!brandEntry && (
                              <button
                                onClick={() => { startNewBrand(domain); setActiveTab('brand') }}
                                className="text-xs text-primary-400 hover:text-primary-300 transition-colors cursor-pointer whitespace-nowrap"
                              >
                                Set up Brand Intel
                              </button>
                            )}
                            {brandEntry && (
                              <button
                                onClick={() => { loadBrandProfile(domain); setActiveTab('brand') }}
                                className="text-xs text-primary-400 hover:text-primary-300 transition-colors cursor-pointer whitespace-nowrap"
                              >
                                View Brand Intel
                              </button>
                            )}
                            {brandEntry && resultMeta?.id && (
                              <button
                                onClick={() => optimizeQuestion(optimizationMeta.questionIndex, true)}
                                disabled={optimizing}
                                className="text-xs px-3 py-1.5 bg-amber-500/10 border border-amber-500/30 text-amber-400 rounded-lg hover:bg-amber-500/20 transition-all disabled:opacity-50 cursor-pointer whitespace-nowrap"
                              >
                                Re-run with Brand Intel
                              </button>
                            )}
                          </div>
                        )}
                      </div>
                    )
                  }
                  return null
                })()}

                {/* Overall Score */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6 flex items-center gap-6">
                  <div className={`w-20 h-20 rounded-2xl flex items-center justify-center text-2xl font-bold shrink-0 ${
                    optimization.overall_score >= 80 ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30' :
                    optimization.overall_score >= 60 ? 'bg-amber-500/15 text-amber-400 border border-amber-500/30' :
                    optimization.overall_score >= 40 ? 'bg-orange-500/15 text-orange-400 border border-orange-500/30' :
                    'bg-red-500/15 text-red-400 border border-red-500/30'
                  }`}>
                    {optimization.overall_score}
                  </div>
                  <div>
                    <h3 className="text-white font-semibold text-lg mb-1">
                      {optimization.overall_score >= 80 ? 'Strong Position' :
                       optimization.overall_score >= 60 ? 'Moderate — Room for Improvement' :
                       optimization.overall_score >= 40 ? 'Weak — Significant Improvements Needed' :
                       'Poor — Major Gaps to Address'}
                    </h3>
                    <p className="text-dark-400 text-sm">
                      Overall LLM Visibility Score based on content authority, structure, source authority, and knowledge persistence.
                    </p>
                  </div>
                </div>

                {/* Dimension Cards */}
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  {([
                    { key: 'content_authority' as const, name: 'Content Authority', weight: '30%' },
                    { key: 'structural_optimization' as const, name: 'Structural Optimization', weight: '20%' },
                    { key: 'source_authority' as const, name: 'Source Authority', weight: '30%' },
                    { key: 'knowledge_persistence' as const, name: 'Knowledge Persistence', weight: '20%' },
                  ] as const).map(dim => {
                    const d = optimization[dim.key]
                    return (
                      <div key={dim.key} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-5">
                        <div className="flex items-center justify-between mb-3">
                          <div>
                            <h4 className="text-white font-medium text-sm">{dim.name}</h4>
                            <span className="text-dark-500 text-xs">{dim.weight} weight</span>
                          </div>
                          <span className={`text-lg font-bold ${
                            d.score >= 80 ? 'text-emerald-400' :
                            d.score >= 60 ? 'text-amber-400' :
                            d.score >= 40 ? 'text-orange-400' :
                            'text-red-400'
                          }`}>{d.score}</span>
                        </div>
                        <div className="h-1.5 bg-dark-800 rounded-full mb-4 overflow-hidden">
                          <div
                            className={`h-full rounded-full transition-all ${
                              d.score >= 80 ? 'bg-emerald-500' :
                              d.score >= 60 ? 'bg-amber-500' :
                              d.score >= 40 ? 'bg-orange-500' :
                              'bg-red-500'
                            }`}
                            style={{ width: `${d.score}%` }}
                          />
                        </div>
                        {d.evidence && d.evidence.length > 0 && (
                          <div className="mb-3">
                            <p className="text-dark-500 text-xs font-semibold uppercase tracking-wider mb-1.5">Evidence</p>
                            <ul className="space-y-1">
                              {d.evidence.map((e, j) => (
                                <li key={j} className="text-dark-300 text-xs leading-relaxed flex gap-2">
                                  <span className="text-dark-600 shrink-0">-</span>
                                  <span>{e}</span>
                                </li>
                              ))}
                            </ul>
                          </div>
                        )}
                        {d.improvements && d.improvements.length > 0 && (
                          <div>
                            <p className="text-dark-500 text-xs font-semibold uppercase tracking-wider mb-1.5">Improvements</p>
                            <ul className="space-y-1">
                              {d.improvements.map((imp, j) => (
                                <li key={j} className="text-primary-400 text-xs leading-relaxed flex gap-2">
                                  <span className="text-primary-600 shrink-0">+</span>
                                  <span>{imp}</span>
                                </li>
                              ))}
                            </ul>
                          </div>
                        )}
                      </div>
                    )
                  })}
                </div>

                {/* Competitors */}
                {optimization.competitors && optimization.competitors.length > 0 && (
                  <div>
                    <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                      Competing Sources ({optimization.competitors.length})
                    </h3>
                    <div className="grid gap-3">
                      {optimization.competitors.map((comp, i) => (
                        <div key={i} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-4 flex items-center gap-4">
                          <span className={`text-lg font-bold shrink-0 w-10 text-center ${
                            comp.score_estimate >= 80 ? 'text-emerald-400' :
                            comp.score_estimate >= 60 ? 'text-amber-400' :
                            comp.score_estimate >= 40 ? 'text-orange-400' :
                            'text-red-400'
                          }`}>{comp.score_estimate}</span>
                          <div className="min-w-0">
                            <p className="text-white font-medium text-sm truncate">{comp.domain}</p>
                            <p className="text-dark-400 text-xs leading-relaxed">{comp.strengths}</p>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* Recommendations with todo status */}
                {optimization.recommendations && optimization.recommendations.length > 0 && (
                  <div>
                    <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                      Recommendations
                    </h3>
                    <div className="space-y-3">
                      {optimization.recommendations.map((rec, i) => {
                        const matchingTodo = optTodos.find(t => t.action === rec.action)
                        return (
                          <div key={i} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-4">
                            <div className="flex items-center gap-2 mb-2">
                              <span className={`text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider ${
                                rec.priority === 'high' ? 'bg-red-500/20 text-red-400 border border-red-500/30' :
                                rec.priority === 'medium' ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30' :
                                'bg-dark-700/50 text-dark-400 border border-dark-600'
                              }`}>
                                {rec.priority}
                              </span>
                              <span className="text-dark-500 text-xs capitalize">{rec.dimension.replace(/_/g, ' ')}</span>
                              {matchingTodo && (
                                <span className={`text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider ml-auto ${
                                  matchingTodo.status === 'completed' ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' :
                                  matchingTodo.status === 'backlogged' ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30' :
                                  'bg-primary-500/20 text-primary-400 border border-primary-500/30'
                                }`}>
                                  {matchingTodo.status === 'todo' ? 'new' : matchingTodo.status}
                                </span>
                              )}
                            </div>
                            <p className="text-white text-sm font-medium mb-1">{rec.action}</p>
                            <p className="text-dark-400 text-xs">{rec.expected_impact}</p>
                          </div>
                        )
                      })}
                    </div>
                  </div>
                )}
              </>
            )}
          </div>
        )}

        {/* ===== TODOS TAB ===== */}
        {activeTab === 'todos' && (
          <div className="max-w-2xl mx-auto animate-fade-in space-y-6">
            {/* Sub-tabs + sort toggle */}
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <div className="inline-flex bg-dark-900/50 border border-dark-800 rounded-lg p-1">
                  {(['todo', 'completed', 'backlogged', 'archived'] as const).map(st => (
                    <button
                      key={st}
                      onClick={() => setTodoSubTab(st)}
                      className={`px-4 py-1.5 rounded-md text-xs font-medium transition-all cursor-pointer capitalize ${
                        todoSubTab === st
                          ? 'bg-dark-700 text-white'
                          : 'text-dark-400 hover:text-white'
                      }`}
                    >
                      {st === 'todo' ? 'To-Do' : st === 'completed' ? 'Completed' : st === 'backlogged' ? 'Backlogged' : 'Archived'}
                      <span className="ml-1.5 text-dark-500">
                        {todos.filter(t => t.status === st && domainKey(t.domain) === domainKey(selectedDomain)).length}
                      </span>
                    </button>
                  ))}
                </div>
                <div className="flex items-center gap-2">
                  {saasEnabled && user && (user.role === 'owner' || user.role === 'admin') && selectedDomain && (
                    <button
                      onClick={() => { setDomainShareState(null); setShareModalDomain(selectedDomain); fetchDomainShare(selectedDomain) }}
                      className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all cursor-pointer flex items-center gap-1.5"
                      title="Share this domain"
                    >
                      <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M7.217 10.907a2.25 2.25 0 1 0 0 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186 9.566-5.314m-9.566 7.5 9.566 5.314m0-12.814a2.25 2.25 0 1 0 0-2.186m0 2.186a2.25 2.25 0 1 0 0 2.186" /></svg>
                      Share
                    </button>
                  )}
                  <button
                    onClick={fetchTodos}
                    disabled={todosLoading}
                    className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all disabled:opacity-50 cursor-pointer"
                  >
                  {todosLoading ? 'Loading...' : 'Refresh'}
                  </button>
                </div>
              </div>
              {/* Sort mode toggle */}
              <div className="flex items-center gap-2">
                <span className="text-dark-500 text-xs">Sort by</span>
                <div className="inline-flex bg-dark-900/50 border border-dark-800 rounded-md p-0.5">
                  {(['priority', 'question'] as const).map(mode => (
                    <button
                      key={mode}
                      onClick={() => setTodoSortMode(mode)}
                      className={`px-3 py-1 rounded text-xs font-medium transition-all cursor-pointer capitalize ${
                        todoSortMode === mode
                          ? 'bg-dark-700 text-white'
                          : 'text-dark-500 hover:text-white'
                      }`}
                    >
                      {mode}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            {/* Brand Intel virtual todo for selected domain */}
            {(() => {
              const brand = brandList.find(b => domainKey(b.domain) === domainKey(selectedDomain))
              const completeness = brand?.completeness ?? 0
              const brandName = brand?.brand_name || selectedDomain
              const brandIntelItems = [{ domain: selectedDomain, completeness, brandName }]

              // Items with <100% go in "todo", items with 100% go in "completed"
              const incompleteBrandItems = brandIntelItems.filter(b => b.completeness < 100)
              const completeBrandItems = brandIntelItems.filter(b => b.completeness === 100)

              return (
                <>
                  {/* Sub-tab counts with brand intel virtual items */}
                  <div className="flex items-center gap-2 text-xs text-dark-500 -mt-2 mb-2">
                    {incompleteBrandItems.length > 0 && todoSubTab === 'todo' && (
                      <span className="text-amber-400">Includes {incompleteBrandItems.length} Brand Intelligence {incompleteBrandItems.length === 1 ? 'item' : 'items'}</span>
                    )}
                    {completeBrandItems.length > 0 && todoSubTab === 'completed' && (
                      <span className="text-emerald-400">{completeBrandItems.length} Brand Intelligence {completeBrandItems.length === 1 ? 'profile' : 'profiles'} at 100%</span>
                    )}
                  </div>

                  {/* Brand Intel Virtual Todo Items */}
                  {todoSubTab === 'todo' && incompleteBrandItems.length > 0 && (
                    <div className="space-y-2 mb-4">
                      <p className="text-xs font-semibold uppercase tracking-widest text-red-400">
                        High Priority ({incompleteBrandItems.length + todos.filter(t => t.status === 'todo' && t.priority === 'high').length})
                      </p>
                      {incompleteBrandItems.map(item => (
                        <div
                          key={`brand-intel-${item.domain}`}
                          className="bg-amber-500/5 border border-amber-500/20 rounded-xl p-4 flex items-start gap-3"
                        >
                          <div className="w-5 h-5 rounded border-2 border-amber-500/50 shrink-0 mt-0.5 flex items-center justify-center bg-amber-500/10">
                            <svg className="w-3 h-3 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" /></svg>
                          </div>
                          <div className="flex-1 min-w-0">
                            <p className="text-white text-sm font-medium leading-snug">
                              Complete Brand Intelligence for {item.brandName}
                            </p>
                            <p className="text-dark-400 text-xs mt-1">
                              {item.completeness === 0
                                ? 'No brand profile set up yet. Brand Intelligence improves analysis and optimization accuracy.'
                                : `Brand profile is ${item.completeness}% complete. Complete it for better analysis and optimization results.`
                              }
                            </p>
                            <div className="flex items-center gap-3 mt-2">
                              <span className="text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider bg-red-500/20 text-red-400 border border-red-500/30">high</span>
                              <div className="flex items-center gap-1.5 flex-1 max-w-32">
                                <div className="h-1.5 bg-dark-800 rounded-full flex-1 overflow-hidden">
                                  <div className={`h-full rounded-full transition-all ${item.completeness >= 70 ? 'bg-emerald-500' : item.completeness >= 40 ? 'bg-amber-500' : 'bg-red-500'}`} style={{ width: `${item.completeness}%` }} />
                                </div>
                                <span className="text-dark-500 text-[10px]">{item.completeness}%</span>
                              </div>
                            </div>
                          </div>
                          <button
                            onClick={() => {
                              const brand = brandList.find(b => b.domain === item.domain)
                              if (brand) { loadBrandProfile(item.domain); } else { startNewBrand(item.domain); }
                              setActiveTab('brand')
                            }}
                            className="text-xs px-3 py-1.5 bg-amber-500/10 border border-amber-500/30 text-amber-400 rounded-lg hover:bg-amber-500/20 transition-all cursor-pointer whitespace-nowrap shrink-0"
                          >
                            {item.completeness === 0 ? 'Set Up' : 'Complete'}
                          </button>
                        </div>
                      ))}
                    </div>
                  )}

                  {todoSubTab === 'completed' && completeBrandItems.length > 0 && (
                    <div className="space-y-2 mb-4">
                      {completeBrandItems.map(item => (
                        <div
                          key={`brand-intel-done-${item.domain}`}
                          className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-xl p-4 flex items-start gap-3"
                        >
                          <div className="w-5 h-5 rounded border-2 border-emerald-500/50 shrink-0 mt-0.5 flex items-center justify-center bg-emerald-500/20">
                            <svg className="w-3 h-3 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                          </div>
                          <div className="flex-1 min-w-0">
                            <p className="text-dark-500 text-sm font-medium leading-snug line-through">
                              Complete Brand Intelligence for {item.brandName}
                            </p>
                            <div className="flex items-center gap-2 mt-1">
                              <span className="text-emerald-500 text-xs">100% complete</span>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </>
              )
            })()}

            {/* Todo list */}
            {todosLoading && todos.length === 0 ? (
              <div className="flex justify-center py-16">
                <div className="w-6 h-6 border-2 border-dark-700 border-t-primary-500 rounded-full animate-spin" />
              </div>
            ) : (() => {
              const filtered = todos.filter(t => t.status === todoSubTab && domainKey(t.domain) === domainKey(selectedDomain))
              if (filtered.length === 0) {
                // Don't show empty state for todo/completed if brand intel items filled the gap
                if (todoSubTab === 'backlogged') {
                  return <div className="text-center text-dark-500 py-16">No backlogged items.</div>
                }
                // For todo/completed, only show empty state if there are also no brand intel virtual items
                return null
              }

              const priorityOrder: Record<string, number> = { high: 0, medium: 1, low: 2 }

              /* ---- Sort by Priority: flat list ---- */
              if (todoSortMode === 'priority') {
                const sorted = [...filtered].sort((a, b) =>
                  (priorityOrder[a.priority] ?? 3) - (priorityOrder[b.priority] ?? 3)
                )

                // Group by priority level
                const byPriority: Record<string, TodoItem[]> = {}
                for (const t of sorted) {
                  const p = t.priority || 'low'
                  if (!byPriority[p]) byPriority[p] = []
                  byPriority[p].push(t)
                }

                const priorityLabels: Record<string, string> = { high: 'High Priority', medium: 'Medium Priority', low: 'Low Priority' }
                const priorityStyles: Record<string, string> = {
                  high: 'text-red-400',
                  medium: 'text-amber-400',
                  low: 'text-dark-400',
                }

                return (
                  <div className="space-y-6">
                    {['high', 'medium', 'low'].filter(p => byPriority[p]?.length).map(p => (
                      <div key={p}>
                        <p className={`text-xs font-semibold uppercase tracking-widest mb-3 ${priorityStyles[p]}`}>
                          {priorityLabels[p]} ({byPriority[p].length})
                        </p>
                        <div className="space-y-1.5">
                          {byPriority[p].map(todo => {
                            const isExpanded = expandedTodos.has(todo.id)
                            return (
                            <div
                              key={todo.id}
                              className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-xl overflow-hidden group hover:border-dark-700 transition-colors"
                            >
                              {/* Compact row — always visible */}
                              <div className="flex items-center gap-3 px-4 py-2.5">
                                {!readOnly && (
                                  <button
                                    onClick={(e) => { e.stopPropagation(); updateTodoStatus(todo.id, todo.status === 'completed' ? 'todo' : 'completed') }}
                                    className={`w-4.5 h-4.5 rounded border-2 shrink-0 flex items-center justify-center transition-all cursor-pointer ${
                                      todo.status === 'completed'
                                        ? 'bg-emerald-500/20 border-emerald-500/50'
                                        : 'border-dark-600 hover:border-primary-500'
                                    }`}
                                  >
                                    {todo.status === 'completed' && (
                                      <svg className="w-2.5 h-2.5 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                                      </svg>
                                    )}
                                  </button>
                                )}
                                <button
                                  onClick={() => setExpandedTodos(prev => { const next = new Set(prev); next.has(todo.id) ? next.delete(todo.id) : next.add(todo.id); return next })}
                                  className="flex-1 min-w-0 text-left cursor-pointer"
                                >
                                  <p className={`text-sm leading-snug truncate ${
                                    todo.status === 'completed' ? 'text-dark-500 line-through' : 'text-white'
                                  }`}>
                                    {todo.summary || todo.action}
                                  </p>
                                </button>
                                <div className="flex items-center gap-1.5 shrink-0">
                                  {todo.source_type === 'video' ? (
                                    <span className="text-[9px] px-1 py-0.5 rounded bg-purple-500/20 text-purple-300 border border-purple-500/30 font-medium">Video</span>
                                  ) : (
                                    <span className="text-[9px] px-1 py-0.5 rounded bg-primary-500/20 text-primary-300 border border-primary-500/30 font-medium">Site</span>
                                  )}
                                  <span className="text-dark-500 text-xs capitalize hidden sm:inline">{todo.dimension.replace(/_/g, ' ')}</span>
                                  <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                                    {(todo.optimization_id || todo.video_analysis_id) && (
                                      <button
                                        onClick={() => {
                                          if (todo.source_type === 'video') {
                                            loadVideoAnalysis(todo.domain)
                                            setActiveTab('video')
                                          } else {
                                            loadOptimizationDetail(todo.optimization_id)
                                            setActiveTab('optimize')
                                          }
                                        }}
                                        title="View report"
                                        className="p-1 text-dark-500 hover:text-primary-400 transition-colors cursor-pointer"
                                      >
                                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                          <path strokeLinecap="round" strokeLinejoin="round" d="M13.5 6H5.25A2.25 2.25 0 003 8.25v10.5A2.25 2.25 0 005.25 21h10.5A2.25 2.25 0 0018 18.75V10.5m-10.5 6L21 3m0 0h-5.25M21 3v5.25" />
                                        </svg>
                                      </button>
                                    )}
                                    {!readOnly && todo.status !== 'backlogged' && todo.status !== 'archived' && (
                                      <button onClick={() => updateTodoStatus(todo.id, 'backlogged')} title="Backlog" className="p-1 text-dark-500 hover:text-amber-400 transition-colors cursor-pointer">
                                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                                      </button>
                                    )}
                                    {!readOnly && todo.status !== 'archived' && (
                                      <button onClick={() => updateTodoStatus(todo.id, 'archived')} title="Archive" className="p-1 text-dark-500 hover:text-dark-300 transition-colors cursor-pointer">
                                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" /></svg>
                                      </button>
                                    )}
                                    {!readOnly && (todo.status === 'backlogged' || todo.status === 'archived') && (
                                      <button onClick={() => updateTodoStatus(todo.id, 'todo')} title="Move to To-Do" className="p-1 text-dark-500 hover:text-primary-400 transition-colors cursor-pointer">
                                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M9 15L3 9m0 0l6-6M3 9h12a6 6 0 010 12h-3" /></svg>
                                      </button>
                                    )}
                                  </div>
                                  <svg className={`w-3.5 h-3.5 text-dark-600 transition-transform cursor-pointer ${isExpanded ? 'rotate-180' : ''}`} onClick={() => setExpandedTodos(prev => { const next = new Set(prev); next.has(todo.id) ? next.delete(todo.id) : next.add(todo.id); return next })} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" /></svg>
                                </div>
                              </div>

                              {/* Expanded details */}
                              {isExpanded && (
                                <div className="px-4 pb-3 pt-0 border-t border-dark-800/50 space-y-2">
                                  <p className="text-dark-300 text-xs leading-relaxed mt-2">{todo.action}</p>
                                  <p className="text-dark-400 text-xs">{todo.expected_impact}</p>
                                  <div className="flex items-center gap-2 flex-wrap text-[10px]">
                                    <span className="text-dark-500 truncate max-w-xs" title={todo.question}>{todo.question}</span>
                                    {todo.created_at && (<><span className="text-dark-600">·</span><span className="text-dark-500">Created {fmtDate(todo.created_at)}</span></>)}
                                    {todo.backlogged_at && (<><span className="text-dark-600">·</span><span className="text-amber-400/70">Backlogged {fmtDate(todo.backlogged_at)}</span></>)}
                                    {todo.completed_at && (<><span className="text-dark-600">·</span><span className="text-dark-500">Done {fmtDate(todo.completed_at)}</span></>)}
                                    {todo.archived_at && (<><span className="text-dark-600">·</span><span className="text-dark-400/70">Archived {fmtDate(todo.archived_at)}</span></>)}
                                  </div>
                                </div>
                              )}
                            </div>
                          )})}

                        </div>
                      </div>
                    ))}
                  </div>
                )
              }

              /* ---- Sort by Question: grouped by domain+question ---- */
              const groups: Record<string, TodoItem[]> = {}
              for (const t of filtered) {
                const key = `${t.domain}|||${t.question}`
                if (!groups[key]) groups[key] = []
                groups[key].push(t)
              }

              return (
                <div className="space-y-6">
                  {Object.entries(groups).map(([key, items]) => {
                    const [, question] = key.split('|||')
                    return (
                      <div key={key}>
                        <div className="mb-3 flex items-start justify-between gap-3">
                          <div>
                            <p className="text-white font-medium text-sm leading-snug">{question}</p>
                          </div>
                          {(items[0]?.optimization_id || items[0]?.video_analysis_id) && (
                            <button
                              onClick={() => {
                                if (items[0].source_type === 'video') {
                                  loadVideoAnalysis(items[0].domain)
                                  setActiveTab('video')
                                } else {
                                  loadOptimizationDetail(items[0].optimization_id)
                                  setActiveTab('optimize')
                                }
                              }}
                              className="text-xs text-primary-400 hover:text-primary-300 transition-colors cursor-pointer whitespace-nowrap shrink-0 flex items-center gap-1"
                            >
                              View report
                              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                <path strokeLinecap="round" strokeLinejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" />
                              </svg>
                            </button>
                          )}
                        </div>
                        <div className="space-y-1.5">
                          {items.map(todo => {
                            const isExpanded = expandedTodos.has(todo.id)
                            return (
                            <div
                              key={todo.id}
                              className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-xl overflow-hidden group hover:border-dark-700 transition-colors"
                            >
                              <div className="flex items-center gap-3 px-4 py-2.5">
                                {!readOnly && (
                                  <button
                                    onClick={(e) => { e.stopPropagation(); updateTodoStatus(todo.id, todo.status === 'completed' ? 'todo' : 'completed') }}
                                    className={`w-4.5 h-4.5 rounded border-2 shrink-0 flex items-center justify-center transition-all cursor-pointer ${
                                      todo.status === 'completed'
                                        ? 'bg-emerald-500/20 border-emerald-500/50'
                                        : 'border-dark-600 hover:border-primary-500'
                                    }`}
                                  >
                                    {todo.status === 'completed' && (
                                      <svg className="w-2.5 h-2.5 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                                      </svg>
                                    )}
                                  </button>
                                )}
                                <button
                                  onClick={() => setExpandedTodos(prev => { const next = new Set(prev); next.has(todo.id) ? next.delete(todo.id) : next.add(todo.id); return next })}
                                  className="flex-1 min-w-0 text-left cursor-pointer"
                                >
                                  <p className={`text-sm leading-snug truncate ${
                                    todo.status === 'completed' ? 'text-dark-500 line-through' : 'text-white'
                                  }`}>
                                    {todo.summary || todo.action}
                                  </p>
                                </button>
                                <div className="flex items-center gap-1.5 shrink-0">
                                  <span className={`text-[9px] px-1 py-0.5 rounded font-semibold uppercase tracking-wider ${
                                    todo.priority === 'high' ? 'bg-red-500/20 text-red-400 border border-red-500/30' :
                                    todo.priority === 'medium' ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30' :
                                    'bg-dark-700/50 text-dark-400 border border-dark-600'
                                  }`}>{todo.priority}</span>
                                  <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                                    {!readOnly && todo.status !== 'backlogged' && todo.status !== 'archived' && (
                                      <button onClick={() => updateTodoStatus(todo.id, 'backlogged')} title="Backlog" className="p-1 text-dark-500 hover:text-amber-400 transition-colors cursor-pointer">
                                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                                      </button>
                                    )}
                                    {!readOnly && todo.status !== 'archived' && (
                                      <button onClick={() => updateTodoStatus(todo.id, 'archived')} title="Archive" className="p-1 text-dark-500 hover:text-dark-300 transition-colors cursor-pointer">
                                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" /></svg>
                                      </button>
                                    )}
                                    {!readOnly && (todo.status === 'backlogged' || todo.status === 'archived') && (
                                      <button onClick={() => updateTodoStatus(todo.id, 'todo')} title="Move to To-Do" className="p-1 text-dark-500 hover:text-primary-400 transition-colors cursor-pointer">
                                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M9 15L3 9m0 0l6-6M3 9h12a6 6 0 010 12h-3" /></svg>
                                      </button>
                                    )}
                                  </div>
                                  <svg className={`w-3.5 h-3.5 text-dark-600 transition-transform cursor-pointer ${isExpanded ? 'rotate-180' : ''}`} onClick={() => setExpandedTodos(prev => { const next = new Set(prev); next.has(todo.id) ? next.delete(todo.id) : next.add(todo.id); return next })} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" /></svg>
                                </div>
                              </div>
                              {isExpanded && (
                                <div className="px-4 pb-3 pt-0 border-t border-dark-800/50 space-y-2">
                                  <p className="text-dark-300 text-xs leading-relaxed mt-2">{todo.action}</p>
                                  <p className="text-dark-400 text-xs">{todo.expected_impact}</p>
                                  <div className="flex items-center gap-2 flex-wrap text-[10px]">
                                    <span className="text-dark-500 capitalize">{todo.dimension.replace(/_/g, ' ')}</span>
                                    {todo.created_at && (<><span className="text-dark-600">·</span><span className="text-dark-500">Created {fmtDate(todo.created_at)}</span></>)}
                                    {todo.backlogged_at && (<><span className="text-dark-600">·</span><span className="text-amber-400/70">Backlogged {fmtDate(todo.backlogged_at)}</span></>)}
                                    {todo.completed_at && (<><span className="text-dark-600">·</span><span className="text-dark-500">Done {fmtDate(todo.completed_at)}</span></>)}
                                    {todo.archived_at && (<><span className="text-dark-600">·</span><span className="text-dark-400/70">Archived {fmtDate(todo.archived_at)}</span></>)}
                                  </div>
                                </div>
                              )}
                            </div>
                          )})}

                        </div>
                      </div>
                    )
                  })}
                </div>
              )
            })()}
          </div>
        )}

        {/* ===== STATUS TAB ===== */}
        {activeTab === 'status' && (
          <div className="max-w-2xl mx-auto animate-fade-in space-y-6">
            {/* Current Status Card */}
            <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest">
                  Claude API Status
                </h3>
                <button
                  onClick={checkHealth}
                  disabled={healthChecking}
                  className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all disabled:opacity-50 cursor-pointer"
                >
                  {healthChecking ? 'Checking...' : 'Check Now'}
                </button>
              </div>

              {healthHistory.length === 0 ? (
                <div className="flex items-center justify-center py-8">
                  {healthChecking ? (
                    <div className="w-6 h-6 border-2 border-dark-700 border-t-primary-500 rounded-full animate-spin" />
                  ) : (
                    <p className="text-dark-500 text-sm">No checks performed yet</p>
                  )}
                </div>
              ) : (
                <div className="space-y-4">
                  {/* Per-model status rows */}
                  <div className="space-y-3">
                    {healthHistory[0].models.map((m) => (
                      <div key={m.model} className={`flex items-center gap-4 p-3 rounded-xl border ${
                        m.status === 'available'
                          ? 'bg-emerald-500/5 border-emerald-500/20'
                          : m.status === 'overloaded'
                          ? 'bg-amber-500/5 border-amber-500/20'
                          : 'bg-red-500/5 border-red-500/20'
                      }`}>
                        <div className={`w-10 h-10 rounded-lg flex items-center justify-center shrink-0 ${
                          m.status === 'available'
                            ? 'bg-emerald-500/20'
                            : m.status === 'overloaded'
                            ? 'bg-amber-500/20'
                            : 'bg-red-500/20'
                        }`}>
                          {m.status === 'available' ? (
                            <svg className="w-5 h-5 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                            </svg>
                          ) : m.status === 'overloaded' ? (
                            <svg className="w-5 h-5 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
                            </svg>
                          ) : (
                            <svg className="w-5 h-5 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                          )}
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2">
                            <span className="text-white font-medium text-sm">{m.name}</span>
                            {m.model === 'claude-sonnet-4-6' && (
                              <span className="text-[10px] px-1.5 py-0.5 rounded bg-primary-500/20 text-primary-400 border border-primary-500/30 font-semibold uppercase tracking-wider">
                                Primary
                              </span>
                            )}
                            <span className={`text-xs font-medium ${
                              m.status === 'available' ? 'text-emerald-400' :
                              m.status === 'overloaded' ? 'text-amber-400' :
                              'text-red-400'
                            }`}>
                              {m.status === 'available' ? 'Available' :
                               m.status === 'overloaded' ? 'Overloaded' :
                               'Error'}
                            </span>
                          </div>
                          <p className="text-dark-500 text-xs truncate">
                            {m.model}
                            {m.latency_ms !== undefined && ` · ${m.latency_ms}ms`}
                            {m.http_status !== undefined && m.http_status !== 200 && ` · HTTP ${m.http_status}`}
                          </p>
                          {m.error && m.status !== 'available' && (
                            <p className="text-dark-500 text-xs mt-1 break-all line-clamp-3">
                              {(() => {
                                try {
                                  const parsed = JSON.parse(m.error)
                                  if (parsed.error?.message) return parsed.error.message
                                  if (parsed.message) return parsed.message
                                  return m.error
                                } catch {
                                  return m.error
                                }
                              })()}
                            </p>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>

                  {/* Fallback indicator */}
                  {healthHistory[0].models[0]?.status !== 'available' &&
                   healthHistory[0].models.some(m => m.status === 'available') && (
                    <div className="bg-amber-500/10 border border-amber-500/20 rounded-lg p-3">
                      <p className="text-amber-300 text-sm">
                        Currently using fallback: {healthHistory[0].models.find(m => m.status === 'available')?.name}
                      </p>
                    </div>
                  )}

                  <p className="text-dark-500 text-xs">
                    Last checked: {new Date(healthHistory[0].checked_at).toLocaleTimeString(undefined, {
                      hour: '2-digit', minute: '2-digit', second: '2-digit'
                    })}
                    {' · '}Auto-checks every 5 minutes
                  </p>
                </div>
              )}
            </div>

            {/* Availability Timeline */}
            <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest">
                  Availability Timeline (24h)
                </h3>
                {healthTimelineLoading && (
                  <div className="w-4 h-4 border-2 border-dark-700 border-t-primary-500 rounded-full animate-spin" />
                )}
              </div>
              {healthTimeline.length === 0 ? (
                <p className="text-dark-500 text-sm text-center py-6">No historical data yet. Checks are recorded every 5 minutes.</p>
              ) : (() => {
                // Extract unique model names from first record
                const modelNames = healthTimeline[0]?.models.map(m => ({ model: m.model, name: m.name })) || []
                return (
                  <div className="space-y-4">
                    {modelNames.map(({ model, name }) => {
                      const statuses = healthTimeline.map(r => {
                        const m = r.models.find(mm => mm.model === model)
                        return { status: m?.status || 'error', time: r.checked_at, latency: m?.latency_ms }
                      })
                      const available = statuses.filter(s => s.status === 'available').length
                      const uptime = statuses.length > 0 ? Math.round((available / statuses.length) * 100) : 0
                      return (
                        <div key={model}>
                          <div className="flex items-center justify-between mb-2">
                            <div className="flex items-center gap-2">
                              <span className="text-white text-sm font-medium">{name}</span>
                              {model === 'claude-sonnet-4-6' && (
                                <span className="text-[10px] px-1.5 py-0.5 rounded bg-primary-500/20 text-primary-400 border border-primary-500/30 font-semibold uppercase tracking-wider">Primary</span>
                              )}
                            </div>
                            <span className={`text-xs font-medium ${uptime >= 95 ? 'text-emerald-400' : uptime >= 80 ? 'text-amber-400' : 'text-red-400'}`}>
                              {uptime}% uptime
                            </span>
                          </div>
                          {/* Visual bar of status dots */}
                          <div className="flex gap-px items-center" title={`${statuses.length} checks`}>
                            {statuses.length <= 120 ? (
                              statuses.map((s, i) => (
                                <div
                                  key={i}
                                  className={`flex-1 h-6 rounded-sm min-w-[2px] ${
                                    s.status === 'available' ? 'bg-emerald-500/60' :
                                    s.status === 'overloaded' ? 'bg-amber-500/60' :
                                    'bg-red-500/60'
                                  }`}
                                  title={`${new Date(s.time).toLocaleTimeString()} — ${s.status}${s.latency ? ` (${s.latency}ms)` : ''}`}
                                />
                              ))
                            ) : (
                              // Downsample to 120 bins for display
                              (() => {
                                const binSize = Math.ceil(statuses.length / 120)
                                const bins: { status: string; time: string }[] = []
                                for (let i = 0; i < statuses.length; i += binSize) {
                                  const chunk = statuses.slice(i, i + binSize)
                                  const worst = chunk.find(c => c.status === 'error') || chunk.find(c => c.status === 'overloaded') || chunk[0]
                                  bins.push({ status: worst.status, time: chunk[0].time })
                                }
                                return bins.map((s, i) => (
                                  <div
                                    key={i}
                                    className={`flex-1 h-6 rounded-sm min-w-[2px] ${
                                      s.status === 'available' ? 'bg-emerald-500/60' :
                                      s.status === 'overloaded' ? 'bg-amber-500/60' :
                                      'bg-red-500/60'
                                    }`}
                                    title={`${new Date(s.time).toLocaleTimeString()} — ${s.status}`}
                                  />
                                ))
                              })()
                            )}
                          </div>
                          <div className="flex justify-between mt-1">
                            <span className="text-dark-600 text-[10px]">
                              {new Date(statuses[0]?.time).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}
                            </span>
                            <span className="text-dark-600 text-[10px]">
                              {new Date(statuses[statuses.length - 1]?.time).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}
                            </span>
                          </div>
                        </div>
                      )
                    })}
                    {/* Legend */}
                    <div className="flex items-center gap-4 pt-2">
                      <div className="flex items-center gap-1.5"><div className="w-3 h-3 rounded-sm bg-emerald-500/60" /><span className="text-dark-500 text-[10px]">Available</span></div>
                      <div className="flex items-center gap-1.5"><div className="w-3 h-3 rounded-sm bg-amber-500/60" /><span className="text-dark-500 text-[10px]">Overloaded</span></div>
                      <div className="flex items-center gap-1.5"><div className="w-3 h-3 rounded-sm bg-red-500/60" /><span className="text-dark-500 text-[10px]">Error</span></div>
                      <span className="text-dark-600 text-[10px] ml-auto">{healthTimeline.length} checks in last 24h</span>
                    </div>
                  </div>
                )
              })()}
            </div>
          </div>
        )}

        {/* ===== BRAND TAB ===== */}
        {activeTab === 'brand' && (
          <div className="max-w-3xl mx-auto animate-fade-in">
            {!brandEditing ? (
              /* Loading / auto-loading brand for selectedDomain */
              <div className="flex justify-center py-16">
                <div className="w-6 h-6 border-2 border-dark-700 border-t-primary-500 rounded-full animate-spin" />
              </div>
            ) : (
              /* Brand Edit View */
              <div>
                <div className="flex items-center justify-between mb-6">
                  {!readOnly ? (
                    <button
                      onClick={() => guardBrandNav(() => { setBrandEditing(false); fetchBrandList() })}
                      className="text-dark-400 hover:text-white text-sm flex items-center gap-1 cursor-pointer"
                    >
                      <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" />
                      </svg>
                      Back
                    </button>
                  ) : (
                    <h3 className="text-sm font-semibold text-dark-400">Brand Intelligence</h3>
                  )}
                  {brandEditing && !readOnly && (
                    <div className="flex items-center gap-3">
                      {/* Completeness */}
                      <div className="flex items-center gap-2 text-xs">
                        <div className="w-24 h-1.5 bg-dark-800 rounded-full overflow-hidden">
                          <div
                            className={`h-full rounded-full transition-all ${
                              brandCompleteness >= 80 ? 'bg-emerald-500' :
                              brandCompleteness >= 50 ? 'bg-amber-500' : 'bg-orange-500'
                            }`}
                            style={{ width: `${brandCompleteness}%` }}
                          />
                        </div>
                        <span className="text-dark-500">{brandCompleteness}%</span>
                      </div>
                      {saasEnabled && user && (user.role === 'owner' || user.role === 'admin') && selectedDomain && (
                        <button
                          onClick={() => { setDomainShareState(null); setShareModalDomain(selectedDomain); fetchDomainShare(selectedDomain) }}
                          className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all cursor-pointer flex items-center gap-1.5"
                          title="Share this domain"
                        >
                          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M7.217 10.907a2.25 2.25 0 1 0 0 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186 9.566-5.314m-9.566 7.5 9.566 5.314m0-12.814a2.25 2.25 0 1 0 0-2.186m0 2.186a2.25 2.25 0 1 0 0 2.186" /></svg>
                          Share
                        </button>
                      )}
                      <button
                        onClick={saveBrandProfile}
                        disabled={brandSaving || !brandDomain.trim()}
                        className="px-4 py-1.5 bg-gradient-to-r from-primary-600 to-primary-500 text-white text-sm font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all disabled:opacity-50 cursor-pointer"
                      >
                        {brandSaving ? 'Saving...' : 'Save'}
                      </button>
                    </div>
                  )}
                </div>


                <div className={readOnly ? 'pointer-events-none' : ''}>
                {/* Section 1: Core Identity (always open) */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6 mb-4">
                  <h4 className="text-sm font-semibold text-white mb-4">Core Identity</h4>
                  <div className="space-y-4">
                    <div>
                      <label className="block text-xs text-dark-400 mb-1">Brand Name</label>
                      <input
                        type="text"
                        value={brandForm.brand_name}
                        onChange={e => setBrandForm(p => ({ ...p, brand_name: e.target.value }))}
                        placeholder="Your Company Name"
                        className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                      />
                    </div>
                    <div>
                      <div className="flex items-center justify-between mb-1">
                        <label className="text-xs text-dark-400">Description</label>
                        <button
                          onClick={generateDescription}
                          disabled={generatingDesc || !brandDomain.trim()}
                          className="text-xs text-primary-400 hover:text-primary-300 cursor-pointer disabled:opacity-50"
                        >
                          {generatingDesc ? 'Generating...' : 'Generate from site'}
                        </button>
                      </div>
                      {generatingDesc && generateMessages.length > 0 && (
                        <div className="mb-2 space-y-1">
                          {generateMessages.slice(-3).map((msg, i) => (
                            <div key={i} className="flex items-center gap-2 text-xs text-dark-500">
                              <div className="w-1.5 h-1.5 rounded-full bg-primary-400 animate-pulse" />
                              {msg}
                            </div>
                          ))}
                        </div>
                      )}
                      <textarea
                        value={brandForm.description}
                        onChange={e => setBrandForm(p => ({ ...p, description: e.target.value }))}
                        placeholder="2-3 sentences about what this company does..."
                        rows={3}
                        className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm resize-none"
                      />
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label className="block text-xs text-dark-400 mb-1">Categories <span className="text-dark-600">(comma-separated)</span></label>
                        <input
                          type="text"
                          value={brandForm.categories}
                          onChange={e => setBrandForm(p => ({ ...p, categories: e.target.value }))}
                          placeholder="SaaS, project management"
                          className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                        />
                      </div>
                      <div>
                        <label className="block text-xs text-dark-400 mb-1">Products / Features</label>
                        <input
                          type="text"
                          value={brandForm.products}
                          onChange={e => setBrandForm(p => ({ ...p, products: e.target.value }))}
                          placeholder="Gantt charts, API integrations"
                          className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                        />
                      </div>
                    </div>
                  </div>
                </div>

                {/* Section 2: Target Audience (collapsible) */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl mb-4 overflow-hidden">
                  <button
                    onClick={() => setBrandSections(p => ({ ...p, audience: !p.audience }))}
                    className="w-full flex items-center justify-between p-5 cursor-pointer"
                  >
                    <div className="flex items-center gap-2">
                      <h4 className="text-sm font-semibold text-white">Target Audience</h4>
                      {(brandForm.primary_audience && brandForm.key_use_cases
                          ? <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-medium">Done</span>
                          : !brandSections.audience ? <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20 font-medium">Needs info</span> : null
                      )}
                    </div>
                    <svg className={`w-4 h-4 text-dark-500 transition-transform ${brandSections.audience ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
                    </svg>
                  </button>
                  {brandSections.audience && (
                    <div className="px-6 pb-6 space-y-4">
                      <div className="flex items-center justify-end">
                        <button
                          onClick={predictAudience}
                          disabled={predictingAudience || !brandDomain.trim()}
                          className="text-xs text-primary-400 hover:text-primary-300 cursor-pointer disabled:opacity-50"
                        >
                          {predictingAudience ? 'Predicting...' : 'Predict based on Website'}
                        </button>
                      </div>
                      {predictingAudience && predictAudienceMessages.length > 0 && (
                        <div className="space-y-1">
                          {predictAudienceMessages.slice(-3).map((msg, i) => (
                            <div key={i} className="flex items-center gap-2 text-xs text-dark-500">
                              {i === predictAudienceMessages.slice(-3).length - 1
                                ? <div className="w-1.5 h-1.5 rounded-full bg-primary-400 animate-pulse shrink-0" />
                                : <svg className="w-3 h-3 text-accent-emerald shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                              }
                              {msg}
                            </div>
                          ))}
                        </div>
                      )}
                      <div>
                        <label className="block text-xs text-dark-400 mb-1">Primary Audience</label>
                        <textarea
                          value={brandForm.primary_audience}
                          onChange={e => setBrandForm(p => ({ ...p, primary_audience: e.target.value }))}
                          placeholder="Small business owners, engineering managers..."
                          rows={2}
                          className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm resize-none"
                        />
                      </div>
                      <div>
                        <label className="block text-xs text-dark-400 mb-1">Key Use Cases <span className="text-dark-600">(comma-separated)</span></label>
                        <input
                          type="text"
                          value={brandForm.key_use_cases}
                          onChange={e => setBrandForm(p => ({ ...p, key_use_cases: e.target.value }))}
                          placeholder="managing remote teams, sprint planning"
                          className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                        />
                      </div>
                    </div>
                  )}
                </div>

                {/* Section 3: Competitive Landscape (collapsible) */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl mb-4 overflow-hidden">
                  <button
                    onClick={() => setBrandSections(p => ({ ...p, competitors: !p.competitors }))}
                    className="w-full flex items-center justify-between p-5 cursor-pointer"
                  >
                    <div className="flex items-center gap-2">
                      <h4 className="text-sm font-semibold text-white">
                        Competitive Landscape
                        {brandCompetitors.length > 0 && <span className="text-dark-500 font-normal ml-2">({brandCompetitors.length})</span>}
                      </h4>
                      {(brandCompetitors.length >= 3
                          ? <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-medium">Done</span>
                          : !brandSections.competitors ? (brandCompetitors.length > 0
                            ? <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20 font-medium">Add more ({brandCompetitors.length}/3)</span>
                            : <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20 font-medium">Needs info</span>) : null
                      )}
                    </div>
                    <svg className={`w-4 h-4 text-dark-500 transition-transform ${brandSections.competitors ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
                    </svg>
                  </button>
                  {brandSections.competitors && (
                    <div className="px-6 pb-6">
                      {/* Existing competitors list */}
                      {brandCompetitors.length > 0 && (
                        <div className="space-y-2 mb-4">
                          {brandCompetitors.map((comp, i) => (
                            <div key={i} className="flex items-center gap-2 bg-dark-800/50 border border-dark-700 rounded-lg p-3">
                              <div className="flex-1 grid grid-cols-4 gap-2">
                                <input
                                  type="text"
                                  value={comp.name}
                                  onChange={e => {
                                    const updated = [...brandCompetitors]
                                    updated[i] = { ...comp, name: e.target.value }
                                    setBrandCompetitors(updated)
                                  }}
                                  placeholder="Name"
                                  className="px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-xs"
                                />
                                <input
                                  type="text"
                                  value={comp.url}
                                  onChange={e => {
                                    const updated = [...brandCompetitors]
                                    updated[i] = { ...comp, url: e.target.value }
                                    setBrandCompetitors(updated)
                                  }}
                                  placeholder="URL"
                                  className="px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-xs"
                                />
                                <select
                                  value={comp.relationship}
                                  onChange={e => {
                                    const updated = [...brandCompetitors]
                                    updated[i] = { ...comp, relationship: e.target.value }
                                    setBrandCompetitors(updated)
                                  }}
                                  className="px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white text-xs cursor-pointer"
                                >
                                  <option value="direct">Direct</option>
                                  <option value="indirect">Indirect</option>
                                  <option value="aspirational">Aspirational</option>
                                  <option value="adjacent">Adjacent</option>
                                </select>
                                <input
                                  type="text"
                                  value={comp.notes}
                                  onChange={e => {
                                    const updated = [...brandCompetitors]
                                    updated[i] = { ...comp, notes: e.target.value }
                                    setBrandCompetitors(updated)
                                  }}
                                  placeholder="Notes"
                                  className="px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-xs"
                                />
                              </div>
                              <button
                                onClick={() => setBrandCompetitors(prev => prev.filter((_, j) => j !== i))}
                                className="text-dark-600 hover:text-red-400 cursor-pointer p-1"
                              >
                                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                                </svg>
                              </button>
                            </div>
                          ))}
                        </div>
                      )}
                      <div className="flex gap-2">
                        <button
                          onClick={() => setBrandCompetitors(prev => [...prev, { name: '', url: '', relationship: 'direct', notes: '' }])}
                          className="text-xs text-dark-400 hover:text-white border border-dark-700 rounded-lg px-3 py-1.5 cursor-pointer"
                        >
                          + Add Manually
                        </button>
                        <button
                          onClick={discoverCompetitors}
                          disabled={discovering || !brandDomain.trim()}
                          className="text-xs text-primary-400 hover:text-primary-300 border border-primary-500/30 rounded-lg px-3 py-1.5 cursor-pointer disabled:opacity-50"
                        >
                          {discovering ? 'Discovering...' : 'Discover Competitors'}
                        </button>
                      </div>

                      {/* Discovery progress */}
                      {discovering && discoverMessages.length > 0 && (
                        <div className="mt-4 space-y-1">
                          {discoverMessages.slice(-5).map((msg, i) => (
                            <div key={i} className="flex items-center gap-2 text-xs text-dark-500">
                              {i === discoverMessages.slice(-5).length - 1
                                ? <div className="w-1.5 h-1.5 rounded-full bg-primary-400 animate-pulse" />
                                : <svg className="w-3 h-3 text-accent-emerald" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                              }
                              {msg}
                            </div>
                          ))}
                        </div>
                      )}

                      {/* Discovery results */}
                      {discoveredCompetitors.length > 0 && (
                        <div className="mt-4">
                          <div className="flex items-center justify-between mb-2">
                            <span className="text-xs font-medium text-dark-400">Discovered ({discoveredCompetitors.length})</span>
                            <div className="flex gap-2">
                              <button
                                onClick={() => setDiscoverSelected(new Set(discoveredCompetitors.map((_, i) => i)))}
                                className="text-[10px] text-dark-500 hover:text-white cursor-pointer"
                              >
                                Select All
                              </button>
                              <button
                                onClick={addSelectedCompetitors}
                                disabled={discoverSelected.size === 0}
                                className="text-xs text-primary-400 hover:text-primary-300 cursor-pointer disabled:opacity-50"
                              >
                                Add Selected ({discoverSelected.size})
                              </button>
                            </div>
                          </div>
                          <div className="space-y-1">
                            {discoveredCompetitors.map((dc, i) => (
                              <label key={i} className="flex items-center gap-3 bg-dark-800/30 border border-dark-700/50 rounded-lg p-2.5 cursor-pointer hover:border-dark-600">
                                <input
                                  type="checkbox"
                                  checked={discoverSelected.has(i)}
                                  onChange={() => {
                                    setDiscoverSelected(prev => {
                                      const next = new Set(prev)
                                      if (next.has(i)) next.delete(i)
                                      else next.add(i)
                                      return next
                                    })
                                  }}
                                  className="accent-primary-500"
                                />
                                <div className="flex-1 min-w-0">
                                  <div className="flex items-center gap-2">
                                    <span className="text-white text-xs font-medium">{dc.name}</span>
                                    <span className={`text-[10px] px-1.5 py-0.5 rounded ${
                                      dc.relationship === 'direct' ? 'bg-primary-500/20 text-primary-300' :
                                      dc.relationship === 'indirect' ? 'bg-amber-500/20 text-amber-300' :
                                      'bg-dark-700 text-dark-400'
                                    }`}>{dc.relationship}</span>
                                    <span className="text-[10px] text-dark-600">{dc.source}</span>
                                  </div>
                                  {dc.url && <span className="text-dark-500 text-[10px] truncate block">{dc.url}</span>}
                                </div>
                              </label>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                </div>

                {/* Section 4: Key Queries & Brand Voice (collapsible) */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl mb-4 overflow-hidden">
                  <button
                    onClick={() => setBrandSections(p => ({ ...p, queries: !p.queries }))}
                    className="w-full flex items-center justify-between p-5 cursor-pointer"
                  >
                    <div className="flex items-center gap-2">
                      <h4 className="text-sm font-semibold text-white">
                        Key Queries & Brand Voice
                        {(brandQueries.length > 0 || brandMessages.length > 0) && (
                          <span className="text-dark-500 font-normal ml-2">({brandQueries.length} queries, {brandMessages.length} claims)</span>
                        )}
                      </h4>
                      {(() => {
                        const hasQueries = brandQueries.length >= 5
                        const hasMessages = brandMessages.length > 0
                        const hasDiffs = !!brandForm.differentiators
                        const allDone = hasQueries && hasMessages && hasDiffs
                        const someDone = brandQueries.length > 0 || hasMessages || hasDiffs
                        return allDone
                          ? <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-medium">Done</span>
                          : !brandSections.queries ? (someDone
                            ? <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20 font-medium">Incomplete</span>
                            : <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20 font-medium">Needs info</span>) : null
                      })()}
                    </div>
                    <svg className={`w-4 h-4 text-dark-500 transition-transform ${brandSections.queries ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
                    </svg>
                  </button>
                  {brandSections.queries && (
                    <div className="px-6 pb-6 space-y-6">
                      {/* Target Queries */}
                      <div>
                        <label className="block text-xs font-medium text-dark-400 mb-1">Target Queries</label>
                        <p className="text-[11px] text-dark-500 mb-2">The most important questions you aspire to answer for users — whether or not they're answered on your site already.</p>
                        {brandQueries.length > 0 && (
                          <div className="space-y-1.5 mb-3">
                            {brandQueries.map((q, i) => (
                              <div key={i} className="flex items-center gap-2 bg-dark-800/50 border border-dark-700 rounded-lg p-2.5">
                                <div className="flex-1 grid grid-cols-6 gap-2">
                                  <input
                                    type="text"
                                    value={q.query}
                                    onChange={e => {
                                      const updated = [...brandQueries]
                                      updated[i] = { ...q, query: e.target.value }
                                      setBrandQueries(updated)
                                    }}
                                    placeholder="Query text"
                                    className="col-span-3 px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-xs"
                                  />
                                  <select
                                    value={q.priority}
                                    onChange={e => {
                                      const updated = [...brandQueries]
                                      updated[i] = { ...q, priority: e.target.value }
                                      setBrandQueries(updated)
                                    }}
                                    className="px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white text-xs cursor-pointer"
                                  >
                                    <option value="high">High</option>
                                    <option value="medium">Medium</option>
                                    <option value="low">Low</option>
                                  </select>
                                  <select
                                    value={q.type}
                                    onChange={e => {
                                      const updated = [...brandQueries]
                                      updated[i] = { ...q, type: e.target.value }
                                      setBrandQueries(updated)
                                    }}
                                    className="col-span-2 px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white text-xs cursor-pointer"
                                  >
                                    <option value="brand">Brand</option>
                                    <option value="category">Category</option>
                                    <option value="comparison">Comparison</option>
                                    <option value="problem">Problem</option>
                                  </select>
                                </div>
                                <button
                                  onClick={() => setBrandQueries(prev => prev.filter((_, j) => j !== i))}
                                  className="text-dark-600 hover:text-red-400 cursor-pointer p-1"
                                >
                                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                    <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                                  </svg>
                                </button>
                              </div>
                            ))}
                          </div>
                        )}
                        <div className="flex gap-2">
                          <button
                            onClick={() => setBrandQueries(prev => [...prev, { query: '', priority: 'medium', type: 'category' }])}
                            className="text-xs text-dark-400 hover:text-white border border-dark-700 rounded-lg px-3 py-1.5 cursor-pointer"
                          >
                            + Add Query
                          </button>
                          <button
                            onClick={suggestQueries}
                            disabled={suggestingQueries || !brandDomain.trim()}
                            className="text-xs text-primary-400 hover:text-primary-300 border border-primary-500/30 rounded-lg px-3 py-1.5 cursor-pointer disabled:opacity-50"
                          >
                            {suggestingQueries ? 'Suggesting...' : 'Suggest Queries'}
                          </button>
                        </div>

                        {/* Suggest progress */}
                        {suggestMessages.length > 0 && (
                          <div className="mt-3 space-y-1">
                            {suggestMessages.slice(-3).map((msg, i) => (
                              <div key={i} className="flex items-center gap-2 text-xs text-dark-500">
                                {suggestingQueries
                                  ? <div className="w-1.5 h-1.5 rounded-full bg-primary-400 animate-pulse" />
                                  : <div className="w-1.5 h-1.5 rounded-full bg-dark-600" />
                                }
                                <span className={msg.startsWith('Error:') ? 'text-red-400' : ''}>{msg}</span>
                              </div>
                            ))}
                          </div>
                        )}

                        {/* Suggested queries */}
                        {suggestedQueries.length > 0 && (
                          <div className="mt-4">
                            <div className="flex items-center justify-between mb-2">
                              <span className="text-xs font-medium text-dark-400">Suggested ({suggestedQueries.length})</span>
                              <div className="flex gap-2">
                                <button
                                  onClick={() => setSuggestSelected(new Set(suggestedQueries.map((_, i) => i)))}
                                  className="text-[10px] text-dark-500 hover:text-white cursor-pointer"
                                >
                                  Select All
                                </button>
                                <button
                                  onClick={addSelectedQueries}
                                  disabled={suggestSelected.size === 0}
                                  className="text-xs text-primary-400 hover:text-primary-300 cursor-pointer disabled:opacity-50"
                                >
                                  Add Selected ({suggestSelected.size})
                                </button>
                              </div>
                            </div>
                            <div className="space-y-1 max-h-64 overflow-y-auto">
                              {suggestedQueries.map((sq, i) => (
                                <label key={i} className="flex items-center gap-3 bg-dark-800/30 border border-dark-700/50 rounded-lg p-2 cursor-pointer hover:border-dark-600">
                                  <input
                                    type="checkbox"
                                    checked={suggestSelected.has(i)}
                                    onChange={() => {
                                      setSuggestSelected(prev => {
                                        const next = new Set(prev)
                                        if (next.has(i)) next.delete(i)
                                        else next.add(i)
                                        return next
                                      })
                                    }}
                                    className="accent-primary-500"
                                  />
                                  <span className="flex-1 text-white text-xs">{sq.query}</span>
                                  <span className={`text-[10px] px-1.5 py-0.5 rounded ${
                                    sq.priority === 'high' ? 'bg-red-500/20 text-red-300' :
                                    sq.priority === 'medium' ? 'bg-amber-500/20 text-amber-300' :
                                    'bg-dark-700 text-dark-400'
                                  }`}>{sq.priority}</span>
                                  <span className="text-[10px] text-dark-500">{sq.type}</span>
                                </label>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>

                      {/* Key Messages */}
                      <div>
                        <label className="block text-xs font-medium text-dark-400 mb-1">Key Brand Claims</label>
                        <p className="text-[11px] text-dark-500 mb-2">Discovers claims from your site content. Add any missing claims manually for a complete picture of your brand aspirations.</p>
                        {brandMessages.length > 0 && (
                          <div className="space-y-1.5 mb-3">
                            {brandMessages.map((m, i) => (
                              <div key={i} className="flex items-center gap-2 bg-dark-800/50 border border-dark-700 rounded-lg p-2.5">
                                <div className="flex-1 grid grid-cols-6 gap-2">
                                  <input
                                    type="text"
                                    value={m.claim}
                                    onChange={e => {
                                      const updated = [...brandMessages]
                                      updated[i] = { ...m, claim: e.target.value }
                                      setBrandMessages(updated)
                                    }}
                                    placeholder="Brand claim"
                                    className="col-span-3 px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-xs"
                                  />
                                  <input
                                    type="text"
                                    value={m.evidence_url}
                                    onChange={e => {
                                      const updated = [...brandMessages]
                                      updated[i] = { ...m, evidence_url: e.target.value }
                                      setBrandMessages(updated)
                                    }}
                                    placeholder="Evidence URL"
                                    className="col-span-2 px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-xs"
                                  />
                                  <select
                                    value={m.priority}
                                    onChange={e => {
                                      const updated = [...brandMessages]
                                      updated[i] = { ...m, priority: e.target.value }
                                      setBrandMessages(updated)
                                    }}
                                    className="px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white text-xs cursor-pointer"
                                  >
                                    <option value="high">High</option>
                                    <option value="medium">Medium</option>
                                    <option value="low">Low</option>
                                  </select>
                                </div>
                                <button
                                  onClick={() => setBrandMessages(prev => prev.filter((_, j) => j !== i))}
                                  className="text-dark-600 hover:text-red-400 cursor-pointer p-1"
                                >
                                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                    <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                                  </svg>
                                </button>
                              </div>
                            ))}
                          </div>
                        )}
                        <div className="flex gap-2">
                          <button
                            onClick={() => setBrandMessages(prev => [...prev, { claim: '', evidence_url: '', priority: 'medium' }])}
                            className="text-xs text-dark-400 hover:text-white border border-dark-700 rounded-lg px-3 py-1.5 cursor-pointer"
                          >
                            + Add Claim
                          </button>
                          <button
                            onClick={suggestClaims}
                            disabled={suggestingClaims || !brandDomain.trim()}
                            className="text-xs text-primary-400 hover:text-primary-300 border border-primary-500/30 rounded-lg px-3 py-1.5 cursor-pointer disabled:opacity-50"
                          >
                            {suggestingClaims ? 'Discovering...' : 'Suggest Claims'}
                          </button>
                        </div>

                        {/* Suggest Claims progress */}
                        {suggestClaimMessages.length > 0 && (
                          <div className="mt-3 space-y-1">
                            {suggestClaimMessages.slice(-5).map((msg, i) => (
                              <div key={i} className="flex items-center gap-2 text-xs text-dark-500">
                                {suggestingClaims && i === suggestClaimMessages.slice(-5).length - 1
                                  ? <div className="w-1.5 h-1.5 rounded-full bg-primary-400 animate-pulse shrink-0" />
                                  : msg.startsWith('Error:')
                                    ? <div className="w-1.5 h-1.5 rounded-full bg-red-400 shrink-0" />
                                    : <svg className="w-3 h-3 text-accent-emerald shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                                }
                                <span className={msg.startsWith('Error:') ? 'text-red-400' : ''}>{msg}</span>
                              </div>
                            ))}
                          </div>
                        )}

                        {/* Suggested Claims review */}
                        {suggestedClaims.length > 0 && (
                          <div className="mt-3 border border-primary-500/20 rounded-xl p-4 bg-primary-500/5">
                            <div className="flex items-center justify-between mb-3">
                              <span className="text-xs font-semibold text-white">Discovered ({suggestedClaims.length})</span>
                              <div className="flex gap-2">
                                <button onClick={() => setSuggestClaimSelected(new Set(suggestedClaims.map((_, i) => i)))} className="text-[10px] text-dark-400 hover:text-white cursor-pointer">Select All</button>
                                <button
                                  onClick={addSelectedClaims}
                                  disabled={suggestClaimSelected.size === 0}
                                  className="text-xs px-2.5 py-1 bg-primary-600 text-white rounded-lg hover:bg-primary-500 cursor-pointer disabled:opacity-50"
                                >
                                  Add Selected ({suggestClaimSelected.size})
                                </button>
                              </div>
                            </div>
                            <div className="space-y-1.5 max-h-64 overflow-y-auto">
                              {suggestedClaims.map((sc, i) => (
                                <label key={i} className="flex items-start gap-3 p-2 rounded-lg hover:bg-dark-800/50 cursor-pointer">
                                  <input
                                    type="checkbox"
                                    checked={suggestClaimSelected.has(i)}
                                    onChange={() => {
                                      setSuggestClaimSelected(prev => {
                                        const next = new Set(prev)
                                        if (next.has(i)) next.delete(i); else next.add(i)
                                        return next
                                      })
                                    }}
                                    className="accent-primary-500 mt-0.5"
                                  />
                                  <div className="flex-1 min-w-0">
                                    <span className="text-white text-xs">{sc.claim}</span>
                                    <div className="flex items-center gap-2 mt-0.5">
                                      <span className={`text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase ${
                                        sc.priority === 'high' ? 'bg-red-500/20 text-red-400' : sc.priority === 'medium' ? 'bg-amber-500/20 text-amber-400' : 'bg-dark-700/50 text-dark-400'
                                      }`}>{sc.priority}</span>
                                      {sc.evidence_url && <span className="text-dark-500 text-[10px] truncate">{sc.evidence_url}</span>}
                                    </div>
                                  </div>
                                </label>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>

                      {/* Differentiators */}
                      <div>
                        <label className="block text-xs text-dark-400 mb-1">Differentiators <span className="text-dark-600">(comma-separated)</span></label>
                        <input
                          type="text"
                          value={brandForm.differentiators}
                          onChange={e => setBrandForm(p => ({ ...p, differentiators: e.target.value }))}
                          placeholder="AI-powered, no per-seat pricing, open API"
                          className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                        />
                        <div className="flex gap-2 mt-2">
                          <button
                            onClick={predictDifferentiators}
                            disabled={predictingDiffs || !brandDomain.trim()}
                            className="text-xs text-primary-400 hover:text-primary-300 border border-primary-500/30 rounded-lg px-3 py-1.5 cursor-pointer disabled:opacity-50"
                          >
                            {predictingDiffs ? 'Discovering...' : 'Predict Differentiators'}
                          </button>
                        </div>
                        <p className="text-[10px] text-dark-600 mt-1">Customize these results — the differentiators we discover may not be the ones you consider most important.</p>

                        {/* Predict Differentiators progress */}
                        {predictDiffMessages.length > 0 && (
                          <div className="mt-3 space-y-1">
                            {predictDiffMessages.slice(-5).map((msg, i) => (
                              <div key={i} className="flex items-center gap-2 text-xs text-dark-500">
                                {predictingDiffs && i === predictDiffMessages.slice(-5).length - 1
                                  ? <div className="w-1.5 h-1.5 rounded-full bg-primary-400 animate-pulse shrink-0" />
                                  : msg.startsWith('Error:')
                                    ? <div className="w-1.5 h-1.5 rounded-full bg-red-400 shrink-0" />
                                    : <svg className="w-3 h-3 text-accent-emerald shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                                }
                                <span className={msg.startsWith('Error:') ? 'text-red-400' : ''}>{msg}</span>
                              </div>
                            ))}
                          </div>
                        )}

                        {/* Suggested Differentiators review */}
                        {suggestedDiffs.length > 0 && (
                          <div className="mt-4">
                            <div className="flex items-center justify-between mb-2">
                              <span className="text-xs font-medium text-dark-400">Suggested ({suggestedDiffs.length})</span>
                              <div className="flex gap-2">
                                <button onClick={() => setSuggestDiffSelected(new Set(suggestedDiffs.map((_, i) => i)))} className="text-[10px] text-dark-500 hover:text-white cursor-pointer">Select All</button>
                                <button
                                  onClick={addSelectedDifferentiators}
                                  disabled={suggestDiffSelected.size === 0}
                                  className="text-xs text-primary-400 hover:text-primary-300 cursor-pointer disabled:opacity-50"
                                >
                                  Add Selected ({suggestDiffSelected.size})
                                </button>
                              </div>
                            </div>
                            <div className="space-y-1 max-h-64 overflow-y-auto">
                              {suggestedDiffs.map((sd, i) => (
                                <label key={i} className="flex items-center gap-3 bg-dark-800/30 border border-dark-700/50 rounded-lg p-2 cursor-pointer hover:border-dark-600">
                                  <input
                                    type="checkbox"
                                    checked={suggestDiffSelected.has(i)}
                                    onChange={() => {
                                      setSuggestDiffSelected(prev => {
                                        const next = new Set(prev)
                                        if (next.has(i)) next.delete(i); else next.add(i)
                                        return next
                                      })
                                    }}
                                    className="accent-primary-500"
                                  />
                                  <span className="flex-1 text-white text-xs">{sd}</span>
                                </label>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>
                    </div>
                  )}
                </div>

                {/* Section 5: Existing Presence (collapsible) */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl mb-4 overflow-hidden">
                  <button
                    onClick={() => setBrandSections(p => ({ ...p, presence: !p.presence }))}
                    className="w-full flex items-center justify-between p-5 cursor-pointer"
                  >
                    <div className="flex items-center gap-2">
                      <h4 className="text-sm font-semibold text-white">Existing Presence</h4>
                      {(brandPresenceComplete || brandPresenceForm.youtube_url || brandPresenceForm.subreddits || brandPresenceForm.review_site_urls
                          ? <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-medium">Done</span>
                          : !brandSections.presence ? <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20 font-medium">Needs review</span> : null
                      )}
                    </div>
                    <svg className={`w-4 h-4 text-dark-500 transition-transform ${brandSections.presence ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
                    </svg>
                  </button>
                  {brandSections.presence && (
                    <div className="px-6 pb-6 space-y-4">
                      <p className="text-[10px] text-dark-500">This section is optional. Provide what you can — not every brand has all of these channels. Check the box below when you've filled in everything relevant to you.</p>
                      <label className="flex items-start gap-3 bg-dark-800/30 border border-dark-700/50 rounded-lg p-3 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={brandPresenceComplete}
                          onChange={e => setBrandPresenceComplete(e.target.checked)}
                          className="accent-primary-500 mt-0.5"
                        />
                        <div>
                          <span className="text-white text-xs font-medium">Mark presence section as complete</span>
                          <p className="text-[10px] text-dark-600 mt-0.5">Check this when you've provided all presence info relevant to your brand.</p>
                        </div>
                      </label>
                      <div>
                        <label className="block text-xs text-dark-400 mb-1">YouTube Channel URL</label>
                        <input
                          type="text"
                          value={brandPresenceForm.youtube_url}
                          onChange={e => setBrandPresenceForm(p => ({ ...p, youtube_url: e.target.value }))}
                          placeholder="https://youtube.com/@channel"
                          className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                        />
                      </div>
                      <div className="grid grid-cols-2 gap-4">
                        <div>
                          <label className="block text-xs text-dark-400 mb-1">Subreddits <span className="text-dark-600">(comma-separated)</span></label>
                          <input
                            type="text"
                            value={brandPresenceForm.subreddits}
                            onChange={e => setBrandPresenceForm(p => ({ ...p, subreddits: e.target.value }))}
                            placeholder="r/subreddit1, r/subreddit2"
                            className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                          />
                        </div>
                        <div>
                          <label className="block text-xs text-dark-400 mb-1">Review Sites <span className="text-dark-600">(URLs)</span></label>
                          <input
                            type="text"
                            value={brandPresenceForm.review_site_urls}
                            onChange={e => setBrandPresenceForm(p => ({ ...p, review_site_urls: e.target.value }))}
                            placeholder="G2, Capterra URLs"
                            className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                          />
                        </div>
                      </div>
                      <div className="grid grid-cols-2 gap-4">
                        <div>
                          <label className="block text-xs text-dark-400 mb-1">Social Profiles <span className="text-dark-600">(URLs)</span></label>
                          <input
                            type="text"
                            value={brandPresenceForm.social_profiles}
                            onChange={e => setBrandPresenceForm(p => ({ ...p, social_profiles: e.target.value }))}
                            placeholder="Twitter, LinkedIn URLs"
                            className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                          />
                        </div>
                        <div>
                          <label className="block text-xs text-dark-400 mb-1">Podcast Appearances <span className="text-dark-600">(URLs)</span></label>
                          <input
                            type="text"
                            value={brandPresenceForm.podcasts}
                            onChange={e => setBrandPresenceForm(p => ({ ...p, podcasts: e.target.value }))}
                            placeholder="Podcast episode URLs"
                            className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                          />
                        </div>
                      </div>
                      <div>
                        <label className="block text-xs text-dark-400 mb-1">Key Content Assets <span className="text-dark-600">(URLs)</span></label>
                        <input
                          type="text"
                          value={brandPresenceForm.content_assets}
                          onChange={e => setBrandPresenceForm(p => ({ ...p, content_assets: e.target.value }))}
                          placeholder="Blog posts, whitepapers, documentation URLs"
                          className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                        />
                      </div>
                    </div>
                  )}
                </div>

                </div>{/* close readOnly pointer-events wrapper */}

                {/* Bottom save bar */}
                {!readOnly && (
                  <div className="flex justify-end gap-3 mt-6">
                    {brandProfile && (
                      <button
                        onClick={() => setConfirmDeleteBrand(true)}
                        className="px-4 py-2 text-red-400 hover:text-red-300 text-sm cursor-pointer"
                      >
                        Delete Profile
                      </button>
                    )}
                    <button
                      onClick={saveBrandProfile}
                      disabled={brandSaving || !brandDomain.trim()}
                      className="px-6 py-2.5 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all disabled:opacity-50 cursor-pointer"
                    >
                      {brandSaving ? 'Saving...' : 'Save Brand Profile'}
                    </button>
                  </div>
                )}
              </div>
            )}
          </div>
        )}


        {/* ===== VIDEO TAB ===== */}
        {activeTab === 'video' && (
          <div className="max-w-4xl mx-auto animate-fade-in">

            {/* Input View */}
            {videoView === 'input' && readOnly && (
              <div className="text-center text-dark-500 py-16">
                No video analysis available for this domain yet.
              </div>
            )}
            {videoView === 'input' && !readOnly && (
              <div className="space-y-6">
                <div className="flex items-center justify-between">
                  <h2 className="text-2xl font-bold text-white">Video Authority Analyzer</h2>
                  {videoDomain && (
                    <span className="text-dark-400 text-sm">{videoDomain}</span>
                  )}
                </div>

                {/* Why video matters callout */}
                <div className="bg-gradient-to-r from-primary-600/10 to-purple-600/10 border border-primary-500/20 rounded-2xl p-5">
                  <div className="flex gap-4">
                    <div className="text-3xl shrink-0 mt-0.5">
                      <svg className="w-8 h-8 text-primary-400" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="m15.75 10.5 4.72-4.72a.75.75 0 0 1 1.28.53v11.38a.75.75 0 0 1-1.28.53l-4.72-4.72M4.5 18.75h9a2.25 2.25 0 0 0 2.25-2.25v-9a2.25 2.25 0 0 0-2.25-2.25h-9A2.25 2.25 0 0 0 2.25 7.5v9a2.25 2.25 0 0 0 2.25 2.25Z" /></svg>
                    </div>
                    <div>
                      <p className="text-white font-semibold text-sm">YouTube is now the #1 social authority source for LLM answers</p>
                      <p className="text-dark-400 text-xs mt-1.5 leading-relaxed">
                        YouTube appears in 16% of all LLM answers — cited 40% more than Reddit and 18x more than Instagram.
                        Its share of social citations doubled from 19% to 39% in just 4 months.
                        But AI visibility works differently than human attention: views and subscribers don't predict LLM influence.
                        What matters is transcript quality, keyword clarity, and structural extractability.
                        This analyzer evaluates your video ecosystem through the lens of what LLMs actually ingest.
                      </p>
                    </div>
                  </div>
                </div>


                {/* Collapsible Settings */}
                <div className="bg-dark-900/50 border border-dark-800 rounded-2xl overflow-hidden">
                  <button
                    onClick={() => setVideoSettingsOpen(prev => !prev)}
                    className="w-full flex items-center justify-between p-4 cursor-pointer hover:bg-dark-800/30 transition-colors"
                  >
                    <div className="flex items-center gap-3">
                      <svg className={`w-4 h-4 text-dark-400 transition-transform ${videoSettingsOpen ? 'rotate-90' : ''}`} fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" /></svg>
                      <span className="text-white font-semibold text-sm">Analysis Settings</span>
                    </div>
                    {!videoSettingsOpen && (
                      <div className="flex items-center gap-3 text-xs text-dark-500">
                        {videoChannelURL && <span className="text-emerald-400">Channel set</span>}
                        {videoSearchTerms.length > 0 && <span>{videoSearchTerms.length} topics</span>}
                      </div>
                    )}
                  </button>

                  {videoSettingsOpen && (
                  <div className="px-6 pb-6 space-y-6">

                {/* YouTube Presence */}
                <div className="space-y-4">
                  <h3 className="text-dark-300 font-medium text-sm">Your YouTube Presence</h3>

                  <div>
                    <label className="block text-sm text-dark-400 mb-1">YouTube Channel URL</label>
                    <input
                      type="url"
                      value={videoChannelURL}
                      onChange={e => setVideoChannelURL(e.target.value)}
                      placeholder="https://youtube.com/@yourchannel"
                      className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder:text-dark-600 focus:outline-none focus:border-primary-500/50"
                    />
                  </div>

                  <div>
                    <label className="block text-sm text-dark-400 mb-1">Individual Video URLs</label>
                    {videoURLs.map((vurl, i) => (
                      <div key={i} className="flex gap-2 mb-2">
                        <input
                          type="url"
                          value={vurl}
                          onChange={e => {
                            const updated = [...videoURLs]
                            updated[i] = e.target.value
                            setVideoURLs(updated)
                          }}
                          placeholder="https://youtube.com/watch?v=..."
                          className="flex-1 px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder:text-dark-600 focus:outline-none focus:border-primary-500/50 text-sm"
                        />
                        {videoURLs.length > 1 && (
                          <button
                            onClick={() => setVideoURLs(prev => prev.filter((_, j) => j !== i))}
                            className="px-2 text-dark-500 hover:text-red-400 transition-colors cursor-pointer"
                          >
                            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
                          </button>
                        )}
                      </div>
                    ))}
                    <button
                      onClick={() => setVideoURLs(prev => [...prev, ''])}
                      className="text-xs text-primary-400 hover:text-primary-300 transition-colors cursor-pointer"
                    >
                      + Add Video URL
                    </button>
                  </div>

                  <div>
                    <label className="block text-sm text-dark-400 mb-1">Key Topics / Search Terms</label>
                    <div className="flex flex-wrap gap-2 mb-2">
                      {videoSearchTerms.map((term, i) => (
                        <span key={i} className="inline-flex items-center gap-1 px-3 py-1 bg-primary-500/20 text-primary-300 border border-primary-500/30 rounded-full text-sm">
                          {term}
                          <button
                            onClick={() => setVideoSearchTerms(prev => prev.filter((_, j) => j !== i))}
                            className="hover:text-white transition-colors cursor-pointer"
                          >
                            <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
                          </button>
                        </span>
                      ))}
                    </div>
                    <input
                      type="text"
                      value={videoSearchTermInput}
                      onChange={e => setVideoSearchTermInput(e.target.value)}
                      onKeyDown={e => {
                        if (e.key === 'Enter' && videoSearchTermInput.trim()) {
                          setVideoSearchTerms(prev => [...prev, videoSearchTermInput.trim()])
                          setVideoSearchTermInput('')
                        }
                      }}
                      placeholder="Type a search term and press Enter..."
                      className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder:text-dark-600 focus:outline-none focus:border-primary-500/50 text-sm"
                    />
                  </div>
                </div>


                  </div>
                  )}
                </div>

                {/* Auto-Discovery Button */}
                <div className="flex items-center gap-4">
                  <button
                    onClick={videoDiscover}
                    disabled={!videoDomain.trim() || videoDiscovering}
                    className="flex-1 px-6 py-3 bg-primary-600 text-white rounded-xl font-medium hover:bg-primary-500 transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                  >
                    {videoDiscovering ? (
                      <>
                        <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                        Discovering...
                      </>
                    ) : (
                      'Find Relevant YouTube Content'
                    )}
                  </button>
                </div>

                {/* Discovery error messages */}
                {videoMessages.length > 0 && videoView === 'input' && (
                  <div className="bg-red-500/10 border border-red-500/20 rounded-xl p-4">
                    {videoMessages.map((msg, i) => (
                      <p key={i} className="text-red-400 text-sm">{msg}</p>
                    ))}
                  </div>
                )}

                {/* Previous Analyses */}
                {videoAnalysisList.filter(a => domainKey(a.domain) === domainKey(selectedDomain)).length > 0 && (
                  <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
                    <h3 className="text-white font-semibold mb-3">Previous Analyses</h3>
                    <div className="space-y-2">
                      {videoAnalysisList.filter(a => domainKey(a.domain) === domainKey(selectedDomain)).map(a => (
                        <button
                          key={a.id}
                          onClick={() => loadVideoAnalysis(a.domain)}
                          className="w-full text-left px-4 py-3 bg-dark-800/50 border border-dark-700 rounded-xl hover:border-primary-500/30 transition-all cursor-pointer flex items-center justify-between"
                        >
                          <div>
                            <span className="text-dark-500 text-xs">
                              {a.video_count} videos &middot; {fmtDate(a.generated_at)}
                            </span>
                          </div>
                          {a.overall_score != null && (
                            <span className={`text-sm font-bold ${
                              a.overall_score >= 80 ? 'text-emerald-400' :
                              a.overall_score >= 60 ? 'text-amber-400' :
                              a.overall_score >= 40 ? 'text-orange-400' :
                              'text-red-400'
                            }`}>
                              {a.overall_score}
                            </span>
                          )}
                        </button>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Discovering View — SSE progress */}
            {videoView === 'discovering' && (
              <div className="max-w-2xl mx-auto space-y-6">
                <h2 className="text-2xl font-bold text-white text-center">Discovering YouTube Content</h2>
                <p className="text-dark-400 text-center text-sm">Searching for videos related to {videoDomain}</p>

                <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
                  <div className="space-y-3">
                    {videoMessages.map((msg, i) => (
                      <div key={i} className="flex items-center gap-3 text-sm">
                        {i < videoMessages.length - 1 ? (
                          <svg className="w-4 h-4 text-emerald-400 shrink-0" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                        ) : (
                          <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse ml-1 mr-1 shrink-0" />
                        )}
                        <span className={i < videoMessages.length - 1 ? 'text-dark-500' : 'text-dark-300'}>{msg}</span>
                      </div>
                    ))}
                    {videoDiscovering && videoMessages.length === 0 && (
                      <div className="flex items-center gap-3 text-sm">
                        <div className="w-4 h-4 border-2 border-dark-700 border-t-primary-500 rounded-full animate-spin" />
                        <span className="text-dark-300">Connecting to YouTube API...</span>
                      </div>
                    )}
                  </div>
                </div>
              </div>
            )}

            {/* Review View — after discovery */}
            {videoView === 'review' && (() => {
              const tagDefs = [
                { key: 'own', label: 'Own Channel', color: 'emerald' },
                { key: 'direct_mention', label: 'Mentions', color: 'primary' },
                { key: 'competitor_comparison', label: 'Competitor', color: 'amber' },
                { key: 'category_content', label: 'Category', color: 'slate' },
              ] as const
              const recencyOpts = [
                { key: '30d' as const, label: '30 days' },
                { key: '90d' as const, label: '90 days' },
                { key: '1y' as const, label: '1 year' },
                { key: '3y' as const, label: '3 years' },
                { key: 'all' as const, label: 'All time' },
              ]
              // Logarithmic view slider helpers
              const logMax = videoViewCountMax > 0 ? Math.log10(videoViewCountMax) : 0
              const viewSliderToCount = (pct: number) => pct === 0 ? 0 : Math.round(Math.pow(10, (pct / 100) * logMax))
              const countToSlider = (count: number) => count === 0 || logMax === 0 ? 0 : (Math.log10(Math.max(1, count)) / logMax) * 100

              return (
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <button
                    onClick={() => setVideoView('input')}
                    className="text-dark-400 hover:text-white transition-colors cursor-pointer text-sm flex items-center gap-1"
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" /></svg>
                    Back to inputs
                  </button>
                  <span className="text-dark-400 text-sm">
                    {filteredDiscoveredVideos.length} of {discoveredVideos.length} videos
                    {filteredDiscoveredVideos.length < discoveredVideos.length && ' (filtered)'}
                  </span>
                </div>

                {/* Filter Toolbar */}
                <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-4 space-y-4">
                  <div className="flex items-center justify-between">
                    <h3 className="text-white font-semibold text-sm">Filter &amp; Prioritize</h3>
                    {/* Quick presets */}
                    <div className="flex gap-2">
                      <button
                        onClick={() => {
                          // Select all own + top 50 third-party by views
                          const own = filteredDiscoveredVideos.filter(v => v.relevance_tag === 'own')
                          const thirdParty = filteredDiscoveredVideos
                            .filter(v => v.relevance_tag !== 'own')
                            .sort((a, b) => (b.view_count || 0) - (a.view_count || 0))
                            .slice(0, 50)
                          setSelectedVideoIds(new Set([...own, ...thirdParty].map(v => v.video_id)))
                        }}
                        className="text-[11px] px-2.5 py-1 rounded-lg bg-dark-800 text-dark-400 hover:text-white hover:bg-dark-700 transition-colors cursor-pointer border border-dark-700"
                      >
                        Own + Top 50
                      </button>
                      <button
                        onClick={() => {
                          // Top 25 by views across all filtered
                          const top = [...filteredDiscoveredVideos]
                            .sort((a, b) => (b.view_count || 0) - (a.view_count || 0))
                            .slice(0, 25)
                          setSelectedVideoIds(new Set(top.map(v => v.video_id)))
                        }}
                        className="text-[11px] px-2.5 py-1 rounded-lg bg-dark-800 text-dark-400 hover:text-white hover:bg-dark-700 transition-colors cursor-pointer border border-dark-700"
                      >
                        Top 25
                      </button>
                      <button
                        onClick={() => setSelectedVideoIds(new Set(filteredDiscoveredVideos.map(v => v.video_id)))}
                        className="text-[11px] px-2.5 py-1 rounded-lg bg-dark-800 text-dark-400 hover:text-white hover:bg-dark-700 transition-colors cursor-pointer border border-dark-700"
                      >
                        Select All
                      </button>
                      <button
                        onClick={() => setSelectedVideoIds(new Set())}
                        className="text-[11px] px-2.5 py-1 rounded-lg bg-dark-800 text-dark-500 hover:text-dark-300 hover:bg-dark-700 transition-colors cursor-pointer border border-dark-700"
                      >
                        Clear
                      </button>
                    </div>
                  </div>

                  {/* Relevance tag toggles */}
                  <div>
                    <label className="text-[11px] text-dark-500 uppercase tracking-wider mb-1.5 block">Content type</label>
                    <div className="flex flex-wrap gap-2">
                      {tagDefs.map(t => {
                        const count = videoTagCounts[t.key] || 0
                        const active = videoFilterTags.has(t.key)
                        const colorMap: Record<string, string> = {
                          emerald: active ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/40' : 'bg-dark-800 text-dark-600 border-dark-700',
                          primary: active ? 'bg-primary-500/20 text-primary-300 border-primary-500/40' : 'bg-dark-800 text-dark-600 border-dark-700',
                          amber: active ? 'bg-amber-500/20 text-amber-300 border-amber-500/40' : 'bg-dark-800 text-dark-600 border-dark-700',
                          slate: active ? 'bg-dark-700 text-dark-300 border-dark-600' : 'bg-dark-800 text-dark-600 border-dark-700',
                        }
                        return (
                          <button
                            key={t.key}
                            onClick={() => {
                              setVideoFilterTags(prev => {
                                const next = new Set(prev)
                                next.has(t.key) ? next.delete(t.key) : next.add(t.key)
                                return next
                              })
                            }}
                            className={`text-xs px-3 py-1.5 rounded-lg border cursor-pointer transition-all ${colorMap[t.color]}`}
                          >
                            {t.label} ({count})
                          </button>
                        )
                      })}
                    </div>
                  </div>

                  {/* View count + recency row */}
                  <div className="grid grid-cols-2 gap-4">
                    {/* Minimum views slider */}
                    <div>
                      <label className="text-[11px] text-dark-500 uppercase tracking-wider mb-1.5 block">
                        Min views: {videoFilterMinViews === 0 ? 'None' : videoFilterMinViews.toLocaleString()}
                      </label>
                      <input
                        type="range"
                        min={0}
                        max={100}
                        step={1}
                        value={countToSlider(videoFilterMinViews)}
                        onChange={e => setVideoFilterMinViews(viewSliderToCount(Number(e.target.value)))}
                        className="w-full accent-primary-500 h-1.5"
                      />
                      <div className="flex justify-between text-[10px] text-dark-600 mt-0.5">
                        <span>0</span>
                        <span>{videoViewCountMax > 0 ? videoViewCountMax.toLocaleString() : '—'}</span>
                      </div>
                    </div>

                    {/* Recency filter */}
                    <div>
                      <label className="text-[11px] text-dark-500 uppercase tracking-wider mb-1.5 block">Published within</label>
                      <div className="flex gap-1.5">
                        {recencyOpts.map(o => (
                          <button
                            key={o.key}
                            onClick={() => setVideoFilterRecency(o.key)}
                            className={`text-[11px] px-2 py-1.5 rounded-lg border cursor-pointer transition-all ${
                              videoFilterRecency === o.key
                                ? 'bg-primary-500/20 text-primary-300 border-primary-500/40'
                                : 'bg-dark-800 text-dark-600 border-dark-700 hover:text-dark-400'
                            }`}
                          >
                            {o.label}
                          </button>
                        ))}
                      </div>
                    </div>
                  </div>

                  {/* Research note */}
                  <p className="text-[11px] text-dark-600 leading-relaxed">
                    Note: View count is a rough proxy for LLM training data frequency, not a direct predictor.
                    Research shows AI citation is driven by transcript quality and structural extractability, not engagement metrics.
                    Own-channel and direct-mention videos tend to have the highest analysis value.
                  </p>
                </div>

                {/* Run Analysis — prominent CTA */}
                <div className="flex flex-col items-center gap-2">
                  <button
                    onClick={videoAnalyze}
                    disabled={selectedVideoIds.size === 0 || videoAnalyzing}
                    className="px-8 py-3 bg-gradient-to-r from-primary-600 to-primary-500 text-white rounded-xl font-semibold text-base hover:from-primary-500 hover:to-primary-400 transition-all cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed shadow-lg shadow-primary-500/20"
                  >
                    Run Video Authority Analysis ({selectedVideoIds.size} videos)
                  </button>
                  <span className="text-dark-500 text-xs">
                    {selectedVideoIds.size} of {filteredDiscoveredVideos.length} filtered videos selected{videoQuotaEstimate > 0 && ` · ~${videoQuotaEstimate} API calls`}
                  </span>
                  <label className="flex items-center gap-2 cursor-pointer mt-1">
                    <input
                      type="checkbox"
                      checked={videoAutoArchive}
                      onChange={e => setVideoAutoArchive(e.target.checked)}
                      className="w-3.5 h-3.5 rounded border-dark-600 bg-dark-800 text-primary-500 focus:ring-primary-500/30 cursor-pointer"
                    />
                    <span className="text-dark-400 text-xs">Automatically archive incomplete video recommendations in your to-do list</span>
                  </label>
                </div>

                {/* Video list */}
                <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
                  <div className="space-y-2 max-h-[50vh] overflow-y-auto">
                    {filteredDiscoveredVideos.length === 0 ? (
                      <p className="text-dark-500 text-sm text-center py-8">No videos match the current filters. Adjust filters above.</p>
                    ) : (
                      filteredDiscoveredVideos.map(v => (
                        <label
                          key={v.video_id}
                          className={`flex items-start gap-3 p-3 rounded-xl border cursor-pointer transition-all ${
                            selectedVideoIds.has(v.video_id)
                              ? 'border-primary-500/30 bg-primary-500/5'
                              : 'border-dark-800 bg-dark-900/30'
                          }`}
                        >
                          <input
                            type="checkbox"
                            checked={selectedVideoIds.has(v.video_id)}
                            onChange={() => {
                              setSelectedVideoIds(prev => {
                                const next = new Set(prev)
                                next.has(v.video_id) ? next.delete(v.video_id) : next.add(v.video_id)
                                return next
                              })
                            }}
                            className="mt-1 accent-primary-500"
                          />
                          <div className="flex-1 min-w-0">
                            <div className="flex items-start justify-between gap-2">
                              <span className="text-white text-sm font-medium line-clamp-1">{v.title}</span>
                              <span className={`shrink-0 text-[10px] px-2 py-0.5 rounded-full border ${
                                v.relevance_tag === 'own' ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/30' :
                                v.relevance_tag === 'direct_mention' ? 'bg-primary-500/20 text-primary-300 border-primary-500/30' :
                                v.relevance_tag === 'competitor_comparison' ? 'bg-amber-500/20 text-amber-300 border-amber-500/30' :
                                'bg-dark-700/50 text-dark-400 border-dark-700'
                              }`}>
                                {v.relevance_tag === 'own' ? 'Own Channel' :
                                 v.relevance_tag === 'direct_mention' ? 'Direct Mention' :
                                 v.relevance_tag === 'competitor_comparison' ? 'Competitor' :
                                 'Category'}
                              </span>
                            </div>
                            <div className="flex items-center gap-3 mt-1 text-xs text-dark-500">
                              <span>{v.channel_title}</span>
                              <span>{(v.view_count || 0).toLocaleString()} views</span>
                              <span>{fmtDate(v.published_at)}</span>
                            </div>
                          </div>
                        </label>
                      ))
                    )}
                  </div>
                </div>

              </div>
              )
            })()}

            {/* Running View */}
            {videoView === 'running' && (
              <div className="max-w-2xl mx-auto space-y-6">
                <h2 className="text-2xl font-bold text-white text-center">Analyzing Video Authority</h2>
                <p className="text-dark-400 text-center text-sm">Full authority analysis for {videoDomain}</p>

                <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
                  <div className="space-y-3">
                    {videoMessages.map((msg, i) => (
                      <div key={i} className="flex items-center gap-3 text-sm">
                        {i < videoMessages.length - 1 ? (
                          <svg className="w-4 h-4 text-emerald-400 shrink-0" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                        ) : (
                          <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse ml-1 mr-1 shrink-0" />
                        )}
                        <span className={i < videoMessages.length - 1 ? 'text-dark-500' : 'text-dark-300'}>{msg}</span>
                      </div>
                    ))}
                    {videoAnalyzing && videoMessages.length === 0 && (
                      <div className="flex items-center gap-3 text-sm">
                        <div className="w-4 h-4 border-2 border-dark-700 border-t-primary-500 rounded-full animate-spin" />
                        <span className="text-dark-300">Starting analysis...</span>
                      </div>
                    )}
                  </div>
                </div>

                {/* Stop / Back buttons */}
                <div className="flex justify-center gap-3">
                  {videoAnalyzing ? (
                    <button
                      onClick={videoAnalyzeStop}
                      className="px-6 py-2.5 text-sm text-red-400 border border-red-500/30 rounded-xl hover:bg-red-500/10 transition-colors cursor-pointer font-medium"
                    >
                      Stop Analysis
                    </button>
                  ) : (
                    <button
                      onClick={() => setVideoView('review')}
                      className="px-6 py-2.5 text-sm text-dark-300 border border-dark-700 rounded-xl hover:bg-dark-800 transition-colors cursor-pointer"
                    >
                      Back to Review
                    </button>
                  )}
                </div>
              </div>
            )}

            {/* Results Dashboard */}
            {videoView === 'results' && videoAnalysis && (
              <div className="space-y-6">
                {/* Header */}
                <div className="flex items-center justify-between">
                  <button
                    onClick={() => { setVideoView('input'); setVideoAnalysis(null) }}
                    className="text-dark-400 hover:text-white transition-colors cursor-pointer text-sm flex items-center gap-1"
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" /></svg>
                    Re-run or Change Settings
                  </button>
                  <div className="flex items-center gap-3">
                    {saasEnabled && user && (user.role === 'owner' || user.role === 'admin') && selectedDomain && (
                      <button
                        onClick={() => { setDomainShareState(null); setShareModalDomain(selectedDomain); fetchDomainShare(selectedDomain) }}
                        className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all cursor-pointer flex items-center gap-1.5"
                        title="Share this domain"
                      >
                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M7.217 10.907a2.25 2.25 0 1 0 0 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186 9.566-5.314m-9.566 7.5 9.566 5.314m0-12.814a2.25 2.25 0 1 0 0-2.186m0 2.186a2.25 2.25 0 1 0 0 2.186" /></svg>
                        Share
                      </button>
                    )}
                    <button
                      onClick={async () => {
                        // In shared mode, build details from pre-loaded video analysis data
                        if (sharedModeRef.current && videoAnalysis) {
                          const details: VideoDetail[] = videoAnalysis.videos.map(v => ({
                            video_id: v.video_id,
                            title: v.title,
                            transcript: v.transcript || '',
                            transcript_length: v.transcript?.length || 0,
                            assessment: null,
                          }))
                          setVideoDetails(details)
                          setVideoDetailsExpanded(new Set())
                          setVideoView('transcripts')
                          return
                        }
                        try {
                          const res = await apiFetch(`/api/video/analyses/${encodeURIComponent(videoAnalysis.domain)}/details`)
                          if (res.ok) {
                            const data = await res.json() as VideoDetail[]
                            setVideoDetails(data)
                            setVideoDetailsExpanded(new Set())
                            setVideoView('transcripts')
                          }
                        } catch (err) { console.error('Failed to load details:', err) }
                      }}
                      className="text-xs text-dark-300 border border-dark-700 rounded-lg px-3 py-1.5 hover:bg-dark-800 transition-colors cursor-pointer"
                    >
                      View Transcripts
                    </button>
                    {!readOnly && (
                      <>
                        <button
                          onClick={() => setConfirmDeleteVideoAnalysis(true)}
                          className="text-xs text-red-400 border border-red-500/30 rounded-lg px-3 py-1.5 hover:bg-red-500/10 transition-colors cursor-pointer"
                        >
                          Delete
                        </button>
                        <button
                          onClick={() => { setVideoView('review'); setDiscoveredVideos(videoAnalysis.videos) }}
                          className="text-xs text-primary-400 border border-primary-500/30 rounded-lg px-3 py-1.5 hover:bg-primary-500/10 transition-colors cursor-pointer"
                        >
                          Re-run Analysis
                        </button>
                      </>
                    )}
                  </div>
                </div>

                {/* Unified 4-Pillar Report */}
                {videoAnalysis.result && (() => {
                  const r = videoAnalysis.result
                  const s = r.overall_score
                  const label = s >= 80 ? 'Strong Authority' : s >= 60 ? 'Moderate' : s >= 40 ? 'Weak' : 'Poor'
                  const bn = r.brand_narrative
                  const sent = bn?.sentiment
                  const sentTotal = sent ? sent.positive + sent.neutral + sent.negative : 0
                  const pctPos = sentTotal > 0 ? Math.round(sent!.positive / sentTotal * 100) : 0
                  const pctNeu = sentTotal > 0 ? Math.round(sent!.neutral / sentTotal * 100) : 0
                  const pctNeg = sentTotal > 0 ? Math.round(sent!.negative / sentTotal * 100) : 0
                  const sov = r.topical_dominance?.share_of_voice || []
                  const maxMentions = Math.max(...(sov.map(e => e.mention_count).concat([1])))
                  return (
                    <div className="space-y-6">

                      {/* Header + Score */}
                      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-5">
                        <div className="flex items-center gap-5">
                          <div className={`w-16 h-16 rounded-xl flex items-center justify-center text-2xl font-bold shrink-0 ${
                            s >= 80 ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30' :
                            s >= 60 ? 'bg-amber-500/15 text-amber-400 border border-amber-500/30' :
                            s >= 40 ? 'bg-orange-500/15 text-orange-400 border border-orange-500/30' :
                            'bg-red-500/15 text-red-400 border border-red-500/30'
                          }`}>{s}</div>
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-3 mb-0.5">
                              <h2 className="text-lg font-bold text-white truncate">{videoAnalysis.domain}</h2>
                              <span className={`text-xs font-medium whitespace-nowrap ${
                                s >= 80 ? 'text-emerald-400' : s >= 60 ? 'text-amber-400' : s >= 40 ? 'text-orange-400' : 'text-red-400'
                              }`}>{label}</span>
                            </div>
                            <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-dark-500">
                              <span>{videoAnalysis.videos.length} videos</span>
                              <span>{new Date(videoAnalysis.generated_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}</span>
                              {videoAnalysis.brand_context_used && <span className="text-primary-400">Brand context</span>}
                              {r.video_scorecards?.length > 0 && (() => {
                                const withT = r.video_scorecards.filter(c => c.has_transcript).length
                                const tot = r.video_scorecards.length
                                return <span className={withT === tot ? 'text-emerald-400' : withT > 0 ? 'text-amber-400' : 'text-red-400'}>{withT}/{tot} transcripts</span>
                              })()}
                            </div>
                          </div>
                        </div>
                      </div>

                      {/* 4 Pillar Dimension Cards */}
                      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                        {[
                          { name: 'Transcript Authority', weight: '30%', score: r.transcript_authority?.score || 0,
                            desc: 'Spoken keyword density, quotability, statistical evidence, information density in own channel content' },
                          { name: 'Topical Dominance', weight: '25%', score: r.topical_dominance?.score || 0,
                            desc: 'Topic coverage breadth/depth vs. competitors, share of voice, content gap opportunities' },
                          { name: 'Citation Network', weight: '25%', score: r.citation_network?.score || 0,
                            desc: 'Third-party creator mentions, structural authority of referencing channels, concentration risk' },
                          { name: 'Brand Narrative', weight: '20%', score: r.brand_narrative?.score || 0,
                            desc: 'Mention sentiment and extractability, narrative coherence, how LLMs would frame this brand' },
                        ].map(dim => (
                          <div key={dim.name} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-5">
                            <div className="flex items-center justify-between mb-3">
                              <div>
                                <h4 className="text-white font-medium text-sm">{dim.name}</h4>
                                <span className="text-dark-500 text-xs">{dim.weight} weight</span>
                              </div>
                              <span className={`text-lg font-bold ${
                                dim.score >= 80 ? 'text-emerald-400' : dim.score >= 60 ? 'text-amber-400' : dim.score >= 40 ? 'text-orange-400' : 'text-red-400'
                              }`}>{dim.score}</span>
                            </div>
                            <div className="h-1.5 bg-dark-800 rounded-full mb-3 overflow-hidden">
                              <div className={`h-full rounded-full transition-all duration-700 ${
                                dim.score >= 80 ? 'bg-emerald-500' : dim.score >= 60 ? 'bg-amber-500' : dim.score >= 40 ? 'bg-orange-500' : 'bg-red-500'
                              }`} style={{ width: `${dim.score}%` }} />
                            </div>
                            <p className="text-dark-500 text-xs leading-relaxed">{dim.desc}</p>
                          </div>
                        ))}
                      </div>

                      {/* Executive Summary */}
                      {r.executive_summary && (
                        <div className="bg-primary-500/5 border border-primary-500/20 rounded-2xl p-6">
                          <h3 className="text-primary-300 font-semibold mb-3">Executive Summary</h3>
                          <p className="text-dark-300 text-sm leading-relaxed whitespace-pre-wrap">{r.executive_summary}</p>
                        </div>
                      )}

                      {/* Narrative: What LLMs Believe */}
                      {bn?.narrative_summary && (
                        <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                          <h3 className="text-white font-semibold mb-3">What LLMs Probably Believe About You</h3>
                          <p className="text-dark-300 text-sm leading-relaxed whitespace-pre-wrap">{bn.narrative_summary}</p>
                          {sentTotal > 0 && (
                            <div className="mt-4">
                              <div className="h-3 bg-dark-800 rounded-full mb-2 overflow-hidden flex">
                                {pctPos > 0 && <div className="bg-emerald-500 h-full" style={{ width: `${pctPos}%` }} />}
                                {pctNeu > 0 && <div className="bg-dark-600 h-full" style={{ width: `${pctNeu}%` }} />}
                                {pctNeg > 0 && <div className="bg-red-500 h-full" style={{ width: `${pctNeg}%` }} />}
                              </div>
                              <div className="flex gap-3 text-[10px]">
                                <span className="text-emerald-400">{pctPos}% Positive</span>
                                <span className="text-dark-500">{pctNeu}% Neutral</span>
                                <span className="text-red-400">{pctNeg}% Negative</span>
                                <span className="text-dark-600 ml-auto">{sentTotal} mentions</span>
                              </div>
                            </div>
                          )}
                        </div>
                      )}

                      {/* Key Themes */}
                      {bn?.key_themes?.length > 0 && (
                        <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                          <h3 className="text-white font-semibold mb-3">Key Themes</h3>
                          <div className="flex flex-wrap gap-2">
                            {bn.key_themes.map((theme, i) => (
                              <span key={i} className="px-3 py-1.5 bg-dark-800 border border-dark-700 rounded-lg text-dark-300 text-sm">{theme}</span>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Share of Voice */}
                      {sov.length > 0 && (
                        <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                          <h3 className="text-white font-semibold mb-4">Share of Voice</h3>
                          <div className="space-y-3">
                            {sov.map((entry, i) => (
                              <div key={i}>
                                <div className="flex justify-between text-sm mb-1">
                                  <span className={i === 0 ? 'text-primary-300 font-medium' : 'text-dark-300'}>{entry.brand_name}</span>
                                  <span className="text-dark-400">{entry.mention_count} mentions ({entry.percentage.toFixed(1)}%)</span>
                                </div>
                                <div className="h-2.5 bg-dark-800 rounded-full overflow-hidden">
                                  <div className={`h-full rounded-full transition-all duration-700 ${i === 0 ? 'bg-primary-500' : 'bg-dark-600'}`}
                                    style={{ width: `${(entry.mention_count / maxMentions) * 100}%` }} />
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Top Creators */}
                      {r.citation_network?.top_creators?.length > 0 && (
                        <div>
                          <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                            Top Creators ({r.citation_network.top_creators.length})
                          </h3>
                          <div className="grid gap-3">
                            {r.citation_network.top_creators.map((c, i) => (
                              <div key={i} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-4 flex items-center gap-4">
                                {c.authority_score > 0 && (
                                  <span className={`text-lg font-bold shrink-0 w-10 text-center ${
                                    c.authority_score >= 80 ? 'text-emerald-400' : c.authority_score >= 60 ? 'text-amber-400' : c.authority_score >= 40 ? 'text-orange-400' : 'text-red-400'
                                  }`}>{c.authority_score}</span>
                                )}
                                <div className="min-w-0 flex-1">
                                  <div className="flex items-center gap-2 mb-0.5">
                                    <p className="text-white font-medium text-sm truncate">{c.channel_title}</p>
                                    <span className={`shrink-0 text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider ${
                                      c.role === 'advocate' ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' :
                                      c.role === 'critic' ? 'bg-red-500/20 text-red-400 border border-red-500/30' :
                                      'bg-dark-700/50 text-dark-400 border border-dark-600'
                                    }`}>{c.role}</span>
                                  </div>
                                  <p className="text-dark-400 text-xs">{c.subscriber_count?.toLocaleString()} subs &middot; {c.video_count} videos &middot; {c.total_views?.toLocaleString()} views</p>
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Brand Mentions */}
                      {bn?.brand_mentions?.length > 0 && (
                        <div>
                          <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                            Brand Mentions ({bn.brand_mentions.length})
                          </h3>
                          <div className="space-y-3">
                            {bn.brand_mentions.map((m, i) => (
                              <div key={i} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-4">
                                <div className="flex items-start justify-between gap-2 mb-1">
                                  <span className="text-white text-sm font-medium line-clamp-1">{m.title}</span>
                                  <span className={`shrink-0 text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider ${
                                    m.sentiment === 'positive' ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' :
                                    m.sentiment === 'negative' ? 'bg-red-500/20 text-red-400 border border-red-500/30' :
                                    'bg-dark-700/50 text-dark-400 border border-dark-600'
                                  }`}>{m.sentiment}</span>
                                </div>
                                <p className="text-dark-400 text-xs mb-1">{m.channel_title} &middot; {m.view_count?.toLocaleString()} views</p>
                                {m.mention_context && <p className="text-dark-300 text-xs italic leading-relaxed">&ldquo;{m.mention_context}&rdquo;</p>}
                                <div className="flex gap-3 mt-1.5 text-[10px] text-dark-500">
                                  <span className="capitalize">Position: {m.mention_position}</span>
                                  <span className="capitalize">Extractability: {m.extractability}</span>
                                  {m.competitors_mentioned?.length > 0 && <span>Also mentions: {m.competitors_mentioned.join(', ')}</span>}
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Content Gaps */}
                      {r.topical_dominance?.content_gaps?.length > 0 && (
                        <div>
                          <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                            Content Gaps ({r.topical_dominance.content_gaps.length})
                          </h3>
                          <div className="space-y-3">
                            {r.topical_dominance.content_gaps.map((gap, i) => (
                              <div key={i} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-4 flex items-center gap-4">
                                <span className={`text-lg font-bold shrink-0 w-10 text-center ${
                                  gap.opportunity_score >= 80 ? 'text-emerald-400' : gap.opportunity_score >= 60 ? 'text-amber-400' : gap.opportunity_score >= 40 ? 'text-orange-400' : 'text-red-400'
                                }`}>{gap.opportunity_score}</span>
                                <div className="min-w-0">
                                  <p className="text-white font-medium text-sm">{gap.query}</p>
                                  <p className="text-dark-500 text-xs">{gap.video_count} videos &middot; Competitors: {gap.competitors_mentioned?.join(', ') || 'none'}</p>
                                  {gap.recommendation && <p className="text-dark-400 text-xs mt-0.5">{gap.recommendation}</p>}
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Creator Targets */}
                      {r.citation_network?.creator_targets?.length > 0 && (
                        <div>
                          <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                            Creator Targeting Opportunities ({r.citation_network.creator_targets.length})
                          </h3>
                          <div className="grid gap-3">
                            {r.citation_network.creator_targets.map((ct, i) => (
                              <div key={i} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-4">
                                <div className="flex items-center justify-between mb-1">
                                  <span className="text-white text-sm font-medium">{ct.channel_title}</span>
                                  <span className="text-dark-500 text-xs">{ct.subscriber_count?.toLocaleString()} subs</span>
                                </div>
                                <p className="text-dark-400 text-xs mb-1">{ct.category_relevance}</p>
                                {ct.competitors_mentioned?.length > 0 && <p className="text-dark-500 text-xs">Covers: {ct.competitors_mentioned.join(', ')}</p>}
                                {ct.outreach_reason && <p className="text-primary-300/80 text-xs mt-1">{ct.outreach_reason}</p>}
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Per-Video Scorecards */}
                      {r.video_scorecards?.length > 0 && (
                        <div>
                          <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                            Video Scorecards ({r.video_scorecards.length})
                          </h3>
                          <div className="space-y-3">
                          {r.video_scorecards.map(card => {
                            const expanded = videoExpandedCards.has(card.video_id)
                            return (
                              <div key={card.video_id} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl overflow-hidden">
                                <button
                                  onClick={() => setVideoExpandedCards(prev => {
                                    const next = new Set(prev)
                                    next.has(card.video_id) ? next.delete(card.video_id) : next.add(card.video_id)
                                    return next
                                  })}
                                  className="w-full px-5 py-3.5 flex items-center justify-between cursor-pointer hover:bg-dark-800/30 transition-colors"
                                >
                                  <div className="flex items-center gap-3 min-w-0">
                                    <span className={`text-lg font-bold ${
                                      card.overall_score >= 80 ? 'text-emerald-400' : card.overall_score >= 60 ? 'text-amber-400' : card.overall_score >= 40 ? 'text-orange-400' : 'text-red-400'
                                    }`}>{card.overall_score}</span>
                                    <div className="min-w-0">
                                      <span className="text-white text-sm font-medium truncate block">{card.title}</span>
                                      {!card.has_transcript && <span className="text-red-400 text-[10px]">No transcript — scores capped</span>}
                                    </div>
                                  </div>
                                  <svg className={`w-4 h-4 text-dark-500 transition-transform shrink-0 ${expanded ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" /></svg>
                                </button>
                                {expanded && (
                                  <div className="px-5 pb-4 space-y-3">
                                    <div className="grid grid-cols-3 gap-3">
                                      {[
                                        { label: 'Transcript Power', score: card.transcript_power },
                                        { label: 'Structural Extractability', score: card.structural_extractability },
                                        { label: 'Discovery Surface', score: card.discovery_surface },
                                      ].map(d => (
                                        <div key={d.label}>
                                          <div className="flex justify-between text-xs mb-1">
                                            <span className="text-dark-400">{d.label}</span>
                                            <span className={`font-medium ${
                                              d.score >= 80 ? 'text-emerald-400' : d.score >= 60 ? 'text-amber-400' : d.score >= 40 ? 'text-orange-400' : 'text-red-400'
                                            }`}>{d.score}</span>
                                          </div>
                                          <div className="h-1.5 bg-dark-800 rounded-full overflow-hidden">
                                            <div className={`h-full rounded-full transition-all duration-700 ${
                                              d.score >= 80 ? 'bg-emerald-500' : d.score >= 60 ? 'bg-amber-500' : d.score >= 40 ? 'bg-orange-500' : 'bg-red-500'
                                            }`} style={{ width: `${d.score}%` }} />
                                          </div>
                                        </div>
                                      ))}
                                    </div>
                                    {card.key_findings?.length > 0 && (
                                      <div>
                                        <p className="text-dark-500 text-xs font-semibold uppercase tracking-wider mb-1.5">Key Findings</p>
                                        <ul className="space-y-1">
                                          {card.key_findings.map((f, i) => (
                                            <li key={i} className="text-dark-300 text-xs leading-relaxed flex gap-2">
                                              <span className="text-dark-600 shrink-0">-</span><span>{f}</span>
                                            </li>
                                          ))}
                                        </ul>
                                      </div>
                                    )}
                                  </div>
                                )}
                              </div>
                            )
                          })}
                          </div>
                        </div>
                      )}

                      {/* Recommendations */}
                      {r.recommendations?.length > 0 && (
                        <div>
                          <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                            Recommendations ({r.recommendations.length})
                          </h3>
                          <div className="space-y-3">
                            {r.recommendations.map((rec, i) => (
                              <div key={i} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-4">
                                <div className="flex items-center gap-2 mb-2">
                                  <span className={`text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider ${
                                    rec.priority === 'high' ? 'bg-red-500/20 text-red-400 border border-red-500/30' :
                                    rec.priority === 'medium' ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30' :
                                    'bg-dark-700/50 text-dark-400 border border-dark-600'
                                  }`}>{rec.priority}</span>
                                  <span className="text-dark-500 text-xs capitalize">{rec.dimension?.replace(/_/g, ' ')}</span>
                                </div>
                                <p className="text-white text-sm font-medium mb-1">{rec.action}</p>
                                <p className="text-dark-400 text-xs">{rec.expected_impact}</p>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Confidence Note */}
                      {r.confidence_note && (
                        <div className="bg-dark-900/30 border border-dark-800/50 rounded-xl p-4">
                          <p className="text-dark-500 text-xs leading-relaxed italic">{r.confidence_note}</p>
                        </div>
                      )}
                    </div>
                  )
                })()}
              </div>
            )}

            {/* Transcript & Assessment Viewer */}
            {videoView === 'transcripts' && (
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <button
                    onClick={() => setVideoView('results')}
                    className="text-dark-400 hover:text-white transition-colors cursor-pointer text-sm flex items-center gap-1"
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" /></svg>
                    Back to Results
                  </button>
                  <span className="text-dark-500 text-xs">{videoDetails.length} videos — {videoDetails.filter(d => d.transcript).length} with transcripts, {videoDetails.filter(d => d.assessment).length} assessed</span>
                </div>

                {videoDetails.map(detail => {
                  const expanded = videoDetailsExpanded.has(detail.video_id)
                  const hasT = !!detail.transcript
                  const hasA = !!detail.assessment
                  return (
                    <div key={detail.video_id} className="bg-dark-900/50 border border-dark-800 rounded-2xl overflow-hidden">
                      <button
                        onClick={() => setVideoDetailsExpanded(prev => {
                          const next = new Set(prev)
                          next.has(detail.video_id) ? next.delete(detail.video_id) : next.add(detail.video_id)
                          return next
                        })}
                        className="w-full px-5 py-3.5 flex items-center justify-between cursor-pointer hover:bg-dark-800/30 transition-colors"
                      >
                        <div className="flex items-center gap-3 min-w-0">
                          <span className="text-white text-sm font-medium truncate">{detail.title}</span>
                        </div>
                        <div className="flex items-center gap-2 shrink-0">
                          {hasA && <span className="text-[10px] px-1.5 py-0.5 rounded bg-primary-500/15 text-primary-400 border border-primary-500/20">Assessed</span>}
                          {hasT ? (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-emerald-500/15 text-emerald-400 border border-emerald-500/20">Transcript</span>
                          ) : (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-dark-700/50 text-dark-500 border border-dark-600">No transcript</span>
                          )}
                          <svg className={`w-4 h-4 text-dark-500 transition-transform ${expanded ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" /></svg>
                        </div>
                      </button>
                      {expanded && (
                        <div className="px-5 pb-4 space-y-4">
                          {hasA && detail.assessment && (
                            <div className="bg-primary-500/5 border border-primary-500/20 rounded-xl p-4 space-y-3">
                              <h4 className="text-primary-300 text-xs font-semibold uppercase tracking-wider">Assessment</h4>
                              <div className="grid grid-cols-3 gap-3">
                                {[
                                  { label: 'Keyword Alignment', val: detail.assessment.keyword_alignment },
                                  { label: 'Quotability', val: detail.assessment.quotability },
                                  { label: 'Info Density', val: detail.assessment.info_density },
                                ].map(m => (
                                  <div key={m.label}>
                                    <div className="flex justify-between text-xs mb-1">
                                      <span className="text-dark-400">{m.label}</span>
                                      <span className={`font-medium ${m.val >= 70 ? 'text-emerald-400' : m.val >= 40 ? 'text-amber-400' : 'text-red-400'}`}>{m.val}</span>
                                    </div>
                                    <div className="h-1.5 bg-dark-800 rounded-full overflow-hidden">
                                      <div className={`h-full rounded-full ${m.val >= 70 ? 'bg-emerald-500' : m.val >= 40 ? 'bg-amber-500' : 'bg-red-500'}`} style={{ width: `${m.val}%` }} />
                                    </div>
                                  </div>
                                ))}
                              </div>
                              <p className="text-dark-300 text-xs">{detail.assessment.summary}</p>
                              {detail.assessment.brand_sentiment !== 'none' && (
                                <p className="text-dark-400 text-xs">Brand sentiment: <span className={detail.assessment.brand_sentiment === 'positive' ? 'text-emerald-400' : detail.assessment.brand_sentiment === 'negative' ? 'text-red-400' : 'text-dark-300'}>{detail.assessment.brand_sentiment}</span></p>
                              )}
                              {detail.assessment.topics?.length > 0 && (
                                <div className="flex flex-wrap gap-1.5">
                                  {detail.assessment.topics.map((t, i) => (
                                    <span key={i} className="px-2 py-0.5 bg-dark-800 border border-dark-700 rounded text-dark-300 text-[10px]">{t}</span>
                                  ))}
                                </div>
                              )}
                              {detail.assessment.key_quotes?.length > 0 && (
                                <div className="space-y-1.5">
                                  <p className="text-dark-500 text-[10px] font-semibold uppercase tracking-wider">Key Quotes</p>
                                  {detail.assessment.key_quotes.map((q, i) => (
                                    <p key={i} className="text-dark-300 text-xs italic border-l-2 border-primary-500/30 pl-3">"{q}"</p>
                                  ))}
                                </div>
                              )}
                            </div>
                          )}
                          {hasT ? (
                            <div>
                              <h4 className="text-dark-500 text-xs font-semibold uppercase tracking-wider mb-2">Transcript ({(detail.transcript_length || detail.transcript.length).toLocaleString()} chars)</h4>
                              <pre className="text-dark-300 text-xs leading-relaxed whitespace-pre-wrap bg-dark-950 border border-dark-800 rounded-xl p-4 max-h-48 overflow-y-auto font-sans">
                                {detail.transcript}{detail.transcript_length > detail.transcript.length ? '…' : ''}
                              </pre>
                              {detail.transcript_length > detail.transcript.length && (
                                <p className="text-dark-600 text-[10px] mt-1 italic">Showing first {detail.transcript.length.toLocaleString()} of {detail.transcript_length.toLocaleString()} chars</p>
                              )}
                            </div>
                          ) : (
                            <p className="text-dark-500 text-xs italic">No transcript available for this video.</p>
                          )}
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        )}
      </main>

      {/* Delete video analysis confirmation modal */}
      {confirmDeleteVideoAnalysis && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-sm w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-2">Delete Video Analysis</h3>
            <p className="text-dark-400 text-sm mb-6">Permanently delete the video analysis for <span className="text-white font-medium">{videoDomain}</span>? This cannot be undone.</p>
            <div className="flex gap-3">
              <button
                onClick={() => setConfirmDeleteVideoAnalysis(false)}
                className="flex-1 px-4 py-2 text-sm text-dark-300 border border-dark-700 rounded-lg hover:bg-dark-800 transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={deleteVideoAnalysis}
                className="flex-1 px-4 py-2 text-sm text-white bg-red-600 rounded-lg hover:bg-red-500 transition-colors cursor-pointer font-medium"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete brand confirmation modal */}
      {confirmDeleteBrand && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-sm w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-2">Delete Brand Profile</h3>
            <p className="text-dark-400 text-sm mb-6">Permanently delete the brand profile for <span className="text-white font-medium">{brandDomain}</span>? This cannot be undone.</p>
            <div className="flex gap-3">
              <button
                onClick={() => setConfirmDeleteBrand(false)}
                className="flex-1 px-4 py-2 text-sm text-dark-300 border border-dark-700 rounded-lg hover:bg-dark-800 transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={async () => {
                  setConfirmDeleteBrand(false)
                  if (!brandDomain) return
                  try {
                    await apiFetch(`/api/brands/${encodeURIComponent(brandDomain)}`, { method: 'DELETE' })
                    setBrandEditing(false)
                    setBrandProfile(null)
                    fetchBrandList()
                  } catch { /* ignore */ }
                }}
                className="flex-1 px-4 py-2 text-sm text-white bg-red-600 rounded-lg hover:bg-red-500 transition-colors cursor-pointer font-medium"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete analysis confirmation modal */}
      {confirmDeleteAnalysis && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-sm w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-2">Delete Analysis</h3>
            <p className="text-dark-400 text-sm mb-6">This will permanently delete this analysis along with all associated optimization reports and to-do items.</p>
            <div className="flex gap-3">
              <button
                onClick={() => setConfirmDeleteAnalysis(null)}
                className="flex-1 px-4 py-2 text-sm text-dark-300 border border-dark-700 rounded-lg hover:bg-dark-800 transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={() => deleteAnalysis(confirmDeleteAnalysis)}
                className="flex-1 px-4 py-2 text-sm text-white bg-red-600 rounded-lg hover:bg-red-500 transition-colors cursor-pointer font-medium"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete optimization confirmation modal */}
      {confirmDeleteOptimization && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-sm w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-2">Delete Report</h3>
            <p className="text-dark-400 text-sm mb-6">This will permanently delete this optimization report and all associated to-do items.</p>
            <div className="flex gap-3">
              <button
                onClick={() => setConfirmDeleteOptimization(null)}
                className="flex-1 px-4 py-2 text-sm text-dark-300 border border-dark-700 rounded-lg hover:bg-dark-800 transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={() => deleteOptimization(confirmDeleteOptimization)}
                className="flex-1 px-4 py-2 text-sm text-white bg-red-600 rounded-lg hover:bg-red-500 transition-colors cursor-pointer font-medium"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Unsaved brand changes modal */}
      {pendingNavAction && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-sm w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-2">Unsaved Changes</h3>
            <p className="text-dark-400 text-sm mb-6">You have unsaved changes to this brand profile. What would you like to do?</p>
            <div className="flex gap-3">
              <button
                onClick={() => {
                  brandDirtyRef.current = false
                  const action = pendingNavAction
                  setPendingNavAction(null)
                  action()
                }}
                className="flex-1 px-4 py-2 text-sm text-red-400 border border-red-500/30 rounded-lg hover:bg-red-500/10 transition-colors cursor-pointer"
              >
                Discard
              </button>
              <button
                onClick={() => setPendingNavAction(null)}
                className="flex-1 px-4 py-2 text-sm text-dark-300 border border-dark-700 rounded-lg hover:bg-dark-800 transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={async () => {
                  const action = pendingNavAction
                  setPendingNavAction(null)
                  await saveBrandProfile()
                  action()
                }}
                className="flex-1 px-4 py-2 text-sm text-white bg-primary-600 rounded-lg hover:bg-primary-500 transition-colors cursor-pointer font-medium"
              >
                Save
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Share Domain Modal */}
      {shareModalDomain && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md w-full mx-4 shadow-2xl">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-white font-semibold text-lg">Share Domain</h3>
              <button
                onClick={() => setShareModalDomain(null)}
                className="text-dark-400 hover:text-white transition-colors cursor-pointer"
              >
                <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
              </button>
            </div>
            <p className="text-dark-400 text-sm mb-4">
              Control who can view reports for <span className="text-white font-medium">{shareModalDomain}</span>.
              All reports (analysis, optimization, video, todos) are shared collectively.
            </p>

            {shareLoading && !domainShareState ? (
              <div className="text-center py-6 text-dark-400">Loading...</div>
            ) : (
              <div className="space-y-3">
                {/* Private option */}
                <button
                  onClick={() => setDomainVisibility(shareModalDomain, 'private')}
                  disabled={shareLoading}
                  className={`w-full text-left p-3 rounded-xl border transition-all cursor-pointer ${
                    domainShareState?.visibility === 'private'
                      ? 'bg-dark-800 border-primary-500/50'
                      : 'bg-dark-900/50 border-dark-700 hover:border-dark-600'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <div className={`w-4 h-4 rounded-full border-2 flex items-center justify-center ${
                      domainShareState?.visibility === 'private' ? 'border-primary-500' : 'border-dark-600'
                    }`}>
                      {domainShareState?.visibility === 'private' && <div className="w-2 h-2 rounded-full bg-primary-500" />}
                    </div>
                    <div>
                      <p className="text-white text-sm font-medium">Private</p>
                      <p className="text-dark-500 text-xs">Only team members can view these reports</p>
                    </div>
                  </div>
                </button>

                {/* Public option */}
                <button
                  onClick={() => setDomainVisibility(shareModalDomain, 'public')}
                  disabled={shareLoading}
                  className={`w-full text-left p-3 rounded-xl border transition-all cursor-pointer ${
                    domainShareState?.visibility === 'public'
                      ? 'bg-dark-800 border-primary-500/50'
                      : 'bg-dark-900/50 border-dark-700 hover:border-dark-600'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <div className={`w-4 h-4 rounded-full border-2 flex items-center justify-center ${
                      domainShareState?.visibility === 'public' ? 'border-primary-500' : 'border-dark-600'
                    }`}>
                      {domainShareState?.visibility === 'public' && <div className="w-2 h-2 rounded-full bg-primary-500" />}
                    </div>
                    <div>
                      <p className="text-white text-sm font-medium">Public</p>
                      <p className="text-dark-500 text-xs">Anyone with the link can view these reports</p>
                    </div>
                  </div>
                </button>

                {/* Popular option (root tenant admin/owner only) */}
                {user?.isRootTenant && (user.role === 'owner' || user.role === 'admin') && (
                  <button
                    onClick={() => setDomainVisibility(shareModalDomain, 'popular')}
                    disabled={shareLoading}
                    className={`w-full text-left p-3 rounded-xl border transition-all cursor-pointer ${
                      domainShareState?.visibility === 'popular'
                        ? 'bg-dark-800 border-accent-purple/50'
                        : 'bg-dark-900/50 border-dark-700 hover:border-dark-600'
                    }`}
                  >
                    <div className="flex items-center gap-3">
                      <div className={`w-4 h-4 rounded-full border-2 flex items-center justify-center ${
                        domainShareState?.visibility === 'popular' ? 'border-accent-purple' : 'border-dark-600'
                      }`}>
                        {domainShareState?.visibility === 'popular' && <div className="w-2 h-2 rounded-full bg-accent-purple" />}
                      </div>
                      <div>
                        <p className="text-white text-sm font-medium">Popular</p>
                        <p className="text-dark-500 text-xs">Featured on the Analyze page + publicly accessible</p>
                      </div>
                    </div>
                  </button>
                )}

                {/* Share URL display */}
                {domainShareState?.share_url && (domainShareState.visibility === 'public' || domainShareState.visibility === 'popular') && (() => {
                  const shareFullUrl = `${window.location.origin}${domainShareState.share_url}?tab=${activeTab}`
                  return (
                  <div className="mt-4 p-3 bg-dark-800 rounded-xl border border-dark-700">
                    <label className="text-dark-500 text-xs font-medium block mb-2">Share Link</label>
                    <div className="flex items-center gap-2">
                      <input
                        type="text"
                        readOnly
                        value={shareFullUrl}
                        className="flex-1 bg-dark-900 border border-dark-700 rounded-lg px-3 py-2 text-sm text-dark-300 font-mono"
                      />
                      <button
                        onClick={() => {
                          navigator.clipboard.writeText(shareFullUrl)
                          setShareCopied(true)
                          setTimeout(() => setShareCopied(false), 2000)
                        }}
                        className="px-3 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-500 transition-colors cursor-pointer font-medium shrink-0"
                      >
                        {shareCopied ? 'Copied!' : 'Copy'}
                      </button>
                    </div>
                  </div>
                  )
                })()}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Optimize Confirmation Modal — shown when clicking a question with no existing report */}
      {optimizeConfirmQ !== null && result && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-3">Generate Optimization Report</h3>
            <p className="text-dark-400 text-sm mb-6">
              Do you want to generate an Optimize report to help surface this question to LLMs at <span className="text-white font-medium">{brandList.find(b => domainKey(b.domain) === domainKey(selectedDomain))?.brand_name || selectedDomain}</span> better?
            </p>
            <p className="text-dark-500 text-xs mb-6 italic">
              &ldquo;{result.questions[optimizeConfirmQ]?.question}&rdquo;
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setOptimizeConfirmQ(null)}
                className="px-4 py-2 text-dark-400 hover:text-white text-sm transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={() => { const qi = optimizeConfirmQ; setOptimizeConfirmQ(null); optimizeQuestion(qi) }}
                className="px-4 py-2 bg-gradient-to-r from-primary-600 to-primary-500 text-white text-sm font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all cursor-pointer"
              >
                Yes, Generate
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ReadOnly "Not Ready" Modal — shown when clicking question with no optimize report in shared view */}
      {readOnlyOptModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-3">Report Not Available</h3>
            <p className="text-dark-400 text-sm mb-6">
              An Optimize report for this question hasn't been generated yet. Would you like to see other optimization reports for <span className="text-white font-medium">{readOnlyOptModal}</span>?
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setReadOnlyOptModal(null)}
                className="px-4 py-2 text-dark-400 hover:text-white text-sm transition-colors cursor-pointer"
              >
                No, Thanks
              </button>
              <button
                onClick={() => { setReadOnlyOptModal(null); setActiveTab('optimize') }}
                className="px-4 py-2 bg-gradient-to-r from-primary-600 to-primary-500 text-white text-sm font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all cursor-pointer"
              >
                View Reports
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Login Required Modal */}
      {loginModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-3">Sign In Required</h3>
            <p className="text-dark-400 text-sm mb-6">
              You need to be signed in to generate new analyses. You can browse the popular domains below for free without an account.
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setLoginModal(false)}
                className="px-4 py-2 text-dark-400 hover:text-white text-sm transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={() => { setLoginModal(false); window.location.href = '/login' }}
                className="px-4 py-2 bg-gradient-to-r from-primary-600 to-primary-500 text-white text-sm font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all cursor-pointer"
              >
                Sign In / Sign Up
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Subscription Required Modal */}
      {subscriptionModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-3">Subscription Required</h3>
            <p className="text-dark-400 text-sm mb-4">
              You need an active subscription to run reports. You can still view popular public reports shared by other users.
            </p>
            <p className="text-dark-400 text-sm mb-6">
              Would you like to choose a plan?
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setSubscriptionModal(false)}
                className="px-4 py-2 text-dark-400 hover:text-white text-sm transition-colors cursor-pointer"
              >
                Not Now
              </button>
              <button
                onClick={() => { setSubscriptionModal(false); window.location.href = '/last/plan' }}
                className="px-4 py-2 bg-gradient-to-r from-primary-600 to-primary-500 text-white text-sm font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all cursor-pointer"
              >
                View Plans
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
