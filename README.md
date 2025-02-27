# slides - A terminal slide presenter

Slides is a simple terminal application that uses [btui](https://github.com/bruce-hill/btui).

Run `./sample.slides` to see usage instructions.

## Building

The build process if you want a standalone binary uses [pyinstaller](https://pyinstaller.org):

First, make sure you have the `btui` submodule:
```
git submodule update --init --recursive
```

Then build the `slides` executable:

```
make
```

To install, run `make install` or `make PREFIX=/usr/local install` if you want
to install to a location other than `~/.local/bin`.
