package zapext

// DiscardingWriteSyncer is a zapcore.WriteSyncer that does nothing.
type DiscardingWriteSyncer int

// Write is a part of zapcore.WriteSyncer interface.
func (ws DiscardingWriteSyncer) Write(p []byte) (int, error) {
	return len(p), nil
}

// Sync is a part of zapcore.WriteSyncer interface.
func (ws DiscardingWriteSyncer) Sync() error {
	return nil
}
