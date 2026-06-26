package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"google.golang.org/genai"
)

// DTO untuk struktur payload
type AITaskPayload struct {
	TaskID string `json:"task_id"`
	Prompt string `json:"prompt"`
	ChatID int64  `json:"chat_id"`
	FileID string `json:"file_id,omitempty"`
}

var redisClient *redis.Client

func main() {
	// 1. Inisialisasi Redis Client untuk Cache Hasil
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}
	redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// 2. Setup Asynq Worker
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{Concurrency: 5},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc("ai:generate", HandleAITask)

	log.Println("Worker AI siap melayani antrean...")
	if err := srv.Run(mux); err != nil {
		log.Fatalf("Gagal menjalankan worker: %v", err)
	}
}

func HandleAITask(ctx context.Context, t *asynq.Task) error {
	var p AITaskPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}

	log.Printf("Memproses Task: %s", p.TaskID)

	// Panggil Gemini API
	result, err := callGeminiAPI(p.Prompt, p.FileID)
	if err != nil {
		return fmt.Errorf("gemini api error: %w", err)
	}

	// Simpan ke Redis sebagai Cache (TTL 5 menit)
	cacheKey := fmt.Sprintf("result:%s", p.TaskID)
	err = redisClient.Set(ctx, cacheKey, result, 5*time.Minute).Err()
	if err != nil {
		log.Printf("Gagal cache ke Redis: %v", err)
	}

	log.Printf("Task %s selesai, hasil disimpan di %s", p.TaskID, cacheKey)

	// Kirim balik ke Telegram jika ChatID valid
	if p.ChatID != 0 {
		err := sendTelegramMessage(p.ChatID, result)
		if err != nil {
			log.Printf("Gagal mengirim balasan ke Telegram: %v", err)
		} else {
			log.Printf("Berhasil mengirim balasan ke Telegram ChatID %d", p.ChatID)
		}
	}

	return nil
}

// Fungsi untuk mengirim pesan via Telegram API
func sendTelegramMessage(chatID int64, text string) error {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN belum diatur")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	body, _ := json.Marshal(payload)

	resp, err := http.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API error status %d", resp.StatusCode)
	}

	return nil
}

func callGeminiAPI(prompt string, fileID string) (string, error) {
	ctx := context.Background()
	// Google GenAI Go SDK picks up GEMINI_API_KEY from environment by default
	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-1.5-flash"
	}

	// Persiapkan part untuk dikirim
	var parts []*genai.Part
	parts = append(parts, genai.NewPartFromText(prompt))

	// Jika ada gambar, download dari Telegram lalu tambahkan ke part
	if fileID != "" {
		imgBytes, err := downloadTelegramPhoto(fileID)
		if err != nil {
			log.Printf("Gagal mendownload gambar dari telegram: %v", err)
			parts = append(parts, genai.NewPartFromText("\n[Gambar gagal diproses]"))
		} else {
			// Telegram photo selalu JPEG
			parts = append(parts, genai.NewPartFromBytes(imgBytes, "image/jpeg"))
		}
	}

	contents := []*genai.Content{
		{
			Parts: parts,
			Role:  "user",
		},
	}

	resp, err := client.Models.GenerateContent(ctx, model, contents, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	return resp.Text(), nil
}

func downloadTelegramPhoto(fileID string) ([]byte, error) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN tidak ditemukan")
	}

	// 1. Dapatkan file_path dari file_id
	urlPath := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", token, fileID)
	respPath, err := http.Get(urlPath)
	if err != nil {
		return nil, err
	}
	defer respPath.Body.Close()

	var getFileResp struct {
		Ok     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}

	if err := json.NewDecoder(respPath.Body).Decode(&getFileResp); err != nil {
		return nil, err
	}
	if !getFileResp.Ok || getFileResp.Result.FilePath == "" {
		return nil, fmt.Errorf("gagal mendapatkan file_path")
	}

	// 2. Download file-nya
	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", token, getFileResp.Result.FilePath)
	respImg, err := http.Get(downloadURL)
	if err != nil {
		return nil, err
	}
	defer respImg.Body.Close()

	if respImg.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status saat download gambar: %s", respImg.Status)
	}

	return io.ReadAll(respImg.Body)
}
