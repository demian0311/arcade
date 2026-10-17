# arcade

Terminal games drawn in braille, one binary each, on a shared renderer.
A cell is as small as a character, the board scales to the terminal height,
and an animated field fills the rest of the screen.

| Game | Install | What it is |
|---|---|---|
| pilldoc | `go install github.com/demian0311/arcade/cmd/pilldoc@latest` | Dr. Mario |
| tetris | `go install github.com/demian0311/arcade/cmd/tetris@latest` | Tetris |

Every game needs a font with braille glyphs. Switching away from the
terminal pauses the game and dims the screen; switching back counts down and
resumes.

## pilldoc

A terminal at least 34 columns by 18 rows.

| Key | Action |
|---|---|
| ◀ ▶ | Move (level on the menu) |
| ▲ or z | Rotate (speed on the menu) |
| ▼ | Tap to nudge down, hold to drop fast |
| space | Slam |
| p | Pause and resume |
| enter | Start, next level, back to the menu |
| q | Quit |

## tetris

A terminal at least 39 columns by 22 rows. Seven-bag pieces, SRS rotation
with wall kicks, a level every ten lines.

| Key | Action |
|---|---|
| ◀ ▶ | Move (level on the menu) |
| ▲ or x | Rotate clockwise |
| z | Rotate counter-clockwise |
| ▼ | Tap to nudge down, hold to drop fast |
| space | Slam |
| p | Pause and resume |
| enter | Start, back to the menu |
| q | Quit |

## Layout

- `cmd/<game>` is one game and its binary.
- `internal/canvas` is the braille dot grid and rounded tiles.
- `internal/backdrop` is the animated field behind a game.
- `internal/layout` places blocks over the backdrop, holds the styles and the dot-font title.
- `internal/session` holds ticks, the focus pause and countdown, and tap-versus-hold key detection.
