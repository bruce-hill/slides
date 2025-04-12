# slides - A terminal slide presenter

Slides is a simple terminal application that uses [btui](https://github.com/bruce-hill/btui).

Read or run `./sample.md` to see usage instructions.

## Building

The build process if you want a standalone binary uses [pyinstaller](https://pyinstaller.org):

First, make sure you have the `btui` submodule:

```
git submodule update --init --recursive
```

Then get the python dependencies in a python virtual environment:

```
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

Then build the `slides` executable:

```
make
```

To install, run `make install` or `make PREFIX=/usr/local install` if you want
to install to a location other than `~/.local/bin`.
