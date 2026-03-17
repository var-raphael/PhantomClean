package output

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type OrganizedData struct {
	URL          string   `json:"url" xml:"url"`
	Title        string   `json:"title" xml:"title"`
	Content      string   `json:"content" xml:"content"`
	WordCount    int      `json:"word_count" xml:"word_count"`
	QualityScore float64  `json:"quality_score" xml:"quality_score"`
	Language     string   `json:"language" xml:"language"`
	CleanedAt    string   `json:"cleaned_at" xml:"cleaned_at"`
	AIUsed       string   `json:"ai_used" xml:"ai_used"`
	LayerUsed    string   `json:"layer_used,omitempty" xml:"layer_used,omitempty"`
	CrawledAt    string   `json:"crawled_at,omitempty" xml:"crawled_at,omitempty"`
	Links        []string `json:"links,omitempty" xml:"links,omitempty"`
	Images       []string `json:"images,omitempty" xml:"images,omitempty"`
	Emails       []string `json:"emails,omitempty" xml:"emails,omitempty"`
	Phones       []string `json:"phones,omitempty" xml:"phones,omitempty"`
	ExportFormat string   `json:"export_format,omitempty" xml:"-"`
}

// Write exports organized data in all requested formats
func Write(outputFolder, fileName string, formats []string, data *OrganizedData) error {
	if err := os.MkdirAll(outputFolder, 0755); err != nil {
		return fmt.Errorf("could not create output folder: %w", err)
	}

	for _, format := range formats {
		switch strings.ToLower(format) {
		case "json":
			if err := writeJSON(outputFolder, fileName, data); err != nil {
				return err
			}
		case "csv":
			if err := writeCSV(outputFolder, fileName, data); err != nil {
				return err
			}
		case "xml":
			if err := writeXML(outputFolder, fileName, data); err != nil {
				return err
			}
		case "txt":
			if err := writeTXT(outputFolder, fileName, data); err != nil {
				return err
			}
		case "html":
			if err := writeHTML(outputFolder, fileName, data); err != nil {
				return err
			}
		case "all_formats":
			all := []string{"json", "csv", "xml", "txt", "html"}
			for _, f := range all {
				if err := Write(outputFolder, fileName, []string{f}, data); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func writeJSON(folder, fileName string, data *OrganizedData) error {
	f, err := os.Create(filepath.Join(folder, fileName+".json"))
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false) // prevent & becoming \u0026
	return encoder.Encode(data)
}

func writeCSV(folder, fileName string, data *OrganizedData) error {
	f, err := os.Create(filepath.Join(folder, fileName+".csv"))
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// header
	w.Write([]string{"url", "title", "content", "word_count", "quality_score", "language", "cleaned_at", "ai_used"})
	// row
	w.Write([]string{
		data.URL,
		data.Title,
		data.Content,
		fmt.Sprintf("%d", data.WordCount),
		fmt.Sprintf("%.2f", data.QualityScore),
		data.Language,
		data.CleanedAt,
		data.AIUsed,
	})
	return nil
}

func writeXML(folder, fileName string, data *OrganizedData) error {
	type XMLWrapper struct {
		XMLName xml.Name      `xml:"document"`
		Data    *OrganizedData `xml:"data"`
	}
	out, err := xml.MarshalIndent(&XMLWrapper{Data: data}, "", "  ")
	if err != nil {
		return err
	}
	content := []byte(xml.Header + string(out))
	return os.WriteFile(filepath.Join(folder, fileName+".xml"), content, 0644)
}

func writeTXT(folder, fileName string, data *OrganizedData) error {
	var sb strings.Builder
	sb.WriteString("Title: " + data.Title + "\n")
	sb.WriteString("URL: " + data.URL + "\n")
	sb.WriteString("Cleaned: " + data.CleanedAt + "\n")
	sb.WriteString("AI Used: " + data.AIUsed + "\n")
	sb.WriteString("Words: " + fmt.Sprintf("%d", data.WordCount) + "\n")
	sb.WriteString("Quality: " + fmt.Sprintf("%.2f", data.QualityScore) + "\n")
	sb.WriteString("\n---\n\n")
	sb.WriteString(data.Content)
	return os.WriteFile(filepath.Join(folder, fileName+".txt"), []byte(sb.String()), 0644)
}

func writeHTML(folder, fileName string, data *OrganizedData) error {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>%s</title>
</head>
<body>
  <h1>%s</h1>
  <p><a href="%s">%s</a></p>
  <pre>%s</pre>
  <footer>
    <small>Cleaned: %s | AI: %s | Words: %d | Quality: %.2f</small>
  </footer>
</body>
</html>`,
		data.Title, data.Title, data.URL, data.URL,
		data.Content, data.CleanedAt, data.AIUsed,
		data.WordCount, data.QualityScore,
	)
	return os.WriteFile(filepath.Join(folder, fileName+".html"), []byte(html), 0644)
}

// Zip creates a zip archive of the organized output folder
func Zip(outputFolder, zipName string, fileCount int) (string, error) {
	// Replace template vars in zip name
	zipName = strings.ReplaceAll(zipName, "{date}", time.Now().Format("2006-01-02"))
	zipName = strings.ReplaceAll(zipName, "{file_count}", fmt.Sprintf("%d", fileCount))
	if !strings.HasSuffix(zipName, ".zip") {
		zipName += ".zip"
	}

	zipPath := zipName
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("could not create zip: %w", err)
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	err = filepath.Walk(outputFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(outputFolder, path)
		if err != nil {
			return err
		}

		f, err := w.Create(relPath)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		_, err = f.Write(data)
		return err
	})

	if err != nil {
		return "", fmt.Errorf("zip failed: %w", err)
	}

	return zipPath, nil
}
