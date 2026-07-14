package core

// MaskCategory categorizes sensitive data types for automatic masking.
type MaskCategory string

const (
	MaskPassword     MaskCategory = "password"
	MaskPIN          MaskCategory = "pin"
	MaskOTP          MaskCategory = "otp"
	MaskJWT          MaskCategory = "jwt"
	MaskAccessToken  MaskCategory = "access_token"
	MaskRefreshToken MaskCategory = "refresh_token"
	MaskSecret       MaskCategory = "secret"
	MaskVaultSecret  MaskCategory = "vault_secret"
	MaskAPIKey       MaskCategory = "api_key"
	MaskCreditCard   MaskCategory = "credit_card"
	MaskCVV          MaskCategory = "cvv"
	MaskNIK          MaskCategory = "nik"
	MaskNPWP         MaskCategory = "npwp"
	MaskEmail        MaskCategory = "email"
	MaskPhoneNumber  MaskCategory = "phone"
)

// MaskStrategy determines how a value is masked.
type MaskStrategy int

const (
	// MaskStrategyFull replaces the entire value with mask chars: "****".
	MaskStrategyFull MaskStrategy = iota
	// MaskStrategyPartial preserves first/last N chars: "ab***yz".
	MaskStrategyPartial
	// MaskStrategyHash replaces with SHA256 prefix (for audit without exposure).
	MaskStrategyHash
	// MaskStrategyDrop removes the field entirely from the log record.
	// Used for fields that must NEVER appear in logs (e.g., CVV).
	MaskStrategyDrop
)

// MaskPattern defines a single masking rule.
type MaskPattern struct {
	Name         MaskCategory
	KeyMatch     []string    // case-insensitive substring match in field name
	ValueRegex   string      // optional regex for value-based detection
	MaskStrategy MaskStrategy
	PreserveFirst int        // chars to preserve at start (for Partial)
	PreserveLast  int        // chars to preserve at end (for Partial)
}

// MaskConfig configures the MaskEngine.
//
// Security defaults (applied by NewDefaultMaskConfig):
//   - Enabled: true (fail-secure)
//   - All categories: true
//   - MaskChar: "*"
//   - CVV: MaskStrategyDrop (never log CVV)
//   - Email/Phone: MaskStrategyPartial (preserve first/last for debugging)
type MaskConfig struct {
	Enabled        bool                    `json:"enabled"`
	Categories     map[MaskCategory]bool   `json:"categories"`
	MaskChar       string                  `json:"mask_char"`
	PreserveFirst  int                     `json:"preserve_first"`
	PreserveLast   int                     `json:"preserve_last"`
	CustomPatterns map[string]MaskPattern  `json:"custom_patterns,omitempty"`
}

// NewDefaultMaskConfig returns a MaskConfig with security-first defaults.
// All categories enabled, CVV dropped, email/phone partial-masked.
func NewDefaultMaskConfig() MaskConfig {
	categories := make(map[MaskCategory]bool, 15)
	for _, c := range AllMaskCategories() {
		categories[c] = true
	}
	return MaskConfig{
		Enabled:       true,
		Categories:    categories,
		MaskChar:      DefaultMaskChar,
		PreserveFirst: 2,
		PreserveLast:  2,
	}
}

// NewDisabledMaskConfig returns a MaskConfig with masking disabled.
// WARNING: disabling masking is a security risk. Only use in trusted
// development environments with non-production data.
func NewDisabledMaskConfig() MaskConfig {
	return MaskConfig{
		Enabled:    false,
		Categories: make(map[MaskCategory]bool),
		MaskChar:   DefaultMaskChar,
	}
}

// AllMaskCategories returns all predefined mask categories.
func AllMaskCategories() []MaskCategory {
	return []MaskCategory{
		MaskPassword,
		MaskPIN,
		MaskOTP,
		MaskJWT,
		MaskAccessToken,
		MaskRefreshToken,
		MaskSecret,
		MaskVaultSecret,
		MaskAPIKey,
		MaskCreditCard,
		MaskCVV,
		MaskNIK,
		MaskNPWP,
		MaskEmail,
		MaskPhoneNumber,
	}
}

// IsEnabled reports whether a specific category is enabled for masking.
func (c MaskConfig) IsEnabled(cat MaskCategory) bool {
	if !c.Enabled {
		return false
	}
	enabled, ok := c.Categories[cat]
	if !ok {
		// Fail-secure: if category not in map, default to enabled.
		return true
	}
	return enabled
}

// defaultKeyPatterns maps field name substrings to mask categories.
// Case-insensitive matching is performed by the MaskEngine.
var defaultKeyPatterns = map[MaskCategory][]string{
	MaskPassword:     {"password", "passwd", "pwd", "pass"},
	MaskPIN:          {"pin", "pin_code"},
	MaskOTP:          {"otp", "one_time_password", "verification_code"},
	MaskJWT:          {"jwt", "token_id", "id_token"},
	MaskAccessToken:  {"access_token", "accesstoken", "bearer"},
	MaskRefreshToken: {"refresh_token", "refreshtoken"},
	MaskSecret:       {"secret", "private_key", "privatekey", "signing_key"},
	MaskVaultSecret:  {"vault_secret", "vault_token", "vault_data"},
	MaskAPIKey:       {"api_key", "apikey", "x_api_key", "client_secret"},
	MaskCreditCard:   {"credit_card", "creditcard", "card_number", "cardnumber", "cc_number", "pan"},
	MaskCVV:          {"cvv", "cvc", "card_verification", "security_code"},
	MaskNIK:          {"nik", "id_card", "idcard", "national_id"},
	MaskNPWP:         {"npwp", "tax_id", "taxid", "tax_number"},
	MaskEmail:        {"email", "mail", "email_address"},
	MaskPhoneNumber:  {"phone", "mobile", "tel", "phone_number", "phonenumber", "msisdn"},
}

// KeyPatternsFor returns the default key-substring patterns for a category.
func KeyPatternsFor(cat MaskCategory) []string {
	return defaultKeyPatterns[cat]
}
