# slides - A terminal slide presenter

Slides is a simple terminal application that uses [btui](https://github.com/bruce-hill/btui).

Read or run `./sample.md` to see usage instructions.

## Building

The build process if you want a standalone binary uses [pyinstaller](https://pyinstaller.org).

You can do the entire process by running

```
make
```

## Installing

To install, run `make install` or `make PREFIX=/usr/local install` if you want
to install to a location other than `~/.local/bin`.
