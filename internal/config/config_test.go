package config

import "testing"

func TestLoad(t *testing.T) {
	for _, name := range []string{"APP_PORT", "DATABASE_URL", "DATABASE_MAX_CONNS", "DATABASE_CONNECT_TIMEOUT"} {
		t.Setenv(name, "")
	}
	if _, err := Load(); err == nil {
		t.Fatal("missing database URL accepted")
	}
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	c, err := Load()
	if err != nil || c.Port != "8080" || c.DatabaseMaxConns != 10 {
		t.Fatalf("defaults: %+v, %v", c, err)
	}
	for _, tc := range []struct{ key, value string }{{"APP_PORT", "0"}, {"APP_PORT", "65536"}, {"DATABASE_MAX_CONNS", "-1"}, {"DATABASE_MAX_CONNS", "2147483648"}, {"DATABASE_CONNECT_TIMEOUT", "0s"}, {"DATABASE_CONNECT_TIMEOUT", "bad"}} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}
