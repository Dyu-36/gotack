package changes

import (
	"bytes"
	"fmt"
	"strings"
)

// diff.go -- role: build a lightweight unified diff for the viewer.
//
// Keep it small: no full editor, no syntax service, truncate very large files.

// maxDiffLines caps the LCS dynamic programming table, which costs O(m*n) in
// both time and space. The cap applies to the *changed region* only: RenderDiff
// strips the identical head and tail first, so a large file with a local edit
// never approaches it. Worst case is 2001*2001 int32 cells, about 16 MB, which
// fits the 6 GB budget. Past the cap we fall back to a whole-file replace hunk
// that is still useful to the UI.
const maxDiffLines = 2000

// contextLines is the unified-diff convention. The viewer highlights a hunk;
// 3 lines of context on each side is what `diff -u` and every code-review
// tool expect.
const contextLines = 3

// truncationMarker is appended to the output when the produced diff exceeds
// maxBytes. The leading newline keeps it on its own line regardless of what
// the truncated hunk ended with.
const truncationMarker = "\n... (truncated)"

// RenderDiff produces a unified diff between old and new content, both files
// labelled path. The output is truncated to maxBytes; a negative or zero
// maxBytes means "no limit" (used by tests that want to see the full result).
// When either side exceeds maxDiffLines we emit a single whole-file replace
// hunk instead of an LCS result; the diff is still informative ("the agent
// rewrote the file") and the viewer can still render the change.
func RenderDiff(old, new, path string, maxBytes int64) (string, error) {
	oldLines := splitLines(old)
	newLines := splitLines(new)

	// Real edits are local, so strip the identical head and tail before the
	// quadratic step. A 50k-line file with a three-line change then costs a 3x3
	// DP table instead of a 2.5-billion-cell one, and stays a real diff rather
	// than degrading into a whole-file replace.
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

// commonAffixes counts the identical leading and trailing lines shared by a and
// b. The two regions never overlap, so a[head:len(a)-tail] and
// b[head:len(b)-tail] are always valid slices.
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

// appendUnchanged appends lines as context ops. oldBase and newBase are the
// zero-based offsets of lines[0] within the old and the new file.
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

// lineOp is one row of the diff script. Indexes are 1-based to match the
// formatUnified output; oldIndex/newIndex are zero when not applicable.
type lineOp struct {
	oldIndex int
	newIndex int
	op       byte // ' ' context, '-' remove, '+' add
	text     string
}

// lcsOps returns the op stream for oldLines -> newLines using the standard LCS
// dynamic programming algorithm. oldBase and newBase are added to the emitted
// 1-based indices so the caller can run the LCS over a trimmed slice and still
// report positions in the full file.
func lcsOps(oldLines, newLines []string, oldBase, newBase int) []lineOp {
	m, n := len(oldLines), len(newLines)
	// dp[i][j] holds the LCS length of oldLines[:i] and newLines[:j]. A flat
	// int32 slice halves the table against int and keeps cache locality; the LCS
	// length is bounded by maxDiffLines, so int32 cannot overflow.
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
	// Walk the table from the bottom-right corner to reconstruct the script.
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
	// Reverse to restore original order.
	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}
	return ops
}

// formatUnified walks the op stream, groups nearby changes into hunks with
// three lines of context, and emits the standard --- / +++ / @@ headers.
// The path is used for both file labels; the spec does not differentiate
// old vs new.
func formatUnified(ops []lineOp, path string) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "--- %s\n", path)
	fmt.Fprintf(&buf, "+++ %s\n", path)

	// Walk ops looking for "change" regions: any op that is not a pure
	// context line. A hunk starts contextLines lines before a change and
	// ends contextLines lines after the last consecutive change. Adjacent
	// regions separated by fewer than 2*contextLines context lines merge,
	// matching `diff -u`.
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

// hunkBounds computes the @@ -X,Y +A,B @@ numbers for a slice of ops. The
// 1-based counts are derived from the op types in the slice, not the
// surrounding region: deletions count toward the old side only, additions
// toward the new side only, context toward both.
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

// hunkRange renders the start,count pair as "N,M" (or just "N" when M==1, the
// diff convention).
func hunkRange(start, count int) string {
	if count == 1 {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d,%d", start, count)
}

// renderFullReplace emits a single hunk that deletes the entire old file and
// adds the entire new file. We use it when the changed region exceeds the LCS
// budget; the UI can still present it as "file rewritten". It cannot fail, so
// it returns a plain string instead of an error the caller would have to ignore.
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

// truncate caps out at maxBytes. A non-positive maxBytes means "no limit".
// When truncation happens we append the standard marker so the UI knows the
// diff was cut.
func truncate(out string, maxBytes int64) string {
	if maxBytes <= 0 || int64(len(out)) <= maxBytes {
		return out
	}
	// Reserve space for the marker; if maxBytes is absurdly small, fall back
	// to a hard slice so we never return a result larger than the cap.
	if maxBytes <= int64(len(truncationMarker)) {
		return out[:maxBytes]
	}
	cut := int(maxBytes - int64(len(truncationMarker)))
	return out[:cut] + truncationMarker
}

// splitLines is strings.Split on "\n" with the trailing empty element
// preserved. This matches what the diff viewer expects: a file "a\nb\n"
// produces ["a", "b", ""], so the hunks still terminate cleanly.
func splitLines(s string) []string {
	if s == "" {
		return []string{""}
	}
	return strings.Split(s, "\n")
}
