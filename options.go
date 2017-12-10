package dhcp4

import (
	"encoding"
	"math"
	"sort"

	"github.com/u-root/dhcp4/util"
)

var (
	zeroLengthOptions = map[OptionCode]struct{}{
		Pad: {},
		End: {},
	}
)

// Options is a map of OptionCode keys with a slice of byte values.
//
// Its methods can be used to easily check for additional information from a
// packet. Get should be used to access data from Options.
type Options map[OptionCode][]byte

// Add adds a new OptionCode key and BinaryMarshaler struct's bytes to the
// Options map.
func (o Options) Add(key OptionCode, value encoding.BinaryMarshaler) error {
	if value == nil {
		o.AddRaw(key, []byte{})
		return nil
	}

	b, err := value.MarshalBinary()
	if err != nil {
		return err
	}

	o.AddRaw(key, b)
	return nil
}

// AddRaw adds a new OptionCode key and raw value byte slice to the Options
// map.
func (o Options) AddRaw(key OptionCode, value []byte) {
	o[key] = append(o[key], value...)
}

// Get attempts to retrieve the value specified by an OptionCode key.
//
// If a value is found, get returns a non-nil byte slice and nil. If it is not
// found, Get returns nil and ErrOptionNotPresent.
func (o Options) Get(key OptionCode) ([]byte, error) {
	// Check for value by key.
	v, ok := o[key]
	if !ok {
		return nil, ErrOptionNotPresent
	}

	// Some options can actually have zero length (OptionRapidCommit), so
	// just return an empty byte slice if this is the case.
	if len(v) == 0 {
		return []byte{}, nil
	}
	return v, nil
}

// Unmarshal fills opts with option codes and corresponding values from an
// input byte slice.
//
// It is used with various different types to enable parsing of both top-level
// options, and options embedded within other options. If options data is
// malformed, it returns ErrInvalidOptions.
func (o *Options) Unmarshal(buf *util.Buffer) error {
	*o = make(Options)

	for buf.Len() >= 2 {
		// 1 byte: option code
		// 1 byte: option length n
		// n bytes: data
		code := OptionCode(buf.Read8())

		if _, ok := zeroLengthOptions[code]; ok {
			o.AddRaw(code, []byte{})
			continue
		}

		length := buf.Read8()
		if length == 0 {
			continue
		}

		// N bytes: option data
		data := buf.Consume(int(length))
		if data == nil {
			return ErrInvalidOptions
		}
		data = data[:int(length):int(length)]

		// RFC 3396: Just concatenate the data if the option code was
		// specified multiple times.
		o.AddRaw(code, data)
	}

	// Report error for any trailing bytes
	if buf.Len() != 0 {
		return ErrInvalidOptions
	}
	return nil
}

// Marshal writes options into the provided Buffer sorted by option codes.
func (o Options) Marshal(b *util.Buffer) {
	for _, code := range o.sortedKeys() {
		data := o[OptionCode(code)]

		// RFC 3396: If more than 256 bytes of data are given, the
		// option is simply listed multiple times.
		for len(data) >= 0 {
			// 1 byte: option code
			b.Write8(uint8(code))

			// Some DHCPv4 options have fixed length and do not put
			// length on the wire.
			if _, ok := zeroLengthOptions[OptionCode(code)]; ok {
				continue
			}

			n := len(data)
			if n > math.MaxUint8 {
				n = math.MaxUint8
			}

			// 1 byte: option length
			b.Write8(uint8(n))

			// N bytes: option data
			b.WriteBytes(data[:n])
			data = data[n:]
		}
	}
}

// enumerate returns an ordered slice of option data from the Options map,
// for use with sending responses to clients.
func (o Options) sortedKeys() []int {
	// Send all values for a given key
	var codes []int
	for k := range o {
		codes = append(codes, int(k))
	}

	sort.Sort(sort.IntSlice(codes))
	return codes
}
