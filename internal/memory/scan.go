package memory

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// The memory store uses Hermes' strict context scanner for both writes and
// prompt snapshots. Keep this table in the same order and with the same
// bounded languages as hermes-agent's tools/threat_patterns.py: strict
// includes the all and context classes as well as memory/skill-only checks.
// Python's “\w“ is Unicode-aware; RE2's shorthand is ASCII-only. Spell out
// the Unicode letter/number classes so an obfuscated phrase cannot bypass the
// same bounded filler check merely by using non-ASCII words.
const threatFiller = `(?:[\pL\pN_]+\s+){0,8}`

type scanRule struct {
	name string
	re   *regexp.Regexp
}

var scanRules = []scanRule{
	{name: "prompt_injection", re: regexp.MustCompile(`(?i)ignore\s+` + threatFiller + `(previous|all|above|prior)\s+` + threatFiller + `instructions`)},
	{name: "sys_prompt_override", re: regexp.MustCompile(`(?i)system\s+prompt\s+override`)},
	{name: "disregard_rules", re: regexp.MustCompile(`(?i)disregard\s+` + threatFiller + `(your|all|any)\s+` + threatFiller + `(instructions|rules|guidelines)`)},
	{name: "bypass_restrictions", re: regexp.MustCompile(`(?i)act\s+as\s+(if|though)\s+` + threatFiller + `you\s+` + threatFiller + `(have\s+no|don't\s+have)\s+` + threatFiller + `(restrictions|limits|rules)`)},
	{name: "html_comment_injection", re: regexp.MustCompile(`(?i)<!--[^>]{0,512}(?:ignore|override|system|secret|hidden)[^>]{0,512}-->`)},
	{name: "hidden_div", re: regexp.MustCompile(`(?i)<\s*div\s+style\s*=\s*["'][^>]{0,1000}[^>]{0,1000}[^>]{0,48}display\s*:\s*none`)},
	{name: "translate_execute", re: regexp.MustCompile(`(?i)translate\s+[^\n]{0,512}\s+into\s+[^\n]{0,512}\s+and\s+(execute|run|eval)`)},
	{name: "deception_hide", re: regexp.MustCompile(`(?i)do\s+not\s+` + threatFiller + `tell\s+` + threatFiller + `the\s+user`)},
	{name: "role_hijack", re: regexp.MustCompile(`(?i)you\s+are\s+` + threatFiller + `now\s+(?:a|an|the)\s+`)},
	{name: "role_pretend", re: regexp.MustCompile(`(?i)pretend\s+` + threatFiller + `(you\s+are|to\s+be)\s+`)},
	{name: "leak_system_prompt", re: regexp.MustCompile(`(?i)output\s+` + threatFiller + `(system|initial)\s+prompt`)},
	{name: "remove_filters", re: regexp.MustCompile(`(?i)(respond|answer|reply)\s+without\s+` + threatFiller + `(restrictions|limitations|filters|safety)`)},
	{name: "fake_update", re: regexp.MustCompile(`(?i)you\s+have\s+been\s+` + threatFiller + `(updated|upgraded|patched)\s+to`)},
	{name: "identity_override", re: regexp.MustCompile(`(?i)\bname\s+yourself\s+\w+`)},
	{name: "c2_node_registration", re: regexp.MustCompile(`(?i)register\s+(as\s+)?a?\s*node`)},
	{name: "c2_heartbeat", re: regexp.MustCompile(`(?i)(heartbeat|beacon|check[\s-]?in)\s+(to|with)\s+`)},
	{name: "c2_task_pull", re: regexp.MustCompile(`(?i)pull\s+(down\s+)?(?:new\s+)?task(?:ing|s)?\b`)},
	{name: "c2_network_connect", re: regexp.MustCompile(`(?i)connect\s+to\s+the\s+network\b`)},
	{name: "forced_action", re: regexp.MustCompile(`(?i)you\s+must\s+(?:\w+\s+){0,3}(register|connect|report|beacon)\b`)},
	{name: "anti_forensic_oneliner", re: regexp.MustCompile(`(?i)only\s+use\s+one[\s-]?liners?\b`)},
	{name: "anti_forensic_disk", re: regexp.MustCompile(`(?i)never\s+` + threatFiller + `(?:create|write)\s+` + threatFiller + `(?:script|file)\s+` + threatFiller + `disk`)},
	{name: "env_var_unset_agent", re: regexp.MustCompile(`(?i)unset\s+\w*(?:CLAUDE|CODEX|HERMES|AGENT|OPENAI|ANTHROPIC)\w*`)},
	{name: "known_c2_framework", re: regexp.MustCompile(`(?i)\b(?:cobalt\s*strike|sliver|havoc|mythic|metasploit|brainworm)\b`)},
	{name: "c2_explicit", re: regexp.MustCompile(`(?i)\bc2\s+(?:server|channel|infrastructure|beacon)\b`)},
	{name: "c2_explicit_long", re: regexp.MustCompile(`(?i)\bcommand\s+and\s+control\b`)},
	{name: "exfil_curl", re: regexp.MustCompile(`(?i)curl\s+[^\n]{0,1000}[^\n]{0,1000}[^\n]{0,48}\$\{?\w*(?:KEY|TOKEN|SECRET|PASSWORD|CREDENTIAL)S?\b`)},
	{name: "exfil_wget", re: regexp.MustCompile(`(?i)wget\s+[^\n]{0,1000}[^\n]{0,1000}[^\n]{0,48}\$\{?\w*(?:KEY|TOKEN|SECRET|PASSWORD|CREDENTIAL)S?\b`)},
	{name: "read_secrets", re: regexp.MustCompile(`(?i)cat\s+[^\n]{0,1000}[^\n]{0,1000}[^\n]{0,48}(\.env|credentials|\.netrc|\.pgpass|\.npmrc|\.pypirc)`)},
	{name: "send_to_url", re: regexp.MustCompile(`(?i)(send|post|upload|transmit)\s+[^\n]{0,1000}[^\n]{0,1000}[^\n]{0,48}\s+(to|at)\s+https?://`)},
	{name: "context_exfil", re: regexp.MustCompile(`(?i)(include|output|print|share)\s+` + threatFiller + `(conversation|chat\s+history|previous\s+messages|full\s+context|entire\s+context)`)},
	{name: "ssh_backdoor", re: regexp.MustCompile(`(?i)authorized_keys`)},
	{name: "ssh_access", re: regexp.MustCompile(`(?i)\$HOME/\.ssh|~/\.ssh`)},
	{name: "hermes_env", re: regexp.MustCompile(`(?i)\$HOME/\.hermes/\.env|~/\.hermes/\.env`)},
	{name: "agent_config_mod", re: regexp.MustCompile(`(?i)(update|modify|edit|write|change|append|add\s+to)\s+[^\n]{0,1000}[^\n]{0,1000}[^\n]{0,48}(?:AGENTS\.md|CLAUDE\.md|\.cursorrules|\.clinerules)`)},
	{name: "hermes_config_mod", re: regexp.MustCompile(`(?i)(update|modify|edit|write|change|append|add\s+to)\s+[^\n]{0,1000}[^\n]{0,1000}[^\n]{0,48}\.hermes/(config\.yaml|SOUL\.md)`)},
	{name: "hardcoded_secret", re: regexp.MustCompile(`(?i)(?:api[_-]?key|token|secret|password)\s*[=:]\s*["'][A-Za-z0-9+/=_-]{20,}`)},
}

