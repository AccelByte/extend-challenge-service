package common

import (
	"testing"
)

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envVal   string
		setEnv   bool
		fallback bool
		want     bool
	}{
		{"true lowercase", "TEST_BOOL", "true", true, false, true},
		{"TRUE uppercase", "TEST_BOOL", "TRUE", true, false, true},
		{"True mixed", "TEST_BOOL", "True", true, false, true},
		{"false lowercase", "TEST_BOOL", "false", true, true, false},
		{"FALSE uppercase", "TEST_BOOL", "FALSE", true, true, false},
		{"False mixed", "TEST_BOOL", "False", true, true, false},
		{"1 is true", "TEST_BOOL", "1", true, false, true},
		{"0 is false", "TEST_BOOL", "0", true, true, false},
		{"yes is true", "TEST_BOOL", "yes", true, false, true},
		{"YES is true", "TEST_BOOL", "YES", true, false, true},
		{"no is false", "TEST_BOOL", "no", true, true, false},
		{"NO is false", "TEST_BOOL", "NO", true, true, false},
		{"empty uses fallback true", "TEST_BOOL", "", true, true, true},
		{"empty uses fallback false", "TEST_BOOL", "", true, false, false},
		{"unset uses fallback true", "TEST_BOOL_UNSET", "", false, true, true},
		{"unset uses fallback false", "TEST_BOOL_UNSET", "", false, false, false},
		{"invalid uses fallback", "TEST_BOOL", "maybe", true, false, false},
		{"invalid uses fallback true", "TEST_BOOL", "maybe", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(tt.envKey, tt.envVal)
			}

			got := GetEnvBool(tt.envKey, tt.fallback)
			if got != tt.want {
				t.Errorf("GetEnvBool(%q, %v) = %v, want %v", tt.envKey, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv("TEST_KEY", "hello")
		if got := GetEnv("TEST_KEY", "fallback"); got != "hello" {
			t.Errorf("GetEnv() = %q, want %q", got, "hello")
		}
	})

	t.Run("returns fallback when unset", func(t *testing.T) {
		if got := GetEnv("NONEXISTENT_KEY_12345", "fallback"); got != "fallback" {
			t.Errorf("GetEnv() = %q, want %q", got, "fallback")
		}
	})
}

func TestGetEnvInt(t *testing.T) {
	t.Run("returns env value when valid int", func(t *testing.T) {
		t.Setenv("TEST_INT", "42")
		if got := GetEnvInt("TEST_INT", 10); got != 42 {
			t.Errorf("GetEnvInt() = %d, want %d", got, 42)
		}
	})

	t.Run("returns fallback for invalid int", func(t *testing.T) {
		t.Setenv("TEST_INT", "notanumber")
		if got := GetEnvInt("TEST_INT", 10); got != 10 {
			t.Errorf("GetEnvInt() = %d, want %d", got, 10)
		}
	})

	t.Run("returns fallback when unset", func(t *testing.T) {
		if got := GetEnvInt("NONEXISTENT_INT_12345", 99); got != 99 {
			t.Errorf("GetEnvInt() = %d, want %d", got, 99)
		}
	})
}
