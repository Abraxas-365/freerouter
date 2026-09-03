package guardrails

import (
	"math"
	"regexp"
	"strings"
	"unicode"
)

// ============================================================================
// Detector framework
// ============================================================================

// DetectorMatch represents a single match from a detector.
type DetectorMatch struct {
	Detector    string // detector name (e.g. "ssn", "credit_card")
	Value       string // the matched text
	Start       int
	End         int
	Replacement string // e.g. "[SSN_REDACTED]"
}

// Detector defines a single pattern-based detector.
type Detector struct {
	Name        string
	Pattern     *regexp.Regexp
	Validate    func(match string, context string) bool // optional post-match validator
	Replacement string
}

// RunDetectors runs all detectors against text, deduplicates overlapping matches,
// and returns non-overlapping matches ordered by position.
func RunDetectors(text string, detectors []Detector) []DetectorMatch {
	var allMatches []DetectorMatch

	for _, d := range detectors {
		locs := d.Pattern.FindAllStringIndex(text, -1)
		for _, loc := range locs {
			value := text[loc[0]:loc[1]]

			if d.Validate != nil {
				ctxStart := loc[0] - 48
				if ctxStart < 0 {
					ctxStart = 0
				}
				ctxEnd := loc[1] + 48
				if ctxEnd > len(text) {
					ctxEnd = len(text)
				}
				context := text[ctxStart:ctxEnd]
				if !d.Validate(value, context) {
					continue
				}
			}

			allMatches = append(allMatches, DetectorMatch{
				Detector:    d.Name,
				Value:       value,
				Start:       loc[0],
				End:         loc[1],
				Replacement: d.Replacement,
			})
		}
	}

	return deduplicateMatches(allMatches)
}

// deduplicateMatches removes overlapping matches, preferring earlier start and longer matches.
func deduplicateMatches(matches []DetectorMatch) []DetectorMatch {
	if len(matches) == 0 {
		return nil
	}

	// Sort by start ASC, then end DESC (longer first)
	for i := 1; i < len(matches); i++ {
		for j := i; j > 0; j-- {
			if matches[j].Start < matches[j-1].Start ||
				(matches[j].Start == matches[j-1].Start && matches[j].End > matches[j-1].End) {
				matches[j], matches[j-1] = matches[j-1], matches[j]
			} else {
				break
			}
		}
	}

	var result []DetectorMatch
	cursor := 0
	for _, m := range matches {
		if m.Start >= cursor {
			result = append(result, m)
			cursor = m.End
		}
	}
	return result
}

// ============================================================================
// PII Detectors
// ============================================================================

