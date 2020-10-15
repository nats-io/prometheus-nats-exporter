/*
 * Copyright 2019 The NATS Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package logging

// Config defines logging flags for the NATS logger
type Config struct {
	Hide   bool
	Time   bool
	Debug  bool
	Trace  bool
	Colors bool
	PID    bool
}

// Logger interface
type Logger interface {
	Debugf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
	Fatalf(format string, v ...interface{})
	Noticef(format string, v ...interface{})
	Tracef(format string, v ...interface{})
	Warnf(format string, v ...interface{})

	TraceEnabled() bool

	Close() error
}
