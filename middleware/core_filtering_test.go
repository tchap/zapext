package middleware_test

import (
	"testing"

	"github.com/tchap/zapext"
	"github.com/tchap/zapext/middleware"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFilteringCore(t *testing.T) {

	Convey("Given a FilteringCore", t, func() {

		filter := func(entry zapcore.Entry, fields []zapcore.Field) bool {
			for _, field := range fields {
				if field.Key == "skip" {
					return false
				}
			}
			return true
		}

		enc := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{})
		next := zapcore.NewCore(enc, zapext.DiscardingWriteSyncer(0), zapcore.ErrorLevel)

		var called bool

		next = zapcore.RegisterHooks(next, func(entry zapcore.Entry) error {
			called = true
			return nil
		})

		logger := zap.New(middleware.NewFilteringCore(next, filter))

		Convey("Writing a log entry that is to be skipped", func() {

			Convey("The log entry is skipped", func() {

				called = false

				logger.Error("We fucked up!", zap.Bool("skip", true))

				So(called, ShouldBeFalse)
			})
		})

		Convey("Writing a log entry that is to be written", func() {

			Convey("The log entry is written", func() {

				called = false

				logger.Error("We fucked up!")

				So(called, ShouldBeTrue)
			})
		})

	})
}
