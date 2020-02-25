package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/getsentry/sentry-go"
	"github.com/tchap/zapext/zapsentry"

	"github.com/google/uuid"
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
	// Read DSN from the environment.
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		return errors.New("SENTRY_DSN is not set")
	}

	// Instantiate a client.
	client, err := sentry.NewClient(sentry.ClientOptions{Dsn: dsn})
	if err != nil {
		return errors.Wrap(err, "failed to get a Sentry client")
	}

	// Instantiate Zap.
	logger := zap.New(zapsentry.NewCore(zapcore.ErrorLevel, client))

	// Generate event ID.
	uu, err := uuid.NewRandom()
	if err != nil {
		return errors.Wrap(err, "failed to generate UUID")
	}

	eventID := hex.EncodeToString(uu[:])

	// Log.
	logger.Error("nuked",
		zap.String("event_id", eventID),
		zap.String("#subsystem", "example"),
	)

	// Print the event ID to check with Sentry manually.
	fmt.Println(eventID)

	// Sync.
	return errors.Wrap(logger.Sync(), "failed to sync packets to Sentry")
}
