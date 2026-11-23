package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

var errBoom = errors.New("boom")

func init() {
	gin.SetMode(gin.TestMode)
}

func jsonServer(t *testing.T, status int, body string, check func(r *http.Request)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if check != nil {
			check(r)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func closedServerURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	return url
}

func parse(t *testing.T, body string) []SearchSource {
	t.Helper()
	var raw interface{}
	assert.NoError(t, json.Unmarshal([]byte(body), &raw))
	return parseSearchResults(raw)
}

// ---------- parsing hasil search ----------

func TestParseSearchResults(t *testing.T) {
	top := parse(t, `[{"name":"A","link":"https://a.com","description":"desc a"}]`)
	assert.Equal(t, []SearchSource{{Title: "A", URL: "https://a.com", Snippet: "desc a"}}, top)

	nested := parse(t, `{"data":{"results":[{"title":"B","href":"https://b.com","body":"isi b"}]}}`)
	assert.Equal(t, []SearchSource{{Title: "B", URL: "https://b.com", Snippet: "isi b"}}, nested)

	skipped := parse(t, `{"results":["teks",{"snippet":"tanpa judul"},{"title":"Valid"}]}`)
	assert.Equal(t, []SearchSource{{Title: "Valid"}}, skipped)

	many := parse(t, `{"items":[{"title":"1"},{"title":"2"},{"title":"3"},{"title":"4"},{"title":"5"},{"title":"6"},{"title":"7"}]}`)
	assert.Len(t, many, 5)

	assert.Empty(t, parse(t, `{"foo":1}`))
	assert.Empty(t, parse(t, `"bukan objek"`))
	assert.Empty(t, parse(t, `{"data":{"results":[]}}`))
}

// ---------- search client ----------

func TestSearchClientSuccess(t *testing.T) {
	var gotQuery, gotAuth string
	srv := jsonServer(t, 200, `{"results":[{"title":"Go","url":"https://go.dev","snippet":"Bahasa Go"}]}`, func(r *http.Request) {
		gotQuery = r.URL.Query().Get("q")
		gotAuth = r.Header.Get("Authorization")
	})

	sources, err := NewSearchClient(srv.URL+"/search", "kunci").Search(context.Background(), "golang terbaru")

	assert.NoError(t, err)
	assert.Equal(t, "golang terbaru", gotQuery)
	assert.Equal(t, "Bearer kunci", gotAuth)
	assert.Equal(t, []SearchSource{{Title: "Go", URL: "https://go.dev", Snippet: "Bahasa Go"}}, sources)
}

func TestSearchClientWithoutKey(t *testing.T) {
	gotAuth := "belum-diisi"
	srv := jsonServer(t, 200, `[]`, func(r *http.Request) { gotAuth = r.Header.Get("Authorization") })

	sources, err := NewSearchClient(srv.URL, "").Search(context.Background(), "x")

	assert.NoError(t, err)
	assert.Empty(t, gotAuth)
	assert.Empty(t, sources)
}

func TestSearchClientErrors(t *testing.T) {
	ctx := context.Background()

	_, err := NewSearchClient("://bad", "").Search(ctx, "x")
	assert.Error(t, err)

	_, err = NewSearchClient(closedServerURL(t), "").Search(ctx, "x")
	assert.Error(t, err)

	_, err = NewSearchClient(jsonServer(t, 500, `{}`, nil).URL, "").Search(ctx, "x")
	assert.ErrorContains(t, err, "500")

	_, err = NewSearchClient(jsonServer(t, 200, `bukan json`, nil).URL, "").Search(ctx, "x")
	assert.Error(t, err)
}

// ---------- chat client ----------

func TestChatClientSuccess(t *testing.T) {
	var gotPath, gotAuth, gotContentType string
	var gotBody chatRequest
	srv := jsonServer(t, 200, `{"choices":[{"message":{"role":"assistant","content":"halo dunia"}}]}`, func(r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
	})

	out, err := NewChatClient(srv.URL+"/", "sk-test", "model-x").Complete(context.Background(), "sistem", "pengguna")

	assert.NoError(t, err)
	assert.Equal(t, "halo dunia", out)
	assert.Equal(t, "/chat/completions", gotPath)
	assert.Equal(t, "Bearer sk-test", gotAuth)
	assert.Equal(t, "application/json", gotContentType)
	assert.Equal(t, "model-x", gotBody.Model)
	assert.Equal(t, []chatMessage{{Role: "system", Content: "sistem"}, {Role: "user", Content: "pengguna"}}, gotBody.Messages)
}

func TestChatClientWithoutKey(t *testing.T) {
	gotAuth := "belum-diisi"
	srv := jsonServer(t, 200, `{"choices":[{"message":{"content":"ok"}}]}`, func(r *http.Request) { gotAuth = r.Header.Get("Authorization") })

	out, err := NewChatClient(srv.URL, "", "m").Complete(context.Background(), "s", "u")

	assert.NoError(t, err)
	assert.Equal(t, "ok", out)
	assert.Empty(t, gotAuth)
}

func TestChatClientErrors(t *testing.T) {
	ctx := context.Background()

	_, err := NewChatClient(":bad", "", "m").Complete(ctx, "s", "u")
	assert.Error(t, err)

	_, err = NewChatClient(closedServerURL(t), "", "m").Complete(ctx, "s", "u")
	assert.Error(t, err)

	_, err = NewChatClient(jsonServer(t, 401, `{}`, nil).URL, "", "m").Complete(ctx, "s", "u")
	assert.ErrorContains(t, err, "401")

	_, err = NewChatClient(jsonServer(t, 200, `bukan json`, nil).URL, "", "m").Complete(ctx, "s", "u")
	assert.Error(t, err)

	_, err = NewChatClient(jsonServer(t, 200, `{"choices":[]}`, nil).URL, "", "m").Complete(ctx, "s", "u")
	assert.Error(t, err)
}

// ---------- service ----------

type stubSearch struct {
	sources []SearchSource
	err     error
	called  bool
	query   string
}

func (s *stubSearch) Search(_ context.Context, q string) ([]SearchSource, error) {
	s.called = true
	s.query = q
	return s.sources, s.err
}

type stubChat struct {
	reply  string
	err    error
	system string
	user   string
}

func (s *stubChat) Complete(_ context.Context, system, user string) (string, error) {
	s.system = system
	s.user = user
	return s.reply, s.err
}

const validReply = `{"title":" Judul AI ","category":"teknologi","excerpt":" Ringkas ","content":" Isi **markdown** "}`

func TestGeneratePostWithSearch(t *testing.T) {
	search := &stubSearch{sources: []SearchSource{{Title: "Go", URL: "https://go.dev", Snippet: "Bahasa Go"}}}
	chat := &stubChat{reply: "```json\n" + validReply + "\n```"}
	svc := NewService(search, chat)

	res, err := svc.GeneratePost(context.Background(), GenerateRequest{Topic: "  Belajar Go  ", Category: "teknologi", IncludeWebSearch: true})

	assert.NoError(t, err)
	assert.True(t, search.called)
	assert.Equal(t, "Belajar Go", search.query)
	assert.Equal(t, "Judul AI", res.Title)
	assert.Equal(t, "Teknologi", res.Category)
	assert.Equal(t, "Ringkas", res.Excerpt)
	assert.True(t, res.WebSearchUsed)
	assert.Len(t, res.Sources, 1)
	assert.True(t, strings.HasPrefix(res.Content, "Isi **markdown**"))
	assert.Contains(t, res.Content, "## Sumber Rujukan")
	assert.Contains(t, res.Content, "1. [Go](https://go.dev)")
	assert.Contains(t, chat.user, "Topik artikel: Belajar Go")
	assert.Contains(t, chat.user, "Kategori pilihan penulis: teknologi")
	assert.Contains(t, chat.user, "[1] Go - https://go.dev")
	assert.Contains(t, chat.system, "Teknologi, Pendidikan")
}

func TestGeneratePostWithoutSearch(t *testing.T) {
	search := &stubSearch{}
	chat := &stubChat{reply: `{"title":"Judul","category":"Olahraga","excerpt":"","content":"Isi"}`}
	svc := NewService(search, chat)

	res, err := svc.GeneratePost(context.Background(), GenerateRequest{Topic: "Topik bebas"})

	assert.NoError(t, err)
	assert.False(t, search.called)
	assert.False(t, res.WebSearchUsed)
	assert.NotNil(t, res.Sources)
	assert.Empty(t, res.Sources)
	assert.Equal(t, "Umum", res.Category)
	assert.Equal(t, "Isi", res.Content)
	assert.NotContains(t, chat.user, "penelusuran")
	assert.NotContains(t, chat.user, "Kategori pilihan")
}

func TestGeneratePostSearchFailureStillWorks(t *testing.T) {
	search := &stubSearch{err: errBoom}
	svc := NewService(search, &stubChat{reply: validReply})

	res, err := svc.GeneratePost(context.Background(), GenerateRequest{Topic: "Topik", IncludeWebSearch: true})

	assert.NoError(t, err)
	assert.True(t, search.called)
	assert.False(t, res.WebSearchUsed)
	assert.Empty(t, res.Sources)
	assert.NotContains(t, res.Content, "Sumber Rujukan")
}

func TestGeneratePostChatError(t *testing.T) {
	svc := NewService(&stubSearch{}, &stubChat{err: errBoom})

	_, err := svc.GeneratePost(context.Background(), GenerateRequest{Topic: "Topik"})

	assert.ErrorIs(t, err, ErrAIUnavailable)
}

func TestGeneratePostInvalidReply(t *testing.T) {
	replies := []string{
		"tidak ada json sama sekali",
		"} {",
		"{bukan json}",
		`{"title":"","content":"isi"}`,
		`{"title":"judul","content":"   "}`,
	}

	for _, reply := range replies {
		svc := NewService(&stubSearch{}, &stubChat{reply: reply})

		_, err := svc.GeneratePost(context.Background(), GenerateRequest{Topic: "Topik"})

		assert.ErrorIs(t, err, ErrInvalidAIResponse, reply)
	}
}

func TestReferencesSection(t *testing.T) {
	assert.Equal(t, "", referencesSection(nil))

	got := referencesSection([]SearchSource{
		{Title: "Go", URL: "https://go.dev"},
		{URL: "https://only-url.com"},
		{Title: "Tanpa Tautan"},
	})

	assert.Equal(t,
		"\n\n## Sumber Rujukan\n\n1. [Go](https://go.dev)\n2. [https://only-url.com](https://only-url.com)\n3. Tanpa Tautan",
		got)
}

func TestNormalizeCategory(t *testing.T) {
	assert.Equal(t, "Gaya Hidup", normalizeCategory(" gaya hidup "))
	assert.Equal(t, "Umum", normalizeCategory("Olahraga"))
	assert.Equal(t, "Umum", normalizeCategory(""))
}

// ---------- handler ----------

func newRouter(search SearchClient, chat ChatClient) *gin.Engine {
	r := gin.New()
	r.POST("/ai-generate", NewHandler(NewService(search, chat)).Generate)
	return r
}

func doPost(r http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/ai-generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

func TestHandlerGenerate(t *testing.T) {
	search := &stubSearch{sources: []SearchSource{{Title: "Go", URL: "https://go.dev"}}}
	r := newRouter(search, &stubChat{reply: validReply})

	w := doPost(r, `bukan json`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = doPost(r, `{"topic":"ab"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = doPost(r, `{"topic":"Belajar Go","include_web_search":true}`)
	assert.Equal(t, http.StatusOK, w.Code)
	body := decode(t, w)
	assert.Equal(t, "success", body["status"])
	data := body["data"].(map[string]interface{})
	assert.Equal(t, "Judul AI", data["title"])
	assert.Equal(t, true, data["web_search_used"])
	assert.Len(t, data["sources"], 1)
}

func TestHandlerGenerateAIErrors(t *testing.T) {
	r := newRouter(&stubSearch{}, &stubChat{err: errBoom})
	w := doPost(r, `{"topic":"Belajar Go"}`)
	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Equal(t, "error", decode(t, w)["status"])

	r = newRouter(&stubSearch{}, &stubChat{reply: "bukan json"})
	w = doPost(r, `{"topic":"Belajar Go"}`)
	assert.Equal(t, http.StatusBadGateway, w.Code)
}
func TestRespondErrorDefault(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	respondError(c, errBoom)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "error", decode(t, w)["status"])
}