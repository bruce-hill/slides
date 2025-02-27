PREFIX=${HOME}/.local

all: slides

slides: btui/Python/libbtui.so slides.py
	pyinstaller --onefile --add-binary btui/Python/libbtui.so:. --distpath . slides.py

btui/Python/libbtui.so:
	make -C btui python

install: slides
	sudo install -p slides $(PREFIX)/bin/
