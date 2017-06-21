package middleware

import (
	"go.uber.org/zap/zapcore"
)

// FilterFunc is the filter function that is called to check
// whether the given entry together with the associated fields
// is to be written to a core or not.
type FilterFunc func(zapcore.Entry, []zapcore.Field) bool

type filteringCore struct {
	zapcore.Core
	filter FilterFunc
}

// NewFilteringCore returns a core middleware that uses the given filter function
// to decide whether to actually call Write on the next core in the chain.
func NewFilteringCore(next zapcore.Core, filter FilterFunc) zapcore.Core {
	return &filteringCore{next, filter}
}

func (core *filteringCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	if !core.filter(entry, fields) {
		return nil
	}

	return core.Core.Write(entry, fields)
}
