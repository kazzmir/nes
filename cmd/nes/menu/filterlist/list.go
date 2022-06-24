package filterlist

/* This package implements a list of elements that can be filtered based on a string.
 * If the filter, f, is non-empty, then list.Filtered() returns only those elements
 * that return true for Contains(f).
 *
 * The main use case is to keep track of all the possible roms that could be loaded,
 * and to filter the roms based on a string the user types in such that only the roms
 * that contain the filter string are shown.
 *
 * The elements can be anything that implements the Base interface below.
 */

import (
    "sort"
)

type Base[T any] interface {
    /* true if this object contains the given string as a substring */
    Contains (string) bool
    /* true if this object is less than the argument */
    Less (T) bool
    /* some string used to compare elements in sorting */
    SortKey() string
}

type List[E Base[E]] struct {
    values []E
    filtered []E
    filter string
}

/* Return all the unfiltered elements in the list */
func (list *List[E]) All() []E {
    return list.values
}

/* Return only filtered elements */
func (list *List[E]) Filtered() []E {
    return list.filtered
}

/* number of elements in list */
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

/* Add a new element to the list */
func (list *List[E]) Add(value E){
     list.values = append(list.values, value)
     sort.Sort(Sortable[E](list.values))
     if value.Contains(list.filter){
         list.filtered = append(list.filtered, value)
         sort.Sort(Sortable[E](list.filtered))
     }
}

/* remove the most recently added letter from the filter string.
 * returns true if a letter was taken off the filter, or false
 * if the filter was empty
 */
func (list *List[E]) BackspaceFilter() bool {
    if len(list.filter) > 0 {
        list.filter = list.filter[0:len(list.filter)-1]
        
        list.ResetFilter()
        return true
    }

    return false
}

/* add a string to the end of the filter list */
func (list *List[E]) AddFilter(f string){
    list.filter += f
    list.ResetFilter()
}

/* update the filter list in case the underlying filter string has changed */
func (list *List[E]) ResetFilter() {
    list.filtered = nil
    for _, value := range list.values {
        if value.Contains(list.filter){
            list.filtered = append(list.filtered, value)
        }
    }
}

/* return the filter string */
func (list *List[E]) Filter() string {
    return list.filter
}
