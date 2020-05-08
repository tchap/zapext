package zapsentry

import (
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// User extends sentry.User to be a zapcore.ObjectMarshaler.
//
// This object can be passed to the logger as a field,
// causing the underlying core to set Event.User to the field value.
type User sentry.User

// MarshalLogObject implements zapcore.ObjectMarshaler interface.
func (user User) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	add := func(key, value string) {
		if value != "" {
			enc.AddString(key, value)
		}
	}

	add("email", user.Email)
	add("id", user.ID)
	add("ip_address", user.IPAddress)
	add("username", user.Username)
	return nil
}

// UserField turns the given user object into a field.
func UserField(user User) zapcore.Field {
	return zap.Object(UserKey, user)
}
