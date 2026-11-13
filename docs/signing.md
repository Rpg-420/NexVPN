# Release Signing (Ed25519)

Every release artifact gets an Ed25519 signature sidecar (`<file>.sig`, hex)
produced by the release workflow. Anyone can verify a download offline:

```
go run ./tools/verify-signature <public-key-hex> NexVPN-Setup-v1.4.1.exe NexVPN-Setup-v1.4.1.exe.sig
# → "signature OK" (exit 0)  |  "SIGNATURE INVALID" (exit 1)
```

## One-time setup

1. **Generate a keypair** — from a directory **outside this repository**
   (the private key must never be committed, and must not be inside the repo
   while you work):

   ```bash
   cd D:/App1
   go run ./tools/keygen -out C:/Users/Ahmad/nexvpn-signing-keys
   ```

   Output: `private.hex` (SECRET, 128 hex chars) and `public.hex` (to pin).
   `-out` keeps both files OUTSIDE the repository, whatever directory you run
   the tool from.

2. **Store the private key** as the `APP_SIGNING_PRIVATE_KEY` repository secret
   (GitHub → repo → Settings → Secrets and variables → Actions → New repository
   secret → Name: `APP_SIGNING_PRIVATE_KEY`, Secret: paste the *entire* content
   of `private.hex`). The release workflow signs every artifact only when this
   secret exists.

3. **Pin the public key** — paste the content of `public.hex` into the line
   below (and keep it in the release-notes template):

`RELEASE_SIGNING_PUBLIC_KEY=7544f81ab0fdf8504601bef822422a09c396d9abbd0f30c5dbc315b3949a8818`

4. **Verify the whole chain** (same flow the workflow performs):

   ```bash
   # sign something locally the way CI does (needs the secret value in the env)
   APP_SIGNING_PRIVATE_KEY=$(cat /tmp/nexvpn-keys/private.hex) \
     go run ./tools/sign-artifact build/bin/NexVPN.exe
   # then check it like a user would
   go run ./tools/verify-signature "$(cat /tmp/nexvpn-keys/public.hex)" \
     build/bin/NexVPN.exe build/bin/NexVPN.exe.sig
   # → "signature OK"   (a modified copy must print "SIGNATURE INVALID", exit 1)
   ```

## Why

Pinned SHA-256 digests protect the *core* update path against a compromised
mirror, but the app *release* artifacts themselves travel over GitHub releases.
A signature produced in the release pipeline (where the private key lives only
as a repository secret) means a tampered installer — on any mirror, CDN, or
re-upload — is detectable by anyone with this public key, offline.
