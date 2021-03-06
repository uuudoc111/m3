// Copyright (c) 2019 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/m3db/m3/src/dbnode/encoding"
	"github.com/m3db/m3/src/dbnode/encoding/m3tsz"
	"github.com/m3db/m3/src/dbnode/namespace"
	"github.com/m3db/m3/src/dbnode/ts"
	"github.com/m3db/m3/src/dbnode/x/xio"
	"github.com/m3db/m3/src/x/checked"
	"github.com/m3db/m3/src/x/instrument"
	xtime "github.com/m3db/m3/src/x/time"

	"github.com/cespare/xxhash"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
)

// Make sure encoder implements encoding.Encoder.
var _ encoding.Encoder = &Encoder{}

const (
	currentEncodingSchemeVersion = 1
)

var (
	encErrPrefix                      = "proto encoder:"
	errEncoderSchemaIsRequired        = fmt.Errorf("%s schema is required", encErrPrefix)
	errEncoderMessageHasUnknownFields = fmt.Errorf("%s message has unknown fields", encErrPrefix)
	errEncoderClosed                  = fmt.Errorf("%s encoder is closed", encErrPrefix)
	errNoEncodedDatapoints            = fmt.Errorf("%s encoder has no encoded datapoints", encErrPrefix)
)

// Encoder compresses arbitrary ProtoBuf streams given a schema.
// TODO(rartoul): Add support for changing the schema (and updating the ordering
// of the custom encoded fields) on demand: https://github.com/m3db/m3/issues/1471
type Encoder struct {
	opts encoding.Options

	stream     encoding.OStream
	schemaDesc namespace.SchemaDescr
	schema     *desc.MessageDescriptor

	numEncoded    int
	lastEncodedDP ts.Datapoint
	lastEncoded   *dynamic.Message
	customFields  []customFieldState
	protoFields   []int32

	// Fields that are reused between function calls to
	// avoid allocations.
	varIntBuf              [8]byte
	changedValues          []int32
	fieldsChangedToDefault []int32
	marshalBuf             []byte

	unmarshaled *dynamic.Message

	hardErr          error
	hasEncodedSchema bool
	closed           bool

	stats            encoderStats
	timestampEncoder m3tsz.TimestampEncoder
}

// EncoderStats contains statistics about the encoders compression performance.
type EncoderStats struct {
	UncompressedBytes int
	CompressedBytes   int
}

type encoderStats struct {
	uncompressedBytes int
}

func (s *encoderStats) IncUncompressedBytes(x int) {
	s.uncompressedBytes += x
}

// NewEncoder creates a new protobuf encoder.
func NewEncoder(start time.Time, opts encoding.Options) *Encoder {
	initAllocIfEmpty := opts.EncoderPool() == nil
	stream := encoding.NewOStream(nil, initAllocIfEmpty, opts.BytesPool())
	return &Encoder{
		opts:   opts,
		stream: stream,
		timestampEncoder: m3tsz.NewTimestampEncoder(
			start, opts.DefaultTimeUnit(), opts),
		varIntBuf: [8]byte{},
	}
}

