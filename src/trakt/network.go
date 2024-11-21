package trakt

type Network struct {
	Name string `json:"name"`
}

type NetworkIterator struct{ BasicIterator }

func (n *NetworkIterator) Network() (*Network, error) {
	rcv := &Network{}
	return rcv, n.Scan(rcv)
}
