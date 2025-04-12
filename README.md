# slides - A terminal slide presenter

Slides is a simple terminal application that uses [btui](https://github.com/bruce-hill/btui).

## Running

To run the slides program from this directory, run this command to set up a
python virtual environment with the necessary requirements:

```
make virtualenv
```

Then, you can view a markdown file by running `./slides.py sample.md`

## Building

If you want to create a standalone executable that can be installed, run:

```
make
```

...which will build a standalone executable file called `./dist/slides/slides`

## Installing

To install, run `make install` or `make PREFIX=/usr/local install` if you want
to install to a location other than `~/.local/bin`.
