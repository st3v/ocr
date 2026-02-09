# ocr

Extract Markdown, images, and structured metadata from documents using Mistral OCR.

## Features

- Extract text content as Markdown
- Extract embedded images with bounding box coordinates
- Optional image metadata: descriptions, types, and structured data from charts, graphs, tables, and diagrams
- Optional document-level structured data extraction via custom JSON schema
- Single API call for both text and annotation extraction

## Installation

Download the latest binary for your platform from the [Releases](https://github.com/st3v/ocr/releases) page.

### macOS

```bash
# Apple Silicon
curl -L -o ocr https://github.com/st3v/ocr/releases/latest/download/ocr-darwin-arm64

# Intel
curl -L -o ocr https://github.com/st3v/ocr/releases/latest/download/ocr-darwin-amd64

# Make executable and move to PATH
chmod +x ocr && sudo mv ocr /usr/local/bin/
```

### Linux

```bash
# x86_64
curl -L -o ocr https://github.com/st3v/ocr/releases/latest/download/ocr-linux-amd64

# ARM64
curl -L -o ocr https://github.com/st3v/ocr/releases/latest/download/ocr-linux-arm64

# Make executable and move to PATH
chmod +x ocr && sudo mv ocr /usr/local/bin/
```

### Windows

Download [`ocr-windows-amd64.exe`](https://github.com/st3v/ocr/releases/latest/download/ocr-windows-amd64.exe) and add it to your PATH.

## Usage

```bash
ocr [options] <document>
```

### Options

| Flag | Description |
|------|-------------|
| `-o <dir>` | Output directory (default: same as input file) |
| `-m` | Extract image metadata (description, type, structured data) |
| `-a <file>` | Extract document data using JSON schema file |
| `-q` | Quiet mode (suppress progress output) |
| `-v` | Verbose mode (extra details to stderr) |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `MISTRAL_API_KEY` | Required. API key for Mistral AI. |

## Examples

```bash
# Set your Mistral API key (get one at https://console.mistral.ai/)
export MISTRAL_API_KEY=your-api-key-here

# Basic extraction
ocr document.pdf

# Extract to specific directory
ocr -o ./output document.pdf

# Extract with image metadata
ocr -m document.pdf

# Extract with custom document schema
ocr -a invoice_schema.json invoice.pdf

# Both image and document annotations
ocr -m -a schema.json document.pdf
```

## Output Structure

```
<output-dir>/
├── <basename>.md              # Extracted text in Markdown format
├── <basename>.annotation.json # Document annotation (with -a flag)
└── images/
    ├── page_0_img_0.png       # Extracted images
    ├── page_0_img_0.json      # Image metadata (with -m flag)
    └── ...
```

## Image Metadata Format

With the `-m` flag, each image gets a companion JSON file:

```json
{
  "description": "Bar chart showing quarterly revenue",
  "type": "chart",
  "structured_data": {
    "chart_type": "bar",
    "title": "Quarterly Revenue",
    "x_axis": {
      "label": "Quarter",
      "categories": ["Q1", "Q2", "Q3", "Q4"]
    },
    "y_axis": {
      "label": "Revenue",
      "unit": "USD",
      "range": [0, 1000000]
    },
    "data_series": [
      {"name": "2024", "values": [100, 150, 200, 175]}
    ],
    "legend": [
      {"label": "2024", "color": "blue"}
    ]
  }
}
```

### Image Types

- `graph` / `chart` - Data visualizations
- `table` - Tabular data
- `diagram` - Flowcharts, architecture diagrams, etc.
- `photo` - Photographs
- `illustration` - Drawings, icons
- `screenshot` - Screen captures
- `other` - Anything else

## Document Schema Format

For the `-a` flag, provide a JSON file with your schema:

```json
{
  "name": "invoice",
  "schema": {
    "type": "object",
    "properties": {
      "vendor": {"type": "string"},
      "invoice_number": {"type": "string"},
      "date": {"type": "string"},
      "line_items": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "description": {"type": "string"},
            "quantity": {"type": "number"},
            "unit_price": {"type": "number"}
          }
        }
      },
      "total": {"type": "number"}
    }
  }
}
```

## Supported Formats

- PDF
- Images: PNG, JPEG, GIF, WebP

## Building from Source

Requires [Go](https://golang.org/dl/) 1.22 or later.

```bash
git clone https://github.com/st3v/ocr.git
cd ocr
go build -o ocr .
```

## License

Apache 2.0
