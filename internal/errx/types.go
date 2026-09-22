package errx

// Type classifies application errors into a closed set of semantic categories,
// each pre-mapped to an HTTP status code.
type Type string

const (
	TypeInternal      Type = "INTERNAL"
	TypeValidation    Type = "VALIDATION"
	TypeAuthorization Type = "AUTHORIZATION"
	TypeForbidden     Type = "FORBIDDEN"
	TypeNotFound      Type = "NOT_FOUND"
	TypeConflict      Type = "CONFLICT"
	TypeBusiness      Type = "BUSINESS"
	TypeRateLimited   Type = "RATE_LIMITED"
	TypeExternal      Type = "EXTERNAL"
)

func (t Type) String() string { return string(t) }
