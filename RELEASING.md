# The Prometheus NATS exporter release process

The Prometheus NATS exporter release process creates releases when a new Github tag is generated and pushed.  It uses [goreleaser](https://goreleaser.com/) to to do this.  To add plaforms, architectures, and assets, modify [.goreleaser.yaml](.goreleaser.yml).

## Steps to create a new release

### 1) Create and push a tag to the repository

**NEVER DUPLICATE AN EXISTING TAG!**

From your local repository, you'll want to be on *master* branch.  Create a local tag and push:

```text
git tag -a vX.Y.Z
git push origin vX.Y.Z
```

e.g. 

```text
$ git tag -a v0.0.1-test
$ git push origin v0.0.1-test
Counting objects: 1, done.
Writing objects: 100% (1/1), 173 bytes | 173.00 KiB/s, done.
Total 1 (delta 0), reused 0 (delta 0)
To github.com:nats-io/prometheus-nats-exporter
 * [new tag]         v0.0.1-test -> v0.0.1-test
```

This will trigger Travis-CI to start a build from the creation of the new tags.  Goreleaser will create a draft release using the NATS continuous integration user.  This can take awhile.

**NOTE:**  If modifying the release process itself, you can test by pushing a tag from a branch.  Use a tag like `v0.0.1-test` to do this.  No need to test on master.

### 2) Edit the Release on Github

Check the the [releases](https://github.com/nats-io/prometheus-nats-exporter/releases) page, and you should see a draft release generated from your tag, along with compiled binaries.  If the release and assets aren't present, check the Travis CI logs for errors.  Edit the release notes and publish.  You have a github release!

### 3) Edit docker files

Modify the docker files to pull down the latest release, and test them.  After committing and merging, use the commit hash from the docker file updates in the docker library.  This means the official docker files for a release will be at least one commit ahead, but that's OK; we have to test.

### 4) Update other distribution channels

Don't forget to update Homebrew, Chocolatey, etc if applicable.
