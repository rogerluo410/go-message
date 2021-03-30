package main

import (
	"time"
	"sync"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/sirupsen/logrus"
)

type Client struct {
	uuid           string
	createTime     string
	cc             *Consumer
	pp             *Producer
	wss            chan *WsConn
	rmWss          chan *WsConn
	wsConns        []*WsConn
	message        chan Message
	closeSignal    chan int
}

func (c *Client) connect() {
	c.wss = make(chan *WsConn)
	c.rmWss = make(chan *WsConn)
	c.message = make(chan Message)
	c.closeSignal = make(chan int, 0)
	c.wsConns = make([]*WsConn, 0)
	go c.cc.read(c)
	go c.connHandler()
}

func (c *Client) connHandler() {
	for {
		select {
			case msg := <- c.message:
				msg.SendDate = time.Now().Format(time.RFC3339)
				for _, wsConn := range c.wsConns {
					if err := wsConn.ws.WriteJSON(msg); err != nil {
						// 如果遇到ws写错误，则关闭websocket连接
						wsConn.connected = false
						log.WithFields(logrus.Fields{
							"Client": c.uuid,
							"Address": wsConn.address,
							"data": msg,
							"err":  err,
						}).Error("ws send message failed, will send close signal to ws conn object")
						wsConn.closeSignal <- 1
					}
				}
			case conn := <- c.wss:
				c.wsConns = append(c.wsConns, conn)
				log.WithFields(logrus.Fields{
					"Client": c.uuid,
					"Address": conn.address,
					"ws count": len(c.wsConns),
				}).Info("connHandler is involved new ws conn")
			case conn := <- c.rmWss:
				c.removeConn(conn)
			case command := <- c.closeSignal:
				if (command == 1) {
					log.WithFields(logrus.Fields{
						"Client": c.uuid,
					}).Info("received client close signal, connHandler will exit")
					return
				}
		}
	}
}

func (c *Client) removeConn(remove *WsConn) {
	var i int
	var found bool
	for i = 0; i < len(c.wsConns); i++ {
		if c.wsConns[i] == remove {
			found = true
			break
		}
	}
	if !found {
		log.WithFields(logrus.Fields{
			"Address": remove.address,
			"ws count": len(c.wsConns),
		}).Info("No ws conn found")
		return
	} else {
		log.WithFields(logrus.Fields{
			"Address": remove.address,
			"ws count": len(c.wsConns),
			"ws index": i,
		}).Info("Remove the ws conn, and send close signal to the removed conn")
		remove.closeSignal <- 1 //发送关闭协程信号
		copy(c.wsConns[i:], c.wsConns[i+1:]) // shift down
		c.wsConns[len(c.wsConns)-1] = nil    // nil last element
		c.wsConns = c.wsConns[:len(c.wsConns)-1]  // truncate slice
		return
	}

}

func (c *Client) deConnect() {
	log.Info("Send client close signal...")
	c.closeSignal <- 1
	log.Info("Send client close signal, will destory connHandler goroutine")
	// If directly close consumer conn, will blocked on close() method
	// c.cc.close()
	// log.Info("收到关闭客户端消息, 销毁kafka consumer连接")
	log.Info("Send client close signal again, will destory kafka consumer goroutine")
	c.closeSignal <- 1
	c.pp.close()
	log.Info("Send client close signal, will destory kafka producer")
}

type ClientManager struct {
	list       []*Client
	lock       *sync.Mutex
	waitSleep  time.Duration
}

func (cm *ClientManager) add(c *Client) {
  cm.list = append(cm.list, c)
}

func (cm *ClientManager) appendWs(uuid string, wsConn *WsConn, subTopics []string) {
	found := false
	for _, client := range cm.list {
		if (client.uuid == uuid) {
			log.Info("Locked by ClientManager.appendWs")
	    cm.lock.Lock()
			client.wss <- wsConn
			log.Info("Unlocked by ClientManager.appendWs when the client exists")
			cm.lock.Unlock()
			found = true

      client.closeSignal <- 0 // 关闭信号重制为0
			log.WithField("Client", client.uuid).Info("appendWs when the client exists successfully")
			go wsConn.writer(client)
			wsConn.reader(client) //blocked
			log.WithField("Client", client.uuid).Info("ws conn reader released the block in the client exists")
		}
	}
  // 没有找到uuid的客户端， 创建新的客户端
	if (!found) {
		c, err := kafka.NewConsumer(&kafka.ConfigMap{
			"bootstrap.servers": kafkaUrl,
			"group.id":          kafkaGroup,
			"auto.offset.reset": "earliest",
		})
		if err != nil {
			log.WithField("err", err).Error("Create Kafka consumer failed")
			return
		}
		p, err := kafka.NewProducer(&kafka.ConfigMap{
			"bootstrap.servers": kafkaUrl })
	
		if  err != nil { 
			log.WithField("err", err).Error("Create Kafka producer failed")
			return
		}
		log.WithField("Topics", subTopics).Info("Subscribe Topics successfully")
		consumer := &Consumer{conn: c, topics: subTopics}
		producer := &Producer{conn: p, flushWait: 15 * 1000}
		producer.create()
	
		client := &Client{ cc: consumer, pp: producer,
			uuid: uuid, createTime: time.Now().Format(time.RFC3339) }
		client.connect()
		client.wss <- wsConn
		log.Info("Locked by ClientManager.appendWs for New one")
	  cm.lock.Lock()
		cm.add(client)
		log.Info("Unlocked from new client after the ws conn joined in the client")
		cm.lock.Unlock()  //先解锁，在执行阻塞协程

		log.WithFields(logrus.Fields{
			"Client uuid": client.uuid,
			"Client create time":  client.createTime,
		}).Info("Client created successfully")

		go wsConn.writer(client)
		wsConn.reader(client)
		log.WithField("Client", client.uuid).Info("ws conn reader released the block in new client")
	}
}

func (cm *ClientManager) length() int {
  return len(cm.list)
}

func (cm *ClientManager) monitor() {
	log.Info("Startup Client manager monitor...")
	for {
		cm.lock.Lock()
		log.Info("locked by client manager")
		needRemoveClients := make([]int, 0)
    for i, client := range cm.list {
      if (len(client.wsConns) == 0 || client.pp.conn == nil || client.cc.conn == nil) {
				// 客户端关闭，从列表中删除
				needRemoveClients = append(needRemoveClients, i)
			}
		}
		log.WithFields(logrus.Fields{"Client list": needRemoveClients}).Info("Need to remove from list")

		for _, removeIndex := range needRemoveClients {
			cm.list[removeIndex].deConnect()
			cm.list = append(cm.list[:removeIndex], cm.list[removeIndex+1:]...)
		}
		log.Info("Unlocked by client manager")
		cm.lock.Unlock()
		
		log.WithFields(logrus.Fields{"Active clients": cm.length()}).Info("Client manager monitoring...")
		time.Sleep(cm.waitSleep)
	}
}

func NewClientManager() ClientManager {
	return ClientManager{list: make([]*Client, 0), 
		waitSleep: time.Second * 30, 
		lock: &sync.Mutex{},
	}
}