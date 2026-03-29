package mood

import (
	"fmt"

	"github.com/charmbracelet/harmonica"
	"github.com/danielfry/spotui/visual"
)

const doneThreshold = 0.99

type Transition struct {
	From   Mood
	To     Mood
	Active bool
	pos    float64
	vel    float64
	spring harmonica.Spring
}

func NewTransition(from, to Mood) *Transition {
	return &Transition{
		From:   from,
		To:     to,
		Active: true,
		pos:    0.0,
		vel:    0.0,
		spring: harmonica.NewSpring(harmonica.FPS(30), 5.0, 1.0),
	}
}

func (t *Transition) Tick() {
	if !t.Active {
		return
	}
	t.pos, t.vel = t.spring.Update(t.pos, t.vel, 1.0)
	if t.pos >= doneThreshold {
		t.pos = 1.0
		t.Active = false
	}
}

func (t *Transition) Progress() float64 { return t.pos }
func (t *Transition) Done() bool        { return !t.Active }

func (t *Transition) Current() Mood {
	p := t.pos
	return Mood{
		Name:        fmt.Sprintf("%s → %s", t.From.Name, t.To.Name),
		Primary:     visual.LerpColor(t.From.Primary, t.To.Primary, p),
		Secondary:   visual.LerpColor(t.From.Secondary, t.To.Secondary, p),
		Background:  visual.LerpColor(t.From.Background, t.To.Background, p),
		PatternChar: t.patternAt(p),
		Energy:      visual.LerpFloat(t.From.Energy, t.To.Energy, p),
	}
}

func (t *Transition) patternAt(p float64) string {
	if p < 0.5 {
		return t.From.PatternChar
	}
	return t.To.PatternChar
}
