# LLM Optimizer

LLM Optimizer analyzes and optimizes your brand's visibility across AI-powered answer engines, search, video, and social platforms. It provides actionable intelligence through site analysis, answer engine optimization scoring, video authority analysis, Reddit authority analysis, search visibility analysis, and LLM knowledge testing.

## MCP Server

LLM Optimizer includes a [Model Context Protocol (MCP)](https://modelcontextprotocol.io) server that lets AI assistants like Claude access your optimization data programmatically. The server uses **Streamable HTTP** transport with **OAuth 2.1** authentication.

**Server URL:** `https://llmopt.fly.dev/mcp`

### Authentication

The MCP server supports two authentication methods:

1. **OAuth 2.1 (recommended for Claude Desktop)** — Full OAuth flow with PKCE and Dynamic Client Registration. Claude Desktop handles this automatically when you add the server URL.

2. **API Key Bearer Token (recommended for Claude Code and scripts)** — Pass your `lsk_` API key directly as a Bearer token. Create API keys at Settings > API Keys in the app.

### Available Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `llmopt_list_domains` | List all domains tracked for your account | _(none)_ |
| `llmopt_get_report` | Get a specific analysis report for a domain | `domain` (required), `report_type` (required): analysis, optimizations, video, reddit, search, summary, tests, brand |
| `llmopt_get_visibility_score` | Get the composite AI visibility score (0-100) for a domain, weighted across 5 components | `domain` (required) |
| `llmopt_list_todos` | List action items from optimization analyses | `status` (optional): todo, completed, backlogged, archived; `domain` (optional) |
| `llmopt_update_todo` | Update a todo item's status (admin/owner only) | `id` (required), `status` (required): todo, completed, backlogged, archived |

### Examples

#### 1. List all tracked domains

Discover which domains have analysis data in your account.

```json
{
  "tool": "llmopt_list_domains",
  "input": {}
}
```

**Response:**
```json
{
  "domains": ["example.com", "competitor.io", "mybrand.co"],
  "count": 3
}
```

#### 2. Get a video authority report

Retrieve the video authority analysis for a domain, including YouTube presence scoring across four pillars: Transcript Authority, Topical Dominance, Citation Network, and Brand Narrative.

```json
{
  "tool": "llmopt_get_report",
  "input": {
    "domain": "example.com",
    "report_type": "video"
  }
}
```

**Response:**
```json
{
  "domain": "example.com",
  "result": {
    "overallScore": 72,
    "pillars": {
      "transcriptAuthority": { "score": 78, "summary": "..." },
      "topicalDominance": { "score": 65, "summary": "..." },
      "citationNetwork": { "score": 70, "summary": "..." },
      "brandNarrative": { "score": 75, "summary": "..." }
    }
  },
  "generatedAt": "2026-02-28T10:30:00Z"
}
```

#### 3. Check visibility score

Get the composite AI visibility score, which aggregates Optimization (30%), Video Authority (20%), Reddit Authority (20%), Search Visibility (15%), and LLM Test (15%) into a single 0-100 score.

```json
{
  "tool": "llmopt_get_visibility_score",
  "input": {
    "domain": "example.com"
  }
}
```

**Response:**
```json
{
  "domain": "example.com",
  "score": 68,
  "components": [
    { "name": "Optimization", "score": 75, "weight": 0.30, "available": true },
    { "name": "Video Authority", "score": 72, "weight": 0.20, "available": true },
    { "name": "Reddit Authority", "score": 58, "weight": 0.20, "available": true },
    { "name": "Search Visibility", "score": 62, "weight": 0.15, "available": true },
    { "name": "LLM Test", "score": 70, "weight": 0.15, "available": true }
  ],
  "available": 5,
  "total": 5
}
```

#### 4. List open todos for a domain

Find all actionable recommendations that haven't been completed yet for a specific domain.

```json
{
  "tool": "llmopt_list_todos",
  "input": {
    "status": "todo",
    "domain": "example.com"
  }
}
```

**Response:**
```json
{
  "todos": [
    {
      "_id": "683abc1234567890abcdef01",
      "domain": "example.com",
      "title": "Add structured FAQ schema to pricing page",
      "description": "The pricing page lacks FAQ schema markup...",
      "status": "todo",
      "priority": "high",
      "createdAt": "2026-02-25T14:00:00Z"
    }
  ],
  "count": 1
}
```

#### 5. Mark a todo as completed

Update the status of a todo item after implementing the recommendation. Requires admin or owner role.

```json
{
  "tool": "llmopt_update_todo",
  "input": {
    "id": "683abc1234567890abcdef01",
    "status": "completed"
  }
}
```

**Response:**
```json
{
  "updated": true,
  "id": "683abc1234567890abcdef01",
  "status": "completed"
}
```

### Configuration

#### Claude Desktop

Add via **Settings > Connectors > Add**:
- **URL:** `https://llmopt.fly.dev/mcp`
- OAuth authentication is handled automatically.

#### Claude Code

Add to your project's `.mcp.json` or user-level `~/.claude.json`:

```json
{
  "mcpServers": {
    "llmopt": {
      "command": "npx",
      "args": [
        "mcp-remote",
        "https://llmopt.fly.dev/mcp",
        "--header",
        "Authorization: Bearer ${LLMOPT_API_KEY}"
      ],
      "env": {
        "LLMOPT_API_KEY": "lsk_your_api_key_here"
      }
    }
  }
}
```

#### Direct HTTP

```bash
# List available tools
curl -X POST https://llmopt.fly.dev/mcp \
  -H "Authorization: Bearer lsk_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Call a tool
curl -X POST https://llmopt.fly.dev/mcp \
  -H "Authorization: Bearer lsk_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"llmopt_list_domains","arguments":{}}}'
```

### OAuth Endpoints

The MCP server implements OAuth 2.1 with PKCE for MCP client compatibility:

| Endpoint | Description |
|----------|-------------|
| `GET /.well-known/oauth-protected-resource` | Protected Resource Metadata (RFC 9728) |
| `GET /.well-known/oauth-authorization-server` | Authorization Server Metadata (RFC 8414) |
| `POST /oauth/register` | Dynamic Client Registration (RFC 7591) |
| `GET /oauth/authorize` | Authorization endpoint (shows API key entry form) |
| `POST /oauth/token` | Token endpoint (code exchange + refresh) |

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MCP_JWT_SECRET` | HMAC-SHA256 signing key for MCP access tokens | Derived from `ENCRYPTION_KEY` |
| `BASE_URL` | Public URL of the server (used in OAuth metadata) | `https://llmopt.fly.dev` |

## REST API

A REST API is also available at `/api/v1/`. See the [API Reference](https://llmopt.fly.dev/api/v1/docs) for full documentation, or visit the [API Docs](https://llmopt.fly.dev/docs) page for a formatted guide.
