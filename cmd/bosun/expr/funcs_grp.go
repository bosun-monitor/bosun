package expr

import (
	"fmt"
	"strings"
	"time"

	"bosun.org/cmd/bosun/expr/doc"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"github.com/MiniProfiler/go/miniprofiler"
)

var groupFuncs = parse.FuncMap{
	"addtags": {
		Args:   addTagsDoc.Arguments.TypeSlice(),
		Return: addTagsDoc.Return,
		Tags:   tagRename,
		F:      AddTags,
		Doc:    addTagsDoc,
	},
	"remove": {
		Args:   removeDoc.Arguments.TypeSlice(),
		Return: removeDoc.Return,
		Tags:   tagRemove,
		F:      Remove,
		Doc:    removeDoc,
	},
	"rename": {
		Args:   renameDoc.Arguments.TypeSlice(),
		Return: renameDoc.Return,
		Tags:   tagRename,
		F:      Rename,
		Doc:    renameDoc,
	},
	"t": {
		Args:   tDoc.Arguments.TypeSlice(),
		Return: tDoc.Return,
		Tags:   tagTranspose,
		F:      Transpose,
		Doc:    tDoc,
	},
	"ungroup": {
		Args:   []models.FuncType{models.TypeNumberSet},
		Return: models.TypeScalar,
		F:      Ungroup,
	},
}

var addTagsDoc = &doc.Func{
	Name:    "addtags",
	Summary: "addtags adds the tags specified in t to each series in s. This function will error if one of the tag keys specified in t is already in s.",
	Arguments: doc.Arguments{
		sSeriesSetArg,
		doc.Arg{
			Name: "t",
			Desc: "tags to add in the format of <code>tagK=tagV,tagK=tagV</code>.",
			Type: models.TypeString,
		},
	},
	Return: models.TypeSeriesSet,
}

func AddTags(e *State, T miniprofiler.Timer, series *Results, s string) (*Results, error) {
	if s == "" {
		return series, nil
	}
	tagSetToAdd, err := opentsdb.ParseTags(s)
	if err != nil {
		return nil, err
	}
	for tagKey, tagValue := range tagSetToAdd {
		for _, res := range series.Results {
			if res.Group == nil {
				res.Group = make(opentsdb.TagSet)
			}
			if _, ok := res.Group[tagKey]; ok {
				return nil, fmt.Errorf("%s key already in group", tagKey)
			}
			res.Group[tagKey] = tagValue
		}
	}
	return series, nil
}

func tagRemove(args []parse.Node) (parse.Tags, error) {
	tags, err := tagFirst(args)
	if err != nil {
		return nil, err
	}
	key := args[1].(*parse.StringNode).Text
	delete(tags, key)
	return tags, nil
}

var removeDoc = &doc.Func{
	Name:    "remove",
	Summary: "remove removes the tag key specified in t for each series in s. This function will error if removing the tag key would result in duplicate items in set s or if the tag in t does not exist in s.",
	Arguments: doc.Arguments{
		sSeriesSetArg,
		doc.Arg{
			Name: "t",
			Desc: "tagkey to remove from the set.",
			Type: models.TypeString,
		},
	},
	Return: models.TypeSeriesSet,
}

func Remove(e *State, T miniprofiler.Timer, seriesSet *Results, tagKey string) (*Results, error) {
	seen := make(map[string]bool)
	for _, r := range seriesSet.Results {
		if _, ok := r.Group[tagKey]; ok {
			delete(r.Group, tagKey)
			if _, ok := seen[r.Group.String()]; ok {
				return seriesSet, fmt.Errorf("duplicate group would result from removing tag key: %v", tagKey)
			}
			seen[r.Group.String()] = true
		} else {
			return seriesSet, fmt.Errorf("tag key %v not found in result", tagKey)
		}
	}
	return seriesSet, nil
}

func tagRename(args []parse.Node) (parse.Tags, error) {
	tags, err := tagFirst(args)
	if err != nil {
		return nil, err
	}
	for _, section := range strings.Split(args[1].(*parse.StringNode).Text, ",") {
		kv := strings.Split(section, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("error passing groups")
		}
		for oldTagKey := range tags {
			if kv[0] == oldTagKey {
				if _, ok := tags[kv[1]]; ok {
					return nil, fmt.Errorf("%s already in group", kv[1])
				}
				delete(tags, kv[0])
				tags[kv[1]] = struct{}{}
			}
		}
	}
	return tags, nil
}

