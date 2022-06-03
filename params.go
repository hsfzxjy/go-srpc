package srpc

import (
	"log"
	"time"
)

var defaultSessionConfig = SessionConfig{
	BufferCapacity: 10,
	ClientTimeout:  10 * time.Second,
	KeepAlive:      10 * time.Second,
}

func SetDefaultSessionConfig(cfg SessionConfig) bool {
	return defaultSessionConfig.copyFrom(&cfg, true)
}

var serverLogFunc func(string, ...any) = func(format string, args ...any) {
	log.Printf(format, args...)
}

func SetServerLogFunc(logf func(string, ...any)) {
	serverLogFunc = logf
}

var clientLogFunc func(string) = func(s string) { log.Print(s) }

func SetClientLogFunc(logf func(string)) {
	clientLogFunc = logf
}
