#!./slides.py

# Slides Demo

*by Bruce Hill*

**slides** is a terminal slide presenter.

You can see the slide number in the bottom left corner.

Slides are separated by three or more dashes on a line: `---`

To advance to the next slide, press **Right** or **Space** or **J**.

---------------------------------------------------------

# Slide Navigation

You can go back a slide by pressing **Left** or **Backspace** or **K**.

You can go to the *first* slide by pressing **Home** or **H**.

You can go to the *last* slide by pressing **End** or **L**.

You can also type in a slide number and hit **Enter** to go to that slide.

---------------------------------------------------------

# Basic Features

This is **italic**, *bold*, and `inline code`
The paragraph continues on line two.

- A list
- With items

1. and
2. some
3. numbered ones

> Block quote
> that spans multiple lines

Here's a shell example:

```run
ls --color=auto
```

---------------------------------------------------------

# Demos

For runnable demos, you can use ```` ```demo ````
and press **Enter** to run the demo

```demo
seq 1000 | less
```

---------------------------------------------------------

# Syntax Highlighting

Some code:

```Python
def sing_bottles(n:int):
    for i in reversed(range(n)):
        if i > 1:
            print(f"{i} bottles of beer on the wall,")
            print(f"{i} bottles of beer!")
            print("Take one down, pass it around...")
        elif i == 1:
            print("One bottle of beer on the wall,")
            print("One bottle of beer!")
            print("Take it down, pass it around...")
        else:
            print("No more bottles of beer on the wall!")
            print("Go to the store, buy some more...")
            sing_bottles(n)
```

```markdown
This is some *markdown* text with **asterisks**.
```

```
Some basic emojis: 🌎💨🔥
They do render correctly with appropriate box size
(But your mileage may vary, depending on how your
terminal renders emojis and what font you use)
```

---------------------------------------------------------

# Embedding files

You can embed files with the syntax `![title](./file.txt)`:

You can scroll up/down with **Up**/**Down**, the **mouse wheel**, or **Ctrl-U**/**Ctrl-D**.

![Tale of Genji](genji.txt)

![The source for this presentation](sample.md)

---------------------------------------------------------

# Images

![sxiv -f -sf](thumbsup.jpg)
```demo
seq 1000 | less
```

Supported! (kinda)

---------------------------------------------------------

# Re-running Code

You can re-run code by pressing **Ctrl-R** or **R**.

It is currently:

```run
date
```

Some random numbers:

```run
openssl rand -hex 16
```

```run
hexdump -n 128 /dev/urandom
```

---------------------------------------------------------

# Inspecting the Source

You can inspect a slide's code at any point using
the **Backtick** key `` ` ``

```run
hexdump -n 128 /dev/urandom
```

---------------------------------------------------------

# Get the Code

You can get the source code [at the github repo](https://github.com/bruce-hill/slides)

(links are `[link text](url)`, press **Enter** to launch)

---------------------------------------------------------

# The End

Thanks for your time!
