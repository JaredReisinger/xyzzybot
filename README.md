# xyzzybot

Bringing interactive fiction to Slack


## Goals

With the popularity of Slack (and related tools), there's been a resurgence of
attention on text-based, conversation interaction.  As a long-time fan of
interactive fiction, it's only natural to want to combine the two.


## Setup

_(TBD)_


## Interaction model

Before diving into a discussion of the interaction model, we need to understand
all of the _kinds_ of interaction/context that will exist, keeping in mind that
a game can be in progress in a channel with multiple participants.  Given that,
a message always has an intended recipient:

* **the in-progress game** — for game commands, like `go north`, or `take lamp`

* **xyzzybot itself** — for interacting with xyzzybot itself ("start playing a
  new game", "what's your status?")

* **other channel members** — messges for other channel members should
  generally be ignored by xyzzybot

Ideally, the experience will feel "natural" in Slack, with the bot/app all but
disappearing behind the story.  In a typical conversation with people, you don't
need to address them directly with each message; there is context that can be
inferred.  To approximate this, as a very simple heuristic, xyzzybot will assume
that messages of 4 words or less are _probably_ game commands if there is a game
currently in progress.

If another Slack user is addressed explicity (`@username` anywhere in the
message), the message will be considered part of the general conversation, and
ignored.

If xyzzybot is addressed explicitly at the beginning of the message (`@xyzzybot
help`), the rest of the message is either a game command, or a xyzzybot command.
If there's not a game in progress, _or_ the first word is preceded by an
exclamation mark (`!`), the message is treated as a xyzzybot command.  If there
is a game in progress, and the first word is _not_ preceded by an exclamation
mark, the message is passed to the game.  (The meta-command prefix was
originally a slash, but that has special meaning to Slack, and can't reliably be
used without deeper slash-command integration.)

> An implication of this is that it is _always_ possible to ensure that a
> xyzzybot command will be understood: whether there is a game in progress or
> not, `@xyzzybot !foo` will _**always**_ be treated as a command `foo` meant
> for xyzzybot itself.


## Components

_(TBD)_


## Roadmap

_(TBD)_
