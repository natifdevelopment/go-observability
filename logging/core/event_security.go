package core

// Default security events for structured security logging.
// These cover OWASP-recommended security events and common attack patterns.
// Additional events can be registered via EventRegistry.Register().

// Security event IDs (dot-notation for hierarchical querying in Loki/ELK/SIEM).
const (
	EventSecurityLoginFailed         = "security.auth.login_failed"
	EventSecurityUnauthorized        = "security.auth.unauthorized"
	EventSecurityForbidden           = "security.auth.forbidden"
	EventSecurityJWTInvalid          = "security.jwt.invalid"
	EventSecurityJWTExpired          = "security.jwt.expired"
	EventSecurityPermissionDenied    = "security.rbac.permission_denied"
	EventSecurityRateLimited         = "security.ratelimit.exceeded"
	EventSecurityBruteForce          = "security.ratelimit.brute_force"
	EventSecuritySQLInjection        = "security.injection.sql"
	EventSecurityXSS                 = "security.injection.xss"
	EventSecurityCSRF                = "security.injection.csrf"
	EventSecurityPrivilegeEscalation = "security.rbac.privilege_escalation"
	EventSecuritySensitiveDataAccess = "security.data.sensitive_access"
	EventSecurityVaultAccess         = "security.vault.access"
	EventSecurityAPIKeyUsage         = "security.apikey.usage"
	EventSecurityMalwareUpload       = "security.malware.upload"
)

// DefaultSecurityEvents returns the 16 default security event definitions.
func DefaultSecurityEvents() []EventMeta {
	return []EventMeta{
		{
			ID:          EventSecurityLoginFailed,
			Category:    EventCategorySecurity,
			Name:        "Login Failed",
			Description: "Authentication attempt failed",
			Severity:    LevelWarn,
			Fields:      []Field{FieldUsername, FieldIP, FieldAttemptCount},
		},
		{
			ID:          EventSecurityUnauthorized,
			Category:    EventCategorySecurity,
			Name:        "Unauthorized Access",
			Description: "Access attempted without authentication",
			Severity:    LevelWarn,
			Fields:      []Field{FieldIP, FieldPath, FieldMethod},
		},
		{
			ID:          EventSecurityForbidden,
			Category:    EventCategorySecurity,
			Name:        "Forbidden Access",
			Description: "Authenticated user denied access to resource",
			Severity:    LevelWarn,
			Fields:      []Field{FieldUserID, FieldRole, FieldIP, FieldPath},
		},
		{
			ID:          EventSecurityJWTInvalid,
			Category:    EventCategorySecurity,
			Name:        "Invalid JWT",
			Description: "JWT token validation failed (invalid signature/format)",
			Severity:    LevelError,
			Fields:      []Field{FieldIP, FieldMetadata},
		},
		{
			ID:          EventSecurityJWTExpired,
			Category:    EventCategorySecurity,
			Name:        "Expired JWT",
			Description: "JWT token has expired",
			Severity:    LevelWarn,
			Fields:      []Field{FieldIP, FieldMetadata},
		},
		{
			ID:          EventSecurityPermissionDenied,
			Category:    EventCategorySecurity,
			Name:        "Permission Denied",
			Description: "User lacks required permission for action",
			Severity:    LevelWarn,
			Fields:      []Field{FieldUserID, FieldRole, FieldPath, FieldMethod},
		},
		{
			ID:          EventSecurityRateLimited,
			Category:    EventCategorySecurity,
			Name:        "Rate Limit Exceeded",
			Description: "Client exceeded rate limit threshold",
			Severity:    LevelWarn,
			Fields:      []Field{FieldIP, FieldPath, FieldMetadata},
		},
		{
			ID:          EventSecurityBruteForce,
			Category:    EventCategorySecurity,
			Name:        "Brute Force Detected",
			Description: "Multiple failed authentication attempts detected",
			Severity:    LevelError,
			Fields:      []Field{FieldIP, FieldAttemptCount, FieldUsername},
		},
		{
			ID:          EventSecuritySQLInjection,
			Category:    EventCategorySecurity,
			Name:        "SQL Injection Detected",
			Description: "Potential SQL injection attack detected",
			Severity:    LevelError,
			Fields:      []Field{FieldIP, FieldDetectionType, FieldDetectionInput},
		},
		{
			ID:          EventSecurityXSS,
			Category:    EventCategorySecurity,
			Name:        "XSS Detected",
			Description: "Potential Cross-Site Scripting attack detected",
			Severity:    LevelError,
			Fields:      []Field{FieldIP, FieldDetectionType, FieldDetectionInput},
		},
		{
			ID:          EventSecurityCSRF,
			Category:    EventCategorySecurity,
			Name:        "CSRF Detected",
			Description: "Potential Cross-Site Request Forgery detected",
			Severity:    LevelError,
			Fields:      []Field{FieldIP, FieldDetectionType},
		},
		{
			ID:          EventSecurityPrivilegeEscalation,
			Category:    EventCategorySecurity,
			Name:        "Privilege Escalation",
			Description: "User attempted to escalate privileges",
			Severity:    LevelError,
			Fields:      []Field{FieldUserID, FieldRole, FieldMetadata},
		},
		{
			ID:          EventSecuritySensitiveDataAccess,
			Category:    EventCategorySecurity,
			Name:        "Sensitive Data Access",
			Description: "Access to sensitive data resource",
			Severity:    LevelInfo,
			Fields:      []Field{FieldUserID, FieldRole, FieldIP, FieldMetadata},
		},
		{
			ID:          EventSecurityVaultAccess,
			Category:    EventCategorySecurity,
			Name:        "Vault Access",
			Description: "Secret vault was accessed",
			Severity:    LevelInfo,
			Fields:      []Field{FieldUserID, FieldMetadata},
		},
		{
			ID:          EventSecurityAPIKeyUsage,
			Category:    EventCategorySecurity,
			Name:        "API Key Usage",
			Description: "API key was used for authentication",
			Severity:    LevelInfo,
			Fields:      []Field{FieldIP, FieldEndpoint, FieldMetadata},
		},
		{
			ID:          EventSecurityMalwareUpload,
			Category:    EventCategorySecurity,
			Name:        "Malware Upload Detected",
			Description: "Uploaded file flagged as potential malware",
			Severity:    LevelError,
			Fields:      []Field{FieldIP, FieldUserID, FieldMetadata},
		},
	}
}
