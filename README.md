# go-wirenet
Simple bidirectional stream server.

## Table of contents
- [Installation](#installation)
- [Examples](#examples)
    + [Creating connection](#creating-connection)
    + [Stream handling](#stream-handling)
    + [Stream opening](#stream-opening)
    + [Writing to stream](#writing-to-stream)
    + [Reading from stream](#reading-from-stream)
     
### Installation
```ssh
go get github.com/mediabuyerbot/go-wirenet
```

### Examples
##### Creating connection 
```go
import "github.com/mediabuyerbot/go-wirenet"

// server side
wire, err := wirenet.Mount(":8989", nil)
if err != nil {
    handleError(err)
}
if err := wire.Connect(); err != nil {
    handleError(err)
}

// client side 
wire, err := wirenet.Join(":8989", nil)
if err != nil {
    handleError(err)
}

if err := wire.Connect(); err != nil {
    handleError(err)
}
```

##### Stream handling 
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

##### Stream opening 
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
...

// write to stream
n, err := stream.ReadFrom(backup)
...
```

##### Writing to stream 
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

##### Reading from stream 
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



