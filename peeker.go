package mobi

// Peeker is a multimorphic byte slice
type Peeker []byte

// Magic as mobiMagicType (string enum)
func (p Peeker) Magic() mobiMagicType {
	return mobiMagicType(p)
}

// String as plain old string
func (p Peeker) String() string {
	return string(p)
}

// Bytes as raw bytes
func (p Peeker) Bytes() []byte {
	return p
}

// Len as len of slice
func (p Peeker) Len() int {
	return len(p)
}
