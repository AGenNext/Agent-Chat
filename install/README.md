# Installing agennextd on macOS

Three ways, fastest first.

## 1. Homebrew (recommended)
```sh
brew tap AGenNextHub/tap https://github.com/AGenNextHub/Agent-Chat
brew install agennextd
agennextd -demo
```
*(The formula is `install/agennextd.rb`. It installs the released binary for your
Mac — Apple Silicon or Intel.)*

## 2. One-line install script
```sh
curl -fsSL https://raw.githubusercontent.com/AGenNextHub/Agent-Chat/main/install/install.sh | sh
```
Downloads the release binary for your OS/arch; if none is published yet, it
builds from source (needs Go). Override with `VERSION=0.1.0` or `PREFIX=$HOME/.local`.

## 3. From source
```sh
git clone https://github.com/AGenNextHub/Agent-Chat.git
cd Agent-Chat
go build -o agennextd ./cmd/agennextd
./agennextd -demo
```

---

## For maintainers — cutting a macOS release
1. Build the binaries + checksums:
   ```sh
   VERSION=0.1.0 scripts/build-macos.sh    # -> dist/agennextd_0.1.0_darwin_{arm64,amd64}.tar.gz + checksums-darwin.txt
   ```
   *(Verified: produces real Mach-O arm64/amd64 binaries; CGO disabled, reproducible.)*
2. Create the GitHub release `v0.1.0` and upload both tarballs.
3. Paste the two sha256 values from `dist/checksums-darwin.txt` into
   `install/agennextd.rb` (replace the `REPLACE_WITH_*` placeholders).
4. Users can now `brew install` or run the install script.

> **Note:** no notarized `.pkg`/`.dmg` is provided — `agennextd` is a CLI, so
> Homebrew/curl is the norm. If you want a notarized GUI installer later, that
> needs an Apple Developer ID + `productbuild` + notarization; ask and I'll add it.
