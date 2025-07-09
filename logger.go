package eureka_client

import (
	"log"
	"os"
	"strings"
)

const (
	DEBUG = 4
	INFO  = 3
	WARN  = 2
	ERROR = 1
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Cyan   = "\033[36m"
)

type Logger interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string, err error)
	Error(msg string, err error)
}

type DefaultLogger struct {
	level int
}

func (l *DefaultLogger) initialize() *DefaultLogger {
	if l.level < 1 {
		switch strings.ToUpper(os.Getenv("LOG_LEVEL")) {
		case "DEBUG":
			l.level = DEBUG
		case "INFO":
			l.level = INFO
		case "WARN":
			l.level = WARN
		case "ERROR":
			l.level = ERROR
		default:
			l.level = INFO
		}
	}
	return l
}

func (l *DefaultLogger) Debug(msg string) {
	if l.level >= DEBUG {
		log.Printf("%sDEBUG%s: %s", Green, Reset, msg)
	}
}

func (l *DefaultLogger) Info(msg string) {
	if l.level >= INFO {
		log.Printf("%sINFO%s: %s", Cyan, Reset, msg)
	}
}

func (l *DefaultLogger) Warn(msg string, err error) {
	if l.level >= WARN {
		log.Printf("%sWARN%s: %s, %v", Yellow, Reset, msg, err)
	}
}

func (l *DefaultLogger) Error(msg string, err error) {
	if l.level >= ERROR {
		log.Printf("%sERROR%s: %s, %v", Red, Reset, msg, err)
	}
}

func NewLogger(logLevel int) Logger {
	logger := &DefaultLogger{
		level: logLevel,
	}
	return logger.initialize()
}
