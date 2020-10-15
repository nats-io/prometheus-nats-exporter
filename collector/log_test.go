// Copyright 2017-2018 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collector

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

func TestConfigureLogger(t *testing.T) {
	defer RemoveLogger()

	checkDebugTraceOff := func() {
		if debug != 0 || trace != 0 {
			t.Fatalf("Expected debug/trace to be disabled.")
		}
	}

	// Test nil options
	ConfigureLogger(nil)
	checkDebugTraceOff()

	opts := &LoggerOptions{}

	// Neither enabled (defaults are off)
	ConfigureLogger(opts)
	checkDebugTraceOff()

	//  debug options enabled
	opts.Debug = true
	opts.Trace = true
	ConfigureLogger(opts)

	// turn off logging we've enabled
	RemoveLogger()
}

func TestLogging(t *testing.T) {

	defer RemoveLogger()

	// test without a logger
	Noticef("noop")

	opts := &LoggerOptions{}

	// test stdout
	opts.Debug = true
	opts.Trace = true
	ConfigureLogger(opts)

	// skip syslog until there is support in CI
	// nOpts = &natsd.Options{}
	// nOpts.Syslog = true
	// ConfigureLogger(sOpts, nOpts)

	// nOpts = &natsd.Options{}
	// nOpts.RemoteSyslog = "udp://localhost:514"
	// ConfigureLogger(sOpts, nOpts)

	// test file
	tmpDir, err := ioutil.TempDir("", "_exporter")
	if err != nil {
		t.Fatal("Could not create tmp dir")
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	file, err := ioutil.TempFile(tmpDir, "exporter:log_")
	if err != nil {
		t.Fatalf("unable to create temporary file")
	}

	opts = &LoggerOptions{}
	opts.LogFile = file.Name()
	opts.LogType = FileLogType
	ConfigureLogger(opts)
}

type dummyLogger struct {
	msg string
}

func (d *dummyLogger) Noticef(format string, args ...interface{}) {
	d.msg = fmt.Sprintf(format, args...)
}

func (d *dummyLogger) Debugf(format string, args ...interface{}) {
	d.msg = fmt.Sprintf(format, args...)
}

func (d *dummyLogger) Tracef(format string, args ...interface{}) {
	d.msg = fmt.Sprintf(format, args...)
}

func (d *dummyLogger) Errorf(format string, args ...interface{}) {
	d.msg = fmt.Sprintf(format, args...)
}

func (d *dummyLogger) Fatalf(format string, args ...interface{}) {
	d.msg = fmt.Sprintf(format, args...)
}

func (d *dummyLogger) Reset() {
	d.msg = ""
}

func TestLogOutput(t *testing.T) {
	defer RemoveLogger()

	// dummy to override the configured logger.
	d := &dummyLogger{}

	checkLogger := func(output string) {
		if d.msg != output {
			t.Fatalf("Unexpected logger message: %v", d.msg)
		}
		d.Reset()
	}

	opts := &LoggerOptions{}
	ConfigureLogger(opts)

	// override the default logger.
	collectorLog.Lock()
	collectorLog.logger = d
	collectorLog.Unlock()

	// write to our logger and check values
	Noticef("foo")
	checkLogger("foo")

	Errorf("foo")
	checkLogger("foo")

	Fatalf("foo")
	checkLogger("foo")

	// debug is NOT set, value should be empty.
	Debugf("foo")
	checkLogger("")

	// trace is NOT set, value should be empty.
	Tracef("foo")
	checkLogger("")

	// enable debug and trace
	opts.Debug = true
	opts.Trace = true

	// reconfigure with debug/trace enabled
	ConfigureLogger(opts)

	// override the default logger.
	collectorLog.Lock()
	collectorLog.logger = d
	collectorLog.Unlock()

	// Debug is set so we should have the value
	Debugf("foo")
	checkLogger("foo")

	// Trace is set so we should have the value
	Tracef("foo")
	checkLogger("foo")
}
