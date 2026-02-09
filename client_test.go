package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessPDF_Success(t *testing.T) {
	expectedResponse := OCRResponse{
		Pages: []Page{
			{
				Index:    0,
				Markdown: "# Test Document\n\nThis is a test.",
				Images: []Image{
					{
						ID:           "img_0",
						ImageBase64:  base64.StdEncoding.EncodeToString([]byte("fake image data")),
						TopLeftX:     0,
						TopLeftY:     0,
						BottomRightX: 100,
						BottomRightY: 100,
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		if r.URL.Path != "/ocr" {
			t.Errorf("expected /ocr, got %s", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Error("expected Bearer token in Authorization header")
		}

		var req OCRRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		if req.Model != ocrModel {
			t.Errorf("expected model %s, got %s", ocrModel, req.Model)
		}

		if !req.IncludeImageBase64 {
			t.Error("expected IncludeImageBase64 to be true")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.baseURL = server.URL

	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4 fake pdf"), 0644); err != nil {
		t.Fatalf("failed to create test PDF: %v", err)
	}

	resp, err := client.ProcessPDF(context.Background(), pdfPath)
	if err != nil {
		t.Fatalf("ProcessPDF failed: %v", err)
	}

	if len(resp.Pages) != 1 {
		t.Errorf("expected 1 page, got %d", len(resp.Pages))
	}

	if resp.Pages[0].Markdown != expectedResponse.Pages[0].Markdown {
		t.Errorf("markdown mismatch")
	}

	if len(resp.Pages[0].Images) != 1 {
		t.Errorf("expected 1 image, got %d", len(resp.Pages[0].Images))
	}
}

func TestProcessPDF_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid api key"}`))
	}))
	defer server.Close()

	client := NewClient("bad-api-key")
	client.baseURL = server.URL

	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4 fake pdf"), 0644); err != nil {
		t.Fatalf("failed to create test PDF: %v", err)
	}

	_, err := client.ProcessPDF(context.Background(), pdfPath)
	if err == nil {
		t.Fatal("expected error for unauthorized request")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected status 401 in error, got: %v", err)
	}
}

func TestProcessPDF_FileNotFound(t *testing.T) {
	client := NewClient("test-api-key")

	_, err := client.ProcessPDF(context.Background(), "/nonexistent/path/to/file.pdf")
	if err == nil {
		t.Fatal("expected error for missing file")
	}

	if !strings.Contains(err.Error(), "reading PDF file") {
		t.Errorf("expected file reading error, got: %v", err)
	}
}

func TestImageDecoding(t *testing.T) {
	originalData := []byte("test image content")
	encoded := base64.StdEncoding.EncodeToString(originalData)

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if string(decoded) != string(originalData) {
		t.Errorf("decoded data mismatch")
	}
}

func TestAnalyzeImage_Success(t *testing.T) {
	expectedMetadata := ImageMetadata{
		Description:    "A photograph of a sunset over the ocean",
		Type:           "photo",
		StructuredData: nil,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected /chat/completions, got %s", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Error("expected Bearer token in Authorization header")
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		if req.Model != visionModel {
			t.Errorf("expected model %s, got %s", visionModel, req.Model)
		}

		if len(req.Messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(req.Messages))
		}

		// Return a mock response
		resp := ChatResponse{
			Choices: []ChatChoice{
				{
					Message: ChatMessage{
						Role:    "assistant",
						Content: `{"description": "A photograph of a sunset over the ocean", "type": "photo", "structured_data": null}`,
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.baseURL = server.URL

	metadata, err := client.AnalyzeImage(context.Background(), "data:image/png;base64,fakeimagedata")
	if err != nil {
		t.Fatalf("AnalyzeImage failed: %v", err)
	}

	if metadata.Description != expectedMetadata.Description {
		t.Errorf("description mismatch: expected %q, got %q", expectedMetadata.Description, metadata.Description)
	}

	if metadata.Type != expectedMetadata.Type {
		t.Errorf("type mismatch: expected %q, got %q", expectedMetadata.Type, metadata.Type)
	}

	if metadata.StructuredData != nil {
		t.Errorf("expected nil structured_data, got %v", metadata.StructuredData)
	}
}

func TestAnalyzeImage_GraphExtraction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{
			Choices: []ChatChoice{
				{
					Message: ChatMessage{
						Role: "assistant",
						Content: `{
							"description": "Bar chart showing quarterly revenue",
							"type": "chart",
							"structured_data": {
								"chart_type": "bar",
								"title": "Quarterly Revenue",
								"x_axis": {"label": "Quarter", "values": ["Q1", "Q2", "Q3", "Q4"]},
								"y_axis": {"label": "Revenue", "unit": "USD"},
								"data_series": [{"name": "Revenue", "values": [100, 150, 200, 175]}]
							}
						}`,
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.baseURL = server.URL

	metadata, err := client.AnalyzeImage(context.Background(), "data:image/png;base64,fakeimagedata")
	if err != nil {
		t.Fatalf("AnalyzeImage failed: %v", err)
	}

	if metadata.Description != "Bar chart showing quarterly revenue" {
		t.Errorf("description mismatch: got %q", metadata.Description)
	}

	if metadata.Type != "chart" {
		t.Errorf("type mismatch: expected 'chart', got %q", metadata.Type)
	}

	if metadata.StructuredData == nil {
		t.Fatal("expected structured_data to be populated")
	}

	structuredData, ok := metadata.StructuredData.(map[string]any)
	if !ok {
		t.Fatalf("expected structured_data to be a map, got %T", metadata.StructuredData)
	}

	if chartType, ok := structuredData["chart_type"].(string); !ok || chartType != "bar" {
		t.Errorf("expected chart_type 'bar', got %v", structuredData["chart_type"])
	}
}

func TestAnalyzeImage_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid api key"}`))
	}))
	defer server.Close()

	client := NewClient("bad-api-key")
	client.baseURL = server.URL

	_, err := client.AnalyzeImage(context.Background(), "data:image/png;base64,fakeimagedata")
	if err == nil {
		t.Fatal("expected error for unauthorized request")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected status 401 in error, got: %v", err)
	}
}
