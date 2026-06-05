# Changelog

## 2026-06-05

- **Video Authority**: added warning banner explaining that YouTube's bot detection frequently blocks transcript fetching from datacenter IPs (including the hosted instance at llmopt.fly.dev), and recommending self-hosting on a residential connection for reliable results.

## 2026-06-03

- **Video Authority**: added `yt-dlp` + `deno` based transcript-fetcher as the primary method, with the existing InnerTube clients (TVHTML5 embed, MWEB, WEB, ANDROID, watch-page scrape) retained as fallbacks. yt-dlp's EJS challenge solver is enabled via `--remote-components ejs:github` so the JS-challenge step succeeds when the egress IP is residential.
- **Video Authority**: sticky preference cache for transcript methods — once a method succeeds it floats to the front of the list for subsequent calls in the same process.
- **Video Discovery**: added opt-out checkbox for auto-generated brand-name search queries (`<brand> review`, `<brand> tutorial`, `<brand> vs <competitor>`) to avoid false positives when the brand name is a common phrase.
- **Video Discovery**: errors from individual YouTube search calls are now surfaced via the progress stream and logged, instead of being silently swallowed.
- **Rate limiting**: keyed by `(client IP, request path)` so per-endpoint limits don't share a counter. `VideoDiscoverLimit` and `VideoAnalyzeLimit` bumped from 3/min to 10/min.
