package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newContext() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) Body {
	t.Helper()
	var b Body
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	return b
}

func TestSuccessWithData(t *testing.T) {
	c, w := newContext()

	Success(c, http.StatusOK, "Berhasil", map[string]string{"nama": "delcom"})

	assert.Equal(t, http.StatusOK, w.Code)
	b := decode(t, w)
	assert.Equal(t, "success", b.Status)
	assert.Equal(t, "Berhasil", b.Message)
	assert.Equal(t, map[string]interface{}{"nama": "delcom"}, b.Data)
}

func TestSuccessWithoutData(t *testing.T) {
	c, w := newContext()

	Success(c, http.StatusCreated, "Dibuat", nil)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.NotContains(t, w.Body.String(), `"data"`)
	assert.Equal(t, "success", decode(t, w).Status)
}

func TestFail(t *testing.T) {
	c, w := newContext()

	Fail(c, http.StatusBadRequest, "Input tidak valid")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	b := decode(t, w)
	assert.Equal(t, "fail", b.Status)
	assert.Equal(t, "Input tidak valid", b.Message)
	assert.Nil(t, b.Data)
}

func TestError(t *testing.T) {
	c, w := newContext()

	Error(c, http.StatusInternalServerError, "Kesalahan server")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	b := decode(t, w)
	assert.Equal(t, "error", b.Status)
	assert.Equal(t, "Kesalahan server", b.Message)
}

func TestAbortFail(t *testing.T) {
	c, w := newContext()

	AbortFail(c, http.StatusUnauthorized, "Tidak terautentikasi")

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "fail", decode(t, w).Status)
}

func TestAbortError(t *testing.T) {
	c, w := newContext()

	AbortError(c, http.StatusInternalServerError, "Gangguan")

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "error", decode(t, w).Status)
}