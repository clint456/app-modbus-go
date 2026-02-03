package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewClient tests the NewClient constructor
func TestNewClient(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		want     string
	}{
		{
			name:     "valid INFO level",
			logLevel: "INFO",
			want:     "INFO",
		},
		{
			name:     "valid DEBUG level",
			logLevel: "DEBUG",
			want:     "DEBUG",
		},
		{
			name:     "lowercase level",
			logLevel: "debug",
			want:     "DEBUG",
		},
		{
			name:     "invalid level defaults to INFO",
			logLevel: "INVALID",
			want:     "INFO",
		},
		{
			name:     "empty level defaults to INFO",
			logLevel: "",
			want:     "INFO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lc := NewClient(tt.logLevel)
			assert.NotNil(t, lc)
			assert.Equal(t, tt.want, lc.LogLevel())
		})
	}
}

// TestSetLogLevel tests the SetLogLevel method
func TestSetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		initial  string
		newLevel string
		wantErr  bool
		wantLevel string
	}{
		{
			name:      "set to DEBUG",
			initial:   "INFO",
			newLevel:  "DEBUG",
			wantErr:   false,
			wantLevel: "DEBUG",
		},
		{
			name:      "set to ERROR",
			initial:   "INFO",
			newLevel:  "ERROR",
			wantErr:   false,
			wantLevel: "ERROR",
		},
		{
			name:      "lowercase level",
			initial:   "INFO",
			newLevel:  "warn",
			wantErr:   false,
			wantLevel: "WARN",
		},
		{
			name:      "invalid level",
			initial:   "INFO",
			newLevel:  "INVALID",
			wantErr:   true,
			wantLevel: "INFO", // should remain unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lc := NewClient(tt.initial)
			err := lc.SetLogLevel(tt.newLevel)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantLevel, lc.LogLevel())
		})
	}
}

// TestLogLevel tests the LogLevel getter
func TestLogLevel(t *testing.T) {
	levels := []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			lc := NewClient(level)
			assert.Equal(t, level, lc.LogLevel())
		})
	}
}

// TestLogLevelFiltering tests that only logs at or above the set level are output
func TestLogLevelFiltering(t *testing.T) {
	// We can't easily capture stdout in tests, but we can test the enabled() method
	// by checking if logs are actually output at different levels
	
	tests := []struct {
		name          string
		setLevel      string
		shouldLog     map[string]bool
	}{
		{
			name:     "INFO level",
			setLevel: "INFO",
			shouldLog: map[string]bool{
				"TRACE": false,
				"DEBUG": false,
				"INFO":  true,
				"WARN":  true,
				"ERROR": true,
			},
		},
		{
			name:     "DEBUG level",
			setLevel: "DEBUG",
			shouldLog: map[string]bool{
				"TRACE": false,
				"DEBUG": true,
				"INFO":  true,
				"WARN":  true,
				"ERROR": true,
			},
		},
		{
			name:     "ERROR level",
			setLevel: "ERROR",
			shouldLog: map[string]bool{
				"TRACE": false,
				"DEBUG": false,
				"INFO":  false,
				"WARN":  false,
				"ERROR": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lc := NewClient(tt.setLevel).(*edgeXLogger)
			
			for level, shouldEnable := range tt.shouldLog {
				enabled := lc.enabled(level)
				assert.Equal(t, shouldEnable, enabled, 
					"Level %s should be enabled=%v when log level is %s", 
					level, shouldEnable, tt.setLevel)
			}
		})
	}
}

// TestLoggingMethods tests that all logging methods can be called without panic
func TestLoggingMethods(t *testing.T) {
	lc := NewClient("DEBUG")
	
	// Test non-formatted methods
	t.Run("non-formatted methods", func(t *testing.T) {
		assert.NotPanics(t, func() {
			lc.Trace("trace message")
			lc.Debug("debug message")
			lc.Info("info message")
			lc.Warn("warn message")
			lc.Error("error message")
		})
	})
	
	// Test formatted methods
	t.Run("formatted methods", func(t *testing.T) {
		assert.NotPanics(t, func() {
			lc.Tracef("trace %s", "formatted")
			lc.Debugf("debug %s", "formatted")
			lc.Infof("info %s", "formatted")
			lc.Warnf("warn %s", "formatted")
			lc.Errorf("error %s", "formatted")
		})
	})
	
	// Test with key-value pairs
	t.Run("with key-value pairs", func(t *testing.T) {
		assert.NotPanics(t, func() {
			lc.Info("message with kvs", "key1", "value1", "key2", "value2")
			lc.Debug("debug with kvs", "user", "alice", "action", "login")
		})
	})
}

// TestNewClientWithConfig tests the NewClientWithConfig constructor
func TestNewClientWithConfig(t *testing.T) {
	t.Run("console only", func(t *testing.T) {
		cfg := LoggerConfig{
			LogLevel:      "DEBUG",
			EnableConsole: true,
		}
		lc := NewClientWithConfig(cfg)
		assert.NotNil(t, lc)
		assert.Equal(t, "DEBUG", lc.LogLevel())
	})
	
	t.Run("invalid log level defaults to INFO", func(t *testing.T) {
		cfg := LoggerConfig{
			LogLevel:      "INVALID",
			EnableConsole: true,
		}
		lc := NewClientWithConfig(cfg)
		assert.NotNil(t, lc)
		assert.Equal(t, "INFO", lc.LogLevel())
	})
	
	t.Run("no console no file defaults to stdout", func(t *testing.T) {
		cfg := LoggerConfig{
			LogLevel:      "INFO",
			EnableConsole: false,
		}
		lc := NewClientWithConfig(cfg)
		assert.NotNil(t, lc)
	})
}
