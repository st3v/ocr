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

func TestProcessDocument_WithBBoxAnnotation(t *testing.T) {
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
						ImageAnnotation: map[string]any{
							"description":     "A photograph of a sunset",
							"type":            "photo",
							"structured_data": nil,
						},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req OCRRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		// Verify bbox annotation format is set
		if req.BBoxAnnotationFormat == nil {
			t.Error("expected BBoxAnnotationFormat to be set")
		} else {
			if req.BBoxAnnotationFormat.Type != "json_schema" {
				t.Errorf("expected type 'json_schema', got %s", req.BBoxAnnotationFormat.Type)
			}
			if req.BBoxAnnotationFormat.JSONSchema.Name != "image_metadata" {
				t.Errorf("expected schema name 'image_metadata', got %s", req.BBoxAnnotationFormat.JSONSchema.Name)
			}
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

	opts := OCROptions{ExtractImageMetadata: true}
	resp, err := client.ProcessDocument(context.Background(), pdfPath, opts)
	if err != nil {
		t.Fatalf("ProcessDocument failed: %v", err)
	}

	if len(resp.Pages) != 1 {
		t.Errorf("expected 1 page, got %d", len(resp.Pages))
	}

	if len(resp.Pages[0].Images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(resp.Pages[0].Images))
	}

	img := resp.Pages[0].Images[0]
	if img.ImageAnnotation == nil {
		t.Fatal("expected annotation to be set")
	}

	annotation, ok := img.ImageAnnotation.(map[string]any)
	if !ok {
		t.Fatalf("expected annotation to be a map, got %T", img.ImageAnnotation)
	}

	if annotation["type"] != "photo" {
		t.Errorf("expected type 'photo', got %v", annotation["type"])
	}
}

func TestProcessDocument_WithDocumentAnnotation(t *testing.T) {
	expectedResponse := OCRResponse{
		Pages: []Page{
			{
				Index:    0,
				Markdown: "# Invoice\n\nVendor: ACME Corp\nTotal: $100.00",
				Images:   []Image{},
			},
		},
		DocumentAnnotation: map[string]any{
			"vendor": "ACME Corp",
			"total":  100.0,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req OCRRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		// Verify document annotation format is set
		if req.DocumentAnnotationFormat == nil {
			t.Error("expected DocumentAnnotationFormat to be set")
		} else {
			if req.DocumentAnnotationFormat.Type != "json_schema" {
				t.Errorf("expected type 'json_schema', got %s", req.DocumentAnnotationFormat.Type)
			}
			if req.DocumentAnnotationFormat.JSONSchema.Name != "invoice" {
				t.Errorf("expected schema name 'invoice', got %s", req.DocumentAnnotationFormat.JSONSchema.Name)
			}
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

	invoiceSchema := &JSONSchema{
		Name: "invoice",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"vendor": map[string]any{"type": "string"},
				"total":  map[string]any{"type": "number"},
			},
		},
	}

	opts := OCROptions{DocumentSchema: invoiceSchema}
	resp, err := client.ProcessDocument(context.Background(), pdfPath, opts)
	if err != nil {
		t.Fatalf("ProcessDocument failed: %v", err)
	}

	if resp.DocumentAnnotation == nil {
		t.Fatal("expected document annotation to be set")
	}

	annotation, ok := resp.DocumentAnnotation.(map[string]any)
	if !ok {
		t.Fatalf("expected document annotation to be a map, got %T", resp.DocumentAnnotation)
	}

	if annotation["vendor"] != "ACME Corp" {
		t.Errorf("expected vendor 'ACME Corp', got %v", annotation["vendor"])
	}
}

func TestProcessDocument_WithBothAnnotations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req OCRRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		// Verify both annotation formats are set
		if req.BBoxAnnotationFormat == nil {
			t.Error("expected BBoxAnnotationFormat to be set")
		}
		if req.DocumentAnnotationFormat == nil {
			t.Error("expected DocumentAnnotationFormat to be set")
		}

		resp := OCRResponse{
			Pages: []Page{
				{
					Index:    0,
					Markdown: "# Invoice",
					Images: []Image{
						{
							ID:          "img_0",
							ImageBase64: base64.StdEncoding.EncodeToString([]byte("fake")),
							ImageAnnotation: map[string]any{
								"description": "Company logo",
								"type":        "illustration",
							},
						},
					},
				},
			},
			DocumentAnnotation: map[string]any{
				"invoice_number": "INV-001",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.baseURL = server.URL

	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4 fake pdf"), 0644); err != nil {
		t.Fatalf("failed to create test PDF: %v", err)
	}

	opts := OCROptions{
		ExtractImageMetadata: true,
		DocumentSchema: &JSONSchema{
			Name:   "invoice",
			Schema: map[string]any{"type": "object"},
		},
	}
	resp, err := client.ProcessDocument(context.Background(), pdfPath, opts)
	if err != nil {
		t.Fatalf("ProcessDocument failed: %v", err)
	}

	// Verify both annotations are present
	if resp.DocumentAnnotation == nil {
		t.Error("expected document annotation")
	}
	if resp.Pages[0].Images[0].ImageAnnotation == nil {
		t.Error("expected image annotation")
	}
}
