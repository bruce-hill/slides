PREFIX=${HOME}/.local

all: dist/slides/slides

dist/slides/slides: btui/Python/libbtui.so slides.py
	pyinstaller -y --onedir --add-binary btui/Python/libbtui.so:. slides.py

btui/Python/libbtui.so:
	make -C btui python

install: dist/slides/slides
	install -d $(PREFIX)/share/slides/
	cp -r dist/slides/* $(PREFIX)/share/slides/
	ln -sf $(PREFIX)/share/slides/slides ~/.local/bin/slides
