#!/bin/bash
# Stdio-to-HTTP bridge for LLM Optimizer MCP server
# Used by Claude Desktop to connect to llmopt's Streamable HTTP MCP endpoint.
# Requires mcp-remote: npm install -g mcp-remote
exec /Users/jonradoff/.local/node-arm64/bin/mcp-remote https://llmopt.fly.dev/mcp 2>/dev/null
