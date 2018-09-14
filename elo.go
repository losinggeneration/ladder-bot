package main

import "math"

const deviation = 400
const k = 32

type Rank float64

func (r Rank) rₙ() float64 {
	return math.Pow(10, math.Abs(float64(r))/deviation)
}

func (r Rank) eₙ(m float64) float64 {
	n := r.rₙ()
	return n / (n + m)
}

func (r Rank) won(m Rank) Rank {
	// r + k * (1 - E)
	return Rank(float64(r) + k*(1-r.eₙ(m.rₙ())))
}

func (r Rank) lost(m Rank) Rank {
	// r + k * (0 - E)
	return Rank(float64(r) + k*(0-r.eₙ(m.rₙ())))
}
