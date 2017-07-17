FROM golang:1.8.3

MAINTAINER Patsura Dmitry <talk@dmtry.me>

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
ENV IMAGEMAGICK_VERSION 7.0.6-1

RUN mkdir -p /etc/confd/{conf.d,templates}
RUN mkdir -p /etc/interpals

COPY conf.d /etc/confd/conf.d
COPY templates /etc/confd/templates

ADD https://github.com/kelseyhightower/confd/releases/download/v0.11.0/confd-0.11.0-linux-amd64 /usr/local/bin/confd
RUN chmod +x /usr/local/bin/confd

ADD . /go/src/github.com/interpals/uploaderd
WORKDIR /go/src/github.com/interpals/uploaderd

RUN apt-get update \
    && apt-get -y upgrade \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        git \
        curl \
        libzmq3-dev \
        libjpeg-dev \
        libpng-dev \
        libgif-dev \
    && curl -o ImageMagick.tar.gz http://www.imagemagick.org/download/ImageMagick-$IMAGEMAGICK_VERSION.tar.gz \
    && tar xvzf ImageMagick.tar.gz && rm ImageMagick.tar.gz \
    && cd ImageMagick-* \
    && ./configure --without-magick-plus-plus \
    && make && make install && ldconfig /usr/local/lib && cd .. \
    && rm -rf ImageMagick-* \
    && curl https://glide.sh/get | sh \
    && glide install \
    && apt-get remove -y curl git \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

RUN go install github.com/interpals/uploaderd

ENTRYPOINT /bin/bash start.sh

EXPOSE 8989
