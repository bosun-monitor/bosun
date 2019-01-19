package expr

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"time"

	"bosun.org/models"
	"bosun.org/opentsdb"
)

// Result contains a single result and is generally contained within a Results Object.
type Result struct {
	// a list of sub computations for the expression. Collecting computations is not always enabled.
	models.Computations
	// The embedded Value which has a Value() method to get the actual Value, and Type() method to get the type
	Value
	// the tags for the result
	Group opentsdb.TagSet
}

// ResultSet contains the results of an expression operation or a expression function.
// It will also be the type returned from any completed evaluation of a complete expression.
// In addition it contains properties about how those results should behave in with certain Union
// operations.
//
// Each Result in the Results property should be of the same type. It is up to functions in the expression
// language to ensure the Results are a set with no conflicting entries and that all entries are of the same type.
type ResultSet struct {
	Results ResultSlice
	// If true, ungrouped joins from this set will be ignored.
	IgnoreUnjoined bool
	// If true, ungrouped joins from the other set will be ignored.
	IgnoreOtherUnjoined bool
	// If non nil, will set any NaN value to it when the nv() function is used.
	NaNValue *float64
}

// ResultSlice is a slice of Result Pointers.
type ResultSlice []*Result

// Value is the interface that all valid types in the expression language must
// fullfill
type Value interface {
	Type() models.FuncType // used to identify the type of the Value
	Value() interface{}    // the actual value
}

// Number is the expression type that should be the value type for all numbers
// in a ResultSet that is a numberSet
type Number float64

// Type returns the type representation so it fullfills the Value interface.
func (n Number) Type() models.FuncType { return models.TypeNumberSet }

// Value returns the value of the number and exists so it fullfills the Value interface.
func (n Number) Value() interface{} { return n }

// MarshalJSON allows the value of the number to be reprented in JSON while also
// allowing for NaN and InF values to be represented.
func (n Number) MarshalJSON() ([]byte, error) { return marshalFloat(float64(n)) }

// Scalar is the expression type that represents a single untagged number.
type Scalar float64

// Type returns the type representation so it fullfills the Value interface.
func (s Scalar) Type() models.FuncType { return models.TypeScalar }

// Value returns the value of the Scalar and exists so it fullfills the Value interface.
func (s Scalar) Value() interface{} { return s }

// MarshalJSON allows the value of the Scalar to be reprented in JSON while also
// allowing for NaN and InF values to be represented.
func (s Scalar) MarshalJSON() ([]byte, error) { return marshalFloat(float64(s)) }

// String is the expression type that represents a string.
type String string

// Type returns the type representation so it fullfills the Value interface.
func (s String) Type() models.FuncType { return models.TypeString }

// Value returns the value of the string and exists so it fullfills the Value interface.
func (s String) Value() interface{} { return s }

// NumberExpr represents a sub number expression in the expression language which is used with map().
type NumberExpr Expr

// Type returns the type representation so it fullfills the Value interface.
func (s NumberExpr) Type() models.FuncType { return models.TypeNumberExpr }

// Value returns the value of the NumberExpr and exists so it fullfills the Value interface.
func (s NumberExpr) Value() interface{} { return s }

// Series is the standard form within bosun to represent timeseries data.
type Series map[time.Time]float64

// Type returns the type representation of the series so it fullfills the Value interface.
func (s Series) Type() models.FuncType { return models.TypeSeriesSet }

// Value returns the value of the Series and exists so it fullfills the Value interface.
func (s Series) Value() interface{} { return s }

// MarshalJSON returns the Series object in JSON.
func (s Series) MarshalJSON() ([]byte, error) {
	r := make(map[string]interface{}, len(s))
	for k, v := range s {
		r[fmt.Sprint(k.Unix())] = Scalar(v)
	}
	return json.Marshal(r)
}

// Equal returns if series s is equal to series b.
func (s Series) Equal(b Series) bool {
	return reflect.DeepEqual(s, b)
}

// Info is a generic object in the expression language which is only used to return
// interative information to the user.
type Info []interface{}

// Type returns the type representation so it fullfills the Value interface.
func (i Info) Type() models.FuncType { return models.TypeInfo }

// Value returns the value of the Info object and exists so it fullfills the Value interface.
func (i Info) Value() interface{} { return i }

// Table is a return type that lines up with Grafana Tables. It can be viewed in the expression
// editor but is primarily meant for integration with Grafana. This type is not used for Alerting.
type Table struct {
	Columns []string
	Rows    [][]interface{}
}

// Type returns the type representation of the Table so it fullfills the Value interface.
func (t Table) Type() models.FuncType { return models.TypeTable }

// Value returns the value of the Series and exists so it fullfills the Value interface.
func (t Table) Value() interface{} { return t }

