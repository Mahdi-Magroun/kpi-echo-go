package main

import (
	"github.com/labstack/echo/v4"
	"github.com/ybkuroki/go-webapp-sample/config"
	"github.com/ybkuroki/go-webapp-sample/container"
	"github.com/ybkuroki/go-webapp-sample/logger"
	"github.com/ybkuroki/go-webapp-sample/middleware"
	"github.com/ybkuroki/go-webapp-sample/migration"
	"github.com/ybkuroki/go-webapp-sample/repository"
	"github.com/ybkuroki/go-webapp-sample/router"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	// "net/http"
	"time"
)

// @title go-webapp-sample API
// @version 1.5.1
// @description This is API specification for go-webapp-sample project.
// @license.name MIT
// @license.url https://opensource.org/licenses/mit-license.php
// @host localhost:8080
// @BasePath /api

// Prometheus metrics
var (
	requestDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests in seconds",
		Buckets: prometheus.DefBuckets,
	})

	errorCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "http_errors_total",
		Help: "Total number of HTTP errors",
	})
)

// Initialize metrics
func init() {
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(errorCounter)
}

// Middleware to measure latency and count errors
func metricsMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()

		// Execute the next handler and capture any errors
		err := next(c)
		if err != nil {
			c.Error(err)
			errorCounter.Inc() // Increment the error counter if an error occurs
		}

		// Record the duration of the request
		duration := time.Since(start).Seconds()
		requestDuration.Observe(duration)

		return err
	}
}

func main() {
	e := echo.New()

	// Use the metrics middleware
	e.Use(metricsMiddleware)

	// Prometheus metrics endpoint
	e.GET("/prometheus", echo.WrapHandler(promhttp.Handler()))

	// Load configuration and initialize logger
	conf, env := config.Load()
	logger := logger.NewLogger(env)
	logger.GetZapLogger().Infof("Loaded configuration: application.%s.yml", env)

	// Initialize repository and container
	rep := repository.NewBookRepository(logger, conf)
	container := container.NewContainer(rep, conf, logger, env)

	// Run database migrations and initialize master data
	migration.CreateDatabase(container)
	migration.InitMasterData(container)

	// Initialize routers and middlewares
	router.Init(e, container)
	middleware.InitLoggerMiddleware(e, container)
	middleware.InitSessionMiddleware(e, container)

	// Serve static files if a path is provided
	if conf.StaticContents.Path != "" {
		e.Static("/", conf.StaticContents.Path)
		logger.GetZapLogger().Infof("Served static contents. Path: %s", conf.StaticContents.Path)
	}

	// Start the server
	if err := e.Start(":8000"); err != nil {
		logger.GetZapLogger().Errorf("Error starting server: %s", err.Error())
	}

	defer rep.Close() // Ensure the repository is closed on exit
}
