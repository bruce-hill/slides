PREFIX=${HOME}/.local

all: dist/slides/slides

dist/slides/slides: btui/Python/libbtui.so slides.py .dependencies-installed
	source .venv/bin/activate && pyinstaller -y --onedir --add-binary btui/Python/libbtui.so:. slides.py

btui/Python/libbtui.so: btui/Makefile
	make -C btui python

btui/Makefile:
	git submodule update --init --recursive

virtualenv: .dependencies-installed .venv/bin/activate

.venv/bin/activate:
	python3 -m venv .venv

.dependencies-installed: requirements.txt .venv/bin/activate
	source .venv/bin/activate && pip install -r requirements.txt && date >.dependencies-installed

install: dist/slides/slides
	install -d $(PREFIX)/share/slides/
	cp -r dist/slides/* $(PREFIX)/share/slides/
	ln -sf $(PREFIX)/share/slides/slides ~/.local/bin/slides

.PHONY: all install virtualenv
