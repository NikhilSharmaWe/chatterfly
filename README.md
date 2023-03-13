# Chatterfly
Chatterfly is a chat application built using Golang and websockets.

### Features
- Real-time messaging using websockets
- Session management with Redis
- User data, chat room data, and chat data stored in MongoDB
- Use of cookies for authentication and security

### Usage
1. Login
2. Create a chatroom
3. Share the unique URL generated for your chatroom with your friends
4. By sharing the link, anyone who is logged in will have the ability to join the chatroom

### Running locally
- Run `docker compose up` for the following docker compose file in the repo: [docker-compose.yaml](https://github.com/NikhilSharmaWe/chatterfly/blob/main/docker-compose.yaml)
