// Package timedate provides an analysis.Analyzer that reports a numeric date
// written as a string in a format that does not sort.
//
// The layout documentation in package time names RFC 3339 as the format to
// prefer for new protocols, and calls the month-first order of the reference
// time "a regrettable historic error". RFC 3339 writes the year first and pads
// every field, so the text of two dates sorts the way the dates do. "3/1/2024"
// reads as March 1 in one country and January 3 in another, and its text sorts
// by month. "2024-03-01" has one reading and sorts by year.
//
// The analyzer reads a string literal whose whole text is a numeric date, and
// reports the ones whose text does not sort. A layout is not a date, and a
// literal in the layout or parsed-value argument of a time call is left alone.
// Every other literal is read as a date, including one a call takes from a
// variable and one a method value or an interface method receives, and a
// literal that parses into the year 2006 is left alone as a layout.
//
// No fix is offered: rewriting the literal changes the text this program
// writes or reads, and the format the other system expects is not visible
// here. The README catalogs every form the analyzer reads and every one it
// leaves alone.
package timedate

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"time"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "timedate",
	Doc:  "reports a numeric date written in a format that does not sort",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		// The walk visits a call before the literals it holds, so an
		// argument is marked by the time its literal arrives.
		skipped := make(map[token.Pos]struct{})
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				layout, value := uses.LayoutArgs(pass, n)
				for _, arg := range []ast.Expr{layout, value} {
					if lit, ok := ast.Unparen(arg).(*ast.BasicLit); ok {
						skipped[lit.Pos()] = struct{}{}
					}
				}
			case *ast.BasicLit:
				if n.Kind != token.STRING {
					return true
				}
				if _, ok := skipped[n.Pos()]; ok {
					return true
				}
				text, err := strconv.Unquote(n.Value)
				if err != nil {
					return true
				}
				layout := layoutFor(text)
				if layout == "" {
					return true
				}
				pass.Reportf(
					n.Pos(),
					"text does not sort as a date; the layout %s does",
					layout,
				)
			}
			return true
		})
	}
	return nil, nil
}

// clocks are the forms that can follow a date; the first is none. Each is a
// padded 24-hour clock, so a sorted date stays sorted with one after it.
var clocks = []string{"", " 15:04", " 15:04:05", "T15:04", "T15:04:05"}

// isoDate is the layout the analyzer names for a date.
const isoDate = "2006-01-02"

// sortedDates lists the date forms that sort: the year first, then the month,
// then the day, each padded to a fixed width.
var sortedDates = []string{isoDate, "2006/01/02"}

// unsortedDates lists the date forms that do not: the year last, or a
// year-first date whose fields are not padded. The month comes first in some
// and the day in others, since a day above 12 reads only as a day.
var unsortedDates = []string{
	"1/2/2006",
	"2/1/2006",
	"1-2-2006",
	"2-1-2006",
	"2006-1-2",
	"2006/1/2",
}

// layoutFor returns the layout that value should be written in, or "" when
// value is not a date written in a form that does not sort.
func layoutFor(value string) string {
	for _, date := range sortedDates {
		for _, clock := range clocks {
			if printsAs(date+clock, value) {
				return ""
			}
		}
	}
	// time.Parse accepts a padded field where the layout does not pad it,
	// as "1/01/2024", so this loop cannot require a round trip.
	for _, date := range unsortedDates {
		for _, clock := range clocks {
			t, err := time.Parse(date+clock, value)
			if err != nil {
				continue
			}
			// A layout writes its year as 2006, so a date in that year
			// is the text a layout would use.
			if t.Year() == 2006 {
				return ""
			}
			// time.Parse reads a fractional second that the layout does
			// not name, and a layout without one would drop it.
			_, frac, ok := fraction(value)
			if ok && frac == "" {
				return "" // no layout names this fraction
			}
			return isoDate + clock + frac
		}
	}
	return ""
}

// printsAs reports whether a string is what the layout prints, with a
// fractional second or without one. time.Parse reads a single digit into a
// padded hour, so the text is compared as printed, and a run of spaces reads
// as one space to it as well.
func printsAs(layout, value string) bool {
	whole := value
	if cut, _, ok := fraction(value); ok {
		whole = cut
	}
	t, err := time.Parse(layout, whole)
	return err == nil && collapse(t.Format(layout)) == collapse(whole)
}

// collapse returns a string with every run of whitespace collapsed to one
// space.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// fraction returns a value without the fractional second it ends with, a
// layout for that fraction, and whether it ends with one. The layout keeps the
// separator and the digit count, since a nines marker drops trailing zeros,
// and is empty when the fraction is longer than a nanosecond field.
func fraction(value string) (whole, layout string, ok bool) {
	i := strings.LastIndexAny(value, ".,")
	if i < 0 {
		return "", "", false
	}
	digits := value[i+1:]
	if digits == "" || strings.Trim(digits, "0123456789") != "" {
		return "", "", false
	}
	// Format names at most the nine digits of a nanosecond field.
	if len(digits) > 9 {
		return value[:i], "", true
	}
	return value[:i], value[i:i+1] + strings.Repeat("0", len(digits)), true
}
