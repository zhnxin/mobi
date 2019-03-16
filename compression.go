package mobi

import "bytes"

// CompressionStrategy is an enum of available compression strategies to use
type CompressionStrategy int

const (
	// CompressFast indicates fast compression with higher memory overhead (the default)
	CompressFast = CompressionStrategy(iota)
	// CompressLowMemory uses a slower, but less memory intensive compression
	CompressLowMemory
)

// The main entry point to using the palmLZ77 compression
// The exact speed is controlled at a module level through SetCompressionStrategy
var palmLZ77Compress func([]byte) []byte

// SetCompressionStrategy picks the compression strategy to use
func SetCompressionStrategy(strategy CompressionStrategy) {
	switch strategy {
	case CompressFast:
		palmLZ77Compress = func(data []byte) []byte { return palmLZ77CompressWithResolver(data, newLZ77TreeResolver) }
	case CompressLowMemory:
		palmLZ77Compress = func(data []byte) []byte { return palmLZ77CompressWithResolver(data, newLZ77LookupResolver) }
	}
}

func init() {
	SetCompressionStrategy(CompressFast)
}

func palmLZ77CompressWithResolver(data []byte, resolverProvider func([]byte) lz77Resolver) []byte {
	// Allocate an output buffer that already has capacity for the entire input
	// This avoids spending CPU cycles extending the output buffer through copies
	// when appending to it.
	outB := make([]byte, 0, len(data))

	// helper function to append just a bit nicer
	output := func(b ...byte) {
		outB = append(outB, b...)
	}

	// Last byte indicates how many bytes we've written past 4k bytes
	// We keep the tail separate, and do not attempt to compress it
	tailLen := int(data[len(data)-1])
	tail := data[(len(data)-1)-tailLen:]
	data = data[:(len(data)-1)-tailLen]

	ldata := len(data)

	// Initialize our chunk lookup resolver
	resolver := resolverProvider(data)

	for i := 0; i < ldata; i++ {
		if i > lz77MaxChunkLen && (ldata-i) > lz77MaxChunkLen {

			// Lookup the largest chunk we can find in the window
			needle := data[i : i+lz77MaxChunkLen] // guaranteed to work due to if statement in loop
			absIdx, chunkLen := resolver.findChunk(needle, i-lz77WindowSize, i)

			// Encode the chunk if we found one
			if absIdx > -1 {
				offset := int64(i) - int64(absIdx)
				// pack it into the data structure
				code := 0x8000 + (offset << 3) + (int64(chunkLen) - lz77MinChunkLen)
				// Write the two-byte structure
				output(byte(code>>8), byte(code))
				i += chunkLen - 1 // jump forward, taking the i++ into account
				continue          // Skip to next chunk to encode
			}
		}

		// We did not find any chunks we could use
		och := data[i]

		// Pack space characters and following characters together
		if och == chSpace && (i+1) < ldata {
			onch := data[i+1]
			if onch >= 0x40 && onch < 0x80 {
				// if following is an ascii letter
				// indicate space by setting 8th bit
				output(onch ^ 0x80)
				i++
				continue
			} else {
				// just output the space
				output(och)
				continue
			}
		}

		if och == 0 || (och > 8 && och < 0x80) {
			// A single-byte character. Just append it
			output(och)
		} else {
			// Multi-byte character - get all of them appended in
			j := i
			var binseq []byte
			for {
				if j < ldata && len(binseq) < 8 {
					och = data[j]
					if och == 0 || (och > 8 && och < 0x80) {
						break
					}
					binseq = append(binseq, och)
					j++
				} else {
					break
				}
			}
			output(byte(len(binseq)))
			output(binseq...)

			i += len(binseq) - 1
		}
	}
	// Reattach the tail, which includes the length
	output(tail...)
	return outB
}

// lz77Resolver objects can find a chunk in their data store between the min and max indices
type lz77Resolver interface {
	// findChunk finds the provided chunk between the idxMin and idxMax indices
	// It returns the longest chunk it found and the last index of that chunk.
	// It returns -1, _ if the chunk could not be found at all
	findChunk(chunk []byte, idxMin, idxMax int) (absIdx, chunkLen int)
}

// lz77LookupResolver uses a looping seek backwards through the data, but uses no extra memory
type lz77LookupResolver struct {
	data []byte
}

func newLZ77LookupResolver(data []byte) lz77Resolver {
	return lz77LookupResolver{data}
}

