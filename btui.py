import ctypes
import enum
import functools
import time
from contextlib import contextmanager

__all__ = ['open', 'TextAttr', 'ClearType', 'CursorType', 'BTUIMode']

# Load the shared library into c types.
libbtui = ctypes.CDLL("./libbtui.so")

class FILE(ctypes.Structure):
    pass

class BTUI_struct(ctypes.Structure):
    _fields_ = [
        ('in', ctypes.POINTER(FILE)),
        ('out', ctypes.POINTER(FILE)),
        ('width', ctypes.c_int),
        ('height', ctypes.c_int),
        ('size_changed', ctypes.c_int),
    ]

libbtui.btui_create.restype = ctypes.POINTER(BTUI_struct)

attr = lambda name: ctypes.c_longlong.in_dll(libbtui, name).value
attr_t = ctypes.c_longlong

class TextAttr(enum.IntEnum):
    NORMAL                 = attr('BTUI_NORMAL')
    BOLD                   = attr('BTUI_BOLD')
    FAINT                  = attr('BTUI_FAINT')
    DIM                    = attr('BTUI_FAINT')
    ITALIC                 = attr('BTUI_ITALIC')
    UNDERLINE              = attr('BTUI_UNDERLINE')
    BLINK_SLOW             = attr('BTUI_BLINK_SLOW')
    BLINK_FAST             = attr('BTUI_BLINK_FAST')
    REVERSE                = attr('BTUI_REVERSE')
    CONCEAL                = attr('BTUI_CONCEAL')
    STRIKETHROUGH          = attr('BTUI_STRIKETHROUGH')
    FRAKTUR                = attr('BTUI_FRAKTUR')
    DOUBLE_UNDERLINE       = attr('BTUI_DOUBLE_UNDERLINE')
    NO_BOLD_OR_FAINT       = attr('BTUI_NO_BOLD_OR_FAINT')
    NO_ITALIC_OR_FRAKTUR   = attr('BTUI_NO_ITALIC_OR_FRAKTUR')
    NO_UNDERLINE           = attr('BTUI_NO_UNDERLINE')
    NO_BLINK               = attr('BTUI_NO_BLINK')
    NO_REVERSE             = attr('BTUI_NO_REVERSE')
    NO_CONCEAL             = attr('BTUI_NO_CONCEAL')
    NO_STRIKETHROUGH       = attr('BTUI_NO_STRIKETHROUGH')
    FG_BLACK               = attr('BTUI_FG_BLACK')
    FG_RED                 = attr('BTUI_FG_RED')
    FG_GREEN               = attr('BTUI_FG_GREEN')
    FG_YELLOW              = attr('BTUI_FG_YELLOW')
    FG_BLUE                = attr('BTUI_FG_BLUE')
    FG_MAGENTA             = attr('BTUI_FG_MAGENTA')
    FG_CYAN                = attr('BTUI_FG_CYAN')
    FG_WHITE               = attr('BTUI_FG_WHITE')
    FG_NORMAL              = attr('BTUI_FG_NORMAL')
    BG_BLACK               = attr('BTUI_BG_BLACK')
    BG_RED                 = attr('BTUI_BG_RED')
    BG_GREEN               = attr('BTUI_BG_GREEN')
    BG_YELLOW              = attr('BTUI_BG_YELLOW')
    BG_BLUE                = attr('BTUI_BG_BLUE')
    BG_MAGENTA             = attr('BTUI_BG_MAGENTA')
    BG_CYAN                = attr('BTUI_BG_CYAN')
    BG_WHITE               = attr('BTUI_BG_WHITE')
    BG_NORMAL              = attr('BTUI_BG_NORMAL')
    FRAMED                 = attr('BTUI_FRAMED')
    ENCIRCLED              = attr('BTUI_ENCIRCLED')
    OVERLINED              = attr('BTUI_OVERLINED')
    NO_FRAMED_OR_ENCIRCLED = attr('BTUI_NO_FRAMED_OR_ENCIRCLED')
    NO_OVERLINED           = attr('BTUI_NO_OVERLINED')