var renameDoc = &doc.Func{
	Name:    "rename",
	Summary: "rename renames tag keys for each series in s based the the tags specificed in t. If the new tag key already exists the function will error. This can be useful for combining results from separate queries that have similar tagsets with different tag keys.",
	Arguments: doc.Arguments{
		sSeriesSetArg,
		doc.Arg{
			Name: "t",
			Desc: "tag keys to rename in the format of <code>tagK1=newTagKey,tagK2=newTagKey2</code>. These are processed in the order listed so tags can be swapped.",
			Type: models.TypeString,
		},
	},
	Return: models.TypeSeriesSet,
	// TODO: Add an example of tag "swapping" since this is a bit hard to describe.
}

func Rename(e *State, T miniprofiler.Timer, series *Results, s string) (*Results, error) {
	for _, section := range strings.Split(s, ",") {
		kv := strings.Split(section, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("error passing groups")
		}
		oldKey, newKey := kv[0], kv[1]
		for _, res := range series.Results {
			for tag, v := range res.Group {
				if oldKey == tag {
					if _, ok := res.Group[newKey]; ok {
						return nil, fmt.Errorf("%s already in group", newKey)
					}
					delete(res.Group, oldKey)
					res.Group[newKey] = v
				}

			}
		}
	}
	return series, nil
}

func tagTranspose(args []parse.Node) (parse.Tags, error) {
	tags := make(parse.Tags)
	sp := strings.Split(args[1].(*parse.StringNode).Text, ",")
	if sp[0] != "" {
		for _, t := range sp {
			tags[t] = struct{}{}
		}
	}
	if atags, err := args[0].Tags(); err != nil {
		return nil, err
	} else if !tags.Subset(atags) {
		return nil, fmt.Errorf("transpose tags (%v) must be a subset of first argument's tags (%v)", tags, atags)
	}
	return tags, nil
}

var tDoc = &doc.Func{
	Name:    "t",
	Summary: "TODO ... something better than what exists, it is confusing as all hell.",
	Arguments: doc.Arguments{
		nNumberSetArg,
		doc.Arg{
			Name: "tagKeys",
			Desc: "a csv of tag keys that will be the keys of the resulting seriesSet. The keys must be part of n. This can also be an empty string.",
			Type: models.TypeString,
		},
	},
	Return:       models.TypeSeriesSet,
	ExtendedInfo: doc.HTMLString(tExtendedInfo),
}

func Transpose(e *State, T miniprofiler.Timer, d *Results, gp string) (*Results, error) {
	gps := strings.Split(gp, ",")
	m := make(map[string]*Result)
	for _, v := range d.Results {
		ts := make(opentsdb.TagSet)
		for k, v := range v.Group {
			for _, b := range gps {
				if k == b {
					ts[k] = v
				}
			}
		}
		if _, ok := m[ts.String()]; !ok {
			m[ts.String()] = &Result{
				Group: ts,
				Value: make(Series),
			}
		}
		switch t := v.Value.(type) {
		case Number:
			r := m[ts.String()]
			i := int64(len(r.Value.(Series)))
			r.Value.(Series)[time.Unix(i, 0).UTC()] = float64(t)
			r.Computations = append(r.Computations, v.Computations...)
		default:
			panic(fmt.Errorf("expr: expected a number"))
		}
	}
	var r Results
	for _, res := range m {
		r.Results = append(r.Results, res)
	}
	return &r, nil
}

var tExtendedInfo = `<p>How transpose works conceptually</p>

<p>Transpose Grouped results into a Single Result</p>

<p>Before Transpose (Value Type is NumberSet):</p>
<table class="table">
    <thead>
        <tr>
            <th>Group</th>
            <th>Value</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>{host=web01}</td>
            <td>1</td>
        </tr>
        <tr>
            <td>{host=web02}</td>
            <td>7</td>
        </tr>
        <tr>
            <td>{host=web03}</td>
            <td>4</td>
        </tr>
    </tbody>
</table>

<p>After Transpose (Value Type is SeriesSet):</p>
<table class="table">
    <thead>
        <tr>
            <th>Group</th>
            <th>Value</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>{}</td>
            <td>1,7,4</td>
        </tr>
    </tbody>
</table>
<p>Transpose Groups results into Multiple Results:</p>
<p>Before Transpose by host (Value Type is NumberSet)</p>
<table class="table">
    <thead>
        <tr>
            <th>Group</th>
            <th>Value</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>{host=web01,disk=c}</td>
            <td>1</td>
        </tr>
        <tr>
            <td>{host=web01,disc=d}</td>
            <td>3</td>
        </tr>
        <tr>
            <td>{host=web02,disc=c}</td>
            <td>4</td>
        </tr>
    </tbody>
</table>
<p>After Transpose by “host” (Value type is SeriesSet)</p>
<table class="table">
    <thead>
        <tr>
            <th>Group</th>
            <th>Value</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>{host=web01}</td>
            <td>1,3</td>
        </tr>
        <tr>
            <td>{host=web02}</td>
            <td>4</td>
        </tr>
    </tbody>
</table>
`
