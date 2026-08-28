package kernel

type UserID string

func NewUserID(id string) UserID { return UserID(id) }
func (u UserID) String() string  { return string(u) }
func (u UserID) IsEmpty() bool   { return string(u) == "" }

type TenantID string

func NewTenantID(id string) TenantID { return TenantID(id) }
func (t TenantID) String() string    { return string(t) }
func (t TenantID) IsEmpty() bool     { return string(t) == "" }

type ProviderID string

func NewProviderID(id string) ProviderID { return ProviderID(id) }
func (p ProviderID) String() string      { return string(p) }
func (p ProviderID) IsEmpty() bool       { return string(p) == "" }

type ModelID string

func NewModelID(id string) ModelID { return ModelID(id) }
func (m ModelID) String() string   { return string(m) }
func (m ModelID) IsEmpty() bool    { return string(m) == "" }

type MappingID string

func NewMappingID(id string) MappingID { return MappingID(id) }
func (m MappingID) String() string     { return string(m) }
func (m MappingID) IsEmpty() bool      { return string(m) == "" }

type ProviderKeyID string

func NewProviderKeyID(id string) ProviderKeyID { return ProviderKeyID(id) }
func (p ProviderKeyID) String() string         { return string(p) }
func (p ProviderKeyID) IsEmpty() bool          { return string(p) == "" }

type UsageLogID string

func NewUsageLogID(id string) UsageLogID { return UsageLogID(id) }
func (u UsageLogID) String() string      { return string(u) }
func (u UsageLogID) IsEmpty() bool       { return string(u) == "" }

type TransactionID string

func NewTransactionID(id string) TransactionID { return TransactionID(id) }
func (t TransactionID) String() string         { return string(t) }
func (t TransactionID) IsEmpty() bool          { return string(t) == "" }

type APIKeyID string

func NewAPIKeyID(id string) APIKeyID { return APIKeyID(id) }
func (a APIKeyID) String() string    { return string(a) }
func (a APIKeyID) IsEmpty() bool     { return string(a) == "" }

type GuardrailConfigID string

func NewGuardrailConfigID(id string) GuardrailConfigID { return GuardrailConfigID(id) }
func (g GuardrailConfigID) String() string             { return string(g) }
func (g GuardrailConfigID) IsEmpty() bool              { return string(g) == "" }

type GuardrailRuleID string

func NewGuardrailRuleID(id string) GuardrailRuleID { return GuardrailRuleID(id) }
func (g GuardrailRuleID) String() string            { return string(g) }
func (g GuardrailRuleID) IsEmpty() bool             { return string(g) == "" }

type GuardrailViolationID string

func NewGuardrailViolationID(id string) GuardrailViolationID { return GuardrailViolationID(id) }
func (g GuardrailViolationID) String() string                { return string(g) }
func (g GuardrailViolationID) IsEmpty() bool                 { return string(g) == "" }

type WebhookID string

func NewWebhookID(id string) WebhookID { return WebhookID(id) }
func (w WebhookID) String() string     { return string(w) }
func (w WebhookID) IsEmpty() bool      { return string(w) == "" }

type WebhookDeliveryID string

func NewWebhookDeliveryID(id string) WebhookDeliveryID { return WebhookDeliveryID(id) }
func (w WebhookDeliveryID) String() string             { return string(w) }
func (w WebhookDeliveryID) IsEmpty() bool              { return string(w) == "" }

type ModelFallbackID string

func NewModelFallbackID(id string) ModelFallbackID { return ModelFallbackID(id) }
func (m ModelFallbackID) String() string           { return string(m) }
func (m ModelFallbackID) IsEmpty() bool            { return string(m) == "" }
