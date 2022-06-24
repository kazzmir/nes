package filterlist

import (
    "sort"
)

type Base interface {
    Contains (string) bool
    Less (Base) bool
    SortKey() string
}

type List[E Base] struct {
    values []E
    filtered []E
    filter string
}

func (list *List[E]) All() []E {
    return list.values
}

func (list *List[E]) Filtered() []E {
    return list.filtered
}

func (list *List[E]) Size() int {
    return len(list.values)
}

type SortRomIds[E Base] []E

func (data SortRomIds[E]) Len() int {
    return len(data)
}

func (data SortRomIds[E]) Swap(left, right int){
    data[left], data[right] = data[right], data[left]
}

func (data SortRomIds[E]) Less(left, right int) bool {
    return data[left].Less(data[right])
    // return strings.Compare(data[left].SortKey(), data[right].SortKey()) == -1
}


func (list *List[E]) Add(value E){
     list.values = append(list.values, value)
     sort.Sort(SortRomIds[E](list.values))
     if value.Contains(list.filter){
         list.filtered = append(list.filtered, value)
         sort.Sort(SortRomIds[E](list.filtered))
     }
}

/* returns true if a letter was taken off the filter */
func (list *List[E]) BackspaceFilter() bool {
    if len(list.filter) > 0 {
        list.filter = list.filter[0:len(list.filter)-1]
        
        list.ResetFilter()
        return true
    }

    return false
}

func (list *List[E]) AddFilter(f string){
    list.filter += f
    list.ResetFilter()
}

func (list *List[E]) ResetFilter() {
    list.filtered = nil
    for _, value := range list.values {
        if value.Contains(list.filter){
            list.filtered = append(list.filtered, value)
        }
    }
}

func (list *List[E]) Filter() string {
    return list.filter
}
