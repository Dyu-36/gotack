---
name: officecli
description: Create, analyze, proofread, and modify Office documents (.docx, .xlsx, .pptx) using the officecli CLI tool. Use when the user wants to create, inspect, check formatting, find issues, add charts, or modify Office documents.
---

# officecli

Built-in AI-friendly CLI for .docx, .xlsx, .pptx. Single binary bundled with Tack, no external dependencies or Office installation needed.

---

## Strategy

**L1 (read) → L2 (DOM edit) → L3 (raw XML)**. Always prefer higher layers. Add `--json` for structured output.

---

## Help System (IMPORTANT)

**When unsure about property names, value formats, or command syntax, ALWAYS run help instead of guessing.**

```bash
officecli help                                  # All commands + global options + schema entry points
officecli help docx                             # List all docx elements
officecli help docx paragraph                   # Full schema: properties, aliases, examples, readbacks
officecli help docx set paragraph               # Verb-filtered: only props usable with `set`
officecli help docx paragraph --json            # Structured schema (machine-readable)
officecli help xlsx cell                        # Excel cell properties & formats
officecli help pptx shape                       # PPT shape properties & animations
```

Format aliases: `word`→`docx`, `excel`→`xlsx`, `ppt`/`powerpoint`→`pptx`.
Verbs: `add`, `set`, `get`, `query`, `remove`.

---

## Performance: Resident Mode

**Every command auto-starts a resident on first access** (60s idle timeout) — file-lock conflicts are automatically avoided. Explicit `open`/`close` is recommended for multi-step sessions:

```bash
officecli open report.docx       # explicitly keep in memory
officecli set report.docx ...    # no file I/O overhead
officecli close report.docx      # save and release to disk
```

---

## Quick Start

### PowerPoint (.pptx)
```bash
# Create a presentation and add slides & shapes
officecli create slides.pptx
officecli add slides.pptx / --type slide --prop title="Q4 Report" --prop background=1A1A2E
officecli add slides.pptx '/slide[1]' --type shape \
  --prop text="Revenue grew 25%" --prop x=2cm --prop y=5cm \
  --prop font=Arial --prop size=24 --prop color=FFFFFF
```

### Word (.docx)
```bash
# Create a document and add structured content
officecli create report.docx
officecli add report.docx /body --type paragraph --prop text="Executive Summary" --prop style=Heading1
officecli add report.docx /body --type paragraph --prop text="Revenue increased by 25% year-over-year."
```

### Excel (.xlsx)
```bash
# Create a spreadsheet and set cell values & styles
officecli create data.xlsx
officecli set data.xlsx /Sheet1/A1 --prop value="Name" --prop bold=true --prop fill=F2F2F2
officecli set data.xlsx /Sheet1/A2 --prop value="Alice"
officecli set data.xlsx /Sheet1/B1 --prop value="Score" --prop bold=true
officecli set data.xlsx /Sheet1/B2 --prop value=95 --prop type=Number
```

---

## L1: Create, Read & Inspect

```bash
officecli create <file>               # Create blank .docx/.xlsx/.pptx (type from extension)
officecli view <file> <mode>          # outline | stats | issues | text | annotated | html | screenshot
officecli get <file> <path> --depth N # Get a node and its children [--json]
officecli query <file> <selector>     # CSS-like query
officecli validate <file>             # Validate against OpenXML schema
```

### View Modes

| Mode | Description | Useful flags |
|---|---|---|
| `outline` | Document structure | |
| `stats` | Statistics (pages, words, shapes) | |
| `issues` | Formatting/content/structure problems (Linter) | `--type format\|content\|structure`, `--limit N` |
| `text` | Plain text extraction | `--start N --end N`, `--max-lines N` |
| `annotated` | Text with formatting annotations | |
| `html` | Static HTML snapshot / visual render | `--browser`, `--page N` (docx), `--start N --end N` (pptx) |
| `screenshot` | PNG via headless browser | `-o preview.png`, `--screenshot-width/-height` |

### Path Addressing & Selectors
- **Stable ID addressing:** Elements with stable IDs return `@attr=value` paths. Always prefer stable IDs over positional indices:
  - `/slide[1]/shape[@id=550950021]` (PPT shape)
  - `/body/p[@paraId=1A2B3C4D]` (Word paragraph)
  - `/comments/comment[@commentId=1]` (Word comment)
- **Query selectors:**
  ```bash
  officecli query report.docx 'paragraph[style=Normal] > run[font!=Arial]'
  officecli query slides.pptx 'shape[fill=FF0000]'
  officecli query data.xlsx 'Sheet1!row[Salary>5000]'
  ```

---

## L2: DOM Operations (Set, Add, Remove, Find & Replace)

### `set` — Modify Properties
```bash
officecli set <file> <path> --prop key=value [--prop ...]
```

**Value Formats:**
- **Colors:** Hex (`#FF0000`, `FF0000`), named (`red`, `blue`), RGB (`rgb(255,0,0)`), theme (`accent1`..`accent6`).
- **Spacing:** Unit-qualified (`12pt`, `0.5cm`, `1.5x`, `150%`).
- **Dimensions:** Suffixed or EMU (`2.54cm`, `1in`, `72pt`, `96px`, `914400`).
- **Dotted aliases:** `font.color=red`, `font.bold=true`, `font.size=14pt`, `font.name=Arial`.

### Find & Replace / Format Matched Text
```bash
# Format matched text
officecli set doc.docx '/body/p[1]' --find weather --prop bold=true --prop color=red

# Replace text across entire document
officecli set doc.docx / --find "Draft" --replace "Final"
```

---

## Live Watch & Interactive Selection

```bash
officecli watch <file> [--port 26315]  # Starts live HTML preview server at localhost:26315
officecli unwatch <file>               # Stops the preview server
officecli get <file> selected [--json] # Reads the element(s) currently clicked by user in browser
```
