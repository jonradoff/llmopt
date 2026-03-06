#!/bin/sh
# Entrypoint for the LLM Optimizer MCP proxy Docker image.
# Connects to the hosted MCP server, optionally authenticating with a lok_ API key.

MCP_URL="https://llmopt.metavert.io/mcp"

if [ -n "$API_KEY" ]; then
    exec mcp-remote "$MCP_URL" --header "Authorization: Bearer $API_KEY"
else
    exec mcp-remote "$MCP_URL"
fi
