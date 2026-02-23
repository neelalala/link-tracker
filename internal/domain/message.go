package domain

type Message struct {
	ID   int64
	From User
	Text string
}
