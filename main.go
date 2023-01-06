package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	lib "github.com/mstephen19/server-side-events/lib"
)

const MaxBytes = 1024

type MessageData struct {
	Message string `json:"message"`
	Time    int64  `json:"time"`
}

func main() {
	router := gin.Default()
	clientServer := lib.CreateServer()

	router.Use(cors.Default())

	// Listen for messages
	router.GET("/events", clientServer.ClientMiddleware(), func(ctx *gin.Context) {
		x, exists := ctx.Get(lib.ClientChannelKey)
		if !exists {
			return
		}
		client, exists := x.(lib.Client)
		if !exists {
			return
		}

		ctx.Stream(func(w io.Writer) bool {
			// On each step, block until a message is received on the
			// Client channel.
			event, ok := <-client

			if !ok {
				return false
			}

			ctx.Render(-1, event)
			return true
		})
	})

	// Send messages
	router.POST("/message", func(ctx *gin.Context) {
		// Prevent buffer overloads
		reader := http.MaxBytesReader(ctx.Writer, ctx.Request.Body, MaxBytes)

		// Validate the JSON first
		messageData := &MessageData{}
		err := json.NewDecoder(reader).Decode(messageData)
		if err != nil || messageData.Message == "" {
			ctx.SecureJSON(http.StatusBadRequest, lib.JsonError{Error: fmt.Sprintf("Invalid data provided or data exceeds %d bytes.", MaxBytes)})
			return
		}

		// Populate the time property
		messageData.Time = time.Now().UnixMilli()

		// Encode the updated JSON
		encodedMessageData, err := json.Marshal(messageData)
		if err != nil {
			ctx.SecureJSON(http.StatusInternalServerError, lib.JsonError{Error: "Failed to parse data."})
			return
		}

		// Send the stringified version of the JSON as an event
		clientServer.Messages <- sse.Event{
			Event: "message",
			Data:  string(encodedMessageData),
		}

		ctx.SecureJSON(http.StatusOK, lib.JsonMessage{Message: "Message sent."})
	})

	router.StaticFile("/", "./index.html")

	router.Run(":3000")
}