// Encode encodes a timestamp and a protobuf message. The function signature is strange
// in order to implement the encoding.Encoder interface. It accepts a ts.Datapoint, but
// only the Timestamp field will be used, the Value field will be ignored and will always
// return 0 on subsequent iteration. In addition, the provided annotation is expected to
// be a marshaled protobuf message that matches the configured schema.
func (enc *Encoder) Encode(dp ts.Datapoint, timeUnit xtime.Unit, protoBytes ts.Annotation) error {
	if unusableErr := enc.isUsable(); unusableErr != nil {
		return unusableErr
	}

	if enc.schema == nil {
		// It is a programmatic error that schema is not set at all prior to encoding, panic to fix it asap.
		return instrument.InvariantErrorf(errEncoderSchemaIsRequired.Error())
	}

	// Proto encoder value is meaningless, but make sure its always zero just to be safe so that
	// it doesn't cause LastEncoded() to produce invalid results.
	dp.Value = float64(0)

	if enc.unmarshaled == nil {
		// Lazy init.
		enc.unmarshaled = dynamic.NewMessage(enc.schema)
	}

	// Unmarshal the ProtoBuf message first to ensure we have a valid message before
	// we do anything else to reduce the change that we'll end up with a partially
	// encoded message.
	// TODO(rartoul): No need to allocate and unmarshal here, could do this in a streaming
	// fashion if we write our own decoder or expose the one in the underlying library.
	if err := enc.unmarshaled.Unmarshal(protoBytes); err != nil {
		return fmt.Errorf(
			"%s error unmarshaling annotation into proto message: %v", encErrPrefix, err)
	}

	if len(enc.unmarshaled.GetUnknownFields()) > 0 {
		// TODO(rartoul): Make this behavior configurable / this may change once we implement
		// mid-stream schema changes to make schema upgrades easier for clients.
		// https://github.com/m3db/m3/issues/1471
		return errEncoderMessageHasUnknownFields
	}

	// From this point onwards all errors are "hard errors" meaning that they should render
	// the encoder unusable since we may have encoded partial data.

	if enc.numEncoded == 0 {
		enc.encodeStreamHeader()
	}

	var (
		needToEncodeSchema   = !enc.hasEncodedSchema
		needToEncodeTimeUnit = timeUnit != enc.timestampEncoder.TimeUnit
	)
	if needToEncodeSchema || needToEncodeTimeUnit {
		// First bit means either there is no more data OR the time unit and/or schema has changed.
		enc.stream.WriteBit(opCodeNoMoreDataOrTimeUnitChangeAndOrSchemaChange)
		// Next bit means there is more data, but the time unit and/or schema has changed has changed.
		enc.stream.WriteBit(opCodeTimeUnitChangeAndOrSchemaChange)

		// Next bit is a boolean indicating whether the time unit has changed.
		if needToEncodeTimeUnit {
			enc.stream.WriteBit(opCodeTimeUnitChange)
		} else {
			enc.stream.WriteBit(opCodeTimeUnitUnchanged)
		}

		// Next bit is a boolean indicating whether the schema has changed.
		if needToEncodeSchema {
			enc.stream.WriteBit(opCodeSchemaChange)
		} else {
			enc.stream.WriteBit(opCodeSchemaUnchanged)
		}

		if needToEncodeTimeUnit {
			// The encoder manages encoding time unit changes manually (instead of deferring to
			// the timestamp encoder) because by default the WriteTime() API will use a marker
			// encoding scheme that relies on looking ahead into the stream for bit combinations that
			// could not possibly exist in the M3TSZ encoding scheme.
			// The protobuf encoder can't rely on this behavior because its possible for the protobuf
			// encoder to encode a legitimate bit combination that matches the "impossible" M3TSZ
			// markers exactly.
			enc.timestampEncoder.WriteTimeUnit(enc.stream, timeUnit)
		}

		if needToEncodeSchema {
			enc.encodeCustomSchemaTypes()
			enc.hasEncodedSchema = true
		}
	} else {
		// Control bit that indicates the stream has more data but no time unit or schema changes.
		enc.stream.WriteBit(opCodeMoreData)
	}

	err := enc.timestampEncoder.WriteTime(enc.stream, dp.Timestamp, nil, timeUnit)
	if err != nil {
		enc.hardErr = err
		return fmt.Errorf(
			"%s error encoding timestamp: %v", encErrPrefix, err)
	}

	if err := enc.encodeProto(enc.unmarshaled); err != nil {
		enc.hardErr = err
		return fmt.Errorf(
			"%s error encoding proto portion of message: %v", encErrPrefix, err)
	}

	enc.numEncoded++
	enc.lastEncodedDP = dp
	enc.stats.IncUncompressedBytes(len(protoBytes))
	return nil
}

// Stream returns a copy of the underlying data stream.
func (enc *Encoder) Stream(opts encoding.StreamOptions) (xio.SegmentReader, bool) {
	seg := enc.segment(true)
	if seg.Len() == 0 {
		return nil, false
	}

	if readerPool := enc.opts.SegmentReaderPool(); readerPool != nil {
		reader := readerPool.Get()
		reader.Reset(seg)
		return reader, true
	}
	return xio.NewSegmentReader(seg), true
}

