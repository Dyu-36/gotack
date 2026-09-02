package changes

import (
	"bytes"
	"fmt"
	"strings"
)

const maxDiffLines = 2000

const contextLines = 3

const truncationMarker = "\n... (truncated)"

func RenderDiff(old, new, path string, maxBytes int64) (string, error) {
	oldLines := splitLines(old)
	newLines := splitLines(new)

	head, tail := commonAffixes(oldLines, newLines)
	oldMid := oldLines[head : len(oldLines)-tail]
	newMid := newLines[head : len(newLines)-tail]
	if len(oldMid) > maxDiffLines || len(newMid) > maxDiffLines {
		return renderFullReplace(oldLines, newLines, path, maxBytes), nil
	}

	ops := make([]lineOp, 0, len(oldLines)+len(newMid))
	ops = appendUnchanged(ops, oldLines[:head], 0, 0)
	ops = append(ops, lcsOps(oldMid, newMid, head, head)...)
	ops = appendUnchanged(ops, oldLines[len(oldLines)-tail:], len(oldLines)-tail, len(newLines)-tail)
	return truncate(formatUnified(ops, path), maxBytes), nil
}

func commonAffixes(a, b []string) (head, tail int) {
	limit := min(len(a), len(b))
	for head < limit && a[head] == b[head] {
		head++
	}
	for tail < limit-head && a[len(a)-1-tail] == b[len(b)-1-tail] {
		tail++
	}
	return head, tail
}

func appendUnchanged(ops []lineOp, lines []string, oldBase, newBase int) []lineOp {
	for i, l := range lines {
		ops = append(ops, lineOp{
			oldIndex: oldBase + i + 1,
			newIndex: newBase + i + 1,
			op:       ' ',
			text:     l,
		})
	}
	return ops
}

type lineOp struct {
	oldIndex int
	newIndex int
	op       byte
	text     string
}

func lcsOps(oldLines, newLines []string, oldBase, newBase int) []lineOp {
	m, n := len(oldLines), len(newLines)

	dp := make([]int32, (m+1)*(n+1))
	for i := 1; i <= m; i++ {
		row := i * (n + 1)
		prev := (i - 1) * (n + 1)
		oi := oldLines[i-1]
		for j := 1; j <= n; j++ {
			if oi == newLines[j-1] {
				dp[row+j] = dp[prev+j-1] + 1
			} else if dp[prev+j] >= dp[row+j-1] {
				dp[row+j] = dp[prev+j]
			} else {
				dp[row+j] = dp[row+j-1]
			}
		}
	}

	ops := make([]lineOp, 0, m+n)
	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && oldLines[i-1] == newLines[j-1]:
			ops = append(ops, lineOp{oldIndex: oldBase + i, newIndex: newBase + j, op: ' ', text: oldLines[i-1]})
			i--
			j--
		case j > 0 && (i == 0 || dp[(i)*(n+1)+j-1] >= dp[(i-1)*(n+1)+j]):
			ops = append(ops, lineOp{oldIndex: 0, newIndex: newBase + j, op: '+', text: newLines[j-1]})
			j--
		default:
			ops = append(ops, lineOp{oldIndex: oldBase + i, newIndex: 0, op: '-', text: oldLines[i-1]})
			i--
		}
	}

	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}
	return ops
}

func formatUnified(ops []lineOp, path string) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "--- %s\n", path)
	fmt.Fprintf(&buf, "+++ %s\n", path)

	type region struct{ start, end int }
	var regions []region
	changeStart := -1
	for idx := range ops {
		if ops[idx].op != ' ' {
			if changeStart == -1 {
				changeStart = idx
			}
			continue
		}
		if changeStart != -1 {
			regions = append(regions, region{start: changeStart, end: idx - 1})
			changeStart = -1
		}
	}
	if changeStart != -1 {
		regions = append(regions, region{start: changeStart, end: len(ops) - 1})
	}

	for _, r := range regions {
		cs := r.start - contextLines
		if cs < 0 {
			cs = 0
		}
		ce := r.end + contextLines
		if ce > len(ops)-1 {
			ce = len(ops) - 1
		}
		oldStart, newStart, oldCount, newCount := hunkBounds(ops, cs, ce)
		fmt.Fprintf(&buf, "@@ -%s +%s @@\n", hunkRange(oldStart, oldCount), hunkRange(newStart, newCount))
		for k := cs; k <= ce; k++ {
			buf.WriteByte(ops[k].op)
			buf.WriteString(ops[k].text)
			if !strings.HasSuffix(ops[k].text, "\n") {
				buf.WriteByte('\n')
			}
		}
	}
	return buf.String()
}

func hunkBounds(ops []lineOp, start, end int) (oldStart, newStart, oldCount, newCount int) {
	oldLine, newLine := 0, 0
	for k := start; k <= end; k++ {
		op := ops[k]
		switch op.op {
		case ' ':
			if oldLine == 0 {
				oldStart = op.oldIndex
			}
			if newLine == 0 {
				newStart = op.newIndex
			}
			oldLine++
			newLine++
		case '-':
			if oldLine == 0 {
				oldStart = op.oldIndex
			}
			oldLine++
		case '+':
			if newLine == 0 {
				newStart = op.newIndex
			}
			newLine++
		}
	}
	return oldStart, newStart, oldLine, newLine
}

func hunkRange(start, count int) string {
	if count == 1 {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d,%d", start, count)
}

func renderFullReplace(oldLines, newLines []string, path string, maxBytes int64) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "--- %s\n", path)
	fmt.Fprintf(&buf, "+++ %s\n", path)
	oldStart := 1
	if len(oldLines) == 0 {
		oldStart = 0
	}
	newStart := 1
	if len(newLines) == 0 {
		newStart = 0
	}
	fmt.Fprintf(&buf, "@@ -%s +%s @@\n",
		hunkRange(oldStart, len(oldLines)),
		hunkRange(newStart, len(newLines)),
	)
	for _, l := range oldLines {
		buf.WriteByte('-')
		buf.WriteString(l)
		if !strings.HasSuffix(l, "\n") {
			buf.WriteByte('\n')
		}
	}
	for _, l := range newLines {
		buf.WriteByte('+')
		buf.WriteString(l)
		if !strings.HasSuffix(l, "\n") {
			buf.WriteByte('\n')
		}
	}
	return truncate(buf.String(), maxBytes)
}

func truncate(out string, maxBytes int64) string {
	if maxBytes <= 0 || int64(len(out)) <= maxBytes {
		return out
	}

	if maxBytes <= int64(len(truncationMarker)) {
		return out[:maxBytes]
	}
	cut := int(maxBytes - int64(len(truncationMarker)))
	return out[:cut] + truncationMarker
}

func splitLines(s string) []string {
	if s == "" {
		return []string{""}
	}
	return strings.Split(s, "\n")
}
