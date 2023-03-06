FROM golang:1.20-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY cmd/greeting-operator/main.go ./

RUN go build -o /operator

CMD [ "/operator" ]
