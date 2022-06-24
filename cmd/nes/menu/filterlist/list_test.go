package filterlist

import (
    "testing"
    "strings"
    "fmt"
)

type Data struct {
    Value string
    Other int
}

func (data *Data) Contains(s string) bool {
    return strings.Contains(data.Value, s)
}

func (data *Data) Less(other *Data) bool {
    return strings.Compare(data.Value, other.Value) == -1
}

func (data *Data) SortKey() string {
    return fmt.Sprintf("%v-%v", data.Value, data.Other)
}

func TestBasicList(test *testing.T){
    var list List[*Data]
    list.Add(&Data{
        Value: "a",
        Other: 0,
    })
    list.Add(&Data{
        Value: "b",
        Other: 0,
    })
    list.Add(&Data{
        Value: "c",
        Other: 0,
    })

    if list.Size() != 3 {
        test.Fatalf("expected size to be 3 but was %v", list.Size())
    }

    all := list.All()
    filtered := list.Filtered()

    if len(all) != 3 {
        test.Fatalf("expected all size to be 3 but was %v", len(all))
    }

    if len(filtered) != 3 {
        test.Fatalf("expected filtered size to be 3 but was %v", len(filtered))
    }
}

func TestFilter(test *testing.T){
    var list List[*Data]
    list.Add(&Data{
        Value: "xxaxx",
        Other: 0,
    })
    list.Add(&Data{
        Value: "xxaaxx",
        Other: 0,
    })
    list.Add(&Data{
        Value: "xxaaaxxx",
        Other: 0,
    })

    if len(list.Filtered()) != 3 {
        test.Fatalf("expected filtered size to be 3 but was %v", len(list.Filtered()))
    }

    list.AddFilter("a")

    if len(list.Filtered()) != 3 {
        test.Fatalf("expected filtered size to be 3 but was %v", len(list.Filtered()))
    }

    list.AddFilter("a")

    if len(list.Filtered()) != 2 {
        test.Fatalf("expected filtered size to be 2 but was %v", len(list.Filtered()))
    }

    list.AddFilter("a")

    if len(list.Filtered()) != 1 {
        test.Fatalf("expected filtered size to be 1 but was %v", len(list.Filtered()))
    }

    list.AddFilter("a")

    if len(list.Filtered()) != 0 {
        test.Fatalf("expected filtered size to be 0 but was %v", len(list.Filtered()))
    }

    list.BackspaceFilter()
    if len(list.Filtered()) != 1 {
        test.Fatalf("expected filtered size to be 1 but was %v", len(list.Filtered()))
    }
}
