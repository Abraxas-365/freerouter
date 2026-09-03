package scopes

// ============================================================================
// COMMON SCOPES - Reusable across any project
// ============================================================================

const (
	// Super scope - full access to everything
	PlatformAdmin = "*"

	// Admin scopes

	ScopeAdminRead  = "admin:read"
	ScopeAdminWrite = "admin:write"

	// User management scopes
	ScopeUsersAll    = "users:*"
	ScopeUsersRead   = "users:read"
	ScopeUsersWrite  = "users:write"
	ScopeUsersDelete = "users:delete"

	// Role management scopes
	ScopeRolesAll    = "roles:*"
	ScopeRolesRead   = "roles:read"
	ScopeRolesWrite  = "roles:write"
	ScopeRolesDelete = "roles:delete"
	ScopeRolesAssign = "roles:assign"

	// Scope management scopes
	ScopeScopesAll    = "scopes:*"
	ScopeScopesRead   = "scopes:read"
	ScopeScopesWrite  = "scopes:write"
	ScopeScopesAssign = "scopes:assign"

	// API Key scopes
	ScopeAPIKeysAll    = "api_keys:*"
	ScopeAPIKeysRead   = "api_keys:read"
	ScopeAPIKeysWrite  = "api_keys:write"
	ScopeAPIKeysDelete = "api_keys:delete"
	ScopeAPIKeysRevoke = "api_keys:revoke"

	// Invitation scopes
	ScopeInvitationsAll    = "invitations:*"
	ScopeInvitationsRead   = "invitations:read"
	ScopeInvitationsWrite  = "invitations:write"
	ScopeInvitationsDelete = "invitations:delete"
	ScopeInvitationsRevoke = "invitations:revoke"

	// Settings scopes
	ScopeSettingsAll   = "settings:*"
	ScopeSettingsRead  = "settings:read"
	ScopeSettingsWrite = "settings:write"

	// Audit log scopes
	ScopeAuditAll  = "audit:*"
	ScopeAuditRead = "audit:read"

	// Reports/Analytics scopes (generic)
	ScopeReportsAll         = "reports:*"
	ScopeReportsView        = "reports:view"
	ScopeReportsExport      = "reports:export"
	ScopeReportsCreate      = "reports:create"
	ScopeAnalyticsDashboard = "analytics:dashboard"

	// Integration scopes (generic for any external integrations)
	ScopeIntegrationsAll    = "integrations:*"
	ScopeIntegrationsRead   = "integrations:read"
	ScopeIntegrationsWrite  = "integrations:write"
	ScopeIntegrationsDelete = "integrations:delete"
	ScopeIntegrationsTest   = "integrations:test"

	// Notification scopes
	ScopeNotificationsAll   = "notifications:*"
	ScopeNotificationsRead  = "notifications:read"
	ScopeNotificationsSend  = "notifications:send"
	ScopeNotificationsWrite = "notifications:write"

	// Template scopes (generic templates)
	ScopeTemplatesAll    = "templates:*"
	ScopeTemplatesRead   = "templates:read"
	ScopeTemplatesWrite  = "templates:write"
	ScopeTemplatesDelete = "templates:delete"
)

