package main

// OCRRequest represents the request body for the Mistral OCR API.
type OCRRequest struct {
	Model                    string            `json:"model"`
	Document                 DocumentURL       `json:"document"`
	IncludeImageBase64       bool              `json:"include_image_base64"`
	BBoxAnnotationFormat     *AnnotationFormat `json:"bbox_annotation_format,omitempty"`
	DocumentAnnotationFormat *AnnotationFormat `json:"document_annotation_format,omitempty"`
}

// AnnotationFormat defines the schema for structured annotation extraction.
type AnnotationFormat struct {
	Type       string     `json:"type"`
	JSONSchema JSONSchema `json:"json_schema"`
}

// JSONSchema defines the schema for annotation extraction.
type JSONSchema struct {
	Name   string `json:"name"`
	Schema any    `json:"schema"`
}

// DocumentURL wraps the document data URL.
type DocumentURL struct {
	Type        string `json:"type"`
	DocumentURL string `json:"document_url"`
}

// OCRResponse represents the response from the Mistral OCR API.
type OCRResponse struct {
	Pages              []Page `json:"pages"`
	DocumentAnnotation any    `json:"document_annotation,omitempty"`
}

// Page represents a single page in the OCR response.
type Page struct {
	Index    int     `json:"index"`
	Markdown string  `json:"markdown"`
	Images   []Image `json:"images"`
}

// Image represents an extracted image from the document.
type Image struct {
	ID              string `json:"id"`
	TopLeftX        int    `json:"top_left_x"`
	TopLeftY        int    `json:"top_left_y"`
	BottomRightX    int    `json:"bottom_right_x"`
	BottomRightY    int    `json:"bottom_right_y"`
	ImageBase64     string `json:"image_base64"`
	ImageAnnotation any    `json:"image_annotation,omitempty"`
}

// ImageMetadata contains extracted metadata for an image.
type ImageMetadata struct {
	Description    string `json:"description"`
	Type           string `json:"type"`
	StructuredData any    `json:"structured_data"`
}
