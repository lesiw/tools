package modernize

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestUUID(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), UUID, "uuid")
}

func TestContainsCut(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ContainsCut, "containscut")
}
