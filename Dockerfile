# docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -f Dockerfile -t bnhf/androidhdmi-for-channels . --push --no-cache
FROM golang:bullseye AS builder
RUN apt update && apt install -y git
RUN mkdir -p /go/src/github.com/bnhf
WORKDIR /go/src/github.com/bnhf
RUN git clone https://github.com/bnhf/androidhdmi-for-channels .
RUN go build -o /opt/androidhdmi-for-channels1
RUN sed -i "s|//2||g" main.go \
    && go build -o /opt/androidhdmi-for-channels2
RUN sed -i "s|//3||g" main.go \
    && go build -o /opt/androidhdmi-for-channels3
RUN sed -i "s|//4||g" main.go \
    && go build -o /opt/androidhdmi-for-channels4

From debian:latest
RUN apt update && apt install -y adb
RUN mkdir -p /opt/scripts
WORKDIR /opt/scripts
COPY --from=builder /opt/androidhdmi-for-channels* /opt
COPY start.sh ..
COPY scripts/* . 
EXPOSE 7654
CMD ../start.sh