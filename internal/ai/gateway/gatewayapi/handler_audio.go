package gatewayapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/Abraxas-365/freerouter/internal/ai/gateway"
	"github.com/Abraxas-365/freerouter/internal/ai/usage"
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/Abraxas-365/freerouter/internal/webhook"
	"github.com/gofiber/fiber/v2"
)

// Transcription handles POST /v1/audio/transcriptions (speech-to-text).
// The multipart form is rebuilt with the model rewritten to the provider's
// external ID and proxied upstream. Billed per minute of audio (or per token
// for token-billed STT models).
func (h *GatewayHandlers) Transcription(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}
	tenantID := authCtx.TenantID

	form, err := c.MultipartForm()
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid multipart form")
	}

	requestedModel := formValue(form, "model")
	if requestedModel == "" {
		return fiber.NewError(fiber.StatusBadRequest, "model is required")
	}
	files := form.File["file"]
	if len(files) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "file is required")
	}

	// Pre-flight checks
	if err := h.checkModelAccess(authCtx, requestedModel); err != nil {
		return err
	}
	if err := h.checkSpendingLimit(c, tenantID); err != nil {
		return err
	}
	if err := h.checkWalletBalance(c, authCtx); err != nil {
		return err
	}
	if err := h.checkRateLimit(c, tenantID); err != nil {
		return err
	}
	defer h.rateLimiter.Release(c.Context(), tenantID.String())

	// Resolve route
	route, err := h.router.ResolveWithCapability(c.Context(), requestedModel, &tenantID, nil, gateway.CapabilityAudio)
	if err != nil {
		return err
	}

	// Rebuild the multipart body with the external model ID
	body, contentType, err := rebuildMultipart(form, files[0], route.ExternalID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "failed to process upload")
	}

	// Call upstream
	start := time.Now()
	respBody, statusCode, err := h.upstream.CallTranscription(c.Context(), route, body, contentType)
	duration := time.Since(start)

	if err != nil {
		h.healthTracker.ReportError(route.KeyID, statusCode)
		h.logModalityRequest(tenantID, route, requestedModel, "transcription", nil, statusCode, duration, err, nil)
		return fiber.NewError(fiber.StatusBadGateway, "transcription request failed")
	}
	h.healthTracker.ReportSuccessWithLatency(route.KeyID, duration)

	// Parse for billing info (json/verbose_json formats)
	var trResp gateway.TranscriptionResponse
	_ = json.Unmarshal(respBody, &trResp)

	cost := gateway.CalculateTranscriptionCost(route, trResp.Duration, trResp.Usage)
	h.debitUsage(c.Context(), tenantID, authCtx.WalletID, cost)

	if h.metrics != nil {
		h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), "transcription", "ok", duration)
	}

	respMeta, _ := json.Marshal(map[string]any{
		"text_length": len(trResp.Text),
		"duration":    trResp.Duration,
		"language":    trResp.Language,
	})
	content := &usage.RequestContent{ResponseBody: respMeta, IsDebug: isDebugMode(c)}
	if content.IsDebug {
		content.RawResponse = respBody
	}
	h.logModalityRequest(tenantID, route, requestedModel, "transcription", nil, http.StatusOK, duration, nil, content)

	h.fireModalityWebhook(tenantID, requestedModel, route, "transcription", cost, duration)

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(respBody)
}

