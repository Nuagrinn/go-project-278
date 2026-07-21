package main

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	errInvalidOriginalURL = errors.New("original_url must be a valid http or https url")
	errInvalidShortName   = errors.New("short_name must be 3-64 characters and contain only letters, numbers, '_' or '-'")
)

type linkHandler struct {
	store   linkStore
	baseURL string
}

type linkRequest struct {
	OriginalURL string `json:"original_url"`
	ShortName   string `json:"short_name"`
}

type linkResponse struct {
	ID          int64  `json:"id"`
	OriginalURL string `json:"original_url"`
	ShortName   string `json:"short_name"`
	ShortURL    string `json:"short_url"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func newLinkHandler(store linkStore, baseURL string) *linkHandler {
	return &linkHandler{
		store:   store,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

func (h *linkHandler) listLinks(context *gin.Context) {
	links, err := h.store.ListLinks(context.Request.Context())
	if err != nil {
		writeError(context, http.StatusInternalServerError, "could not load links")

		return
	}

	response := make([]linkResponse, 0, len(links))
	for _, item := range links {
		response = append(response, h.toResponse(context, item))
	}

	context.JSON(http.StatusOK, response)
}

func (h *linkHandler) createLink(context *gin.Context) {
	var request linkRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		writeError(context, http.StatusBadRequest, "request body must be valid json")

		return
	}

	item, err := h.createValidatedLink(context, request)
	if err != nil {
		h.handleStoreError(context, err)

		return
	}

	context.JSON(http.StatusCreated, h.toResponse(context, item))
}

func (h *linkHandler) getLink(context *gin.Context) {
	id, ok := parseID(context)
	if !ok {
		writeError(context, http.StatusNotFound, "link not found")

		return
	}

	item, err := h.store.GetLink(context.Request.Context(), id)
	if err != nil {
		h.handleStoreError(context, err)

		return
	}

	context.JSON(http.StatusOK, h.toResponse(context, item))
}

func (h *linkHandler) updateLink(context *gin.Context) {
	id, ok := parseID(context)
	if !ok {
		writeError(context, http.StatusNotFound, "link not found")

		return
	}

	var request linkRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		writeError(context, http.StatusBadRequest, "request body must be valid json")

		return
	}

	originalURL, shortName, err := validateLinkRequest(request, true)
	if err != nil {
		h.handleStoreError(context, err)

		return
	}

	item, err := h.store.UpdateLink(context.Request.Context(), updateLinkParams{
		ID:          id,
		OriginalURL: originalURL,
		ShortName:   shortName,
	})
	if err != nil {
		h.handleStoreError(context, err)

		return
	}

	context.JSON(http.StatusOK, h.toResponse(context, item))
}

func (h *linkHandler) deleteLink(context *gin.Context) {
	id, ok := parseID(context)
	if !ok {
		writeError(context, http.StatusNotFound, "link not found")

		return
	}

	if err := h.store.DeleteLink(context.Request.Context(), id); err != nil {
		h.handleStoreError(context, err)

		return
	}

	context.Status(http.StatusNoContent)
}

func (h *linkHandler) redirectLink(context *gin.Context) {
	shortName := context.Param("shortName")
	if !isValidShortName(shortName) {
		writeError(context, http.StatusNotFound, "link not found")

		return
	}

	item, err := h.store.GetLinkByShortName(context.Request.Context(), shortName)
	if err != nil {
		h.handleStoreError(context, err)

		return
	}

	context.Redirect(http.StatusFound, item.OriginalURL)
}

func (h *linkHandler) createValidatedLink(context *gin.Context, request linkRequest) (link, error) {
	originalURL, shortName, err := validateLinkRequest(request, false)
	if err != nil {
		return link{}, err
	}

	if shortName != "" {
		return h.store.CreateLink(context.Request.Context(), createLinkParams{
			OriginalURL: originalURL,
			ShortName:   shortName,
		})
	}

	for range maxGenerateAttempts {
		generated, err := generateShortName(defaultShortNameLength)
		if err != nil {
			return link{}, err
		}

		item, err := h.store.CreateLink(context.Request.Context(), createLinkParams{
			OriginalURL: originalURL,
			ShortName:   generated,
		})
		if errors.Is(err, errDuplicateShortName) {
			continue
		}

		return item, err
	}

	return link{}, errDuplicateShortName
}

func (h *linkHandler) toResponse(context *gin.Context, item link) linkResponse {
	return linkResponse{
		ID:          item.ID,
		OriginalURL: item.OriginalURL,
		ShortName:   item.ShortName,
		ShortURL:    h.shortURL(context, item.ShortName),
	}
}

func (h *linkHandler) shortURL(context *gin.Context, shortName string) string {
	baseURL := h.baseURL
	if baseURL == "" {
		baseURL = requestBaseURL(context)
	}

	return baseURL + "/r/" + shortName
}

func (h *linkHandler) handleStoreError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, errLinkNotFound):
		writeError(context, http.StatusNotFound, "link not found")
	case errors.Is(err, errDuplicateShortName):
		writeError(context, http.StatusConflict, "short_name already exists")
	case errors.Is(err, errInvalidOriginalURL), errors.Is(err, errInvalidShortName):
		writeError(context, http.StatusUnprocessableEntity, err.Error())
	default:
		writeError(context, http.StatusInternalServerError, "internal server error")
	}
}

func validateLinkRequest(request linkRequest, requireShortName bool) (string, string, error) {
	originalURL := strings.TrimSpace(request.OriginalURL)
	if !isValidOriginalURL(originalURL) {
		return "", "", errInvalidOriginalURL
	}

	shortName := strings.TrimSpace(request.ShortName)
	if shortName == "" {
		if requireShortName {
			return "", "", errInvalidShortName
		}

		return originalURL, "", nil
	}

	if !isValidShortName(shortName) {
		return "", "", errInvalidShortName
	}

	return originalURL, shortName, nil
}

func isValidOriginalURL(value string) bool {
	parsedURL, err := url.Parse(value)
	if err != nil {
		return false
	}

	return parsedURL.Host != "" && (parsedURL.Scheme == "http" || parsedURL.Scheme == "https")
}

func parseID(context *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(context.Param("id"), 10, 64)

	return id, err == nil && id > 0
}

func requestBaseURL(context *gin.Context) string {
	protocol := context.GetHeader("X-Forwarded-Proto")
	if protocol == "" {
		if context.Request.TLS != nil {
			protocol = "https"
		} else {
			protocol = "http"
		}
	}

	return protocol + "://" + context.Request.Host
}

func writeError(context *gin.Context, status int, message string) {
	context.JSON(status, errorResponse{Error: message})
}
