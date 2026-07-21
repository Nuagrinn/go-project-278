package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
)

func main() {
	flushSentry := initSentry()
	defer flushSentry()

	router := setupRouter()

	if err := router.Run(serverAddress()); err != nil {
		log.Fatal(err)
	}
}

func setupRouter() *gin.Engine {
	router := gin.New()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	if sentry.CurrentHub().Client() != nil {
		router.Use(sentryMiddleware())
	}

	router.GET("/ping", func(context *gin.Context) {
		context.String(http.StatusOK, "pong")
	})

	if os.Getenv("ENABLE_SENTRY_TEST_ENDPOINT") == "true" {
		router.GET("/debug/sentry", captureTestError)
	}

	return router
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
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return ":" + port
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
