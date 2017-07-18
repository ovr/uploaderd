FROM golang:1.8.3

MAINTAINER Patsura Dmitry <talk@dmtry.me>

ENV PATH /go/bin:/usr/local/go/bin:/usr/bin$PATH
ENV GOPATH /go
ENV PKG_CONFIG_PATH /ffmpeg_build/lib/pkgconfig:$PKG_CONFIG_PATH
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
        yasm \
        bzip2 \
        autoconf \
        automake \
        build-essential \
        libass-dev \
        libfreetype6-dev \
        libtheora-dev \
        libtool \
        libvorbis-dev \
        pkg-config \
        texinfo \
        zlib1g-dev \
        gzip \
    && mkdir ffmpeg_sources && mkdir ffmpeg_build \
    && cd ffmpeg_sources \
    && curl -L -O http://downloads.sourceforge.net/project/lame/lame/3.99/lame-3.99.5.tar.gz \
    && tar xzvf lame-3.99.5.tar.gz && cd lame-3.99.5 \
    && ./configure --prefix="/ffmpeg_build" --enable-nasm --disable-shared \
    && make && make install && cd .. \
    && curl -L -O http://ffmpeg.org/releases/ffmpeg-3.3.2.tar.bz2 \
    && tar xjvf ffmpeg-3.3.2.tar.bz2 && cd ffmpeg-3.3.2 \
    && ./configure \
      --prefix="/ffmpeg_build" \
      --pkg-config-flags="--static" \
      --extra-cflags="-I/ffmpeg_build/include" \
      --extra-ldflags="-L/ffmpeg_build/lib" \
      --bindir="/usr/bin" \
      --enable-libmp3lame \
    && make && make install && cd .. \
    && rm -rf ffmpeg-3.3.2 && rm -rf lame-3.99.5 \
    && curl -o ImageMagick.tar.gz https://codeload.github.com/ImageMagick/ImageMagick/tar.gz/$IMAGEMAGICK_VERSION \
    && tar xvzf ImageMagick.tar.gz && rm ImageMagick.tar.gz \
    && cd ImageMagick-* \
    && ./configure --without-magick-plus-plus \
    && make && make install && ldconfig /usr/local/lib && cd .. \
    && rm -rf ImageMagick-* \
    && curl https://glide.sh/get | sh \
    && glide install \
    && apt-get remove -y curl git bzip2 \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

RUN go install github.com/interpals/uploaderd

ENTRYPOINT /bin/bash start.sh

EXPOSE 8989
