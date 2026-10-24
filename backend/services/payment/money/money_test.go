package money_test

import (
	"errors"
	"math"
	"testing"

	"goshop/services/payment/money"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The classic float trap: 0.1+0.2 != 0.3. In integer xu the sum is exact.
func TestAdd_NoBinaryFloatDrift(t *testing.T) {
	a := money.FromMinorUnits(10) // 0.10
	b := money.FromMinorUnits(20) // 0.20

	sum, err := a.Add(b)

	require.NoError(t, err)
	assert.Equal(t, money.FromMinorUnits(30), sum)
	assert.Equal(t, "0.30", sum.String())
}

// Accumulating many small amounts must land exactly on the ceiling, which is
// what makes a cumulative-refund check trustworthy.
func TestAdd_CumulativeRefundHitsLimitExactly(t *testing.T) {
	var refunded money.Amount
	for i := 0; i < 3; i++ {
		var err error
		refunded, err = refunded.Add(money.FromMinorUnits(3333)) // 33.33 each
		require.NoError(t, err)
	}

	assert.Equal(t, money.FromMinorUnits(9999), refunded)
	// 9999 xu is exactly 0.01 dong below 100.00; float accumulation of
	// 33.33 * 3 frequently misses this boundary, integer addition never does.
	assert.True(t, money.FromMinorUnits(10000).Exceeds(refunded))
	assert.False(t, money.FromMinorUnits(9999).Exceeds(money.FromMinorUnits(9999)))
}

func TestAdd_OverflowIsReported(t *testing.T) {
	_, err := money.FromMinorUnits(1 << 62).Add(money.FromMinorUnits(1 << 62))

	assert.ErrorContains(t, err, "amount overflow")
}

func TestParseDecimal(t *testing.T) {
	cases := map[string]int64{
		"150000":             15_000_000,
		"150000.00":          15_000_000,
		"1500.5":             150_050,
		"-9000.10":           -900_010,
		"+12.34":             1_234,
		"0":                  0,
		"0.01":               1,
		"999999999999999.99": 99_999_999_999_999_999,
	}
	for raw, want := range cases {
		got, err := money.ParseDecimal(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, want, got.MinorUnits(), raw)
	}
}

func TestParseDecimal_Rejects(t *testing.T) {
	for _, raw := range []string{"", "abc", "1.2.3", "1e5", "--1"} {
		_, err := money.ParseDecimal(raw)
		assert.Error(t, err, raw)
	}
	_, err := money.ParseDecimal("10.005")
	assert.True(t, errors.Is(err, money.ErrFractional))
	// One minor unit past the NUMERIC(15,2) capacity must be rejected.
	_, err = money.ParseDecimal("1000000000000000")
	assert.ErrorContains(t, err, "supported magnitude")
}

// A float cannot hold 10.005; converting it must fail instead of silently
// biasing the balance by a fraction of a xu.
func TestFromFloat_ExactValuesOnly(t *testing.T) {
	got, err := money.FromFloat(150000) // order totals arrive as whole dong
	require.NoError(t, err)
	assert.Equal(t, int64(15_000_000), got.MinorUnits())

	got, err = money.FromFloat(150000.5)
	require.NoError(t, err)
	assert.Equal(t, int64(15_000_050), got.MinorUnits())

	_, err = money.FromFloat(10.005)
	assert.True(t, errors.Is(err, money.ErrFractional))

	// Runtime float arithmetic (not compile-time constant folding) produces
	// the 0.1+0.2 classic; it must not sneak in as a silently-adjusted 0.30.
	a := float64(1) / 10
	b := float64(2) / 10
	_, err = money.FromFloat(a + b)
	assert.True(t, errors.Is(err, money.ErrFractional))
}

func TestFromFloat_RejectsNonFinite(t *testing.T) {
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := money.FromFloat(v)
		assert.Error(t, err)
	}
}

// MoMo's API and the order totals speak whole dong; the xu representation
// must convert losslessly in both directions.
func TestWholeDong_RoundTrip(t *testing.T) {
	amount, err := money.ParseDecimal("150000.00")
	require.NoError(t, err)

	dong, err := amount.WholeDong()
	require.NoError(t, err)
	assert.Equal(t, int64(150000), dong)

	back, err := money.FromWholeDong(dong)
	require.NoError(t, err)
	assert.Equal(t, amount, back)

	fractional, err := money.ParseDecimal("150000.50")
	require.NoError(t, err)
	_, err = fractional.WholeDong()
	assert.ErrorContains(t, err, "not a whole number of dong")
}

func TestFromWholeDong_OverflowIsReported(t *testing.T) {
	_, err := money.FromWholeDong(1 << 62)
	assert.ErrorContains(t, err, "amount overflow")
}

func TestString(t *testing.T) {
	assert.Equal(t, "150000.00", money.FromMinorUnits(15_000_000).String())
	assert.Equal(t, "-9.10", money.FromMinorUnits(-910).String())
	assert.Equal(t, "0.05", money.FromMinorUnits(5).String())
}
