package app

import (
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func NewRouter(cfg Config, client *http.Client, logger *log.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.HandleMethodNotAllowed = true
	r.NoMethod(func(c *gin.Context) {
		c.Status(http.StatusMethodNotAllowed)
	})

	forwarder := NewForwarder(cfg.ForwardURL, cfg.ForwardToken, client)

	r.HEAD("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	r.POST("/", func(c *gin.Context) {
		raw, err := io.ReadAll(c.Request.Body)
		if err != nil {
			logger.Printf("failed reading body: %v", err)
			c.Status(http.StatusBadRequest)
			return
		}

		headerSig := c.GetHeader("X-Trello-Webhook")
		ok := VerifySignature(cfg.TrelloSecret, raw, cfg.CallbackURL, headerSig)
		logger.Printf("signature_valid=%t", ok)
		if !ok {
			c.Status(http.StatusForbidden)
			return
		}

		msg := BuildMessage(raw)
		status, respBody, err := forwarder.Forward(c.Request.Context(), msg, raw)
		if err != nil {
			logger.Printf("forward failed: %v", err)
			c.Status(http.StatusBadGateway)
			return
		}

		logger.Printf("forward_status=%d", status)
		c.Data(status, "application/json", respBody)
	})

	return r
}
