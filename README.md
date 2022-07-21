# nerrgroup - drop-in replacement for [errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup)

It's allow nested call Go() function.

```go
    g := nerrgroup.New()
    var fn func() error
    fn = func() error {
        ...
        g.Go(fn)
    }

    g.Go(fn)
    if err := g.Wait(); err != nil {
        return err
    }
```
