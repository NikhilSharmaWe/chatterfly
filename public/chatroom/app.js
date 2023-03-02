window.addEventListener("DOMContentLoaded", (_) => {
    let websocket = new WebSocket("ws://" + window.location.host + "/websocket");
  });