func (enc *Encoder) segment(copy bool) ts.Segment {
	length := enc.stream.Len()
	if enc.stream.Len() == 0 {
		return ts.Segment{}
	}

	var head checked.Bytes
	buffer, _ := enc.stream.Rawbytes()
	if !copy {
		// Take ref from the ostream.
		head = enc.stream.Discard()
	} else {
		// Copy into new buffer.
		head = enc.newBuffer(length)
		head.IncRef()
		head.AppendAll(buffer)
		head.DecRef()
	}

	return ts.NewSegment(head, nil, ts.FinalizeHead)
}

// NumEncoded returns the number of encoded messages.
func (enc *Encoder) NumEncoded() int {
	return enc.numEncoded
}

// LastEncoded returns the last encoded datapoint. Does not include
// annotation / protobuf message for interface purposes.
func (enc *Encoder) LastEncoded() (ts.Datapoint, error) {
	if unusableErr := enc.isUsable(); unusableErr != nil {
		return ts.Datapoint{}, unusableErr
	}

	if enc.numEncoded == 0 {
		return ts.Datapoint{}, errNoEncodedDatapoints
	}

	// Value is meaningless for proto encoder and should already be zero,
	// but set it again to be safe.
	enc.lastEncodedDP.Value = 0
	return enc.lastEncodedDP, nil
}

// Len returns the length of the data stream.
func (enc *Encoder) Len() int {
	return enc.stream.Len()
}

// Stats returns EncoderStats which contain statistics about the encoders compression
// ratio.
func (enc *Encoder) Stats() EncoderStats {
	return EncoderStats{
		UncompressedBytes: enc.stats.uncompressedBytes,
		CompressedBytes:   enc.Len(),
	}
}

func (enc *Encoder) encodeStreamHeader() {
	enc.encodeVarInt(currentEncodingSchemeVersion)
	enc.encodeVarInt(uint64(enc.opts.ByteFieldDictionaryLRUSize()))
}

func (enc *Encoder) encodeCustomSchemaTypes() {
	if len(enc.customFields) == 0 {
		enc.encodeVarInt(0)
		return
	}

	// Field numbers are 1-indexed so encoding the maximum field number
	// at the beginning is equivalent to encoding the number of types
	// we need to read after if we imagine that we're encoding a 1-indexed
	// bitset where the position in the bitset encodes the field number (I.E
	// the first value is the type for field number 1) and the values are
	// the number of bits required to unique identify a custom type instead of
	// just being a single bit (3 bits in the case of version 1 of the encoding
	// scheme.)
	maxFieldNum := enc.customFields[len(enc.customFields)-1].fieldNum
	enc.encodeVarInt(uint64(maxFieldNum))

	// Start at 1 because we're zero-indexed.
	for i := 1; i <= maxFieldNum; i++ {
		customTypeBits := uint64(notCustomEncodedField)
		for _, customField := range enc.customFields {
			if customField.fieldNum == i {
				customTypeBits = uint64(customField.fieldType)
				break
			}
		}

		enc.stream.WriteBits(
			customTypeBits,
			numBitsToEncodeCustomType)
	}
}

func (enc *Encoder) encodeProto(m *dynamic.Message) error {
	if err := enc.encodeCustomValues(m); err != nil {
		return err
	}
	if err := enc.encodeProtoValues(m); err != nil {
		return err
	}

	return nil
}

// Reset resets the encoder for reuse.
func (enc *Encoder) Reset(
	start time.Time,
	capacity int,
	descr namespace.SchemaDescr,
) {
	enc.SetSchema(descr)
	enc.reset(start, capacity)
}

func (enc *Encoder) SetSchema(descr namespace.SchemaDescr) {
	if descr == nil {
		enc.schemaDesc = nil
		enc.resetSchema(nil)
		return
	}

	// Noop if schema has not changed.
	if enc.schemaDesc != nil && len(descr.DeployId()) != 0 && enc.schemaDesc.DeployId() == descr.DeployId() {
		return
	}

	enc.schemaDesc = descr
	enc.resetSchema(descr.Get().MessageDescriptor)
}