func (r lz77LookupResolver) findChunk(chunk []byte, idxMin, idxMax int) (absIdx, chunkLen int) {
	// Only seek through the available window size
	boundOffset := idxMin
	if boundOffset < 0 {
		boundOffset = 0
	}
	i := idxMax
	absIdx = -1

	// Lookup the largest chunk we can find in the window
	relIdx, chunkLen := lz77Lookup(r.data[boundOffset:i], chunk)
	if relIdx > -1 {
		absIdx = boundOffset + relIdx // get an absolute index
	}
	return
}

// lz77Lookup implements a single-pass lookup for all lengths of chunks.
func lz77Lookup(window, chunk []byte) (idx, foundLen int) {
	chLen := len(chunk)
	if chLen < lz77MinChunkLen {
		panic("Unable to search for chunks smaller than min chunk size!")
	}
	if chLen > lz77MaxChunkLen {
		panic("Unable to search for chunks larger than the max chunk size!")
	}

	idxs := make([]int, chLen)
	for i := range idxs {
		idxs[i] = -1
	}

	foundLen = lz77MinChunkLen - 1 // haven't found any, but guaranteed to be > 0

	c := chunk[0]

	// Running backwards through the window to find the last indices
	// stop if we've found a chunk of the full chunk length
	for idx := len(window) - lz77MinChunkLen; foundLen < chLen && idx >= 0; idx-- {
		if window[idx] == c { // shortcut check before expensive operation
			// Ignoring the length of chunk we've already found, try all longer chunk lenghts
			for currLen := foundLen + 1; currLen <= chLen; currLen++ {
				eq := bytes.Equal(window[idx:idx+currLen], chunk[:currLen])
				if eq {
					idxs[currLen-1] = idx
					// We've found the current length, only search for longer chunks now
					foundLen = currLen
				} else {
					// Longer chunks have this one as prefix, and will not be present
					break
				}
			}
		}
	}

	return idxs[foundLen-1], foundLen
}

// lz77TreeResolver implements the lz77Resolver interface with a prefix tree to find length 3 chunks before evaluating the best one.
// It is a lot faster than the lz77LookupResolver, but uses more memory
type lz77TreeResolver struct {
	data []byte
	tree map[[3]byte][]int
}

func newLZ77TreeResolver(data []byte) lz77Resolver {
	tree := make(map[[3]byte][]int)
	resolver := lz77TreeResolver{data, tree}
	for i := 0; i < len(data)-3; i++ {
		resolver.addPrefixLocation(data[i:i+3], i)
	}
	return &resolver
}

func (r *lz77TreeResolver) findChunk(needle []byte, idxMin, idxMax int) (absIdx, chunkLen int) {
	locations := r.getPrefixLocations(needle)
	absIdx, chunkLen = -1, -1
	if idxMin < 0 {
		idxMin = 0
	}
	if idxMax > len(r.data) {
		idxMax = len(r.data)
	}

	for _, loc := range locations {
		if loc < idxMin {
			// skip any prefixes that are outside of our max encoded value
			continue
		}
		if loc >= idxMax {
			// never return an offset jumping forward
			break // locations are sorted due to the way the tree is constructed
		}
		// candidate for replacement
		candiate := r.data[loc : loc+lz77MaxChunkLen]
		len := lz77MinChunkLen
		for j := len - 1; j < lz77MaxChunkLen; j++ {
			if needle[j] == candiate[j] {
				len = j + 1
			} else {
				break
			}
		}
		if len >= chunkLen {
			// replacing for same length ensures we get the shortest jump value possible
			chunkLen = len
			absIdx = loc
		}
	}
	return
}

// addPrefixLocation inserts a new prefix, which must be at least 3 bytes long - we don't care about
// anything beyond the first 3 bytes
func (r *lz77TreeResolver) addPrefixLocation(prefix []byte, location int) {
	if len(prefix) < 3 {
		panic("Prefixes must be 3 bytes long")
	}

	// Create the lookup key
	key := [3]byte{prefix[0], prefix[1], prefix[2]}
	locations, ok := r.tree[key]
	if !ok {
		locations = []int{location}
		r.tree[key] = locations
	} else {
		r.tree[key] = append(locations, location)
	}

}

// getPrefixLocations gets all locations of the prefix stored in the tree.
func (r *lz77TreeResolver) getPrefixLocations(prefix []byte) []int {

	// Grab the stored slice for the 3-byte prefix

	// this is safe because every single 3-byte prefix in the body is in the data structure
	// and we are only expecting to be asked about prefixes from the initial body
	return r.tree[[3]byte{prefix[0], prefix[1], prefix[2]}]
}
