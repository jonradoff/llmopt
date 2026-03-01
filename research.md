# LLM Optimization Research

Research findings informing how LLM Optimizer assesses whether a website's content will be surfaced and cited by AI search engines and language models.

## Source Papers

| Paper | Year | Key Contribution |
|-------|------|------------------|
| [Lost in the Middle](https://arxiv.org/abs/2307.03172) | 2024 | Position bias in LLM context windows |
| [GEO: Generative Engine Optimization](https://arxiv.org/abs/2311.09735) | 2024 | Content optimization methods for LLM citation |
| [NanoKnow](https://arxiv.org/abs/2602.20122) | 2026 | Training data frequency → model knowledge |
| [GEO: How to Dominate AI Search](https://arxiv.org/abs/2509.08919) | 2025 | Source type preferences in AI search engines |
| [YouTube vs Reddit AI Citations](https://www.adweek.com/media/youtube-reddit-ai-search-engine-citations) | 2025 | YouTube #1 social citation source; 16% of LLM answers |
| [News Source Citing Patterns in AI Search](https://arxiv.org/abs/2507.05301) | 2025 | Citation concentration + gatekeeping across 366K citations |
| [LiveCC: Video LLM Training from ASR](https://arxiv.org/abs/2504.16030) | 2025 | How video LLMs train on transcripts — what actually gets extracted |
| [The False Promise of Source-Cited Responses](https://arxiv.org/abs/2410.22349) | 2024 | Citation accuracy (49-68%), hallucination rates in answer engines |

---

## 1. Position Bias (Lost in the Middle)

LLMs exhibit a U-shaped attention curve over their input context — content at the beginning and end is reliably used, while content in the middle is effectively ignored.

**Key metrics:**
- GPT-3.5-Turbo with answer in middle of 20-30 docs: accuracy **below closed-book** (worse than no documents at all)
- Performance consistently declines as document count increases from 10 to 30
- Extended context windows (16K, 100K) do not fix this — the U-shaped pattern persists
- Encoder-decoder models (bidirectional) show more robustness within training-length windows

**Implications for content optimization:**
- Front-load key information in pages — opening paragraphs matter most for retrieval
- Concise, focused pages outperform comprehensive but sprawling ones
- In RAG systems, retrieval rank matters more than retrieval volume — top 10-20 results plateau quickly

---

## 2. Content Optimization Methods (GEO, Princeton/KDD 2024)

The first GEO paper tested 9 optimization strategies on a benchmark of 10,000 queries. Results measured via Position-Adjusted Word Count (visibility weighted by citation position):

| Method | Improvement | Notes |
|--------|-------------|-------|
| Quotation Addition | **+41%** | Best single method — embed quotes from authoritative sources |
| Statistics Addition | **+33%** | Concrete numbers make content more citable |
| Fluency Optimization | +29% | Clean, well-written prose preferred |
| Cite Sources | +28% | Content that itself cites other sources is treated as more authoritative |
| Technical Terms | +19% | Domain-specific vocabulary signals expertise |
| Easy-to-Understand | +14% | Accessible language helps |
| Authoritative Tone | +12% | Confident, expert voice |
| Unique Words | +6% | Minimal impact |
| Keyword Stuffing | **-9%** | Actively harmful in generative engines |

**Combinations:** Fluency + Statistics outperformed any single method by 5.5%+.

**Democratization effect:** Lower-ranked sites benefit disproportionately. Rank-5 sites saw +115% visibility improvement from Cite Sources optimization, while rank-1 sites saw -30%.

**Domain-specific effectiveness:**
- Authoritative tone → debate, history, science
- Statistics → law & government, debate, opinion
- Quotations → people & society, explanation, history
- Cite Sources → factual statements, law & government
- Fluency → business, science, health

**Validated on Perplexity.ai** — Quotations: +22% position-adjusted, +30% subjective impression.

---

## 3. Training Data Frequency (NanoKnow)

Using fully open pre-training data (FineWeb-Edu, 100B tokens), this paper quantified the relationship between answer frequency in training data and model knowledge.

**Key findings:**
- Accuracy **more than doubles** from rare (1-5 documents) to high-frequency (51+ documents) answers
- ~66-71% of benchmark questions have answers present in training data
- Even with oracle RAG context provided, models score **~11 percentage points higher** on questions whose answers appeared in training data (parametric + retrieval = compounding advantage)
- Each additional irrelevant document costs ~4-6 LLM-Judge percentage points
- Non-relevant context is **worse than no context** (distractor-only < closed-book)

**Implications:**
- Information widely repeated across high-quality educational sources is more likely encoded in model weights
- Being in training data AND being retrievable at inference provides a compounding advantage
- Content quality of surrounding context matters — clearly-written educational content outperforms naturally-occurring text by 19+ percentage points
- Minimum model size for meaningful memorization: ~1.9B parameters

---

## 4. AI Search Source Preferences (GEO, Toronto 2025)

Empirical analysis of what types of sources AI search engines (ChatGPT, Perplexity, Gemini, Claude) actually cite.

**The earned media bias (central finding):**

| Source Type | Google | AI Search |
|-------------|--------|-----------|
| Earned media (reviews, independent analysis) | 45-54% | **72-92%** |
| Brand-owned content | 33-44% | 18-27% |
| Social (Reddit, YouTube, forums) | 10-15% | **~0%** |

**Additional findings:**
- AI citations overlap with Google results only 15-50% — they cite fundamentally different sources
- Claude shows highest cross-language citation stability; ChatGPT switches entire site ecosystems by language
- AI citations are more robust to query paraphrasing than Google results
- When Google shows AI summaries, link clicks fell to 8% (from 15%); 26% of searches ended with zero clicks

**Four GEO imperatives proposed:**
1. Prioritize earned media — third-party reviews, analyst coverage, independent mentions
2. Structure content for machine extraction — Schema.org, comparison tables, explicit justification
3. Develop language-specific authority strategies
4. Create lifecycle content (awareness → consideration → decision → post-purchase)

---

## 5. Synthesis: Answer Optimization Scoring Framework

Based on the research above, we can assess how likely an LLM is to surface and cite a website's answer to a given question across several dimensions:

### Dimension 1: Content Authority Signals
Derived from GEO (Princeton) optimization research:
- **Quotation density** — Does the page embed quotes from authoritative sources?
- **Statistical evidence** — Does it include specific data points and numbers?
- **Source citations** — Does the content itself cite external references?
- **Fluency quality** — Is the writing clean and well-structured?
- **Technical precision** — Does it use appropriate domain-specific terminology?
- **Anti-patterns** — Keyword stuffing, marketing fluff, vague claims without evidence

### Dimension 2: Structural Optimization
Derived from Lost in the Middle + GEO (Toronto):
- **Answer prominence** — Is the answer front-loaded or buried deep in the page?
- **Content conciseness** — Focused, direct answers vs. sprawling, padded content
- **Machine readability** — Schema.org markup, clean HTML structure, comparison tables
- **Justification language** — Does content explain "why" not just "what"?

### Dimension 3: Source Authority
Derived from GEO (Toronto) source preference analysis:
- **Earned media coverage** — Is the site mentioned by independent third-party sources?
- **Source type classification** — Brand-owned, earned media, or social content?
- **Domain authority in AI search** — Known authoritative domains for the topic area
- **Cross-engine consistency** — Would the site be cited across multiple AI engines?

### Dimension 4: Knowledge Persistence
Derived from NanoKnow training data analysis:
- **Information frequency** — How widely does this answer appear across the web?
- **Educational quality** — Is the content written in a clear, didactic style?
- **Retrieval complementarity** — Is the content both likely in training data AND easily retrievable?
- **Surrounding context quality** — Does the page provide clear, self-contained answer passages?

### Scoring Model

Each dimension contributes to an overall **LLM Visibility Score** (0-100):

| Dimension | Weight | Rationale |
|-----------|--------|-----------|
| Content Authority Signals | 30% | Strongest direct evidence from GEO (+41% from quotations alone) |
| Structural Optimization | 20% | Position and structure gate whether content is used at all |
| Source Authority | 30% | AI search shows 72-92% earned media preference |
| Knowledge Persistence | 20% | Training data frequency doubles accuracy; compounding with RAG |

Sub-scores within each dimension are assessed on content analysis of the specific page(s) answering the question, combined with web search for third-party coverage signals.

---

## 6. Video Authority in LLM Search

YouTube has emerged as the dominant social citation source in AI search. Video content shapes what LLMs "know" about brands — not through visual understanding (LLMs can't watch video) but through transcripts, titles, descriptions, and metadata that get ingested into training data and retrieval systems.

### Source Papers

| Paper / Report | Year | Key Contribution |
|----------------|------|------------------|
| [YouTube vs Reddit: AI Citation Analysis](https://www.adweek.com/media/youtube-reddit-ai-search-engine-citations) (Adweek, citing Bluefish / Emberos / Goodie AI) | 2025 | Empirical citation share data across AI search engines |
| [News Source Citing Patterns in AI Search Systems](https://arxiv.org/abs/2507.05301) | 2025 | Citation concentration, gatekeeping dynamics, provider divergence across 366K citations |
| [LiveCC: Learning Video LLM with Streaming Speech Transcription](https://arxiv.org/abs/2504.16030) | 2025 | How video LLMs are trained from ASR transcripts — reveals what actually gets extracted |
| [The False Promise of Factual and Verifiable Source-Cited Responses](https://arxiv.org/abs/2410.22349) | 2024 | Citation accuracy, hallucination rates, and misattribution in answer engines |

---

### 6.1 YouTube's Citation Dominance

**YouTube is now the #1 social citation source for LLMs:**
- YouTube appears in **16% of LLM answers** vs. 10% for Reddit (Bluefish, 2025)
- YouTube is cited **40% more frequently** than Reddit across ChatGPT, Gemini, and Perplexity (Emberos)
- YouTube's social citation share **doubled** from 18.9% (Aug 2024) to **39.2%** (Dec 2024); Reddit fell from 44.2% to 20.3% (Goodie AI, 6.1M citations across 66 brands)
- YouTube generates **18x more citations** than Instagram, **50x more** than TikTok, **500x more** than Vimeo (Profound)

**Platform-specific rates:**
- Google AI Overviews cite YouTube in 18-25% of eligible answers
- ChatGPT cites YouTube in ~18% of answers
- Social citations overall represent ~7% of all LLM citations, but growing rapidly

**Critical insight:** "AI visibility is earned differently than human attention. Views, followers and creator influence don't reliably translate into AI influence." Human engagement metrics (subscriber count, view count) do not predict AI searchability — structural factors like indexable transcripts matter most.

---

### 6.2 What LLMs Actually Extract from Video

The LiveCC paper (CVPR 2025) reveals how video LLMs are trained, which tells us exactly what signals matter:

- Video LLMs are trained on **ASR transcripts** (automatic speech recognition), not visual frames for text understanding
- The training approach **densely interleaves ASR words and video frames** according to timestamps — meaning transcript quality directly determines what the model learns
- A 7B-parameter model trained on YouTube transcripts **surpassed 72B models** (Qwen2.5-VL-72B, LLaVA-Video-72B) in commentary quality — proving transcript quality > model size
- Training datasets: **Live-CC-5M** (5M YouTube videos with closed captions) and **Live-WhisperX-526K** (high-quality transcriptions)
- YouTube's auto-generated captions are the primary training data pipeline — if a video has no captions, it effectively doesn't exist to these models

**Implications for video optimization:**
- Transcripts are the #1 signal — clear, keyword-rich spoken content matters more than production quality
- Videos with human-edited captions/subtitles provide cleaner training signal than auto-generated ones
- Timestamp-aligned transcripts give models fine-grained understanding of what was said when
- A well-structured 10-minute explainer with clear transcripts > a viral 60-second clip with no captions

---

### 6.3 Citation Concentration and Gatekeeping

The AI Search Arena study (366K citations, 24K conversations across OpenAI, Perplexity, Google) reveals extreme citation concentration that applies equally to video:

**Winner-take-all dynamics:**
- OpenAI: top 20 news sources capture **67.3%** of all citations (Gini = 0.83)
- Google: top 20 capture **31.9%** (Gini = 0.69)
- Perplexity: top 20 capture **28.5%** (Gini = 0.77)

**Provider divergence:**
- Different AI providers cite substantially different sources (cross-family cosine similarity: 0.11-0.58)
- Intra-family similarity is high (0.82-0.99) — models from the same provider behave similarly
- OpenAI heavily favors Reuters (22.8%) and AP News (12.2%); Google favors Indian media; Perplexity favors BBC

**Quality filtering:**
- OpenAI cites high-quality sources 96.2% of the time; Google 92.2%; Perplexity 89.7%
- Low-credibility sources are rarely cited across all providers

**For video:** This means a small number of authoritative YouTube channels likely capture a disproportionate share of LLM citations. Getting mentioned by those high-authority channels provides outsized visibility.

---

### 6.4 Citation Accuracy and the Hallucination Problem

The answer engine evaluation study (You.com, BingChat, Perplexity) reveals serious reliability issues in how AI search engines use their sources:

**Citation accuracy is poor:**
- Perplexity: only **49%** citation accuracy (half its citations don't actually support the claim)
- You.com: 68.3% accuracy
- BingChat: 65.8% accuracy

**Thoroughness is uniformly bad:**
- All systems: only **20-24%** citation thoroughness (citations cover only ~1/4 of possible source-statement relationships)

**Unsupported claims are common:**
- 23-32% of relevant statements have **no source backing at all**
- Systems display more sources than they actually cite (Perplexity uses only 51% of displayed sources)

**Overconfidence compounds the problem:**
- Perplexity generates one-sided answers in **83.4%** of debate queries, with **81.6%** simultaneously overconfident
- Users verify sources less when answers align with existing beliefs (1.08 sources hovered vs. 2.95 for opposing views)
- Users interact with answer engine sources far less than search results (2 sources hovered vs. 11.8 for Google Search)

**For video analysis interpretation:**
- When LLMs cite video content, the citation may not accurately reflect what the video actually says
- Brand sentiment in LLM outputs may be a distorted version of actual video sentiment
- Videos that are clear, structured, and unambiguous in their claims are less likely to be misrepresented
- The "extractability" dimension in Level 2 analysis directly addresses this — measuring how clearly a video's content can be accurately cited

---

### 6.5 Implications for Video Report Interpretation

These findings reshape how we should interpret each analysis level:

**Level 1 (Channel Health) — informed by LiveCC + GEO:**
- **Transcript quality is the dominant signal.** A video's LLM influence is almost entirely determined by its transcript, not its visual content or production value.
- **Keyword alignment in transcripts** maps directly to how video LLMs are trained (ASR → model weights). If target keywords aren't spoken clearly in the video, the model won't learn the association.
- **Structural clarity** (clear topic sentences, explicit entity naming, quotable statements) mirrors the GEO finding that quotation-ready content gets +41% visibility.
- **Caption availability** is binary and critical — no captions = invisible to LLMs.

**Level 2 (Brand Perception) — informed by citation accuracy research + concentration data:**
- **Sentiment classification needs a confidence discount.** Given that 23-32% of answer engine claims are unsupported and citation accuracy ranges 49-68%, what LLMs "believe" about a brand from video may be a distorted signal.
- **Mention position matters more than mention frequency.** Per Lost in the Middle, content at the beginning of a video transcript gets disproportionate attention in LLM context windows.
- **Extractability is the key differentiator.** Videos where brand mentions are clear, contextual, and unambiguous will be cited more accurately than passing references.
- **High-authority creators dominate.** The winner-take-all citation pattern (Gini 0.69-0.83) means a mention by a top-authority channel in a niche is worth more than dozens of small-channel mentions.

**Level 3 (Competitive Landscape) — informed by provider divergence + gatekeeping:**
- **Share of voice varies by AI platform.** Different providers cite different sources (cross-family similarity only 0.11-0.58), so a brand's video share of voice in ChatGPT may differ substantially from Perplexity or Gemini.
- **Content gaps represent real opportunity.** The concentration data shows that being the first authoritative video voice in a gap query could capture disproportionate citation share.
- **Creator targeting should prioritize channels already cited by AI systems**, not just channels with high view counts. Human engagement metrics don't predict AI citation (Adweek finding).

---

### 6.6 Implications for Prompt Architecture

The research suggests several refinements to how we prompt Claude for video analysis:

**Level 1 prompt refinements:**
- Weight transcript analysis above all other signals — it's what LLMs actually ingest
- Score caption availability/quality as a binary gate (no captions = score cap)
- Assess "quotability" — does the transcript contain clear, standalone statements that could be extracted and cited?
- Check for entity clarity: is the brand name spoken explicitly or only implied/shown visually?

**Level 2 prompt refinements:**
- Include a confidence discount in sentiment assessment — note that LLM citation of video content has ~50-68% accuracy
- Weight mention position within transcript (first 20% and last 20% matter most, per U-shaped attention)
- Assess extractability explicitly: could an LLM accurately represent what this video says about the brand?
- Flag cases where brand mentions are ambiguous or easily misattributed

**Level 3 prompt refinements:**
- Note that share-of-voice may differ across AI platforms — recommend checking multiple engines
- Prioritize content gap identification where no authoritative video exists (winner-take-all means first-mover advantage is large)
- Score creator targets by structural authority (transcript quality, caption availability, topical consistency) rather than human engagement metrics
- Include the "AI visibility ≠ human attention" principle as an explicit instruction

---

## 7. Implementation Guidance: Answer Optimization Feature

The Answer Optimization feature should analyze a specific question-answer pair from a website analysis and assess how likely an LLM is to surface and cite that answer.

### What to Analyze
For a given question + page URL(s) from the site analysis:
1. Fetch and analyze the actual page content for authority signals (quotes, stats, citations, fluency, structure)
2. Search for third-party coverage of the site for this topic area (earned media signals)
3. Assess how well the content is structured for machine extraction
4. Evaluate the answer's prominence on the page (front-loaded vs. buried)
5. Check how widely the answer appears across the web (frequency/persistence)

### LLM Prompt Strategy
Use Claude with web_search to:
- Visit the specific page(s) and analyze content characteristics
- Search for third-party mentions and reviews of the site in this topic area
- Search for competing answers to the same question from other sources
- Assess relative authority compared to competing sources

### Output Structure
- Overall LLM Visibility Score (0-100)
- Per-dimension scores with specific evidence
- Actionable recommendations for improving each dimension
- Competitive landscape — who else answers this question and how they compare
- Priority ranking of improvement actions by expected impact

### Caching Strategy
- Cache optimization results linked to the specific analysis + question index
- Same 30-day expiry as site analyses
- Force re-analysis option available

---

## 8. Search Visibility and AI Crawl Accessibility

Traditional SEO is fragmenting into two distinct optimization pathways: Google AI Overviews (which still favor organically top-ranked pages) and standalone LLMs (which operate on fundamentally different signals). Understanding this split — and the role of crawl accessibility, content freshness, and brand search momentum — is critical for comprehensive LLM visibility assessment.

### Source Papers

| Paper / Report | Year | Key Contribution |
|----------------|------|------------------|
| [How LLMs & AI Agents Get Info From Your Site](https://ahrefs.com/blog/llms-and-ai-agents/) (Ahrefs, 1.9M citations) | 2025 | AI Overview citation patterns — 76% from Google's top-10 organic results |
| [Where Do AI Search Engines Get Results?](https://ahrefs.com/blog/ai-search-traffic-study/) (Ahrefs, 17M citations) | 2025 | The two-world split — only 12% overlap between Google top-10 and ChatGPT/Perplexity citations |
| [AI Brand Visibility Study: 75K Brands](https://ahrefs.com/blog/ai-brand-visibility-study/) (Ahrefs) | 2025 | Brand search volume, web mentions, and YouTube as strongest predictors of AI citation |
| [Content Freshness and AI Answers](https://ahrefs.com/blog/freshness-seo/) (Ahrefs) | 2025 | AI assistants cite content 25.7% newer on average than traditional search |
| [The Impact of Content Recency on AI Visibility](https://www.seerinteractive.com/insights/content-recency-ai-overview) (Seer Interactive) | 2025 | Freshness signals shift LLM ranking by up to 95 positions |
| [Recency Bias in AI: LLM Performance on Outdated Facts](https://arxiv.org/abs/2504.01248) | 2025 | Systematic recency bias in LLM knowledge — newer facts preferred even when wrong |
| [2025 AI Bot & Crawler Report](https://blog.cloudflare.com/ai-crawlers-update-2025) (Cloudflare) | 2025 | GPTBot traffic grew 305% YoY; 3-bot architecture per AI provider; robots.txt as visibility lever |
| [AIO Study: 200K Keywords](https://www.semrush.com/blog/ai-overviews/) (Semrush) | 2025 | Only 12.47% of keywords trigger AI Overviews; 73% of AIO sources are in organic top-10 |

---

### 8.1 The Two-World Split: AI Overviews vs. Standalone LLMs

The most critical finding for search visibility assessment is that **Google AI Overviews and standalone LLMs cite fundamentally different sources.**

**Google AI Overviews (AIO):**
- **76% of AIO citations** come from pages already in Google's top-10 organic results (Ahrefs, 1.9M citations)
- 63% of AIO sources have **Domain Rating (DR) 70+** — established, authoritative domains dominate
- Only **12.47% of keywords** trigger AI Overviews at all (Semrush, 200K keywords)
- AIO strongly favors informational queries, how-to content, and educational material
- Organic SEO ranking is effectively a **prerequisite** for AIO citation

**Standalone LLMs (ChatGPT, Perplexity, Claude):**
- Only **12% overlap** between Google's top-10 organic results and ChatGPT/Perplexity citations (Ahrefs, 17M citations)
- LLMs cite from a much wider pool of sources, weighted by training data presence and earned media signals
- Brand authority and web mention frequency matter more than traditional link-based authority
- Content structure (Schema.org, tables, clear definitions) plays a larger role than backlink profiles

**Implication:** A site can rank #1 in Google and still be invisible to ChatGPT — and vice versa. Comprehensive search visibility assessment must evaluate both pathways independently.

---

### 8.2 AI Crawl Accessibility

The Cloudflare 2025 crawler report reveals that AI companies now operate a **3-bot architecture**, each requiring separate robots.txt consideration:

| Bot Type | Purpose | Examples |
|----------|---------|---------|
| Training bots | Scrape content for model training | GPTBot, Google-Extended, ClaudeBot, Applebot-Extended |
| Indexing bots | Build real-time retrieval indexes | ChatGPT-User, PerplexityBot, Google-Extended |
| User-fetch bots | Fetch pages live during conversations | ChatGPT-User, PerplexityBot |

**Key metrics:**
- **GPTBot traffic grew 305% YoY** — AI crawlers are now a major share of web traffic
- **ClaudeBot and Bytespider grew 246% and 76%** respectively
- Approximately **30-40% of high-traffic sites** now block at least one AI crawler
- Blocking training bots prevents future knowledge acquisition; blocking user-fetch bots prevents real-time citation
- Sites that block user-fetch bots (e.g., ChatGPT-User) are **invisible in real-time AI conversations** regardless of training data presence

**robots.txt is now a direct AI visibility lever:**
- `Disallow: /` for GPTBot = excluded from OpenAI's training AND real-time fetch
- Selective blocking is possible (e.g., allow indexing bots, block training bots)
- Many sites have overly broad blocks that unintentionally exclude themselves from AI search
- Sitemap presence and quality affect how efficiently AI crawlers can index content

---

### 8.3 Brand Search Momentum

The Ahrefs 75K brands study quantified which signals predict whether an AI assistant will mention a brand:

| Signal | Correlation with AI Citation | Notes |
|--------|------------------------------|-------|
| YouTube mentions | **0.737** | Strongest single predictor |
| Web mentions (earned media) | **0.664** | Second strongest — independent coverage |
| Brand search volume | **0.334** | Google Trends searches for the brand name |
| Domain authority / backlinks | 0.12-0.18 | Surprisingly weak predictor |
| Social media followers | <0.1 | Nearly irrelevant |

**Key findings:**
- Brand search volume is a **proxy for real-world prominence** — brands that people actively search for are more likely to appear in AI answers
- Web mentions (being talked about by others) are **3.7x more predictive** than traditional domain authority
- YouTube mentions are the **strongest single signal** (0.737 correlation) — this connects to our Video Authority findings in Section 6
- Traditional SEO metrics (backlinks, DA) are **poor predictors** of AI citation — the signal landscape has shifted

**Entity recognition** also matters: brands that Google recognizes as named entities (appearing in Knowledge Panels) receive disproportionate AI citation, suggesting that entity databases (Wikidata, Google Knowledge Graph) influence LLM knowledge.

---

### 8.4 Content Freshness Bias

Multiple studies confirm that AI systems systematically prefer newer content:

**Ahrefs (17M citations study):**
- AI assistants cite content that is **25.7% newer** on average than content cited in traditional Google results
- The median age of AI-cited content is **~14 months** vs. **~18 months** for Google organic results
- For rapidly evolving topics (technology, health, policy), the freshness bias is even more extreme

**Seer Interactive:**
- Content freshness signals can shift an article's **position by up to 95 ranks** in AI search results
- Pages updated within the last 6 months receive **significantly higher** AIO citation rates than stale content
- The effect is strongest for queries with informational intent

**arXiv — LLM Recency Bias (2025):**
- LLMs show systematic recency bias: they prefer information from more recent sources even when older sources contain more accurate answers
- This bias is **inherent to training data distributions** — more recent web pages are overrepresented in training corpora
- The bias compounds in RAG systems: retrieval algorithms also favor recent documents, amplifying the preference

**Implications for optimization:**
- Regular content updates provide an ongoing visibility advantage in AI search
- "Content decay" is real — pages that haven't been updated in 12+ months lose AI visibility even if the information remains accurate
- Publication and modification dates should be explicitly marked in structured data (Schema.org `dateModified`)
- Evergreen content strategies must include regular refresh cycles to maintain AI visibility

---

### 8.5 Search Visibility Scoring Framework

Based on the research above, we assess a website's search visibility for AI search engines across four dimensions:

#### Pillar 1: AI Overview Readiness (30%)
How well-positioned is the site to appear in Google AI Overviews?

- **Organic presence** — Does the site rank in Google's top-10 for relevant queries? (76% of AIO sources come from top-10)
- **Structured data** — Schema.org markup, FAQ schema, HowTo schema that AIO can extract from
- **Content format** — Clear, concise answers to common questions; comparison tables; step-by-step formats
- **Answer prominence** — Key answers front-loaded in content; not buried in long-form prose

#### Pillar 2: Crawl Accessibility (20%)
Can AI systems actually access and index the site's content?

- **robots.txt policy** — Which AI bots are allowed vs. blocked? Overly broad blocks?
- **AI bot access** — Are training bots, indexing bots, and user-fetch bots specifically addressed?
- **Sitemap quality** — Is there a valid, comprehensive sitemap? Is it submitted to search engines?
- **Render accessibility** — Can content be accessed without JavaScript rendering? Are there bot-specific barriers?

#### Pillar 3: Brand Search Momentum (25%)
How visible is the brand in the broader search and mention landscape?

- **Brand search trend** — Is brand search volume growing, stable, or declining? (Google Trends signal)
- **Competitor comparison** — How does brand search volume compare to key competitors?
- **Web mention strength** — Is the brand mentioned by independent third-party sources? (0.664 correlation with AI citation)
- **Entity recognition** — Does Google recognize the brand as a named entity? (Knowledge Panel presence)

#### Pillar 4: Content Freshness (25%)
How current is the site's content, and is it being maintained?

- **Average content age** — How old is the typical page on the site? (AI prefers content 25.7% newer)
- **Update frequency** — How often is content being refreshed or added?
- **Freshness signals** — Are `dateModified` and `datePublished` structured data present and accurate?
- **Content decay risk** — What percentage of content hasn't been updated in 12+ months?

#### Scoring

| Pillar | Weight | Rationale |
|--------|--------|-----------|
| AI Overview Readiness | 30% | AIO is the primary discovery channel; 76% organic top-10 correlation |
| Crawl Accessibility | 20% | Binary gate — if bots can't crawl, nothing else matters |
| Brand Search Momentum | 25% | Web mentions (0.664) and brand search (0.334) are strong AI citation predictors |
| Content Freshness | 25% | 25.7% newer content preference; up to 95-position ranking shifts |

### Implementation Notes

The Search Visibility analysis uses Claude with `web_search` to check:
- The site's robots.txt file for AI crawler policies
- Google search results for organic ranking signals
- Third-party mentions and earned media coverage
- Content publication dates and update patterns
- Structured data and Schema.org markup presence
- Sitemap availability and quality
- Brand search trends and competitor comparison

Results are cached for 30 days per domain, with force re-analysis option. Recommendations are auto-converted to actionable todos when priority is high or medium.
