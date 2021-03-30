var box = new ReconnectingWebSocket("wss://morgan.qxyc.io/ws?topic=rogerluo&uuid=rogerluo");
// var box = new ReconnectingWebSocket("ws://localhost:8080/ws?topic=rogerluo&uuid=rogerluo");

box.onmessage = function(message) {
  var data = JSON.parse(message.data);
  $("#chat-text").prepend("<div class='panel panel-default'><div class='panel-heading'>" + $('<span/>').text("From morgan to " + data.topic + ", 发送时间:" + data.send_date).html() + "</div><div class='panel-body'>" + $('<span/>').text(data.text).html() + "</div></div>");
  // $("#chat-text").stop().animate({
  //   scrollTop: $('#chat-text')[0].scrollHeight
  // }, 800);
};

box.onclose = function(){
    console.log('box closed');
    this.box = new WebSocket(box.url);

};

$("#input-form").on("submit", function(event) {
  event.preventDefault();
  var handle = $("#input-send-topic")[0].value;
  var text   = $("#input-text")[0].value;
  box.send(JSON.stringify({ topic: handle, text: text }));
  $("#input-text")[0].value = "";
});
