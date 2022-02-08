# Contributing to `rename-pvc`

Welcome and thank you for making it this far and considering contributing to `rename-pvc`.
We always appreciate any contributions by raising issues, improving the documentation, fixing bugs in the CLI or adding new features.

Before opening a PR please read through this document.

## Process of making an addition

> Please keep in mind to open an issue whenever you plan to make an addition to features to discuss it before implementing it.

To contribute any code to this repository just do the following:

1. Make sure you have Go's latest version installed
2. Fork this repository
3. Run `make all` to make sure everything's setup correctly
4. Make your changes
   > Please follow the [seven rules of great Git commit messages](https://chris.beams.io/posts/git-commit/#seven-rules)
   > and make sure to keep your commits clean and atomic.
   > Your PR won't be squashed before merging so the commits should tell a story.
   >
   > Add documentation and tests for your addition if needed.
5. Run `make lint test` to ensure your code is ready to be merged
   > If any linting issues occur please fix them.
   > Using a nolint directive should only be used as a last resort.
6. Open a PR and make sure the CI pipelines succeeds.
7. Wait for one of the maintainers to review your code and react to the comments.
8. After approval merge the PR
9. Thank you for your contribution! :)
