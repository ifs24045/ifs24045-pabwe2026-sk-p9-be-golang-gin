package response

import "github.com/gin-gonic/gin"

// Body adalah format JSON standar seluruh respons API: {status, message, data?}.
type Body struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success mengirim respons 2xx dengan status "success".
func Success(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(code, Body{Status: "success", Message: message, Data: data})
}

// Fail mengirim respons 4xx (kesalahan klien) dengan status "fail".
func Fail(c *gin.Context, code int, message string) {
	c.JSON(code, Body{Status: "fail", Message: message})
}

// Error mengirim respons 5xx (kesalahan server) dengan status "error".
func Error(c *gin.Context, code int, message string) {
	c.JSON(code, Body{Status: "error", Message: message})
}

// AbortFail seperti Fail tetapi menghentikan rantai middleware/handler.
func AbortFail(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, Body{Status: "fail", Message: message})
}

// AbortError seperti Error tetapi menghentikan rantai middleware/handler.
func AbortError(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, Body{Status: "error", Message: message})
}