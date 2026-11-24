package fx

import "go.uber.org/fx"

var AppModule = fx.Module("app",
	InfraModule,
)
