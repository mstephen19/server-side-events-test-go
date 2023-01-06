package lib

import (
	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
)

const port = "3000"

const ClientChannelKey = "ClientChannel"

// A client-specific channel to which messages can be sent
type Client chan sse.Event

type EventManager struct {
	// A channel for receiving messages send via a POST
	// route request. Each message can then be sent to each
	// Client channel.
	Messages chan sse.Event
	// The channel that new Client channels can be sent to
	// in order to add them to the ClientMap.
	newClients chan Client
	// A Client channel is sent on this channel when the
	// corresponding connection has ended. Then, that
	// Client channel can be deleted from the ClientMap.
	closedClients chan Client
	// A map where only the keys are used for easy additions
	// and deletions of Client channels.
	clientMap map[Client]bool
}

// Endlessly loops, listening for messages on the EventManager's
// channels.
// Intended to be run as a goroutine.
func (em *EventManager) start() {
	for {
		// Waits for one of the cases to pass before
		// iterating again.
		select {
		// If a new message is received on the main Messages channel,
		// Send the message to each Client channel so that it can be
		// send in the client's event stream.
		case message := <-em.Messages:
			for client := range em.clientMap {
				client <- message
			}
			// When a new client is received on the NewClients channel,
			// immediately add it to the ClientMap.
		case newClient := <-em.newClients:
			em.clientMap[newClient] = true
			// If a client is received on the ClosedClients channel, it
			// will be closed and deleted from the ClientMap.
		case closedClient := <-em.closedClients:
			delete(em.clientMap, closedClient)
			close(closedClient)
		}
	}
}

func setHeaders(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")
}

func (em *EventManager) ClientMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		client := make(Client)
		// Send the new client to the NewClients channel to be added to
		// the ClientMap.
		em.newClients <- client
		// Add the client to the context so that it can be used later on.
		ctx.Set(ClientChannelKey, client)

		// Once the connection has ended, send the client over to the
		// ClosedClients channel to be removed from the ClientMap.
		defer func() {
			em.closedClients <- client
		}()

		setHeaders(ctx)
		ctx.Writer.Flush()

		ctx.Next()
	}
}

func CreateServer() (server *EventManager) {
	// Create the EventManager
	server = &EventManager{
		Messages:      make(chan sse.Event),
		newClients:    make(chan Client),
		closedClients: make(chan Client),
		clientMap:     make(map[Client]bool),
	}

	// Start the server in a goroutine
	go server.start()

	return
}