func (enc *Encoder) reset(start time.Time, capacity int) {
	enc.stream.Reset(enc.newBuffer(capacity))
	enc.timestampEncoder = m3tsz.NewTimestampEncoder(
		start, enc.opts.DefaultTimeUnit(), enc.opts)
	enc.lastEncoded = nil
	enc.lastEncodedDP = ts.Datapoint{}
	enc.unmarshaled = nil

	// Prevent this from growing too large and remaining in the pools.
	enc.marshalBuf = nil

	if enc.schema != nil {
		enc.customFields, enc.protoFields = customAndProtoFields(enc.customFields, enc.protoFields, enc.schema)
	}

	enc.closed = false
	enc.numEncoded = 0
}

func (enc *Encoder) resetSchema(schema *desc.MessageDescriptor) {
	enc.schema = schema
	if enc.schema == nil {
		enc.protoFields = nil
		enc.customFields = nil
		enc.lastEncoded = nil
		enc.unmarshaled = nil
	} else {
		enc.customFields, enc.protoFields = customAndProtoFields(enc.customFields, enc.protoFields, enc.schema)

		enc.lastEncoded = dynamic.NewMessage(schema)
		enc.unmarshaled = dynamic.NewMessage(schema)
	}
	enc.hasEncodedSchema = false
}

// Close closes the encoder.
func (enc *Encoder) Close() {
	if enc.closed {
		return
	}

	enc.Reset(time.Time{}, 0, nil)
	enc.stream.Reset(nil)
	enc.closed = true

	if pool := enc.opts.EncoderPool(); pool != nil {
		pool.Put(enc)
	}
}

// Discard closes the encoder and transfers ownership of the data stream to
// the caller.
func (enc *Encoder) Discard() ts.Segment {
	segment := enc.discard()
	// Close the encoder since its no longer needed
	enc.Close()
	return segment
}

// DiscardReset does the same thing as Discard except it also resets the encoder
// for reuse.
func (enc *Encoder) DiscardReset(start time.Time, capacity int, descr namespace.SchemaDescr) ts.Segment {
	segment := enc.discard()
	enc.Reset(start, capacity, descr)
	return segment
}

func (enc *Encoder) discard() ts.Segment {
	return enc.segment(false)
}

// Bytes returns the raw bytes of the underlying data stream. Does not
// transfer ownership and is generally unsafe.
func (enc *Encoder) Bytes() ([]byte, error) {
	if unusableErr := enc.isUsable(); unusableErr != nil {
		return nil, unusableErr
	}

	bytes, _ := enc.stream.Rawbytes()
	return bytes, nil
}

func (enc *Encoder) encodeCustomValues(m *dynamic.Message) error {
	for i, customField := range enc.customFields {
		iVal, err := m.TryGetFieldByNumber(customField.fieldNum)
		if err != nil {
			return fmt.Errorf(
				"%s error trying to get field number: %d",
				encErrPrefix, customField.fieldNum)
		}

		switch {
		case isCustomFloatEncodedField(customField.fieldType):
			if err := enc.encodeTSZValue(i, iVal); err != nil {
				return err
			}
		case isCustomIntEncodedField(customField.fieldType):
			if err := enc.encodeIntValue(i, iVal); err != nil {
				return err
			}
		case customField.fieldType == bytesField:
			if err := enc.encodeBytesValue(i, iVal); err != nil {
				return err
			}
		case customField.fieldType == boolField:
			if err := enc.encodeBoolValue(i, iVal); err != nil {
				return err
			}
		default:
			// This should never happen.
			return fmt.Errorf(
				"%s error no logic for custom encoding field number: %d",
				encErrPrefix, customField.fieldNum)
		}
	}

	return nil
}

func (enc *Encoder) encodeTSZValue(i int, iVal interface{}) error {
	var (
		val         float64
		customField = enc.customFields[i]
	)
	switch typedVal := iVal.(type) {
	case float64:
		val = typedVal
	case float32:
		val = float64(typedVal)
	default:
		return fmt.Errorf(
			"%s found unknown type in fieldNum %d", encErrPrefix, customField.fieldNum)
	}

	enc.customFields[i].floatEncAndIter.WriteFloat(enc.stream, val)
	return nil
}

