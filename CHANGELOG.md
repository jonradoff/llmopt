# Changelog

## 2026-06-18

- **LLM Test**: added a periodic SSE heartbeat to all 15 streaming endpoints, keeping the connection alive through long, silent LLM calls. Fixes the "Load Failed" error that dropped the Test stream mid-run behind the production proxy chain (Fly edge -> Caddy).
- **LLM Test**: Phase 1 provider queries now run concurrently (capped at 6) instead of sequentially, cutting a large provider×query matrix from the sum of all calls to roughly the slowest single call. The overall request timeout was raised from 5 to 15 minutes. Together these fix the "Could not resolve primary LLM for evaluation" error that occurred when a long run exhausted the request budget before the evaluation step.
- **API keys**: `resolveProviderKey`/`resolvePrimaryLLM` no longer map transient DB/context errors to `api_key_required`; a genuinely-missing key returns a sentinel error while real failures are wrapped and logged. The Test evaluation also falls back to a tested provider's key when the primary provider has none configured.
- **Report**: the aggregate PDF's "YouTube Video Authority" section now renders the brand narrative ("What LLMs Probably Believe About You") with sentiment, key themes, share of voice, content-gap opportunities, top creators, creator outreach targets, and the confidence note — matching the YouTube tab instead of showing only scores and scorecard titles.

## 2026-06-05

- **Video Authority**: added warning banner explaining that YouTube's bot detection frequently blocks transcript fetching from datacenter IPs (including the hosted instance at llmopt.fly.dev), and recommending self-hosting on a residential connection for reliable results.

## 2026-06-03

- **Video Authority**: added `yt-dlp` + `deno` based transcript-fetcher as the primary method, with the existing InnerTube clients (TVHTML5 embed, MWEB, WEB, ANDROID, watch-page scrape) retained as fallbacks. yt-dlp's EJS challenge solver is enabled via `--remote-components ejs:github` so the JS-challenge step succeeds when the egress IP is residential.
- **Video Authority**: sticky preference cache for transcript methods — once a method succeeds it floats to the front of the list for subsequent calls in the same process.
- **Video Discovery**: added opt-out checkbox for auto-generated brand-name search queries (`<brand> review`, `<brand> tutorial`, `<brand> vs <competitor>`) to avoid false positives when the brand name is a common phrase.
- **Video Discovery**: errors from individual YouTube search calls are now surfaced via the progress stream and logged, instead of being silently swallowed.
- **Rate limiting**: keyed by `(client IP, request path)` so per-endpoint limits don't share a counter. `VideoDiscoverLimit` and `VideoAnalyzeLimit` bumped from 3/min to 10/min.
