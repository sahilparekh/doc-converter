package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jhillyerd/enmime"
)

const tempDir = "./tmp" // Directory for temporary files

func main() {
	// Ensure the temporary directory exists
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		fmt.Println("Failed to create temp directory:", err)
		return
	}

	// Start the file cleanup goroutine
	go cleanupOldFiles(tempDir, 1*time.Hour)

	http.HandleFunc("/", apiKeyMiddleware(handleHealthCheck))
	http.HandleFunc("/convert", apiKeyMiddleware(handleConvert))
	http.HandleFunc("/doc-to-txt", apiKeyMiddleware(handleDocToTxt))
	http.HandleFunc("/msg-to-txt", apiKeyMiddleware(handleMsgToTxt))

	fmt.Println("Starting server on :80")
	if err := http.ListenAndServe(":80", nil); err != nil {
		fmt.Println("Failed to start server:", err)
	}
}

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

// apiKeyMiddleware checks for the API key in the request header
func apiKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := os.Getenv("API_KEY")
		key := r.Header.Get("X-API-Key")
		if key != apiKey {
			http.Error(w, "Unauthorized: invalid API key", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func handleConvert(w http.ResponseWriter, r *http.Request) {
	// Ensure the request method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the uploaded file
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to read uploaded file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Save the Excel file to a temporary location
	baseName := time.Now().Format("20060102150405") // Timestamp format
	inputFilePath := filepath.Join(tempDir, baseName+".xlsx")
	outputFilePath := filepath.Join(tempDir, baseName+".pdf")

	inputFile, err := os.Create(inputFilePath)
	if err != nil {
		http.Error(w, "Failed to create temporary file", http.StatusInternalServerError)
		return
	}
	defer inputFile.Close()
	defer os.Remove(inputFilePath)

	_, err = io.Copy(inputFile, file)
	if err != nil {
		http.Error(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}

	// Convert the Excel file to PDF using LibreOffice
	cmd := exec.Command("soffice", "--headless", "--convert-to", "pdf:calc_pdf_Export", inputFilePath, "--outdir", tempDir)
	if err := cmd.Run(); err != nil {
		fmt.Println(err)
		http.Error(w, "Failed to convert file to PDF", http.StatusInternalServerError)
		return
	}
	defer os.Remove(outputFilePath)

	// Read the converted PDF file
	pdfFile, err := os.Open(outputFilePath)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Failed to read converted PDF", http.StatusInternalServerError)
		return
	}
	defer pdfFile.Close()

	// Write the PDF file as response
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="output.pdf"`)

	if _, err := io.Copy(w, pdfFile); err != nil {
		http.Error(w, "Failed to write PDF to response", http.StatusInternalServerError)
		return
	}
}

func handleDocToTxt(w http.ResponseWriter, r *http.Request) {
	// Ensure the request method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the uploaded file
	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to read uploaded file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Save the .doc file to a temporary location
	baseName := time.Now().Format("20060102150405") + "_" + handler.Filename
	inputFilePath := filepath.Join(tempDir, baseName)

	inputFile, err := os.Create(inputFilePath)
	if err != nil {
		http.Error(w, "Failed to create temporary file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(inputFilePath) // Remove file after processing

	_, err = io.Copy(inputFile, file)
	if err != nil {
		http.Error(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}
	inputFile.Close() // ensure file is flushed and closed before conversion

	// Process the .doc file to get text content
	content, err := processDocFile(inputFilePath)
	if err != nil {
		http.Error(w, "Failed to process .doc file", http.StatusInternalServerError)
		return
	}

	// Write the text content as response
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(content))
}

// processDocFile processes a DOC file using soffice and returns its text content.
func processDocFile(filePath string) (string, error) {
	fmt.Printf("Attempting to process DOC file with soffice: %s\n", filePath)

	// Determine output file name by replacing .doc extension with .txt
	base := filepath.Base(filePath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	convertedFileName := stem + ".txt"
	convertedFilePath := filepath.Join(tempDir, convertedFileName)

	// Convert the .doc file to .txt using soffice
	cmd := exec.Command("soffice", "--headless", "--convert-to", "txt:Text", filePath, "--outdir", tempDir)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error executing soffice for %s: %v\n", filePath, err)
		return "", fmt.Errorf("soffice execution error: %w", err)
	}

	// Read the converted .txt file
	content, err := os.ReadFile(convertedFilePath)
	if err != nil {
		fmt.Printf("Failed to read converted .txt file %s: %v\n", convertedFilePath, err)
		return "", fmt.Errorf("failed to read converted file: %w", err)
	}

	// Clean up converted file
	defer os.Remove(convertedFilePath)

	return strings.TrimSpace(string(content)), nil
}

func handleMsgToTxt(w http.ResponseWriter, r *http.Request) {
	// Ensure the request method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the uploaded file
	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to read uploaded file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Save the .msg file to a temporary location
	baseName := time.Now().Format("20060102150405") + "_" + handler.Filename
	inputFilePath := filepath.Join(tempDir, baseName)

	inputFile, err := os.Create(inputFilePath)
	if err != nil {
		http.Error(w, "Failed to create temporary file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(inputFilePath) // Remove file after processing

	_, err = io.Copy(inputFile, file)
	if err != nil {
		http.Error(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}
	inputFile.Close() // ensure file is flushed and closed before conversion

	// Convert .msg to .eml using msgconvert
	// Determine eml output path
	stem := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	emlPath := filepath.Join(tempDir, stem+".eml")
	cmd := exec.Command("msgconvert", "--outfile", emlPath, inputFilePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		http.Error(w, fmt.Sprintf("msgconvert failed: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.Remove(emlPath)

	// Read raw .eml data
	raw, err := os.ReadFile(emlPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read converted EML: %v", err), http.StatusInternalServerError)
		return
	}

	// Parse headers
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse headers: %v", err), http.StatusInternalServerError)
		return
	}

	// Parse full MIME envelope
	env, err := enmime.ReadEnvelope(bytes.NewReader(raw))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse MIME body: %v", err), http.StatusInternalServerError)
		return
	}

	// Write response headers and body
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	// Write metadata
	hdr := msg.Header
	fmt.Fprintf(w, "From:   %s\n", hdr.Get("From"))
	fmt.Fprintf(w, "To:     %s\n", hdr.Get("To"))
	fmt.Fprintf(w, "Cc:     %s\n", hdr.Get("Cc"))
	fmt.Fprintf(w, "Bcc:    %s\n", hdr.Get("Bcc"))
	fmt.Fprintf(w, "Subject:%s\n", hdr.Get("Subject"))
	fmt.Fprintf(w, "Date:   %s\n\n", hdr.Get("Date"))

	// Extract body: prefer plain text, then HTML, then RTF
	var body string
	if env.Text != "" {
		body = env.Text
	} else if env.HTML != "" {
		body = env.HTML
	} else {
		// Find any RTF parts
		rtfParts := env.Root.DepthMatchAll(func(p *enmime.Part) bool {
			ct := strings.ToLower(p.ContentType)
			return strings.HasPrefix(ct, "application/rtf") || strings.HasPrefix(ct, "text/rtf")
		})
		if len(rtfParts) > 0 {
			tmpRTF := filepath.Join(tempDir, "body.rtf")
			if err := os.WriteFile(tmpRTF, rtfParts[0].Content, 0644); err != nil {
				http.Error(w, fmt.Sprintf("Failed writing temp RTF: %v", err), http.StatusInternalServerError)
				return
			}
			// Convert RTF to text
			outBytes, err := exec.Command("unrtf", "--text", tmpRTF).CombinedOutput()
			if err != nil {
				http.Error(w, fmt.Sprintf("unrtf conversion failed: %v", err), http.StatusInternalServerError)
				return
			}
			body = string(outBytes)
			os.Remove(tmpRTF)
		}
	}
	if body == "" {
		body = "(no body content)\n"
	}

	// Remove specific lines from the output
	lines := strings.Split(body, "\n")
	filteredLines := []string{}
	for _, line := range lines {
		if !strings.Contains(line, "Translation from RTF performed by UnRTF") &&
			!strings.Contains(line, "font table contains 4 fonts tota") {
			filteredLines = append(filteredLines, line)
		}
	}
	body = strings.Join(filteredLines, "\n")

	fmt.Fprint(w, body)
}

func cleanupOldFiles(dir string, maxAge time.Duration) {
	for {
		time.Sleep(1 * time.Hour) // Check every minute

		files, err := os.ReadDir(dir)
		if err != nil {
			fmt.Println("Failed to read temp directory:", err)
			continue
		}

		for _, file := range files {
			filePath := filepath.Join(dir, file.Name())
			info, err := os.Stat(filePath)
			if err != nil {
				fmt.Println("Failed to get file info:", err)
				continue
			}

			// Check if the file is older than maxAge
			if time.Since(info.ModTime()) > maxAge {
				if err := os.Remove(filePath); err != nil {
					fmt.Println("Failed to delete file:", err)
				} else {
					fmt.Println("Deleted old file:", filePath)
				}
			}
		}
	}
}
