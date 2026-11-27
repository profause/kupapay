package fx

import (
	"testing"

	"go.uber.org/fx"
)

func TestAppModuleDependenciesAreResolvable(t *testing.T) {
	if err := fx.ValidateApp(AppModule); err != nil {
		t.Fatalf("ValidateApp() error = %v", err)
	}
}
