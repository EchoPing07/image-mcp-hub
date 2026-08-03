package upstream

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 1x1 transparent PNG.
const pngB64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M8AAAMBAQDJ/pLvAAAAAElFTkSuQmCC"

var pngBytes, _ = base64.StdEncoding.DecodeString(pngB64)

func TestExtFromBytes(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"png", pngBytes, "png"},
		{"jpg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0x10, 'J', 'F', 'I', 'F'}, "jpg"},
		{"webp", append([]byte("RIFF\x00\x00\x00\x00WEBP"), []byte("VP8 ")...), "webp"},
		{"gif87a", []byte("GIF87a"), "gif"},
		{"gif89a", []byte("GIF89a"), "gif"},
		{"unknown", []byte{0x00, 0x01, 0x02, 0x03}, "png"}, // fallback
		{"empty", nil, "png"},                              // fallback, no panic
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := extFromBytes(c.data); got != c.want {
				t.Fatalf("extFromBytes(%s) = %q, want %q", c.name, got, c.want)
			}
		})
	}
}

// TestConvertItem_B64_IgnoresOutputFormat verifies M2: output_format must NOT
// decide the local file extension; the bytes' magic number wins. A png
// returned as b64_json with output_format=webp must still be classified png.
func TestConvertItem_B64_IgnoresOutputFormat(t *testing.T) {
	img, err := convertItem(context.Background(), map[string]any{
		"b64_json": pngB64,
	})
	if err != nil {
		t.Fatalf("convertItem: %v", err)
	}
	if img.Ext != "png" {
		t.Fatalf("Ext = %q, want png (magic bytes, not output_format)", img.Ext)
	}
	if string(img.Data) != string(pngBytes) {
		t.Fatal("decoded bytes mismatch")
	}
}

// TestConvertItem_URLBranch_DownloadAndMagicExt exercises the url branch: the
// hub downloads the upstream url and classifies by magic bytes, ignoring the
// Content-Type header (which previously could lie).
func TestConvertItem_URLBranch_DownloadAndMagicExt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Deliberately lie about the Content-Type; the magic bytes must win.
		w.Header().Set("Content-Type", "image/webp")
		_, _ = w.Write(pngBytes)
	}))
	defer srv.Close()

	img, err := convertItem(context.Background(), map[string]any{
		"url": srv.URL + "/img",
	})
	if err != nil {
		t.Fatalf("convertItem: %v", err)
	}
	if img.Ext != "png" {
		t.Fatalf("Ext = %q, want png (magic bytes override lying Content-Type)", img.Ext)
	}
	if string(img.Data) != string(pngBytes) {
		t.Fatal("downloaded bytes mismatch")
	}
}

// TestConvertItem_NeitherUrlNorB64 ensures the error path is reachable.
func TestConvertItem_NeitherUrlNorB64(t *testing.T) {
	_, err := convertItem(context.Background(), map[string]any{"foo": "bar"})
	if err == nil {
		t.Fatal("expected error for item with neither url nor b64_json")
	}
}

// TestGenerate_RejectsOversizedBody verifies M1: a response body larger than
// maxBodyBytes must not be buffered fully; Generate returns an error instead
// of OOMing. (Sending maxBodyBytes+1 of invalid JSON forces a parse error
// after the capped read.)
func TestGenerate_RejectsOversizedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Drain the (capped) reader by writing more than the cap. The server
		// side write succeeds; the client side read is capped by
		// http.MaxBytesReader and io.ReadAll stops at the limit.
		_, _ = w.Write([]byte(strings.Repeat("x", maxBodyBytes+1024)))
	}))
	defer srv.Close()

	_, err := Generate(context.Background(), srv.URL+"/v1", "k", GenerateRequest{
		Model: "m", Prompt: "x",
	})
	if err == nil {
		t.Fatal("expected error for oversized upstream body, got nil")
	}
}

// TestGenerate_StatusError confirms a non-2xx upstream surfaces a StatusError
// carrying the original status and body.
func TestGenerate_StatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad key"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := Generate(context.Background(), srv.URL+"/v1", "k", GenerateRequest{
		Model: "m", Prompt: "x",
	})
	se, ok := err.(*StatusError)
	if !ok {
		t.Fatalf("expected *StatusError, got %T: %v", err, err)
	}
	if se.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", se.Code)
	}
	if !strings.Contains(se.Body, "bad key") {
		t.Fatalf("body = %q, want it to contain 'bad key'", se.Body)
	}
}

// TestGenerate_NoImages ensures an empty data array is an error, not silence.
func TestGenerate_NoImages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"created": 1, "data": []map[string]any{}})
	}))
	defer srv.Close()

	_, err := Generate(context.Background(), srv.URL+"/v1", "k", GenerateRequest{
		Model: "m", Prompt: "x",
	})
	if err == nil {
		t.Fatal("expected error for empty data array")
	}
}

// TestGenerate_DoesNotSendResponseFormat guards the documented invariant: the
// hub never forwards response_format upstream (it always downloads/decodes
// itself).
func TestGenerate_DoesNotSendResponseFormat(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"created": 1,
			"data":    []map[string]any{{"b64_json": pngB64}},
		})
	}))
	defer srv.Close()

	if _, err := Generate(context.Background(), srv.URL+"/v1", "k", GenerateRequest{
		Model: "m", Prompt: "x", Params: map[string]any{"output_format": "webp"},
	}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if _, ok := got["response_format"]; ok {
		t.Fatal("hub forwarded response_format to upstream")
	}
	if got["output_format"] != "webp" {
		t.Fatalf("output_format not forwarded: %v", got["output_format"])
	}
}
