# 🌚 ZenithPlanner

**ZenithPlanner** lets you plan (and keep a record of) the locations where you
will be working from in Google Calendar.

It creates a daily event for every day of the week, and you can change its
title to the desired working location. The color of the event will
automatically be updated to reflect the type of location, and you will receive
a confirmation email with the change.

![Screenshot of Google Calendar, showing the created daily events with
different colors.](./docs/img/calendar.jpg)

Afterwards, you can view a dashboard which summarizes where you're working from
and helps you keep track of your progress and habits:

![Screenshot of the Grafana dashboard, which shows a count for "Total Vacation
Days" and "Total Study Days", a list of locations, and a pie graph and
accompanying table for the "Location distribution".](./docs/img/dashboard.jpg)

## Motivation

When I was working full-time for [Basetis][basetis], I fell in love with an
internal tool called [Cadiretis][cadiretis] ([internal
documentation][cadiretis-internal] –link might not be up to date). At the
office, there was a hot desk system: by default employees didn't have a fixed
desk; instead, this tool allowed us to book a seat in advance directly from
Google Calendar. We could even book adjacent seats for teammates or let a
randomizer decide our fate :)

Thus, this project is an effort to replicate part of Cadiretis (on which we
have been very heavily inspired) for nostalgic reasons, and to deal with some
new personal needs.

Right now I'm in a study impass, and I wanted to track where I will be
studying/working from everyday. This is so I can plan my study locations in
advance, keep track of my "vacation" days and try to reach a balance between
the different working locations, so e.g. I don't stay at home too often.

### Initial project plan

If you want to gain more context about this project, you may read the initial
project plan at [//docs/project\_plan][project-plan], but it is a little bit
outdated.

*** note
**Disclaimer:** the initial project plan and the inital version of this project
were developed using an LLM (Gemini 2.5 Pro (experimental)). [Read more about
the process.][llm-usage]
***

## Getting started

If you want to use this project, please join me in the fun! Here's what you
need to do:

1. Create a calendar for ZenithPlanner events in Google Calendar.
1. Create a project in Google Console, enable the Calendar API and create an
   OAuth client.
1. Generate a refresh token in order to be able to access your calendar:

   ``` sh
   (
       export GOOGLE_CLIENT_ID=changeme
       export GOOGLE_CLIENT_SECRET=changeme
       bazel run //cmd/oauthcli
   )
   ```

1. Set up Docker Compose with [examples/compose.yml][compose] and
   [examples/.env.example][env].
1. Start the database with `docker compose up -d db`, and run the initial
   database migration via `docker compose -f compose.yml exec db psql -U
   zenithplanner -d zenithplanner < database/schema.sql`.
1. Start the whole ZenithPlanner system with `docker compose up -d`.

## More documentation

- [Roadmap][roadmap]
- [Set up a development environment][development]
- [Release][release]

[basetis]: https://www.basetis.com/
[cadiretis]: https://memoria21.basetis.com/en/covid-impact/#:~:text=We%20launch%20Cadiretis,covid%20data.
[cadiretis-internal]: https://intranet.basetis.com/support-tools/cadiretis-app
[project-plan]: ./docs/project_plan/README.md
[llm-usage]: ./docs/llm_usage.md
[compose]: ./examples/compose.yml
[env]: ./examples/.env.example
[roadmap]: ./docs/roadmap.md
[development]: ./docs/development.md
[release]: ./docs/release.md
