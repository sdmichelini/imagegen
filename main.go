package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	openRouterBaseURL = "https://openrouter.ai/api/v1"
	maxBrandFileSize  = 512 * 1024
)

var modelAliases = map[string][]string{
	"google": {"google/gemini-2.5-flash-image"},
	"openai": {"openai/gpt-5-image-mini"},
	"both":   {"google/gemini-2.5-flash-image", "openai/gpt-5-image-mini"},
}

var (
	imageSizePattern   = regexp.MustCompile(`(?i)^(1k|2k|4k)$`)
	aspectRatioPattern = regexp.MustCompile(`^(1:1|2:3|3:2|3:4|4:3|4:5|5:4|9:16|16:9|21:9)$`)
)

type chatCompletionsRequest struct {
	Model       string           `json:"model"`
	Messages    []chatMessage    `json:"messages"`
	Modalities  []string         `json:"modalities"`
	Stream      bool             `json:"stream"`
	ImageConfig *imageConfigBody `json:"image_config,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type imageConfigBody struct {
	AspectRatio string `json:"aspect_ratio,omitempty"`
	ImageSize   string `json:"image_size,omitempty"`
}

type chatCompletionsResponse struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Choices []struct {
		Message struct {
			Images []struct {
				ImageURL struct {
					URL string `json:"url"`
				} `json:"image_url"`
			} `json:"images"`
		} `json:"message"`
	} `json:"choices"`
}

func main() {
	var (
		prompt      = flag.String("prompt", "", "Short prompt describing the desired image (required)")
		brandDir    = flag.String("brand-dir", "", "Optional directory containing brand files")
		modelOpt    = flag.String("model", "both", "Model selector: google | openai | both")
		outDir      = flag.String("out", "output", "Output directory for generated images")
		imgSize     = flag.String("image-size", "1K", "Image size: 1K | 2K | 4K")
		aspectRatio = flag.String("aspect-ratio", "", "Optional aspect ratio: 1:1, 2:3, 3:2, 3:4, 4:3, 4:5, 5:4, 9:16, 16:9, 21:9")
		count       = flag.Int("n", 1, "Number of images per selected model")
	)
	flag.Parse()

	if strings.TrimSpace(*prompt) == "" {
		exitWithUsage("-prompt is required")
	}
	if *count < 1 {
		exitWithUsage("-n must be >= 1")
	}

	selectedImageSize := strings.TrimSpace(*imgSize)
	selectedImageSize = strings.ToUpper(selectedImageSize)
	if !imageSizePattern.MatchString(selectedImageSize) {
		exitWithUsage("invalid image size; use 1K, 2K, or 4K")
	}

	selectedAspectRatio := strings.TrimSpace(*aspectRatio)
	if selectedAspectRatio != "" && !aspectRatioPattern.MatchString(selectedAspectRatio) {
		exitWithUsage("invalid aspect ratio; use one of: 1:1, 2:3, 3:2, 3:4, 4:3, 4:5, 5:4, 9:16, 16:9, 21:9")
	}

	models, ok := modelAliases[strings.ToLower(strings.TrimSpace(*modelOpt))]
	if !ok {
		exitWithUsage("invalid -model; use google, openai, or both")
	}

	apiKey := strings.TrimSpace(loadAPIKey())
	if apiKey == "" {
		fatalf("OPEN_ROUTER_API_KEY is not set")
	}

	finalPrompt := strings.TrimSpace(*prompt)
	if strings.TrimSpace(*brandDir) != "" {
		brandContext, err := loadBrandContext(*brandDir)
		if err != nil {
			fatalf("failed to load brand files: %v", err)
		}
		finalPrompt = mergePromptWithBrandContext(finalPrompt, brandContext)
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatalf("failed to create output directory: %v", err)
	}

	client := &http.Client{Timeout: 2 * time.Minute}
	ctx := context.Background()

	for _, model := range models {
		for i := 1; i <= *count; i++ {
			fmt.Printf("Generating image with %s (%d/%d)\n", model, i, *count)
			imageBytes, ext, err := generateImage(ctx, client, apiKey, model, finalPrompt, selectedImageSize, selectedAspectRatio)
			if err != nil {
				fatalf("image generation failed for model %s: %v", model, err)
			}

			outPath := filepath.Join(*outDir, buildFilename(model, i, ext))
			if err := os.WriteFile(outPath, imageBytes, 0o644); err != nil {
				fatalf("failed to write %s: %v", outPath, err)
			}
			fmt.Printf("Saved: %s\n", outPath)
		}
	}
}

func generateImage(ctx context.Context, client *http.Client, apiKey, model, prompt, imageSize, aspectRatio string) ([]byte, string, error) {
	var cfg *imageConfigBody
	if strings.HasPrefix(model, "google/gemini") || aspectRatio != "" {
		cfg = &imageConfigBody{}
		if strings.HasPrefix(model, "google/gemini") {
			cfg.ImageSize = imageSize
		}
		if aspectRatio != "" {
			cfg.AspectRatio = aspectRatio
		}
	}

	reqBody := chatCompletionsRequest{
		Model:       model,
		Messages:    []chatMessage{{Role: "user", Content: prompt}},
		Modalities:  []string{"image", "text"},
		Stream:      false,
		ImageConfig: cfg,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterBaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read response: %w", err)
	}

	var parsed chatCompletionsResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, "", fmt.Errorf("parse response (%d): %s", resp.StatusCode, truncate(string(respBody), 500))
	}

	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, "", fmt.Errorf("api error (%d): %s", resp.StatusCode, parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return nil, "", fmt.Errorf("no image data returned (%d): %s", resp.StatusCode, truncate(string(respBody), 500))
	}
	images := parsed.Choices[0].Message.Images
	if len(images) == 0 {
		return nil, "", fmt.Errorf("no images in first choice (%d): %s", resp.StatusCode, truncate(string(respBody), 500))
	}
	imageURL := strings.TrimSpace(images[0].ImageURL.URL)
	if imageURL == "" {
		return nil, "", errors.New("image URL is empty")
	}

	if strings.HasPrefix(imageURL, "data:") {
		raw, ext, err := decodeDataURL(imageURL)
		if err != nil {
			return nil, "", err
		}
		return raw, ext, nil
	}

	img, ext, err := downloadImage(ctx, client, imageURL)
	if err != nil {
		return nil, "", fmt.Errorf("download image url: %w", err)
	}
	return img, ext, nil
}

func downloadImage(ctx context.Context, client *http.Client, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, "", fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	switch {
	case strings.Contains(ct, "image/jpeg"):
		return data, ".jpg", nil
	case strings.Contains(ct, "image/webp"):
		return data, ".webp", nil
	case strings.Contains(ct, "image/png"):
		return data, ".png", nil
	default:
		return data, ".img", nil
	}
}

func decodeDataURL(dataURL string) ([]byte, string, error) {
	const marker = ";base64,"
	if !strings.HasPrefix(dataURL, "data:") {
		return nil, "", errors.New("invalid data URL prefix")
	}
	idx := strings.Index(dataURL, marker)
	if idx < 0 {
		return nil, "", errors.New("data URL missing base64 marker")
	}

	meta := strings.TrimPrefix(dataURL[:idx], "data:")
	payload := dataURL[idx+len(marker):]
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, "", fmt.Errorf("decode image base64: %w", err)
	}

	ext := extensionFromMIME(meta)
	return raw, ext, nil
}

func extensionFromMIME(mt string) string {
	mt = strings.TrimSpace(strings.ToLower(mt))
	if mt == "" {
		return ".png"
	}
	exts, err := mime.ExtensionsByType(mt)
	if err != nil || len(exts) == 0 {
		return ".png"
	}
	return exts[0]
}

func loadBrandContext(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var chunks []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filePath := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return "", err
		}
		if info.Size() > maxBrandFileSize {
			continue
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			continue
		}

		text := strings.TrimSpace(string(data))
		if text == "" {
			continue
		}

		chunks = append(chunks, fmt.Sprintf("File: %s\n%s", entry.Name(), text))
	}

	if len(chunks) == 0 {
		return "", errors.New("no readable text files found in brand directory")
	}

	slices.Sort(chunks)
	return strings.Join(chunks, "\n\n"), nil
}

func mergePromptWithBrandContext(prompt, brandContext string) string {
	return fmt.Sprintf(
		"You are generating a branded image.\n"+
			"Follow the brand information below strictly.\n\n"+
			"Brand information:\n%s\n\n"+
			"Image request:\n%s",
		brandContext,
		prompt,
	)
}

func buildFilename(model string, index int, ext string) string {
	safeModel := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(model)
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	return fmt.Sprintf("%s_%s_%02d%s", safeModel, timestamp, index, ext)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func exitWithUsage(msg string) {
	fmt.Fprintln(os.Stderr, "Error:", msg)
	flag.Usage()
	os.Exit(2)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

func loadAPIKey() string {
	if v := strings.TrimSpace(os.Getenv("OPEN_ROUTER_API_KEY")); v != "" {
		return v
	}

	data, err := os.ReadFile(".env")
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "OPEN_ROUTER_API_KEY=") {
			continue
		}

		val := strings.TrimSpace(strings.TrimPrefix(line, "OPEN_ROUTER_API_KEY="))
		val = strings.Trim(val, `"'`)
		return val
	}

	return ""
}
