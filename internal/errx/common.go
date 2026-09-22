package errx

// Semantic constructors — the only way domain/service code should create errors.

func Internal(message string) *Error    { return New(message, TypeInternal) }
func Validation(message string) *Error  { return New(message, TypeValidation) }
func NotFound(message string) *Error    { return New(message, TypeNotFound) }
func Unauthorized(message string) *Error { return New(message, TypeAuthorization) }

// Forbidden is distinct from Unauthorized: the caller IS authenticated but
// lacks permission.
func Forbidden(message string) *Error { return New(message, TypeForbidden) }

// RateLimited indicates the caller has exceeded their rate limit (HTTP 429).
func RateLimited(message string) *Error { return New(message, TypeRateLimited) }

func Conflict(message string) *Error { return New(message, TypeConflict) }
func Business(message string) *Error { return New(message, TypeBusiness) }
func External(message string) *Error { return New(message, TypeExternal) }
