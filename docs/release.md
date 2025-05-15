# Release

After committing, one can build OCI images and upload them to all the
registries by running:

```sh
bazel run //cmd/backend:push_all --config=release
```

This command will push the images with tags corresponding to the current
checked out commit hash.

As time allows, the intention is to support a proper release cycle with version
tags.
