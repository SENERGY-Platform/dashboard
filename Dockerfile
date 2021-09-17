FROM golang:1.17

COPY . /go/src/dashboard
WORKDIR /go/src/dashboard

ENV GO111MODULE=on

RUN go build

EXPOSE 8080

CMD ./dashboard
