package usagemodule

import (
	"time"

	"github.com/Abraxas-365/freerouter/internal/usage"
	"github.com/Abraxas-365/freerouter/internal/usage/adapters/usagehttp"
	"github.com/Abraxas-365/freerouter/internal/usage/adapters/usagepg"
	"github.com/Abraxas-365/freerouter/internal/usage/usagesvc"
	"github.com/jmoiron/sqlx"
)

// Deps holds external dependencies for the usage module.
type Deps struct {
	DB            *sqlx.DB
	BufferSize    int           // async log buffer (default 1000)
	PurgeInterval time.Duration // background retention-purge interval (default 1h)
}

// Module exposes the usage module's public interfaces and HTTP handler.
type Module struct {
	Commands usage.Commands
	Queries  usage.Queries
	HTTP     *usagehttp.Handler

	svc *usagesvc.Service // kept for Close()/purge worker lifecycle
}

// New wires the usage module: repo → service → handler.
func New(deps Deps) Module {
	repo := usagepg.New(deps.DB)
	svc := usagesvc.New(repo, deps.BufferSize)

	return Module{
		Commands: svc,
		Queries:  svc,
		HTTP:     usagehttp.New(svc, svc),
		svc:      svc,
	}
}

// StartPurgeWorker launches the background retention-purge worker.
func (m *Module) StartPurgeWorker(interval time.Duration) {
	if m.svc != nil {
		m.svc.StartPurgeWorker(interval)
	}
}

// StopPurgeWorker stops the background retention-purge worker.
func (m *Module) StopPurgeWorker() {
	if m.svc != nil {
		m.svc.StopPurgeWorker()
	}
}

// Close drains the async log buffer.
func (m *Module) Close() {
	if m.svc != nil {
		m.svc.Close()
	}
}
