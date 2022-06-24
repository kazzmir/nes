package filterlist

import (
    "sort"
)

type Base[T any] interface {
    Contains (string) bool
    Less (T) bool
    SortKey() string
}

type List[E Base[E]] struct {
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

type Sortable[E Base[E]] []E

func (data Sortable[E]) Len() int {
    return len(data)
}

func (data Sortable[E]) Swap(left, right int){
    data[left], data[right] = data[right], data[left]
}

func (data Sortable[E]) Less(left, right int) bool {
    return data[left].Less(data[right])
}

func (list *List[E]) Add(value E){
     list.values = append(list.values, value)
     sort.Sort(Sortable[E](list.values))
     if value.Contains(list.filter){
         list.filtered = append(list.filtered, value)
         sort.Sort(Sortable[E](list.filtered))
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
