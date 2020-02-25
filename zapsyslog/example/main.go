package main

import (
	"flag"
	"fmt"
	"log/syslog"
	"os"

	"github.com/tchap/zapext/v2/zapsyslog"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Flags.
	flagTag := flag.String("syslog_tag", "zap", "syslog tag")

	flag.Parse()

	// Initialize a syslog writer.
	writer, err := syslog.New(syslog.LOG_ERR|syslog.LOG_LOCAL0, *flagTag)
	if err != nil {
		return errors.Wrap(err, "failed to set up syslog")
	}

	// Initialize Zap.
	encoder := zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig())

	core := zapsyslog.NewCore(zapcore.ErrorLevel, encoder, writer)

	logger := zap.New(core, zap.Development(), zap.AddStacktrace(zapcore.ErrorLevel))

	// Log.
	logger.Error("nuked", zap.String("subsystem", "example"))

	// Sync.
	return errors.Wrap(logger.Sync(), "failed to sync packets to Sentry")
}
