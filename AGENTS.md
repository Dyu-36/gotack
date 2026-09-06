<!-- codebase-memory-mcp:start -->
# Codebase Knowledge Graph (codebase-memory-mcp)

This project uses codebase-memory-mcp to maintain a knowledge graph of the codebase.
ALWAYS prefer MCP graph tools over grep/glob/file-search for code discovery.
NOTE: The `project` argument is REQUIRED for all tool calls (for this repository, use `project="gotack"`).

## Priority Order
1. `search_graph` — find functions, classes, routes, variables by pattern (requires `project`)
2. `trace_path` — trace who calls a function or what it calls (requires `project` and `function_name`)
3. `get_code_snippet` — read specific function/class source code (requires `project` and `qualified_name`)
4. `query_graph` — run Cypher queries for complex patterns (requires `project` and `query`)
5. `get_architecture` — high-level project summary (requires `project`)

## When to fall back to grep/glob
- Searching for string literals, error messages, config values
- Searching non-code files (Dockerfiles, shell scripts, configs)
- When MCP tools return insufficient results

## Examples
- List projects: `list_projects()`
- Find a handler: `search_graph(project="gotack", name_pattern=".*Handler.*")`
- Who calls it: `trace_path(project="gotack", function_name="Handler", direction="inbound")`
- Read source: `get_code_snippet(project="gotack", qualified_name="main.App")`
<!-- codebase-memory-mcp:end -->


