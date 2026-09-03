package notifx

// SendOptions holds optional configuration for a send operation.
type SendOptions struct {
	Tags     map[string]string
	ConfigID string
}

// Option is a functional option for send operations.
type Option func(*SendOptions)

// WithTags adds metadata tags to the send operation.
func WithTags(tags map[string]string) Option {
	return func(o *SendOptions) {
		o.Tags = tags
	}
}

// WithConfigID sets a provider-specific configuration set identifier.
func WithConfigID(id string) Option {
	return func(o *SendOptions) {
		o.ConfigID = id
	}
}
