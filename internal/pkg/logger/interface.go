/*******************************************************************************
 * Copyright 2019 Dell Inc.
 * Copyright (C) 2025 IOTech Ltd
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/

package logger

// LoggingClient defines the interface for logging operations.
type LoggingClient interface {
	// SetLogLevel sets minimum severity log level. If a logging method is called with a lower level of severity than
	// what is set, it will result in no output.
	SetLogLevel(logLevel string) error
	// LogLevel returns the current log level setting
	LogLevel() string
	// Debug logs a message at the DEBUG severity level
	Debug(msg string, args ...interface{})
	// Error logs a message at the ERROR severity level
	Error(msg string, args ...interface{})
	// Info logs a message at the INFO severity level
	Info(msg string, args ...interface{})
	// Trace logs a message at the TRACE severity level
	Trace(msg string, args ...interface{})
	// Warn logs a message at the WARN severity level
	Warn(msg string, args ...interface{})
	// Debugf logs a formatted message at the DEBUG severity level
	Debugf(msg string, args ...interface{})
	// Errorf logs a formatted message at the ERROR severity level
	Errorf(msg string, args ...interface{})
	// Infof logs a formatted message at the INFO severity level
	Infof(msg string, args ...interface{})
	// Tracef logs a formatted message at the TRACE severity level
	Tracef(msg string, args ...interface{})
	// Warnf logs a formatted message at the WARN severity level
	Warnf(msg string, args ...interface{})
	// Close closes any open file handles
	Close() error
}
