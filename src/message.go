package main

type Message struct {
	Topic string `json:"topic"`
	Text  string `json:"text"`
	ReceiveDate  string `json:"receive_date"`
	SendDate  string `json:"send_date"`
}