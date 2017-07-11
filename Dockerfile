# Use golang to build the bot binary, and then add it to the fizmo-remglk
# image to get the complete set of tools.
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
    make acquire-external-tools; \
    echo "build/install..."; \
    make build; \
    echo "cleanup..."; \
    apk del .build-deps; \
    echo "DONE";

# CMD ["go-wrapper", "run"]


FROM jaredreisinger/fizmo-remglk

COPY --from=build /go/src/github.com/JaredReisinger/xyzzybot/xyzzybot /usr/local/bin/.
