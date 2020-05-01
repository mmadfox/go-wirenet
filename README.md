# go-wirenet
Simple bidirectional client &lt;-> server

#### TODO: 
1. TLS/SSL
2. JWT Auth
3. Reconnect - DONE
4. Remote call
5. Middleware
6. Graceful shutdown - DONE
7. Error handler
8. Options
9. Logger interface


#### Some design...
```go
addr := "0:5678"

// server side
wire := wirenet.New(addr, wirenet.ServerSide)
wire.OpenSession(func(sess wirenet.Session) error {
     sessRegisterCh <- sess
     return nil
})
wire.CloseSession(func(sess wirenet.Session) error {
     sessUnregisterCh <- sess
     return nil
})
wire.Mount("balance:read", func(cmd wirenet.Cmd) error {
     file, err := os.Open("/path/to/balance.mxn")
     if err != nil {
     	return err 
     }
     return cmd.ReadFrom(file)
})
wire.Mount("balance:write", func(cmd wirenet.Cmd) error {
     file, err := os.Create("/path/to/balance.mxm")
     if err != nil {
        return err 
     }
     return cmd.WriteTo(file)
})
if err := wire.Listen(); err != nil {
    panic(err)
}
defer wire.Close()

// client side 1
wire := wirenet.New(addr, wirenet.ClientSide)
wire.OpenSession(func(sess wirenet.Session) error {
     sessRegisterCh <- sess
     return nil
})
wire.CloseSession(func(sess wirenet.Session) error {
     sessUnregisterCh <- sess
     return nil
})
wire.Mount("balance:geo:it:read", func(cmd wirenet.Cmd) error {
     file, err := os.Open("/path/to/balance.mxn")
     if err != nil {
     	return err 
     }
     return cmd.ReadFrom(file)
})
wire.Mount("balance:geo:it:write", func(cmd wirenet.Cmd) error {
     file, err := os.Create("/path/to/balance.mxm")
     if err != nil {
        return err 
     }
     return cmd.WriteTo(file)
})
if err := wire.Listen(); err != nil {
    panic(err)
}
defer wire.Close()

// client side 2
wire := wirenet.New(addr, wirenet.ClientSide)
wire.OpenSession(func(sess wirenet.Session) error {
     sessRegisterCh <- sess
     return nil
})
wire.CloseSession(func(sess wirenet.Session) error {
     sessUnregisterCh <- sess
     return nil
})
wire.Mount("balance:geo:usa:read", func(cmd wirenet.Cmd) error {
     file, err := os.Open("/path/to/balance.mxn")
     if err != nil {
     	return err 
     }
     return cmd.ReadFrom(file)
})
wire.Mount("balance:geo:usa:write", func(cmd wirenet.Cmd) error {
     file, err := os.Create("/path/to/balance.mxm")
     if err != nil {
        return err 
     }
     return cmd.WriteTo(file)
})
if err := wire.Listen(); err != nil {
    panic(err)
}
defer wire.Close()
```