BTUI_INVERSE_ATTRS = {
    TextAttr.NORMAL          : TextAttr.NORMAL,
    TextAttr.BOLD            : TextAttr.NO_BOLD_OR_FAINT,
    TextAttr.FAINT           : TextAttr.NO_BOLD_OR_FAINT,
    TextAttr.DIM             : TextAttr.NO_BOLD_OR_FAINT,
    TextAttr.ITALIC          : TextAttr.NO_ITALIC_OR_FRAKTUR,
    TextAttr.UNDERLINE       : TextAttr.NO_UNDERLINE,
    TextAttr.BLINK_SLOW      : TextAttr.NO_BLINK,
    TextAttr.BLINK_FAST      : TextAttr.NO_BLINK,
    TextAttr.REVERSE         : TextAttr.NO_REVERSE,
    TextAttr.CONCEAL         : TextAttr.NO_CONCEAL,
    TextAttr.STRIKETHROUGH   : TextAttr.NO_STRIKETHROUGH,
    TextAttr.FRAKTUR         : TextAttr.NO_ITALIC_OR_FRAKTUR,
    TextAttr.DOUBLE_UNDERLINE: TextAttr.NO_UNDERLINE,
    TextAttr.FG_BLACK        : TextAttr.FG_NORMAL,
    TextAttr.FG_RED          : TextAttr.FG_NORMAL,
    TextAttr.FG_GREEN        : TextAttr.FG_NORMAL,
    TextAttr.FG_YELLOW       : TextAttr.FG_NORMAL,
    TextAttr.FG_BLUE         : TextAttr.FG_NORMAL,
    TextAttr.FG_MAGENTA      : TextAttr.FG_NORMAL,
    TextAttr.FG_CYAN         : TextAttr.FG_NORMAL,
    TextAttr.FG_WHITE        : TextAttr.FG_NORMAL,
    TextAttr.FG_NORMAL       : TextAttr.FG_NORMAL,
    TextAttr.BG_BLACK        : TextAttr.BG_NORMAL,
    TextAttr.BG_RED          : TextAttr.BG_NORMAL,
    TextAttr.BG_GREEN        : TextAttr.BG_NORMAL,
    TextAttr.BG_YELLOW       : TextAttr.BG_NORMAL,
    TextAttr.BG_BLUE         : TextAttr.BG_NORMAL,
    TextAttr.BG_MAGENTA      : TextAttr.BG_NORMAL,
    TextAttr.BG_CYAN         : TextAttr.BG_NORMAL,
    TextAttr.BG_WHITE        : TextAttr.BG_NORMAL,
    TextAttr.BG_NORMAL       : TextAttr.BG_NORMAL,
    TextAttr.FRAMED          : TextAttr.NO_FRAMED_OR_ENCIRCLED,
    TextAttr.ENCIRCLED       : TextAttr.NO_FRAMED_OR_ENCIRCLED,
    TextAttr.OVERLINED       : TextAttr.NO_OVERLINED,
}

class BTUIMode(enum.IntEnum):
    UNINITIALIZED = 0
    NORMAL        = 1
    TUI           = 2

class CursorType(enum.IntEnum):
    DEFAULT            = 0
    BLINKING_BLOCK     = 1
    BLOCK              = 2
    BLINKING_UNDERLINE = 3
    UNDERLINE          = 4
    BLINKING_BAR       = 5
    BAR                = 6

class ClearType(enum.IntEnum):
    SCREEN = 0
    ABOVE  = 1
    BELOW  = 2
    LINE   = 3
    LEFT   = 4
    RIGHT  = 5

