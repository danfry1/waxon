package app

import (
	"math"

	"github.com/danielfry/spotui/visual"
)

type Effects struct {
	Particles *visual.ParticleSystem
	GlowGrid  [][]string
	Breathing float64
}

func NewEffects(width, height int) Effects {
	return Effects{
		Particles: visual.NewParticleSystem(35, width, height),
	}
}

func (e *Effects) Resize(width, height int) {
	if e.Particles != nil {
		e.Particles.Resize(width, height)
	}
}

func (e *Effects) Tick(energy, beatPhase float64, primary, secondary, background string, artW, artH, screenW, screenH int) {
	if e.Particles != nil {
		e.Particles.Update(energy, primary, secondary)
	}

	if artW > 0 && artH > 0 && screenW > 0 && screenH > 0 {
		e.GlowGrid = visual.RenderGlow(screenW, screenH, artW, artH, primary, secondary, background, energy)
	}

	amplitude := energy * 0.03
	e.Breathing = amplitude * math.Sin(beatPhase*2*math.Pi)
}
