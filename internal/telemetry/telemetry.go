package telemetry

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

const meterName = "github.com/Ayush330/uno_engine"

var (
	// For latency of game actions
	ActionDuration metric.Float64Histogram
	// For latency of broadcasting game state to players
	BroadcastDuration metric.Float64Histogram
)

type Config struct {
	GetActiveGames   func() int
	GetActivePlayers func() int
}

func Setup(cfg Config) (http.Handler, func(context.Context) error, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize prometheus exporter: %w", err)
	}
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)
	otel.SetMeterProvider(provider)
	meter := provider.Meter(meterName)

	// Register the game metrics
	ActionDuration, err = meter.Float64Histogram(
		"uno_action_duration_seconds",
		metric.WithDescription("Latency of player actions in the engine"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create action duration histogram: %w", err)
	}
	BroadcastDuration, err = meter.Float64Histogram(
		"uno_broadcast_duration_seconds",
		metric.WithDescription("Latency of websocket broadcasts to players"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create broadcast duration histogram: %w", err)
	}

	// 4. Register Observable Gauges (Active games & players)
	if cfg.GetActiveGames != nil {
		_, err = meter.Int64ObservableGauge(
			"uno_active_games",
			metric.WithDescription("Number of active rooms/games"),
			metric.WithInt64Callback(func(_ context.Context, obs metric.Int64Observer) error {
				obs.Observe(int64(cfg.GetActiveGames()))
				return nil
			}),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to register active games gauge: %w", err)
		}
	}

	if cfg.GetActivePlayers != nil {
		_, err = meter.Int64ObservableGauge(
			"uno_active_players",
			metric.WithDescription("Number of connected players across all rooms"),
			metric.WithInt64Callback(func(_ context.Context, obs metric.Int64Observer) error {
				obs.Observe(int64(cfg.GetActivePlayers()))
				return nil
			}),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to register active players gauge: %w", err)
		}
	}

	handler := promhttp.Handler()
	return handler, provider.Shutdown, nil
}