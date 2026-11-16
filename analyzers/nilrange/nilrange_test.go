package nilrange

import (
	"embed"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

func TestConstraint(t *testing.T) {
	var (
		ints  = types.NewSlice(types.Typ[types.Int])
		bytes = types.NewSlice(types.Typ[types.Byte])
		strs  = types.NewSlice(types.Typ[types.String])
		maps  = types.NewMap(types.Typ[types.String], types.Typ[types.Int])
		first = types.NewNamed(
			types.NewTypeName(0, nil, "first", nil),
			ints, nil,
		)
		other = types.NewNamed(
			types.NewTypeName(0, nil, "other", nil),
			ints, nil,
		)
		cases = []struct {
			name       string
			constraint types.Type
			want       bool
		}{{
			"slice",
			union(tilde(ints)),
			true,
		}, {
			"map",
			union(tilde(maps)),
			true,
		}, {
			"same core",
			union(named(first), named(other)),
			true,
		}, {
			"different slices",
			union(tilde(ints), tilde(bytes)),
			false,
		}, {
			"slice and map",
			union(tilde(ints), tilde(maps)),
			false,
		}, {
			"intersection",
			intersection(
				union(tilde(ints), tilde(strs)),
				union(tilde(ints)),
			),
			true,
		}, {
			"exact and approximate",
			intersection(union(named(first)), union(tilde(ints))),
			true,
		}, {
			"approximate and exact",
			intersection(union(tilde(ints)), union(named(first))),
			true,
		}, {
			"same exact",
			intersection(union(named(first)), union(named(first))),
			true,
		}, {
			"different exact",
			intersection(union(named(first)), union(named(other))),
			false,
		}}
	)
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := constraint(tt.constraint); got != tt.want {
				t.Errorf("constraint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func tilde(t types.Type) *types.Term { return types.NewTerm(true, t) }

func named(t types.Type) *types.Term { return types.NewTerm(false, t) }

func union(terms ...*types.Term) types.Type {
	iface := types.NewInterfaceType(nil, []types.Type{types.NewUnion(terms)})
	iface.Complete()
	return iface
}

func intersection(a, b types.Type) types.Type {
	iface := types.NewInterfaceType(nil, []types.Type{a, b})
	iface.Complete()
	return iface
}
