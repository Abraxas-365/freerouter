package webhookmodule

import (
	"github.com/Abraxas-365/freerouter/internal/webhook"
	"github.com/Abraxas-365/freerouter/internal/webhook/adapters/webhookhttp"
	"github.com/Abraxas-365/freerouter/internal/webhook/adapters/webhookpg"
	"github.com/Abraxas-365/freerouter/internal/webhook/webhooksvc"
	"github.com/jmoiron/sqlx"
)

// Deps holds external dependencies for the webhook module.
type Deps struct {
	DB *sqlx.DB
}

// Module exposes the webhook module's public interfaces and HTTP handler.
type Module struct {
	Commands webhook.Commands
	Queries  webhook.Queries
	HTTP     *webhookhttp.Handler

	// Dispatcher exposes Fire for other modules (e.g. gateway) to emit events.
	Dispatcher webhook.Dispatcher

	svc *webhooksvc.Service // kept for StartWorker/Stop
}

// New wires the webhook module: repo → service → handler.
// The caller must invoke Module.StartWorker() after construction and
// Module.Stop() on shutdown to run the delivery retry loop.
func New(deps Deps) Module {
	repo := webhookpg.New(deps.DB)
	svc := webhooksvc.New(repo)

	return Module{
		Commands:   svc,
		Queries:    svc,
		HTTP:       webhookhttp.New(svc, svc, svc),
		Dispatcher: svc,
		svc:        svc,
	}
}

// StartWorker starts the background delivery retry loop.
func (m *Module) StartWorker() {
	m.svc.StartWorker()
}

// Stop gracefully stops the background delivery retry loop.
func (m *Module) Stop() {
	m.svc.Stop()
}
