package main

import (
    "testing"
    "context"
    "time"
    _ "fmt"
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

func TestCancel(test *testing.T){
    quit, cancel := context.WithCancel(context.Background())
    defer cancel()
    group := NewThreadGroup(quit)

    x := 0

    c := make(chan int)

    group.Spawn(func (){
        select {
            case <-group.Done():
                return
            case <-c:
                x = 2
        }
    })

    group.Spawn(func (){
        group.Cancel()
        x = 1
    })

    go func(){
        time.Sleep(5 * time.Millisecond)
        c <- 3
        close(c)
    }()

    group.Wait()

    if x != 1 {
        test.Fatalf("Expected x to be 1 but was %v", x)
    }

}

func TestSubGroup(test *testing.T){
    quit, cancel := context.WithCancel(context.Background())
    defer cancel()

    group := NewThreadGroup(quit)

    x := 0

    group1 := group.SubGroup()
    group1.Spawn(func(){
        defer func(){
            x = 3
        }()
        for {
            select {
                case <-group1.Done():
                    return
            }
        }
    })

    time.Sleep(1 * time.Millisecond)
    group.Cancel()
    group.Wait()

    if x != 3 {
        test.Fatalf("Expected x to be 3 but was %v", x)
    }
}
