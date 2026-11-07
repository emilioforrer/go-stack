package observability

import (
	"testing"
)

// blankEnv blanks every environment variable read by ConfigFromEnv so the
// default values are deterministic regardless of the host environment.
func blankEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"OBSERVABILITY_ENABLED",
		"OBSERVABILITY_TRACING_ENABLED",
		"OBSERVABILITY_METRICS_ENABLED",
		"OTEL_SERVICE_NAME",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_INSECURE",
		"OTEL_TRACES_SAMPLER",
		"OTEL_TRACES_SAMPLER_ARG",
	} {
		t.Setenv(key, "")
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "disabled always valid",
			cfg:     Config{Enabled: false},
			wantErr: false,
		},
		{
			name:    "valid enabled config",
			cfg:     Config{Enabled: true, ServiceName: "api", OTLPEndpoint: "localhost:4317", SampleRatio: 0.5},
			wantErr: false,
		},
		{
			name:    "valid ratio boundaries",
			cfg:     Config{Enabled: true, ServiceName: "api", OTLPEndpoint: "localhost:4317", SampleRatio: 1},
			wantErr: false,
		},
		{
			name:    "missing service name",
			cfg:     Config{Enabled: true, OTLPEndpoint: "localhost:4317"},
			wantErr: true,
		},
		{
			name:    "missing OTLP endpoint",
			cfg:     Config{Enabled: true, ServiceName: "api"},
			wantErr: true,
		},
		{
			name:    "sample ratio below zero",
			cfg:     Config{Enabled: true, ServiceName: "api", OTLPEndpoint: "localhost:4317", SampleRatio: -0.01},
			wantErr: true,
		},
		{
			name:    "sample ratio above one",
			cfg:     Config{Enabled: true, ServiceName: "api", OTLPEndpoint: "localhost:4317", SampleRatio: 1.01},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigFromEnv_defaults(t *testing.T) {
	blankEnv(t)

	cfg := ConfigFromEnv()
	if !cfg.Enabled {
		t.Error("expected Enabled default true")
	}
	if !cfg.TracingEnabled {
		t.Error("expected TracingEnabled default true")
	}
	if !cfg.MetricsEnabled {
		t.Error("expected MetricsEnabled default true")
	}
	if cfg.ServiceName != "" {
		t.Errorf("expected empty ServiceName, got %q", cfg.ServiceName)
	}
	if cfg.OTLPEndpoint != "" {
		t.Errorf("expected empty OTLPEndpoint, got %q", cfg.OTLPEndpoint)
	}
	if cfg.Insecure {
		t.Error("expected Insecure default false")
	}
	if cfg.SampleRatio != 0 {
		t.Errorf("expected SampleRatio default 0, got %f", cfg.SampleRatio)
	}
}

func TestConfigFromEnv_custom(t *testing.T) {
	blankEnv(t)

	t.Setenv("OBSERVABILITY_ENABLED", "false")
	t.Setenv("OBSERVABILITY_TRACING_ENABLED", "0")
	t.Setenv("OBSERVABILITY_METRICS_ENABLED", "TRUE")
	t.Setenv("OTEL_SERVICE_NAME", "my-service")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "collector:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "1")
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0.25")

	cfg := ConfigFromEnv()
	if cfg.Enabled {
		t.Error("expected Enabled=false")
	}
	if cfg.TracingEnabled {
		t.Error("expected TracingEnabled=false")
	}
	if !cfg.MetricsEnabled {
		t.Error("expected MetricsEnabled=true")
	}
	if cfg.ServiceName != "my-service" {
		t.Errorf("got ServiceName %q, want %q", cfg.ServiceName, "my-service")
	}
	if cfg.OTLPEndpoint != "collector:4317" {
		t.Errorf("got OTLPEndpoint %q, want %q", cfg.OTLPEndpoint, "collector:4317")
	}
	if !cfg.Insecure {
		t.Error("expected Insecure=true")
	}
	if cfg.SampleRatio != 0.25 {
		t.Errorf("got SampleRatio %f, want 0.25", cfg.SampleRatio)
	}
}

func TestConfigFromEnv_sampler(t *testing.T) {
	tests := []struct {
		name       string
		sampler    string
		samplerArg string
		want       float64
	}{
		{name: "always_off forces ratio to zero", sampler: "always_off", samplerArg: "0.5", want: 0},
		{name: "always_on keeps configured ratio", sampler: "always_on", samplerArg: "0.5", want: 0.5},
		{name: "traceidratio keeps configured ratio", sampler: "traceidratio", samplerArg: "0.3", want: 0.3},
		{name: "parentbased_traceidratio keeps configured ratio", sampler: "parentbased_traceidratio", samplerArg: "0.7", want: 0.7},
		{name: "unknown sampler keeps configured ratio", sampler: "custom", samplerArg: "0.4", want: 0.4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blankEnv(t)
			t.Setenv("OTEL_TRACES_SAMPLER", tt.sampler)
			t.Setenv("OTEL_TRACES_SAMPLER_ARG", tt.samplerArg)

			cfg := ConfigFromEnv()
			if cfg.SampleRatio != tt.want {
				t.Fatalf("got SampleRatio %f, want %f", cfg.SampleRatio, tt.want)
			}
		})
	}
}

func Test_boolEnv(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantSet bool
		def     bool
		want    bool
	}{
		{name: "unset falls back to default", value: "", wantSet: false, def: true, want: true},
		{name: "true", value: "true", wantSet: true, def: true, want: true},
		{name: "1", value: "1", wantSet: true, def: false, want: true},
		{name: "false", value: "false", wantSet: true, def: false, want: false},
		{name: "0", value: "0", wantSet: true, def: true, want: false},
		{name: "invalid falls back to default", value: "yes", wantSet: true, def: false, want: false},
		{name: "invalid falls back to default true", value: "banana", wantSet: true, def: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantSet {
				t.Setenv("TEST_BOOL_ENV", tt.value)
			}
			if got := boolEnv("TEST_BOOL_ENV", tt.def); got != tt.want {
				t.Fatalf("boolEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_float64Env(t *testing.T) {
	t.Run("unset falls back to default", func(t *testing.T) {
		blankEnv(t)
		t.Setenv("TEST_FLOAT_ENV", "")
		if got := float64Env("TEST_FLOAT_ENV", 0.42); got != 0.42 {
			t.Fatalf("float64Env() = %v, want 0.42", got)
		}
	})

	t.Run("valid parse", func(t *testing.T) {
		blankEnv(t)
		t.Setenv("TEST_FLOAT_ENV", "0.75")
		if got := float64Env("TEST_FLOAT_ENV", 0); got != 0.75 {
			t.Fatalf("float64Env() = %v, want 0.75", got)
		}
	})

	t.Run("invalid parse falls back to default", func(t *testing.T) {
		blankEnv(t)
		t.Setenv("TEST_FLOAT_ENV", "not-a-number")
		if got := float64Env("TEST_FLOAT_ENV", 0.1); got != 0.1 {
			t.Fatalf("float64Env() = %v, want 0.1", got)
		}
	})
}
