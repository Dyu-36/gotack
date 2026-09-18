package guard

import (
	"fmt"
	"regexp"
	"strings"
)

type blockRule struct {
	Name     string
	Halt     bool
	patterns []*regexp.Regexp
	match    func(string) bool
}

func (r blockRule) reason(command string) string {
	return fmt.Sprintf(
		"gotack-guard: denied by rule %q — %s (command: %s)",
		r.Name, r.description(), command,
	)
}

func (r blockRule) description() string {
	switch r.Name {
	case ruleRecursiveForceDelete:
		return "recursive force delete of a filesystem root or home directory"
	case ruleDiskFormatWipe:
		return "disk format or wipe"
	case ruleMassPermissionChange:
		return "mass recursive permission change"
	case ruleShutdownReboot:
		return "system shutdown or reboot"
	case ruleHistoryCredentialDestruction:
		return "destruction of shell history or credential stores"
	case ruleCredentialExfiltration:
		return "credential material sent to a network sink"
	default:
		return "unrecoverable command"
	}
}

const (
	ruleRecursiveForceDelete         = "recursive-force-delete"
	ruleDiskFormatWipe               = "disk-format-wipe"
	ruleMassPermissionChange         = "mass-permission-change"
	ruleShutdownReboot               = "shutdown-reboot"
	ruleHistoryCredentialDestruction = "history-credential-destruction"
	ruleCredentialExfiltration       = "credential-exfiltration"
)

func mustRules() []blockRule {
	mk := func(name string, halt bool, patterns ...string) blockRule {
		compiled := make([]*regexp.Regexp, len(patterns))
		for i, p := range patterns {
			compiled[i] = regexp.MustCompile(p)
		}
		return blockRule{Name: name, Halt: halt, patterns: compiled}
	}
	rules := []blockRule{
		mk(ruleRecursiveForceDelete, true,

			`(?i)\brm\s+(-[a-z]*r[a-z]*f[a-z]*|-[a-z]*f[a-z]*r[a-z]*)\s+(/|/\*|~|\$HOME)(\s|$)`,
			`(?i)\brm\s+(-[a-z]+\s+)*--no-preserve-root`,
			`(?i)\b(rd|rmdir|del|erase)\b\s+/s\s+/q\s+[a-z]:\\?(\s|$)`,
			`(?i)Remove-Item\s+("')?[a-z]:\\?("')?\s+-Recurse\s+-Force`,
		),
		mk(ruleDiskFormatWipe, true,
			`(?i)\bformat\b\s+[a-z]:`,
			`(?i)\bmkfs(\.[a-z0-9]+)?\b`,
			`(?i)\bdiskpart\b`,
			`(?i)\bdd\b\s+[^\n]*\bof=/dev/`,
			`(?i)\bshred\b`,
			`(?i)\bwipefs\b`,
			`(?i)\bsgdisk\b[^\n]*--zap-all`,
		),
		mk(ruleMassPermissionChange, false,
			`(?i)\bchmod\b\s+(-R\s+)?(777|000)\s+/(\s|$)`,
			`(?i)\bchown\b\s+-R\s+\S+\s+/(\s|$)`,
			`(?i)\bicacls\b\s+[a-z]:\\[^\n]*/t\b`,
			`(?i)\btakeown\b[^\n]*/r\b`,
		),
		mk(ruleShutdownReboot, false,
			`(?i)\bshutdown\b`,
			`(?i)\breboot\b`,
			`(?i)\bpoweroff\b`,
			`(?i)\bhalt\b`,
			`(?i)\binit\s+[06]\b`,
			`(?i)Restart-Computer\b`,
			`(?i)Stop-Computer\b`,
		),
		mk(ruleHistoryCredentialDestruction, false,
			`(?i)\bhistory\s+(-c|--clear)\b`,
			`(?i)Clear-History\b`,

			`(?i)\brm\b[^\n]*(\.bash_history|\.zsh_history|_history\b)`,

			`(?i)\brm\b[^\n]*(~|/)\.ssh\b`,
			`(?i)\brm\b[^\n]*(~|/)\.gnupg\b`,
			`(?i)\bgit\s+filter-branch\b`,
			`(?i)\bcmdkey\b[^\n]*/delete:\*`,
		),
		mk(ruleCredentialExfiltration, false,

			`(?i)\b(curl|wget)\b[^\n]*(--data|-d\s|--upload-file|-T\s|--form|-F\s)[^\n]*(id_rsa|id_ed25519|\.aws/credentials|\.netrc|\.ssh/|\.env\b|credentials)`,
			`(?i)\b(curl|wget)\b[^\n]*(id_rsa|id_ed25519|\.aws/credentials|\.netrc|\.ssh/)[^\n]*(--data|-d\s|--upload-file|-T\s|--form|-F\s)`,

			`(?i)(cat|type|Get-Content)\b[^\n]*(id_rsa|id_ed25519|\.aws/credentials|\.netrc|\.ssh/|\.env\b)[^\n]*\|\s*(curl|wget|nc|Invoke-WebRequest|Invoke-RestMethod)`,
			`(?i)(Invoke-WebRequest|Invoke-RestMethod)\b[^\n]*-File\b[^\n]*(id_rsa|\.env\b|\.netrc|credentials)`,
		),
	}
	for i := range rules {
		if rules[i].Name == ruleRecursiveForceDelete {
			rules[i].match = recursiveForceDeleteMatch
		}
	}
	return rules
}

