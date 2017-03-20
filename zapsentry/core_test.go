package zapsentry

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestCore_CheckZapCoreInterface(t *testing.T) {
	var _ zapcore.Core = &Core{}
}
