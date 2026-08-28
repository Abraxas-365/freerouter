package guardrailscontainer

import (
	"github.com/Abraxas-365/freerouter/internal/ai/guardrails/guardrailsapi"
	"github.com/Abraxas-365/freerouter/internal/ai/guardrails/guardrailsinfra"
	"github.com/Abraxas-365/freerouter/internal/ai/guardrails/guardrailssrv"
	"github.com/jmoiron/sqlx"
)

type Container struct {
	Service  *guardrailssrv.GuardrailsService
	Handlers *guardrailsapi.GuardrailHandlers
}

func New(db *sqlx.DB) *Container {
	repo := guardrailsinfra.NewPostgresRepository(db)
	service := guardrailssrv.New(repo)
	handlers := guardrailsapi.NewGuardrailHandlers(service)

	return &Container{
		Service:  service,
		Handlers: handlers,
	}
}