class BTUI:
    _autoflush = True

    @contextmanager
    def attributes(self, *attrs):
        self.set_attributes(*attrs)
        try: yield
        finally: self.unset_attributes(*attrs)

    @contextmanager
    def bg(self, r, g, b):
        self.set_bg(r, g, b)
        try: yield
        finally: self.set_attributes("bg_normal")

    def clear(self, clear_type=ClearType.SCREEN):
        assert self._btui
        if isinstance(clear_type, str):
            clear_type = ClearType[clear_type.upper()]
        libbtui.btui_clear(self._btui, clear_type)
        if self._autoflush:
            libbtui.btui_flush(self._btui)

    def disable(self):
        libbtui.btui_disable(self._btui)

    @contextmanager
    def disabled(self):
        self.disable()
        try: yield self
        finally: self.enable()

    def draw_shadow(self, x, y, w, h):
        assert self._btui
        libbtui.btui_draw_shadow(self._btui, int(x), int(y), int(w), int(h))
        libbtui.btui_flush(self._btui)

    def enable(self, mode=BTUIMode.TUI):
        if isinstance(mode, str):
            mode = BTUIMode[mode.upper()]
        self._btui = libbtui.btui_create(mode)

    def set_mode(self, mode):
        if isinstance(mode, str):
            mode = BTUIMode[mode.upper()]
        self._btui = libbtui.set_mode(self._btui, mode)

    @contextmanager
    def fg(self, r, g, b):
        self.set_fg(r, g, b)
        try: yield
        finally: self.set_attributes("fg_normal")

    def fill_box(self, x, y, w, h):
        assert self._btui
        libbtui.btui_fill_box(self._btui, int(x), int(y), int(w), int(h))
        if self._autoflush:
            libbtui.btui_flush(self._btui)

    def flush(self):
        assert self._btui
        libbtui.btui_flush(self._btui)

    @contextmanager
    def buffered(self):
        assert self._btui
        prev_autoflush = self._autoflush
        self._autoflush = False
        yield
        self._autoflush = prev_autoflush
        self.flush()

    def getkey(self, timeout=None):
        assert self._btui
        timeout = -1 if timeout is None else int(timeout)
        mouse_x, mouse_y = ctypes.c_int(-1), ctypes.c_int(-1)
        key = libbtui.btui_getkey(self._btui, timeout,
                ctypes.byref(mouse_x), ctypes.byref(mouse_y))
        buf = ctypes.create_string_buffer(64)
        libbtui.btui_keyname(key, buf)
        if mouse_x.value == -1:
            return buf.value.decode('utf8'), None, None
        else:
            return buf.value.decode('utf8'), mouse_x.value, mouse_y.value

    @property
    def height(self):
        assert self._btui
        return self._btui.contents.height

    def move(self, x, y):
        assert self._btui
        libbtui.btui_move_cursor(self._btui, int(x), int(y))
        if self._autoflush:
            libbtui.btui_flush(self._btui)

    def set_cursor(self, cursor_type=CursorType.DEFAULT):
        assert self._btui
        if isinstance(cursor_type, str):
            cursor_type = CursorType[cursor_type.upper()]
        libbtui.btui_set_cursor(self._btui, cursor_type)
        if self._autoflush:
            libbtui.btui_flush(self._btui)

    def hide_cursor(self):
        assert self._btui
        libbtui.btui_hide_cursor(self._btui)
        if self._autoflush:
            libbtui.btui_flush(self._btui)

    def show_cursor(self):
        assert self._btui
        libbtui.btui_show_cursor(self._btui)
        if self._autoflush:
            libbtui.btui_flush(self._btui)

    def outline_box(self, x, y, w, h):
        assert self._btui
        libbtui.btui_draw_linebox(self._btui, int(x), int(y), int(w), int(h))
        if self._autoflush:
            libbtui.btui_flush(self._btui)

    def scroll(self, firstline, lastline=None, amount=None):
        assert self._btui
        if amount is None:
            amount = firstline
            firstline, lastline = 0, self.height-1
        libbtui.btui_scroll(self._btui, firstline, lastline, amount)
        if self._autoflush:
            libbtui.btui_flush(self._btui)

    def set_attributes(self, *attrs):
        assert self._btui
        attr_long = ctypes.c_longlong(0)
        for a in attrs:
            if isinstance(a, str):
                a = TextAttr[a.upper()]
            attr_long.value |= a
        libbtui.btui_set_attributes(self._btui, attr_long)

    def set_bg(self, r, g, b):
        assert self._btui
        libbtui.btui_set_bg(self._btui, int(r*255), int(g*255), int(b*255))

    def set_fg(self, r, g, b):
        assert self._btui
        libbtui.btui_set_fg(self._btui, int(r*255), int(g*255), int(b*255))

    def suspend(self):
        assert self._btui
        libbtui.btui_suspend(self._btui)

    def unset_attributes(self, *attrs):
        assert self._btui
        attr_long = ctypes.c_longlong(0)
        for a in attrs:
            if isinstance(a, str):
                a = TextAttr[a.upper()]
            attr_long.value |= BTUI_INVERSE_ATTRS[a]
        libbtui.btui_set_attributes(self._btui, attr_long)

    @property
    def width(self):
        assert self._btui
        return self._btui.contents.width

    def write(self, *args, sep=''):
        assert self._btui
        s = sep.join(args)
        self.write_bytes(bytes(s, 'utf8'))

    def write_bytes(self, b):
        assert self._btui
        libbtui.btui_puts(self._btui, b)
        if self._autoflush:
            libbtui.btui_flush(self._btui)

def delay(fn):
    @functools.wraps(fn)
    def wrapped(self, *a, **k):
        assert self._btui
        ret = fn(self, *a, **k)
        time.sleep(self.delay)
        libbtui.btui_show_cursor(self._btui)
        return ret
    return wrapped

class DebugBTUI(BTUI):
    delay = 0.05

for fn_name in ('clear', 'draw_shadow', 'fill_box', 'move', 'set_cursor', 'hide_cursor', 'show_cursor', 'outline_box',
                'scroll', 'set_attributes', 'set_bg', 'set_fg', 'unset_attributes', 'write_bytes'):
    setattr(DebugBTUI, fn_name, delay(getattr(BTUI, fn_name)))

_btui = None
@contextmanager
def open(*, debug=False, delay=0.05, mode=BTUIMode.TUI):
    global _btui
    if not _btui:
        if debug:
            _btui = DebugBTUI()
            _btui.delay = delay
        else:
            _btui = BTUI()
    _btui.enable(mode=mode)
    _btui.move(0, 0)
    try: yield _btui
    finally: _btui.disable()

