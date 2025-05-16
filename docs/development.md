# Set up a development environment

In order to set up the project for local development, follow the steps in the
["Getting started" section of the README][getting-started], with the following
exceptions:

- Use [compose.dev.yml][compose] instead of the example for production. The
  schema doesn't need to be manually created.
- Run the backend with the following command:

  ```sh
  bazel run //cmd/backend
  ```

[getting-started]: ../README.md#getting-started
[compose]: ../compose.dev.yml
