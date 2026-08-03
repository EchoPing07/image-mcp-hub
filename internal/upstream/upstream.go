package upstream

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GenerateRequest is the input to Generate.
type GenerateRequest struct {
	Model  string
	Prompt string
	Params map[string]any // size, n, quality, style, background, output_format
}

// Image is one generated image with its bytes and provenance.
type Image struct {
	Data     []byte
	Ext      string
	Upstream map[string]any // the raw upstream item (url/b64_json/...) for metadata
}

// Result holds all images returned by one generation call.
type Result struct {
	Images  []Image
	Created int64
}

// StatusError preserves an upstream HTTP status so callers can surface it.
type StatusError struct {
	Code int
	Body string
}

func (e *StatusError) Error() string { return fmt.Sprintf("upstream %d: %s", e.Code, e.Body) }

var client = &http.Client{Timeout: 5 * time.Minute}

// Generate calls baseURL + "/v1/images/generations" with the OpenAI-compatible
// request body. response_format is never sent: we download url results and
// decode b64_json results ourselves.
func Generate(ctx context.Context, baseURL, apiKey string, req GenerateRequest) (*Result, error) {
	body := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
	}
	for k, v := range req.Params {
		body[k] = v
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(endpoint, "/v1") {
		endpoint += "/images/generations"
	} else {
		endpoint += "/v1/images/generations"
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &StatusError{Code: resp.StatusCode, Body: string(raw)}
	}

	// Parse OpenAI-compatible shape: { created, data: [ {url}|{b64_json} ] }
	var parsed struct {
		Created int64            `json:"created"`
		Data    []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse upstream response: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("upstream returned no images")
	}

	out := &Result{Created: parsed.Created}
	for _, item := range parsed.Data {
		img, err := convertItem(ctx, item, req.Params)
		if err != nil {
			return nil, fmt.Errorf("process image: %w", err)
		}
		out.Images = append(out.Images, img)
	}
	return out, nil
}

func convertItem(ctx context.Context, item map[string]any, params map[string]any) (Image, error) {
	img := Image{Upstream: item}
	// ext from output_format param if provided, else from url, else png
	if of, ok := params["output_format"].(string); ok && of != "" {
		img.Ext = of
	}
	if u, ok := item["url"].(string); ok && u != "" {
		data, ext, err := download(ctx, u)
		if err != nil {
			return img, fmt.Errorf("download %s: %w", u, err)
		}
		img.Data = data
		if img.Ext == "" {
			img.Ext = ext
		}
		return img, nil
	}
	if b64, ok := item["b64_json"].(string); ok && b64 != "" {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return img, fmt.Errorf("decode b64_json: %w", err)
		}
		img.Data = data
		if img.Ext == "" {
			img.Ext = "png"
		}
		return img, nil
	}
	return img, fmt.Errorf("image item has neither url nor b64_json")
}

func download(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download status %d", resp.StatusCode)
	}
	return data, extFromContentType(resp.Header.Get("Content-Type")), nil
}

func extFromContentType(ct string) string {
	ct = strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	switch ct {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	default:
		return "png"
	}
}
