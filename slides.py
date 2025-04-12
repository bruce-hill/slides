#!/usr/bin/env python3
#
# This file contains a simple test program for demonstrating some basic Python
# BTUI usage.
#
import btui.Python.btui as btui
import climage
import marko
import os
import re
import subprocess
import webbrowser
import PIL

from collections import namedtuple
from marko.helpers import MarkoExtension
from marko.inline import *
from pygments import highlight
from pygments.formatters import Terminal256Formatter
from pygments.lexers import get_lexer_by_name
from pygments.util import ClassNotFound
from wcwidth import wcswidth

Slide = namedtuple("Slide", ("filename", "text"))

FORMATTER = Terminal256Formatter(style="native")
BULLET = "\033[1m\033(0`\033(B\033[m"

terminal_width, terminal_height = 0, 0

def render_width(text:str)->int:
    # Strip out escape sequences that are used:
    text = text.replace(BULLET, "*")
    text = re.sub("\033\\[[\\d;]*.", "", text)
    text = re.sub("\033\\(.", "", text)
    text = re.sub("\t", "    ", text)

    width = wcswidth(text)
    assert(width >= 0) # Can happen if we missed some escape characters
    return width

def is_image(path:str)->bool:
    return any(path.lower().endswith(ext) for ext in ('.png', '.jpg', '.jpeg', '.bmp', '.gif', '.webp'))

def boxed(text:str, line_numbers=False, box_color="", min_width=0):
    text = re.sub("\t", "    ", text)
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
            rendered.append(f"{box_color}\033(0x\033(B\033[0;2m{box_color}{i+1:>{num_w}}\033(0x\033(B\033[m {line}{' '*pad} {box_color}\033(0x\033(B\033[m")
        else:
            rendered.append(f"{box_color}\033(0x\033(B\033[m {line}{' '*pad} {box_color}\033(0x\033(B\033[m")
    rendered.append(box_color + "\033(0m" + "q"*(width-2) + "j\033(B" + "\033[m")
    return "\n".join(rendered)