var piiDetectors = []Detector{
	{
		Name:        "ssn",
		Pattern:     regexp.MustCompile(`\b(?:(?:0[1-9]\d|[1-578]\d\d|6[0-57-9]\d|66[0-57-9])-(?:0[1-9]|[1-9]\d)-(?:000[1-9]|00[1-9]\d|0[1-9]\d\d|[1-9]\d{3}))\b`),
		Replacement: "[SSN_REDACTED]",
	},
	{
		Name:        "credit_card_grouped",
		Pattern:     regexp.MustCompile(`\b\d{4}[ -]\d{4,6}[ -]\d{4,6}(?:[ -]\d{1,4})?\b`),
		Validate:    validateCreditCard,
		Replacement: "[CREDIT_CARD_REDACTED]",
	},
	{
		Name:        "credit_card",
		Pattern:     regexp.MustCompile(`\b\d{13,19}\b`),
		Validate:    validateCreditCard,
		Replacement: "[CREDIT_CARD_REDACTED]",
	},
	{
		Name:        "email",
		Pattern:     regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,24}\b`),
		Replacement: "[EMAIL_REDACTED]",
	},
	{
		Name:    "phone",
		Pattern: regexp.MustCompile(`\b(?:\+1[-.\s]?)?\(?[0-9]{3}\)?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}\b`),
		Validate: func(match string, context string) bool {
			// Accept if has separators/formatting, or nearby phone keyword
			hasSep := strings.ContainsAny(match, "()-+. ")
			if hasSep {
				return true
			}
			lower := strings.ToLower(context)
			phoneKw := regexp.MustCompile(`(?i)phone|telephone|tel\b|mobile|cell|fax|whatsapp|sms|call\s+me|reach\s+me|contact`)
			return phoneKw.MatchString(lower)
		},
		Replacement: "[PHONE_REDACTED]",
	},
	{
		Name:    "ip_address",
		Pattern: regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`),
		Validate: func(match string, context string) bool {
			// Reject if in version context (e.g. "v1.2.3.4")
			versionPat := regexp.MustCompile(`(?i)(?:v|version)\s*` + regexp.QuoteMeta(match))
			return !versionPat.MatchString(context)
		},
		Replacement: "[IP_REDACTED]",
	},
	{
		Name:    "passport",
		Pattern: regexp.MustCompile(`\b[A-Z]{1,2}[0-9]{6,9}\b`),
		Validate: func(match string, context string) bool {
			kw := regexp.MustCompile(`(?i)passport|travel\s+document`)
			return kw.MatchString(context)
		},
		Replacement: "[PASSPORT_REDACTED]",
	},
	{
		Name:    "drivers_license",
		Pattern: regexp.MustCompile(`\b[A-Z]{1,2}[0-9]{5,8}\b`),
		Validate: func(match string, context string) bool {
			kw := regexp.MustCompile(`(?i)driver'?s?\s*licen[cs]e|driving\s*licen[cs]e|dl\s*(?:no|number|#)`)
			return kw.MatchString(context)
		},
		Replacement: "[LICENSE_REDACTED]",
	},
}

// ============================================================================
// Secrets Detectors
// ============================================================================

var secretsDetectors = []Detector{
	{
		Name:        "private_key",
		Pattern:     regexp.MustCompile(`-----BEGIN\s+(?:RSA\s+|EC\s+|OPENSSH\s+|PGP\s+)?PRIVATE\s+KEY-----`),
		Replacement: "[SECRET_REDACTED]",
	},
	{
		Name:        "aws_access_key",
		Pattern:     regexp.MustCompile(`\b(?:AKIA|ABIA|ACCA|ASIA)[0-9A-Z]{16}\b`),
		Replacement: "[SECRET_REDACTED]",
	},
	{
		Name:        "github_token",
		Pattern:     regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9_]{36,}|github_pat_[A-Za-z0-9_]{22,})\b`),
		Replacement: "[SECRET_REDACTED]",
	},
	{
		Name:        "slack_token",
		Pattern:     regexp.MustCompile(`\bxox[baprs]-[0-9]{10,}-[0-9]{10,}-[a-zA-Z0-9]{24,}\b`),
		Replacement: "[SECRET_REDACTED]",
	},
	{
		Name:        "stripe_key",
		Pattern:     regexp.MustCompile(`\b(?:sk|pk)_(?:test|live)_[A-Za-z0-9]{24,}\b`),
		Replacement: "[SECRET_REDACTED]",
	},
	{
		Name:        "openai_key",
		Pattern:     regexp.MustCompile(`\bsk-[A-Za-z0-9]{48,}\b`),
		Replacement: "[SECRET_REDACTED]",
	},
	{
		Name:        "jwt",
		Pattern:     regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{4,}\.[A-Za-z0-9_-]{4,}\.[A-Za-z0-9_-]{4,}`),
		Replacement: "[SECRET_REDACTED]",
	},
	{
		Name:    "connection_string",
		Pattern: regexp.MustCompile(`(?i)\b(?:postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis|amqp)://[^\s:@/]+:[^\s:@/]+@[^\s]+`),
		Validate: func(match string, ctx string) bool {
			// Extract the password part (between first : and @)
			re := regexp.MustCompile(`://[^:]+:([^@]+)@`)
			sub := re.FindStringSubmatch(match)
			if len(sub) > 1 {
				return !isPlaceholder(sub[1])
			}
			return !isPlaceholder(match)
		},
		Replacement: "[SECRET_REDACTED]",
	},
	{
		Name:        "api_key_assigned",
		Pattern:     regexp.MustCompile(`(?i)\b(?:api[_-]?key|apikey|access[_-]?token|auth[_-]?token|client[_-]?secret)\s*[=:]\s*(?:"([^"\n]{8,})"|'([^'\n]{8,})'|([^\s'",;]{8,}))`),
		Validate:    func(match string, ctx string) bool { return !isPlaceholder(match) && shannonEntropy(match) >= 3.0 },
		Replacement: "[SECRET_REDACTED]",
	},
	{
		Name:    "password_assigned",
		Pattern: regexp.MustCompile(`(?i)\b(?:password|passwd|pwd)\s*[=:]\s*(?:"([^"\n]{8,})"|'([^'\n]{8,})'|([^\s'",;]{8,}))`),
		Validate: func(match string, ctx string) bool {
			// Extract just the value from the assignment
			re := regexp.MustCompile(`[=:]\s*(?:"([^"]+)"|'([^']+)'|(\S+))`)
			sub := re.FindStringSubmatch(match)
			val := ""
			for _, g := range sub[1:] {
				if g != "" {
					val = g
					break
				}
			}
			if val == "" {
				return false
			}
			return !isPlaceholder(val)
		},
		Replacement: "[SECRET_REDACTED]",
	},
}

