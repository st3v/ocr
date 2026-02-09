package main

// OCRRequest represents the request body for the Mistral OCR API.
type OCRRequest struct {
	Model              string       `json:"model"`
	Document           DocumentURL  `json:"document"`
	IncludeImageBase64 bool         `json:"include_image_base64"`
}

// DocumentURL wraps the document data URL.
type DocumentURL struct {
	Type        string `json:"type"`
	DocumentURL string `json:"document_url"`
}

// OCRResponse represents the response from the Mistral OCR API.
type OCRResponse struct {
	Pages []Page `json:"pages"`
}

// Page represents a single page in the OCR response.
type Page struct {
	Index    int     `json:"index"`
	Markdown string  `json:"markdown"`
	Images   []Image `json:"images"`
}

// Image represents an extracted image from the document.
type Image struct {
	ID           string `json:"id"`
	TopLeftX     int    `json:"top_left_x"`
	TopLeftY     int    `json:"top_left_y"`
	BottomRightX int    `json:"bottom_right_x"`
	BottomRightY int    `json:"bottom_right_y"`
	ImageBase64  string `json:"image_base64"`
}

// ChatRequest represents a request to the Mistral chat/completions API.
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

// ChatMessage represents a message in the chat request.
type ChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// ContentPart represents a part of multi-modal content.
type ContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

// ChatResponse represents the response from the chat/completions API.
type ChatResponse struct {
	Choices []ChatChoice `json:"choices"`
}

// ChatChoice represents a choice in the chat response.
type ChatChoice struct {
	Message ChatMessage `json:"message"`
}

// ImageMetadata contains extracted metadata for an image.
type ImageMetadata struct {
	Description    string `json:"description"`
	Type           string `json:"type"`
	StructuredData any    `json:"structured_data"`
}
