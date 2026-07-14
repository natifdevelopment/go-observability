package core

// Default business events for structured business logging.
// These cover common business operations across enterprise applications.
// Additional events can be registered via EventRegistry.Register().

// Business event IDs (dot-notation for hierarchical querying in Loki/ELK).
const (
	EventLoginSuccess    = "user.login.success"
	EventLoginFailure    = "user.login.failure"
	EventLogout          = "user.logout"
	EventRegister        = "user.register"
	EventCreate          = "entity.create"
	EventUpdate          = "entity.update"
	EventDelete          = "entity.delete"
	EventApprove         = "workflow.approve"
	EventReject          = "workflow.reject"
	EventPayment         = "transaction.payment"
	EventTransfer        = "transaction.transfer"
	EventImport          = "data.import"
	EventExport          = "data.export"
	EventUpload          = "file.upload"
	EventDownload        = "file.download"
)

// DefaultBusinessEvents returns the 14 default business event definitions.
func DefaultBusinessEvents() []EventMeta {
	return []EventMeta{
		{
			ID:          EventLoginSuccess,
			Category:    EventCategoryBusiness,
			Name:        "User Login Success",
			Description: "User successfully authenticated",
			Severity:    LevelInfo,
			Fields:      []Field{FieldUserID, FieldUsername, FieldIP, FieldSessionID},
		},
		{
			ID:          EventLoginFailure,
			Category:    EventCategoryBusiness,
			Name:        "User Login Failure",
			Description: "User authentication failed",
			Severity:    LevelWarn,
			Fields:      []Field{FieldUsername, FieldIP},
		},
		{
			ID:          EventLogout,
			Category:    EventCategoryBusiness,
			Name:        "User Logout",
			Description: "User logged out",
			Severity:    LevelInfo,
			Fields:      []Field{FieldUserID, FieldUsername, FieldSessionID},
		},
		{
			ID:          EventRegister,
			Category:    EventCategoryBusiness,
			Name:        "User Register",
			Description: "New user registered",
			Severity:    LevelInfo,
			Fields:      []Field{FieldUserID, FieldUsername, FieldIP},
		},
		{
			ID:          EventCreate,
			Category:    EventCategoryBusiness,
			Name:        "Entity Create",
			Description: "A new entity was created",
			Severity:    LevelInfo,
			Fields:      []Field{FieldUserID, FieldMetadata},
		},
		{
			ID:          EventUpdate,
			Category:    EventCategoryBusiness,
			Name:        "Entity Update",
			Description: "An entity was updated",
			Severity:    LevelInfo,
			Fields:      []Field{FieldUserID, FieldMetadata},
		},
		{
			ID:          EventDelete,
			Category:    EventCategoryBusiness,
			Name:        "Entity Delete",
			Description: "An entity was deleted",
			Severity:    LevelWarn,
			Fields:      []Field{FieldUserID, FieldMetadata},
		},
		{
			ID:          EventApprove,
			Category:    EventCategoryBusiness,
			Name:        "Workflow Approve",
			Description: "A workflow item was approved",
			Severity:    LevelInfo,
			Fields:      []Field{FieldUserID, FieldRole, FieldMetadata},
		},
		{
			ID:          EventReject,
			Category:    EventCategoryBusiness,
			Name:        "Workflow Reject",
			Description: "A workflow item was rejected",
			Severity:    LevelInfo,
			Fields:      []Field{FieldUserID, FieldRole, FieldMetadata},
		},
		{
			ID:          EventPayment,
			Category:    EventCategoryBusiness,
			Name:        "Payment",
			Description: "A payment was processed",
			Severity:    LevelInfo,
			Fields:      []Field{FieldUserID, FieldMetadata},
		},
		{
			ID:          EventTransfer,
			Category:    EventCategoryBusiness,
			Name:        "Transfer",
			Description: "A fund transfer was processed",
			Severity:    LevelInfo,
			Fields:      []Field{FieldUserID, FieldMetadata},
		},
		{
			ID:          EventImport,
			Category:    EventCategoryBusiness,
			Name:        "Data Import",
			Description: "Data was imported",
			Severity:    LevelInfo,
			Fields:      []Field{FieldUserID, FieldMetadata},
		},
		{
			ID:          EventExport,
			Category:    EventCategoryBusiness,
			Name:        "Data Export",
			Description: "Data was exported",
			Severity:    LevelInfo,
			Fields:      []Field{FieldUserID, FieldMetadata},
		},
		{
			ID:          EventUpload,
			Category:    EventCategoryBusiness,
			Name:        "File Upload",
			Description: "A file was uploaded",
			Severity:    LevelInfo,
			Fields:      []Field{FieldUserID, FieldMetadata},
		},
		{
			ID:          EventDownload,
			Category:    EventCategoryBusiness,
			Name:        "File Download",
			Description: "A file was downloaded",
			Severity:    LevelInfo,
			Fields:      []Field{FieldUserID, FieldMetadata},
		},
	}
}
