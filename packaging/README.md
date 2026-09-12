# Packaging

Ready-to-submit package manifests for doupass, generated from release **v0.1.0-rc4** (hashes verified against the signed `checksums.txt`).

> Recommendation: submit to package managers once a stable `v0.1.0` exists; prerelease identifiers are rejected by some channels. These files are templates for that moment (and for private taps/buckets today).

## Scoop (Windows)

`packaging/scoop/doupass.json` is a complete manifest. To publish:

1. Create a bucket repository (for example `ogzhncnmr/scoop-bucket`) with a `bucket/` directory.
2. Copy the manifest to `bucket/doupass.json`.
3. Users install with `scoop bucket add ogzhncnmr https://github.com/ogzhncnmr/scoop-bucket && scoop install doupass`.

## Homebrew (macOS/Linux)

`packaging/homebrew/doupass.rb` is a formula. To publish:

1. Create a tap repository `ogzhncnmr/homebrew-tap` with a `Formula/` directory.
2. Copy the formula to `Formula/doupass.rb`.
3. Users install with `brew tap ogzhncnmr/tap && brew install doupass`.

A local test before pushing: `brew install --build-from-source ./packaging/homebrew/doupass.rb`.

## winget (Windows)

`packaging/winget/` contains the three manifests (`version`, `installer`, `locale`). To publish:

1. Install `wingetcreate`: `winget install Microsoft.WingetCreate`.
2. `wingetcreate update Ogzhncnmr.Doupass --version <new> --urls <x64-url> <arm64-url> --submit` after the package exists, or open a PR to `microsoft/winget-pkgs` with these files under `manifests/o/Ogzhncnmr/Doupass/<version>/`.

## Refreshing hashes

Hashes are copied verbatim from the release's `checksums.txt`. After each release:

1. Download `checksums.txt` and its `.sigstore.json` bundle.
2. Verify the checksums first (see below).
3. Update the version and hash fields in the three package families.

## Verify a release before packaging

```sh
cosign verify-blob checksums.txt --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp "^https://github.com/ogzhncnmr/doupass/.github/workflows/release.yml@refs/tags/" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com"

sha256sum --check checksums.txt --ignore-missing
```
