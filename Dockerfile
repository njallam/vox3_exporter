FROM golang:alpine as builder
RUN apk update && apk add --no-cache git
WORKDIR $GOPATH/src/vox3_exporter
COPY . .
RUN go get -d -v
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-s -w" -o /go/bin/vox3_exporter

FROM scratch
COPY --from=builder /go/bin/vox3_exporter /go/bin/vox3_exporter
COPY --from=builder /go/src/vox3_exporter/nat.html nat.html
EXPOSE 9917
ENTRYPOINT ["/go/bin/vox3_exporter"]
