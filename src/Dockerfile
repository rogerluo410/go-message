# FROM ubuntu
FROM golang:1.13-buster

WORKDIR /root
RUN git clone https://github.com/edenhill/librdkafka.git
WORKDIR /root/librdkafka
RUN LDFLAGS='-Wl,-rpath,\$$ORIGIN' /root/librdkafka/configure
RUN make
RUN make install
RUN ldconfig

WORKDIR /home/app
RUN groupadd app
RUN useradd -g app app
RUN chown app:app -R /home/app

COPY . .

RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN GOOS=linux go build .

USER app

EXPOSE 8080

HEALTHCHECK --interval=3s CMD [ -e /tmp/.lock ] || exit 1

ENTRYPOINT ./morgan
