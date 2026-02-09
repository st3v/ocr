package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// version is set via ldflags at build time
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	outputDir := flag.String("o", "", "Output directory (default: same directory as input)")
	extractMetadata := flag.Bool("m", false, "Extract image metadata (description, type, structured data)")
	annotationSchema := flag.String("a", "", "Extract document data using JSON schema file")
	quiet := flag.Bool("q", false, "Quiet mode (suppress progress output)")
	verbose := flag.Bool("v", false, "Verbose mode (extra details to stderr)")
	showVersion := flag.Bool("version", false, "Print version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `ocr - Extract Markdown, images, and image metadata from documents using LLMs

Usage: %s [options] <document>

Description:
  Uses large language models to extract content from documents:
  - Text content (as Markdown) saved to <basename>.md
  - Embedded images saved to images/ directory
  - Optional image metadata: descriptions, types, and structured data
    extracted from charts, graphs, tables, and diagrams
  - Optional document-level structured data extraction via JSON schema

  Supported formats: PDF, images (PNG, JPEG, GIF, WebP)

  Uses Mistral OCR with built-in annotation support for structured extraction.

  Prints the path to the output Markdown file on stdout.
  Progress messages are written to stderr.

Options:
`, os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Output Structure:
  <output-dir>/
  ├── <basename>.md              # Extracted text in Markdown format
  ├── <basename>.annotation.json # Document annotation (with -a flag)
  └── images/
      ├── page_0_img_0.png       # Extracted images
      ├── page_0_img_0.json      # Image metadata (with -m flag)
      └── ...

Image Metadata JSON Format (with -m flag):
  {
    "description": "Brief description of image contents",
    "type": "graph|chart|diagram|table|photo|illustration|screenshot|other",
    "structured_data": { ... } or null
  }

Document Schema File Format (for -a flag):
  {
    "name": "schema_name",
    "schema": { <JSON Schema object> }
  }

Environment:
  MISTRAL_API_KEY   Required. API key for Mistral AI.

Examples:
  %s document.pdf
      Extract text and images to current directory

  %s -o ./output document.pdf
      Extract to ./output directory

  %s -m document.pdf
      Extract with image metadata analysis

  %s -a invoice_schema.json invoice.pdf
      Extract with document-level structured data

  %s -m -a schema.json document.pdf
      Extract with both image and document annotations
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
	}

	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return nil
	}

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	docPath := flag.Arg(0)

	if _, err := os.Stat(docPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", docPath)
	}

	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("MISTRAL_API_KEY environment variable is required")
	}

	outDir := *outputDir
	if outDir == "" {
		outDir = filepath.Dir(docPath)
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	report := NewReporter(os.Stderr, *quiet, *verbose)
	baseName := strings.TrimSuffix(filepath.Base(docPath), filepath.Ext(docPath))

	// Build OCR options
	opts := OCROptions{
		ExtractImageMetadata: *extractMetadata,
	}

	// Load document schema if specified
	if *annotationSchema != "" {
		schema, err := loadDocumentSchema(*annotationSchema)
		if err != nil {
			return fmt.Errorf("loading schema file: %w", err)
		}
		opts.DocumentSchema = schema
	}

	report.Progress("Processing: %s\n", docPath)

	client := NewClient(apiKey)
	resp, err := client.ProcessDocument(context.Background(), docPath, opts)
	if err != nil {
		return err
	}

	report.Progress("Extracted %d pages\n", len(resp.Pages))

	text, imageCount := extractText(resp)

	textPath := filepath.Join(outDir, baseName+".md")
	if err := os.WriteFile(textPath, []byte(text), 0644); err != nil {
		return fmt.Errorf("writing text file: %w", err)
	}

	report.Verbose("Wrote text to: %s\n", textPath)

	// Write document annotation if present
	if resp.DocumentAnnotation != nil {
		annotationPath := filepath.Join(outDir, baseName+".annotation.json")
		if err := saveAnnotation(resp.DocumentAnnotation, annotationPath); err != nil {
			return fmt.Errorf("writing document annotation: %w", err)
		}
		report.Verbose("Wrote document annotation to: %s\n", annotationPath)
	}

	if imageCount > 0 {
		if err := extractImages(resp, outDir, *extractMetadata, report); err != nil {
			return err
		}
	}

	fmt.Println(textPath)
	return nil
}

// loadDocumentSchema reads and parses a JSON schema file.
func loadDocumentSchema(path string) (*JSONSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var schema JSONSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, err
	}

	return &schema, nil
}

func extractText(resp *OCRResponse) (string, int) {
	var b strings.Builder
	var imageCount int

	for _, page := range resp.Pages {
		b.WriteString(page.Markdown)
		b.WriteString("\n\n")
		imageCount += len(page.Images)
	}

	return b.String(), imageCount
}

func extractImages(resp *OCRResponse, outDir string, extractMetadata bool, report *Reporter) error {
	imagesDir := filepath.Join(outDir, "images")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return fmt.Errorf("creating images directory: %w", err)
	}

	imageCount := countImages(resp)
	report.Progress("Extracting %d images\n", imageCount)

	imgIndex := 0
	for _, page := range resp.Pages {
		for _, img := range page.Images {
			imgPath, err := saveImage(img, page.Index, imgIndex, imagesDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				imgIndex++
				continue
			}

			report.Verbose("Wrote image: %s\n", imgPath)

			// Save annotation metadata if present (from bbox_annotation_format)
			if extractMetadata && img.ImageAnnotation != nil {
				if err := saveAnnotationMetadata(img.ImageAnnotation, imgPath); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving metadata for %s: %v\n", imgPath, err)
				}
			}

			imgIndex++
		}
	}

	return nil
}

func countImages(resp *OCRResponse) int {
	count := 0
	for _, page := range resp.Pages {
		count += len(page.Images)
	}
	return count
}

func saveImage(img Image, pageIndex, imgIndex int, imagesDir string) (string, error) {
	b64Data := img.ImageBase64
	if idx := strings.Index(b64Data, ","); idx != -1 {
		b64Data = b64Data[idx+1:]
	}

	imgData, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return "", fmt.Errorf("decoding image: %w", err)
	}

	ext := imageExtension(img.ImageBase64)
	imgPath := filepath.Join(imagesDir, fmt.Sprintf("page_%d_img_%d%s", pageIndex, imgIndex, ext))

	if err := os.WriteFile(imgPath, imgData, 0644); err != nil {
		return "", fmt.Errorf("writing image: %w", err)
	}

	return imgPath, nil
}

func imageExtension(dataURL string) string {
	switch {
	case strings.Contains(dataURL, "image/jpeg"):
		return ".jpg"
	case strings.Contains(dataURL, "image/gif"):
		return ".gif"
	case strings.Contains(dataURL, "image/webp"):
		return ".webp"
	default:
		return ".png"
	}
}

// saveAnnotation writes an annotation to a file, handling string-encoded JSON.
func saveAnnotation(annotation any, path string) error {
	var data []byte
	var err error

	// If the annotation is a string, it's already JSON - parse and re-format it
	if str, ok := annotation.(string); ok {
		var parsed any
		if err := json.Unmarshal([]byte(str), &parsed); err != nil {
			return fmt.Errorf("parsing annotation JSON string: %w", err)
		}
		data, err = json.MarshalIndent(parsed, "", "  ")
	} else {
		data, err = json.MarshalIndent(annotation, "", "  ")
	}

	if err != nil {
		return fmt.Errorf("marshaling annotation: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing annotation: %w", err)
	}

	return nil
}

func saveAnnotationMetadata(annotation any, imgPath string) error {
	metadataPath := strings.TrimSuffix(imgPath, filepath.Ext(imgPath)) + ".json"
	return saveAnnotation(annotation, metadataPath)
}
