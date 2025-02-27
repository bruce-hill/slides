#!/usr/bin/env python3
#
# This file contains a simple test program for demonstrating some basic Python
# BTUI usage.
#
import btui.Python.btui as btui
import subprocess
import re
import marko

from marko.helpers import MarkoExtension
from marko.inline import *
from pygments import highlight
from pygments.lexers import get_lexer_by_name
from pygments.formatters import Terminal256Formatter
from pygments.util import ClassNotFound

FORMATTER = Terminal256Formatter(style="monokai")

def render_width(text:str)->int:
    text = re.sub("\033\\[[\\d;]*.", "", text)
    text = re.sub("\033\\(.", "", text)
    return len(text)

def boxed(text:str, line_numbers=False, title="", box_color="", min_width=0):
    lines = text.splitlines()
    width = 2 + max(render_width(line) for line in lines) + 2
    if line_numbers: width += len(str(len(lines))) + 1
    width = max(min_width, width)

    rendered = [box_color + "\033(0l" + "q"*(width-2) + "k\033(B" + "\033[m"]
    for i,line in enumerate(lines):
        pad = width - render_width(line) - 4
        if line_numbers:
            num_w = len(str(len(lines)))
            pad -= num_w+1
            rendered.append(f"{box_color}\033(0x\033(B\033[0;2m{i+1:>{num_w}}\033(0x\033(B\033[m {line}{' '*pad} {box_color}\033(0x\033(B\033[m")
        else:
            rendered.append(f"{box_color}\033(0x\033(B\033[m {line}{' '*pad} {box_color}\033(0x\033(B\033[m")
    rendered.append(box_color + "\033(0m" + "q"*(width-2) + "j\033(B" + "\033[m")
    return "\n".join(rendered)

class TerminalRenderer(marko.Renderer):
    width = 40

    def render_document(self, element):
        return self.render_children(element)

    def render_heading(self, element):
        return f"\033[1;7m{self.render_children(element):^{TerminalRenderer.width}}\033[22;27m\n\n"

    def render_children(self, element):
        if isinstance(element, str):
            return element  # Avoid treating it as an object
        return super().render_children(element)

    def render_list(self, element) -> str:
        lines = []
        if element.ordered:
            for num, child in enumerate(element.children, element.start):
                lines.append(f"\033[1m{num}.\033[m {self.render(child).rstrip()}\n")
        else:
            for child in element.children:
                lines.append(f"\033[1m\033(0`\033(B\033[m {self.render(child).rstrip()}\n")
        return "".join(lines) + "\n"

    def render_link(self, element) -> str:
        return f"\033[34;4m{self.render_children(element)}\033[0m"

    def render_emphasis(self, element) -> str:
        return f"\033[3m{self.render_children(element)}\033[23m"

    def render_strong_emphasis(self, element) -> str:
        return f"\033[1m{self.render_children(element)}\033[22m"

    def render_code_span(self, element) -> str:
        return f"\033[1;32;48;2;40;50;40m{element.children}\033[22;39;49m"

    def render_raw_text(self, element) -> str:
        assert(isinstance(element.children, str))
        return str(element.children)

    def render_literal(self, element) -> str:
        return self.render_raw_text(element)

    def render_code_block(self, element) -> str:
        raw_code = element.children[0].children
        if element.lang == "run":
            lexer = get_lexer_by_name("bash", stripall=True)
            code = highlight(raw_code, lexer, FORMATTER)
            output = subprocess.check_output(["script", "-qc", raw_code.strip(), "/dev/null"], stdin=open("/dev/null", "r")).decode("utf-8").rstrip("\n")
            return boxed("\033[33;1m$\033[m " + code + "\n" + output, line_numbers=False, box_color="\033[33m", min_width=TerminalRenderer.width) + "\n\n"

        code = raw_code
        assert(isinstance(code, str))
        try:
            lexer = get_lexer_by_name(element.lang, stripall=True)
        except ClassNotFound:
            pass
        else:
            code = highlight(code, lexer, FORMATTER)

        return boxed(code, line_numbers=True, box_color="\033[34m", min_width=TerminalRenderer.width) + "\n\n"

    def render_fenced_code(self, element) -> str:
        return self.render_code_block(element)

    def render_quote(self, element) -> str:
        return f"\033[34;3m{self.render_children(element)}\033[39;23m"

    def render_paragraph(self, element) -> str:
        return f"{self.render_children(element).strip()}\n\n"

