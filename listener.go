package wirenet

const (
	ClientSide Side = 1
	ServerSide Side = 2
)

type Side int

func (s Side) String() (side string) {
	switch s {
	case ClientSide:
		side = "client side wire"
	case ServerSide:
		side = "server side wire"
	default:
		side = "unknown"
	}
	return side
}

type Mounter interface {
	Mount(string, func(Cmd)) error
}

type Listener interface {
	Listen() error
}

type Wire struct {
}
