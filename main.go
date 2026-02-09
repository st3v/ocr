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

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	outputDir := flag.String("o", "", "Output directory (default: same directory as input)")
	extractMetadata := flag.Bool("m", false, "Extract image metadata (description, type, structured data)")
	quiet := flag.Bool("q", false, "Quiet mode (suppress progress output)")
	verbose := flag.Bool("v", false, "Verbose mode (extra details to stderr)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `ocr - Extract Markdown, images, and image metadata from documents using LLMs

Usage: %s [options] <document>

Description:
  Uses large language models to extract content from documents:
  - Text content (as Markdown) saved to <basename>.md
  - Embedded images saved to images/ directory
  - Optional image metadata: descriptions, types, and structured data
    extracted from charts, graphs, tables, and diagrams

  Supported formats: PDF, images (PNG, JPEG, GIF, WebP)

  Currently uses Mistral OCR for text/image extraction and Pixtral for
  image analysis. Model backends may become pluggable in future versions.

  Prints the path to the output Markdown file on stdout.
  Progress messages are written to stderr.

Options:
`, os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Output Structure:
  <output-dir>/
  ├── <basename>.md           # Extracted text in Markdown format
  └── images/
      ├── page_0_img_0.png    # Extracted images
      ├── page_0_img_0.json   # Image metadata (with -m flag)
      └── ...

Metadata JSON Format (with -m flag):
  {
    "description": "Brief description of image contents",
    "type": "graph|chart|diagram|table|photo|illustration|screenshot|other",
    "structured_data": { ... } or null
  }

  For graphs/charts, structured_data includes: chart_type, title, axes, data_series
  For tables: headers, rows
  For photos/illustrations: null

Environment:
  MISTRAL_API_KEY   Required. API key for Mistral AI.

Examples:
  %s document.pdf
      Extract text and images to current directory

  %s -o ./output document.pdf
      Extract to ./output directory

  %s -m document.pdf
      Extract with image metadata analysis

  %s -q -m document.pdf
      Extract with metadata, suppress progress output
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0])
	}

	flag.Parse()

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

	report.Progress("Processing: %s\n", docPath)

	client := NewClient(apiKey)
	resp, err := client.ProcessPDF(context.Background(), docPath)
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

	if imageCount > 0 {
		if err := extractImages(resp, outDir, client, *extractMetadata, report); err != nil {
			return err
		}
	}

	fmt.Println(textPath)
	return nil
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

func extractImages(resp *OCRResponse, outDir string, client *Client, extractMetadata bool, report *Reporter) error {
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

			if extractMetadata {
				report.Progress("\rAnalyzing image %d/%d...", imgIndex+1, imageCount)
				if err := analyzeAndSaveMetadata(client, img, imgPath); err != nil {
					fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
				}
			}

			imgIndex++
		}
	}

	if extractMetadata {
		report.Progress("\rAnalyzing image %d/%d... done\n", imageCount, imageCount)
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

func analyzeAndSaveMetadata(client *Client, img Image, imgPath string) error {
	imageDataURL := img.ImageBase64
	if !strings.HasPrefix(imageDataURL, "data:") {
		imageDataURL = "data:image/png;base64," + imageDataURL
	}

	metadata, err := client.AnalyzeImage(context.Background(), imageDataURL)
	if err != nil {
		return fmt.Errorf("analyzing image %s: %w", imgPath, err)
	}

	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	metadataPath := strings.TrimSuffix(imgPath, filepath.Ext(imgPath)) + ".json"
	if err := os.WriteFile(metadataPath, metadataJSON, 0644); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}

	return nil
}
