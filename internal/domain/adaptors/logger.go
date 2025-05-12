package adaptors

type LogLevel string

const (
	Trace LogLevel = "trace"
	Debug LogLevel = "debug"
	Info  LogLevel = "info"
	Warn  LogLevel = "warn"
	Error LogLevel = "error"
)
