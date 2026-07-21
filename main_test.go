package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := setupRouter()
	response := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/ping", nil)
	if err != nil {
		t.Fatal(err)
	}

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	if response.Body.String() != "pong" {
		t.Fatalf("expected body %q, got %q", "pong", response.Body.String())
	}
}