class TerminalRenderer(marko.Renderer):
    width = 40
    relative_filename = "."

    def render_document(self, element):
        return self.render_children(element)

    def render_children(self, element):
        if isinstance(element, str):
            return element  # Avoid treating it as an object
        return super().render_children(element)

    def render_heading(self, element):
        title = " "+self.render_children(element)+" "
        if element.level == 1:
            #return "\033[1;34m" + pyfiglet.figlet_format(title, width=200, font="big").rstrip('\n') + "\033[m\n\n"
            line = "\033[1;7m" + " "*TerminalRenderer.width + "\033[22;27m\n"
            return line + f"\033[1;7m{title:^{TerminalRenderer.width}}\033[22;27m\n" + line + "\n"
        else:
            return f"\033[1;7m{title:^{TerminalRenderer.width}}\033[22;27m\n\n"

    _list_depth = 0
    def render_list(self, element) -> str:
        lines = []
        if element.ordered:
            for num, child in enumerate(element.children, element.start):
                child.bullet = f"\033[1m{num:>2}.\033[m "
                child_rendered = self.render(child).strip('\n')
                lines.append(child_rendered)
        else:
            for child in element.children:
                child.bullet = "  "+BULLET+" "
                child_rendered = self.render(child).strip('\n')
                lines.append(child_rendered)
        return "\n".join(lines) + "\n\n"
    
    def render_list_item(self, element) -> str:
        indent = "  "*self._list_depth
        self._list_depth += 1
        result = indent + element.bullet + "\n".join(self.render(child).strip('\n') for child in element.children)
        self._list_depth -= 1
        return result

    def render_image(self, element) -> str:
        path = element.dest if os.path.isabs(element.dest) else os.path.join(os.path.dirname(self.relative_filename), element.dest)
        if is_image(path):
            img = PIL.Image.open(path)
            img_width, img_height = img.size
            render_width, render_height = img_width, img_height

            two_thirds_width = max(40, 2*terminal_width//3)
            if two_thirds_width < render_width:
                render_width = two_thirds_width
                render_height = int(img_height * (render_width/img_width)/2)

            two_thirds_height = max(40, 2*terminal_height//3)
            if two_thirds_height < render_height:
                render_height = two_thirds_height
                render_width = int(img_width * 2*(render_height/img_height))

            option_string = markdown.render(element.children[0]) if element.children else ""
            for w in re.findall(r'width=(\d+)%', option_string):
                render_width = int(terminal_width * float(w)/100)
                render_height = int(img_height * (render_width/img_width)/2)

            for h in re.findall(r'height=(\d+)%', option_string):
                render_height = int(terminal_height * float(h)/100)
                render_width = int(img_width * 2*(render_height/img_height))

            output = climage.convert(path, width=render_width, is_truecolor=True, is_256color=False, is_unicode=True)
            return output + "\n"

        try:
            # Embedded file:
            with open(path) as f:
                contents = f.read()
        except FileNotFoundError:
            return f"\n\033[31;1m<File not found: {element.dest}>\033[m\n"

        extension = path.rpartition(".")[2] if "." in path else ""
        try:
            lexer = get_lexer_by_name(extension, stripall=True)
        except ClassNotFound:
            code = contents
        else:
            code = highlight(contents, lexer, FORMATTER)

        title = self.render_children(element) or path
        heading = f"\033[1;36;7m{title:^{TerminalRenderer.width}}\033[22;27m"
        return "\n" + heading + "\n" + boxed(code, line_numbers=True, box_color="\033[36m", min_width=TerminalRenderer.width) + "\n\n"

    def render_link(self, element) -> str:
        title = self.render_children(element) or element.dest
        return f"\033[1;4;34m{title}\033[m"

    def render_emphasis(self, element) -> str:
        return f"\033[3m{self.render_children(element)}\033[23m"

    def render_strong_emphasis(self, element) -> str:
        return f"\033[1m{self.render_children(element)}\033[22m"

    def render_strikethrough(self, element) -> str:
        return f"\033[9m{self.render_children(element)}\033[29m"

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
            output = subprocess.check_output(
                ["script", "-qc", raw_code.strip(), "/dev/null"],
                stdin=open("/dev/null", "r"),
                cwd=os.path.dirname(TerminalRenderer.relative_filename) or '.',
            ).decode("utf-8").rstrip("\n")
            return boxed("\033[33;1m$\033[m " + code + "\n" + output, line_numbers=False, box_color="\033[33m", min_width=TerminalRenderer.width) + "\n\n"
        elif element.lang == "demo":
            lexer = get_lexer_by_name("bash", stripall=True)
            code = highlight(raw_code, lexer, FORMATTER)
            title = "Demo (press Enter to run)"
            heading = f"\033[1;32;7m{title:^{TerminalRenderer.width}}\033[22;27m"
            return heading + "\n" + boxed("\033[33;1m$\033[m " + code, line_numbers=False, box_color="\033[32m", min_width=TerminalRenderer.width) + "\n\n"

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

def get_demos(ast) -> list:
    if isinstance(ast, (marko.block.CodeBlock, marko.block.FencedCode)):
        if ast.lang == "demo":
            def demo():
                raw_code = ast.children[0].children
                subprocess.run(
                    ["bash", "-c", raw_code.strip()],
                    cwd=os.path.dirname(TerminalRenderer.relative_filename) or '.',
                )
            return [demo]
    elif isinstance(ast, marko.inline.Link):
        def demo():
            webbrowser.open(ast.dest, new=1)
        return [demo]
    elif isinstance(ast, marko.inline.Image):
        if not ast.children: return []
        path = ast.dest if os.path.isabs(ast.dest) else os.path.join(os.path.dirname(TerminalRenderer.relative_filename), ast.dest)
        if is_image(path):
            return [lambda: PIL.Image.open(path).show()]
    elif isinstance(ast, marko.element.Element):
        if hasattr(ast, "children"):
            demos = []
            for child in ast.children:
                demos += get_demos(child)
            return demos
        else:
            raise ValueError(f"No children! {ast}")
    elif not isinstance(ast, str):
        raise ValueError(f"Not an element! {ast}")
    return []

markdown = marko.Markdown(renderer=TerminalRenderer)
def render_markdown(slide:Slide) -> str:
    TerminalRenderer.relative_filename = slide.filename
    ast = markdown.parse(slide.text)
    return markdown.render(ast)

def show_slide(bt:btui.BTUI, slides:[Slide], index:int, *, scroll=0, raw=False) -> int:
    slide = slides[index]
    with bt.buffered():
        bt.clear()

        if raw:
            lexer = get_lexer_by_name("markdown", stripall=True)
            code = highlight(slide.text, lexer, FORMATTER)
            for i,line in enumerate(code.splitlines()):
                bt.move(0, i)
                bt.write(line)
            return

        if slide.text.strip():
            TerminalRenderer.width = bt.width//4
            rendered = render_markdown(slide)
            TerminalRenderer.width = max(render_width(line) for line in rendered.splitlines())
            rendered = render_markdown(slide)

            lines = rendered.splitlines()
            width = max(render_width(line) for line in lines)
            height = len(lines)

            x = max(0, (bt.width - width)//2)
            y = max(0, (bt.height - height)//2) - scroll
            for i,line in enumerate(rendered.splitlines()):
                if y + i in range(bt.height):
                    bt.move(x, y + i)
                    bt.write(line)
        else:
            width,height = 0,0

        pos_str = f"{index+1}/{len(slides)}"
        bt.move(bt.width-len(pos_str), bt.height)
        with bt.attributes("dim"):
            bt.write(pos_str)

    return height

def present(slides:[str]):
    global terminal_width, terminal_height
    redraw = True
    index, prev_index = 0, None
    raw = False
    scroll = 0
    render_height = 0
    search = ''
    with btui.open() as bt:
        terminal_width, terminal_height = bt.width, bt.height
        key = None
        while key != 'q' and key != 'Ctrl-c':
            if index != prev_index:
                redraw = True
                scroll = 0

                ast = markdown.parse(slides[index].text)
                demos = get_demos(ast)
                demo_index = 0

            if redraw:
                render_height = show_slide(bt, slides, index, scroll=scroll, raw=raw)
                redraw = False
                prev_index = index

            key, mx, my = bt.getkey()

            if key == 'Left' or key == 'k' or key == 'Backspace':
                index = max(0, index - 1)
            elif key == 'Right' or key == 'Space' or key == 'j':
                index = min(len(slides)-1, index + 1)
            elif key == "Up" or key == "Mouse wheel up":
                redraw = True
                scroll = max(0, scroll-1)
            elif key == "Ctrl-u":
                redraw = True
                scroll = max(0, scroll-10)
            elif key == "Down" or key == "Mouse wheel down":
                scroll = min(scroll+1, max(0, render_height-bt.height-1))
                redraw = True
            elif key == "Ctrl-d":
                scroll = min(scroll+10, max(0, render_height-bt.height-1))
                redraw = True
            elif key == 'Ctrl-r' or key == 'r':
                redraw = True
            elif key == 'Home' or key == 'h':
                index = 0
            elif key == 'End' or key == 'l':
                index = len(slides)-1
            elif key == 'Ctrl-z':
                bt.suspend()
                redraw = True
            elif key == 'Enter':
                if len(demos) > 0:
                    with bt.disabled():
                        bt.flush()
                        demos[demo_index]()
                        if demo_index + 1 < len(demos):
                            demo_index += 1
                    redraw = True
            elif key == '`':
                raw = not raw
                redraw = True
            elif key == "Resize":
                terminal_width, terminal_height = bt.width, bt.height
                redraw = True
            elif key in '0123456789':
                bt.move(1, bt.height)
                with bt.attributes("bold"):
                    bt.write("Go to slide: ")
                index_str = ''
                while key in '0123456789':
                    bt.write(key)
                    index_str += key
                    key, mx, my = bt.getkey()

                if key == 'Enter':
                    index = max(0, min(len(slides)-1, int(index_str)-1))
            elif key == '/':
                bt.move(1, bt.height)
                with bt.attributes("bold"):
                    bt.write("Go to slide: ")

                search = ''
                key, mx, my = bt.getkey()
                while key not in ('Ctrl-c', 'Enter'):
                    if key == 'Backspace':
                        if search:
                            search = search[:-1]
                            bt.write('\b \b')
                    else:
                        search += key
                        bt.write(key)
                    key, mx, my = bt.getkey()

                if key == 'Enter':
                    for offset in range(len(slides)):
                        if search.lower() in slides[(index + 1 + offset) % len(slides)].text.lower():
                            index = (index + 1 + offset) % len(slides)
                            break
            elif key == 'n': # Next search result
                for offset in range(len(slides)):
                    if search.lower() in slides[(index + 1 + offset) % len(slides)].text.lower():
                        index = (index + 1 + offset) % len(slides)
                        break
            elif key == 'p': # Previous search result
                for offset in range(len(slides)):
                    if search.lower() in slides[(index - 1 - offset + 2*len(slides)) % len(slides)].text.lower():
                        index = (index - 1 - offset + 2*len(slides)) % len(slides)
                        break

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
                    slides += [Slide(filename, slide.strip()) for slide in re.split(r'(?m)^\-{3,}$', text)]
                else:
                    extension = filename.rpartition(".")[2] if "." in filename else ""
                    slides += [Slide(filename, f"```{extension}\n{text.strip()}\n```")]
        except FileNotFoundError:
            print(f"File not found: {filename}")
            sys.exit(1)
    present(slides)
