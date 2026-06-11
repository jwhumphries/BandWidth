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
	"github.com/jwhumphries/bandwidth/internal/mail"
	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/repository"
	"github.com/jwhumphries/bandwidth/internal/static"
	"github.com/jwhumphries/bandwidth/version"
)

func runServer() error {
	logger := newLogger(viper.GetString("log_level"))

	repo, err := repository.Open(viper.GetString("db_path"))
	if err != nil {
		return err
	}
	defer func() {
		if err := repo.Close(); err != nil {
			logger.Error("closing database", "error", err)
		}
	}()
	mailer := mail.New(mail.Config{
		Host: viper.GetString("smtp_host"),
		Port: viper.GetInt("smtp_port"),
		User: viper.GetString("smtp_user"),
		Pass: viper.GetString("smtp_pass"),
		From: viper.GetString("smtp_from"),
	})
	if mailer.Enabled() {
		logger.Info("smtp configured, password reset enabled")
	}
	api := &handlers.API{
		Repo:          repo,
		Mailer:        mailer,
		Logger:        logger,
		BaseURL:       viper.GetString("base_url"),
		SecureCookies: viper.GetBool("secure_cookies"),
	}

	e, err := newEcho(logger, api)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              viper.GetString("port"),
		Handler:           e,
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

func newEcho(logger *slog.Logger, api *handlers.API) (*echo.Echo, error) {
	e := echo.New()
	// fly.io's edge proxy appends the real client IP to X-Forwarded-For;
	// the default legacy extractor trusts the leftmost (spoofable) entry.
	e.IPExtractor = echo.ExtractIPFromXFFHeader()
	e.Use(middleware.Recover())
	e.Use(requestLogger(logger))

	e.GET("/healthz", handlers.Healthz)

	csrfMW, err := middleware.CSRFConfig{
		CookiePath:     "/",
		CookieSameSite: http.SameSiteLaxMode,
		CookieSecure:   api.SecureCookies,
	}.ToMiddleware()
	if err != nil {
		return nil, err
	}

	apiGroup := e.Group("/api", csrfMW)

	// One shared store across signup/login/reset: a NAT'd office burns the
	// burst faster, but the 1 rps refill keeps the window short.
	authLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{Rate: 1, Burst: 5, ExpiresIn: 3 * time.Minute},
	))
	authGroup := apiGroup.Group("/auth")
	authGroup.POST("/signup", api.Signup, authLimiter)
	authGroup.POST("/login", api.Login, authLimiter)
	authGroup.POST("/logout", api.Logout)
	authGroup.GET("/features", api.Features)
	authGroup.POST("/password-reset", api.RequestPasswordReset, authLimiter)
	authGroup.POST("/password-reset/confirm", api.ConfirmPasswordReset, authLimiter)

	twofa := apiGroup.Group("/auth/2fa", appmw.RequireAuth(api.Repo))
	twofa.POST("/setup", api.TwoFactorSetup)
	twofa.POST("/verify", api.TwoFactorVerify)
	twofa.POST("/disable", api.TwoFactorDisable)

	me := apiGroup.Group("/me", appmw.RequireAuth(api.Repo))
	me.GET("", api.Me)
	me.PATCH("", api.UpdateMe)
	me.PUT("/password", api.ChangePassword)

	songs := apiGroup.Group("/songs", appmw.RequireAuth(api.Repo))
	songs.GET("", api.Songs)
	songs.POST("", api.CreateSong)
	songs.GET("/:id", api.Song)
	songs.PATCH("/:id", api.UpdateSong)
	songs.DELETE("/:id", api.DeleteSong)
	songs.PUT("/:id/practice", api.LogPractice)
	songs.DELETE("/:id/practice/:date", api.DeletePractice)
	songs.POST("/:id/resources", api.CreateResource)
	songs.PATCH("/:id/resources/:resourceId", api.UpdateResource)
	songs.DELETE("/:id/resources/:resourceId", api.DeleteResource)

	folders := apiGroup.Group("/folders", appmw.RequireAuth(api.Repo))
	folders.GET("", api.Folders)
	folders.POST("", api.CreateFolder)
	folders.PUT("/order", api.ReorderFolders)
	folders.PATCH("/:id", api.UpdateFolder)
	folders.DELETE("/:id", api.DeleteFolder)
	folders.PUT("/:id/entries", api.SetFolderEntries)

	dist, err := fs.Sub(static.Dist, "dist")
	if err != nil {
		return nil, err
	}
	handlers.RegisterSPA(e, dist)
	return e, nil
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
