package antch

// Logger is an interface for logging message.
type Logger interface {
	Output(maxdepth int, s string) error
}

type nilLogger struct{}

// NilLogger is a Logger that will not logging any message.
var NilLogger nilLogger

func (l nilLogger) Output(maxdepth int, s string) error {
	return nil
}
