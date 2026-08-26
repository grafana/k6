package faker

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/brianvoe/gofakeit/v6"
)

func init() {
	gofakeit.AddFuncLookup("creditcardstring", gofakeit.Info{
		Display:     "Credit Card Number Formatted",
		Category:    "payment",
		Description: "Unique numerical identifier on a credit card used for making electronic payments and transactions",
		Example:     "4136-4599-4899-5369",
		Output:      "string",
		Params:      nil,
		Generate:    creditcardstring,
	})

	gofakeit.AddFuncLookup("creditcardexpmonth", gofakeit.Info{
		Display:     "Credit Card Exp Month",
		Category:    "payment",
		Description: "Month of the date when a credit card becomes invalid and cannot be used for transactions",
		Example:     "11",
		Output:      "string",
		Params:      nil,
		Generate:    creditcardexpmonth,
	})

	gofakeit.AddFuncLookup("creditcardexpyear", gofakeit.Info{
		Display:     "Credit Card Exp Year",
		Category:    "payment",
		Description: "Year of the date when a credit card becomes invalid and cannot be used for transactions",
		Example:     "28",
		Output:      "string",
		Params:      nil,
		Generate:    creditcardexpyear,
	})
}

var testcards = []string{ //nolint:gochecknoglobals
	"4111-1111-1111-1111",
	"4242-4242-4242-4242",
	"4000-0566-5566-5556",
	"5555-5555-5555-4444",
	"5200-8282-8282-8210",
	"5105-1051-0510-5100",
}

func creditcardstring(r *rand.Rand, _ *gofakeit.MapParams, _ *gofakeit.Info) (any, error) {
	return testcards[r.Intn(len(testcards))], nil
}

func creditcardexpmonth(r *rand.Rand, _ *gofakeit.MapParams, _ *gofakeit.Info) (any, error) {
	const months = 12

	month := strconv.Itoa(1 + r.Intn(months))
	if len(month) == 1 {
		month = "0" + month
	}

	return month, nil
}

func creditcardexpyear(r *rand.Rand, _ *gofakeit.MapParams, _ *gofakeit.Info) (any, error) {
	const Y2K = 2000

	current := time.Now().Year() - Y2K

	const maxYear = 10

	return strconv.Itoa(current + 1 + r.Intn(maxYear)), nil
}