// Speech handles POST /v1/audio/speech (text-to-speech).
// Proxied in OpenAI-compatible format; billed per input character.
func (h *GatewayHandlers) Speech(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}
	tenantID := authCtx.TenantID

	var req gateway.SpeechRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if req.Model == "" {
		return fiber.NewError(fiber.StatusBadRequest, "model is required")
	}
	if req.Input == "" {
		return fiber.NewError(fiber.StatusBadRequest, "input is required")
	}

	requestedModel := req.Model

	if err := h.checkModelAccess(authCtx, requestedModel); err != nil {
		return err
	}
	if err := h.checkSpendingLimit(c, tenantID); err != nil {
		return err
	}
	if err := h.checkWalletBalance(c, authCtx); err != nil {
		return err
	}
	if err := h.checkRateLimit(c, tenantID); err != nil {
		return err
	}
	defer h.rateLimiter.Release(c.Context(), tenantID.String())

	route, err := h.router.Resolve(c.Context(), requestedModel, &tenantID, nil)
	if err != nil {
		return err
	}

	req.Model = route.ExternalID
	body, _ := json.Marshal(req)

	start := time.Now()
	audio, upstreamCT, statusCode, err := h.upstream.CallSpeech(c.Context(), route, body)
	duration := time.Since(start)

	if err != nil {
		h.healthTracker.ReportError(route.KeyID, statusCode)
		h.logModalityRequest(tenantID, route, requestedModel, "speech", nil, statusCode, duration, err, nil)
		return fiber.NewError(fiber.StatusBadGateway, "speech request failed")
	}
	h.healthTracker.ReportSuccessWithLatency(route.KeyID, duration)

	cost := gateway.CalculateSpeechCost(route, len(req.Input))
	h.debitUsage(c.Context(), tenantID, authCtx.WalletID, cost)

	if h.metrics != nil {
		h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), "speech", "ok", duration)
	}

	respMeta, _ := json.Marshal(map[string]any{
		"input_chars": len(req.Input),
		"voice":       req.Voice,
		"format":      req.ResponseFormat,
		"audio_bytes": len(audio),
	})
	content := &usage.RequestContent{ResponseBody: respMeta, IsDebug: isDebugMode(c)}
	h.logModalityRequest(tenantID, route, requestedModel, "speech", nil, http.StatusOK, duration, nil, content)

	h.fireModalityWebhook(tenantID, requestedModel, route, "speech", cost, duration)

	if upstreamCT == "" {
		upstreamCT = gateway.SpeechContentType(req.ResponseFormat)
	}
	c.Set("Content-Type", upstreamCT)
	return c.Status(http.StatusOK).Send(audio)
}

// formValue returns the first value for a multipart form field.
func formValue(form *multipart.Form, key string) string {
	if vals := form.Value[key]; len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// rebuildMultipart reconstructs the multipart body with the model field
// rewritten to the provider's external ID.
func rebuildMultipart(form *multipart.Form, file *multipart.FileHeader, externalModel string) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for key, vals := range form.Value {
		if key == "model" {
			continue
		}
		for _, v := range vals {
			if err := w.WriteField(key, v); err != nil {
				return nil, "", err
			}
		}
	}
	if err := w.WriteField("model", externalModel); err != nil {
		return nil, "", err
	}

	part, err := w.CreateFormFile("file", file.Filename)
	if err != nil {
		return nil, "", err
	}
	src, err := file.Open()
	if err != nil {
		return nil, "", err
	}
	defer src.Close()
	if _, err := io.Copy(part, src); err != nil {
		return nil, "", err
	}

	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}

// logModalityRequest logs a non-chat modality request through the usage service.
func (h *GatewayHandlers) logModalityRequest(
	tenantID kernel.TenantID,
	route *gateway.RouteResult,
	requestedModel string,
	modality string,
	usageData *gateway.Usage,
	statusCode int,
	duration time.Duration,
	reqErr error,
	content *usage.RequestContent,
) {
	resp := &gateway.ChatResponse{Object: modality, Usage: usageData}
	h.usage.LogRequest(tenantID, route, requestedModel, resp, statusCode, duration, false, reqErr, content)
}

// fireModalityWebhook fires a request-completed webhook for a modality request.
func (h *GatewayHandlers) fireModalityWebhook(
	tenantID kernel.TenantID,
	requestedModel string,
	route *gateway.RouteResult,
	modality string,
	cost float64,
	duration time.Duration,
) {
	if h.webhooks == nil {
		return
	}
	h.webhooks.Fire(tenantID, webhook.EventRequestCompleted, map[string]any{
		"type":            modality,
		"requested_model": requestedModel,
		"provider":        route.ProviderID.String(),
		"total_cost_usd":  cost,
		"duration_ms":     duration.Milliseconds(),
	})
}
