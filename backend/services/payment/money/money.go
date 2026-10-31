// Package money holds exact monetary amounts for the payment service.
//
// An Amount is an integer number of minor units (VND xu: 1 dong = 100 xu, the
// unit VNPay itself charges in). Every financial comparison and accumulation —
// refund limits, webhook coverage checks, totals — happens in this integer
// domain, so binary floating point never decides a money question.
//
// Conversions from foreign representations (float64 wire fields, provider
// payloads, NUMERIC columns) happen once at the boundary and fail loudly when
// a value cannot be represented exactly: silently losing money is worse than
// refusing the operation.
package money

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// Scale is the number of minor digits used by every currency this service
// accepts (VND in the payment flow, and the two-decimal shape of the
// NUMERIC(15,2) ledger).
const Scale = 2

// Amount is an exact monetary value in minor units (xu).
type Amount int64

// WholeDongUpper is the largest whole dong magnitude the payments table can
// hold: NUMERIC(15,2) keeps 13 integer digits.
const WholeDongUpper int64 = 999_999_999_999_999

// MaxAmount bounds the magnitude the payments table can hold, in minor units:
// NUMERIC(15,2) keeps 13 integer digits plus 2 decimals.
const MaxAmount Amount = Amount(WholeDongUpper)*100 + 99

// ErrFractional reports a value carrying more fraction digits than the
// currency scale can represent. Truncating it would misstate a balance.
var ErrFractional = errors.New("amount has a fractional part")

var decimalPattern = regexp.MustCompile(`^([+-])?(\d+)(?:\.(\d+))?$`)

// FromMinorUnits wraps raw minor units.
func FromMinorUnits(units int64) Amount { return Amount(units) }

// Covers reports whether a >= b.
func (a Amount) Covers(b Amount) bool { return a >= b }

// MinorUnits returns the integer minor-unit value.
func (a Amount) MinorUnits() int64 { return int64(a) }

// Add returns a+b; it reports an overflow instead of wrapping into a wrong
// balance.
func (a Amount) Add(b Amount) (Amount, error) {
	sum := int64(a) + int64(b)
	if (b > 0 && sum < int64(a)) || (b < 0 && sum > int64(a)) {
		return 0, fmt.Errorf("amount overflow: %d + %d", a, b)
	}
	return Amount(sum), nil
}

// Exceeds reports whether a > b.
func (a Amount) Exceeds(b Amount) bool { return a > b }

// Neg returns -a.
func (a Amount) Neg() Amount { return -a }

// String returns the decimal representation, e.g. "-150000.50".
func (a Amount) String() string {
	sign := ""
	v := int64(a)
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}

// ParseDecimal parses a plain decimal string ("150000", "-9000.10") into minor
// units. More than two fraction digits yields ErrFractional.
func ParseDecimal(raw string) (Amount, error) {
	trimmed := strings.TrimSpace(raw)
	m := decimalPattern.FindStringSubmatch(trimmed)
	if m == nil {
		return 0, fmt.Errorf("invalid amount %q", raw)
	}
	fraction := m[3]
	if len(fraction) > Scale {
		return 0, fmt.Errorf("invalid amount %q: %w", raw, ErrFractional)
	}
	fraction += strings.Repeat("0", Scale-len(fraction))

	units, ok := new(big.Int).SetString(m[2]+fraction, 10)
	if !ok {
		return 0, fmt.Errorf("invalid amount %q", raw)
	}
	if !units.IsInt64() || units.Cmp(big.NewInt(int64(MaxAmount))) > 0 {
		return 0, fmt.Errorf("invalid amount %q: exceeds supported magnitude", raw)
	}

	value := units.Int64()
	if m[1] == "-" {
		value = -value
	}
	return Amount(value), nil
}

// FromFloat converts a float64 that crossed a wire boundary (legacy JSON
// fields, the order service's gRPC double) into exact minor units.
//
// It parses the float's shortest decimal representation, so 150000 stays
// 150000 and a value a float cannot hold exactly (10.005) is rejected rather
// than silently biasing a balance.
func FromFloat(value float64) (Amount, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("invalid amount %v", value)
	}
	return ParseDecimal(strconv.FormatFloat(value, 'f', -1, 64))
}

// ToFloat renders the amount for wire fields that still carry a double. This
// is a serialization step, never an input to a calculation.
func (a Amount) ToFloat() float64 { return float64(a) / 100 }

// WholeDong returns the amount as whole dong (MoMo speaks whole dong). A
// remainder is an error: rounding it away would misstate what a gateway
// charges.
func (a Amount) WholeDong() (int64, error) {
	if int64(a)%100 != 0 {
		return 0, fmt.Errorf("amount %s is not a whole number of dong", a)
	}
	return int64(a) / 100, nil
}

// FromWholeDong builds an Amount from whole dong (MoMo's integer payloads),
// guarding the multiplication against overflow.
func FromWholeDong(units int64) (Amount, error) {
	if units > WholeDongUpper || units < -WholeDongUpper {
		return 0, fmt.Errorf("amount overflow converting %d dong", units)
	}
	return Amount(units * 100), nil
}

// Numeric wraps the amount as the payments ledger's NUMERIC(15,2) shape with
// two fraction digits: value = minor units * 10^-2.
func (a Amount) Numeric() pgtype.Numeric {
	return pgtype.Numeric{
		Int:   big.NewInt(int64(a)),
		Exp:   -2,
		Valid: true,
	}
}

// NumericToAmount reads a NUMERIC column as an exact Amount. Sub-scale digits
// (beyond two fraction places) cannot round-trip and are reported instead of
// truncated.
func NumericToAmount(n pgtype.Numeric) (Amount, error) {
	if !n.Valid || n.Int == nil {
		return 0, errors.New("amount column is NULL or NaN")
	}
	if n.NaN {
		return 0, errors.New("amount column is NaN")
	}
	if n.Exp+Scale < 0 {
		return 0, fmt.Errorf("column value %s carries more than %d fraction digits", numericString(n), Scale)
	}

	value := new(big.Int).Set(n.Int)
	if shift := n.Exp + Scale; shift > 0 {
		value.Mul(value, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(shift)), nil))
	}
	if limit := new(big.Int).SetInt64(int64(MaxAmount)); value.Abs(value).Cmp(limit) > 0 {
		return 0, fmt.Errorf("column value %s exceeds supported magnitude", numericString(n))
	}
	return Amount(value.Int64()), nil
}

func numericString(n pgtype.Numeric) string {
	return fmt.Sprintf("%s*10^%d", n.Int, n.Exp)
}
