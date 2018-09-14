package main

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRankRₙ(t *testing.T) {
	tests := []struct {
		name     string
		r        Rank
		expected float64
	}{{
		"should be 1",
		0,
		1,
	}, {
		"should be 10",
		400,
		10,
	}, {
		"should be 100",
		800,
		100,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, test.r.rₙ())
		})
	}
}

func TestRankEₙ(t *testing.T) {
	tests := []struct {
		name     string
		r        Rank
		m        float64
		expected float64
	}{{
		"should be 1",
		0,
		0,
		1,
	}, {
		"should also be 1",
		1000,
		0,
		1,
	}, {
		"should be a draw",
		1000,
		Rank(1000).rₙ(),
		0.5,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, test.r.eₙ(test.m))
		})
	}
}

func TestRankWon(t *testing.T) {
	tests := []struct {
		name     string
		r        Rank
		m        Rank
		expected Rank
	}{{
		"should be 1016",
		1000,
		1000,
		1016,
	}, {
		"should be 1032",
		1000,
		2000,
		1032,
	}, {
		"should be 1112",
		1100,
		1000,
		1112,
	}, {
		"should be 2001",
		2000,
		1000,
		2001,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, Rank(math.Ceil(float64(test.r.won(test.m)))))
		})
	}
}

func TestRankLost(t *testing.T) {
	tests := []struct {
		name     string
		r        Rank
		m        Rank
		expected Rank
	}{{
		"should be 984",
		1000,
		1000,
		984,
	}, {
		"should be 100",
		1000,
		2000,
		1000,
	}, {
		"should be 1080",
		1100,
		1000,
		1080,
	}, {
		"should be 1969",
		2000,
		1000,
		1969,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, Rank(math.Ceil(float64(test.r.lost(test.m)))))
		})
	}
}
