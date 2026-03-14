package glicko

import (
	"math"
	"testing"
)

// TestGlickman uses the worked example from Mark Glickman's paper
// "Example of the Glicko-2 system" to verify the implementation.
// Player: r=1500, RD=200, σ=0.06
// Opponents: (1400,30,win), (1550,100,loss), (1700,300,loss)
// Expected after one rating period: r'≈1464.06, RD'≈151.52, σ'≈0.05999
func TestGlickman(t *testing.T) {
	results := []Result{
		{OpponentRating: 1400, OpponentRD: 30, Score: 1},
		{OpponentRating: 1550, OpponentRD: 100, Score: 0},
		{OpponentRating: 1700, OpponentRD: 300, Score: 0},
	}

	r, rd, sigma := Update(1500, 200, 0.06, results)

	tol := 1.0
	if math.Abs(r-1464.06) > tol {
		t.Errorf("rating: got %.4f, want ~1464.06 (tol %.1f)", r, tol)
	}
	if math.Abs(rd-151.52) > tol {
		t.Errorf("RD: got %.4f, want ~151.52 (tol %.1f)", rd, tol)
	}
	if math.Abs(sigma-0.05999) > 0.001 {
		t.Errorf("volatility: got %.6f, want ~0.05999", sigma)
	}
}

// TestNoGames ensures that a player who does not compete has their RD widened.
func TestNoGames(t *testing.T) {
	_, rd, sigma := Update(1500, 200, 0.06, nil)
	// phi* = sqrt(phi^2 + sigma^2)  ->  slightly larger than 200
	if rd <= 200 {
		t.Errorf("RD should increase when no games played, got %.4f", rd)
	}
	// Volatility unchanged when no games.
	if math.Abs(sigma-0.06) > 0.0001 {
		t.Errorf("volatility should stay 0.06 when no games played, got %.6f", sigma)
	}
}
