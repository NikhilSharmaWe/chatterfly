window.addEventListener("DOMContentLoaded", (_) => { 
  let websocket = new WebSocket("ws://" + window.location.host + "/websocket");
  let room = document.getElementById("chat-text");
  path = window.location.pathname;
  crKey = path.replace("/chatroom/c/",'').replace("/",'');

  websocket.addEventListener("message", function (e) {
    let data = JSON.parse(e.data);
    if ("ChatRoomName" in data) {
      let chatroom = document.getElementById("chatroom");
      chatroom.innerHTML = data.ChatRoomName
    } else {
      let p = document.createElement("p")
      p.innerHTML = `<p><strong>${data.Firstname}: </strong>${data.Message}</p>`;
      room.append(p);
      room.scrollTop = room.scrollHeight;
    }
  });

  document.getElementById("input-form").addEventListener("submit", function (event) {
    event.preventDefault();
    let text = document.getElementById("input-text");
    websocket.send(
      JSON.stringify({
        key: crKey,
        message: text.value,
      })
    );
    document.getElementById("input-text").value = "";
  });
});
