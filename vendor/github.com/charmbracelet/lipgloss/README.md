# Lip Gloss

<p>
    <a href="https://stuff.charm.sh/lipgloss/lipgloss-mascot-2k.png"><img width="340" alt="Lip Gloss title treatment" src="https://github.com/charmbracelet/lipgloss/assets/25087/147cadb1-4254-43ec-ae6b-8d6ca7b029a1"></a><br>
    <a href="https://github.com/charmbracelet/lipgloss/releases"><img src="https://img.shields.io/github/release/charmbracelet/lipgloss.svg" alt="Latest Release"></a>
    <a href="https://pkg.go.dev/github.com/charmbracelet/lipgloss?tab=doc"><img src="https://godoc.org/github.com/golang/gddo?status.svg" alt="GoDoc"></a>
    <a href="https://github.com/charmbracelet/lipgloss/actions"><img src="https://github.com/charmbracelet/lipgloss/workflows/build/badge.svg" alt="Build Status"></a>
    <a href="https://www.phorm.ai/query?projectId=a0e324b6-b706-4546-b951-6671ea60c13f"><img src="https://stuff.charm.sh/misc/phorm-badge.svg" alt="phorm.ai"></a>
</p>

Style definitions for nice terminal layouts. Built with TUIs in mind.

![Lip Gloss example](https://github.com/user-attachments/assets/7950b1c1-e0e3-427e-8e7d-6f7f6ad17ca7)

Lip Gloss takes an expressive, declarative approach to terminal rendering.
Users familiar with CSS will feel at home with Lip Gloss.

```go

import "github.com/charmbracelet/lipgloss"

var style = lipgloss.NewStyle().
    Bold(true).
    Foreground(lipgloss.Color("#FAFAFA")).
    Background(lipgloss.Color("#7D56F4")).
    PaddingTop(2).
    PaddingLeft(4).
    Width(22)

fmt.Println(style.Render("Hello, kitty"))
```

## Colors

Lip Gloss supports the following color profiles:

### ANSI 16 colors (4-bit)

```go
lipgloss.Color("5")  // magenta
lipgloss.Color("9")  // red
lipgloss.Color("12") // light blue
```

### ANSI 256 Colors (8-bit)

```go
lipgloss.Color("86")  // aqua
lipgloss.Color("201") // hot pink
lipgloss.Color("202") // orange
```

### True Color (16,777,216 colors; 24-bit)

```go
lipgloss.Color("#0000FF") // good ol' 100% blue
lipgloss.Color("#04B575") // a green
lipgloss.Color("#3C3C3C") // a dark gray
```

...as well as a 1-bit ASCII profile, which is black and white only.

The terminal's color profile will be automatically detected, and colors outside
the gamut of the current palette will be automatically coerced to their closest
available value.

### Adaptive Colors

You can also specify color options for light and dark backgrounds:

```go
lipgloss.AdaptiveColor{Light: "236", Dark: "248"}
```

The terminal's background color will automatically be detected and the
appropriate color will be chosen at runtime.

### Complete Colors

CompleteColor specifies exact values for True Color, ANSI256, and ANSI color
profiles.

```go
lipgloss.CompleteColor{TrueColor: "#0000FF", ANSI256: "86", ANSI: "5"}
```

Automatic color degradation will not be performed in this case and it will be
based on the color specified.

### Complete Adaptive Colors

You can use `CompleteColor` with `AdaptiveColor` to specify the exact values for
light and dark backgrounds without automatic color degradation.

```go
lipgloss.CompleteAdaptiveColor{
    Light: CompleteColor{TrueColor: "#d7ffae", ANSI256: "193", ANSI: "11"},
    Dark:  CompleteColor{TrueColor: "#d75fee", ANSI256: "163", ANSI: "5"},
}
```

## Inline Formatting

Lip Gloss supports the usual ANSI text formatting options:

```go
var style = lipgloss.NewStyle().
    Bold(true).
    Italic(true).
    Faint(true).
    Blink(true).
    Strikethrough(true).
    Underline(true).
    Reverse(true)
```

## Block-Level Formatting

Lip Gloss also supports rules for block-level formatting:

```go
// Padding
var style = lipgloss.NewStyle().
    PaddingTop(2).
    PaddingRight(4).
    PaddingBottom(2).
    PaddingLeft(4)

// Margins
var style = lipgloss.NewStyle().
    MarginTop(2).
    MarginRight(4).
    MarginBottom(2).
    MarginLeft(4)
```

There is also shorthand syntax for margins and padding, which follows the same
format as CSS:

```go
// 2 cells on all sides
lipgloss.NewStyle().Padding(2)

// 2 cells on the top and bottom, 4 cells on the left and right
lipgloss.NewStyle().Margin(2, 4)

// 1 cell on the top, 4 cells on the sides, 2 cells on the bottom
lipgloss.NewStyle().Padding(1, 4, 2)

// Clockwise, starting from the top: 2 cells on the top, 4 on the right, 3 on
// the bottom, and 1 on the left
lipgloss.NewStyle().Margin(2, 4, 3, 1)
```

## Aligning Text

You can align paragraphs of text to the left, right, or center.

```go
var style = lipgloss.NewStyle().
    Width(24).
    Align(lipgloss.Left).  // align it left
    Align(lipgloss.Right). // no wait, align it right
    Align(lipgloss.Center) // just kidding, align it in the center
```

## Width and Height

Setting a minimum width and height is simple and straightforward.

```go
var style = lipgloss.NewStyle().
    SetString("What’s for lunch?").
    Width(24).
    Height(32).
    Foreground(lipgloss.Color("63"))
```

## Borders

Adding borders is easy:

```go
// Add a purple, rectangular border
var style = lipgloss.NewStyle().
    BorderStyle(lipgloss.NormalBorder()).
    BorderForeground(lipgloss.Color("63"))

// Set a rounded, yellow-on-purple border to the top and left
var anotherStyle = lipgloss.NewStyle().
    BorderStyle(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("228")).
    BorderBackground(lipgloss.Color("63")).
    BorderTop(true).
    BorderLeft(true)

// Make your own border
var myCuteBorder = lipgloss.Border{
    Top:         "._.:*:",
    Bottom:      "._.:*:",
    Left:        "|*",
    Right:       "|*",
    TopLeft:     "*",
    TopRight:    "*",
    BottomLeft:  "*",
    BottomRight: "*",
}
```

There are also shorthand functions for defining borders, which follow a similar
pattern to the margin and padding shorthand functions.

```go
// Add a thick border to the top and bottom
lipgloss.NewStyle().
    Border(lipgloss.ThickBorder(), true, false)

// Add a double border to the top and left sides. Rules are set clockwise
// from top.
lipgloss.NewStyle().
    Border(lipgloss.DoubleBorder(), true, false, false, true)
```

For more on borders see [the docs][docs].

## Copying Styles

Just use assignment:

```go
style := lipgloss.NewStyle().Foreground(lipgloss.Color("219"))

copiedStyle := style // this is a true copy

wildStyle := style.Blink(true) // this is also true copy, with blink added

```

Since `Style` data structures contains only primitive types, assigning a style
to another effectively creates a new copy of the style without mutating the
original.

## Inheritance

Styles can inherit rules from other styles. When inheriting, only unset rules
on the receiver are inherited.

```go
var styleA = lipgloss.NewStyle().
    Foreground(lipgloss.Color("229")).
    Background(lipgloss.Color("63"))

// Only the background color will be inherited here, because the foreground
// color will have been already set:
var styleB = lipgloss.NewStyle().
    Foreground(lipgloss.Color("201")).
    Inherit(styleA)
```

## Unsetting Rules

All rules can be unset:

```go
var style = lipgloss.NewStyle().
    Bold(true).                        // make it bold
    UnsetBold().                       // jk don't make it bold
    Background(lipgloss.Color("227")). // yellow background
    UnsetBackground()                  // never mind
```

When a rule is unset, it won't be inherited or copied.

## Enforcing Rules

Sometimes, such as when developing a component, you want to make sure style
definitions respect their intended purpose in the UI. This is where `Inline`
and `MaxWidth`, and `MaxHeight` come in:

```go
// Force rendering onto a single line, ignoring margins, padding, and borders.
someStyle.Inline(true).Render("yadda yadda")

// Also limit rendering to five cells
someStyle.Inline(true).MaxWidth(5).Render("yadda yadda")

// Limit rendering to a 5x5 cell block
someStyle.MaxWidth(5).MaxHeight(5).Render("yadda yadda")
```

## Tabs

The tab character (`\t`) is rendered differently in different terminals (often
as 8 spaces, sometimes 4). Because of this inconsistency, Lip Gloss converts
tabs to 4 spaces at render time. This behavior can be changed on a per-style
basis, however:

```go
style := lipgloss.NewStyle() // tabs will render as 4 spaces, the default
style = style.TabWidth(2)    // render tabs as 2 spaces
style = style.TabWidth(0)    // remove tabs entirely
style = style.TabWidth(lipgloss.NoTabConversion) // leave tabs intact
```

## Rendering

Generally, you just call the `Render(string...)` method on a `lipgloss.Style`:

```go
style := lipgloss.NewStyle().Bold(true).SetString("Hello,")
fmt.Println(style.Render("kitty.")) // Hello, kitty.
fmt.Println(style.Render("puppy.")) // Hello, puppy.
```

But you could also use the Stringer interface:

```go
var style = lipgloss.NewStyle().SetString("你好，猫咪。").Bold(true)
fmt.Println(style) // 你好，猫咪。
```

### Custom Renderers

Custom renderers allow you to render to a specific outputs. This is
particularly important when you want to render to different outputs and
correctly detect the color profile and dark background status for each, such as
in a server-client situation.

```go
func myLittleHandler(sess ssh.Session) {
    // Create a renderer for the client.
    renderer := lipgloss.NewRenderer(sess)

    // Create a new style on the renderer.
    style := renderer.NewStyle().Background(lipgloss.AdaptiveColor{Light: "63", Dark: "228"})

    // Render. The color profile and dark background state will be correctly detected.
    io.WriteString(sess, style.Render("Heyyyyyyy"))
}
```

For an example on using a custom renderer over SSH with [Wish][wish] see the
[SSH example][ssh-example].

## Utilities

In addition to pure styling, Lip Gloss also ships with some utilities to help
assemble your layouts.

### Joining Paragraphs

Horizontally and vertically joining paragraphs is a cinch.

```go
// Horizontally join three paragraphs along their bottom edges
lipgloss.JoinHorizontal(lipgloss.Bottom, paragraphA, paragraphB, paragraphC)

// Vertically join two paragraphs along their center axes
lipgloss.JoinVertical(lipgloss.Center, paragraphA, paragraphB)

// Horizontally join three paragraphs, with the shorter ones aligning 20%
// from the top of the tallest
lipgloss.JoinHorizontal(0.2, paragraphA, paragraphB, paragraphC)
```

### Measuring Width and Height

Sometimes you’ll want to know the width and height of text blocks when building
your layouts.

```go
// Render a block of text.
var style = lipgloss.NewStyle().
    Width(40).
    Padding(2)
var block string = style.Render(someLongString)

// Get the actual, physical dimensions of the text block.
width := lipgloss.Width(block)
height := lipgloss.Height(block)

// Here's a shorthand function.
w, h := lipgloss.Size(block)
```

### Placing Text in Whitespace

Sometimes you’ll simply want to place a block of text in whitespace.

```go
// Center a paragraph horizontally in a space 80 cells wide. The height of
// the block returned will be as tall as the input paragraph.
block := lipgloss.PlaceHorizontal(80, lipgloss.Center, fancyStyledParagraph)

// Place a paragraph at the bottom of a space 30 cells tall. The width of
// the text block returned will be as wide as the input paragraph.
block := lipgloss.PlaceVertical(30, lipgloss.Bottom, fancyStyledParagraph)

// Place a paragraph in the bottom right corner of a 30x80 cell space.
block := lipgloss.Place(30, 80, lipgloss.Right, lipgloss.Bottom, fancyStyledParagraph)
```

You can also style the whitespace. For details, see [the docs][docs].

## Rendering Tables

Lip Gloss ships with a table rendering sub-package.

```go
import "github.com/charmbracelet/lipgloss/table"
```

Define some rows of data.

```go
rows := [][]string{
    {"Chinese", "您好", "你好"},
    {"Japanese", "こんにちは", "やあ"},
    {"Arabic", "أهلين", "أهلا"},
    {"Russian", "Здравствуйте", "Привет"},
    {"Spanish", "Hola", "¿Qué tal?"},
}
```

Use the table package to style and render the table.

```go
var (
    purple    = lipgloss.Color("99")
    gray      = lipgloss.Color("245")
    lightGray = lipgloss.Color("241")

    headerStyle  = lipgloss.NewStyle().Foreground(purple).Bold(true).Align(lipgloss.Center)
    cellStyle    = lipgloss.NewStyle().Padding(0, 1).Width(14)
    oddRowStyle  = cellStyle.Foreground(gray)
    evenRowStyle = cellStyle.Foreground(lightGray)
)

t := table.New().
    Border(lipgloss.NormalBorder()).
    BorderStyle(lipgloss.NewStyle().Foreground(purple)).
    StyleFunc(func(row, col int) lipgloss.Style {
        switch {
        case row == table.HeaderRow:
            return headerStyle
        case row%2 == 0:
            return evenRowStyle
        default:
            return oddRowStyle
        }
    }).
    Headers("LANGUAGE", "FORMAL", "INFORMAL").
    Rows(rows...)

// You can also add tables row-by-row
t.Row("English", "You look absolutely fabulous.", "How's it going?")
```

Print the table.

```go
fmt.Println(t)
```

![Table Example](https://github.com/charmbracelet/lipgloss/assets/42545625/6e4b70c4-f494-45da-a467-bdd27df30d5d)

> [!WARNING]
> Table `Rows` need to be declared before `Offset` otherwise it does nothing.

### Table Borders

There are helpers to generate tables in markdown or ASCII style:

#### Markdown Table

```go
table.New().Border(lipgloss.MarkdownBorder()).BorderTop(false).BorderBottom(false)
```

```
| LANGUAGE |    FORMAL    | INFORMAL  |
|----------|--------------|-----------|
| Chinese  | Nǐn hǎo      | Nǐ hǎo    |
| French   | Bonjour      | Salut     |
| Russian  | Zdravstvuyte | Privet    |
| Spanish  | Hola         | ¿Qué tal? |
```

#### ASCII Table

```go
table.New().Border(lipgloss.ASCIIBorder())
```

```
+----------+--------------+-----------+
| LANGUAGE |    FORMAL    | INFORMAL  |
+----------+--------------+-----------+
| Chinese  | Nǐn hǎo      | Nǐ hǎo    |
| French   | Bonjour      | Salut     |
| Russian  | Zdravstvuyte | Privet    |
| Spanish  | Hola         | ¿Qué tal? |
+----------+--------------+-----------+
```

For more on tables see [the docs](https://pkg.go.dev/github.com/charmbracelet/lipgloss?tab=doc) and [examples](https://github.com/charmbracelet/lipgloss/tree/master/examples/table).

## Rendering Lists

Lip Gloss ships with a list rendering sub-package.

```go
import "github.com/charmbracelet/lipgloss/list"
```

Define a new list.

```go
l := list.New("A", "B", "C")
```

Print the list.

```go
fmt.Println(l)

// • A
// • B
// • C
```

Lists have the ability to nest.

```go
l := list.New(
    "A", list.New("Artichoke"),
    "B", list.New("Baking Flour", "Bananas", "Barley", "Bean Sprouts"),
    "C", list.New("Cashew Apple", "Cashews", "Coconut Milk", "Curry Paste", "Currywurst"),
    "D", list.New("Dill", "Dragonfruit", "Dried Shrimp"),
    "E", list.New("Eggs"),
    "F", list.New("Fish Cake", "Furikake"),
    "J", list.New("Jicama"),
    "K", list.New("Kohlrabi"),
    "L", list.New("Leeks", "Lentils", "Licorice Root"),
)
```

Print the list.

```go
fmt.Println(l)
```

<p align="center">
<img width="600" alt="image" src="https://github.com/charmbracelet/lipgloss/assets/42545625/0dc9f440-0748-4151-a3b0-7dcf29dfcdb0">
</p>

Lists can be customized via their enumeration function as well as using
`lipgloss.Style`s.

```go
enumeratorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).MarginRight(1)
itemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).MarginRight(1)

l := list.New(
    "Glossier",
    "Claire’s Boutique",
    "Nyx",
    "Mac",
    "Milk",
    ).
    Enumerator(list.Roman).
    EnumeratorStyle(enumeratorStyle).
    ItemStyle(itemStyle)
```

Print the list.

<p align="center">
<img width="600" alt="List example" src="https://github.com/charmbracelet/lipgloss/assets/42545625/360494f1-57fb-4e13-bc19-0006efe01561">
</p>

In addition to the predefined enumerators (`Arabic`, `Alphabet`, `Roman`, `Bullet`, `Tree`),
you may also define your own custom enumerator:

```go
l := list.New("Duck", "Duck", "Duck", "Duck", "Goose", "Duck", "Duck")

func DuckDuckGooseEnumerator(l list.Items, i int) string {
    if l.At(i).Value() == "Goose" {
        return "Honk →"
    }
    return ""
}

l = l.Enumerator(DuckDuckGooseEnumerator)
```

Print the list:

<p align="center">
<img width="600" alt="image" src="https://github.com/charmbracelet/lipgloss/assets/42545625/157aaf30-140d-4948-9bb4-dfba46e5b87e">
</p>

If you need, you can also build lists incrementally:

```go
l := list.New()

for i := 0; i < repeat; i++ {
    l.Item("Lip Gloss")
}
```

## Rendering Trees

Lip Gloss ships with a tree rendering sub-package.

```go
import "github.com/charmbracelet/lipgloss/tree"
```

Define a new tree.

```go
t := tree.Root(".").
    Child("A", "B", "C")
```

Print the tree.

```go
fmt.Println(t)

// .
// ├── A
// ├── B
// └── C
```

Trees have the ability to nest.

```go
t := tree.Root(".").
    Child("macOS").
    Child(
        tree.New().
            Root("Linux").
            Child("NixOS").
            Child("Arch Linux (btw)").
            Child("Void Linux"),
        ).
    Child(
        tree.New().
            Root("BSD").
            Child("FreeBSD").
            Child("OpenBSD"),
    )
```

Print the tree.

```go
fmt.Println(t)
```

<p align="center">
<img width="663" alt="Tree Example (simple)" src="https://github.com/user-attachments/assets/5ef14eb8-a5d4-4f94-8834-e15d1e714f89">
</p>

Trees can be customized via their enumeration function as well as using
`lipgloss.Style`s.

```go
enumeratorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("63")).MarginRight(1)
rootStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("35"))
itemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

t := tree.
    Root("⁜ Makeup").
    Child(
        "Glossier",
        "Fenty Beauty",
        tree.New().Child(
            "Gloss Bomb Universal Lip Luminizer",
            "Hot Cheeks Velour Blushlighter",
        ),
        "Nyx",
        "Mac",
        "Milk",
    ).
    Enumerator(tree.RoundedEnumerator).
    EnumeratorStyle(enumeratorStyle).
    RootStyle(rootStyle).
    ItemStyle(itemStyle)
```

Print the tree.

<p align="center">
<img width="663" alt="Tree Example (makeup)" src="https://github.com/user-attachments/assets/06d12d87-744a-4c89-bd98-45de9094a97e">
</p>

The predefined enumerators for trees are `DefaultEnumerator` and `RoundedEnumerator`.

If you need, you can also build trees incrementally:

```go
t := tree.New()

for i := 0; i < repeat; i++ {
    t.Child("Lip Gloss")
}
```

---

## FAQ

<details>
<summary>
Why are things misaligning? Why are borders at the wrong widths?
</summary>
<p>This is most likely due to your locale and encoding, particularly with
regard to Chinese, Japanese, and Korean (for example, <code>zh_CN.UTF-8</code>
or <code>ja_JP.UTF-8</code>). The most direct way to fix this is to set
<code>RUNEWIDTH_EASTASIAN=0</code> in your environment.</p>

<p>For details see <a href="https://github.com/charmbracelet/lipgloss/issues/40">https://github.com/charmbracelet/lipgloss/issues/40.</a></p>
</details>

<details>
<summary>
Why isn't Lip Gloss displaying colors?
</summary>
<p>Lip Gloss automatically degrades colors to the best available option in the
given terminal, and if output's not a TTY it will remove color output entirely.
This is common when running tests, CI, or when piping output elsewhere.</p>

<p>If necessary, you can force a color profile in your tests with
<a href="https://pkg.go.dev/github.com/charmbracelet/lipgloss#SetColorProfile"><code>SetColorProfile</code></a>.</p>

```go
import (
    "github.com/charmbracelet/lipgloss"
    "github.com/muesli/termenv"
)

lipgloss.SetColorProfile(termenv.TrueColor)
```

_Note:_ this option limits the flexibility of your application and can cause
ANSI escape codes to be output in cases where that might not be desired. Take
careful note of your use case and environment before choosing to force a color
profile.

</details>

## What about [Bubble Tea][tea]?

Lip Gloss doesn’t replace Bubble Tea. Rather, it is an excellent Bubble Tea
companion. It was designed to make assembling terminal user interface views as
simple and fun as possible so that you can focus on building your application
instead of concerning yourself with low-level layout details.

In simple terms, you can use Lip Gloss to help build your Bubble Tea views.

[tea]: https://github.com/charmbracelet/tea

## Under the Hood

Lip Gloss is built on the excellent [Termenv][termenv] and [Reflow][reflow]
libraries which deal with color and ANSI-aware text operations, respectively.
For many use cases Termenv and Reflow will be sufficient for your needs.

[termenv]: https://github.com/muesli/termenv
[reflow]: https://github.com/muesli/reflow

## Rendering Markdown

For a more document-centric rendering solution with support for things like
lists, tables, and syntax-highlighted code have a look at [Glamour][glamour],
the stylesheet-based Markdown renderer.

[glamour]: https://github.com/charmbracelet/glamour

## Contributing

See [contributing][contribute].

[contribute]: https://github.com/charmbracelet/lipgloss/contribute

## Feedback

We’d love to hear your thoughts on this project. Feel free to drop us a note!

- [Twitter](https://twitter.com/charmcli)
- [The Fediverse](https://mastodon.social/@charmcli)
- [Discord](https://charm.sh/chat)

## License

[MIT](https://github.com/charmbracelet/lipgloss/raw/master/LICENSE)

---

Part of [Charm](https://charm.sh).

<a href="https://charm.sh/"><img alt="The Charm logo" src="https://stuff.charm.sh/charm-badge.jpg" width="400"></a>

Charm热爱开源 • Charm loves open source

[docs]: https://pkg.go.dev/github.com/charmbracelet/lipgloss?tab=doc
[wish]: https://github.com/charmbracelet/wish
[ssh-example]: examples/ssh
