FROM golang:1.7

RUN apt-get update && apt-get install -y libsqlite3-dev
RUN curl https://glide.sh/get | sh

ADD . src/app
WORKDIR src/app
RUN glide install
RUN go install -v app

CMD ["app"]
