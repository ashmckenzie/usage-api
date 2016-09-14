FROM golang:latest
MAINTAINER ash@the-rebellion.net

ENV GOPATH /gopath
ENV APP_HOME /app
ENV RELEASE_DIR /release
ARG CURRENT_USER_UID
ARG BUILD_USER

RUN apt-get -qq update && apt-get -qq install -y sudo bash
RUN useradd -d ${APP_HOME} -u ${CURRENT_USER_UID} builder

RUN mkdir ${GOPATH} ${RELEASE_DIR}
WORKDIR ${APP_HOME}

COPY . ${APP_HOME}/

RUN chown -R builder ${APP_HOME} ${GOPATH}

RUN make static

RUN echo "extract_releases:\n\tinstall -o ${BUILD_USER} release/* ${RELEASE_DIR}/" > Makefile