var invisibleRunes = map[rune]struct{}{
	'\u200b': {}, // zero-width space
	'\u200c': {}, // zero-width non-joiner
	'\u200d': {}, // zero-width joiner
	'\u2060': {}, // word joiner
	'\u2062': {}, // invisible times
	'\u2063': {}, // invisible separator
	'\u2064': {}, // invisible plus
	'\ufeff': {}, // zero-width no-break space (BOM)
	'\u202a': {}, // left-to-right embedding
	'\u202b': {}, // right-to-left embedding
	'\u202c': {}, // pop directional formatting
	'\u202d': {}, // left-to-right override
	'\u202e': {}, // right-to-left override
	'\u2066': {}, // left-to-right isolate
	'\u2067': {}, // right-to-left isolate
	'\u2068': {}, // first strong isolate
	'\u2069': {}, // pop directional isolate
}

// MaxScanChars is Hermes' hard scanner bound. It is a rune/character bound,
// matching Python's content[:MAX_SCAN_CHARS], not a byte bound.
const MaxScanChars = 65_536

// scanFindings is the direct Go equivalent of Hermes' scan_for_threats(...,
// scope="strict"). It scans only the first 65,536 runes, checks invisible
// characters before NFKC (normalization can remove some), then applies the
// strict expressions to the normalized copy.
func scanFindings(content string) []string {
	if content == "" {
		return nil
	}
	truncated := truncateRunes(content, MaxScanChars)
	findings := make([]string, 0, 2)
	seenInvisible := make(map[rune]struct{}, 2)
	for _, ch := range truncated {
		if _, ok := invisibleRunes[ch]; ok {
			seenInvisible[ch] = struct{}{}
		}
	}
	// Hermes' Python set iteration is intentionally not an ordering contract;
	// deterministic code-point order keeps diagnostics stable without changing
	// which findings are returned.
	for _, ch := range []rune{
		'\u200b', '\u200c', '\u200d', '\u2060', '\u2062', '\u2063', '\u2064',
		'\ufeff', '\u202a', '\u202b', '\u202c', '\u202d', '\u202e', '\u2066',
		'\u2067', '\u2068', '\u2069',
	} {
		if _, ok := seenInvisible[ch]; ok {
			findings = append(findings, fmt.Sprintf("invisible_unicode_U+%04X", ch))
		}
	}
	normalized := norm.NFKC.String(truncated)
	for _, rule := range scanRules {
		if rule.re.MatchString(normalized) {
			findings = append(findings, rule.name)
		}
	}
	return findings
}

