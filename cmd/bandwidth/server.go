package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/spf13/viper"

	"github.com/jwhumphries/bandwidth/internal/handlers"
	"github.com/jwhumphries/bandwidth/internal/static"
	"github.com/jwhumphries/bandwidth/version"
)

func runServer() error {
	logger := newLogger(viper.GetString("log_level"))

	srv := &http.Server{
		Addr:              viper.GetString("port"),
		Handler:           newEcho(logger),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting server", "addr", srv.Addr, "version", version.Version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func newEcho(logger *slog.Logger) *echo.Echo {
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(requestLogger(logger))

	e.GET("/healthz", handlers.Healthz)

	dist, err := fs.Sub(static.Dist, "dist")
	if err != nil {
		panic(err) // embed is checked at compile time; this cannot fail
	}
	handlers.RegisterSPA(e, dist)
	return e
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}

func requestLogger(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			// Snapshot method/path before next: the SPA handler rewrites
			// req.URL.Path to "/" on fallback.
			req := c.Request()
			method := req.Method
			path := req.URL.Path
			start := time.Now()
			err := next(c)
			_, status := echo.ResolveResponseStatus(c.Response(), err)
			logger.Info("request",
				"method", method,
				"path", path,
				"status", status,
				"duration", time.Since(start).String(),
			)
			return err
		}
	}
}
