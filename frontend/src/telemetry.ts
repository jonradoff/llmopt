// telemetry.ts — Lightweight telemetry client for LLM Optimizer
//
// Anonymous users: page.view events via POST /api/telemetry/track
// Authenticated users: custom.* events via POST /api/telemetry/events (+ batch)

const SESSION_KEY = 'llmopt_telemetry_session'
const BATCH_INTERVAL = 5000
const BATCH_MAX = 50

function getSessionId(): string {
  let sid = localStorage.getItem(SESSION_KEY)
  if (!sid) {
    sid = crypto.randomUUID()
    localStorage.setItem(SESSION_KEY, sid)
  }
  return sid
}

function isAuthenticated(): boolean {
  return !!localStorage.getItem('lastsaas_access_token') && !!localStorage.getItem('lastsaas_active_tenant')
}

function getAuthHeaders(): Record<string, string> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  const token = localStorage.getItem('lastsaas_access_token')
  const tenantId = localStorage.getItem('lastsaas_active_tenant')
  if (token) headers['Authorization'] = `Bearer ${token}`
  if (tenantId) headers['X-Tenant-ID'] = tenantId
  return headers
}

// Shared context — set by App.tsx
let _currentTab = ''
let _currentDomain = ''

export function setTelemetryContext(tab: string, domain: string): void {
  _currentTab = tab
  _currentDomain = domain
}

// Batch queue for authenticated events
let _eventQueue: Array<{ event: string; properties: Record<string, unknown> }> = []
let _flushTimer: ReturnType<typeof setTimeout> | null = null

function scheduleFlush(): void {
  if (_flushTimer) return
  _flushTimer = setTimeout(flushQueue, BATCH_INTERVAL)
}

async function flushQueue(): Promise<void> {
  _flushTimer = null
  if (_eventQueue.length === 0) return

  const events = _eventQueue.splice(0, BATCH_MAX)
  try {
    await fetch('/api/telemetry/events/batch', {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ events }),
      keepalive: true,
    })
  } catch {
    // Best-effort — silently discard
  }
}

function enrich(properties: Record<string, unknown>): Record<string, unknown> {
  return {
    ...properties,
    tab: properties.tab || _currentTab || undefined,
    domain: properties.domain || _currentDomain || undefined,
  }
}

/**
 * Track a page view. Works for both anonymous and authenticated users.
 */
export function trackPageView(url?: string, title?: string): void {
  const properties: Record<string, unknown> = {
    url: url || window.location.href,
    title: title || document.title,
    referrer: document.referrer || undefined,
    tab: _currentTab || undefined,
    domain: _currentDomain || undefined,
  }

  if (isAuthenticated()) {
    trackEventImmediate('custom.page.view', properties)
  } else {
    fetch('/api/telemetry/track', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        sessionId: getSessionId(),
        event: 'page.view',
        properties,
      }),
      keepalive: true,
    }).catch(() => {})
  }
}

/**
 * Track a custom event (authenticated only). Batched for efficiency.
 * Auto-prefixes "custom." if not present. No-ops for anonymous users.
 */
export function trackEvent(eventName: string, properties: Record<string, unknown> = {}): void {
  if (!isAuthenticated()) return
  if (!eventName.startsWith('custom.')) eventName = `custom.${eventName}`

  _eventQueue.push({ event: eventName, properties: enrich(properties) })

  if (_eventQueue.length >= BATCH_MAX) {
    flushQueue()
  } else {
    scheduleFlush()
  }
}

/**
 * Track a custom event immediately (bypasses batching).
 * Use for high-value funnel events. Authenticated only.
 */
export function trackEventImmediate(eventName: string, properties: Record<string, unknown> = {}): void {
  if (!isAuthenticated()) return
  if (!eventName.startsWith('custom.')) eventName = `custom.${eventName}`

  fetch('/api/telemetry/events', {
    method: 'POST',
    headers: getAuthHeaders(),
    body: JSON.stringify({ event: eventName, properties: enrich(properties) }),
    keepalive: true,
  }).catch(() => {})
}

// Flush on page hide
if (typeof window !== 'undefined') {
  window.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') flushQueue()
  })
}
