FROM golang

RUN go get -u github.com/golang/dep/cmd/dep

COPY . /go/src/dashboard
WORKDIR /go/src/dashboard

RUN dep ensure
RUN go build

EXPOSE 8080

CMD ./dashboard