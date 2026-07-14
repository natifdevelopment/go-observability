package core

import (
	"regexp"
	"sync"
)

// Precompiled regex patterns for value-based detection.
// These detect sensitive values even when the field name is not obvious
// (e.g., a field named "data" containing a JWT token).
//
// All patterns are compiled once at init time (no per-call compilation).
// Thread-safe (regexp.Regexp is safe for concurrent use).

var (
	// creditCardRegex matches 13-19 digit numbers with optional spaces/dashes.
	// Does NOT validate Luhn (that's done separately in MaskEngine).
	creditCardRegex = regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`)

	// jwtRegex matches JWT tokens (3 base64url segments separated by dots).
	jwtRegex = regexp.MustCompile(`^eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`)

	// emailRegex matches standard email addresses.
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

	// phoneRegex matches international phone numbers: +62..., +1..., etc.
	phoneRegex = regexp.MustCompile(`^\+?[1-9]\d{6,14}$`)

	// nikRegex matches Indonesian NIK (16 digits).
	nikRegex = regexp.MustCompile(`^\d{16}$`)

	// npwpRegex matches Indonesian NPWP (XX.XXX.XXX.X-XXX.XXX).
	npwpRegex = regexp.MustCompile(`^\d{2}\.\d{3}\.\d{3}\.\d-\d{3}\.\d{3}$`)

	// apiKeyRegex matches common API key formats (32+ hex or base64 chars).
	apiKeyRegex = regexp.MustCompile(`^[a-fA-F0-9]{32,}$|^[A-Za-z0-9+/]{40,}={0,2}$`)
)

// patternRegistry holds compiled value-detection regexes per category.
type patternRegistry struct {
	patterns map[MaskCategory]*regexp.Regexp
}

var valuePatterns = patternRegistry{
	patterns: map[MaskCategory]*regexp.Regexp{
		MaskCreditCard:  creditCardRegex,
		MaskJWT:         jwtRegex,
		MaskEmail:       emailRegex,
		MaskPhoneNumber: phoneRegex,
		MaskNIK:         nikRegex,
		MaskNPWP:        npwpRegex,
		MaskAPIKey:      apiKeyRegex,
	},
}

// compiledCustomPatterns caches custom regex patterns by their source string.
// Avoids recompiling the same pattern on every MaskEngine construction.
var customPatternCache sync.Map // map[string]*regexp.Regexp

// compilePattern compiles a regex string and caches the result.
// Returns nil if the pattern is empty or invalid (invalid patterns are
// silently skipped to avoid breaking logging on config errors).
func compilePattern(pattern string) *regexp.Regexp {
	if pattern == "" {
		return nil
	}
	if cached, ok := customPatternCache.Load(pattern); ok {
		return cached.(*regexp.Regexp)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	customPatternCache.Store(pattern, re)
	return re
}

// luhnCheck validates a credit card number using the Luhn algorithm.
// Returns true if the number passes Luhn (likely a real card number).
// Used to reduce false positives in credit card detection.
func luhnCheck(number string) bool {
	// Strip non-digits.
	digits := make([]byte, 0, len(number))
	for i := 0; i < len(number); i++ {
		c := number[i]
		if c >= '0' && c <= '9' {
			digits = append(digits, c)
		}
	}
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}

	sum := 0
	alternate := false
	for i := len(digits) - 1; i >= 0; i-- {
		n := int(digits[i] - '0')
		if alternate {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alternate = !alternate
	}
	return sum%10 == 0
}

// ValueMatchesPattern checks if a value matches the detection regex for a category.
// For credit cards, also performs Luhn check to reduce false positives.
func ValueMatchesPattern(cat MaskCategory, value string) bool {
	re, ok := valuePatterns.patterns[cat]
	if !ok || re == nil {
		return false
	}
	if !re.MatchString(value) {
		return false
	}
	if cat == MaskCreditCard {
		return luhnCheck(value)
	}
	return true
}

// DetectCategoryByValue attempts to detect the mask category of a value
// by matching against all value patterns. Returns the category and true
// if a match is found, or "" and false otherwise.
//
// This is used as a secondary detection mechanism when the field name
// does not match any key pattern.
func DetectCategoryByValue(value string) (MaskCategory, bool) {
	// Check JWT first (most specific pattern).
	if ValueMatchesPattern(MaskJWT, value) {
		return MaskJWT, true
	}
	if ValueMatchesPattern(MaskEmail, value) {
		return MaskEmail, true
	}
	if ValueMatchesPattern(MaskNPWP, value) {
		return MaskNPWP, true
	}
	if ValueMatchesPattern(MaskNIK, value) {
		return MaskNIK, true
	}
	if ValueMatchesPattern(MaskPhoneNumber, value) {
		return MaskPhoneNumber, true
	}
	if ValueMatchesPattern(MaskCreditCard, value) {
		return MaskCreditCard, true
	}
	if ValueMatchesPattern(MaskAPIKey, value) {
		return MaskAPIKey, true
	}
	return "", false
}
