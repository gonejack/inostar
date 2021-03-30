# inostar
Command line for converting inoreader starred.json to html

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/gonejack/inostar)
![Build](https://github.com/gonejack/inostar/actions/workflows/go.yml/badge.svg)
[![GitHub license](https://img.shields.io/github/license/gonejack/inostar.svg?color=blue)](LICENSE)


### Install
```shell
> go get github.com/gonejack/inostar
```

### Usage
```shell
> inostar starred.json
```
```
Usage:
  inostar *.json [flags]

Flags:
  -e, --offline   download remote images and replace html <img> references
  -v, --verbose   verbose
  -h, --help      help for inostar
```
