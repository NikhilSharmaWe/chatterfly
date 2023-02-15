window.addEventListener("DOMContentLoaded", (_) => {
    let websocket = new WebSocket("ws://" + window.location.host + "/websocket");
    let room = document.getElementById("chatbox");
    let chatinfo = document.getElementById("usersender")
    let sender;
    let receiver;
  
    websocket.addEventListener("message", function (e) {
      let data = JSON.parse(e.data);
      let p = document.createElement("p")
      p.innerHTML = `<p><strong>${data.user}</strong>: ${data.text}</p>`;
      room.append(p);
      room.scrollTop = room.scrollHeight; // Auto scroll to the bottom
     });
  
     document.getElementById("input-form").addEventListener("submit", function (event) {
      event.preventDefault();
      let username = document.getElementById("username");
      let friend = document.getElementById("friend");
      let password = document.getElementById("password");
      sender = username.value;
      receiver = friend.value;
      websocket.send(
        JSON.stringify({
          user: username.value,
          friend: friend.value,
          password: password.value,
        })
      );
    });
    document.getElementById("chat-form").addEventListener("submit", function (event) {
        event.preventDefault();
        let message = document.getElementById("message");
        let p = document.createElement("p");
        p = `Sender: ${sender} | Receiver: ${receiver}`;
        chatinfo.append(p);        
        websocket.send(
          JSON.stringify({
            sender: sender,
            receiver: receiver,
            password: message.value,
          })
        );
        document.getElementById("message") = "";
      });
  });
