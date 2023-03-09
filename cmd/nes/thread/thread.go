package main

import (
    "sync"
    "context"
)

type ThreadGroup struct {
    wait sync.WaitGroup
    quit context.Context
    cancel context.CancelFunc
}

type ThreadFuncCancel func(quit context.Context, cancel context.CancelFunc)
type ThreadFunc func()

func NewThreadGroup(parent context.Context) *ThreadGroup {
    quit, cancel := context.WithCancel(parent)
    out := &ThreadGroup{
        quit: quit,
        cancel: cancel,
    }

    return out
}

/* create a new group that can have its own set of threads.
 * the current group will wait for all subgroups to exit
 */
func (group *ThreadGroup) SubGroup() *ThreadGroup {
    out := NewThreadGroup(group.quit)

    group.wait.Add(1)
    go func(){
        <-out.quit.Done()
        out.wait.Wait()
        defer group.wait.Done()
    }()
    
    return out
}

func (group *ThreadGroup) SpawnWithCancel(f ThreadFuncCancel){
    group.wait.Add(1)
    go func(){
        defer group.wait.Done()
        f(group.quit, group.cancel)
    }()
}

func (group *ThreadGroup) Spawn(f ThreadFunc) {
    group.wait.Add(1)
    go func(){
        defer group.wait.Done()
        f()
    }()
}

func (group *ThreadGroup) SpawnN(f ThreadFunc, i int) {
    for n := 0; n < i; n++ {
        group.Spawn(f)
    }
}

func (group *ThreadGroup) Cancel(){
    group.cancel()
}

func (group *ThreadGroup) Context() context.Context {
    return group.quit
}

func (group *ThreadGroup) Done() <-chan struct{} {
    return group.quit.Done()
}

func (group *ThreadGroup) Wait(){
    group.wait.Wait()
    group.cancel()
}
