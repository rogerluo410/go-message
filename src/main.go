package main

import (
	"net/http"
	"os"
	"github.com/sirupsen/logrus"
)

var (
	port = GetEnvDefault("MORGAN_PORT", "8080")
	kafkaUrl = GetEnvDefault("MORGAN_KAFKA_URL", "localhost")
	kafkaGroup = GetEnvDefault("MORGAN_KAFKA_GROUP", "wxj")
	wsOrigin = GetEnvDefault("MORGAN_CORS_ORIGIN", "")
)


var (
	log = logrus.WithField("cmd", "morgan")
	cm  ClientManager
)

func GetEnvDefault(key, defVal string) string {
	val, ex := os.LookupEnv(key)
	if !ex {
		return defVal
	}
	return val
}

func main() {
	port := port
	if port == "" {
		log.WithField("PORT", port).Fatal("$PORT must be set")
	}

	cm = NewClientManager()
	go cm.monitor()

	http.Handle("/", http.FileServer(http.Dir("./public")))
	http.HandleFunc("/ping", handlePing)
	http.HandleFunc("/ws", handleWebsocket)
	log.Info(http.ListenAndServe(":"+port, nil))
}
