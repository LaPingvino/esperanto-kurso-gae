// Package glicko implements the Glicko-2 rating algorithm.
// Reference: Mark Glickman, "Example of the Glicko-2 system" (2012).
package glicko

import "math"

const (
	// tau is the system constant constraining volatility changes.
	tau = 0.5
	// epsilon is the convergence tolerance for the Illinois algorithm.
	epsilon = 0.000001
	// scale converts between Glicko-1 and Glicko-2 scale.
	scale = 173.7178
)

// Result represents a single game outcome against one opponent.
type Result struct {
	OpponentRating float64
	OpponentRD     float64
	Score          float64 // 1=win, 0.5=draw, 0=loss
}

// g computes the g function from the paper: g(φ) = 1 / sqrt(1 + 3φ²/π²)
func g(phi float64) float64 {
	return 1.0 / math.Sqrt(1.0+3.0*phi*phi/(math.Pi*math.Pi))
}

// e computes the expected score: E(μ, μj, φj) = 1 / (1 + exp(-g(φj)(μ-μj)))
func e(mu, muJ, phiJ float64) float64 {
	return 1.0 / (1.0 + math.Exp(-g(phiJ)*(mu-muJ)))
}

// Update calculates new Glicko-2 rating parameters given a slice of results in one rating period.
// If results is empty the rating deviation widens (player did not compete).
func Update(rating, rd, volatility float64, results []Result) (newRating, newRD, newVolatility float64) {
	// Step 2: convert to Glicko-2 scale.
	mu := (rating - 1500.0) / scale
	phi := rd / scale

	// Step 3: if no games played, just widen phi.
	if len(results) == 0 {
		phiStar := math.Sqrt(phi*phi + volatility*volatility)
		return rating, phiStar * scale, volatility
	}

	// Pre-compute opponent values on Glicko-2 scale.
	type opp struct {
		muJ  float64
		phiJ float64
		gJ   float64
		eJ   float64
		sJ   float64
	}
	opps := make([]opp, len(results))
	for i, r := range results {
		muJ := (r.OpponentRating - 1500.0) / scale
		phiJ := r.OpponentRD / scale
		gJ := g(phiJ)
		eJ := e(mu, muJ, phiJ)
		opps[i] = opp{muJ, phiJ, gJ, eJ, r.Score}
	}

	// Step 4: compute v (estimated variance of player's rating based on outcomes).
	var v float64
	for _, o := range opps {
		v += o.gJ * o.gJ * o.eJ * (1 - o.eJ)
	}
	v = 1.0 / v

	// Step 5a: compute delta (estimated improvement).
	var deltaSum float64
	for _, o := range opps {
		deltaSum += o.gJ * (o.sJ - o.eJ)
	}
	delta := v * deltaSum

	// Step 5b: update volatility via Illinois algorithm.
	// Define f(x) = exp(x)*(delta²-phi²-v-exp(x)) / (2*(phi²+v+exp(x))²) - (x-ln(sigma²))/tau²
	sigma := volatility
	lnSigma2 := math.Log(sigma * sigma)
	phi2 := phi * phi
	delta2 := delta * delta

	f := func(x float64) float64 {
		ex := math.Exp(x)
		numer := ex * (delta2 - phi2 - v - ex)
		denom := 2.0 * (phi2 + v + ex) * (phi2 + v + ex)
		return numer/denom - (x-lnSigma2)/(tau*tau)
	}

	// Initialise a and b.
	a := lnSigma2
	var b float64
	if delta2 > phi2+v {
		b = math.Log(delta2 - phi2 - v)
	} else {
		k := 1.0
		for f(lnSigma2-k*tau) < 0 {
			k++
		}
		b = lnSigma2 - k*tau
	}

	fA := f(a)
	fB := f(b)

	for math.Abs(b-a) > epsilon {
		c := a + (a-b)*fA/(fB-fA)
		fC := f(c)
		if fC*fB <= 0 {
			a = b
			fA = fB
		} else {
			fA = fA / 2.0
		}
		b = c
		fB = fC
	}

	sigmaPrime := math.Exp(a / 2.0)

	// Step 6: update phi*.
	phiStar := math.Sqrt(phi2 + sigmaPrime*sigmaPrime)

	// Step 7: update phi' and mu'.
	phiPrime := 1.0 / math.Sqrt(1.0/phiStar/phiStar+1.0/v)

	muPrime := mu + phiPrime*phiPrime*deltaSum

	// Step 8: convert back to Glicko-1 scale.
	newRating = scale*muPrime + 1500.0
	newRD = scale * phiPrime
	newVolatility = sigmaPrime
	return
}
