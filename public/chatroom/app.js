window.addEventListener("DOMContentLoaded", (_) => { 
  let websocket = new WebSocket("ws://" + window.location.host + "/websocket");
  let room = document.getElementById("chat-text");
  path = window.location.pathname;
  crKey = path.replace("/chatroom/",'').replace("/",'');

  websocket.addEventListener("message", function (e) {
    console.log("HELLO")
    let data = JSON.parse(e.data);
    console.log(data);
    let p = document.createElement("p")
    p.innerHTML = `<p><strong>${data.Firstname}: </strong>${data.Message}</p>`;
    room.append(p);
    room.scrollTop = room.scrollHeight; // Auto scroll to the bottom
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
