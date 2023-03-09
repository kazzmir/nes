package main

import (
    "testing"
    "context"
)

func TestBasic(test *testing.T){
    quit, cancel := context.WithCancel(context.Background())
    defer cancel()
    group := NewThreadGroup(quit)

    var x int
    
    group.Spawn(func (){
        for i := 0; i < 10; i++ {
            x += 1
        }
    })

    group.Wait()
    
    if x != 10 {
        test.Fatalf("expected x to be 10 but was %v", x)
    }
}
