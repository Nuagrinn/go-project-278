package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	flushSentry := initSentry()
	defer flushSentry()

	store, closeStore, err := openStore()
	if err != nil {
		log.Fatal(err)
	}
	defer closeStore()

	router := setupRouter(store, appBaseURL())

	if err := router.Run(serverAddress()); err != nil {
		log.Fatal(err)
	}
}

func setupRouter(store linkStore, baseURL string) *gin.Engine {
	registerValidators()

	router := gin.New()
	router.TrustedPlatform = gin.PlatformCloudflare

	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	if sentry.CurrentHub().Client() != nil {
		router.Use(sentryMiddleware())
	}

	router.GET("/ping", func(context *gin.Context) {
		context.String(http.StatusOK, "pong")
	})

	links := newLinkHandler(store, baseURL)
	router.GET("/r/:code", links.redirectLink)

	api := router.Group("/api")
	api.GET("/links", links.listLinks)
	api.POST("/links", links.createLink)
	api.GET("/links/:id", links.getLink)
	api.PUT("/links/:id", links.updateLink)
	api.DELETE("/links/:id", links.deleteLink)
	api.GET("/link_visits", links.listLinkVisits)

	if os.Getenv("ENABLE_SENTRY_TEST_ENDPOINT") == "true" {
		router.GET("/debug/sentry", captureTestError)
	}

	return router
}

func registerValidators() {
	if validatorEngine, ok := binding.Validator.Engine().(*validator.Validate); ok {
		validatorEngine.RegisterTagNameFunc(jsonFieldName)
		if err := validatorEngine.RegisterValidation("shortname", validateShortNameField); err != nil {
			log.Printf("could not register shortname validator: %v", err)
		}
	}
}

func jsonFieldName(field reflect.StructField) string {
	name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
	if name == "-" {
		return ""
	}
	if name == "" {
		return field.Name
	}

	return name
}

func validateShortNameField(field validator.FieldLevel) bool {
	return isValidShortName(field.Field().String())
}

func openStore() (linkStore, func(), error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Println("DATABASE_URL is empty; using in-memory storage")

		return newMemoryStore(), func() {}, nil
	}

	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()

		return nil, nil, err
	}

	store := newPostgresStore(database)

	return store, func() {
		if err := store.Close(); err != nil {
			log.Printf("could not close database: %v", err)
		}
	}, nil
}

func initSentry() func() {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		return func() {}
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		EnableTracing:    true,
		TracesSampleRate: 1.0,
	}); err != nil {
		log.Printf("sentry initialization failed: %v", err)

		return func() {}
	}

	return func() {
		sentry.Flush(2 * time.Second)
	}
}

func serverAddress() string {
	port := os.Getenv("BACKEND_PORT")
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = "8080"
	}

	host := os.Getenv("BACKEND_HOST")
	if host != "" {
		return net.JoinHostPort(host, port)
	}

	return ":" + port
}

func appBaseURL() string {
	return strings.TrimRight(os.Getenv("BASE_URL"), "/")
}

func corsMiddleware() gin.HandlerFunc {
	config := cors.Config{
		AllowOrigins: corsAllowedOrigins(),
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"Origin",
			"Range",
		},
		ExposeHeaders: []string{
			"Accept-Ranges",
			"Content-Range",
		},
		MaxAge: 12 * time.Hour,
	}

	return cors.New(config)
}

func corsAllowedOrigins() []string {
	rawOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if rawOrigins == "" {
		return []string{"http://localhost:5173"}
	}

	origins := strings.Split(rawOrigins, ",")
	result := make([]string, 0, len(origins))
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			result = append(result, strings.TrimRight(origin, "/"))
		}
	}

	if len(result) == 0 {
		return []string{"http://localhost:5173"}
	}

	return result
}

func sentryMiddleware() gin.HandlerFunc {
	return sentrygin.New(sentrygin.Options{
		Repanic: true,
		Timeout: 2 * time.Second,
	})
}

func captureTestError(context *gin.Context) {
	err := errors.New("sentry test error")

	if hub := sentrygin.GetHubFromContext(context); hub != nil {
		hub.CaptureException(err)
	} else {
		sentry.CaptureException(err)
	}

	context.JSON(http.StatusInternalServerError, gin.H{"error": "test error sent to Sentry"})
}
