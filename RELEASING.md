# Releasing The Prometheus NATS Exporter

The Prometheus NATS exporter release process creates releases when a new Github tag is generated and pushed.  It uses [goreleaser](https://goreleaser.com/) to to do this.  To add plaforms, architectures, and assets, modify [.goreleaser.yaml](.goreleaser.yml).  This document is for maintainers with push access to the repo.

## Steps to create a new release

Determine your release version.  For these instructions, we'll use `v0.0.1-test` as an example.  Visit the [releases](https://github.com/nats-io/prometheus-nats-exporter/releases) page and **MAKE SURE YOU DO NOT DUPLICATE A RELEASE!**

### 1) Update docker files

Modify the docker files to pull down your future release.

In the dockerfiles modify the git clone command:
```text
RUN git clone --branch <your new release tag> https://github.com/nats-io/prometheus-nats-exporter.git .
``` 

For example:
```text
RUN git clone --branch v0.0.1-test https://github.com/nats-io/prometheus-nats-exporter.git .
```

Create a PR and merge into master.

### 2) Create and push a tag to the repository

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

This will trigger Travis-CI to start a build from the creation of the new tags.  Goreleaser will create a *draft* release by the NATS continuous integration user.  This can take awhile.

**NOTE:**  If modifying the release process itself, you can test by pushing a tag from a branch.  Use a tag like `v0.0.1-test` to do this.  No need to test on master.

### 3) Test the dockerfiles

At this point, you have tagged the repo with your new version and have a draft release.  Test building the docker files and sanity check functionality.  While building, you can ignore messages indicating that `You are in 'detached HEAD' state.` - it is OK.

**If something is wrong, delete the tag and draft release on github**, fix the problem (which may require another PR), and start over.  The release will have never been made public (although the tag was visible for awhile).

### 4) Edit the Release on Github

Check the [releases](https://github.com/nats-io/prometheus-nats-exporter/releases) page, and you should see a draft release generated from your tag, along with compiled binaries.  If the release and assets aren't present, check the Travis CI logs for errors, delete the draft release and tag, and start over.

### 5) Publish the release

Edit the release notes and publish.  You have a release!  Update the docker library, other distibution channels, and let the world know.