var blocklist = mustRules()

func MatchBlocklist(command string) (blockRule, bool) {
	if command == "" {
		return blockRule{}, false
	}
	for _, r := range blocklist {
		for _, p := range r.patterns {
			if p.MatchString(command) {
				return r, true
			}
		}
		if r.match != nil && r.match(command) {
			return r, true
		}
	}
	return blockRule{}, false
}

func recursiveForceDeleteMatch(command string) bool {
	for _, segment := range splitCommandSegments(command) {
		if recursiveForceTargetsRoot(segment) {
			return true
		}
	}
	return false
}

func splitCommandSegments(command string) []string {
	return strings.FieldsFunc(command, func(r rune) bool {
		switch r {
		case ';', '|', '&', '(', ')', '\n':
			return true
		}
		return false
	})
}

func isRmToken(token string) bool {
	base := token
	if idx := strings.LastIndexAny(base, `/\`); idx >= 0 {
		base = base[idx+1:]
	}
	return strings.EqualFold(base, "rm")
}

func recursiveForceTargetsRoot(segment string) bool {
	tokens := make([]string, 0, 8)
	for _, tok := range strings.Fields(segment) {
		tokens = append(tokens, stripQuotes(tok))
	}
	for i, tok := range tokens {
		if !isRmToken(tok) {
			continue
		}
		recursive, force := false, false
		var operands []string
		endOfFlags := false
		for _, arg := range tokens[i+1:] {
			switch {
			case !endOfFlags && arg == "--":
				endOfFlags = true
			case !endOfFlags && strings.HasPrefix(arg, "--"):
				switch strings.ToLower(strings.TrimPrefix(arg, "--")) {
				case "recursive":
					recursive = true
				case "force":
					force = true
				}
			case !endOfFlags && len(arg) > 1 && arg[0] == '-':
				for _, ch := range strings.ToLower(arg[1:]) {
					switch ch {
					case 'r':
						recursive = true
					case 'f':
						force = true
					}
				}
			default:
				operands = append(operands, arg)
			}
		}
		if recursive && force {
			for _, op := range operands {
				if isRootOrHomeTarget(op) {
					return true
				}
			}
		}
	}
	return false
}

func stripQuotes(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '"' || r == '\'' {
			return -1
		}
		return r
	}, s)
}

func isRootOrHomeTarget(arg string) bool {
	t := strings.TrimSuffix(arg, "/*")
	t = strings.TrimRight(t, `/\`)
	switch {
	case t == "" || t == "~":
		return true
	case strings.EqualFold(t, "$HOME") || strings.EqualFold(t, "${HOME}"):
		return true
	}
	return false
}
