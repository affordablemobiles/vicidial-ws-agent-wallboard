package main

import (
    "fmt"
    "time"
    "log"
    "net/http"
    "sync"
    //"encoding/json"
    "github.com/gorilla/websocket"
    "github.com/satori/go.uuid"
)

var clientMutex = &sync.Mutex{}
var clientMAP map[string]*WallboardWS = make(map[string]*WallboardWS)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
}

const (
	// Time allowed to write a message to the peer.
	writeWait = time.Second
)

type WallboardWS struct {
	// The websocket connection.
	ws *websocket.Conn
	// Write Mutex
	writeMutex *sync.Mutex
	// Buffered channel of outbound messages.
	send chan []byte
	// String containing our random UUID
	uuid string
}

func sendLatestJSON(){
    var latestJSON []byte
    statusTicker := time.NewTicker(time.Second)

    for {
        select {
            case <-statusTicker.C:
                clientMutex.Lock()
                clients := len(clientMAP)
                clientMutex.Unlock()

                if clients > 0 {
                    latestJSON = AgentStatusJSON()

                    clientMutex.Lock()
                    for _, v := range clientMAP {
                        go v.write(websocket.TextMessage, latestJSON)
                    }
                    clientMutex.Unlock()
                }
        }
    }
}

func AgentStatusWS(w http.ResponseWriter, r *http.Request) {
    ws, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Print("upgrade:", err)
        return
    }
    c := &WallboardWS{send: make(chan []byte, 256), ws: ws}
    c.writeMutex = &sync.Mutex{}

    c.uuid = fmt.Sprintf("%s", uuid.NewV4())
    clientMutex.Lock()
    clientMAP[c.uuid] = c
    clientMutex.Unlock()

    for {
		_, _, err := c.ws.ReadMessage()
		if err != nil {
            break
		}
    }

    c.CloseCleanup()
}

func (c *WallboardWS) write(mt int, payload []byte) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, payload)
}

func (c *WallboardWS) CloseCleanup() {
	// Close the WebSocket connection.
	c.ws.Close()

	// Remove the user from our user map.
	clientMutex.Lock()
	if _, ok := clientMAP[c.uuid]; ok {
		delete(clientMAP, c.uuid)
	}
	clientMutex.Unlock()
}