func (enc *Encoder) encodeIntValue(i int, iVal interface{}) error {
	var (
		signedVal   int64
		unsignedVal uint64
	)
	switch typedVal := iVal.(type) {
	case uint64:
		unsignedVal = typedVal
	case uint32:
		unsignedVal = uint64(typedVal)
	case int64:
		signedVal = typedVal
	case int32:
		signedVal = int64(typedVal)
	default:
		return fmt.Errorf(
			"%s found unknown type in fieldNum %d", encErrPrefix, enc.customFields[i].fieldNum)
	}

	if isUnsignedInt(enc.customFields[i].fieldType) {
		enc.customFields[i].intEncAndIter.encodeUnsignedIntValue(enc.stream, unsignedVal)
	} else {
		enc.customFields[i].intEncAndIter.encodeSignedIntValue(enc.stream, signedVal)
	}

	return nil
}

func (enc *Encoder) encodeBytesValue(i int, iVal interface{}) error {
	customField := enc.customFields[i]
	currBytes, ok := iVal.([]byte)
	if !ok {
		currString, ok := iVal.(string)
		if !ok {
			return fmt.Errorf(
				"%s found unknown type in fieldNum %d", encErrPrefix, customField.fieldNum)
		}
		currBytes = []byte(currString)
	}

	var (
		hash             = xxhash.Sum64(currBytes)
		numPreviousBytes = len(customField.bytesFieldDict)
		lastStateIdx     = numPreviousBytes - 1
		lastState        encoderBytesFieldDictState
	)
	if numPreviousBytes > 0 {
		lastState = customField.bytesFieldDict[lastStateIdx]
	}

	if numPreviousBytes > 0 && hash == lastState.hash {
		streamBytes, _ := enc.stream.Rawbytes()
		match, err := enc.bytesMatchEncodedDictionaryValue(
			streamBytes, lastState, currBytes)
		if err != nil {
			return fmt.Errorf(
				"%s error checking if bytes match last encoded dictionary bytes: %v",
				encErrPrefix, err)
		}
		if match {
			// No changes control bit.
			enc.stream.WriteBit(opCodeNoChange)
			return nil
		}
	}

	// Bytes changed control bit.
	enc.stream.WriteBit(opCodeChange)

	streamBytes, _ := enc.stream.Rawbytes()
	for j, state := range customField.bytesFieldDict {
		if hash != state.hash {
			continue
		}

		match, err := enc.bytesMatchEncodedDictionaryValue(
			streamBytes, state, currBytes)
		if err != nil {
			return fmt.Errorf(
				"%s error checking if bytes match encoded dictionary bytes: %v",
				encErrPrefix, err)
		}
		if !match {
			continue
		}

		// Control bit means interpret next n bits as the index for the previous write
		// that this matches where n is the number of bits required to represent all
		// possible array indices in the configured LRU size.
		enc.stream.WriteBit(opCodeInterpretSubsequentBitsAsLRUIndex)
		enc.stream.WriteBits(
			uint64(j),
			numBitsRequiredForNumUpToN(
				enc.opts.ByteFieldDictionaryLRUSize()))
		enc.moveToEndOfBytesDict(i, j)
		return nil
	}

	// Control bit means interpret subsequent bits as varInt encoding length of a new
	// []byte we haven't seen before.
	enc.stream.WriteBit(opCodeInterpretSubsequentBitsAsBytesLengthVarInt)

	length := len(currBytes)
	enc.encodeVarInt(uint64(length))

	// Add padding bits until we reach the next byte. This ensures that the startPos
	// that we're going to store in the dictionary LRU will be aligned on a physical
	// byte boundary which makes retrieving the bytes again later for comparison much
	// easier.
	//
	// Note that this will waste up to a maximum of 7 bits per []byte that we encode
	// which is acceptable for now, but in the future we may want to make the code able
	// to do the comparison even if the bytes aren't aligned on a byte boundary in order
	// to improve the compression.
	enc.padToNextByte()

	// Track the byte position we're going to start at so we can store it in the LRU after.
	streamBytes, _ = enc.stream.Rawbytes()
	bytePos := len(streamBytes)

	// Write the actual bytes.
	enc.stream.WriteBytes(currBytes)

	enc.addToBytesDict(i, encoderBytesFieldDictState{
		hash:     hash,
		startPos: bytePos,
		length:   length,
	})
	return nil
}

