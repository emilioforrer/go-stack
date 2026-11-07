package observability

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// Config holds observability configuration.
type Config struct {
	Enabled        bool
	TracingEnabled bool
	MetricsEnabled bool

	ServiceName    string
	ServiceVersion string
	Environment    string

	OTLPEndpoint string
	Insecure     bool
	SampleRatio  float64
}

// Sentinel validation errors returned from Config.Validate.
var (
	ErrMissingServiceName  = errors.New("service name is required")
	ErrMissingOTLPEndpoint = errors.New("OTLP endpoint is required")
	ErrInvalidSampleRatio  = errors.New("sample ratio must be between 0 and 1")
)

// Validate checks the configuration for required fields.
// It returns nil when the configuration is completely disabled.
func (cfg Config) Validate() error {
	if !cfg.Enabled {
		return nil
	}

	if cfg.ServiceName == "" {
		return ErrMissingServiceName
	}

	if cfg.OTLPEndpoint == "" {
		return ErrMissingOTLPEndpoint
	}

	if cfg.SampleRatio < 0 || cfg.SampleRatio > 1 {
		return ErrInvalidSampleRatio
	}

	return nil
}

// WithDefaults returns a copy of cfg with default values applied.
func (cfg Config) WithDefaults() Config {
	if cfg.SampleRatio == 0 && cfg.TracingEnabled {
		cfg.SampleRatio = 1.0
	}
	return cfg
}

// ConfigFromEnv builds a Config from standard and custom environment variables.
func ConfigFromEnv() Config {
	cfg := Config{
		Enabled:        boolEnv("OBSERVABILITY_ENABLED", true),
		TracingEnabled: boolEnv("OBSERVABILITY_TRACING_ENABLED", true),
		MetricsEnabled: boolEnv("OBSERVABILITY_METRICS_ENABLED", true),
		ServiceName:    os.Getenv("OTEL_SERVICE_NAME"),
		OTLPEndpoint:   os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		Insecure:       boolEnv("OTEL_EXPORTER_OTLP_INSECURE", false),
		SampleRatio:    float64Env("OTEL_TRACES_SAMPLER_ARG", 0),
	}

	if v := os.Getenv("OTEL_TRACES_SAMPLER"); v != "" {
		// Parse common sampler strings to a ratio.
		switch v {
		case "always_on", "traceidratio", "parentbased_traceidratio":
			// Keep whatever ratio is configured.
		case "always_off":
			cfg.SampleRatio = 0
		}
	}

	return cfg
}

func boolEnv(key string, defaultValue bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func float64Env(key string, defaultValue float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: invalid float %s=%s: %v\n", key, v, err)
		return defaultValue
	}
	return parsed
}
