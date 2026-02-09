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
	"strings"
)

const (
	defaultBaseURL = "https://api.mistral.ai/v1"
	ocrModel       = "mistral-ocr-latest"
	visionModel    = "pixtral-large-latest"
)

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
	pdfData, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("reading PDF file: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(pdfData)
	dataURL := "data:application/pdf;base64," + encoded

	req := OCRRequest{
		Model: ocrModel,
		Document: DocumentURL{
			Type:        "document_url",
			DocumentURL: dataURL,
		},
		IncludeImageBase64: true,
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

const imageAnalysisPrompt = `Analyze this image and respond with JSON only:
{
  "description": "<brief description of what the image shows>",
  "type": "<one of: graph, chart, diagram, table, photo, illustration, screenshot, other>",
  "structured_data": <if graph/chart/table/diagram, extract as structured JSON with labels, values, axes, etc. Otherwise null>
}

For graphs/charts include: chart_type, title, x_axis, y_axis, data_series, legend
For tables include: headers, rows
For diagrams include: elements, relationships`

// AnalyzeImage sends an image to the Mistral vision API to extract metadata.
func (c *Client) AnalyzeImage(ctx context.Context, imageBase64 string) (*ImageMetadata, error) {
	content := []ContentPart{
		{Type: "text", Text: imageAnalysisPrompt},
		{Type: "image_url", ImageURL: imageBase64},
	}

	chatReq := ChatRequest{
		Model: visionModel,
		Messages: []ChatMessage{
			{Role: "user", Content: content},
		},
	}

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
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

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned")
	}

	content_str, ok := chatResp.Choices[0].Message.Content.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected content type in response")
	}

	// Strip markdown code block wrapper if present
	jsonStr := strings.TrimSpace(content_str)
	if strings.HasPrefix(jsonStr, "```") {
		// Remove opening ```json or ```
		if idx := strings.Index(jsonStr, "\n"); idx != -1 {
			jsonStr = jsonStr[idx+1:]
		}
		// Remove closing ```
		if idx := strings.LastIndex(jsonStr, "```"); idx != -1 {
			jsonStr = jsonStr[:idx]
		}
		jsonStr = strings.TrimSpace(jsonStr)
	}

	// Parse the JSON response from the model
	var metadata ImageMetadata
	if err := json.Unmarshal([]byte(jsonStr), &metadata); err != nil {
		return nil, fmt.Errorf("parsing metadata JSON: %w", err)
	}

	return &metadata, nil
}
