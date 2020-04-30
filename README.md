# go-wirenet
Simple bidirectional client &lt;-> server

#### TODO: 
1. TLS/SSL
2. JWT Auth
3. Reconnect
4. Remote call
5. Middleware
6. Graceful shutdown


#### Some design...
```go
addr := "0.0.0.0:5678"
sessHub := somehub.New() 

// server side
wire := wirenet.New(addr, wirenet.ServerSide, 
   wirenet.WithOpenSession(func(sess wirenet.Session) error {
        return sessHub.register(sess)
   }),
   wirenet.WithCloseSession(func(sess wirenet.Session) error {
        return sessHub.unregister(sess)
   }),
)
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

...

sess := sessHub.Find("123")
cmd := sess.Command("balance:geo:it:read")
cmd.ReadFrom(someReader)

...

cmd = sess.Command("history:add")
json.NewEncoder(cmd).Encode(SomeRequest{
    ID: 1,
    Method: "Pay",
})
json.NewDecoder(cmd).Decode(&SomeResponse{})

defer wire.Close()

// client side 1
wire := wirenet.New(addr, wirenet.ClientSide)
wire.Mount("history:add", func(cmd wirenet.Cmd) {
    
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