markdown = marko.Markdown(renderer=TerminalRenderer)
def render_markdown(md_text: str) -> str:
    ast = markdown.parse(md_text)
    return markdown.render(ast)

def show_slide(bt:btui.BTUI, slides:[str], index:int, raw=False):
    slide = slides[index]
    with bt.buffered():
        bt.clear()

        if raw:
            lexer = get_lexer_by_name("markdown", stripall=True)
            code = highlight(slides[index], lexer, FORMATTER)
            for i,line in enumerate(code.splitlines()):
                bt.move(0, i)
                bt.write(line)
            return

        TerminalRenderer.width = bt.width//4
        rendered = render_markdown(slide)
        TerminalRenderer.width = max(render_width(line) for line in rendered.splitlines())
        rendered = render_markdown(slide)

        lines = rendered.splitlines()
        width = max(render_width(line) for line in lines)
        height = len(lines)

        x = max(0, (bt.width - width)//2)
        y = max(0, (bt.height - height)//2)
        for i,line in enumerate(rendered.splitlines()):
            bt.move(x, y + i)
            bt.write(line)

        pos_str = f"{index+1}/{len(slides)}"
        bt.move(bt.width-len(pos_str), bt.height)
        with bt.attributes("dim"):
            bt.write(pos_str)


def present(slides:[str]):
    prev_index = None
    index = 0
    raw = False
    with btui.open() as bt:
        key = None
        x, y = 1, 1
        while key != 'q' and key != 'Ctrl-c':
            if index != prev_index:
                show_slide(bt, slides, index, raw=raw)
                prev_index = index

            key, mx, my = bt.getkey()
            if mx: x, y = mx, my

            if key == 'Left' or key == 'k' or key == 'Backspace':
                index = max(0, index - 1)
            elif key == 'Right' or key == 'Space' or key == 'j':
                index = min(len(slides)-1, index + 1)
            elif key == 'Ctrl-r' or key == 'r':
                show_slide(bt, slides, index, raw=raw)
            elif key == 'Home' or key == 'h':
                index = 0
            elif key == 'End' or key == 'l':
                index = len(slides)-1
            elif key == 'Ctrl-z':
                bt.suspend()
            elif key == '`':
                raw = not raw
                prev_index = None
            elif key in '0123456789':
                bt.move(1, bt.height)
                with bt.attributes("bold"):
                    bt.write("Go to slide: ")
                index_str = ''
                while key in '0123456789':
                    bt.write(key)
                    index_str += key
                    key, mx, my = bt.getkey()
                    if mx: x, y = mx, my

                if key == 'Enter':
                    index = max(0, min(len(slides)-1, int(index_str)-1))

if __name__ == "__main__":
    import sys
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} file1.slides [file2.slides...]")
        sys.exit(1)


    slides = []
    for filename in sys.argv[1:]:
        try:
            with open(filename) as f:
                text = f.read()
                if any(filename.endswith(ext) for ext in (".slides", ".md", ".txt")):
                    if text.startswith("#!"):
                        _,text = text.split('\n', maxsplit=1)
                    slides += [slide.strip() for slide in re.split(r'(?m)^\-{3,}$', text)]
                else:
                    extension = filename.rpartition(".")[2] if "." in filename else ""
                    slides += [f"```{extension}\n{text.strip()}\n```"]
        except FileNotFoundError:
            print(f"File not found: {filename}")
            sys.exit(1)
    present(slides)
