package main

import (
	// "encoding/json"
	"io"
	"net/http"
	"time"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const (
	// Time allowed to write the file to the client.
	writeWait = 60 * time.Second

	// Time allowed to read the next pong message from the client.
	pongWait = 60 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		HandshakeTimeout: 62 * time.Second,
		CheckOrigin: func(r *http.Request) bool {
			//r.URL *url.URL
      //r.Header Header
			return true
		},
	}
)

type WsConn struct {
	ws             *websocket.Conn
	address        string
	message        chan Message
	closeSignal    chan int
	connected      bool
}

func (c *WsConn) close() {
	c.connected = false
  c.ws.Close()
}

func (c *WsConn) create() {
	c.connected = true
	c.message = make(chan Message)
	c.closeSignal = make(chan int, 0)
}

func (c *WsConn) reader(client *Client) {
	defer func() {
		c.ws.Close()
	} ()
	c.ws.SetReadLimit(1024 * 1024)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error { 
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		log.WithFields(logrus.Fields{
			"Client": client.uuid,
			"Address": c.address,
		}).Info("Received heartbeat from client...")
		// c.ws.WriteMessage(websocket.PongMessage, []byte{})
		return nil 
	})

	c.ws.SetCloseHandler(func(code int, text string) error {
		log.WithFields(logrus.Fields{"code": code, "text":text}).Info("Received close signal from client")
		c.ws.WriteMessage(websocket.CloseMessage, []byte{})
    return nil
	})
	for {
		m := Message{}
		err := c.ws.ReadJSON(&m)
		l := log.WithFields(logrus.Fields{"Client": client.uuid, "Client ws count": len(client.wss), 
		"Msg": m, "Err": err})
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway) || err == io.EOF {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				l.Info("Websocket client closed with websocket.CloseGoingAway, will close the websocket conn and finish the read goroutine!")
			} else {
				c.ws.WriteMessage(websocket.CloseAbnormalClosure, []byte(err.Error()))
				l.Error("Websocket read failed，will close the websocket conn and finish the read goroutine")
			}
			// 如果遇到ws读错误，则关闭websocket连接
			c.connected = false
			client.rmWss <- c
			break
		}
		l.Info("Read message from client，will publish to topic via Kafka producer")
		if (m.Topic != "" && c.connected == true && client.pp != nil) {
			m.ReceiveDate = time.Now().Format(time.RFC3339)
			go client.pp.write(m.Topic, m)
		}
	}
}

func (c *WsConn) writer(client *Client) {
	pingTicker := time.NewTicker(pingPeriod)
	// sendTicker := time.NewTicker(writeWait)
	defer func() {
		pingTicker.Stop()
		// sendTicker.Stop()
		c.ws.Close()
	}()
	for {
		select {
		case <-pingTicker.C:
			log.WithFields(logrus.Fields{
				"Client": client.uuid,
				"Address": c.address,
			}).Info("Send heartbeat to client...")
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			err := c.ws.WriteMessage(websocket.PingMessage, []byte{})
			if err != nil {
				// 如果遇到ws写错误，则关闭websocket连接
				c.connected = false
				// delete(client.wss, c.address)
				log.WithFields(logrus.Fields{
					"Client": client.uuid,
					"Client ws count": len(client.wss), 
					"Address": c.address,
					"err":  err,
				}).Error("Send heartbeat failed, will close the websocket conn and finish the writer goroutine")
				// return
				client.rmWss <- c
				return
			}
		// case msg := <- c.message:
		// 	msg.SendDate = time.Now().Format(time.RFC3339)
		// 	if err := c.ws.WriteJSON(msg); err != nil {
		// 		// 如果遇到ws写错误，则关闭websocket连接
		// 		c.connected = false
		// 		// delete(client.wss, c.address)
		// 		log.WithFields(logrus.Fields{
		// 			"Client": client.uuid,
		// 			"Client ws count": len(client.wss), 
		// 			"Address": c.address,
		// 			"data": msg,
		// 			"err":  err,
		// 		}).Error("Websocket写消息失败，将关闭websocket连接")
		// 		// return
		// 		client.rmWss <- c
		// 		return
		// 	}
		case command := <- c.closeSignal:
			if (command == 1) {
				c.connected = false
				log.WithFields(logrus.Fields{
					"Client": client.uuid,
					"Client ws count": len(client.wss), 
					"Address": c.address,
				}).Info("Received the close signal, will close the websocket conn and finish the writer goroutine")
				// return
				client.rmWss <- c
				return
			}
		}
	}
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
  w.Write([]byte("hello world\n"))
	return
}

// handleWebsocket connection.
func handleWebsocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	subTopics := r.URL.Query()["topic"]
	if len(subTopics) == 0 {
		http.Error(w, "No topic provide error", http.StatusMethodNotAllowed)
		return
	}

	uuid := r.URL.Query()["uuid"]
	if len(uuid) == 0 {
		http.Error(w, "No client uuid provide error", http.StatusMethodNotAllowed)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		m := "Unable to upgrade to websockets"
		log.WithField("err", err).Println(m)
		http.Error(w, m, http.StatusBadRequest)
		return
	}
	// 添加 client 或 追加 websocket 连接
	wsConn := &WsConn{ws: ws, address: ws.RemoteAddr().String()}
	wsConn.create()
	cm.appendWs(uuid[0], wsConn, subTopics)
}
