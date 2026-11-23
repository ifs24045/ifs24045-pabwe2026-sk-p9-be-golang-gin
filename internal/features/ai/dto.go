package ai

// GenerateRequest adalah payload pembuatan draf artikel dengan AI.
type GenerateRequest struct {
	Topic            string `json:"topic" binding:"required,min=3,max=255"`
	Category         string `json:"category"`
	IncludeWebSearch bool   `json:"include_web_search"`
}

// SearchSource adalah satu sumber rujukan hasil penelusuran Search Engine.
type SearchSource struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// GenerateResponse adalah draf artikel hasil AI beserta sumber penelusuran.
type GenerateResponse struct {
	Title         string         `json:"title"`
	Category      string         `json:"category"`
	Excerpt       string         `json:"excerpt"`
	Content       string         `json:"content"`
	Sources       []SearchSource `json:"sources"`
	WebSearchUsed bool           `json:"web_search_used"`
}