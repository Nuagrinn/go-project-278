package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

const testBaseURL = "https://short.test"

func TestPing(t *testing.T) {
	router := newTestRouter()
	response := performRequest(router, http.MethodGet, "/ping", "")

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	if response.Body.String() != "pong" {
		t.Fatalf("expected body %q, got %q", "pong", response.Body.String())
	}
}

func TestCORSPreflight(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	router := newTestRouter()

	request := httptest.NewRequest(http.MethodOptions, "/api/links", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "Content-Type")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, response.Code)
	}

	assertHeader(t, response, "Access-Control-Allow-Origin", "http://localhost:5173")
}

func TestCreateLink(t *testing.T) {
	router := newTestRouter()

	link := createTestLink(t, router, `{"original_url":"https://example.com/long-url","short_name":"exmpl"}`)

	assertLink(t, link, linkResponse{
		ID:          1,
		OriginalURL: "https://example.com/long-url",
		ShortName:   "exmpl",
		ShortURL:    testBaseURL + "/r/exmpl",
	})
}

func TestCreateLinkGeneratesShortName(t *testing.T) {
	router := newTestRouter()

	link := createTestLink(t, router, `{"original_url":"https://example.com/long-url"}`)

	if link.ID != 1 {
		t.Fatalf("expected link ID 1, got %d", link.ID)
	}

	if len(link.ShortName) != defaultShortNameLength {
		t.Fatalf("expected generated short_name length %d, got %d", defaultShortNameLength, len(link.ShortName))
	}

	if link.ShortURL != testBaseURL+"/r/"+link.ShortName {
		t.Fatalf("expected generated short_url, got %q", link.ShortURL)
	}
}

func TestListLinks(t *testing.T) {
	router := newTestRouter()

	first := createTestLink(t, router, `{"original_url":"https://example.com/one","short_name":"one"}`)
	second := createTestLink(t, router, `{"original_url":"https://example.com/two","short_name":"two"}`)

	response := performRequest(router, http.MethodGet, "/api/links", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var links []linkResponse
	decodeResponse(t, response, &links)

	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}

	assertHeader(t, response, "Accept-Ranges", "links")
	assertHeader(t, response, "Content-Range", "links 0-1/2")
	assertLink(t, links[0], first)
	assertLink(t, links[1], second)
}

