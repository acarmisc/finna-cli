# tap/

This directory is a local preview of what the `acarmisc/homebrew-finna` tap
repository will contain after the first release.

## How it works

1. You create a git tag: `git tag v0.1.0 && git push origin v0.1.0`
2. `.github/workflows/release.yml` fires, runs goreleaser
3. goreleaser builds 4 binaries (darwin/linux × amd64/arm64), creates archives,
   generates SBOMs, signs the checksum file with cosign (keyless OIDC), and
   pushes `tap/Formula/finna.rb` (filled with real SHAs) to
   `github.com/acarmisc/homebrew-finna`.

## Setup checklist

- [ ] Create `github.com/acarmisc/homebrew-finna` (public repo)
- [ ] Generate a fine-grained PAT with **Contents: write** on that repo
- [ ] Add it as `HOMEBREW_TAP_TOKEN` secret in the finna-cli repo
- [ ] `GITHUB_TOKEN` is automatically provided by Actions (no setup needed)

## User install after first release

```sh
brew tap acarmisc/finna
brew install finna
```
