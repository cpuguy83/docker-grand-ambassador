FROM golang:1.3
ADD . /go/src/github.com/cpuguy83/docker-grand-ambassador
WORKDIR /go/src/github.com/cpuguy83/docker-grand-ambassador
RUN go build && cp docker-grand-ambassador /usr/bin/grand-ambassador
ENTRYPOINT ["/usr/bin/grand-ambassador"]
CMD []

