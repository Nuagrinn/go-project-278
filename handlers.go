package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

var (
	errInvalidRange = errors.New("range must be an array with two non-negative numbers, for example [0,9]")
)

const maxRangeLimit = int64(1<<31 - 1)

type linkHandler struct {
	store   linkStore
	baseURL string
}

type linkRequest struct {
	OriginalURL string `json:"original_url" binding:"required,url"`
	ShortName   string `json:"short_name" binding:"omitempty,min=3,max=32,shortname"`
}

type linkResponse struct {
	ID          int64  `json:"id"`
	OriginalURL string `json:"original_url"`
	ShortName   string `json:"short_name"`
	ShortURL    string `json:"short_url"`
}

type linkVisitResponse struct {
	ID        int64     `json:"id"`
	LinkID    int64     `json:"link_id"`
	CreatedAt time.Time `json:"created_at"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Referer   string    `json:"referer"`
	Status    int32     `json:"status"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type validationErrorResponse struct {
	Errors map[string]string `json:"errors"`
}

type listRange struct {
	Start int64
	End   int64
}

func newLinkHandler(store linkStore, baseURL string) *linkHandler {
	return &linkHandler{
		store:   store,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

func (h *linkHandler) listLinks(context *gin.Context) {
	total, err := h.store.CountLinks(context.Request.Context())
	if err != nil {
		writeError(context, http.StatusInternalServerError, "could not count links")

		return
	}

	requestedRange, err := parseRequestListRange(context, total)
	if err != nil {
		writeError(context, http.StatusBadRequest, err.Error())

		return
	}

	links, err := h.store.ListLinks(context.Request.Context(), listLinksParams{
		Offset: int32(requestedRange.Start),
		Limit:  int32(requestedRange.End - requestedRange.Start + 1),
	})
	if err != nil {
		writeError(context, http.StatusInternalServerError, "could not load links")

		return
	}

	response := make([]linkResponse, 0, len(links))
	for _, item := range links {
		response = append(response, h.toResponse(context, item))
	}

	writeRangeHeaders(context, "links", requestedRange, total, len(response))
	context.JSON(http.StatusOK, response)
}

func (h *linkHandler) listLinkVisits(context *gin.Context) {
	total, err := h.store.CountLinkVisits(context.Request.Context())
	if err != nil {
		writeError(context, http.StatusInternalServerError, "could not count link visits")

		return
	}

	requestedRange, err := parseRequestListRange(context, total)
	if err != nil {
		writeError(context, http.StatusBadRequest, err.Error())

		return
	}

	visits, err := h.store.ListLinkVisits(context.Request.Context(), listLinkVisitsParams{
		Offset: int32(requestedRange.Start),
		Limit:  int32(requestedRange.End - requestedRange.Start + 1),
	})
	if err != nil {
		writeError(context, http.StatusInternalServerError, "could not load link visits")

		return
	}

	response := make([]linkVisitResponse, 0, len(visits))
	for _, item := range visits {
		response = append(response, toVisitResponse(item))
	}

	writeRangeHeaders(context, "link_visits", requestedRange, total, len(response))
	context.JSON(http.StatusOK, response)
}

func (h *linkHandler) createLink(context *gin.Context) {
	var request linkRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		writeBindingError(context, err)

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
		writeBindingError(context, err)

		return
	}

	item, err := h.updateValidatedLink(context, id, request)
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
	shortName := context.Param("code")
	if shortName == "" {
		shortName = context.Param("shortName")
	}
	if !isValidShortName(shortName) {
		writeError(context, http.StatusNotFound, "link not found")

		return
	}

	item, err := h.store.GetLinkByShortName(context.Request.Context(), shortName)
	if err != nil {
		h.handleStoreError(context, err)

		return
	}

	status := int32(http.StatusFound)
	if _, err := h.store.CreateLinkVisit(context.Request.Context(), createLinkVisitParams{
		LinkID:    item.ID,
		IP:        context.ClientIP(),
		UserAgent: context.Request.UserAgent(),
		Referer:   context.Request.Referer(),
		Status:    status,
	}); err != nil {
		writeError(context, http.StatusInternalServerError, "could not record link visit")

		return
	}

	context.Redirect(http.StatusFound, item.OriginalURL)
}

func (h *linkHandler) createValidatedLink(context *gin.Context, request linkRequest) (link, error) {
	originalURL := strings.TrimSpace(request.OriginalURL)
	shortName := strings.TrimSpace(request.ShortName)

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

func (h *linkHandler) updateValidatedLink(context *gin.Context, id int64, request linkRequest) (link, error) {
	originalURL := strings.TrimSpace(request.OriginalURL)
	shortName := strings.TrimSpace(request.ShortName)

	if shortName != "" {
		return h.store.UpdateLink(context.Request.Context(), updateLinkParams{
			ID:          id,
			OriginalURL: originalURL,
			ShortName:   shortName,
		})
	}

	for range maxGenerateAttempts {
		generated, err := generateShortName(defaultShortNameLength)
		if err != nil {
			return link{}, err
		}

		item, err := h.store.UpdateLink(context.Request.Context(), updateLinkParams{
			ID:          id,
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

func toVisitResponse(item linkVisit) linkVisitResponse {
	return linkVisitResponse(item)
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
		writeValidationErrors(context, map[string]string{"short_name": "short name already in use"})
	default:
		writeError(context, http.StatusInternalServerError, "internal server error")
	}
}

func parseID(context *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(context.Param("id"), 10, 64)

	return id, err == nil && id > 0
}

func parseRequestListRange(context *gin.Context, total int64) (listRange, error) {
	rawRange := context.Query("range")
	if rawRange == "" {
		rawRange = context.GetHeader("Range")
	}

	return parseListRange(rawRange, total)
}

func parseListRange(rawRange string, total int64) (listRange, error) {
	if rawRange == "" {
		if total == 0 {
			return listRange{Start: 0, End: 0}, nil
		}

		end := min(total-1, maxRangeLimit-1)

		return listRange{Start: 0, End: end}, nil
	}

	var values []int64
	if err := json.Unmarshal([]byte(rawRange), &values); err != nil {
		return listRange{}, errInvalidRange
	}

	if len(values) != 2 || values[0] < 0 || values[1] < values[0] {
		return listRange{}, errInvalidRange
	}

	rangeLimit := values[1] - values[0] + 1
	if values[0] > maxRangeLimit || rangeLimit > maxRangeLimit {
		return listRange{}, errInvalidRange
	}

	return listRange{Start: values[0], End: values[1]}, nil
}

func writeRangeHeaders(context *gin.Context, unit string, requestedRange listRange, total int64, itemsCount int) {
	context.Header("Accept-Ranges", unit)
	context.Header("Access-Control-Expose-Headers", "Content-Range, Accept-Ranges")

	if total == 0 || itemsCount == 0 {
		context.Header("Content-Range", fmt.Sprintf("%s */%d", unit, total))

		return
	}

	actualEnd := requestedRange.Start + int64(itemsCount) - 1
	context.Header("Content-Range", fmt.Sprintf("%s %d-%d/%d", unit, requestedRange.Start, actualEnd, total))
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

func writeBindingError(context *gin.Context, err error) {
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		messages := make(map[string]string, len(validationErrors))
		for _, fieldError := range validationErrors {
			messages[fieldError.Field()] = fieldError.Error()
		}

		writeValidationErrors(context, messages)

		return
	}

	writeError(context, http.StatusBadRequest, "invalid request")
}

func writeValidationErrors(context *gin.Context, messages map[string]string) {
	context.JSON(http.StatusUnprocessableEntity, validationErrorResponse{Errors: messages})
}
