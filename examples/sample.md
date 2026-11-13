#!./slides

# Slides Demo

*by Bruce Hill*

![Tomo file](../tomo/examples/learnxiny.tm)

**slides** is a terminal slide presenter.

You can see the slide number in the bottom left corner.

Slides are separated by three or more dashes on a line: `---`

To advance to the next slide, press **Right** or **Space** or **L**.

---------------------------------------------------------

# Slide Navigation

You can go back a slide by pressing **Left** or **Backspace** or **H**.

You can go to the *first* slide by pressing **Home** or **G**.

You can go to the *last* slide by pressing **End** or **Shift-G**.

You can also type in a slide number and hit **Enter** to go to that slide.

---------------------------------------------------------

# Step by Step

A slide can be revealed a step at a time,

<!-- pause -->

by putting `<!-- pause -->` on a line by itself:

<!-- pause -->

- Each step
<!-- pause -->
- reveals a little more

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

| Table  | Alignment |
|--------|----------:|
| Tables | *work*    |
| too    | `!`       |

---------------------------------------------------------

# Demos

A ```` ```demo ```` block is a terminal with the code typed in.
Click it (or press **Tab** then **Enter**) to type in it, and
press **Ctrl-]** to return to the slides.

```demo
seq 1000 | less
```

Another demo:

```demo
for i in `seq 5`; do
    echo "$i"
    sleep 1
done
echo "DONE"
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

You can scroll up/down with **Up**/**Down** (or **K**/**J**), the **mouse wheel**, or **Ctrl-U**/**Ctrl-D**.

![Tale of Genji](genji.txt)

![The source for this presentation](sample.md)

---------------------------------------------------------

# Images

![](thumbsup.jpg)

Supported!

---------------------------------------------------------

# Inspecting the Source

You can inspect a slide's code at any point using
the **Backtick** key `` ` ``

---------------------------------------------------------

# Get the Code

You can get the source code [at the github repo](https://github.com/bruce-hill/slides)

(links are `[link text](url)`, press **Enter** to launch)

---------------------------------------------------------

# The End

Thanks for your time!
