PREFIX=${HOME}/.local

all: dist/slides/slides

dist/slides/slides: btui/Python/libbtui.so slides.py .dependencies-installed
	uv run pyinstaller -y --onedir --add-binary btui/Python/libbtui.so:. slides.py

btui/Python/libbtui.so: btui/Makefile
	make -C btui python

btui/Makefile:
	git submodule update --init --recursive

.dependencies-installed: pyproject.toml
	uv sync && date >.dependencies-installed

install: dist/slides/slides
	install -d $(PREFIX)/share/slides/
	cp -r dist/slides/* $(PREFIX)/share/slides/
	ln -sf $(PREFIX)/share/slides/slides ~/.local/bin/slides

.PHONY: all install virtualenv
