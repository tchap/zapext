package zapsentry

import (
	"fmt"
	"strings"

	"github.com/getsentry/raven-go"
	"go.uber.org/zap/zapcore"
)

//
// Levels
//

var zapLevelToRavenSeverity = map[zapcore.Level]raven.Severity{
	zapcore.DebugLevel:  raven.DEBUG,
	zapcore.InfoLevel:   raven.INFO,
	zapcore.WarnLevel:   raven.WARNING,
	zapcore.ErrorLevel:  raven.ERROR,
	zapcore.DPanicLevel: raven.FATAL,
	zapcore.PanicLevel:  raven.FATAL,
	zapcore.FatalLevel:  raven.FATAL,
}

//
// Environment
//

type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvProduction  Environment = "production"
)

//
// Significant field keys
//

const TagPrefix = "#"

const (
	EventIDKey    = "event_id"
	ProjectKey    = "project"
	TimestampKey  = "timestamp"
	LoggerKey     = "logger"
	PlatformKey   = "platform"
	CulpritKey    = "culprit"
	ServerNameKey = "server_name"
	ErrorKey      = "error"
)

//
// Core options
//

type Option func(*Core)

func SetStackTraceSkip(skip int) Option {
	return func(core *Core) {
		core.stSkip = skip
	}
}

func SetStackTraceContext(context int) Option {
	return func(core *Core) {
		core.stContext = context
	}
}

func SetStackTracePackagePrefixes(prefixes []string) Option {
	return func(core *Core) {
		core.stPackagePrefixes = prefixes
	}
}

func SetWaitEnabler(enab zapcore.LevelEnabler) Option {
	return func(core *Core) {
		core.wait = enab
	}
}

//
// Core
//

const (
	DefaultEnvironment       = EnvProduction
	DefaultStackTraceContext = 5
	DefaultWaitEnabler       = zapcore.PanicLevel
)

type Core struct {
	zapcore.LevelEnabler

	client *raven.Client

	stSkip            int
	stContext         int
	stPackagePrefixes []string

	wait zapcore.LevelEnabler

	fields []zapcore.Field
}

func NewCore(enab zapcore.LevelEnabler, client *raven.Client, options ...Option) *Core {
	core := &Core{
		LevelEnabler: enab,
		client:       client,
		stContext:    DefaultStackTraceContext,
		wait:         DefaultWaitEnabler,
	}

	for _, opt := range options {
		opt(core)
	}

	return core
}

func (core *Core) With(fields []zapcore.Field) zapcore.Core {
	// Clone core.
	clone := *core

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
	// Create a Raven packet.
	packet := raven.NewPacket(entry.Message)

	// Process entry.
	packet.Level = zapLevelToRavenSeverity[entry.Level]
	packet.Timestamp = raven.Timestamp(entry.Time)
	packet.Logger = entry.LoggerName

	// Process fields.
	encoder := zapcore.NewMapObjectEncoder()
	var err error

	processField := func(field zapcore.Field) {
		// Check for significant keys.
		switch field.Key {
		case EventIDKey:
			packet.EventID = field.String

		case ProjectKey:
			packet.Project = field.String

		case PlatformKey:
			packet.Platform = field.String

		case CulpritKey:
			packet.Culprit = field.String

		case ServerNameKey:
			packet.ServerName = field.String

		case ErrorKey:
			if ex, ok := field.Interface.(error); ok {
				err = ex
			} else {
				field.AddTo(encoder)
			}

		default:
			// Add to the encoder in case this is not a significant key.
			field.AddTo(encoder)
		}
	}

	// Process core fields first.
	for _, field := range core.fields {
		processField(field)
	}

	// Then process the fields passed directly.
	// These can be then used to overwrite the core fields.
	for _, field := range fields {
		processField(field)
	}

	tags := make(map[string]string)
	extra := make(map[string]interface{})

	for key, value := range encoder.Fields {
		if strings.HasPrefix(key, TagPrefix) {
			key = key[len(TagPrefix):]
			if v, ok := value.(string); ok {
				tags[key] = v
			} else {
				tags[key] = fmt.Sprintf("%v", value)
			}
		} else {
			extra[key] = value
		}
	}

	if len(tags) != 0 {
		packet.AddTags(tags)
	}
	if len(extra) != 0 {
		packet.Extra = extra
	}

	// In case an error object is present, create an exception.
	// Capture the stack trace in any case.
	stackTrace := raven.NewStacktrace(core.stSkip, core.stContext, core.stPackagePrefixes)
	if err != nil {
		packet.Interfaces = append(packet.Interfaces, raven.NewException(err, stackTrace))
	} else {
		packet.Interfaces = append(packet.Interfaces, stackTrace)
	}

	// Capture the packet.
	_, errCh := core.client.Capture(packet, nil)

	if core.wait.Enabled(entry.Level) {
		return <-errCh
	}
	return nil
}

func (core *Core) Sync() error {
	core.client.Wait()
	return nil
}
