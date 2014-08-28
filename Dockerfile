FROM golang:1.3
ADD . /go/src/github.com/panoptix/grand-ambassador
WORKDIR /go/src/github.com/panoptix/grand-ambassador
RUN go get && go build && cp docker-grand-ambassador /usr/bin/grand-ambassador
ENTRYPOINT ["/usr/bin/grand-ambassador"]
CMD []

