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

Replace this with good stuff

## Installing on a Linux Host

Replace this with valid information
Use the startup.sh script then restart.sh

## References

1. [ZKillboard API](https://github.com/zKillboard/zKillboard/wiki/API-(Statistics))
2. [CCP ESI API](https://esi.tech.ccp.is/latest/)
3. [Pirate's Little Helper](eve-plh.com)
4. [Javascript DataTables](https://datatables.net/)
