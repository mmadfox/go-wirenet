# go-wirenet
Simple  bidirectional TCP stream server. Useful for NAT traversal.
---
[![Coverage Status](https://coveralls.io/repos/github/mediabuyerbot/go-wirenet/badge.svg?branch=master&2)](https://coveralls.io/github/mediabuyerbot/go-wirenet?branch=master)
[![Go Documentation](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)][godocs]
[![Go Report Card](https://goreportcard.com/badge/mediabuyerbot/go-wirenet)](https://goreportcard.com/report/mediabuyerbot/go-wirenet)

[godocs]: https://godoc.org/github.com/mediabuyerbot/go-wirenet 

## Design
![Design](assets/design.jpg)
##### Client-Server
```
// client <-> server
client1 join  to the server  ---------NAT------> server
client2 join to the server   ---------NAT------> server
client3 join to the server   ---------NAT------> server
client4 join to the server   ---------NAT------> server

// call from the server
call client1 from the server ---------NAT------> client1
call client 2 from the server --------NAT------> client2  
call client 3 from the server --------NAT------> client3  
call client 4 from the server --------NAT------> client4

// call from the client
call server from the client1 ---------NAT------> server
call server from the client2 ---------NAT------> server  
call server from the client3  --------NAT------> server  
call server from the client4  --------NAT------> server
```
##### Client-Hub
```
// clients <-> hub
client1 join to the hub  ---------NAT------> hub
client2 join to the hub  ---------NAT------> hub
client3 join to the hub  ---------NAT------> hub
client4 join to the hub  ---------NAT------> hub

// call from the client
call client2 from the client1 ---------NAT------> client2
call client1 from the client2 ---------NAT------> client1 
call client2 from the client3  --------NAT------> client2  
call client1 from the client4  --------NAT------> client1
```

## Table of contents
- [Installation](#installation)
- [Examples](#examples)
    + [Creating connection](#creating-connection)
    + [Stream handling](#stream-handling)
    + [Stream opening](#stream-opening)
    + [Writing to stream](#writing-to-stream)
    + [Reading from stream](#reading-from-stream)
    + [Using authentication](#using-authentication)
    + [Using SSL/TLS certs](#using-ssltls-certs)
    + [Shutdown](#shutdown)
    + [KeepAlive](#keepalive)
    + [Hub mode](#hub-mode)
- [Options](#options)    
     
### Installation
```ssh
go get github.com/mediabuyerbot/go-wirenet
```
### Examples
#### Creating connection 
```go
import "github.com/mediabuyerbot/go-wirenet"

// make server side
wire, err := wirenet.Mount(":8989", nil)
if err != nil {
    handleError(err)
}
if err := wire.Connect(); err != nil {
    handleError(err)
}

// OR make client side 
wire, err := wirenet.Join(":8989", nil)
if err != nil {
    handleError(err)
}

// connection
if err := wire.Connect(); err != nil {
    handleError(err)
}
```

#### Stream handling 
```go
import "github.com/mediabuyerbot/go-wirenet"

// server side
wire, err := wirenet.Mount(":8989", nil)
if err != nil {
    handleError(err)
}
// or client side
wire, err := wirenet.Join(":8989", nil)
if err != nil {
    handleError(err)
}

backupStream := func(ctx context.Context, stream wirenet.Stream) {
    file, err := os.Open("/backup.log")
    ...
    // write to stream
    n, err := stream.ReadFrom(file)
    ...
    stream.Close()
}

openChromeStream := func(ctx context.Context, stream wirenet.Stream) {
      // read from stream
      n, err := stream.WriteTo(os.Stdout)
} 

wire.Stream("backup", backupStream)
wire.Stream("openChrome", openChromeStream)

if err := wire.Connect(); err != nil {
    handleError(err)
}
```

#### Stream opening 
```go
// make options
opts := []wirenet.Option{
   wirenet.WithSessionOpenHook(func(session wirenet.Session) {
   		     hub.registerSession(session)	
   		}),
   		wirenet.WithSessionCloseHook(func(session wirenet.Session) {
   			 hub.unregisterSession(session)
        }),
}
// make client side
wire, err := wirenet.Join(":8989", opts...)
// OR make server side
wire, err := wirenet.Mount(":8989", opts...)

...

// find an open session in some repository
sess := hub.findSession("sessionID")
stream, err := sess.OpenStream("backup")
if err != nil {
   handleError(err)
}
defer stream.Close()
 
backup, err := os.Open("/backup.log")
if err != nil {
   handleError(err)
}
defer backup.Close()

// write to stream
n, err := stream.ReadFrom(backup)
...
```

#### Writing to stream 
```go
wire.Stream("account.set", func(ctx context.Context, stream wirenet.Stream) {
   // write to stream using writer 
   writer := stream.Writer()
   for {
      n, err := fileOne.Read(buf)
      if err != nil {
          handleError(err)
          break
      }
   	  n, err := writer.Write(buf[:n])
      ...
   }
   // EOF frame
   writer.Close()
   
   for {
         n, err := fileTwo.Read(buf)
         if err != nil {
             handleError(err)
             break
         }
      	  n, err := writer.Write(buf[:n])
         ...
      }
      // EOF frame
      writer.Close() 
   ...

   // or write to stream (recommended) 
   n, err := stream.ReadFrom(fileOne)
   ...
   n, err := stream.ReadFrom(fileTwo)
})
```

#### Reading from stream 
```go
wire.Stream("account.set", func(ctx context.Context, stream wirenet.Stream) {
   // reading from stream using reader 
   reader := stream.Reader()
   buf := make([]byte, wirenet.BufSize)
   n, err := reader.Read(buf)
   // EOF frame
   reader.Close()
   ...

   // or reader from stream (recommended)  
   n, err := stream.WriteTo(file)
   ...
})
```

#### Using authentication
server
```go
tokenValidator := func(streamName string, id wirenet.Identification, token wirenet.Token) error {
   if streamName == "public" {
      return nil 
   }
   return validate(token)
}

wire, err := wirenet.Mount(":8989", wirenet.WithTokenValidator(tokenValidator))
go func() {
	if err := wire.Connect(); err != nil {
	   handleError(err)
    }
}()

<-terminate()
wire.Close()
```

client
```go
 token := wirenet.Token("token")
 identification := wirenet.Identification("uuid")
 wire, err := wirenet.Join(":8989",
 		wirenet.WithIdentification(identification, token),
 )
 if err := wire.Connect(); err != nil {
    handleError(err)
 }
```

#### Using SSL/TLS certs
server
```go
// make keys 
// ./certs/server.key
// ./certs/server.pem
tlsConf, err := wirenet.LoadCertificates("server", "./certs")
if err != nil {
	handleError(err)
}
wire, err := wirenet.Mount(":8989", wirenet.WithTLS(tlsConf))
go func() {
	if err := wire.Connect(); err != nil {
	   handleError(err)
    }
}()

<-terminate()
wire.Close()
```
client
```go
// make keys 
// ./certs/client.key
// ./certs/client.pem
tlsConf, err := wirenet.LoadCertificates("client", "./certs")
if err != nil {
	handleError(err)
}
wire, err := wirenet.Mount(":8989", wirenet.WithTLS(tlsConf))
if err := wire.Connect(); err != nil {
    handleError(err)
}
```

#### Shutdown 
```go
timeout := 120*time.Second 
wire, err := wirenet.Mount(":8989",
    // Waiting time for completion of all streams
    wirenet.WithSessionCloseTimeout(timeout),
)
go func() {
	if err := wire.Connect(); err != nil {
	   handleError(err)
    }
}()

<-terminate()

wire.Close()
```

#### KeepAlive
```go
// server side
wire, err := wirenet.Mount(":8989",
     WithKeepAlive(true),
     WithKeepAliveInterval(30 * time.Second), 
)

// OR client side
wire, err := wirenet.Join(":8989",
     WithKeepAlive(true),
     WithKeepAliveInterval(30 * time.Second), 
)

go func() {
	if err := wire.Connect(); err != nil {
	   handleError(err)
    }
}()

<-terminate()

wire.Close()
```

#### Hub mode
##### Attention! The name of the stream for each client must be unique!
hub
```go
 hub, err := wirenet.Hub(":8989")
 hub.Connect()
```

client1
```go
client1, err := wirenet.Join(":8989")
client1.Stream("client1:readBalance", func(ctx context.Context, s Stream) {})
go func() {
   client1.Connect()
}()
...
sess, err := client2.Session("uuid")
stream, err := sess.OpenStream("client2:readBalance")
<-termiate()
client1.Close()
```

client2
```go
client2, err := wirenet.Join(":8989")
client2.Stream("client2:readBalance", func(ctx context.Context, s Stream) {})
go func() {
   client2.Connect()
}()
...
sess, err := client2.Session("uuid")
stream, err := sess.OpenStream("client1:readBalance")
<-termiate()
client2.Close()
```

#### Options
```go
wirenet.WithConnectHook(hook func(io.Closer)) Option
wirenet.WithSessionOpenHook(hook wirenet.SessionHook) Option
wirenet.WithSessionCloseHook(hook wirenet.SessionHook) Option
wirenet.WithIdentification(id wirenet.Identification, token wirenet.Token) Option
wirenet.WithTokenValidator(v wirenet.TokenValidator) Option                   // server side
wirenet.WithTLS(conf *tls.Config) Option
wirenet.WithRetryWait(min, max time.Duration) Option
wirenet.WithRetryMax(n int) Option
wirenet.WithReadWriteTimeouts(read, write time.Duration) Option
wirenet.WithSessionCloseTimeout(dur time.Duration) Option
```