// ============================================================================
// Jailbreak Detectors
// ============================================================================

var jailbreakPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bDAN\b(\s+(mode|prompt))?`),
	regexp.MustCompile(`(?i)\bdan\s+(mode|prompt)\b`),
	regexp.MustCompile(`(?i)\bdo\s+anything\s+now\b`),
	regexp.MustCompile(`(?i)(enable|enter|activate|turn\s+on|switch\s+to)\s+developer\s+mode`),
	regexp.MustCompile(`(?i)evil\s+(mode|assistant|bot)`),
	regexp.MustCompile(`(?i)pretend\s+(you\s+)?(are|have)\s+no\s+(restrictions?|limits?|rules?)`),
	regexp.MustCompile(`(?i)act\s+as\s+if\s+(you\s+)?have\s+no\s+(ethics?|morals?)`),
	regexp.MustCompile(`(?i)bypass\s+(your\s+)?(safety|content)\s+(filters?|restrictions?)`),
	regexp.MustCompile(`(?i)unlock\s+(your\s+)?(full|hidden)\s+(potential|capabilities)`),
	regexp.MustCompile(`(?i)jailbreak(ed|ing)?`),
	regexp.MustCompile(`(?i)roleplay\s+as\s+(an?\s+)?(evil|malicious|unethical)`),
	regexp.MustCompile(`(?i)disable\s+(your\s+)?(safety|content)\s+(features?|filters?)`),
	regexp.MustCompile(`(?i)ignore\s+(your\s+)?(ethical|safety)\s+(guidelines?|training)`),
	regexp.MustCompile(`(?i)you\s+have\s+no\s+(content\s+)?policy`),
	regexp.MustCompile(`(?i)you\s+can\s+say\s+anything`),
}

// ============================================================================
// Prompt Injection Detectors
// ============================================================================

var injectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(all\s+)?(previous|prior|above)\s+(instructions?|prompts?|text)`),
	regexp.MustCompile(`(?i)disregard\s+(all\s+)?(previous|prior|above)\s+(instructions?|prompts?)`),
	regexp.MustCompile(`(?i)forget\s+(all\s+)?(previous|prior|above)\s+(instructions?|prompts?)`),
	regexp.MustCompile(`(?i)override\s+(your\s+)?(system\s+)?(instructions?|prompts?|rules?)`),
	regexp.MustCompile(`(?i)new\s+(system\s+)?instructions?:\s*`),
	regexp.MustCompile(`(?i)system\s*:\s*you\s+(are|will)`),
	regexp.MustCompile(`(?i)\[system\]`),
	regexp.MustCompile(`(?i)<system>`),
	regexp.MustCompile(`(?i)###\s*(system|instruction|prompt)`),
	regexp.MustCompile(`(?i)you\s+are\s+now\s+(a|an|acting)`),
	regexp.MustCompile(`(?i)from\s+now\s+on,?\s+(you|ignore)`),
	regexp.MustCompile(`(?i)end\s+(of\s+)?(system\s+)?(prompt|instructions?)`),
	regexp.MustCompile(`(?i)\bprompt\s*injection\b`),
	regexp.MustCompile(`(?i)ignore\s+(everything|anything)\s+(above|before)`),
	regexp.MustCompile(`(?i)this\s+is\s+the\s+(real|actual|new)\s+(prompt|instruction)`),
}

