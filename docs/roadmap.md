# Roadmap

Until we have a proper issue tracker, here are some of the things we have
planned for the future:

- Set up CI/CD.
- Fix known bugs:
    - When starting the backend, it will send a watch request to Google
      Calendar even if a webhook is already active, so we will end up receiving
      duplicate requests
- Refactor code.
- Send mail to Cadiretis when an Office-type assignation is changed.
- Add more configuration options. Right now a lot of options are hard-coded in
  the codebase which doesn't make this program useful for other people. This
  change will allow everyone to adapt this program to their needs. Some
  examples of things which should be configurable.
  - Ability to customize who to notify of changes (generalizing the previous
    bullet point).
  - Ability to customize the working location types.
- Set up a proper issue tracker ;)
