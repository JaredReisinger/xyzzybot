# ![](https://github.com/JaredReisinger/xyzzybot/raw/master/docs/logo-60.png) xyzzybot

Before 8-bit, there was 7-bit...


## Goals

With the popularity of Slack (and related tools), there’s been a resurgence of
attention on text-based, conversational interaction.  As a long-time fan of
interactive fiction, it’s only natural to want to combine the two.


## Setup

> This project is not-yet-ready for primetime.  The intention is to provide a
> ready-to-run Docker image that only needs some configuration—for your team’s
> specific app account—and a volume for game files.
>
> _(more to come)_


## Interaction model

Before diving into a discussion of the interaction model, we need to understand
all of the _kinds_ of interaction/context that will exist, keeping in mind that
a game can be in progress in a channel with multiple participants.  Given that,
a message always has an intended recipient:

* **the in-progress game** — for game commands, like `go north`, or `take lamp`

* **xyzzybot itself** — for interacting with xyzzybot itself (“start playing a
  new game, “what’s your status?”)

* **other channel members** — messges for other channel members should
  generally be ignored by xyzzybot

Ideally, the experience will feel “natural” in Slack, with the bot/app all but
disappearing behind the story.  In a typical conversation with people, you don’t
need to address them directly with each message; there is context that can be
inferred.  To approximate this, as a very simple heuristic, xyzzybot will assume
that messages of 4 words or less are _probably_ game commands if there is a game
currently in progress.

If another Slack user is addressed explicity (`@username` anywhere in the
message), the message will be considered part of the general conversation, and
ignored.

If xyzzybot is addressed explicitly at the beginning of the message (`@xyzzybot
help`), the rest of the message is either a game command, or a xyzzybot command.
If there’s not a game in progress, _or_ the first word is preceded by an
exclamation mark (`!`), the message is treated as a xyzzybot command.  If there
is a game in progress, and the first word is _not_ preceded by an exclamation
mark, the message is passed to the game.  (The meta-command prefix was
originally a slash, but that has special meaning to Slack, and can’t reliably be
used without deeper slash-command integration.)

> An implication of this is that it is _always_ possible to ensure that a
> xyzzybot command will be understood: whether there is a game in progress or
> not, `@xyzzybot !foo` will _**always**_ be treated as a command `foo` meant
> for xyzzybot itself.


## Components

xyzzybot stands on the shoulders of giants... under the covers, it’s using
[Christoph Ender’s libfizmo](https://github.com/chrender/libfizmo) as
the game interpreter, which in turn is a combination of [Ender’s
libfizmo](https://github.com/chrender/libfizmo) and [Andrew Plotkin’s RemGlk
library](http://eblong.com/zarf/glk/index.html).  This provides a game
interpreter whose stdin/stdout is structured JSON data, rather than simple lines
of text from which paragraphs and other formatting must be inferred.  (These are
all conveniently packaged as the Docker image
[jaredreisinger/fizmo-json](https://hub.docker.com/r/jaredreisinger/fizmo-json/),
making it easy to acquire and consume.)

Internally, xyzzybot is composed of two basic parts: interacting with the game
interpreter (fizmo-json), and interacting with Slack.


### Interpreter model

> _(more to come)_


### Slack handling

> _(more to come)_


## Roadmap

These items are in roughly the order I think they’ll be addressed, but things
may shift:

* [x] run interpreter as a subprocess

* [x] proxy input/output between Slack and interpreter

* [x] improve `status` command (requires tracking better information per-channel)

* [ ] handle persistence across restarts (save/restore in-progress games)

* [x] create Docker image for easier deployment

* [ ] allow for on-the-fly configuration changes (and persist them)

* [ ] allow for per-channel configuration (and persistence)

* [x] administrative commands (a very few)

* [x] in-Slack commands for uploading new games (very simple, first-pass UI)

* [x] gameplay in xyzzybot direct messages (?)

> _(more to come)_
