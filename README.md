# Signal Cartel's Little Helper

## Purpose

This is a utility for Eve Online, it looks up a list of characters and presents threat data. 
I have been a user of [Pirate's Little Helper](eve-plh.com) for awhile and this is
intended to replace that service for the Eve corporation, [Signal Cartel](http://www.eve-scout.com/signal-cartel/).

## Requirements

The server code is written in Go.  The client side
code is Javascript with DataTables.  The information is pulled from
CCP using the ESI interface and zkillboard using their API.

## Imports

1. [go-cache](https://github.com/patrickmn/go-cache)
```
$ go get github.com/patrickmn/go-cache
```
2. [mergo](https://github.com/imdario/mergo)
```
$ go get -u github.com/imdario/mergo
```
3. [logrus](https://github.com/sirupsen/logrus)
```
 $ go get -u github.com/sirupsen/logrus
```

## Running locally

Quick steps to build and run the server locally:

- Run directly (recommended for development):

```sh
# run with local mode (no TLS) and specify port
go run . -local -port 8443 -debug
```

- Build a binary and run it:

```sh
go build -o go-plh .
./go-plh -local -port 8080
```

- Useful flags:
  - `-local` : run without TLS (binds to :8443 by default when set)
  - `-port`  : port to listen on (default 80)
  - `-debug` : enable debug logging to stdout
  - `-kills` : enable extra kill analysis (slower)

## Testing

- Run the Go unit tests:

```sh
go test ./... -v
# include the race detector when appropriate
go test -race ./... -v
```

- Run specific tests:

```sh
go test ./... -run TestFetchKillHistory -v
```

## Formatting & Linting (JavaScript)

This repository includes ESLint and Prettier configs for the client-side code under `static/`.

- Install Node dev dependencies:

```sh
npm ci
```

- Format JavaScript files with Prettier:

```sh
npm run format
```

- Lint and auto-fix (if possible):

```sh
npm run lint
```

- CI check (what the GitHub Actions workflow runs):

```sh
npm run check
```

The CI workflow will run ESLint (strict, no auto-fix) and Prettier checks on push and pull requests for files under `static/`.

## Installing on a Linux Host

Use the startup.sh script then restart.sh

## References

1. [ZKillboard API](https://github.com/zKillboard/zKillboard/wiki/API-(Statistics))
2. [CCP ESI API](https://esi.tech.ccp.is/latest/)
3. [Pirate's Little Helper](eve-plh.com)
4. [Javascript DataTables](https://datatables.net/)