func (enc *Encoder) encodeBoolValue(i int, val interface{}) error {
	boolVal, ok := val.(bool)
	if !ok {
		return fmt.Errorf(
			"%s found unknown type in fieldNum %d", encErrPrefix, enc.customFields[i].fieldNum)
	}

	if boolVal {
		enc.stream.WriteBit(opCodeBoolTrue)
	} else {
		enc.stream.WriteBit(opCodeBoolFalse)
	}

	return nil
}

func (enc *Encoder) encodeProtoValues(m *dynamic.Message) error {
	if len(enc.protoFields) == 0 {
		// Fast path, skip all the encoding logic entirely because there are
		// no fields that require proto encoding.
		// TODO(rartoul): Note that the encoding scheme could be further optimized
		// such that if there are no fields that require proto encoding then we don't
		// need to waste this bit per write.
		enc.stream.WriteBit(opCodeNoChange)
		return nil
	}

	// Reset for re-use.
	enc.changedValues = enc.changedValues[:0]
	changedFields := enc.changedValues

	enc.fieldsChangedToDefault = enc.fieldsChangedToDefault[:0]
	fieldsChangedToDefault := enc.fieldsChangedToDefault

	if enc.lastEncoded == nil {
		enc.lastEncoded = dynamic.NewMessage(enc.schema)
	}

	for _, fieldNum := range enc.protoFields {
		var (
			field       = enc.schema.FindFieldByNumber(fieldNum)
			fieldNumInt = int(fieldNum)
			prevVal     = enc.lastEncoded.GetFieldByNumber(fieldNumInt)
			curVal      = m.GetFieldByNumber(fieldNumInt)
		)

		if fieldsEqual(curVal, prevVal) {
			// Clear fields that haven't changed.
			if err := m.TryClearFieldByNumber(fieldNumInt); err != nil {
				return fmt.Errorf("error: %v clearing field: %d", err, fieldNumInt)
			}
		} else {
			isDefaultValue, err := isDefaultValue(field, curVal)
			if err != nil {
				return fmt.Errorf(
					"error: %v, checking if %v is default value for field %s",
					err, curVal, field.String())
			}
			if isDefaultValue {
				fieldsChangedToDefault = append(fieldsChangedToDefault, fieldNum)
			}

			changedFields = append(changedFields, fieldNum)
			if err := enc.lastEncoded.TrySetFieldByNumber(fieldNumInt, curVal); err != nil {
				return fmt.Errorf(
					"error: %v setting field %d with value %v on lastEncoded",
					err, fieldNumInt, curVal)
			}
		}
	}

	if len(changedFields) == 0 {
		// Only want to skip encoding if nothing has changed AND we've already
		// encoded the first message.
		enc.stream.WriteBit(opCodeNoChange)
		return nil
	}

	marshaled, err := m.MarshalAppend(enc.marshalBuf[:0])
	if err != nil {
		return fmt.Errorf("%s error trying to marshal protobuf: %v", encErrPrefix, err)
	}
	// Make sure we update the marshalBuf with the returned slice in case a new one was
	// allocated as part of the MarshalAppend call.
	enc.marshalBuf = marshaled

	// Control bit indicating that proto values have changed.
	enc.stream.WriteBit(opCodeChange)
	if len(fieldsChangedToDefault) > 0 {
		// Control bit indicating that some fields have been set to default values
		// and that a bitset will follow specifying which fields have changed.
		enc.stream.WriteBit(opCodeFieldsSetToDefaultProtoMarshal)
		enc.encodeBitset(fieldsChangedToDefault)
	} else {
		// Control bit indicating that none of the changed fields have been set to
		// their default values so we can do a clean merge on read.
		enc.stream.WriteBit(opCodeNoFieldsSetToDefaultProtoMarshal)
	}
	enc.encodeVarInt(uint64(len(marshaled)))
	enc.stream.WriteBytes(marshaled)

	return nil
}

func (enc *Encoder) isUsable() error {
	if enc.closed {
		return errEncoderClosed
	}

	if enc.hardErr != nil {
		return fmt.Errorf(
			"%s err encoder unusable due to hard err: %v",
			encErrPrefix, enc.hardErr)
	}

	return nil
}

