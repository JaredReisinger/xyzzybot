# Use golang to build the bot binary, and then add it to the fizmo-remglk
# image to get the complete set of tools.

# Note that this will not auto-build on Docker Hub until shortly after
# Docker 17.06.01 CE is released.  (Currently slated for late July.)
FROM golang:alpine AS build

# Instead of /go/src/app, which is typical for golang and the go-wrapper helper,
# we go straight to the proper path for this tool.  By doing this we avoid
# having to worry about creating the symbolic link, etc., and most go tools will
# *just work*.
WORKDIR /go/src/github.com/JaredReisinger/xyzzybot
COPY . .

RUN set -eux; \
    echo "acquire tools..."; \
    apk add --no-cache --virtual .build-deps \
        git \
        make \
        ; \
    git clean -f -x -d; \
    echo "build/install..."; \
    make build; \
    echo "cleanup..."; \
    apk del .build-deps; \
    echo "DONE";

# CMD ["go-wrapper", "run"]


FROM jaredreisinger/fizmo-remglk

LABEL maintainer="jaredreisinger@hotmail.com" \
    xyzzybot.version="0.1"

COPY --from=build /go/src/github.com/JaredReisinger/xyzzybot/xyzzybot /usr/local/bin/

RUN set -eux; \
    echo "creating config directory"; \
    mkdir -p /usr/local/etc/xyzzybot; \
    echo "DONE";

VOLUME /usr/local/etc/xyzzybot

CMD [ "/usr/local/bin/xyzzybot" ]