func TestListLinksWithPagination(t *testing.T) {
	router := newTestRouter()
	createSeedLinks(t, router, 12)

	response := performRequest(router, http.MethodGet, "/api/links?range=%5B0,9%5D", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var firstPage []linkResponse
	decodeResponse(t, response, &firstPage)

	if len(firstPage) != 10 {
		t.Fatalf("expected 10 links, got %d", len(firstPage))
	}

	assertHeader(t, response, "Accept-Ranges", "links")
	assertHeader(t, response, "Content-Range", "links 0-9/12")
	assertLinkID(t, firstPage[0], 1)
	assertLinkID(t, firstPage[9], 10)

	response = performRequest(router, http.MethodGet, "/api/links?range=%5B5,%209%5D", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var secondPage []linkResponse
	decodeResponse(t, response, &secondPage)

	if len(secondPage) != 5 {
		t.Fatalf("expected 5 links, got %d", len(secondPage))
	}

	assertHeader(t, response, "Content-Range", "links 5-9/12")
	assertLinkID(t, secondPage[0], 6)
	assertLinkID(t, secondPage[4], 10)
}

func TestListLinksWithPaginationClampsEnd(t *testing.T) {
	router := newTestRouter()
	createSeedLinks(t, router, 3)

	response := performRequest(router, http.MethodGet, "/api/links?range=%5B0,9%5D", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var links []linkResponse
	decodeResponse(t, response, &links)

	if len(links) != 3 {
		t.Fatalf("expected 3 links, got %d", len(links))
	}

	assertHeader(t, response, "Content-Range", "links 0-2/3")
}

func TestListLinksWithPaginationBeyondTotal(t *testing.T) {
	router := newTestRouter()
	createSeedLinks(t, router, 3)

	response := performRequest(router, http.MethodGet, "/api/links?range=%5B5,9%5D", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var links []linkResponse
	decodeResponse(t, response, &links)

	if len(links) != 0 {
		t.Fatalf("expected 0 links, got %d", len(links))
	}

	assertHeader(t, response, "Content-Range", "links */3")
}

func TestListLinksRejectsInvalidRange(t *testing.T) {
	router := newTestRouter()

	for _, path := range []string{
		"/api/links?range=broken",
		"/api/links?range=%5B2,1%5D",
		"/api/links?range=%5B-1,1%5D",
	} {
		response := performRequest(router, http.MethodGet, path, "")
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected status %d, got %d", path, http.StatusBadRequest, response.Code)
		}
	}
}

func TestGetLink(t *testing.T) {
	router := newTestRouter()

	created := createTestLink(t, router, `{"original_url":"https://example.com/long-url","short_name":"exmpl"}`)

	response := performRequest(router, http.MethodGet, "/api/links/1", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var link linkResponse
	decodeResponse(t, response, &link)
	assertLink(t, link, created)
}

func TestUpdateLink(t *testing.T) {
	router := newTestRouter()
	createTestLink(t, router, `{"original_url":"https://example.com/old","short_name":"old"}`)

	response := performRequest(router, http.MethodPut, "/api/links/1", `{"original_url":"https://example.com/new","short_name":"new"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var link linkResponse
	decodeResponse(t, response, &link)

	assertLink(t, link, linkResponse{
		ID:          1,
		OriginalURL: "https://example.com/new",
		ShortName:   "new",
		ShortURL:    testBaseURL + "/r/new",
	})
}

func TestUpdateLinkGeneratesShortName(t *testing.T) {
	router := newTestRouter()
	createTestLink(t, router, `{"original_url":"https://example.com/old","short_name":"old"}`)

	response := performRequest(router, http.MethodPut, "/api/links/1", `{"original_url":"https://example.com/new"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var link linkResponse
	decodeResponse(t, response, &link)

	if link.ID != 1 {
		t.Fatalf("expected link ID 1, got %d", link.ID)
	}
	if link.OriginalURL != "https://example.com/new" {
		t.Fatalf("expected updated original_url, got %q", link.OriginalURL)
	}
	if len(link.ShortName) != defaultShortNameLength {
		t.Fatalf("expected generated short_name length %d, got %d", defaultShortNameLength, len(link.ShortName))
	}
}

func TestDeleteLink(t *testing.T) {
	router := newTestRouter()
	createTestLink(t, router, `{"original_url":"https://example.com/long-url","short_name":"exmpl"}`)

	response := performRequest(router, http.MethodDelete, "/api/links/1", "")
	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, response.Code)
	}

	response = performRequest(router, http.MethodGet, "/api/links/1", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func TestRedirectLink(t *testing.T) {
	router := newTestRouter()
	createTestLink(t, router, `{"original_url":"https://example.com/long-url","short_name":"exmpl"}`)

	response := performRequest(router, http.MethodGet, "/r/exmpl", "")
	if response.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, response.Code)
	}

	if location := response.Header().Get("Location"); location != "https://example.com/long-url" {
		t.Fatalf("expected redirect location, got %q", location)
	}
}

func TestRedirectLinkRecordsVisit(t *testing.T) {
	router := newTestRouter()
	createTestLink(t, router, `{"original_url":"https://example.com/long-url","short_name":"exmpl"}`)

	response := performRequestWithHeaders(router, http.MethodGet, "/r/exmpl", "", map[string]string{
		"CF-Connecting-IP": "203.0.113.10",
		"Referer":          "https://example.org/source",
		"User-Agent":       "curl/8.5.0",
	})
	if response.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, response.Code)
	}

	response = performRequest(router, http.MethodGet, "/api/link_visits", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var visits []linkVisitResponse
	decodeResponse(t, response, &visits)

	if len(visits) != 1 {
		t.Fatalf("expected 1 link visit, got %d", len(visits))
	}

	expected := linkVisitResponse{
		ID:        1,
		LinkID:    1,
		CreatedAt: visits[0].CreatedAt,
		IP:        "203.0.113.10",
		UserAgent: "curl/8.5.0",
		Referer:   "https://example.org/source",
		Status:    http.StatusFound,
	}
	if visits[0] != expected {
		t.Fatalf("expected visit %+v, got %+v", expected, visits[0])
	}

	assertHeader(t, response, "Accept-Ranges", "link_visits")
	assertHeader(t, response, "Content-Range", "link_visits 0-0/1")
}

func TestListLinkVisitsWithPagination(t *testing.T) {
	router := newTestRouter()
	createTestLink(t, router, `{"original_url":"https://example.com/long-url","short_name":"exmpl"}`)

	for range 12 {
		response := performRequest(router, http.MethodGet, "/r/exmpl", "")
		if response.Code != http.StatusFound {
			t.Fatalf("expected status %d, got %d", http.StatusFound, response.Code)
		}
	}

	response := performRequestWithHeaders(router, http.MethodGet, "/api/link_visits", "", map[string]string{
		"Range": "[5, 9]",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var visits []linkVisitResponse
	decodeResponse(t, response, &visits)

	if len(visits) != 5 {
		t.Fatalf("expected 5 link visits, got %d", len(visits))
	}

	assertHeader(t, response, "Accept-Ranges", "link_visits")
	assertHeader(t, response, "Content-Range", "link_visits 5-9/12")
	assertVisitID(t, visits[0], 6)
	assertVisitID(t, visits[4], 10)
}

func TestLinkNotFound(t *testing.T) {
	router := newTestRouter()

	for _, request := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/links/404"},
		{method: http.MethodPut, path: "/api/links/404", body: `{"original_url":"https://example.com","short_name":"exmpl"}`},
		{method: http.MethodDelete, path: "/api/links/404"},
		{method: http.MethodGet, path: "/r/missing"},
	} {
		response := performRequest(router, request.method, request.path, request.body)
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s %s: expected status %d, got %d", request.method, request.path, http.StatusNotFound, response.Code)
		}
	}
}

func TestCreateLinkRejectsDuplicateShortName(t *testing.T) {
	router := newTestRouter()
	body := `{"original_url":"https://example.com/long-url","short_name":"exmpl"}`
	createTestLink(t, router, body)

	response := performRequest(router, http.MethodPost, "/api/links", body)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, response.Code)
	}

	assertValidationError(t, response, "short_name", "short name already in use")
}

func TestCreateLinkRejectsInvalidURL(t *testing.T) {
	router := newTestRouter()

	response := performRequest(router, http.MethodPost, "/api/links", `{"original_url":"not-a-url","short_name":"exmpl"}`)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, response.Code)
	}

	assertValidationErrorContains(t, response, "original_url", "failed on the 'url' tag")
}