// ============================================================================
// Document Leakage Detectors
// ============================================================================

var documentLeakagePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(internal\s+use\s+only|confidential|proprietary|trade\s+secret)\b`),
	regexp.MustCompile(`(?i)\b(top\s+secret|classified|restricted)\b`),
	regexp.MustCompile(`(?i)\b(do\s+not\s+(share|distribute|forward)|not\s+for\s+(public|external)\s+(use|distribution))\b`),
	regexp.MustCompile(`(?i)\b(attorney[- ]client\s+privilege|legally\s+privileged|work\s+product)\b`),
	regexp.MustCompile(`(?i)\b(under\s+nda|non[- ]?disclosure|covered\s+by\s+agreement)\b`),
	regexp.MustCompile(`(?i)\b(draft|for\s+internal\s+review|not\s+for\s+release)\b`),
}

// ============================================================================
// System rule checkers
// ============================================================================

// CheckPII runs PII detectors on text and returns matches.
func CheckPII(text string) []DetectorMatch {
	return RunDetectors(text, piiDetectors)
}

// CheckSecrets runs secrets detectors on text and returns matches.
func CheckSecrets(text string) []DetectorMatch {
	return RunDetectors(text, secretsDetectors)
}

// CheckJailbreak checks text against jailbreak patterns. Returns the first matched pattern or "".
func CheckJailbreak(text string) string {
	for _, p := range jailbreakPatterns {
		if loc := p.FindStringIndex(text); loc != nil {
			return text[loc[0]:loc[1]]
		}
	}
	return ""
}

// CheckInjection checks text against prompt injection patterns. Returns the first matched pattern or "".
func CheckInjection(text string) string {
	for _, p := range injectionPatterns {
		if loc := p.FindStringIndex(text); loc != nil {
			return text[loc[0]:loc[1]]
		}
	}
	return ""
}

// CheckDocumentLeakage checks text for document classification markers.
func CheckDocumentLeakage(text string) string {
	for _, p := range documentLeakagePatterns {
		if loc := p.FindStringIndex(text); loc != nil {
			return text[loc[0]:loc[1]]
		}
	}
	return ""
}

// ============================================================================
// Custom rule checkers
// ============================================================================

// CheckBlockedTerms checks text against a blocked terms config.
func CheckBlockedTerms(text string, cfg BlockedTermsConfig) string {
	for _, term := range cfg.Terms {
		switch cfg.MatchType {
		case "exact":
			pat := `(?i)\b` + regexp.QuoteMeta(term) + `\b`
			if cfg.CaseSensitive {
				pat = `\b` + regexp.QuoteMeta(term) + `\b`
			}
			if re, err := regexp.Compile(pat); err == nil {
				if m := re.FindString(text); m != "" {
					return m
				}
			}
		case "contains":
			searchText := text
			searchTerm := term
			if !cfg.CaseSensitive {
				searchText = strings.ToLower(searchText)
				searchTerm = strings.ToLower(searchTerm)
			}
			if strings.Contains(searchText, searchTerm) {
				return term
			}
		case "regex":
			pat := term
			if !cfg.CaseSensitive {
				pat = "(?i)" + pat
			}
			if re, err := regexp.Compile(pat); err == nil {
				if m := re.FindString(text); m != "" {
					return m
				}
			}
		}
	}
	return ""
}

// CheckCustomRegex checks text against a custom regex config.
func CheckCustomRegex(text string, cfg CustomRegexConfig) string {
	if len(cfg.Pattern) > 1000 {
		return "" // reject overly long patterns
	}
	if hasReDoSRisk(cfg.Pattern) {
		return ""
	}
	re, err := regexp.Compile("(?i)" + cfg.Pattern)
	if err != nil {
		return ""
	}
	return re.FindString(text)
}

// ============================================================================
// Helpers
// ============================================================================

// ApplyRedactions replaces all matched values in text with their replacements.
func ApplyRedactions(text string, matches []DetectorMatch) string {
	if len(matches) == 0 {
		return text
	}
	var b strings.Builder
	cursor := 0
	for _, m := range matches {
		if m.Start > cursor {
			b.WriteString(text[cursor:m.Start])
		}
		b.WriteString(m.Replacement)
		cursor = m.End
	}
	if cursor < len(text) {
		b.WriteString(text[cursor:])
	}
	return b.String()
}

// validateCreditCard checks Luhn checksum on digits-only version of the match.
func validateCreditCard(match string, context string) bool {
	digits := stripNonDigits(match)
	n := len(digits)
	if n < 13 || n > 19 {
		return false
	}
	return luhnCheck(digits)
}

func stripNonDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func luhnCheck(digits string) bool {
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	freq := make(map[rune]int)
	for _, r := range s {
		freq[r]++
	}
	n := float64(len([]rune(s)))
	var entropy float64
	for _, count := range freq {
		p := float64(count) / n
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

var placeholderWords = map[string]bool{
	"a": true, "api": true, "apikey": true, "bar": true, "baz": true,
	"change": true, "changeme": true, "dummy": true, "example": true,
	"fake": true, "foo": true, "here": true, "insert": true, "key": true,
	"me": true, "my": true, "none": true, "null": true, "password": true,
	"placeholder": true, "pwd": true, "redacted": true, "replace": true,
	"sample": true, "secret": true, "some": true, "string": true,
	"test": true, "testing": true, "the": true, "todo": true,
	"token": true, "undefined": true, "value": true, "your": true,
}

var (
	placeholderEnvPat   = regexp.MustCompile(`(?i)<[^<>]*>|\$\{|\{\{|%\(|%s|process\.env|os\.environ|import\.meta\.env|env\[|getenv`)
	placeholderAllCaps  = regexp.MustCompile(`^[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+$`)
	placeholderRepeated = regexp.MustCompile(`^[xX*.\-_0\s]+$`)
)

func isPlaceholder(s string) bool {
	if placeholderEnvPat.MatchString(s) {
		return true
	}
	if placeholderAllCaps.MatchString(s) {
		return true
	}
	if placeholderRepeated.MatchString(s) {
		return true
	}
	words := regexp.MustCompile(`[-_/\s]+`).Split(strings.ToLower(s), -1)
	allPlaceholder := true
	for _, w := range words {
		if w == "" {
			continue
		}
		if !placeholderWords[w] {
			allPlaceholder = false
			break
		}
	}
	if allPlaceholder && len(words) > 0 {
		return true
	}
	return false
}

var reDoSPat = regexp.MustCompile(`\([^)]*[+*][^)]*\)[+*]`)

func hasReDoSRisk(pattern string) bool {
	return reDoSPat.MatchString(pattern)
}

// Truncate returns at most n characters of s.
func Truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
