package mobi

// Mint is a number that can wear many types
type Mint int

// UInt16 casts to uint16
func (i Mint) UInt16() uint16 {
	return uint16(i)
}

// UInt32 casts to uint32
func (i Mint) UInt32() uint32 {
	return uint32(i)
}

// Int returns int value
func (i Mint) Int() int {
	return int(i)
}
