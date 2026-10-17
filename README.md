# pilldoc

A terminal Dr. Mario. Pills and viruses are drawn in braille, so a cell is
as small as a character and the bottle scales to the terminal height.

## Install

```
go install github.com/demian0311/pilldoc@latest
```

Then run `pilldoc`. It needs a terminal at least 34 columns by 18 rows and a
font with braille glyphs.

## Keys

| Key | Action |
|---|---|
| ◀ ▶ | Move (level on the menu) |
| ▲ or z | Rotate (speed on the menu) |
| ▼ | Tap to nudge down, hold to drop fast |
| space | Slam |
| p | Pause and resume |
| enter | Start, next level, back to the menu |
| q | Quit |

Switching away from the terminal pauses the game and dims the screen.
Switching back counts down and resumes.