// CommonScopeCategories organizes common scopes by domain
var CommonScopeCategories = map[string][]string{
	"Administration": {
		PlatformAdmin,

		ScopeAdminRead,
		ScopeAdminWrite,
	},
	"Users": {
		ScopeUsersAll,
		ScopeUsersRead,
		ScopeUsersWrite,
		ScopeUsersDelete,
	},
	"Roles": {
		ScopeRolesAll,
		ScopeRolesRead,
		ScopeRolesWrite,
		ScopeRolesDelete,
		ScopeRolesAssign,
	},
	"Scopes": {
		ScopeScopesAll,
		ScopeScopesRead,
		ScopeScopesWrite,
		ScopeScopesAssign,
	},
	"API Keys": {
		ScopeAPIKeysAll,
		ScopeAPIKeysRead,
		ScopeAPIKeysWrite,
		ScopeAPIKeysDelete,
		ScopeAPIKeysRevoke,
	},
	"Invitations": {
		ScopeInvitationsAll,
		ScopeInvitationsRead,
		ScopeInvitationsWrite,
		ScopeInvitationsDelete,
		ScopeInvitationsRevoke,
	},
	"Settings": {
		ScopeSettingsAll,
		ScopeSettingsRead,
		ScopeSettingsWrite,
	},
	"Audit": {
		ScopeAuditAll,
		ScopeAuditRead,
	},
	"Reports & Analytics": {
		ScopeReportsAll,
		ScopeReportsView,
		ScopeReportsExport,
		ScopeReportsCreate,
		ScopeAnalyticsDashboard,
	},
	"Integrations": {
		ScopeIntegrationsAll,
		ScopeIntegrationsRead,
		ScopeIntegrationsWrite,
		ScopeIntegrationsDelete,
		ScopeIntegrationsTest,
	},
	"Notifications": {
		ScopeNotificationsAll,
		ScopeNotificationsRead,
		ScopeNotificationsSend,
		ScopeNotificationsWrite,
	},
	"Templates": {
		ScopeTemplatesAll,
		ScopeTemplatesRead,
		ScopeTemplatesWrite,
		ScopeTemplatesDelete,
	},
}

// CommonScopeDescriptions provides human-readable descriptions
var CommonScopeDescriptions = map[string]string{
	// Super admin
	PlatformAdmin: "Full access to all system resources",

	// Admin

	ScopeAdminRead:  "View administrative settings",
	ScopeAdminWrite: "Modify administrative settings",

	// Users
	ScopeUsersAll:    "Full access to user management",
	ScopeUsersRead:   "View users",
	ScopeUsersWrite:  "Create and edit users",
	ScopeUsersDelete: "Delete users",

	// Roles
	ScopeRolesAll:    "Full access to role management",
	ScopeRolesRead:   "View roles",
	ScopeRolesWrite:  "Create and edit roles",
	ScopeRolesDelete: "Delete roles",
	ScopeRolesAssign: "Assign roles to users",

	// Scopes
	ScopeScopesAll:    "Full access to scope management",
	ScopeScopesRead:   "View available scopes and user scopes",
	ScopeScopesWrite:  "Set and modify user scopes",
	ScopeScopesAssign: "Add or remove scopes from users",

	// API Keys
	ScopeAPIKeysAll:    "Full access to API key management",
	ScopeAPIKeysRead:   "View API keys",
	ScopeAPIKeysWrite:  "Create and edit API keys",
	ScopeAPIKeysDelete: "Delete API keys",
	ScopeAPIKeysRevoke: "Revoke API keys",

	// Invitations
	ScopeInvitationsAll:    "Full access to invitation management",
	ScopeInvitationsRead:   "View invitations",
	ScopeInvitationsWrite:  "Create invitations",
	ScopeInvitationsDelete: "Delete invitations",
	ScopeInvitationsRevoke: "Revoke invitations",

	// Settings
	ScopeSettingsAll:   "Full access to settings",
	ScopeSettingsRead:  "View settings",
	ScopeSettingsWrite: "Modify settings",

	// Audit
	ScopeAuditAll:  "Full access to audit logs",
	ScopeAuditRead: "View audit logs",

	// Reports
	ScopeReportsAll:         "Full access to reporting",
	ScopeReportsView:        "View reports",
	ScopeReportsExport:      "Export reports",
	ScopeReportsCreate:      "Create custom reports",
	ScopeAnalyticsDashboard: "Access analytics dashboard",

	// Integrations
	ScopeIntegrationsAll:    "Full access to integrations",
	ScopeIntegrationsRead:   "View integrations",
	ScopeIntegrationsWrite:  "Create and edit integrations",
	ScopeIntegrationsDelete: "Delete integrations",
	ScopeIntegrationsTest:   "Test integrations",

	// Notifications
	ScopeNotificationsAll:   "Full access to notifications",
	ScopeNotificationsRead:  "View notifications",
	ScopeNotificationsSend:  "Send notifications",
	ScopeNotificationsWrite: "Configure notifications",

	// Templates
	ScopeTemplatesAll:    "Full access to templates",
	ScopeTemplatesRead:   "View templates",
	ScopeTemplatesWrite:  "Create and edit templates",
	ScopeTemplatesDelete: "Delete templates",
}