func (enc *Encoder) bytesMatchEncodedDictionaryValue(
	streamBytes []byte,
	dictState encoderBytesFieldDictState,
	currBytes []byte,
) (bool, error) {
	var (
		prevEncodedBytesStart = dictState.startPos
		prevEncodedBytesEnd   = prevEncodedBytesStart + dictState.length
	)

	if prevEncodedBytesEnd > len(streamBytes) {
		// Should never happen.
		return false, fmt.Errorf(
			"bytes position in LRU is outside of stream bounds, streamSize: %d, startPos: %d, length: %d",
			len(streamBytes), prevEncodedBytesStart, dictState.length)
	}

	return bytes.Equal(streamBytes[prevEncodedBytesStart:prevEncodedBytesEnd], currBytes), nil
}

// padToNextByte will add padding bits in the current byte until the ostream
// reaches the beginning of the next byte. This allows us begin encoding data
// with the guarantee that we're aligned at a physical byte boundary.
func (enc *Encoder) padToNextByte() {
	_, bitPos := enc.stream.Rawbytes()
	for bitPos%8 != 0 {
		enc.stream.WriteBit(0)
		bitPos++
	}
}

func (enc *Encoder) moveToEndOfBytesDict(fieldIdx, i int) {
	existing := enc.customFields[fieldIdx].bytesFieldDict
	for j := i; j < len(existing); j++ {
		nextIdx := j + 1
		if nextIdx >= len(existing) {
			break
		}

		currVal := existing[j]
		nextVal := existing[nextIdx]
		existing[j] = nextVal
		existing[nextIdx] = currVal
	}
}

func (enc *Encoder) addToBytesDict(fieldIdx int, state encoderBytesFieldDictState) {
	existing := enc.customFields[fieldIdx].bytesFieldDict
	if len(existing) < enc.opts.ByteFieldDictionaryLRUSize() {
		enc.customFields[fieldIdx].bytesFieldDict = append(existing, state)
		return
	}

	// Shift everything down 1 and replace the last value to evict the
	// least recently used entry and add the newest one.
	//     [1,2,3]
	// becomes
	//     [2,3,3]
	// after shift, and then becomes
	//     [2,3,4]
	// after replacing the last value.
	for i := range existing {
		nextIdx := i + 1
		if nextIdx >= len(existing) {
			break
		}

		existing[i] = existing[nextIdx]
	}

	existing[len(existing)-1] = state
}

// encodeBitset writes out a bitset in the form of:
//
//      varint(number of bits)|bitset
//
// I.E first it encodes a varint which specifies the number of following
// bits to interpret as a bitset and then it encodes the provided values
// as zero-indexed bitset.
func (enc *Encoder) encodeBitset(values []int32) {
	var max int32
	for _, v := range values {
		if v > max {
			max = v
		}
	}

	// Encode a varint that indicates how many of the remaining
	// bits to interpret as a bitset.
	enc.encodeVarInt(uint64(max))

	// Encode the bitset
	for i := int32(0); i < max; i++ {
		wroteExists := false

		for _, v := range values {
			// Subtract one because the values are 1-indexed but the bitset
			// is 0-indexed.
			if i == v-1 {
				enc.stream.WriteBit(opCodeBitsetValueIsSet)
				wroteExists = true
				break
			}
		}

		if wroteExists {
			continue
		}

		enc.stream.WriteBit(opCodeBitsetValueIsNotSet)
	}
}

func (enc *Encoder) encodeVarInt(x uint64) {
	var (
		// Convert array to slice we can reuse the buffer.
		buf      = enc.varIntBuf[:]
		numBytes = binary.PutUvarint(buf, x)
	)

	// Reslice so we only write out as many bytes as is required
	// to represent the number.
	buf = buf[:numBytes]
	enc.stream.WriteBytes(buf)
}

func (enc *Encoder) newBuffer(capacity int) checked.Bytes {
	if bytesPool := enc.opts.BytesPool(); bytesPool != nil {
		return bytesPool.Get(capacity)
	}
	return checked.NewBytes(make([]byte, 0, capacity), nil)
}
