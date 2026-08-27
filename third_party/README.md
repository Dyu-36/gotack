# third_party

Vendored upstream code. Not part of the gotack Go module and not covered by the
gotack code standards.

| Folder | Upstream | Notes |
| --- | --- | --- |
| crush | https://github.com/charmbracelet/crush | Core agent engine. Keeps its own .git history and go.mod. Ignored by this repo, see .gitignore. |

## Rules

- Contents are read-only for desktop needs: desktop-only behaviour goes in internal/.
- gotack talks to crush over REST + SSE only.
- Go forbids importing third_party/crush/internal/... from another module, so the
  wire contract is re-declared in internal/crushapi.
- Record the pinned upstream commit whenever this folder is refreshed.
