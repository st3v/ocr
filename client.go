package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const (
	defaultBaseURL = "https://api.mistral.ai/v1"
	ocrModel       = "mistral-ocr-latest"
)

// OCROptions configures the OCR request.
type OCROptions struct {
	ExtractImageMetadata bool
	DocumentSchema       *JSONSchema
}

// ImageMetadataSchema is the built-in schema for bbox annotations.
var ImageMetadataSchema = JSONSchema{
	Name: "image_metadata",
	Schema: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"description": map[string]any{
				"type":        "string",
				"description": "Brief description of what the image shows",
			},
			"type": map[string]any{
				"type":        "string",
				"enum":        []string{"graph", "chart", "diagram", "table", "photo", "illustration", "screenshot", "other"},
				"description": "The type of image",
			},
			"structured_data": map[string]any{
				"type":        "object",
				"description": "Extracted structured data from the image. For charts/graphs include: chart_type, title, x_axis (with label and values/categories), y_axis (with label, unit, range), data_series (array with name and values), legend, and annotations. For tables include: headers and rows. For diagrams include: elements and relationships. Null for photos/illustrations.",
				"properties": map[string]any{
					"chart_type": map[string]any{
						"type":        "string",
						"description": "Type of chart: bar, line, scatter, pie, area, etc.",
					},
					"title": map[string]any{
						"type":        "string",
						"description": "Title of the chart or table",
					},
					"x_axis": map[string]any{
						"type":        "object",
						"description": "X-axis information with label, values or categories",
						"properties": map[string]any{
							"label":      map[string]any{"type": "string"},
							"categories": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							"values":     map[string]any{"type": "array"},
						},
					},
					"y_axis": map[string]any{
						"type":        "object",
						"description": "Y-axis information with label, unit, and range",
						"properties": map[string]any{
							"label": map[string]any{"type": "string"},
							"unit":  map[string]any{"type": "string"},
							"range": map[string]any{"type": "array", "items": map[string]any{"type": "number"}},
						},
					},
					"data_series": map[string]any{
						"type":        "array",
						"description": "Data series with name/label and values",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"name":   map[string]any{"type": "string"},
								"label":  map[string]any{"type": "string"},
								"values": map[string]any{"type": "array"},
							},
						},
					},
					"legend": map[string]any{
						"type":        "array",
						"description": "Legend entries with label and color",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"label": map[string]any{"type": "string"},
								"color": map[string]any{"type": "string"},
							},
						},
					},
					"annotations": map[string]any{
						"type":        "array",
						"description": "Statistical annotations or markers on the chart",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"symbol":   map[string]any{"type": "string"},
								"meaning":  map[string]any{"type": "string"},
								"location": map[string]any{"type": "string"},
							},
						},
					},
					"headers": map[string]any{
						"type":        "array",
						"description": "Table column headers",
						"items":       map[string]any{"type": "string"},
					},
					"rows": map[string]any{
						"type":        "array",
						"description": "Table rows as arrays of cell values",
						"items": map[string]any{
							"type":  "array",
							"items": map[string]any{},
						},
					},
					"elements": map[string]any{
						"type":        "array",
						"description": "Diagram elements/nodes",
						"items":       map[string]any{"type": "object"},
					},
					"relationships": map[string]any{
						"type":        "array",
						"description": "Diagram relationships/connections between elements",
						"items":       map[string]any{"type": "object"},
					},
				},
			},
		},
		"required": []string{"description", "type"},
	},
}

// Client is the Mistral OCR API client.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Mistral OCR client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		httpClient: http.DefaultClient,
	}
}

// ProcessPDF reads a PDF file and sends it to the Mistral OCR API.
func (c *Client) ProcessPDF(ctx context.Context, pdfPath string) (*OCRResponse, error) {
	return c.ProcessDocument(ctx, pdfPath, OCROptions{})
}

// ProcessDocument reads a document file and sends it to the Mistral OCR API with options.
func (c *Client) ProcessDocument(ctx context.Context, docPath string, opts OCROptions) (*OCRResponse, error) {
	docData, err := os.ReadFile(docPath)
	if err != nil {
		return nil, fmt.Errorf("reading PDF file: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(docData)
	dataURL := "data:application/pdf;base64," + encoded

	req := OCRRequest{
		Model: ocrModel,
		Document: DocumentURL{
			Type:        "document_url",
			DocumentURL: dataURL,
		},
		IncludeImageBase64: true,
	}

	if opts.ExtractImageMetadata {
		req.BBoxAnnotationFormat = &AnnotationFormat{
			Type:       "json_schema",
			JSONSchema: ImageMetadataSchema,
		}
	}

	if opts.DocumentSchema != nil {
		req.DocumentAnnotationFormat = &AnnotationFormat{
			Type:       "json_schema",
			JSONSchema: *opts.DocumentSchema,
		}
	}

	return c.doRequest(ctx, req)
}

// doRequest sends the OCR request to the Mistral API.
func (c *Client) doRequest(ctx context.Context, ocrReq OCRRequest) (*OCRResponse, error) {
	body, err := json.Marshal(ocrReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/ocr", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var ocrResp OCRResponse
	if err := json.Unmarshal(respBody, &ocrResp); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	return &ocrResp, nil
}