func truncateRunes(content string, limit int) string {
	if limit <= 0 {
		return ""
	}
	var out strings.Builder
	count := 0
	for _, ch := range content {
		if count == limit {
			break
		}
		out.WriteRune(ch)
		count++
	}
	return out.String()
}

// Scan rejects content using Hermes' strict memory/skill scanner.
func Scan(content string) error {
	findings := scanFindings(content)
	if len(findings) == 0 {
		return nil
	}
	first := findings[0]
	if strings.HasPrefix(first, "invisible_unicode_") {
		return fmt.Errorf("%w: content contains invisible unicode character %s (possible injection)", ErrBlocked, strings.TrimPrefix(first, "invisible_unicode_"))
	}
	return fmt.Errorf("%w: content matches threat pattern '%s'. Content is injected into the system prompt and must not contain injection or exfiltration payloads", ErrBlocked, first)
}

// ScanRuleNames exposes the exact strict pattern identifiers for diagnostics
// and contract tests. Invisible Unicode findings are content-specific and are
// therefore not included in this static list.
func ScanRuleNames() []string {
	names := make([]string, 0, len(scanRules))
	for _, rule := range scanRules {
		names = append(names, rule.name)
	}
	return names
}

// SanitizeEntriesForPrompt replaces threat-bearing entries with Hermes' load
// time placeholder while leaving the caller's raw entries untouched. This is
// used to build the frozen context mirror that Crush reads.
func SanitizeEntriesForPrompt(entries []string, filename string) []string {
	sanitized := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if _, ok := seen[entry]; ok {
			continue
		}
		seen[entry] = struct{}{}
		if entry == "" || strings.HasPrefix(entry, "[BLOCKED:") {
			sanitized = append(sanitized, entry)
			continue
		}
		findings := scanFindings(entry)
		if len(findings) == 0 {
			sanitized = append(sanitized, entry)
			continue
		}
		sanitized = append(sanitized, fmt.Sprintf(
			"[BLOCKED: %s entry contained threat pattern(s): %s. Removed from system prompt; use memory(action=remove) to delete the original.]",
			filename, strings.Join(findings, ", "),
		))
	}
	return sanitized
}

// SanitizeFileForPrompt parses one raw memory file and renders the sanitized
// Hermes block. Invalid UTF-8 is refused so the caller cannot accidentally
// replace or expose undecodable source content.
func SanitizeFileForPrompt(target Target, data []byte) (string, error) {
	if target != TargetMemory && target != TargetUser {
		return "", ErrUnknownTarget
	}
	if !utf8.Valid(data) {
		return "", ErrInvalidUTF8
	}
	parsed := parseFile(string(data))
	entries := SanitizeEntriesForPrompt(parsed.Entries, FileNameFor(target))
	return Render(target, serializeEntries(entries)), nil
}
