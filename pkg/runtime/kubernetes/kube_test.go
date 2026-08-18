package kubernetes

import (
	"testing"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime/runtimetest"
)

func TestBackendSatisfiesContract(t *testing.T) {
	runtimetest.AssertRuntimeContract(t, func() runtime.Runtime { return New() })
}
