FROM alpine
MAINTAINER ash@the-rebellion.net

RUN apk --update add bash curl wget

ENV APP_HOME /app

RUN cd /usr/local/bin && curl https://bin.equinox.io/c/ekMN3bCZFUn/forego-stable-linux-amd64.tgz | tar xzvf - && chmod 755 forego

RUN mkdir ${APP_HOME}
WORKDIR ${APP_HOME}

COPY Procfile app/release/* ${APP_HOME}/

CMD forego start
