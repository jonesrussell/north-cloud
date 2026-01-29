// Command gentemplate generates the Excel import template for sources.
// Usage: go run cmd/gentemplate/main.go
package main

import (
	"log"
	"os"

	"github.com/xuri/excelize/v2"
)

func main() {
	f := excelize.NewFile()

	// Rename Sheet1 to Sources
	if err := f.SetSheetName("Sheet1", "Sources"); err != nil {
		log.Fatal(err)
	}

	// Add headers
	headers := []string{"name", "url", "enabled", "rate_limit", "max_depth", "time", "selectors"}
	for i, h := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			log.Fatal(err)
		}
		if err := f.SetCellValue("Sources", cell, h); err != nil {
			log.Fatal(err)
		}
	}

	// Add example row 1
	row1 := []string{
		"example-news",
		"https://example.com/news",
		"true",
		"1s",
		"3",
		`["morning", "evening"]`,
		`{"article":{"title":"h1.headline","body":".article-content"}}`,
	}
	for i, v := range row1 {
		cell, err := excelize.CoordinatesToCellName(i+1, 2)
		if err != nil {
			log.Fatal(err)
		}
		if err := f.SetCellValue("Sources", cell, v); err != nil {
			log.Fatal(err)
		}
	}

	// Add example row 2
	row2 := []string{"local-blog", "https://blog.local", "false", "500ms", "2", "", ""}
	for i, v := range row2 {
		cell, err := excelize.CoordinatesToCellName(i+1, 3)
		if err != nil {
			log.Fatal(err)
		}
		if err := f.SetCellValue("Sources", cell, v); err != nil {
			log.Fatal(err)
		}
	}

	// Create Instructions sheet
	if _, err := f.NewSheet("Instructions"); err != nil {
		log.Fatal(err)
	}
	instructions := []string{
		"Column Descriptions:",
		"",
		"name - Required. Unique identifier for the source",
		"url - Required. Base URL to crawl (must start with http:// or https://)",
		"enabled - Optional. true/false/1/0/yes/no (default: false)",
		"rate_limit - Optional. Delay between requests (e.g., '1s', '500ms')",
		"max_depth - Optional. Maximum crawl depth (default: 0)",
		`time - Optional. JSON array of times (e.g., '["morning", "evening"]')`,
		"selectors - Optional. JSON object with CSS selectors for article/list/page extraction",
	}
	for i, line := range instructions {
		cell, err := excelize.CoordinatesToCellName(1, i+1)
		if err != nil {
			log.Fatal(err)
		}
		if err := f.SetCellValue("Instructions", cell, line); err != nil {
			log.Fatal(err)
		}
	}

	// Ensure examples directory exists
	if err := os.MkdirAll("examples", 0755); err != nil {
		log.Fatal(err)
	}

	// Save the file
	if err := f.SaveAs("examples/source-import-template.xlsx"); err != nil {
		log.Fatal(err)
	}
	log.Println("Created examples/source-import-template.xlsx")
}