func TestCreateLinkRejectsInvalidShortName(t *testing.T) {
	router := newTestRouter()

	for _, body := range []string{
		`{"original_url":"https://example.com","short_name":"xy"}`,
		`{"original_url":"https://example.com","short_name":"abcdefghijklmnopqrstuvwxyzabcdefg"}`,
		`{"original_url":"https://example.com","short_name":"not valid"}`,
	} {
		response := performRequest(router, http.MethodPost, "/api/links", body)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, response.Code)
		}

		assertValidationErrorContains(t, response, "short_name", "Field validation")
	}
}

func TestCreateLinkRejectsInvalidJSON(t *testing.T) {
	router := newTestRouter()

	response := performRequest(router, http.MethodPost, "/api/links", `{"original_url":`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}

	var body errorResponse
	decodeResponse(t, response, &body)
	if body.Error != "invalid request" {
		t.Fatalf("expected error %q, got %q", "invalid request", body.Error)
	}
}

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	return setupRouter(newMemoryStore(), testBaseURL)
}

func createTestLink(t *testing.T, router http.Handler, body string) linkResponse {
	t.Helper()

	response := performRequest(router, http.MethodPost, "/api/links", body)
	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d; body: %s", http.StatusCreated, response.Code, response.Body.String())
	}

	var link linkResponse
	decodeResponse(t, response, &link)

	return link
}

func createSeedLinks(t *testing.T, router http.Handler, count int) {
	t.Helper()

	for i := range count {
		body := fmt.Sprintf(
			`{"original_url":"https://example.com/%02d","short_name":"seed-%02d"}`,
			i+1,
			i+1,
		)
		createTestLink(t, router, body)
	}
}

func performRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	return performRequestWithHeaders(router, method, path, body, nil)
}

func performRequestWithHeaders(
	router http.Handler,
	method string,
	path string,
	body string,
	headers map[string]string,
) *httptest.ResponseRecorder {
	requestBody := strings.NewReader(body)
	if body != "" {
		requestBody = strings.NewReader(body)
	}

	request := httptest.NewRequest(method, path, requestBody)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	return response
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()

	if err := json.NewDecoder(bytes.NewReader(response.Body.Bytes())).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func assertLink(t *testing.T, actual, expected linkResponse) {
	t.Helper()

	if actual != expected {
		t.Fatalf("expected link %+v, got %+v", expected, actual)
	}
}

func assertLinkID(t *testing.T, actual linkResponse, expectedID int64) {
	t.Helper()

	if actual.ID != expectedID {
		t.Fatalf("expected link ID %d, got %d", expectedID, actual.ID)
	}
}

func assertVisitID(t *testing.T, actual linkVisitResponse, expectedID int64) {
	t.Helper()

	if actual.ID != expectedID {
		t.Fatalf("expected link visit ID %d, got %d", expectedID, actual.ID)
	}
}

func assertValidationError(t *testing.T, response *httptest.ResponseRecorder, field, expected string) {
	t.Helper()

	var body validationErrorResponse
	decodeResponse(t, response, &body)

	if actual := body.Errors[field]; actual != expected {
		t.Fatalf("expected validation error for %s %q, got %q", field, expected, actual)
	}
}

func assertValidationErrorContains(t *testing.T, response *httptest.ResponseRecorder, field, expected string) {
	t.Helper()

	var body validationErrorResponse
	decodeResponse(t, response, &body)

	if actual := body.Errors[field]; !strings.Contains(actual, expected) {
		t.Fatalf("expected validation error for %s to contain %q, got %q", field, expected, actual)
	}
}

func assertHeader(t *testing.T, response *httptest.ResponseRecorder, name, expected string) {
	t.Helper()

	if actual := response.Header().Get(name); actual != expected {
		t.Fatalf("expected %s header %q, got %q", name, expected, actual)
	}
}
