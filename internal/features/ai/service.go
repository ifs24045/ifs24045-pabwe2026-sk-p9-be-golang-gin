package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"app/internal/features/post"
)

var (
	ErrAIUnavailable    = errors.New("Layanan AI gagal merespons")
	ErrInvalidAIResponse = errors.New("Respons AI tidak dapat dibaca")
)

// Service menghasilkan draf artikel dengan AI yang didukung Search Engine.
type Service struct {
	search SearchClient
	chat   ChatClient
}

// NewService membuat Service AI.
func NewService(search SearchClient, chat ChatClient) *Service {
	return &Service{search: search, chat: chat}
}

type generated struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Excerpt  string `json:"excerpt"`
	Content  string `json:"content"`
}

func systemPrompt() string {
	return "Anda adalah penulis blog profesional berbahasa Indonesia. " +
		"Balas HANYA dengan satu objek JSON valid, tanpa teks lain dan tanpa pagar kode markdown, dengan kunci: " +
		"\"title\" (judul menarik), \"category\" (salah satu dari: " + strings.Join(post.Categories, ", ") + "), " +
		"\"excerpt\" (ringkasan 1-2 kalimat), dan \"content\" (isi artikel lengkap dalam format Markdown " +
		"dengan sub-judul, minimal 400 kata). " +
		"Jangan menambahkan bagian sumber rujukan; bagian itu ditambahkan otomatis."
}

func userPrompt(topic, category string, sources []SearchSource) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Topik artikel: %s\n", topic)

	if strings.TrimSpace(category) != "" {
		fmt.Fprintf(&b, "Kategori pilihan penulis: %s\n", strings.TrimSpace(category))
	}

	if len(sources) > 0 {
		b.WriteString("\nGunakan hasil penelusuran web terkini berikut sebagai bahan rujukan faktual, " +
			"dan sebutkan sitasinya dengan format [1], [2], dst.:\n")
		for i, s := range sources {
			fmt.Fprintf(&b, "[%d] %s - %s\n%s\n", i+1, s.Title, s.URL, s.Snippet)
		}
	}
	return b.String()
}

// parseGenerated mengambil objek JSON dari jawaban model (pagar ```json diabaikan).
func parseGenerated(raw string) (*generated, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end <= start {
		return nil, ErrInvalidAIResponse
	}

	var g generated
	if err := json.Unmarshal([]byte(raw[start:end+1]), &g); err != nil {
		return nil, ErrInvalidAIResponse
	}
	if strings.TrimSpace(g.Title) == "" || strings.TrimSpace(g.Content) == "" {
		return nil, ErrInvalidAIResponse
	}
	return &g, nil
}

// normalizeCategory mencocokkan kategori dengan daftar resmi; selain itu menjadi "Umum".
func normalizeCategory(category string) string {
	category = strings.TrimSpace(category)
	for _, known := range post.Categories {
		if strings.EqualFold(known, category) {
			return known
		}
	}
	return "Umum"
}

// referencesSection membuat bagian "Sumber Rujukan" dalam Markdown.
func referencesSection(sources []SearchSource) string {
	if len(sources) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n## Sumber Rujukan\n\n")
	for i, s := range sources {
		if s.URL == "" {
			fmt.Fprintf(&b, "%d. %s\n", i+1, s.Title)
			continue
		}
		label := s.Title
		if label == "" {
			label = s.URL
		}
		fmt.Fprintf(&b, "%d. [%s](%s)\n", i+1, label, s.URL)
	}
	return strings.TrimRight(b.String(), "\n")
}

// GeneratePost menghasilkan draf artikel lengkap beserta sumber rujukan.
func (s *Service) GeneratePost(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	topic := strings.TrimSpace(req.Topic)

	sources := []SearchSource{}
	if req.IncludeWebSearch {
		// Jika penelusuran gagal, artikel tetap dibuat tanpa sumber rujukan.
		if found, err := s.search.Search(ctx, topic); err == nil {
			sources = found
		}
	}

	reply, err := s.chat.Complete(ctx, systemPrompt(), userPrompt(topic, req.Category, sources))
	if err != nil {
		return nil, ErrAIUnavailable
	}

	g, err := parseGenerated(reply)
	if err != nil {
		return nil, err
	}

	return &GenerateResponse{
		Title:         strings.TrimSpace(g.Title),
		Category:      normalizeCategory(g.Category),
		Excerpt:       strings.TrimSpace(g.Excerpt),
		Content:       strings.TrimSpace(g.Content) + referencesSection(sources),
		Sources:       sources,
		WebSearchUsed: len(sources) > 0,
	}, nil
}