package guardrailmodule

import (
	"github.com/Abraxas-365/freerouter/internal/guardrail"
	"github.com/Abraxas-365/freerouter/internal/guardrail/adapters/guardrailhttp"
	"github.com/Abraxas-365/freerouter/internal/guardrail/adapters/guardrailpg"
	"github.com/Abraxas-365/freerouter/internal/guardrail/guardrailsvc"
	"github.com/jmoiron/sqlx"
)

// Deps holds external dependencies for the guardrail module.
type Deps struct {
	DB *sqlx.DB
}

// Module exposes the guardrail module's public interfaces and HTTP handler.
type Module struct {
	Commands guardrail.Commands
	Queries  guardrail.Queries
	HTTP     *guardrailhttp.Handler

	// Evaluator exposes CheckMessages for the gateway to call before routing.
	Evaluator guardrail.Evaluator
}

// New wires the guardrail module: repo → service → handler.
func New(deps Deps) Module {
	repo := guardrailpg.New(deps.DB)
	svc := guardrailsvc.New(repo)

	return Module{
		Commands:  svc,
		Queries:   svc,
		HTTP:      guardrailhttp.New(svc, svc),
		Evaluator: svc,
	}
}
