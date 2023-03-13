FROM golang:1.19.3
WORKDIR /app
RUN cd /app && git clone https://github.com/NikhilSharmaWe/chatterfly.git . && go build -o main .
EXPOSE 4444
CMD ["/app/main"]