// SortableSeries is an alternative datastructure for timeseries data,
// which stores points in a time-ordered fashion instead of a map.
// see discussion at https://github.com/bosun-monitor/bosun/pull/699
type SortableSeries []SortablePoint

// SortablePoint in a member for Sortable Series.
type SortablePoint struct {
	T time.Time
	V float64
}

func (s SortableSeries) Len() int           { return len(s) }
func (s SortableSeries) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s SortableSeries) Less(i, j int) bool { return s[i].T.Before(s[j].T) }

// NewSortedSeries takes a Series and returns it as a Sorted SortableSeries
func NewSortedSeries(dps Series) SortableSeries {
	series := make(SortableSeries, 0, len(dps))
	for t, v := range dps {
		series = append(series, SortablePoint{t, v})
	}
	sort.Sort(series)
	return series
}

// Equal inspects if two ResultSets have the same content.
// An error will return explaing why they are not equal if they are not equal.
func (r *ResultSet) Equal(b *ResultSet) (bool, error) {
	if len(r.Results) != len(b.Results) {
		return false, fmt.Errorf("unequal number of results: length a: %v, length b: %v", len(r.Results), len(b.Results))
	}
	if r.IgnoreUnjoined != b.IgnoreUnjoined {
		return false, fmt.Errorf("ignoreUnjoined flag does not match a: %v, b: %v", r.IgnoreUnjoined, b.IgnoreUnjoined)
	}
	if r.IgnoreOtherUnjoined != b.IgnoreOtherUnjoined {
		return false, fmt.Errorf("ignoreUnjoined flag does not match a: %v, b: %v", r.IgnoreOtherUnjoined, b.IgnoreOtherUnjoined)
	}
	if r.NaNValue != b.NaNValue {
		return false, fmt.Errorf("NaNValue does not match a: %v, b: %v", r.NaNValue, b.NaNValue)
	}
	sortedA := ResultSliceByGroup(r.Results)
	sort.Sort(sortedA)
	sortedB := ResultSliceByGroup(b.Results)
	sort.Sort(sortedB)
	for i, result := range sortedA {
		for ic, computation := range result.Computations {
			if computation != sortedB[i].Computations[ic] {
				return false, fmt.Errorf("mismatched computation a: %v, b: %v", computation, sortedB[ic])
			}
		}
		if !result.Group.Equal(sortedB[i].Group) {
			return false, fmt.Errorf("mismatched groups a: %v, b: %v", result.Group, sortedB[i].Group)
		}
		switch t := result.Value.(type) {
		case Number, Scalar, String:
			if result.Value != sortedB[i].Value {
				return false, fmt.Errorf("values do not match a: %v, b: %v", result.Value, sortedB[i].Value)
			}
		case Series:
			if !t.Equal(sortedB[i].Value.(Series)) {
				return false, fmt.Errorf("mismatched series in result (Group: %s) a: %v, b: %v", result.Group, t, sortedB[i].Value.(Series))
			}
		default:
			panic(fmt.Sprintf("can't compare results with type %T", t))
		}

	}
	return true, nil
}

// ResultSliceByGroup allows a ResultSlice to be sorted by Group (a.k.a. Tags).
type ResultSliceByGroup ResultSlice

// ResultSliceByValue allows a ResultSlice to be sorted by value.
type ResultSliceByValue ResultSlice

// DescByValue sorts a ResultSlice in Descending order by value.
func (r ResultSlice) DescByValue() ResultSlice {
	for _, v := range r {
		if _, ok := v.Value.(Number); !ok {
			return r
		}
	}
	c := r[:]
	sort.Sort(sort.Reverse(ResultSliceByValue(c)))
	return c
}

// Filter returns a slice with only the results that have a tagset that conforms to the given key/value pair restrictions
func (r ResultSlice) Filter(filter opentsdb.TagSet) ResultSlice {
	output := make(ResultSlice, 0, len(r))
	for _, res := range r {
		if res.Group.Compatible(filter) {
			output = append(output, res)
		}
	}
	return output
}

func (r ResultSliceByValue) Len() int           { return len(r) }
func (r ResultSliceByValue) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r ResultSliceByValue) Less(i, j int) bool { return r[i].Value.(Number) < r[j].Value.(Number) }

func (r ResultSliceByGroup) Len() int           { return len(r) }
func (r ResultSliceByGroup) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r ResultSliceByGroup) Less(i, j int) bool { return r[i].Group.String() < r[j].Group.String() }

func marshalFloat(n float64) ([]byte, error) {
	if math.IsNaN(n) {
		return json.Marshal("NaN")
	} else if math.IsInf(n, 1) {
		return json.Marshal("+Inf")
	} else if math.IsInf(n, -1) {
		return json.Marshal("-Inf")
	}
	return json.Marshal(n)
}
