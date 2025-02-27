PREFIX=${HOME}/.local

all: slides

slides: btui.py libbtui.so slides.py
	pyinstaller --onefile --add-binary ./libbtui.so:. --distpath . slides.py

btui.py:
	make -C btui python && cp btui/Python/btui.py .

libbtui.so:
	make -C btui python && cp btui/Python/libbtui.so .

install: slides
	sudo install -p slides $(PREFIX)/bin/
