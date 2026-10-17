package backdrop

import (
	"testing"
	"time"
)

func TestCoversTerminalAndFlares(t *testing.T) {
	b := &Backdrop{}
	b.Resize(60, 20)
	b.Step(time.Second)
	if len(b.lines) != 20 {
		t.Fatalf("%d lines", len(b.lines))
	}
	dots := 0
	for _, l := range b.lines {
		if n := len([]rune(l)); n != 60 {
			t.Fatalf("line width %d", n)
		}
		for _, r := range l {
			if r != ' ' {
				dots++
			}
		}
	}
	if dots == 0 {
		t.Fatal("backdrop is blank")
	}
	b.Flare("#ff6b6b")
	if b.Style().GetForeground() == (&Backdrop{t: b.t}).Style().GetForeground() {
		t.Fatal("flare did not change the tint")
	}
}
