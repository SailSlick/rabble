FROM rabblenetwork/rabble_base

RUN mkdir -p /repo/

# Install dependencies
RUN apk add --no-cache --update protobuf
RUN apk add --no-cache --update nodejs nodejs-npm chromium nss
RUN apk add --no-cache --update go ca-certificates git

# Install gRPC for Go
RUN mkdir /go/
ENV GOPATH "/go"
ENV PATH $PATH:/go/bin
RUN go get -u google.golang.org/grpc
RUN go get -u github.com/golang/protobuf/protoc-gen-go

# Dependencies for skinny server
RUN go get -u \
  github.com/gorilla/mux \
  github.com/gorilla/sessions \
  github.com/gomarkdown/markdown \
  github.com/microcosm-cc/bluemonday \
  github.com/mmcdole/gofeed \
  github.com/blevesearch/bleve/... \
  github.com/writeas/go-webfinger

# Dependencies for auth service
RUN apk add --no-cache libffi-dev
RUN apk add --update openssl
RUN pip3 install bcrypt

# Dependencies for proto builds
RUN pip3 install grpcio-tools googleapis-common-protos
RUN apk add --no-cache --update protobuf-dev

CMD ["/repo/containers/build_container/entry.sh"]
