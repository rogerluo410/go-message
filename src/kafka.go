package main

import (
	"errors"
	"encoding/json"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/sirupsen/logrus"
)

type Consumer struct {
	conn  *kafka.Consumer
	topics []string
}

func (c *Consumer) read(client *Client) {
	c.conn.SubscribeTopics(c.topics, nil)
	defer func() {
		log.Info("Received client close signal, consumer read goroutine closed!")
		if (c.conn != nil) {
			c.conn.Close()
			log.Info("Received client close signal, consumer conn closed!")
		}
		c.conn = nil
	}()

	for {
		if (c.conn == nil) {
			return
		}
		select {
			case command := <- client.closeSignal:
				log.WithFields(logrus.Fields{"Consumer Caught signal": command}).Info("Caught signal terminating")
				return
			default:
			ev, err := c.poll(100)
			if err != nil {
				log.WithFields(logrus.Fields{"Consumer read failed": err}).Error("Err all broker down")
				return
			} else {
				if ev != nil {
					log.WithFields(logrus.Fields{"On topicPartition": ev.TopicPartition, "Value": string(ev.Value)}).Info("Kafka消息已读，将推送到Websocket连接上")
					var m Message
					json.Unmarshal(ev.Value, &m)
					client.message <- m
				}
			}
		}
	}
}

func (c *Consumer) poll(timeoutMS int) (*kafka.Message, error) {
	ev := c.conn.Poll(timeoutMS)
	if ev == nil {
		return nil, nil
	}

	switch e := ev.(type) {
	case *kafka.Message:
		return e, nil
	case kafka.OffsetsCommitted:
		log.WithFields(logrus.Fields{"event": e}).Info("kafka read OffsetsCommitted")
	case kafka.Error:
		if e.Code() == kafka.ErrAllBrokersDown {
			return nil, errors.New(e.String())
		}
	default:
		log.WithFields(logrus.Fields{"event": e}).Info("kafka read unknown event type")
	}

	return nil, nil
}

func (c *Consumer) close() {
	if (c.conn != nil) {
		c.conn.Close()
		log.Info("Consumer closed by calling close method!")
	}
	c.conn = nil
}

type Producer struct {
	conn  *kafka.Producer
	flushWait int
}

func (p *Producer) create() {
	// p.conn = conn
	// p.flushWait = flushWait
	go p.monitor()
}

func (p *Producer) monitor() {
	for e := range p.conn.Events() {
		switch ev := e.(type) {
		case *kafka.Message:
			l := log.WithFields(logrus.Fields{"Type": "kafka.Message"})
			if ev.TopicPartition.Error != nil {
				l.WithFields(logrus.Fields{"TopicPartition": ev.TopicPartition}).Error("Delivery failed")
			} else {
				l.WithFields(logrus.Fields{"TopicPartition": ev.TopicPartition}).Info("Delivered message success")
			}
		}
	}
}


func (p *Producer) write(topic string, msg Message) {
	if (p.conn == nil) {
		return
	}
	data, _ := json.Marshal(msg)
	l := log.WithFields(logrus.Fields{"Msg": msg, "Topic": topic})
	l.Info("Kafka is ready to publish message")
	err := p.conn.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          data,
	}, nil)
	if (err != nil) {
    l.WithFields(logrus.Fields{"Err": err}).Error("Kafka publishes message failed")
	}
	p.conn.Flush(p.flushWait)
}

func (p *Producer) close() {
	if (p.conn != nil) {
    p.conn.Close()
	}
	p.conn = nil
}