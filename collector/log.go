// Copyright 2016 Apcera Inc. All rights reserved.

package collector

import (
	"os"
	"sync"
	"sync/atomic"

	"github.com/nats-io/gnatsd/logger"
)

// Logging in the collector
//
// The collector logger is an instance of a NATS logger, (basically duplicated
// from the NATS server code), and is passed into the NATS server.
//
// All logging functions are fully implemented (versus calling into the NATS
// server).

// Logger interface, duplicating that of the NATS Server
// This interface was copied from gnatsd to avoid vendoring
// dependencies.
type Logger interface {

	// Log a notice statement
	Noticef(format string, v ...interface{})

	// Log a fatal error
	Fatalf(format string, v ...interface{})

	// Log an error
	Errorf(format string, v ...interface{})

	// Log a debug statement
	Debugf(format string, v ...interface{})

	// Log a trace statement
	Tracef(format string, v ...interface{})
}

// Package globals for performance checks
var trace int32
var debug int32

// The STAN logger, encapsulates a NATS logger
var collectorLog = struct {
	sync.Mutex
	logger Logger
}{}

// Log Types
const (
	ConsoleLogType = iota
	SysLogType
	RemoteSysLogType
	FileLogType
)

// LoggerOptions configure the logger
type LoggerOptions struct {
	Debug        bool
	Trace        bool
	Logtime      bool
	LogFile      string
	LogType      int
	RemoteSyslog string
}

// ConfigureLogger configures logging for the NATS exporter.
func ConfigureLogger(lOpts *LoggerOptions) {
	var newLogger Logger

	var opts *LoggerOptions
	if lOpts != nil {
		opts = lOpts
	} else {
		opts = &LoggerOptions{}
	}

	// always log time
	opts.Logtime = true

	switch opts.LogType {
	case FileLogType:
		newLogger = logger.NewFileLogger(opts.LogFile, opts.Logtime, opts.Debug, opts.Trace, true)
	case RemoteSysLogType:
		newLogger = logger.NewRemoteSysLogger(opts.RemoteSyslog, opts.Debug, opts.Trace)
	case ConsoleLogType:
		colors := true
		// Check to see if stderr is being redirected and if so turn off color
		// Also turn off colors if we're running on Windows where os.Stderr.Stat() returns an invalid handle-error
		stat, err := os.Stderr.Stat()
		if err != nil || (stat.Mode()&os.ModeCharDevice) == 0 {
			colors = false
		}
		newLogger = logger.NewStdLogger(opts.Logtime, opts.Debug, opts.Trace, colors, true)
	case SysLogType:
		newLogger = logger.NewSysLogger(opts.Debug, opts.Trace)
	}
	if opts.Debug {
		atomic.StoreInt32(&debug, 1)
	}
	if opts.Trace {
		atomic.StoreInt32(&trace, 1)
	}

	collectorLog.Lock()
	collectorLog.logger = newLogger
	collectorLog.Unlock()
}

// RemoveLogger clears the logger instance and debug/trace flags.
// Used for testing.
func RemoveLogger() {
	atomic.StoreInt32(&trace, 0)
	atomic.StoreInt32(&debug, 0)

	collectorLog.Lock()
	collectorLog.logger = nil
	collectorLog.Unlock()
}

// Noticef logs a notice statement
func Noticef(format string, v ...interface{}) {
	executeLogCall(func(log Logger, format string, v ...interface{}) {
		log.Noticef(format, v...)
	}, format, v...)
}

// Errorf logs an error
func Errorf(format string, v ...interface{}) {
	executeLogCall(func(log Logger, format string, v ...interface{}) {
		log.Errorf(format, v...)
	}, format, v...)
}

// Fatalf logs a fatal error
func Fatalf(format string, v ...interface{}) {
	executeLogCall(func(log Logger, format string, v ...interface{}) {
		log.Fatalf(format, v...)
	}, format, v...)
}

// Debugf logs a debug statement
// nolint
func Debugf(format string, v ...interface{}) {
	if atomic.LoadInt32(&debug) != 0 {
		executeLogCall(func(log Logger, format string, v ...interface{}) {
			log.Debugf(format, v...)
		}, format, v...)
	}
}

// Tracef logs a trace statement
// nolint
func Tracef(format string, v ...interface{}) {
	if atomic.LoadInt32(&trace) != 0 {
		executeLogCall(func(logger Logger, format string, v ...interface{}) {
			logger.Tracef(format, v...)
		}, format, v...)
	}
}

func executeLogCall(f func(logger Logger, format string, v ...interface{}), format string, args ...interface{}) {
	collectorLog.Lock()
	defer collectorLog.Unlock()
	if collectorLog.logger == nil {
		return
	}
	f(collectorLog.logger, format, args...)
}
