FROM alpine:3.7

WORKDIR /run

COPY ./aux-service .

CMD ["./aux-service"]