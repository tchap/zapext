package zapsentry

import (
	"fmt"
	"strings"

	"github.com/getsentry/raven-go"
	"github.com/pkg/errors"
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
)

//
// Core options
//

type Option func(*Core)

func SetEnvironment(env Environment) Option {
	return func(core *Core) {
		core.env = env
	}
}

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

func SetWait(enab zapcore.LevelEnabler) Option {
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
	DefaultWait              = zapcore.PanicLevel
)

type Core struct {
	zapcore.LevelEnabler

	client *raven.Client

	env Environment

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
		env:          DefaultEnvironment,
		stContext:    DefaultStackTraceContext,
		wait:         DefaultWait,
	}

	for _, opt := range options {
		opt(core)
	}

	switch core.env {
	case EnvDevelopment:
		if !core.Enabled(zapcore.DebugLevel) {
			core.LevelEnabler = zapcore.DebugLevel
		}

	case EnvProduction:
		if !core.Enabled(zapcore.ErrorLevel) {
			core.LevelEnabler = zapcore.ErrorLevel
		}
	}

	client.SetEnvironment(string(core.env))

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
	// Assemble Sentry interface objects.
	var interfaces []raven.Interface

	if entry.Level >= zapcore.ErrorLevel {
		ex := raven.NewException(errors.New(entry.Message),
			raven.NewStacktrace(core.stSkip, core.stContext, core.stPackagePrefixes))

		interfaces = append(interfaces, ex)
	}

	// Create a Raven packet.
	packet := raven.NewPacket(entry.Message, interfaces...)

	// Process entry.
	packet.Level = zapLevelToRavenSeverity[entry.Level]
	packet.Timestamp = raven.Timestamp(entry.Time)
	packet.Logger = entry.LoggerName

	// Process fields.
	encoder := zapcore.NewMapObjectEncoder()

	for _, field := range fields {
		// Check for significant keys.

		/*
			TODO: We Could also try to set:

			packet.Modules =
			packet.Fingerprint =
		*/

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

		default:
			// Add to the encoder in case this is not a significant key.
			field.AddTo(encoder)
		}
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
