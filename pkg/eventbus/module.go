package eventbus

import "go.uber.org/fx"

// Module provides the in-memory bus singleton.
var Module = fx.Module("eventbus", fx.Provide(New))
