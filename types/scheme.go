package types

type Scheme string

const (
	Exact Scheme = "exact"
)

type X402Version int

const (
	X402VersionV1 X402Version = 1
	X402VersionV2 X402Version = 2
)

type Signer func(digest []byte) (signature []byte, err error)
