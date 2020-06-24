# go-wirenet
Simple bidirectional TCP server.

## Table of contents
- [Installation](#installation)
- [Examples](#examples)
    + [Creating connection server side](#creating-connection-server-side)
    + [Creating connection client side](#creating-connection-client-side)
    + [Stream handling client or server side](#stream-handling-client-or-server-side)
    + [Writing payload one session](#writing-payload-one-session)
    + [Writing multi-payload one session](#writing-multi-payload-one-session)  
    + [Read from stream (Recommended)](#read-from-stream)
    + [Write to stream (Recommended)](#write-to-stream)
    + [Reading payload ](#reading-payload)
    + [Invoke stream from client or server side](#invoke-stream-from-client-or-server-side)

### Installation
```ssh
go get github.com/mediabuyerbot/go-wirenet
```

### Examples
##### Creating connection server side
```go
import "github.com/mediabuyerbot/go-wirenet"

wire, err := wirenet.Mount(":8989", nil)
if err != nil {
    panic(err)
}
if err := wire.Connect(); err != nil {
    panic(err)
}
```

##### Creating connection client side
```go
import "github.com/mediabuyerbot/go-wirenet"

wire, err := wirenet.Join(":8989", nil)
if err != nil {
    panic(err)
}
if err := wire.Connect(); err != nil {
	panic(err)
}
```

##### Stream handling client or server side
```go
wire.Stream("streamName", func(ctx context.Context, stream wirenet.Stream) {
    // ,,,  
})
```

##### Writing payload one session
```go
wire.Stream("streamName", func(ctx context.Context, stream wirenet.Stream) {
   writer := stream.Writer()
   for i := 0; i < 100; i++ {
       writer.Write([]byte("some payload"))
   }
   writer.Close()
})
```

##### Writing multi-payload one session
```go
wire.Stream("streamName", func(ctx context.Context, stream wirenet.Stream) {
   writer := stream.Writer()
   for i := 0; i < 100; i++ {
   	    writer.Write([]byte("one payload"))
   }
   writer.Close()
   for i := 0; i < 100; i++ {
   	    writer.Write([]byte("two payload"))
   }
   writer.Close()
})
```

##### Read from stream
```go
wire.Stream("streamName", func(ctx context.Context, stream wirenet.Stream) {
	n, err := stream.ReadFrom(os.Stdout)
    ...
})
```

##### Write to stream
```go
wire.Stream("streamName", func(ctx context.Context, stream wirenet.Stream) {
    n, err := stream.WriteTo(os.Stdout) 
    ...
})
```

##### Reading payload 
```go
wire.Stream("streamName", func(ctx context.Context, stream wirenet.Stream) {
   reader := stream.Reader()
   buf := make([]byte, wirenet.BufSize)
   for {
      n, err := reader.Read(buf)
      if err != nil {
      	  break
      } 
   }
   reader.Close() 
})
```

##### Invoke stream from client or server side
```go
wire, err := wirenet.Join(":8989",
		wirenet.WithSessionOpenHook(func(session wirenet.Session) {
		     hub.registerSession(session)	
		}),
        wirenet.WithSessionCloseHook(func(session wirenet.Session) {
            hub.unregisterSession(session)
        }),
)
...

sess := hub.findSession("sessionID")
stream, err := sess.OpenStream("streamName")
if err != nil {
   panic(err)
}
defer stream.Close() 

n, err := stream.ReadFrom(os.Stdin)
...
```




