# Use golang to build the bot binary, and then add it to the fizmo-remglk
# image to get the complete set of tools.
# Docker Hub doesn't seem to support the "AS build" syntax!
# FROM golang:alpine AS build
FROM golang:alpine

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
    make acquire-external-tools; \
    echo "build/install..."; \
    make build; \
    echo "cleanup..."; \
    apk del .build-deps; \
    echo "DONE";

# CMD ["go-wrapper", "run"]


FROM jaredreisinger/fizmo-remglk

# Docker Hub doesn't seem to support the "AS build" syntax, so we have to use
# a numerical reference...
# COPY --from=build /go/src/github.com/JaredReisinger/xyzzybot/xyzzybot /usr/local/bin/.
COPY --from=0 /go/src/github.com/JaredReisinger/xyzzybot/xyzzybot /usr/local/bin/.
