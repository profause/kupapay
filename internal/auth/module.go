package auth

import "go.uber.org/fx"

var Module = fx.Module("auth",
	fx.Provide(NewStore),
	fx.Provide(func(store *Store) repository { return store }),
	fx.Provide(NewService),
	fx.Provide(NewMiddleware),
	fx.Provide(NewHandler))
