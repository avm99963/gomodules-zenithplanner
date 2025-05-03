# ZenithPlanner

## Under development

If you want to gain some context about this project, you may read the initial
project plan at [//docs/project\_plan](./docs/project_plan/README.md).

In the future, when I can dedicate more time to this project, I will update
this README file to be much more friendly.

## To do

- Write this README file.
- Set up CI/CD.
- Fix known bugs:
    - Recurring events sometimes are not processed well, and they are sometimes
      deleted.
    - When starting the backend, it will send a watch request to Google
      Calendar even if a webhook is already active, so we will end up receiving
      duplicate requests
- Refactor code.

## Release

```sh
bazel run //cmd/backend:push_all --config=release
```
