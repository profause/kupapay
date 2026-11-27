package validation

import "go.uber.org/fx"

// Module provides the validation service for FX dependency injection
var Module = fx.Module("validation",
	fx.Provide(NewValidator),
)
