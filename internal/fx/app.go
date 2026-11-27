package fx

import (
	"github.com/profause/kupapay/internal/auth"
	"go.uber.org/fx"
)

var AppModule = fx.Module("app",
	InfraModule,
	auth.Module,
)
