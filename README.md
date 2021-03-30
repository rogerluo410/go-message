# 消息发布/订阅系统
 Kafka / Websocket  

# Development
go run message.go kafka.go websocket.go client.go main.go  

# Build
go build ./  

# Deployment
faas-cli up --yaml morgan.yml  

# Description

* 消息格式

```json
  Json:
  {
    "topic": "xxxx", //指定发布消息到哪一个topic, 使用Kafka client也带上这个field
    "text": "<a>123</a>",
    "receive_date": "2019-12-16T15:01:38Z", //服务器接收消息时间(服务器设置)
    "send_date": "2019-12-16T15:01:38Z",  //服务器发送消息时间(服务器设置)
  }
```

* 绑定Websocket连接，需要指定订阅的topic(可以指定多个topic)和Client UUID   
  e.g: ws://localhost:10086/ws?`topic=rogerluo`&`topic=rogerluo1`&`uuid=rogerluo`  

* 测试地址  
  http://localhost:10086/topic1.html  
  http://localhost:10086/topic2.html  

* Kubernetes负载均衡器Ingress支持websocket配置： 
```json
kind: Ingress
metadata:
  annotations:
    field.cattle.io/ingress.class: traefik
    traefik.wss.protocol: https
```       