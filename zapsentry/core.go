package zapsentry

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/tchap/zapext/types"

	"github.com/getsentry/raven-go"
	"github.com/pkg/errors"
	"go.uber.org/zap"
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
// Significant field keys
//

const TagPrefix = "#"

const (
	EventIDKey     = "event_id"
	ProjectKey     = "project"
	TimestampKey   = "timestamp"
	LoggerKey      = "logger"
	PlatformKey    = "platform"
	CulpritKey     = "culprit"
	ServerNameKey  = "server_name"
	ErrorKey       = "error"
	HTTPRequestKey = "http_request"
)

const ErrorStackTraceKey = "error_stack_trace"

const SkipKey = "_zapsentry_skip"

// Skip returns a field that tells zapsentry to skip the log entry.
func Skip() zapcore.Field {
	return zap.Bool(SkipKey, true)
}

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

	// When set, relevant Sentry interfaces are added.
	var (
		err error
		req *http.Request
	)

	// processField processes the given field.
	// When false is returned, the whole entry is to be skipped.
	processField := func(field zapcore.Field) bool {
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

		case HTTPRequestKey:
			switch r := field.Interface.(type) {
			case *http.Request:
				req = r
			case types.HTTPRequest:
				req = r.R
			case *types.HTTPRequest:
				req = r.R
			default:
				field.AddTo(encoder)
			}

		case SkipKey:
			return false

		default:
			// Add to the encoder in case this is not a significant key.
			field.AddTo(encoder)
		}

		return true
	}

	// Process core fields first.
	for _, field := range core.fields {
		if !processField(field) {
			return nil
		}
	}

	// Then process the fields passed directly.
	// These can be then used to overwrite the core fields.
	for _, field := range fields {
		if !processField(field) {
			return nil
		}
	}

	// Split fields into tags and extra.
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

	// In case an error object is present, create an exception.
	// Capture the stack trace in any case.
	stackTrace := raven.NewStacktrace(core.stSkip, core.stContext, core.stPackagePrefixes)
	if err != nil {
		packet.Interfaces = append(packet.Interfaces, raven.NewException(err, stackTrace))

		// In case this is a stack tracer, record the actual error stack trace.
		if stackTracer, ok := err.(StackTracer); ok {
			frames := stackTracer.StackTrace()
			record := make([][]string, 0, len(frames))
			for _, frame := range frames {
				record = append(record, strings.Split(fmt.Sprintf("%+v", frame), "\n"))
			}
			extra[ErrorStackTraceKey] = record
		}
	} else {
		packet.Interfaces = append(packet.Interfaces, stackTrace)
	}

	// In case an HTTP request is present, add the HTTP interface.
	if req != nil {
		packet.Interfaces = append(packet.Interfaces, raven.NewHttp(req))
	}

	// Add tags and extra into the packet.
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

type StackTracer interface {
	StackTrace() errors.StackTrace
}
