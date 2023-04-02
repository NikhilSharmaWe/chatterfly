const endpoint = 'http://localhost:4444/linkdata'

fetch(endpoint)
  .then(response => response.json())
  .then(data => {
    console.log(data.Chatrooms); 
    let crs = data.Chatrooms || [];
    const paragraph = document.getElementById('chatrooms');

    const listItems = crs.map(item => `<li><a href="${window.location.origin}/chatroom/c/${item.Key}/">${item.ChatRoomName}</li>`);

    const listHTML = listItems.join('');

    paragraph.innerHTML = `<ul>${listHTML}</ul>`;
});
