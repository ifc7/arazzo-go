# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26 AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
      -ldflags "-s -w \
        -X github.com/ifc7/arazzo-go/internal.BuildVersion=${VERSION} \
        -X github.com/ifc7/arazzo-go/internal.GitCommit=${GIT_COMMIT} \
        -X github.com/ifc7/arazzo-go/internal.BuildTime=${BUILD_TIME}" \
      -o /out/arazzo ./cmd/arazzo

FROM gcr.io/distroless/static:nonroot

COPY --from=build /out/arazzo /usr/local/bin/arazzo

WORKDIR /work

ENTRYPOINT ["arazzo"]
