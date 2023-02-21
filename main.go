package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	gubrak "github.com/novalagung/gubrak/v2"
	"github.com/regorov/logwriter"
)

type M map[string]interface{}

const MESSAGE_NEW_USER = "New User"
const MESSAGE_CHAT = "Chat"
const MESSAGE_LEAVE = "Leave"

var connections = make([]*WebSocketConnection, 0)

type SocketPayload struct {
	Message string
}

type SocketResponse struct {
	From    string
	Type    string
	Message string
}

type WebSocketConnection struct {
	*websocket.Conn
	Username string
}

func main() {
	i := 0
	router := gin.Default()
	upgrader := websocket.Upgrader{
		WriteBufferSize: 1024,
		ReadBufferSize:  1024,
	}

	cfg := &logwriter.Config{
		BufferSize:       0,                 // no buffering
		FreezeInterval:   1 * time.Hour,     // freeze log file every hour
		HotMaxSize:       10 * logwriter.MB, // 100 MB max file size
		CompressColdFile: true,              // compress cold file
		HotPath:          "./log",
		ColdPath:         "./log/arch",
		Mode:             logwriter.ProductionMode, // write to file only if prod and write to console and file if debugmode
	}

	lw, err := logwriter.NewLogWriter("chat app", cfg, false, nil)
	if err != nil {
		panic(err.Error())
	}

	logger := log.New(lw, "chatting app", log.Ldate|log.Ltime)
	logger.Println("Start chat app")

	router.LoadHTMLFiles("./index.html")
	router.GET("/", func(ctx *gin.Context) {
		ctx.HTML(http.StatusOK, "index.html", nil)
	})

	router.GET("/ws", func(ctx *gin.Context) {
		currentGorillaConn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			http.Error(ctx.Writer, "Could not open websocket connection", http.StatusBadRequest)
		}
		username := ctx.Query("username")
		i++
		logger.Printf("koneksi ke-%d, username : %s\n", i, username)
		logger.Printf("jumlah koneksi : %d\n", len(connections))
		currentConn := &WebSocketConnection{Conn: currentGorillaConn, Username: username}
		connections = append(connections, currentConn)
		go handleIO(currentConn, connections)
	})

	fmt.Println("Server starting at :10020")
	router.Run(":10020")
}

func handleIO(currentConn *WebSocketConnection, connections []*WebSocketConnection) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("ERROR", fmt.Sprintf("%v", r))
		}
	}()

	broadcastMessage(currentConn, MESSAGE_NEW_USER, "")

	for {
		payload := SocketPayload{}
		err := currentConn.ReadJSON(&payload)
		if err != nil {
			if strings.Contains(err.Error(), "websocket: close") {
				broadcastMessage(currentConn, MESSAGE_LEAVE, "")
				ejectConnection(currentConn)
				return
			}
			log.Println("ERROR", err.Error())
			continue
		}

		broadcastMessage(currentConn, MESSAGE_CHAT, payload.Message)
	}
}

func ejectConnection(currentConn *WebSocketConnection) {
	filtered := gubrak.From(connections).Reject(func(each *WebSocketConnection) bool {
		return each == currentConn
	}).Result()
	connections = filtered.([]*WebSocketConnection)
}

func broadcastMessage(currentConn *WebSocketConnection, kind, message string) {
	for _, eachConn := range connections {
		if eachConn == currentConn {
			continue
		}

		eachConn.WriteJSON(SocketResponse{
			From:    currentConn.Username,
			Type:    kind,
			Message: message,
		})
	}
}
