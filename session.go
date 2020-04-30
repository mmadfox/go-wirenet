package wirenet

type Session interface {
	IsClosed() bool
	Close() error
	Command(string) Cmd
}
