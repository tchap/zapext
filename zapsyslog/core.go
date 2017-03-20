package zapsyslog

import (
	"log/syslog"

	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
)

//
// Core
//

type Core struct {
	zapcore.LevelEnabler

	encoder zapcore.Encoder
	writer  *syslog.Writer

	fields []zapcore.Field
}

func NewCore(enab zapcore.LevelEnabler, encoder zapcore.Encoder, writer *syslog.Writer) *Core {
	return &Core{
		LevelEnabler: enab,
		encoder:      encoder,
		writer:       writer,
	}
}

func (core *Core) With(fields []zapcore.Field) zapcore.Core {
	// Clone core.
	clone := *core

	// Clone encoder.
	clone.encoder = core.encoder.Clone()

	// Clone and append fields.
	clone.fields = make([]zapcore.Field, len(core.fields)+len(fields))
	copy(clone.fields, core.fields)
	copy(clone.fields[len(core.fields):], fields)

	// Done.
	return &clone
}

func (core *Core) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if core.Enabled(entry.Level) {
		return checked.AddCore(entry, core)
	}
	return checked
}

func (core *Core) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	// Generate the message.
	buffer, err := core.encoder.EncodeEntry(entry, fields)
	if err != nil {
		return errors.Wrap(err, "failed to encode log entry")
	}

	message := buffer.String()

	// Write the message.
	switch entry.Level {
	case zapcore.DebugLevel:
		return core.writer.Debug(message)

	case zapcore.InfoLevel:
		return core.writer.Info(message)

	case zapcore.WarnLevel:
		return core.writer.Warning(message)

	case zapcore.ErrorLevel:
		return core.writer.Err(message)

	case zapcore.DPanicLevel:
		return core.writer.Crit(message)

	case zapcore.PanicLevel:
		return core.writer.Crit(message)

	case zapcore.FatalLevel:
		return core.writer.Crit(message)

	default:
		return errors.Errorf("unknown log level: %v", entry.Level)
	}
}

func (core *Core) Sync() error {
	return nil
}
