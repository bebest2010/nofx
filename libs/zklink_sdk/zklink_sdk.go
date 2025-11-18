package zklink_sdk

// #include <zklink_sdk.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/big"
	"runtime"
	"sync/atomic"
	"unsafe"
)

// This is needed, because as of go 1.24
// type RustBuffer C.RustBuffer cannot have methods,
// RustBuffer is treated as non-local type
type GoRustBuffer struct {
	inner C.RustBuffer
}

type RustBufferI interface {
	AsReader() *bytes.Reader
	Free()
	ToGoBytes() []byte
	Data() unsafe.Pointer
	Len() uint64
	Capacity() uint64
	ExternalBuffer() C.RustBuffer
}

func RustBufferFromExternal(b RustBufferI) GoRustBuffer {
	return GoRustBuffer{
		inner: C.RustBuffer{
			capacity: C.uint64_t(b.Capacity()),
			len:      C.uint64_t(b.Len()),
			data:     (*C.uchar)(b.Data()),
		},
	}
}

func (cb GoRustBuffer) ExternalBuffer() C.RustBuffer {
	return cb.inner
}
func (cb GoRustBuffer) Capacity() uint64 {
	return uint64(cb.inner.capacity)
}

func (cb GoRustBuffer) Len() uint64 {
	return uint64(cb.inner.len)
}

func (cb GoRustBuffer) Data() unsafe.Pointer {
	return unsafe.Pointer(cb.inner.data)
}

func (cb GoRustBuffer) AsReader() *bytes.Reader {
	b := unsafe.Slice((*byte)(cb.inner.data), C.uint64_t(cb.inner.len))
	return bytes.NewReader(b)
}

func (cb GoRustBuffer) Free() {
	rustCall(func(status *C.RustCallStatus) bool {
		C.ffi_zklink_sdk_rustbuffer_free(cb.inner, status)
		return false
	})
}

func (cb GoRustBuffer) ToGoBytes() []byte {
	return C.GoBytes(unsafe.Pointer(cb.inner.data), C.int(cb.inner.len))
}

func stringToRustBuffer(str string) C.RustBuffer {
	return bytesToRustBuffer([]byte(str))
}

func bytesToRustBuffer(b []byte) C.RustBuffer {
	if len(b) == 0 {
		return C.RustBuffer{}
	}
	// We can pass the pointer along here, as it is pinned
	// for the duration of this call
	foreign := C.ForeignBytes{
		len:  C.int(len(b)),
		data: (*C.uchar)(unsafe.Pointer(&b[0])),
	}

	return rustCall(func(status *C.RustCallStatus) C.RustBuffer {
		return C.ffi_zklink_sdk_rustbuffer_from_bytes(foreign, status)
	})
}

type BufLifter[GoType any] interface {
	Lift(value RustBufferI) GoType
}

type BufLowerer[GoType any] interface {
	Lower(value GoType) C.RustBuffer
}

type BufReader[GoType any] interface {
	Read(reader io.Reader) GoType
}

type BufWriter[GoType any] interface {
	Write(writer io.Writer, value GoType)
}

func LowerIntoRustBuffer[GoType any](bufWriter BufWriter[GoType], value GoType) C.RustBuffer {
	// This might be not the most efficient way but it does not require knowing allocation size
	// beforehand
	var buffer bytes.Buffer
	bufWriter.Write(&buffer, value)

	bytes, err := io.ReadAll(&buffer)
	if err != nil {
		panic(fmt.Errorf("reading written data: %w", err))
	}
	return bytesToRustBuffer(bytes)
}

func LiftFromRustBuffer[GoType any](bufReader BufReader[GoType], rbuf RustBufferI) GoType {
	defer rbuf.Free()
	reader := rbuf.AsReader()
	item := bufReader.Read(reader)
	if reader.Len() > 0 {
		// TODO: Remove this
		leftover, _ := io.ReadAll(reader)
		panic(fmt.Errorf("Junk remaining in buffer after lifting: %s", string(leftover)))
	}
	return item
}

func rustCallWithError[E any, U any](converter BufReader[*E], callback func(*C.RustCallStatus) U) (U, *E) {
	var status C.RustCallStatus
	returnValue := callback(&status)
	err := checkCallStatus(converter, status)
	return returnValue, err
}

func checkCallStatus[E any](converter BufReader[*E], status C.RustCallStatus) *E {
	switch status.code {
	case 0:
		return nil
	case 1:
		return LiftFromRustBuffer(converter, GoRustBuffer{inner: status.errorBuf})
	case 2:
		// when the rust code sees a panic, it tries to construct a rustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer{inner: status.errorBuf})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		panic(fmt.Errorf("unknown status code: %d", status.code))
	}
}

func checkCallStatusUnknown(status C.RustCallStatus) error {
	switch status.code {
	case 0:
		return nil
	case 1:
		panic(fmt.Errorf("function not returning an error returned an error"))
	case 2:
		// when the rust code sees a panic, it tries to construct a C.RustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer{
				inner: status.errorBuf,
			})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		return fmt.Errorf("unknown status code: %d", status.code)
	}
}

func rustCall[U any](callback func(*C.RustCallStatus) U) U {
	returnValue, err := rustCallWithError[error](nil, callback)
	if err != nil {
		panic(err)
	}
	return returnValue
}

type NativeError interface {
	AsError() error
}

func writeInt8(writer io.Writer, value int8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint8(writer io.Writer, value uint8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt16(writer io.Writer, value int16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint16(writer io.Writer, value uint16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt32(writer io.Writer, value int32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint32(writer io.Writer, value uint32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt64(writer io.Writer, value int64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint64(writer io.Writer, value uint64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat32(writer io.Writer, value float32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat64(writer io.Writer, value float64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func readInt8(reader io.Reader) int8 {
	var result int8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint8(reader io.Reader) uint8 {
	var result uint8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt16(reader io.Reader) int16 {
	var result int16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint16(reader io.Reader) uint16 {
	var result uint16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt32(reader io.Reader) int32 {
	var result int32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint32(reader io.Reader) uint32 {
	var result uint32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt64(reader io.Reader) int64 {
	var result int64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint64(reader io.Reader) uint64 {
	var result uint64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat32(reader io.Reader) float32 {
	var result float32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat64(reader io.Reader) float64 {
	var result float64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func init() {

	uniffiCheckChecksums()
}

func uniffiCheckChecksums() {
	// Get the bindings contract version from our ComponentInterface
	bindingsContractVersion := 26
	// Get the scaffolding contract version by calling the into the dylib
	scaffoldingContractVersion := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.ffi_zklink_sdk_uniffi_contract_version()
	})
	if bindingsContractVersion != int(scaffoldingContractVersion) {
		// If this happens try cleaning and rebuilding your project
		panic("zklink_sdk: UniFFI contract version mismatch")
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_func_closest_packable_fee_amount()
		})
		if checksum != 56787 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_func_closest_packable_fee_amount: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_func_closest_packable_token_amount()
		})
		if checksum != 15676 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_func_closest_packable_token_amount: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_func_create_signed_change_pubkey()
		})
		if checksum != 27447 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_func_create_signed_change_pubkey: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_func_eth_signature_of_change_pubkey()
		})
		if checksum != 48679 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_func_eth_signature_of_change_pubkey: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_func_get_public_key_hash()
		})
		if checksum != 28741 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_func_get_public_key_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_func_is_fee_amount_packable()
		})
		if checksum != 52232 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_func_is_fee_amount_packable: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_func_is_token_amount_packable()
		})
		if checksum != 11776 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_func_is_token_amount_packable: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_func_verify_musig()
		})
		if checksum != 53405 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_func_verify_musig: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_func_zklink_main_net_url()
		})
		if checksum != 63488 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_func_zklink_main_net_url: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_func_zklink_test_net_url()
		})
		if checksum != 4933 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_func_zklink_test_net_url: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_autodeleveraging_create_signed_tx()
		})
		if checksum != 8966 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_autodeleveraging_create_signed_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_autodeleveraging_get_bytes()
		})
		if checksum != 52953 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_autodeleveraging_get_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_autodeleveraging_get_signature()
		})
		if checksum != 21114 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_autodeleveraging_get_signature: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_autodeleveraging_is_signature_valid()
		})
		if checksum != 2829 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_autodeleveraging_is_signature_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_autodeleveraging_is_valid()
		})
		if checksum != 32196 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_autodeleveraging_is_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_autodeleveraging_json_str()
		})
		if checksum != 3439 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_autodeleveraging_json_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_autodeleveraging_to_zklink_tx()
		})
		if checksum != 43427 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_autodeleveraging_to_zklink_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_autodeleveraging_tx_hash()
		})
		if checksum != 62875 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_autodeleveraging_tx_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_changepubkey_get_bytes()
		})
		if checksum != 22958 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_changepubkey_get_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_changepubkey_get_signature()
		})
		if checksum != 20330 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_changepubkey_get_signature: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_changepubkey_is_onchain()
		})
		if checksum != 10977 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_changepubkey_is_onchain: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_changepubkey_is_signature_valid()
		})
		if checksum != 25271 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_changepubkey_is_signature_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_changepubkey_is_valid()
		})
		if checksum != 31315 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_changepubkey_is_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_changepubkey_json_str()
		})
		if checksum != 43695 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_changepubkey_json_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_changepubkey_to_zklink_tx()
		})
		if checksum != 29950 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_changepubkey_to_zklink_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_changepubkey_tx_hash()
		})
		if checksum != 33612 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_changepubkey_tx_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_contract_create_signed_contract()
		})
		if checksum != 33956 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_contract_create_signed_contract: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_contract_get_bytes()
		})
		if checksum != 33474 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_contract_get_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_contract_get_signature()
		})
		if checksum != 40014 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_contract_get_signature: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_contract_is_long()
		})
		if checksum != 52375 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_contract_is_long: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_contract_is_short()
		})
		if checksum != 24664 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_contract_is_short: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_contract_is_signature_valid()
		})
		if checksum != 33071 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_contract_is_signature_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_contractmatching_create_signed_tx()
		})
		if checksum != 19417 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_contractmatching_create_signed_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_contractmatching_get_bytes()
		})
		if checksum != 32127 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_contractmatching_get_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_contractmatching_get_signature()
		})
		if checksum != 41645 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_contractmatching_get_signature: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_contractmatching_is_signature_valid()
		})
		if checksum != 33576 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_contractmatching_is_signature_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_contractmatching_is_valid()
		})
		if checksum != 55586 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_contractmatching_is_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_contractmatching_json_str()
		})
		if checksum != 42918 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_contractmatching_json_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_contractmatching_to_zklink_tx()
		})
		if checksum != 24807 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_contractmatching_to_zklink_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_contractmatching_tx_hash()
		})
		if checksum != 50267 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_contractmatching_tx_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_deposit_get_bytes()
		})
		if checksum != 65486 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_deposit_get_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_deposit_json_str()
		})
		if checksum != 17811 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_deposit_json_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_deposit_tx_hash()
		})
		if checksum != 38176 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_deposit_tx_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_ethsigner_get_address()
		})
		if checksum != 26188 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_ethsigner_get_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_ethsigner_sign_message()
		})
		if checksum != 50013 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_ethsigner_sign_message: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_forcedexit_create_signed_tx()
		})
		if checksum != 40833 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_forcedexit_create_signed_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_forcedexit_get_bytes()
		})
		if checksum != 41883 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_forcedexit_get_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_forcedexit_get_signature()
		})
		if checksum != 27026 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_forcedexit_get_signature: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_forcedexit_is_signature_valid()
		})
		if checksum != 6534 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_forcedexit_is_signature_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_forcedexit_is_valid()
		})
		if checksum != 46100 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_forcedexit_is_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_forcedexit_json_str()
		})
		if checksum != 4050 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_forcedexit_json_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_forcedexit_to_zklink_tx()
		})
		if checksum != 41198 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_forcedexit_to_zklink_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_forcedexit_tx_hash()
		})
		if checksum != 29436 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_forcedexit_tx_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_fullexit_get_bytes()
		})
		if checksum != 47530 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_fullexit_get_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_fullexit_is_valid()
		})
		if checksum != 57198 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_fullexit_is_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_fullexit_json_str()
		})
		if checksum != 24199 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_fullexit_json_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_fullexit_to_zklink_tx()
		})
		if checksum != 51916 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_fullexit_to_zklink_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_fullexit_tx_hash()
		})
		if checksum != 23215 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_fullexit_tx_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_funding_create_signed_tx()
		})
		if checksum != 9980 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_funding_create_signed_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_funding_get_bytes()
		})
		if checksum != 59564 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_funding_get_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_funding_get_signature()
		})
		if checksum != 40848 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_funding_get_signature: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_funding_is_signature_valid()
		})
		if checksum != 50669 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_funding_is_signature_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_funding_is_valid()
		})
		if checksum != 4189 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_funding_is_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_funding_json_str()
		})
		if checksum != 55097 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_funding_json_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_funding_to_zklink_tx()
		})
		if checksum != 8152 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_funding_to_zklink_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_funding_tx_hash()
		})
		if checksum != 4435 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_funding_tx_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_liquidation_create_signed_tx()
		})
		if checksum != 25926 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_liquidation_create_signed_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_liquidation_get_bytes()
		})
		if checksum != 23568 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_liquidation_get_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_liquidation_get_signature()
		})
		if checksum != 60548 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_liquidation_get_signature: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_liquidation_is_signature_valid()
		})
		if checksum != 8478 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_liquidation_is_signature_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_liquidation_is_valid()
		})
		if checksum != 2828 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_liquidation_is_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_liquidation_json_str()
		})
		if checksum != 62587 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_liquidation_json_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_liquidation_to_zklink_tx()
		})
		if checksum != 34516 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_liquidation_to_zklink_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_liquidation_tx_hash()
		})
		if checksum != 49785 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_liquidation_tx_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_order_create_signed_order()
		})
		if checksum != 18682 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_order_create_signed_order: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_order_get_bytes()
		})
		if checksum != 34383 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_order_get_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_order_get_eth_sign_msg()
		})
		if checksum != 11725 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_order_get_eth_sign_msg: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_order_get_signature()
		})
		if checksum != 51023 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_order_get_signature: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_order_is_signature_valid()
		})
		if checksum != 6764 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_order_is_signature_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_order_is_valid()
		})
		if checksum != 56951 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_order_is_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_order_json_str()
		})
		if checksum != 20284 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_order_json_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_ordermatching_create_signed_tx()
		})
		if checksum != 30551 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_ordermatching_create_signed_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_ordermatching_get_bytes()
		})
		if checksum != 62780 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_ordermatching_get_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_ordermatching_get_signature()
		})
		if checksum != 32489 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_ordermatching_get_signature: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_ordermatching_is_signature_valid()
		})
		if checksum != 54946 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_ordermatching_is_signature_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_ordermatching_is_valid()
		})
		if checksum != 51995 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_ordermatching_is_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_ordermatching_json_str()
		})
		if checksum != 33830 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_ordermatching_json_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_ordermatching_to_zklink_tx()
		})
		if checksum != 50976 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_ordermatching_to_zklink_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_ordermatching_tx_hash()
		})
		if checksum != 58375 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_ordermatching_tx_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_signer_sign_auto_deleveraging()
		})
		if checksum != 1081 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_signer_sign_auto_deleveraging: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_signer_sign_change_pubkey_with_create2data_auth()
		})
		if checksum != 15882 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_signer_sign_change_pubkey_with_create2data_auth: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_signer_sign_change_pubkey_with_eth_ecdsa_auth()
		})
		if checksum != 52766 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_signer_sign_change_pubkey_with_eth_ecdsa_auth: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_signer_sign_change_pubkey_with_onchain_auth_data()
		})
		if checksum != 22310 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_signer_sign_change_pubkey_with_onchain_auth_data: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_signer_sign_contract_matching()
		})
		if checksum != 44480 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_signer_sign_contract_matching: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_signer_sign_forced_exit()
		})
		if checksum != 11179 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_signer_sign_forced_exit: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_signer_sign_funding()
		})
		if checksum != 11762 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_signer_sign_funding: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_signer_sign_liquidation()
		})
		if checksum != 49506 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_signer_sign_liquidation: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_signer_sign_order_matching()
		})
		if checksum != 57629 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_signer_sign_order_matching: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_signer_sign_transfer()
		})
		if checksum != 14421 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_signer_sign_transfer: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_signer_sign_withdraw()
		})
		if checksum != 41078 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_signer_sign_withdraw: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_starksigner_sign_message()
		})
		if checksum != 41141 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_starksigner_sign_message: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_transfer_create_signed_tx()
		})
		if checksum != 197 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_transfer_create_signed_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_transfer_eth_signature()
		})
		if checksum != 21975 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_transfer_eth_signature: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_transfer_get_bytes()
		})
		if checksum != 36810 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_transfer_get_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_transfer_get_eth_sign_msg()
		})
		if checksum != 46393 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_transfer_get_eth_sign_msg: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_transfer_get_signature()
		})
		if checksum != 51800 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_transfer_get_signature: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_transfer_is_signature_valid()
		})
		if checksum != 31540 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_transfer_is_signature_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_transfer_is_valid()
		})
		if checksum != 46475 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_transfer_is_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_transfer_json_str()
		})
		if checksum != 28252 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_transfer_json_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_transfer_to_zklink_tx()
		})
		if checksum != 61833 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_transfer_to_zklink_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_transfer_tx_hash()
		})
		if checksum != 20921 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_transfer_tx_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_updateglobalvar_get_bytes()
		})
		if checksum != 50292 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_updateglobalvar_get_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_updateglobalvar_is_valid()
		})
		if checksum != 7961 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_updateglobalvar_is_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_updateglobalvar_json_str()
		})
		if checksum != 48653 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_updateglobalvar_json_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_updateglobalvar_to_zklink_tx()
		})
		if checksum != 32093 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_updateglobalvar_to_zklink_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_updateglobalvar_tx_hash()
		})
		if checksum != 4132 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_updateglobalvar_tx_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_withdraw_create_signed_tx()
		})
		if checksum != 48417 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_withdraw_create_signed_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_withdraw_eth_signature()
		})
		if checksum != 43950 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_withdraw_eth_signature: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_withdraw_get_bytes()
		})
		if checksum != 49783 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_withdraw_get_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_withdraw_get_eth_sign_msg()
		})
		if checksum != 27813 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_withdraw_get_eth_sign_msg: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_withdraw_get_signature()
		})
		if checksum != 65388 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_withdraw_get_signature: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_withdraw_is_signature_valid()
		})
		if checksum != 9636 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_withdraw_is_signature_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_withdraw_is_valid()
		})
		if checksum != 32004 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_withdraw_is_valid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_withdraw_json_str()
		})
		if checksum != 3719 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_withdraw_json_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_withdraw_to_zklink_tx()
		})
		if checksum != 803 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_withdraw_to_zklink_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_withdraw_tx_hash()
		})
		if checksum != 21707 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_withdraw_tx_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_zklinksigner_public_key()
		})
		if checksum != 30823 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_zklinksigner_public_key: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_method_zklinksigner_sign_musig()
		})
		if checksum != 6025 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_method_zklinksigner_sign_musig: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_autodeleveraging_new()
		})
		if checksum != 18996 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_autodeleveraging_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_changepubkey_new()
		})
		if checksum != 47899 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_changepubkey_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_contract_new()
		})
		if checksum != 60387 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_contract_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_contractmatching_new()
		})
		if checksum != 58427 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_contractmatching_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_deposit_new()
		})
		if checksum != 9606 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_deposit_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_ethsigner_new()
		})
		if checksum != 52490 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_ethsigner_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_forcedexit_new()
		})
		if checksum != 39419 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_forcedexit_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_fullexit_new()
		})
		if checksum != 64399 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_fullexit_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_funding_new()
		})
		if checksum != 22904 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_funding_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_liquidation_new()
		})
		if checksum != 42652 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_liquidation_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_order_new()
		})
		if checksum != 3461 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_order_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_ordermatching_new()
		})
		if checksum != 55522 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_ordermatching_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_signer_new()
		})
		if checksum != 59644 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_signer_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_starksigner_new()
		})
		if checksum != 56808 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_starksigner_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_starksigner_new_from_hex_str()
		})
		if checksum != 15148 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_starksigner_new_from_hex_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_transfer_new()
		})
		if checksum != 427 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_transfer_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_typeddata_new()
		})
		if checksum != 19115 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_typeddata_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_updateglobalvar_new()
		})
		if checksum != 59178 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_updateglobalvar_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_withdraw_new()
		})
		if checksum != 46805 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_withdraw_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_zklinksigner_new()
		})
		if checksum != 26045 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_zklinksigner_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_zklinksigner_new_from_bytes()
		})
		if checksum != 17191 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_zklinksigner_new_from_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_zklinksigner_new_from_hex_eth_signer()
		})
		if checksum != 65185 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_zklinksigner_new_from_hex_eth_signer: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_zklinksigner_new_from_hex_stark_signer()
		})
		if checksum != 54137 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_zklinksigner_new_from_hex_stark_signer: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zklink_sdk_checksum_constructor_zklinksigner_new_from_seed()
		})
		if checksum != 25994 {
			// If this happens try cleaning and rebuilding your project
			panic("zklink_sdk: uniffi_zklink_sdk_checksum_constructor_zklinksigner_new_from_seed: UniFFI API checksum mismatch")
		}
	}
}

type FfiConverterUint8 struct{}

var FfiConverterUint8INSTANCE = FfiConverterUint8{}

func (FfiConverterUint8) Lower(value uint8) C.uint8_t {
	return C.uint8_t(value)
}

func (FfiConverterUint8) Write(writer io.Writer, value uint8) {
	writeUint8(writer, value)
}

func (FfiConverterUint8) Lift(value C.uint8_t) uint8 {
	return uint8(value)
}

func (FfiConverterUint8) Read(reader io.Reader) uint8 {
	return readUint8(reader)
}

type FfiDestroyerUint8 struct{}

func (FfiDestroyerUint8) Destroy(_ uint8) {}

type FfiConverterUint16 struct{}

var FfiConverterUint16INSTANCE = FfiConverterUint16{}

func (FfiConverterUint16) Lower(value uint16) C.uint16_t {
	return C.uint16_t(value)
}

func (FfiConverterUint16) Write(writer io.Writer, value uint16) {
	writeUint16(writer, value)
}

func (FfiConverterUint16) Lift(value C.uint16_t) uint16 {
	return uint16(value)
}

func (FfiConverterUint16) Read(reader io.Reader) uint16 {
	return readUint16(reader)
}

type FfiDestroyerUint16 struct{}

func (FfiDestroyerUint16) Destroy(_ uint16) {}

type FfiConverterInt16 struct{}

var FfiConverterInt16INSTANCE = FfiConverterInt16{}

func (FfiConverterInt16) Lower(value int16) C.int16_t {
	return C.int16_t(value)
}

func (FfiConverterInt16) Write(writer io.Writer, value int16) {
	writeInt16(writer, value)
}

func (FfiConverterInt16) Lift(value C.int16_t) int16 {
	return int16(value)
}

func (FfiConverterInt16) Read(reader io.Reader) int16 {
	return readInt16(reader)
}

type FfiDestroyerInt16 struct{}

func (FfiDestroyerInt16) Destroy(_ int16) {}

type FfiConverterUint32 struct{}

var FfiConverterUint32INSTANCE = FfiConverterUint32{}

func (FfiConverterUint32) Lower(value uint32) C.uint32_t {
	return C.uint32_t(value)
}

func (FfiConverterUint32) Write(writer io.Writer, value uint32) {
	writeUint32(writer, value)
}

func (FfiConverterUint32) Lift(value C.uint32_t) uint32 {
	return uint32(value)
}

func (FfiConverterUint32) Read(reader io.Reader) uint32 {
	return readUint32(reader)
}

type FfiDestroyerUint32 struct{}

func (FfiDestroyerUint32) Destroy(_ uint32) {}

type FfiConverterUint64 struct{}

var FfiConverterUint64INSTANCE = FfiConverterUint64{}

func (FfiConverterUint64) Lower(value uint64) C.uint64_t {
	return C.uint64_t(value)
}

func (FfiConverterUint64) Write(writer io.Writer, value uint64) {
	writeUint64(writer, value)
}

func (FfiConverterUint64) Lift(value C.uint64_t) uint64 {
	return uint64(value)
}

func (FfiConverterUint64) Read(reader io.Reader) uint64 {
	return readUint64(reader)
}

type FfiDestroyerUint64 struct{}

func (FfiDestroyerUint64) Destroy(_ uint64) {}

type FfiConverterBool struct{}

var FfiConverterBoolINSTANCE = FfiConverterBool{}

func (FfiConverterBool) Lower(value bool) C.int8_t {
	if value {
		return C.int8_t(1)
	}
	return C.int8_t(0)
}

func (FfiConverterBool) Write(writer io.Writer, value bool) {
	if value {
		writeInt8(writer, 1)
	} else {
		writeInt8(writer, 0)
	}
}

func (FfiConverterBool) Lift(value C.int8_t) bool {
	return value != 0
}

func (FfiConverterBool) Read(reader io.Reader) bool {
	return readInt8(reader) != 0
}

type FfiDestroyerBool struct{}

func (FfiDestroyerBool) Destroy(_ bool) {}

type FfiConverterString struct{}

var FfiConverterStringINSTANCE = FfiConverterString{}

func (FfiConverterString) Lift(rb RustBufferI) string {
	defer rb.Free()
	reader := rb.AsReader()
	b, err := io.ReadAll(reader)
	if err != nil {
		panic(fmt.Errorf("reading reader: %w", err))
	}
	return string(b)
}

func (FfiConverterString) Read(reader io.Reader) string {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading string, expected %d, read %d", length, read_length))
	}
	return string(buffer)
}

func (FfiConverterString) Lower(value string) C.RustBuffer {
	return stringToRustBuffer(value)
}

func (FfiConverterString) Write(writer io.Writer, value string) {
	if len(value) > math.MaxInt32 {
		panic("String is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := io.WriteString(writer, value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing string, expected %d, written %d", len(value), write_length))
	}
}

type FfiDestroyerString struct{}

func (FfiDestroyerString) Destroy(_ string) {}

// Below is an implementation of synchronization requirements outlined in the link.
// https://github.com/mozilla/uniffi-rs/blob/0dc031132d9493ca812c3af6e7dd60ad2ea95bf0/uniffi_bindgen/src/bindings/kotlin/templates/ObjectRuntime.kt#L31

type FfiObject struct {
	pointer       unsafe.Pointer
	callCounter   atomic.Int64
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer
	freeFunction  func(unsafe.Pointer, *C.RustCallStatus)
	destroyed     atomic.Bool
}

func newFfiObject(
	pointer unsafe.Pointer,
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer,
	freeFunction func(unsafe.Pointer, *C.RustCallStatus),
) FfiObject {
	return FfiObject{
		pointer:       pointer,
		cloneFunction: cloneFunction,
		freeFunction:  freeFunction,
	}
}

func (ffiObject *FfiObject) incrementPointer(debugName string) unsafe.Pointer {
	for {
		counter := ffiObject.callCounter.Load()
		if counter <= -1 {
			panic(fmt.Errorf("%v object has already been destroyed", debugName))
		}
		if counter == math.MaxInt64 {
			panic(fmt.Errorf("%v object call counter would overflow", debugName))
		}
		if ffiObject.callCounter.CompareAndSwap(counter, counter+1) {
			break
		}
	}

	return rustCall(func(status *C.RustCallStatus) unsafe.Pointer {
		return ffiObject.cloneFunction(ffiObject.pointer, status)
	})
}

func (ffiObject *FfiObject) decrementPointer() {
	if ffiObject.callCounter.Add(-1) == -1 {
		ffiObject.freeRustArcPtr()
	}
}

func (ffiObject *FfiObject) destroy() {
	if ffiObject.destroyed.CompareAndSwap(false, true) {
		if ffiObject.callCounter.Add(-1) == -1 {
			ffiObject.freeRustArcPtr()
		}
	}
}

func (ffiObject *FfiObject) freeRustArcPtr() {
	rustCall(func(status *C.RustCallStatus) int32 {
		ffiObject.freeFunction(ffiObject.pointer, status)
		return 0
	})
}

type AutoDeleveragingInterface interface {
	CreateSignedTx(signer *ZkLinkSigner) (*AutoDeleveraging, error)
	GetBytes() []uint8
	GetSignature() ZkLinkSignature
	IsSignatureValid() bool
	IsValid() bool
	JsonStr() string
	ToZklinkTx() ZkLinkTx
	TxHash() []uint8
}
type AutoDeleveraging struct {
	ffiObject FfiObject
}

func NewAutoDeleveraging(builder AutoDeleveragingBuilder) *AutoDeleveraging {
	return FfiConverterAutoDeleveragingINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_autodeleveraging_new(FfiConverterAutoDeleveragingBuilderINSTANCE.Lower(builder), _uniffiStatus)
	}))
}

func (_self *AutoDeleveraging) CreateSignedTx(signer *ZkLinkSigner) (*AutoDeleveraging, error) {
	_pointer := _self.ffiObject.incrementPointer("*AutoDeleveraging")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_method_autodeleveraging_create_signed_tx(
			_pointer, FfiConverterZkLinkSignerINSTANCE.Lower(signer), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *AutoDeleveraging
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAutoDeleveragingINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *AutoDeleveraging) GetBytes() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*AutoDeleveraging")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_autodeleveraging_get_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *AutoDeleveraging) GetSignature() ZkLinkSignature {
	_pointer := _self.ffiObject.incrementPointer("*AutoDeleveraging")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterZkLinkSignatureINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_autodeleveraging_get_signature(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *AutoDeleveraging) IsSignatureValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*AutoDeleveraging")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_autodeleveraging_is_signature_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *AutoDeleveraging) IsValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*AutoDeleveraging")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_autodeleveraging_is_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *AutoDeleveraging) JsonStr() string {
	_pointer := _self.ffiObject.incrementPointer("*AutoDeleveraging")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_autodeleveraging_json_str(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *AutoDeleveraging) ToZklinkTx() ZkLinkTx {
	_pointer := _self.ffiObject.incrementPointer("*AutoDeleveraging")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeZkLinkTxINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_autodeleveraging_to_zklink_tx(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *AutoDeleveraging) TxHash() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*AutoDeleveraging")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_autodeleveraging_tx_hash(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *AutoDeleveraging) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAutoDeleveraging struct{}

var FfiConverterAutoDeleveragingINSTANCE = FfiConverterAutoDeleveraging{}

func (c FfiConverterAutoDeleveraging) Lift(pointer unsafe.Pointer) *AutoDeleveraging {
	result := &AutoDeleveraging{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_autodeleveraging(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_autodeleveraging(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*AutoDeleveraging).Destroy)
	return result
}

func (c FfiConverterAutoDeleveraging) Read(reader io.Reader) *AutoDeleveraging {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAutoDeleveraging) Lower(value *AutoDeleveraging) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*AutoDeleveraging")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterAutoDeleveraging) Write(writer io.Writer, value *AutoDeleveraging) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAutoDeleveraging struct{}

func (_ FfiDestroyerAutoDeleveraging) Destroy(value *AutoDeleveraging) {
	value.Destroy()
}

type ChangePubKeyInterface interface {
	GetBytes() []uint8
	GetSignature() ZkLinkSignature
	IsOnchain() bool
	IsSignatureValid() bool
	IsValid() bool
	JsonStr() string
	ToZklinkTx() ZkLinkTx
	TxHash() []uint8
}
type ChangePubKey struct {
	ffiObject FfiObject
}

func NewChangePubKey(builder ChangePubKeyBuilder) *ChangePubKey {
	return FfiConverterChangePubKeyINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_changepubkey_new(FfiConverterChangePubKeyBuilderINSTANCE.Lower(builder), _uniffiStatus)
	}))
}

func (_self *ChangePubKey) GetBytes() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*ChangePubKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_changepubkey_get_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ChangePubKey) GetSignature() ZkLinkSignature {
	_pointer := _self.ffiObject.incrementPointer("*ChangePubKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterZkLinkSignatureINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_changepubkey_get_signature(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ChangePubKey) IsOnchain() bool {
	_pointer := _self.ffiObject.incrementPointer("*ChangePubKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_changepubkey_is_onchain(
			_pointer, _uniffiStatus)
	}))
}

func (_self *ChangePubKey) IsSignatureValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*ChangePubKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_changepubkey_is_signature_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *ChangePubKey) IsValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*ChangePubKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_changepubkey_is_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *ChangePubKey) JsonStr() string {
	_pointer := _self.ffiObject.incrementPointer("*ChangePubKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_changepubkey_json_str(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ChangePubKey) ToZklinkTx() ZkLinkTx {
	_pointer := _self.ffiObject.incrementPointer("*ChangePubKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeZkLinkTxINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_changepubkey_to_zklink_tx(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ChangePubKey) TxHash() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*ChangePubKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_changepubkey_tx_hash(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *ChangePubKey) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterChangePubKey struct{}

var FfiConverterChangePubKeyINSTANCE = FfiConverterChangePubKey{}

func (c FfiConverterChangePubKey) Lift(pointer unsafe.Pointer) *ChangePubKey {
	result := &ChangePubKey{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_changepubkey(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_changepubkey(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*ChangePubKey).Destroy)
	return result
}

func (c FfiConverterChangePubKey) Read(reader io.Reader) *ChangePubKey {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterChangePubKey) Lower(value *ChangePubKey) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*ChangePubKey")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterChangePubKey) Write(writer io.Writer, value *ChangePubKey) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerChangePubKey struct{}

func (_ FfiDestroyerChangePubKey) Destroy(value *ChangePubKey) {
	value.Destroy()
}

type ContractInterface interface {
	CreateSignedContract(zklinkSigner *ZkLinkSigner) (*Contract, error)
	GetBytes() []uint8
	GetSignature() ZkLinkSignature
	IsLong() bool
	IsShort() bool
	IsSignatureValid() bool
}
type Contract struct {
	ffiObject FfiObject
}

func NewContract(builder ContractBuilder) *Contract {
	return FfiConverterContractINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_contract_new(FfiConverterContractBuilderINSTANCE.Lower(builder), _uniffiStatus)
	}))
}

func (_self *Contract) CreateSignedContract(zklinkSigner *ZkLinkSigner) (*Contract, error) {
	_pointer := _self.ffiObject.incrementPointer("*Contract")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_method_contract_create_signed_contract(
			_pointer, FfiConverterZkLinkSignerINSTANCE.Lower(zklinkSigner), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Contract
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterContractINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Contract) GetBytes() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*Contract")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_contract_get_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Contract) GetSignature() ZkLinkSignature {
	_pointer := _self.ffiObject.incrementPointer("*Contract")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterZkLinkSignatureINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_contract_get_signature(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Contract) IsLong() bool {
	_pointer := _self.ffiObject.incrementPointer("*Contract")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_contract_is_long(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Contract) IsShort() bool {
	_pointer := _self.ffiObject.incrementPointer("*Contract")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_contract_is_short(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Contract) IsSignatureValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*Contract")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_contract_is_signature_valid(
			_pointer, _uniffiStatus)
	}))
}
func (object *Contract) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterContract struct{}

var FfiConverterContractINSTANCE = FfiConverterContract{}

func (c FfiConverterContract) Lift(pointer unsafe.Pointer) *Contract {
	result := &Contract{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_contract(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_contract(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Contract).Destroy)
	return result
}

func (c FfiConverterContract) Read(reader io.Reader) *Contract {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterContract) Lower(value *Contract) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Contract")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterContract) Write(writer io.Writer, value *Contract) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerContract struct{}

func (_ FfiDestroyerContract) Destroy(value *Contract) {
	value.Destroy()
}

type ContractMatchingInterface interface {
	CreateSignedTx(signer *ZkLinkSigner) (*ContractMatching, error)
	GetBytes() []uint8
	GetSignature() ZkLinkSignature
	IsSignatureValid() bool
	IsValid() bool
	JsonStr() string
	ToZklinkTx() ZkLinkTx
	TxHash() []uint8
}
type ContractMatching struct {
	ffiObject FfiObject
}

func NewContractMatching(builder ContractMatchingBuilder) *ContractMatching {
	return FfiConverterContractMatchingINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_contractmatching_new(FfiConverterContractMatchingBuilderINSTANCE.Lower(builder), _uniffiStatus)
	}))
}

func (_self *ContractMatching) CreateSignedTx(signer *ZkLinkSigner) (*ContractMatching, error) {
	_pointer := _self.ffiObject.incrementPointer("*ContractMatching")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_method_contractmatching_create_signed_tx(
			_pointer, FfiConverterZkLinkSignerINSTANCE.Lower(signer), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ContractMatching
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterContractMatchingINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *ContractMatching) GetBytes() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*ContractMatching")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_contractmatching_get_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ContractMatching) GetSignature() ZkLinkSignature {
	_pointer := _self.ffiObject.incrementPointer("*ContractMatching")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterZkLinkSignatureINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_contractmatching_get_signature(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ContractMatching) IsSignatureValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*ContractMatching")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_contractmatching_is_signature_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *ContractMatching) IsValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*ContractMatching")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_contractmatching_is_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *ContractMatching) JsonStr() string {
	_pointer := _self.ffiObject.incrementPointer("*ContractMatching")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_contractmatching_json_str(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ContractMatching) ToZklinkTx() ZkLinkTx {
	_pointer := _self.ffiObject.incrementPointer("*ContractMatching")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeZkLinkTxINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_contractmatching_to_zklink_tx(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ContractMatching) TxHash() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*ContractMatching")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_contractmatching_tx_hash(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *ContractMatching) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterContractMatching struct{}

var FfiConverterContractMatchingINSTANCE = FfiConverterContractMatching{}

func (c FfiConverterContractMatching) Lift(pointer unsafe.Pointer) *ContractMatching {
	result := &ContractMatching{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_contractmatching(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_contractmatching(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*ContractMatching).Destroy)
	return result
}

func (c FfiConverterContractMatching) Read(reader io.Reader) *ContractMatching {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterContractMatching) Lower(value *ContractMatching) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*ContractMatching")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterContractMatching) Write(writer io.Writer, value *ContractMatching) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerContractMatching struct{}

func (_ FfiDestroyerContractMatching) Destroy(value *ContractMatching) {
	value.Destroy()
}

type DepositInterface interface {
	GetBytes() []uint8
	JsonStr() string
	TxHash() []uint8
}
type Deposit struct {
	ffiObject FfiObject
}

func NewDeposit(builder DepositBuilder) *Deposit {
	return FfiConverterDepositINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_deposit_new(FfiConverterDepositBuilderINSTANCE.Lower(builder), _uniffiStatus)
	}))
}

func (_self *Deposit) GetBytes() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*Deposit")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_deposit_get_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Deposit) JsonStr() string {
	_pointer := _self.ffiObject.incrementPointer("*Deposit")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_deposit_json_str(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Deposit) TxHash() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*Deposit")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_deposit_tx_hash(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *Deposit) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterDeposit struct{}

var FfiConverterDepositINSTANCE = FfiConverterDeposit{}

func (c FfiConverterDeposit) Lift(pointer unsafe.Pointer) *Deposit {
	result := &Deposit{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_deposit(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_deposit(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Deposit).Destroy)
	return result
}

func (c FfiConverterDeposit) Read(reader io.Reader) *Deposit {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterDeposit) Lower(value *Deposit) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Deposit")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterDeposit) Write(writer io.Writer, value *Deposit) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerDeposit struct{}

func (_ FfiDestroyerDeposit) Destroy(value *Deposit) {
	value.Destroy()
}

type EthSignerInterface interface {
	GetAddress() Address
	SignMessage(message []uint8) (PackedEthSignature, error)
}
type EthSigner struct {
	ffiObject FfiObject
}

func NewEthSigner(privateKey string) (*EthSigner, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[EthSignerError](FfiConverterEthSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_ethsigner_new(FfiConverterStringINSTANCE.Lower(privateKey), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *EthSigner
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterEthSignerINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *EthSigner) GetAddress() Address {
	_pointer := _self.ffiObject.incrementPointer("*EthSigner")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeAddressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_ethsigner_get_address(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *EthSigner) SignMessage(message []uint8) (PackedEthSignature, error) {
	_pointer := _self.ffiObject.incrementPointer("*EthSigner")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[EthSignerError](FfiConverterEthSignerError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_ethsigner_sign_message(
				_pointer, FfiConverterSequenceUint8INSTANCE.Lower(message), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue PackedEthSignature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTypePackedEthSignatureINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *EthSigner) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterEthSigner struct{}

var FfiConverterEthSignerINSTANCE = FfiConverterEthSigner{}

func (c FfiConverterEthSigner) Lift(pointer unsafe.Pointer) *EthSigner {
	result := &EthSigner{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_ethsigner(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_ethsigner(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*EthSigner).Destroy)
	return result
}

func (c FfiConverterEthSigner) Read(reader io.Reader) *EthSigner {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterEthSigner) Lower(value *EthSigner) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*EthSigner")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterEthSigner) Write(writer io.Writer, value *EthSigner) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerEthSigner struct{}

func (_ FfiDestroyerEthSigner) Destroy(value *EthSigner) {
	value.Destroy()
}

type ForcedExitInterface interface {
	CreateSignedTx(signer *ZkLinkSigner) (*ForcedExit, error)
	GetBytes() []uint8
	GetSignature() ZkLinkSignature
	IsSignatureValid() bool
	IsValid() bool
	JsonStr() string
	ToZklinkTx() ZkLinkTx
	TxHash() []uint8
}
type ForcedExit struct {
	ffiObject FfiObject
}

func NewForcedExit(builder ForcedExitBuilder) *ForcedExit {
	return FfiConverterForcedExitINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_forcedexit_new(FfiConverterForcedExitBuilderINSTANCE.Lower(builder), _uniffiStatus)
	}))
}

func (_self *ForcedExit) CreateSignedTx(signer *ZkLinkSigner) (*ForcedExit, error) {
	_pointer := _self.ffiObject.incrementPointer("*ForcedExit")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_method_forcedexit_create_signed_tx(
			_pointer, FfiConverterZkLinkSignerINSTANCE.Lower(signer), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ForcedExit
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterForcedExitINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *ForcedExit) GetBytes() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*ForcedExit")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_forcedexit_get_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ForcedExit) GetSignature() ZkLinkSignature {
	_pointer := _self.ffiObject.incrementPointer("*ForcedExit")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterZkLinkSignatureINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_forcedexit_get_signature(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ForcedExit) IsSignatureValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*ForcedExit")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_forcedexit_is_signature_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *ForcedExit) IsValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*ForcedExit")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_forcedexit_is_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *ForcedExit) JsonStr() string {
	_pointer := _self.ffiObject.incrementPointer("*ForcedExit")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_forcedexit_json_str(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ForcedExit) ToZklinkTx() ZkLinkTx {
	_pointer := _self.ffiObject.incrementPointer("*ForcedExit")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeZkLinkTxINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_forcedexit_to_zklink_tx(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ForcedExit) TxHash() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*ForcedExit")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_forcedexit_tx_hash(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *ForcedExit) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterForcedExit struct{}

var FfiConverterForcedExitINSTANCE = FfiConverterForcedExit{}

func (c FfiConverterForcedExit) Lift(pointer unsafe.Pointer) *ForcedExit {
	result := &ForcedExit{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_forcedexit(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_forcedexit(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*ForcedExit).Destroy)
	return result
}

func (c FfiConverterForcedExit) Read(reader io.Reader) *ForcedExit {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterForcedExit) Lower(value *ForcedExit) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*ForcedExit")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterForcedExit) Write(writer io.Writer, value *ForcedExit) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerForcedExit struct{}

func (_ FfiDestroyerForcedExit) Destroy(value *ForcedExit) {
	value.Destroy()
}

type FullExitInterface interface {
	GetBytes() []uint8
	IsValid() bool
	JsonStr() string
	ToZklinkTx() ZkLinkTx
	TxHash() []uint8
}
type FullExit struct {
	ffiObject FfiObject
}

func NewFullExit(builder FullExitBuilder) *FullExit {
	return FfiConverterFullExitINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_fullexit_new(FfiConverterFullExitBuilderINSTANCE.Lower(builder), _uniffiStatus)
	}))
}

func (_self *FullExit) GetBytes() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*FullExit")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_fullexit_get_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *FullExit) IsValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*FullExit")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_fullexit_is_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *FullExit) JsonStr() string {
	_pointer := _self.ffiObject.incrementPointer("*FullExit")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_fullexit_json_str(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *FullExit) ToZklinkTx() ZkLinkTx {
	_pointer := _self.ffiObject.incrementPointer("*FullExit")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeZkLinkTxINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_fullexit_to_zklink_tx(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *FullExit) TxHash() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*FullExit")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_fullexit_tx_hash(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *FullExit) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterFullExit struct{}

var FfiConverterFullExitINSTANCE = FfiConverterFullExit{}

func (c FfiConverterFullExit) Lift(pointer unsafe.Pointer) *FullExit {
	result := &FullExit{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_fullexit(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_fullexit(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*FullExit).Destroy)
	return result
}

func (c FfiConverterFullExit) Read(reader io.Reader) *FullExit {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterFullExit) Lower(value *FullExit) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*FullExit")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterFullExit) Write(writer io.Writer, value *FullExit) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerFullExit struct{}

func (_ FfiDestroyerFullExit) Destroy(value *FullExit) {
	value.Destroy()
}

type FundingInterface interface {
	CreateSignedTx(signer *ZkLinkSigner) (*Funding, error)
	GetBytes() []uint8
	GetSignature() ZkLinkSignature
	IsSignatureValid() bool
	IsValid() bool
	JsonStr() string
	ToZklinkTx() ZkLinkTx
	TxHash() []uint8
}
type Funding struct {
	ffiObject FfiObject
}

func NewFunding(builder FundingBuilder) *Funding {
	return FfiConverterFundingINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_funding_new(FfiConverterFundingBuilderINSTANCE.Lower(builder), _uniffiStatus)
	}))
}

func (_self *Funding) CreateSignedTx(signer *ZkLinkSigner) (*Funding, error) {
	_pointer := _self.ffiObject.incrementPointer("*Funding")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_method_funding_create_signed_tx(
			_pointer, FfiConverterZkLinkSignerINSTANCE.Lower(signer), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Funding
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterFundingINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Funding) GetBytes() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*Funding")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_funding_get_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Funding) GetSignature() ZkLinkSignature {
	_pointer := _self.ffiObject.incrementPointer("*Funding")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterZkLinkSignatureINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_funding_get_signature(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Funding) IsSignatureValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*Funding")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_funding_is_signature_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Funding) IsValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*Funding")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_funding_is_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Funding) JsonStr() string {
	_pointer := _self.ffiObject.incrementPointer("*Funding")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_funding_json_str(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Funding) ToZklinkTx() ZkLinkTx {
	_pointer := _self.ffiObject.incrementPointer("*Funding")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeZkLinkTxINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_funding_to_zklink_tx(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Funding) TxHash() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*Funding")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_funding_tx_hash(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *Funding) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterFunding struct{}

var FfiConverterFundingINSTANCE = FfiConverterFunding{}

func (c FfiConverterFunding) Lift(pointer unsafe.Pointer) *Funding {
	result := &Funding{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_funding(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_funding(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Funding).Destroy)
	return result
}

func (c FfiConverterFunding) Read(reader io.Reader) *Funding {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterFunding) Lower(value *Funding) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Funding")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterFunding) Write(writer io.Writer, value *Funding) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerFunding struct{}

func (_ FfiDestroyerFunding) Destroy(value *Funding) {
	value.Destroy()
}

type LiquidationInterface interface {
	CreateSignedTx(signer *ZkLinkSigner) (*Liquidation, error)
	GetBytes() []uint8
	GetSignature() ZkLinkSignature
	IsSignatureValid() bool
	IsValid() bool
	JsonStr() string
	ToZklinkTx() ZkLinkTx
	TxHash() []uint8
}
type Liquidation struct {
	ffiObject FfiObject
}

func NewLiquidation(builder LiquidationBuilder) *Liquidation {
	return FfiConverterLiquidationINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_liquidation_new(FfiConverterLiquidationBuilderINSTANCE.Lower(builder), _uniffiStatus)
	}))
}

func (_self *Liquidation) CreateSignedTx(signer *ZkLinkSigner) (*Liquidation, error) {
	_pointer := _self.ffiObject.incrementPointer("*Liquidation")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_method_liquidation_create_signed_tx(
			_pointer, FfiConverterZkLinkSignerINSTANCE.Lower(signer), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Liquidation
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLiquidationINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Liquidation) GetBytes() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*Liquidation")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_liquidation_get_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Liquidation) GetSignature() ZkLinkSignature {
	_pointer := _self.ffiObject.incrementPointer("*Liquidation")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterZkLinkSignatureINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_liquidation_get_signature(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Liquidation) IsSignatureValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*Liquidation")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_liquidation_is_signature_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Liquidation) IsValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*Liquidation")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_liquidation_is_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Liquidation) JsonStr() string {
	_pointer := _self.ffiObject.incrementPointer("*Liquidation")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_liquidation_json_str(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Liquidation) ToZklinkTx() ZkLinkTx {
	_pointer := _self.ffiObject.incrementPointer("*Liquidation")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeZkLinkTxINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_liquidation_to_zklink_tx(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Liquidation) TxHash() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*Liquidation")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_liquidation_tx_hash(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *Liquidation) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLiquidation struct{}

var FfiConverterLiquidationINSTANCE = FfiConverterLiquidation{}

func (c FfiConverterLiquidation) Lift(pointer unsafe.Pointer) *Liquidation {
	result := &Liquidation{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_liquidation(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_liquidation(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Liquidation).Destroy)
	return result
}

func (c FfiConverterLiquidation) Read(reader io.Reader) *Liquidation {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLiquidation) Lower(value *Liquidation) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Liquidation")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterLiquidation) Write(writer io.Writer, value *Liquidation) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLiquidation struct{}

func (_ FfiDestroyerLiquidation) Destroy(value *Liquidation) {
	value.Destroy()
}

type OrderInterface interface {
	CreateSignedOrder(zklinkSigner *ZkLinkSigner) (*Order, error)
	GetBytes() []uint8
	GetEthSignMsg(quoteToken string, basedToken string, decimals uint8) string
	GetSignature() ZkLinkSignature
	IsSignatureValid() bool
	IsValid() bool
	JsonStr() string
}
type Order struct {
	ffiObject FfiObject
}

func NewOrder(accountId AccountId, subAccountId SubAccountId, slotId SlotId, nonce Nonce, baseTokenId TokenId, quoteTokenId TokenId, amount BigUint, price BigUint, isSell bool, hasSubsidy bool, makerFeeRate uint8, takerFeeRate uint8, signature *ZkLinkSignature) *Order {
	return FfiConverterOrderINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_order_new(FfiConverterTypeAccountIdINSTANCE.Lower(accountId), FfiConverterTypeSubAccountIdINSTANCE.Lower(subAccountId), FfiConverterTypeSlotIdINSTANCE.Lower(slotId), FfiConverterTypeNonceINSTANCE.Lower(nonce), FfiConverterTypeTokenIdINSTANCE.Lower(baseTokenId), FfiConverterTypeTokenIdINSTANCE.Lower(quoteTokenId), FfiConverterTypeBigUintINSTANCE.Lower(amount).ExternalBuffer(), FfiConverterTypeBigUintINSTANCE.Lower(price).ExternalBuffer(), FfiConverterBoolINSTANCE.Lower(isSell), FfiConverterBoolINSTANCE.Lower(hasSubsidy), FfiConverterUint8INSTANCE.Lower(makerFeeRate), FfiConverterUint8INSTANCE.Lower(takerFeeRate), FfiConverterOptionalZkLinkSignatureINSTANCE.Lower(signature), _uniffiStatus)
	}))
}

func (_self *Order) CreateSignedOrder(zklinkSigner *ZkLinkSigner) (*Order, error) {
	_pointer := _self.ffiObject.incrementPointer("*Order")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_method_order_create_signed_order(
			_pointer, FfiConverterZkLinkSignerINSTANCE.Lower(zklinkSigner), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Order
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOrderINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Order) GetBytes() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*Order")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_order_get_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Order) GetEthSignMsg(quoteToken string, basedToken string, decimals uint8) string {
	_pointer := _self.ffiObject.incrementPointer("*Order")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_order_get_eth_sign_msg(
				_pointer, FfiConverterStringINSTANCE.Lower(quoteToken), FfiConverterStringINSTANCE.Lower(basedToken), FfiConverterUint8INSTANCE.Lower(decimals), _uniffiStatus),
		}
	}))
}

func (_self *Order) GetSignature() ZkLinkSignature {
	_pointer := _self.ffiObject.incrementPointer("*Order")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterZkLinkSignatureINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_order_get_signature(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Order) IsSignatureValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*Order")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_order_is_signature_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Order) IsValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*Order")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_order_is_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Order) JsonStr() string {
	_pointer := _self.ffiObject.incrementPointer("*Order")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_order_json_str(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *Order) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterOrder struct{}

var FfiConverterOrderINSTANCE = FfiConverterOrder{}

func (c FfiConverterOrder) Lift(pointer unsafe.Pointer) *Order {
	result := &Order{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_order(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_order(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Order).Destroy)
	return result
}

func (c FfiConverterOrder) Read(reader io.Reader) *Order {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterOrder) Lower(value *Order) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Order")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterOrder) Write(writer io.Writer, value *Order) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerOrder struct{}

func (_ FfiDestroyerOrder) Destroy(value *Order) {
	value.Destroy()
}

type OrderMatchingInterface interface {
	CreateSignedTx(signer *ZkLinkSigner) (*OrderMatching, error)
	GetBytes() []uint8
	GetSignature() ZkLinkSignature
	IsSignatureValid() bool
	IsValid() bool
	JsonStr() string
	ToZklinkTx() ZkLinkTx
	TxHash() []uint8
}
type OrderMatching struct {
	ffiObject FfiObject
}

func NewOrderMatching(builder OrderMatchingBuilder) *OrderMatching {
	return FfiConverterOrderMatchingINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_ordermatching_new(FfiConverterOrderMatchingBuilderINSTANCE.Lower(builder), _uniffiStatus)
	}))
}

func (_self *OrderMatching) CreateSignedTx(signer *ZkLinkSigner) (*OrderMatching, error) {
	_pointer := _self.ffiObject.incrementPointer("*OrderMatching")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_method_ordermatching_create_signed_tx(
			_pointer, FfiConverterZkLinkSignerINSTANCE.Lower(signer), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *OrderMatching
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOrderMatchingINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *OrderMatching) GetBytes() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*OrderMatching")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_ordermatching_get_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *OrderMatching) GetSignature() ZkLinkSignature {
	_pointer := _self.ffiObject.incrementPointer("*OrderMatching")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterZkLinkSignatureINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_ordermatching_get_signature(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *OrderMatching) IsSignatureValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*OrderMatching")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_ordermatching_is_signature_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *OrderMatching) IsValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*OrderMatching")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_ordermatching_is_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *OrderMatching) JsonStr() string {
	_pointer := _self.ffiObject.incrementPointer("*OrderMatching")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_ordermatching_json_str(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *OrderMatching) ToZklinkTx() ZkLinkTx {
	_pointer := _self.ffiObject.incrementPointer("*OrderMatching")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeZkLinkTxINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_ordermatching_to_zklink_tx(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *OrderMatching) TxHash() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*OrderMatching")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_ordermatching_tx_hash(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *OrderMatching) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterOrderMatching struct{}

var FfiConverterOrderMatchingINSTANCE = FfiConverterOrderMatching{}

func (c FfiConverterOrderMatching) Lift(pointer unsafe.Pointer) *OrderMatching {
	result := &OrderMatching{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_ordermatching(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_ordermatching(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*OrderMatching).Destroy)
	return result
}

func (c FfiConverterOrderMatching) Read(reader io.Reader) *OrderMatching {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterOrderMatching) Lower(value *OrderMatching) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*OrderMatching")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterOrderMatching) Write(writer io.Writer, value *OrderMatching) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerOrderMatching struct{}

func (_ FfiDestroyerOrderMatching) Destroy(value *OrderMatching) {
	value.Destroy()
}

type SignerInterface interface {
	SignAutoDeleveraging(tx *AutoDeleveraging) (TxSignature, error)
	SignChangePubkeyWithCreate2dataAuth(tx *ChangePubKey, crate2data Create2Data) (TxSignature, error)
	SignChangePubkeyWithEthEcdsaAuth(tx *ChangePubKey) (TxSignature, error)
	SignChangePubkeyWithOnchainAuthData(tx *ChangePubKey) (TxSignature, error)
	SignContractMatching(tx *ContractMatching) (TxSignature, error)
	SignForcedExit(tx *ForcedExit) (TxSignature, error)
	SignFunding(tx *Funding) (TxSignature, error)
	SignLiquidation(tx *Liquidation) (TxSignature, error)
	SignOrderMatching(tx *OrderMatching) (TxSignature, error)
	SignTransfer(tx *Transfer, tokenSybmol string, chainId *string, addr *string) (TxSignature, error)
	SignWithdraw(tx *Withdraw, l2SourceTokenSymbol string, chainId *string, addr *string) (TxSignature, error)
}
type Signer struct {
	ffiObject FfiObject
}

func NewSigner(privateKey string, l1Type L1SignerType) (*Signer, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[SignError](FfiConverterSignError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_signer_new(FfiConverterStringINSTANCE.Lower(privateKey), FfiConverterL1SignerTypeINSTANCE.Lower(l1Type), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Signer
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSignerINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Signer) SignAutoDeleveraging(tx *AutoDeleveraging) (TxSignature, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[SignError](FfiConverterSignError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_signer_sign_auto_deleveraging(
				_pointer, FfiConverterAutoDeleveragingINSTANCE.Lower(tx), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue TxSignature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxSignatureINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Signer) SignChangePubkeyWithCreate2dataAuth(tx *ChangePubKey, crate2data Create2Data) (TxSignature, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[SignError](FfiConverterSignError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_signer_sign_change_pubkey_with_create2data_auth(
				_pointer, FfiConverterChangePubKeyINSTANCE.Lower(tx), FfiConverterCreate2DataINSTANCE.Lower(crate2data), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue TxSignature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxSignatureINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Signer) SignChangePubkeyWithEthEcdsaAuth(tx *ChangePubKey) (TxSignature, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[SignError](FfiConverterSignError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_signer_sign_change_pubkey_with_eth_ecdsa_auth(
				_pointer, FfiConverterChangePubKeyINSTANCE.Lower(tx), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue TxSignature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxSignatureINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Signer) SignChangePubkeyWithOnchainAuthData(tx *ChangePubKey) (TxSignature, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[SignError](FfiConverterSignError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_signer_sign_change_pubkey_with_onchain_auth_data(
				_pointer, FfiConverterChangePubKeyINSTANCE.Lower(tx), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue TxSignature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxSignatureINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Signer) SignContractMatching(tx *ContractMatching) (TxSignature, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[SignError](FfiConverterSignError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_signer_sign_contract_matching(
				_pointer, FfiConverterContractMatchingINSTANCE.Lower(tx), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue TxSignature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxSignatureINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Signer) SignForcedExit(tx *ForcedExit) (TxSignature, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[SignError](FfiConverterSignError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_signer_sign_forced_exit(
				_pointer, FfiConverterForcedExitINSTANCE.Lower(tx), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue TxSignature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxSignatureINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Signer) SignFunding(tx *Funding) (TxSignature, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[SignError](FfiConverterSignError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_signer_sign_funding(
				_pointer, FfiConverterFundingINSTANCE.Lower(tx), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue TxSignature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxSignatureINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Signer) SignLiquidation(tx *Liquidation) (TxSignature, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[SignError](FfiConverterSignError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_signer_sign_liquidation(
				_pointer, FfiConverterLiquidationINSTANCE.Lower(tx), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue TxSignature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxSignatureINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Signer) SignOrderMatching(tx *OrderMatching) (TxSignature, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[SignError](FfiConverterSignError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_signer_sign_order_matching(
				_pointer, FfiConverterOrderMatchingINSTANCE.Lower(tx), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue TxSignature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxSignatureINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Signer) SignTransfer(tx *Transfer, tokenSybmol string, chainId *string, addr *string) (TxSignature, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[SignError](FfiConverterSignError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_signer_sign_transfer(
				_pointer, FfiConverterTransferINSTANCE.Lower(tx), FfiConverterStringINSTANCE.Lower(tokenSybmol), FfiConverterOptionalStringINSTANCE.Lower(chainId), FfiConverterOptionalStringINSTANCE.Lower(addr), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue TxSignature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxSignatureINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Signer) SignWithdraw(tx *Withdraw, l2SourceTokenSymbol string, chainId *string, addr *string) (TxSignature, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[SignError](FfiConverterSignError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_signer_sign_withdraw(
				_pointer, FfiConverterWithdrawINSTANCE.Lower(tx), FfiConverterStringINSTANCE.Lower(l2SourceTokenSymbol), FfiConverterOptionalStringINSTANCE.Lower(chainId), FfiConverterOptionalStringINSTANCE.Lower(addr), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue TxSignature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxSignatureINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *Signer) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterSigner struct{}

var FfiConverterSignerINSTANCE = FfiConverterSigner{}

func (c FfiConverterSigner) Lift(pointer unsafe.Pointer) *Signer {
	result := &Signer{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_signer(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_signer(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Signer).Destroy)
	return result
}

func (c FfiConverterSigner) Read(reader io.Reader) *Signer {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterSigner) Lower(value *Signer) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Signer")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterSigner) Write(writer io.Writer, value *Signer) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerSigner struct{}

func (_ FfiDestroyerSigner) Destroy(value *Signer) {
	value.Destroy()
}

type StarkSignerInterface interface {
	SignMessage(typedData *TypedData, addr string) (StarkEip712Signature, error)
}
type StarkSigner struct {
	ffiObject FfiObject
}

func NewStarkSigner() *StarkSigner {
	return FfiConverterStarkSignerINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_starksigner_new(_uniffiStatus)
	}))
}

func StarkSignerNewFromHexStr(hexStr string) (*StarkSigner, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[StarkSignerError](FfiConverterStarkSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_starksigner_new_from_hex_str(FfiConverterStringINSTANCE.Lower(hexStr), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *StarkSigner
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStarkSignerINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *StarkSigner) SignMessage(typedData *TypedData, addr string) (StarkEip712Signature, error) {
	_pointer := _self.ffiObject.incrementPointer("*StarkSigner")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[StarkSignerError](FfiConverterStarkSignerError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_starksigner_sign_message(
				_pointer, FfiConverterTypedDataINSTANCE.Lower(typedData), FfiConverterStringINSTANCE.Lower(addr), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue StarkEip712Signature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTypeStarkEip712SignatureINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *StarkSigner) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterStarkSigner struct{}

var FfiConverterStarkSignerINSTANCE = FfiConverterStarkSigner{}

func (c FfiConverterStarkSigner) Lift(pointer unsafe.Pointer) *StarkSigner {
	result := &StarkSigner{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_starksigner(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_starksigner(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*StarkSigner).Destroy)
	return result
}

func (c FfiConverterStarkSigner) Read(reader io.Reader) *StarkSigner {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterStarkSigner) Lower(value *StarkSigner) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*StarkSigner")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterStarkSigner) Write(writer io.Writer, value *StarkSigner) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerStarkSigner struct{}

func (_ FfiDestroyerStarkSigner) Destroy(value *StarkSigner) {
	value.Destroy()
}

type TransferInterface interface {
	CreateSignedTx(signer *ZkLinkSigner) (*Transfer, error)
	EthSignature(ethSigner *EthSigner, tokenSymbol string) (TxLayer1Signature, error)
	GetBytes() []uint8
	GetEthSignMsg(tokenSymbol string) string
	GetSignature() ZkLinkSignature
	IsSignatureValid() bool
	IsValid() bool
	JsonStr() string
	ToZklinkTx() ZkLinkTx
	TxHash() []uint8
}
type Transfer struct {
	ffiObject FfiObject
}

func NewTransfer(builder TransferBuilder) *Transfer {
	return FfiConverterTransferINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_transfer_new(FfiConverterTransferBuilderINSTANCE.Lower(builder), _uniffiStatus)
	}))
}

func (_self *Transfer) CreateSignedTx(signer *ZkLinkSigner) (*Transfer, error) {
	_pointer := _self.ffiObject.incrementPointer("*Transfer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_method_transfer_create_signed_tx(
			_pointer, FfiConverterZkLinkSignerINSTANCE.Lower(signer), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Transfer
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTransferINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Transfer) EthSignature(ethSigner *EthSigner, tokenSymbol string) (TxLayer1Signature, error) {
	_pointer := _self.ffiObject.incrementPointer("*Transfer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_transfer_eth_signature(
				_pointer, FfiConverterEthSignerINSTANCE.Lower(ethSigner), FfiConverterStringINSTANCE.Lower(tokenSymbol), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue TxLayer1Signature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTypeTxLayer1SignatureINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Transfer) GetBytes() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*Transfer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_transfer_get_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Transfer) GetEthSignMsg(tokenSymbol string) string {
	_pointer := _self.ffiObject.incrementPointer("*Transfer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_transfer_get_eth_sign_msg(
				_pointer, FfiConverterStringINSTANCE.Lower(tokenSymbol), _uniffiStatus),
		}
	}))
}

func (_self *Transfer) GetSignature() ZkLinkSignature {
	_pointer := _self.ffiObject.incrementPointer("*Transfer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterZkLinkSignatureINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_transfer_get_signature(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Transfer) IsSignatureValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*Transfer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_transfer_is_signature_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Transfer) IsValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*Transfer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_transfer_is_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Transfer) JsonStr() string {
	_pointer := _self.ffiObject.incrementPointer("*Transfer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_transfer_json_str(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Transfer) ToZklinkTx() ZkLinkTx {
	_pointer := _self.ffiObject.incrementPointer("*Transfer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeZkLinkTxINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_transfer_to_zklink_tx(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Transfer) TxHash() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*Transfer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_transfer_tx_hash(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *Transfer) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterTransfer struct{}

var FfiConverterTransferINSTANCE = FfiConverterTransfer{}

func (c FfiConverterTransfer) Lift(pointer unsafe.Pointer) *Transfer {
	result := &Transfer{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_transfer(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_transfer(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Transfer).Destroy)
	return result
}

func (c FfiConverterTransfer) Read(reader io.Reader) *Transfer {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterTransfer) Lower(value *Transfer) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Transfer")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterTransfer) Write(writer io.Writer, value *Transfer) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerTransfer struct{}

func (_ FfiDestroyerTransfer) Destroy(value *Transfer) {
	value.Destroy()
}

type TypedDataInterface interface {
}
type TypedData struct {
	ffiObject FfiObject
}

func NewTypedData(message TypedDataMessage, chainId string) *TypedData {
	return FfiConverterTypedDataINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_typeddata_new(FfiConverterTypedDataMessageINSTANCE.Lower(message), FfiConverterStringINSTANCE.Lower(chainId), _uniffiStatus)
	}))
}

func (object *TypedData) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterTypedData struct{}

var FfiConverterTypedDataINSTANCE = FfiConverterTypedData{}

func (c FfiConverterTypedData) Lift(pointer unsafe.Pointer) *TypedData {
	result := &TypedData{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_typeddata(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_typeddata(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*TypedData).Destroy)
	return result
}

func (c FfiConverterTypedData) Read(reader io.Reader) *TypedData {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterTypedData) Lower(value *TypedData) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*TypedData")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterTypedData) Write(writer io.Writer, value *TypedData) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerTypedData struct{}

func (_ FfiDestroyerTypedData) Destroy(value *TypedData) {
	value.Destroy()
}

type UpdateGlobalVarInterface interface {
	GetBytes() []uint8
	IsValid() bool
	JsonStr() string
	ToZklinkTx() ZkLinkTx
	TxHash() []uint8
}
type UpdateGlobalVar struct {
	ffiObject FfiObject
}

func NewUpdateGlobalVar(builder UpdateGlobalVarBuilder) *UpdateGlobalVar {
	return FfiConverterUpdateGlobalVarINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_updateglobalvar_new(FfiConverterUpdateGlobalVarBuilderINSTANCE.Lower(builder), _uniffiStatus)
	}))
}

func (_self *UpdateGlobalVar) GetBytes() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*UpdateGlobalVar")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_updateglobalvar_get_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *UpdateGlobalVar) IsValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*UpdateGlobalVar")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_updateglobalvar_is_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *UpdateGlobalVar) JsonStr() string {
	_pointer := _self.ffiObject.incrementPointer("*UpdateGlobalVar")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_updateglobalvar_json_str(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *UpdateGlobalVar) ToZklinkTx() ZkLinkTx {
	_pointer := _self.ffiObject.incrementPointer("*UpdateGlobalVar")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeZkLinkTxINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_updateglobalvar_to_zklink_tx(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *UpdateGlobalVar) TxHash() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*UpdateGlobalVar")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_updateglobalvar_tx_hash(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *UpdateGlobalVar) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterUpdateGlobalVar struct{}

var FfiConverterUpdateGlobalVarINSTANCE = FfiConverterUpdateGlobalVar{}

func (c FfiConverterUpdateGlobalVar) Lift(pointer unsafe.Pointer) *UpdateGlobalVar {
	result := &UpdateGlobalVar{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_updateglobalvar(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_updateglobalvar(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*UpdateGlobalVar).Destroy)
	return result
}

func (c FfiConverterUpdateGlobalVar) Read(reader io.Reader) *UpdateGlobalVar {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterUpdateGlobalVar) Lower(value *UpdateGlobalVar) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*UpdateGlobalVar")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterUpdateGlobalVar) Write(writer io.Writer, value *UpdateGlobalVar) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerUpdateGlobalVar struct{}

func (_ FfiDestroyerUpdateGlobalVar) Destroy(value *UpdateGlobalVar) {
	value.Destroy()
}

type WithdrawInterface interface {
	CreateSignedTx(signer *ZkLinkSigner) (*Withdraw, error)
	EthSignature(ethSigner *EthSigner, l2SourceTokenSymbol string) (PackedEthSignature, error)
	GetBytes() []uint8
	GetEthSignMsg(tokenSymbol string) string
	GetSignature() ZkLinkSignature
	IsSignatureValid() bool
	IsValid() bool
	JsonStr() string
	ToZklinkTx() ZkLinkTx
	TxHash() []uint8
}
type Withdraw struct {
	ffiObject FfiObject
}

func NewWithdraw(builder WithdrawBuilder) *Withdraw {
	return FfiConverterWithdrawINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_withdraw_new(FfiConverterWithdrawBuilderINSTANCE.Lower(builder), _uniffiStatus)
	}))
}

func (_self *Withdraw) CreateSignedTx(signer *ZkLinkSigner) (*Withdraw, error) {
	_pointer := _self.ffiObject.incrementPointer("*Withdraw")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_method_withdraw_create_signed_tx(
			_pointer, FfiConverterZkLinkSignerINSTANCE.Lower(signer), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Withdraw
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWithdrawINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Withdraw) EthSignature(ethSigner *EthSigner, l2SourceTokenSymbol string) (PackedEthSignature, error) {
	_pointer := _self.ffiObject.incrementPointer("*Withdraw")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_withdraw_eth_signature(
				_pointer, FfiConverterEthSignerINSTANCE.Lower(ethSigner), FfiConverterStringINSTANCE.Lower(l2SourceTokenSymbol), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue PackedEthSignature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTypePackedEthSignatureINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Withdraw) GetBytes() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*Withdraw")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_withdraw_get_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Withdraw) GetEthSignMsg(tokenSymbol string) string {
	_pointer := _self.ffiObject.incrementPointer("*Withdraw")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_withdraw_get_eth_sign_msg(
				_pointer, FfiConverterStringINSTANCE.Lower(tokenSymbol), _uniffiStatus),
		}
	}))
}

func (_self *Withdraw) GetSignature() ZkLinkSignature {
	_pointer := _self.ffiObject.incrementPointer("*Withdraw")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterZkLinkSignatureINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_withdraw_get_signature(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Withdraw) IsSignatureValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*Withdraw")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_withdraw_is_signature_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Withdraw) IsValid() bool {
	_pointer := _self.ffiObject.incrementPointer("*Withdraw")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_method_withdraw_is_valid(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Withdraw) JsonStr() string {
	_pointer := _self.ffiObject.incrementPointer("*Withdraw")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_withdraw_json_str(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Withdraw) ToZklinkTx() ZkLinkTx {
	_pointer := _self.ffiObject.incrementPointer("*Withdraw")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeZkLinkTxINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_withdraw_to_zklink_tx(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Withdraw) TxHash() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*Withdraw")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_withdraw_tx_hash(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *Withdraw) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterWithdraw struct{}

var FfiConverterWithdrawINSTANCE = FfiConverterWithdraw{}

func (c FfiConverterWithdraw) Lift(pointer unsafe.Pointer) *Withdraw {
	result := &Withdraw{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_withdraw(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_withdraw(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Withdraw).Destroy)
	return result
}

func (c FfiConverterWithdraw) Read(reader io.Reader) *Withdraw {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterWithdraw) Lower(value *Withdraw) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Withdraw")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterWithdraw) Write(writer io.Writer, value *Withdraw) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerWithdraw struct{}

func (_ FfiDestroyerWithdraw) Destroy(value *Withdraw) {
	value.Destroy()
}

type ZkLinkSignerInterface interface {
	PublicKey() PackedPublicKey
	SignMusig(msg []uint8) (ZkLinkSignature, error)
}
type ZkLinkSigner struct {
	ffiObject FfiObject
}

func NewZkLinkSigner() (*ZkLinkSigner, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_zklinksigner_new(_uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ZkLinkSigner
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterZkLinkSignerINSTANCE.Lift(_uniffiRV), nil
	}
}

func ZkLinkSignerNewFromBytes(slice []uint8) (*ZkLinkSigner, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_zklinksigner_new_from_bytes(FfiConverterSequenceUint8INSTANCE.Lower(slice), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ZkLinkSigner
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterZkLinkSignerINSTANCE.Lift(_uniffiRV), nil
	}
}

func ZkLinkSignerNewFromHexEthSigner(ethHexPrivateKey string) (*ZkLinkSigner, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_zklinksigner_new_from_hex_eth_signer(FfiConverterStringINSTANCE.Lower(ethHexPrivateKey), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ZkLinkSigner
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterZkLinkSignerINSTANCE.Lift(_uniffiRV), nil
	}
}

func ZkLinkSignerNewFromHexStarkSigner(hexPrivateKey string, addr string, chainId string) (*ZkLinkSigner, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_zklinksigner_new_from_hex_stark_signer(FfiConverterStringINSTANCE.Lower(hexPrivateKey), FfiConverterStringINSTANCE.Lower(addr), FfiConverterStringINSTANCE.Lower(chainId), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ZkLinkSigner
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterZkLinkSignerINSTANCE.Lift(_uniffiRV), nil
	}
}

func ZkLinkSignerNewFromSeed(seed []uint8) (*ZkLinkSigner, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_constructor_zklinksigner_new_from_seed(FfiConverterSequenceUint8INSTANCE.Lower(seed), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ZkLinkSigner
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterZkLinkSignerINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *ZkLinkSigner) PublicKey() PackedPublicKey {
	_pointer := _self.ffiObject.incrementPointer("*ZkLinkSigner")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypePackedPublicKeyINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_zklinksigner_public_key(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ZkLinkSigner) SignMusig(msg []uint8) (ZkLinkSignature, error) {
	_pointer := _self.ffiObject.incrementPointer("*ZkLinkSigner")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[ZkSignerError](FfiConverterZkSignerError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_method_zklinksigner_sign_musig(
				_pointer, FfiConverterSequenceUint8INSTANCE.Lower(msg), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue ZkLinkSignature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterZkLinkSignatureINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *ZkLinkSigner) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterZkLinkSigner struct{}

var FfiConverterZkLinkSignerINSTANCE = FfiConverterZkLinkSigner{}

func (c FfiConverterZkLinkSigner) Lift(pointer unsafe.Pointer) *ZkLinkSigner {
	result := &ZkLinkSigner{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_zklink_sdk_fn_clone_zklinksigner(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_zklink_sdk_fn_free_zklinksigner(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*ZkLinkSigner).Destroy)
	return result
}

func (c FfiConverterZkLinkSigner) Read(reader io.Reader) *ZkLinkSigner {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterZkLinkSigner) Lower(value *ZkLinkSigner) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*ZkLinkSigner")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterZkLinkSigner) Write(writer io.Writer, value *ZkLinkSigner) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerZkLinkSigner struct{}

func (_ FfiDestroyerZkLinkSigner) Destroy(value *ZkLinkSigner) {
	value.Destroy()
}

type AutoDeleveragingBuilder struct {
	AccountId       AccountId
	SubAccountId    SubAccountId
	SubAccountNonce Nonce
	ContractPrices  []ContractPrice
	MarginPrices    []SpotPriceInfo
	AdlAccountId    AccountId
	PairId          PairId
	AdlSize         BigUint
	AdlPrice        BigUint
	Fee             BigUint
	FeeToken        TokenId
}

func (r *AutoDeleveragingBuilder) Destroy() {
	FfiDestroyerTypeAccountId{}.Destroy(r.AccountId)
	FfiDestroyerTypeSubAccountId{}.Destroy(r.SubAccountId)
	FfiDestroyerTypeNonce{}.Destroy(r.SubAccountNonce)
	FfiDestroyerSequenceContractPrice{}.Destroy(r.ContractPrices)
	FfiDestroyerSequenceSpotPriceInfo{}.Destroy(r.MarginPrices)
	FfiDestroyerTypeAccountId{}.Destroy(r.AdlAccountId)
	FfiDestroyerTypePairId{}.Destroy(r.PairId)
	FfiDestroyerTypeBigUint{}.Destroy(r.AdlSize)
	FfiDestroyerTypeBigUint{}.Destroy(r.AdlPrice)
	FfiDestroyerTypeBigUint{}.Destroy(r.Fee)
	FfiDestroyerTypeTokenId{}.Destroy(r.FeeToken)
}

type FfiConverterAutoDeleveragingBuilder struct{}

var FfiConverterAutoDeleveragingBuilderINSTANCE = FfiConverterAutoDeleveragingBuilder{}

func (c FfiConverterAutoDeleveragingBuilder) Lift(rb RustBufferI) AutoDeleveragingBuilder {
	return LiftFromRustBuffer[AutoDeleveragingBuilder](c, rb)
}

func (c FfiConverterAutoDeleveragingBuilder) Read(reader io.Reader) AutoDeleveragingBuilder {
	return AutoDeleveragingBuilder{
		FfiConverterTypeAccountIdINSTANCE.Read(reader),
		FfiConverterTypeSubAccountIdINSTANCE.Read(reader),
		FfiConverterTypeNonceINSTANCE.Read(reader),
		FfiConverterSequenceContractPriceINSTANCE.Read(reader),
		FfiConverterSequenceSpotPriceInfoINSTANCE.Read(reader),
		FfiConverterTypeAccountIdINSTANCE.Read(reader),
		FfiConverterTypePairIdINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterTypeTokenIdINSTANCE.Read(reader),
	}
}

func (c FfiConverterAutoDeleveragingBuilder) Lower(value AutoDeleveragingBuilder) C.RustBuffer {
	return LowerIntoRustBuffer[AutoDeleveragingBuilder](c, value)
}

func (c FfiConverterAutoDeleveragingBuilder) Write(writer io.Writer, value AutoDeleveragingBuilder) {
	FfiConverterTypeAccountIdINSTANCE.Write(writer, value.AccountId)
	FfiConverterTypeSubAccountIdINSTANCE.Write(writer, value.SubAccountId)
	FfiConverterTypeNonceINSTANCE.Write(writer, value.SubAccountNonce)
	FfiConverterSequenceContractPriceINSTANCE.Write(writer, value.ContractPrices)
	FfiConverterSequenceSpotPriceInfoINSTANCE.Write(writer, value.MarginPrices)
	FfiConverterTypeAccountIdINSTANCE.Write(writer, value.AdlAccountId)
	FfiConverterTypePairIdINSTANCE.Write(writer, value.PairId)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.AdlSize)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.AdlPrice)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.Fee)
	FfiConverterTypeTokenIdINSTANCE.Write(writer, value.FeeToken)
}

type FfiDestroyerAutoDeleveragingBuilder struct{}

func (_ FfiDestroyerAutoDeleveragingBuilder) Destroy(value AutoDeleveragingBuilder) {
	value.Destroy()
}

type ChangePubKeyBuilder struct {
	ChainId       ChainId
	AccountId     AccountId
	SubAccountId  SubAccountId
	NewPubkeyHash PubKeyHash
	FeeToken      TokenId
	Fee           BigUint
	Nonce         Nonce
	EthSignature  *PackedEthSignature
	Timestamp     TimeStamp
}

func (r *ChangePubKeyBuilder) Destroy() {
	FfiDestroyerTypeChainId{}.Destroy(r.ChainId)
	FfiDestroyerTypeAccountId{}.Destroy(r.AccountId)
	FfiDestroyerTypeSubAccountId{}.Destroy(r.SubAccountId)
	FfiDestroyerTypePubKeyHash{}.Destroy(r.NewPubkeyHash)
	FfiDestroyerTypeTokenId{}.Destroy(r.FeeToken)
	FfiDestroyerTypeBigUint{}.Destroy(r.Fee)
	FfiDestroyerTypeNonce{}.Destroy(r.Nonce)
	FfiDestroyerOptionalTypePackedEthSignature{}.Destroy(r.EthSignature)
	FfiDestroyerTypeTimeStamp{}.Destroy(r.Timestamp)
}

type FfiConverterChangePubKeyBuilder struct{}

var FfiConverterChangePubKeyBuilderINSTANCE = FfiConverterChangePubKeyBuilder{}

func (c FfiConverterChangePubKeyBuilder) Lift(rb RustBufferI) ChangePubKeyBuilder {
	return LiftFromRustBuffer[ChangePubKeyBuilder](c, rb)
}

func (c FfiConverterChangePubKeyBuilder) Read(reader io.Reader) ChangePubKeyBuilder {
	return ChangePubKeyBuilder{
		FfiConverterTypeChainIdINSTANCE.Read(reader),
		FfiConverterTypeAccountIdINSTANCE.Read(reader),
		FfiConverterTypeSubAccountIdINSTANCE.Read(reader),
		FfiConverterTypePubKeyHashINSTANCE.Read(reader),
		FfiConverterTypeTokenIdINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterTypeNonceINSTANCE.Read(reader),
		FfiConverterOptionalTypePackedEthSignatureINSTANCE.Read(reader),
		FfiConverterTypeTimeStampINSTANCE.Read(reader),
	}
}

func (c FfiConverterChangePubKeyBuilder) Lower(value ChangePubKeyBuilder) C.RustBuffer {
	return LowerIntoRustBuffer[ChangePubKeyBuilder](c, value)
}

func (c FfiConverterChangePubKeyBuilder) Write(writer io.Writer, value ChangePubKeyBuilder) {
	FfiConverterTypeChainIdINSTANCE.Write(writer, value.ChainId)
	FfiConverterTypeAccountIdINSTANCE.Write(writer, value.AccountId)
	FfiConverterTypeSubAccountIdINSTANCE.Write(writer, value.SubAccountId)
	FfiConverterTypePubKeyHashINSTANCE.Write(writer, value.NewPubkeyHash)
	FfiConverterTypeTokenIdINSTANCE.Write(writer, value.FeeToken)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.Fee)
	FfiConverterTypeNonceINSTANCE.Write(writer, value.Nonce)
	FfiConverterOptionalTypePackedEthSignatureINSTANCE.Write(writer, value.EthSignature)
	FfiConverterTypeTimeStampINSTANCE.Write(writer, value.Timestamp)
}

type FfiDestroyerChangePubKeyBuilder struct{}

func (_ FfiDestroyerChangePubKeyBuilder) Destroy(value ChangePubKeyBuilder) {
	value.Destroy()
}

type ContractBuilder struct {
	AccountId    AccountId
	SubAccountId SubAccountId
	SlotId       SlotId
	Nonce        Nonce
	PairId       PairId
	Size         BigUint
	Price        BigUint
	Direction    bool
	TakerFeeRate uint8
	MakerFeeRate uint8
	HasSubsidy   bool
}

func (r *ContractBuilder) Destroy() {
	FfiDestroyerTypeAccountId{}.Destroy(r.AccountId)
	FfiDestroyerTypeSubAccountId{}.Destroy(r.SubAccountId)
	FfiDestroyerTypeSlotId{}.Destroy(r.SlotId)
	FfiDestroyerTypeNonce{}.Destroy(r.Nonce)
	FfiDestroyerTypePairId{}.Destroy(r.PairId)
	FfiDestroyerTypeBigUint{}.Destroy(r.Size)
	FfiDestroyerTypeBigUint{}.Destroy(r.Price)
	FfiDestroyerBool{}.Destroy(r.Direction)
	FfiDestroyerUint8{}.Destroy(r.TakerFeeRate)
	FfiDestroyerUint8{}.Destroy(r.MakerFeeRate)
	FfiDestroyerBool{}.Destroy(r.HasSubsidy)
}

type FfiConverterContractBuilder struct{}

var FfiConverterContractBuilderINSTANCE = FfiConverterContractBuilder{}

func (c FfiConverterContractBuilder) Lift(rb RustBufferI) ContractBuilder {
	return LiftFromRustBuffer[ContractBuilder](c, rb)
}

func (c FfiConverterContractBuilder) Read(reader io.Reader) ContractBuilder {
	return ContractBuilder{
		FfiConverterTypeAccountIdINSTANCE.Read(reader),
		FfiConverterTypeSubAccountIdINSTANCE.Read(reader),
		FfiConverterTypeSlotIdINSTANCE.Read(reader),
		FfiConverterTypeNonceINSTANCE.Read(reader),
		FfiConverterTypePairIdINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterUint8INSTANCE.Read(reader),
		FfiConverterUint8INSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
	}
}

func (c FfiConverterContractBuilder) Lower(value ContractBuilder) C.RustBuffer {
	return LowerIntoRustBuffer[ContractBuilder](c, value)
}

func (c FfiConverterContractBuilder) Write(writer io.Writer, value ContractBuilder) {
	FfiConverterTypeAccountIdINSTANCE.Write(writer, value.AccountId)
	FfiConverterTypeSubAccountIdINSTANCE.Write(writer, value.SubAccountId)
	FfiConverterTypeSlotIdINSTANCE.Write(writer, value.SlotId)
	FfiConverterTypeNonceINSTANCE.Write(writer, value.Nonce)
	FfiConverterTypePairIdINSTANCE.Write(writer, value.PairId)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.Size)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.Price)
	FfiConverterBoolINSTANCE.Write(writer, value.Direction)
	FfiConverterUint8INSTANCE.Write(writer, value.TakerFeeRate)
	FfiConverterUint8INSTANCE.Write(writer, value.MakerFeeRate)
	FfiConverterBoolINSTANCE.Write(writer, value.HasSubsidy)
}

type FfiDestroyerContractBuilder struct{}

func (_ FfiDestroyerContractBuilder) Destroy(value ContractBuilder) {
	value.Destroy()
}

type ContractMatchingBuilder struct {
	AccountId      AccountId
	SubAccountId   SubAccountId
	Taker          *Contract
	Maker          []*Contract
	Fee            BigUint
	FeeToken       TokenId
	ContractPrices []ContractPrice
	MarginPrices   []SpotPriceInfo
}

func (r *ContractMatchingBuilder) Destroy() {
	FfiDestroyerTypeAccountId{}.Destroy(r.AccountId)
	FfiDestroyerTypeSubAccountId{}.Destroy(r.SubAccountId)
	FfiDestroyerContract{}.Destroy(r.Taker)
	FfiDestroyerSequenceContract{}.Destroy(r.Maker)
	FfiDestroyerTypeBigUint{}.Destroy(r.Fee)
	FfiDestroyerTypeTokenId{}.Destroy(r.FeeToken)
	FfiDestroyerSequenceContractPrice{}.Destroy(r.ContractPrices)
	FfiDestroyerSequenceSpotPriceInfo{}.Destroy(r.MarginPrices)
}

type FfiConverterContractMatchingBuilder struct{}

var FfiConverterContractMatchingBuilderINSTANCE = FfiConverterContractMatchingBuilder{}

func (c FfiConverterContractMatchingBuilder) Lift(rb RustBufferI) ContractMatchingBuilder {
	return LiftFromRustBuffer[ContractMatchingBuilder](c, rb)
}

func (c FfiConverterContractMatchingBuilder) Read(reader io.Reader) ContractMatchingBuilder {
	return ContractMatchingBuilder{
		FfiConverterTypeAccountIdINSTANCE.Read(reader),
		FfiConverterTypeSubAccountIdINSTANCE.Read(reader),
		FfiConverterContractINSTANCE.Read(reader),
		FfiConverterSequenceContractINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterTypeTokenIdINSTANCE.Read(reader),
		FfiConverterSequenceContractPriceINSTANCE.Read(reader),
		FfiConverterSequenceSpotPriceInfoINSTANCE.Read(reader),
	}
}

func (c FfiConverterContractMatchingBuilder) Lower(value ContractMatchingBuilder) C.RustBuffer {
	return LowerIntoRustBuffer[ContractMatchingBuilder](c, value)
}

func (c FfiConverterContractMatchingBuilder) Write(writer io.Writer, value ContractMatchingBuilder) {
	FfiConverterTypeAccountIdINSTANCE.Write(writer, value.AccountId)
	FfiConverterTypeSubAccountIdINSTANCE.Write(writer, value.SubAccountId)
	FfiConverterContractINSTANCE.Write(writer, value.Taker)
	FfiConverterSequenceContractINSTANCE.Write(writer, value.Maker)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.Fee)
	FfiConverterTypeTokenIdINSTANCE.Write(writer, value.FeeToken)
	FfiConverterSequenceContractPriceINSTANCE.Write(writer, value.ContractPrices)
	FfiConverterSequenceSpotPriceInfoINSTANCE.Write(writer, value.MarginPrices)
}

type FfiDestroyerContractMatchingBuilder struct{}

func (_ FfiDestroyerContractMatchingBuilder) Destroy(value ContractMatchingBuilder) {
	value.Destroy()
}

type ContractPrice struct {
	PairId      PairId
	MarketPrice BigUint
}

func (r *ContractPrice) Destroy() {
	FfiDestroyerTypePairId{}.Destroy(r.PairId)
	FfiDestroyerTypeBigUint{}.Destroy(r.MarketPrice)
}

type FfiConverterContractPrice struct{}

var FfiConverterContractPriceINSTANCE = FfiConverterContractPrice{}

func (c FfiConverterContractPrice) Lift(rb RustBufferI) ContractPrice {
	return LiftFromRustBuffer[ContractPrice](c, rb)
}

func (c FfiConverterContractPrice) Read(reader io.Reader) ContractPrice {
	return ContractPrice{
		FfiConverterTypePairIdINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
	}
}

func (c FfiConverterContractPrice) Lower(value ContractPrice) C.RustBuffer {
	return LowerIntoRustBuffer[ContractPrice](c, value)
}

func (c FfiConverterContractPrice) Write(writer io.Writer, value ContractPrice) {
	FfiConverterTypePairIdINSTANCE.Write(writer, value.PairId)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.MarketPrice)
}

type FfiDestroyerContractPrice struct{}

func (_ FfiDestroyerContractPrice) Destroy(value ContractPrice) {
	value.Destroy()
}

type Create2Data struct {
	CreatorAddress ZkLinkAddress
	SaltArg        H256
	CodeHash       H256
}

func (r *Create2Data) Destroy() {
	FfiDestroyerTypeZkLinkAddress{}.Destroy(r.CreatorAddress)
	FfiDestroyerTypeH256{}.Destroy(r.SaltArg)
	FfiDestroyerTypeH256{}.Destroy(r.CodeHash)
}

type FfiConverterCreate2Data struct{}

var FfiConverterCreate2DataINSTANCE = FfiConverterCreate2Data{}

func (c FfiConverterCreate2Data) Lift(rb RustBufferI) Create2Data {
	return LiftFromRustBuffer[Create2Data](c, rb)
}

func (c FfiConverterCreate2Data) Read(reader io.Reader) Create2Data {
	return Create2Data{
		FfiConverterTypeZkLinkAddressINSTANCE.Read(reader),
		FfiConverterTypeH256INSTANCE.Read(reader),
		FfiConverterTypeH256INSTANCE.Read(reader),
	}
}

func (c FfiConverterCreate2Data) Lower(value Create2Data) C.RustBuffer {
	return LowerIntoRustBuffer[Create2Data](c, value)
}

func (c FfiConverterCreate2Data) Write(writer io.Writer, value Create2Data) {
	FfiConverterTypeZkLinkAddressINSTANCE.Write(writer, value.CreatorAddress)
	FfiConverterTypeH256INSTANCE.Write(writer, value.SaltArg)
	FfiConverterTypeH256INSTANCE.Write(writer, value.CodeHash)
}

type FfiDestroyerCreate2Data struct{}

func (_ FfiDestroyerCreate2Data) Destroy(value Create2Data) {
	value.Destroy()
}

type DepositBuilder struct {
	FromAddress   ZkLinkAddress
	ToAddress     ZkLinkAddress
	FromChainId   ChainId
	SubAccountId  SubAccountId
	L2TargetToken TokenId
	L1SourceToken TokenId
	Amount        BigUint
	SerialId      uint64
	L2Hash        H256
	EthHash       *H256
}

func (r *DepositBuilder) Destroy() {
	FfiDestroyerTypeZkLinkAddress{}.Destroy(r.FromAddress)
	FfiDestroyerTypeZkLinkAddress{}.Destroy(r.ToAddress)
	FfiDestroyerTypeChainId{}.Destroy(r.FromChainId)
	FfiDestroyerTypeSubAccountId{}.Destroy(r.SubAccountId)
	FfiDestroyerTypeTokenId{}.Destroy(r.L2TargetToken)
	FfiDestroyerTypeTokenId{}.Destroy(r.L1SourceToken)
	FfiDestroyerTypeBigUint{}.Destroy(r.Amount)
	FfiDestroyerUint64{}.Destroy(r.SerialId)
	FfiDestroyerTypeH256{}.Destroy(r.L2Hash)
	FfiDestroyerOptionalTypeH256{}.Destroy(r.EthHash)
}

type FfiConverterDepositBuilder struct{}

var FfiConverterDepositBuilderINSTANCE = FfiConverterDepositBuilder{}

func (c FfiConverterDepositBuilder) Lift(rb RustBufferI) DepositBuilder {
	return LiftFromRustBuffer[DepositBuilder](c, rb)
}

func (c FfiConverterDepositBuilder) Read(reader io.Reader) DepositBuilder {
	return DepositBuilder{
		FfiConverterTypeZkLinkAddressINSTANCE.Read(reader),
		FfiConverterTypeZkLinkAddressINSTANCE.Read(reader),
		FfiConverterTypeChainIdINSTANCE.Read(reader),
		FfiConverterTypeSubAccountIdINSTANCE.Read(reader),
		FfiConverterTypeTokenIdINSTANCE.Read(reader),
		FfiConverterTypeTokenIdINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterTypeH256INSTANCE.Read(reader),
		FfiConverterOptionalTypeH256INSTANCE.Read(reader),
	}
}

func (c FfiConverterDepositBuilder) Lower(value DepositBuilder) C.RustBuffer {
	return LowerIntoRustBuffer[DepositBuilder](c, value)
}

func (c FfiConverterDepositBuilder) Write(writer io.Writer, value DepositBuilder) {
	FfiConverterTypeZkLinkAddressINSTANCE.Write(writer, value.FromAddress)
	FfiConverterTypeZkLinkAddressINSTANCE.Write(writer, value.ToAddress)
	FfiConverterTypeChainIdINSTANCE.Write(writer, value.FromChainId)
	FfiConverterTypeSubAccountIdINSTANCE.Write(writer, value.SubAccountId)
	FfiConverterTypeTokenIdINSTANCE.Write(writer, value.L2TargetToken)
	FfiConverterTypeTokenIdINSTANCE.Write(writer, value.L1SourceToken)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.Amount)
	FfiConverterUint64INSTANCE.Write(writer, value.SerialId)
	FfiConverterTypeH256INSTANCE.Write(writer, value.L2Hash)
	FfiConverterOptionalTypeH256INSTANCE.Write(writer, value.EthHash)
}

type FfiDestroyerDepositBuilder struct{}

func (_ FfiDestroyerDepositBuilder) Destroy(value DepositBuilder) {
	value.Destroy()
}

type ForcedExitBuilder struct {
	ToChainId             ChainId
	InitiatorAccountId    AccountId
	InitiatorSubAccountId SubAccountId
	Target                ZkLinkAddress
	TargetSubAccountId    SubAccountId
	L2SourceToken         TokenId
	L1TargetToken         TokenId
	InitiatorNonce        Nonce
	ExitAmount            BigUint
	WithdrawToL1          bool
	Timestamp             TimeStamp
}

func (r *ForcedExitBuilder) Destroy() {
	FfiDestroyerTypeChainId{}.Destroy(r.ToChainId)
	FfiDestroyerTypeAccountId{}.Destroy(r.InitiatorAccountId)
	FfiDestroyerTypeSubAccountId{}.Destroy(r.InitiatorSubAccountId)
	FfiDestroyerTypeZkLinkAddress{}.Destroy(r.Target)
	FfiDestroyerTypeSubAccountId{}.Destroy(r.TargetSubAccountId)
	FfiDestroyerTypeTokenId{}.Destroy(r.L2SourceToken)
	FfiDestroyerTypeTokenId{}.Destroy(r.L1TargetToken)
	FfiDestroyerTypeNonce{}.Destroy(r.InitiatorNonce)
	FfiDestroyerTypeBigUint{}.Destroy(r.ExitAmount)
	FfiDestroyerBool{}.Destroy(r.WithdrawToL1)
	FfiDestroyerTypeTimeStamp{}.Destroy(r.Timestamp)
}

type FfiConverterForcedExitBuilder struct{}

var FfiConverterForcedExitBuilderINSTANCE = FfiConverterForcedExitBuilder{}

func (c FfiConverterForcedExitBuilder) Lift(rb RustBufferI) ForcedExitBuilder {
	return LiftFromRustBuffer[ForcedExitBuilder](c, rb)
}

func (c FfiConverterForcedExitBuilder) Read(reader io.Reader) ForcedExitBuilder {
	return ForcedExitBuilder{
		FfiConverterTypeChainIdINSTANCE.Read(reader),
		FfiConverterTypeAccountIdINSTANCE.Read(reader),
		FfiConverterTypeSubAccountIdINSTANCE.Read(reader),
		FfiConverterTypeZkLinkAddressINSTANCE.Read(reader),
		FfiConverterTypeSubAccountIdINSTANCE.Read(reader),
		FfiConverterTypeTokenIdINSTANCE.Read(reader),
		FfiConverterTypeTokenIdINSTANCE.Read(reader),
		FfiConverterTypeNonceINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterTypeTimeStampINSTANCE.Read(reader),
	}
}

func (c FfiConverterForcedExitBuilder) Lower(value ForcedExitBuilder) C.RustBuffer {
	return LowerIntoRustBuffer[ForcedExitBuilder](c, value)
}

func (c FfiConverterForcedExitBuilder) Write(writer io.Writer, value ForcedExitBuilder) {
	FfiConverterTypeChainIdINSTANCE.Write(writer, value.ToChainId)
	FfiConverterTypeAccountIdINSTANCE.Write(writer, value.InitiatorAccountId)
	FfiConverterTypeSubAccountIdINSTANCE.Write(writer, value.InitiatorSubAccountId)
	FfiConverterTypeZkLinkAddressINSTANCE.Write(writer, value.Target)
	FfiConverterTypeSubAccountIdINSTANCE.Write(writer, value.TargetSubAccountId)
	FfiConverterTypeTokenIdINSTANCE.Write(writer, value.L2SourceToken)
	FfiConverterTypeTokenIdINSTANCE.Write(writer, value.L1TargetToken)
	FfiConverterTypeNonceINSTANCE.Write(writer, value.InitiatorNonce)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.ExitAmount)
	FfiConverterBoolINSTANCE.Write(writer, value.WithdrawToL1)
	FfiConverterTypeTimeStampINSTANCE.Write(writer, value.Timestamp)
}

type FfiDestroyerForcedExitBuilder struct{}

func (_ FfiDestroyerForcedExitBuilder) Destroy(value ForcedExitBuilder) {
	value.Destroy()
}

type FullExitBuilder struct {
	ToChainId      ChainId
	AccountId      AccountId
	SubAccountId   SubAccountId
	ExitAddress    ZkLinkAddress
	L2SourceToken  TokenId
	L1TargetToken  TokenId
	ContractPrices []ContractPrice
	MarginPrices   []SpotPriceInfo
	SerialId       uint64
	L2Hash         H256
}

func (r *FullExitBuilder) Destroy() {
	FfiDestroyerTypeChainId{}.Destroy(r.ToChainId)
	FfiDestroyerTypeAccountId{}.Destroy(r.AccountId)
	FfiDestroyerTypeSubAccountId{}.Destroy(r.SubAccountId)
	FfiDestroyerTypeZkLinkAddress{}.Destroy(r.ExitAddress)
	FfiDestroyerTypeTokenId{}.Destroy(r.L2SourceToken)
	FfiDestroyerTypeTokenId{}.Destroy(r.L1TargetToken)
	FfiDestroyerSequenceContractPrice{}.Destroy(r.ContractPrices)
	FfiDestroyerSequenceSpotPriceInfo{}.Destroy(r.MarginPrices)
	FfiDestroyerUint64{}.Destroy(r.SerialId)
	FfiDestroyerTypeH256{}.Destroy(r.L2Hash)
}

type FfiConverterFullExitBuilder struct{}

var FfiConverterFullExitBuilderINSTANCE = FfiConverterFullExitBuilder{}

func (c FfiConverterFullExitBuilder) Lift(rb RustBufferI) FullExitBuilder {
	return LiftFromRustBuffer[FullExitBuilder](c, rb)
}

func (c FfiConverterFullExitBuilder) Read(reader io.Reader) FullExitBuilder {
	return FullExitBuilder{
		FfiConverterTypeChainIdINSTANCE.Read(reader),
		FfiConverterTypeAccountIdINSTANCE.Read(reader),
		FfiConverterTypeSubAccountIdINSTANCE.Read(reader),
		FfiConverterTypeZkLinkAddressINSTANCE.Read(reader),
		FfiConverterTypeTokenIdINSTANCE.Read(reader),
		FfiConverterTypeTokenIdINSTANCE.Read(reader),
		FfiConverterSequenceContractPriceINSTANCE.Read(reader),
		FfiConverterSequenceSpotPriceInfoINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterTypeH256INSTANCE.Read(reader),
	}
}

func (c FfiConverterFullExitBuilder) Lower(value FullExitBuilder) C.RustBuffer {
	return LowerIntoRustBuffer[FullExitBuilder](c, value)
}

func (c FfiConverterFullExitBuilder) Write(writer io.Writer, value FullExitBuilder) {
	FfiConverterTypeChainIdINSTANCE.Write(writer, value.ToChainId)
	FfiConverterTypeAccountIdINSTANCE.Write(writer, value.AccountId)
	FfiConverterTypeSubAccountIdINSTANCE.Write(writer, value.SubAccountId)
	FfiConverterTypeZkLinkAddressINSTANCE.Write(writer, value.ExitAddress)
	FfiConverterTypeTokenIdINSTANCE.Write(writer, value.L2SourceToken)
	FfiConverterTypeTokenIdINSTANCE.Write(writer, value.L1TargetToken)
	FfiConverterSequenceContractPriceINSTANCE.Write(writer, value.ContractPrices)
	FfiConverterSequenceSpotPriceInfoINSTANCE.Write(writer, value.MarginPrices)
	FfiConverterUint64INSTANCE.Write(writer, value.SerialId)
	FfiConverterTypeH256INSTANCE.Write(writer, value.L2Hash)
}

type FfiDestroyerFullExitBuilder struct{}

func (_ FfiDestroyerFullExitBuilder) Destroy(value FullExitBuilder) {
	value.Destroy()
}

type FundingBuilder struct {
	AccountId         AccountId
	SubAccountId      SubAccountId
	SubAccountNonce   Nonce
	FundingAccountIds []AccountId
	Fee               BigUint
	FeeToken          TokenId
}

func (r *FundingBuilder) Destroy() {
	FfiDestroyerTypeAccountId{}.Destroy(r.AccountId)
	FfiDestroyerTypeSubAccountId{}.Destroy(r.SubAccountId)
	FfiDestroyerTypeNonce{}.Destroy(r.SubAccountNonce)
	FfiDestroyerSequenceTypeAccountId{}.Destroy(r.FundingAccountIds)
	FfiDestroyerTypeBigUint{}.Destroy(r.Fee)
	FfiDestroyerTypeTokenId{}.Destroy(r.FeeToken)
}

type FfiConverterFundingBuilder struct{}

var FfiConverterFundingBuilderINSTANCE = FfiConverterFundingBuilder{}

func (c FfiConverterFundingBuilder) Lift(rb RustBufferI) FundingBuilder {
	return LiftFromRustBuffer[FundingBuilder](c, rb)
}

func (c FfiConverterFundingBuilder) Read(reader io.Reader) FundingBuilder {
	return FundingBuilder{
		FfiConverterTypeAccountIdINSTANCE.Read(reader),
		FfiConverterTypeSubAccountIdINSTANCE.Read(reader),
		FfiConverterTypeNonceINSTANCE.Read(reader),
		FfiConverterSequenceTypeAccountIdINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterTypeTokenIdINSTANCE.Read(reader),
	}
}

func (c FfiConverterFundingBuilder) Lower(value FundingBuilder) C.RustBuffer {
	return LowerIntoRustBuffer[FundingBuilder](c, value)
}

func (c FfiConverterFundingBuilder) Write(writer io.Writer, value FundingBuilder) {
	FfiConverterTypeAccountIdINSTANCE.Write(writer, value.AccountId)
	FfiConverterTypeSubAccountIdINSTANCE.Write(writer, value.SubAccountId)
	FfiConverterTypeNonceINSTANCE.Write(writer, value.SubAccountNonce)
	FfiConverterSequenceTypeAccountIdINSTANCE.Write(writer, value.FundingAccountIds)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.Fee)
	FfiConverterTypeTokenIdINSTANCE.Write(writer, value.FeeToken)
}

type FfiDestroyerFundingBuilder struct{}

func (_ FfiDestroyerFundingBuilder) Destroy(value FundingBuilder) {
	value.Destroy()
}

type FundingInfo struct {
	PairId      PairId
	Price       BigUint
	FundingRate int16
}

func (r *FundingInfo) Destroy() {
	FfiDestroyerTypePairId{}.Destroy(r.PairId)
	FfiDestroyerTypeBigUint{}.Destroy(r.Price)
	FfiDestroyerInt16{}.Destroy(r.FundingRate)
}

type FfiConverterFundingInfo struct{}

var FfiConverterFundingInfoINSTANCE = FfiConverterFundingInfo{}

func (c FfiConverterFundingInfo) Lift(rb RustBufferI) FundingInfo {
	return LiftFromRustBuffer[FundingInfo](c, rb)
}

func (c FfiConverterFundingInfo) Read(reader io.Reader) FundingInfo {
	return FundingInfo{
		FfiConverterTypePairIdINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterInt16INSTANCE.Read(reader),
	}
}

func (c FfiConverterFundingInfo) Lower(value FundingInfo) C.RustBuffer {
	return LowerIntoRustBuffer[FundingInfo](c, value)
}

func (c FfiConverterFundingInfo) Write(writer io.Writer, value FundingInfo) {
	FfiConverterTypePairIdINSTANCE.Write(writer, value.PairId)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.Price)
	FfiConverterInt16INSTANCE.Write(writer, value.FundingRate)
}

type FfiDestroyerFundingInfo struct{}

func (_ FfiDestroyerFundingInfo) Destroy(value FundingInfo) {
	value.Destroy()
}

type LiquidationBuilder struct {
	AccountId            AccountId
	SubAccountId         SubAccountId
	SubAccountNonce      Nonce
	ContractPrices       []ContractPrice
	MarginPrices         []SpotPriceInfo
	LiquidationAccountId AccountId
	Fee                  BigUint
	FeeToken             TokenId
}

func (r *LiquidationBuilder) Destroy() {
	FfiDestroyerTypeAccountId{}.Destroy(r.AccountId)
	FfiDestroyerTypeSubAccountId{}.Destroy(r.SubAccountId)
	FfiDestroyerTypeNonce{}.Destroy(r.SubAccountNonce)
	FfiDestroyerSequenceContractPrice{}.Destroy(r.ContractPrices)
	FfiDestroyerSequenceSpotPriceInfo{}.Destroy(r.MarginPrices)
	FfiDestroyerTypeAccountId{}.Destroy(r.LiquidationAccountId)
	FfiDestroyerTypeBigUint{}.Destroy(r.Fee)
	FfiDestroyerTypeTokenId{}.Destroy(r.FeeToken)
}

type FfiConverterLiquidationBuilder struct{}

var FfiConverterLiquidationBuilderINSTANCE = FfiConverterLiquidationBuilder{}

func (c FfiConverterLiquidationBuilder) Lift(rb RustBufferI) LiquidationBuilder {
	return LiftFromRustBuffer[LiquidationBuilder](c, rb)
}

func (c FfiConverterLiquidationBuilder) Read(reader io.Reader) LiquidationBuilder {
	return LiquidationBuilder{
		FfiConverterTypeAccountIdINSTANCE.Read(reader),
		FfiConverterTypeSubAccountIdINSTANCE.Read(reader),
		FfiConverterTypeNonceINSTANCE.Read(reader),
		FfiConverterSequenceContractPriceINSTANCE.Read(reader),
		FfiConverterSequenceSpotPriceInfoINSTANCE.Read(reader),
		FfiConverterTypeAccountIdINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterTypeTokenIdINSTANCE.Read(reader),
	}
}

func (c FfiConverterLiquidationBuilder) Lower(value LiquidationBuilder) C.RustBuffer {
	return LowerIntoRustBuffer[LiquidationBuilder](c, value)
}

func (c FfiConverterLiquidationBuilder) Write(writer io.Writer, value LiquidationBuilder) {
	FfiConverterTypeAccountIdINSTANCE.Write(writer, value.AccountId)
	FfiConverterTypeSubAccountIdINSTANCE.Write(writer, value.SubAccountId)
	FfiConverterTypeNonceINSTANCE.Write(writer, value.SubAccountNonce)
	FfiConverterSequenceContractPriceINSTANCE.Write(writer, value.ContractPrices)
	FfiConverterSequenceSpotPriceInfoINSTANCE.Write(writer, value.MarginPrices)
	FfiConverterTypeAccountIdINSTANCE.Write(writer, value.LiquidationAccountId)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.Fee)
	FfiConverterTypeTokenIdINSTANCE.Write(writer, value.FeeToken)
}

type FfiDestroyerLiquidationBuilder struct{}

func (_ FfiDestroyerLiquidationBuilder) Destroy(value LiquidationBuilder) {
	value.Destroy()
}

type Message struct {
	Data string
}

func (r *Message) Destroy() {
	FfiDestroyerString{}.Destroy(r.Data)
}

type FfiConverterMessage struct{}

var FfiConverterMessageINSTANCE = FfiConverterMessage{}

func (c FfiConverterMessage) Lift(rb RustBufferI) Message {
	return LiftFromRustBuffer[Message](c, rb)
}

func (c FfiConverterMessage) Read(reader io.Reader) Message {
	return Message{
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterMessage) Lower(value Message) C.RustBuffer {
	return LowerIntoRustBuffer[Message](c, value)
}

func (c FfiConverterMessage) Write(writer io.Writer, value Message) {
	FfiConverterStringINSTANCE.Write(writer, value.Data)
}

type FfiDestroyerMessage struct{}

func (_ FfiDestroyerMessage) Destroy(value Message) {
	value.Destroy()
}

type OraclePrices struct {
	ContractPrices []ContractPrice
	MarginPrices   []SpotPriceInfo
}

func (r *OraclePrices) Destroy() {
	FfiDestroyerSequenceContractPrice{}.Destroy(r.ContractPrices)
	FfiDestroyerSequenceSpotPriceInfo{}.Destroy(r.MarginPrices)
}

type FfiConverterOraclePrices struct{}

var FfiConverterOraclePricesINSTANCE = FfiConverterOraclePrices{}

func (c FfiConverterOraclePrices) Lift(rb RustBufferI) OraclePrices {
	return LiftFromRustBuffer[OraclePrices](c, rb)
}

func (c FfiConverterOraclePrices) Read(reader io.Reader) OraclePrices {
	return OraclePrices{
		FfiConverterSequenceContractPriceINSTANCE.Read(reader),
		FfiConverterSequenceSpotPriceInfoINSTANCE.Read(reader),
	}
}

func (c FfiConverterOraclePrices) Lower(value OraclePrices) C.RustBuffer {
	return LowerIntoRustBuffer[OraclePrices](c, value)
}

func (c FfiConverterOraclePrices) Write(writer io.Writer, value OraclePrices) {
	FfiConverterSequenceContractPriceINSTANCE.Write(writer, value.ContractPrices)
	FfiConverterSequenceSpotPriceInfoINSTANCE.Write(writer, value.MarginPrices)
}

type FfiDestroyerOraclePrices struct{}

func (_ FfiDestroyerOraclePrices) Destroy(value OraclePrices) {
	value.Destroy()
}

type OrderMatchingBuilder struct {
	AccountId         AccountId
	SubAccountId      SubAccountId
	Taker             *Order
	Maker             *Order
	Fee               BigUint
	FeeToken          TokenId
	ContractPrices    []ContractPrice
	MarginPrices      []SpotPriceInfo
	ExpectBaseAmount  BigUint
	ExpectQuoteAmount BigUint
}

func (r *OrderMatchingBuilder) Destroy() {
	FfiDestroyerTypeAccountId{}.Destroy(r.AccountId)
	FfiDestroyerTypeSubAccountId{}.Destroy(r.SubAccountId)
	FfiDestroyerOrder{}.Destroy(r.Taker)
	FfiDestroyerOrder{}.Destroy(r.Maker)
	FfiDestroyerTypeBigUint{}.Destroy(r.Fee)
	FfiDestroyerTypeTokenId{}.Destroy(r.FeeToken)
	FfiDestroyerSequenceContractPrice{}.Destroy(r.ContractPrices)
	FfiDestroyerSequenceSpotPriceInfo{}.Destroy(r.MarginPrices)
	FfiDestroyerTypeBigUint{}.Destroy(r.ExpectBaseAmount)
	FfiDestroyerTypeBigUint{}.Destroy(r.ExpectQuoteAmount)
}

type FfiConverterOrderMatchingBuilder struct{}

var FfiConverterOrderMatchingBuilderINSTANCE = FfiConverterOrderMatchingBuilder{}

func (c FfiConverterOrderMatchingBuilder) Lift(rb RustBufferI) OrderMatchingBuilder {
	return LiftFromRustBuffer[OrderMatchingBuilder](c, rb)
}

func (c FfiConverterOrderMatchingBuilder) Read(reader io.Reader) OrderMatchingBuilder {
	return OrderMatchingBuilder{
		FfiConverterTypeAccountIdINSTANCE.Read(reader),
		FfiConverterTypeSubAccountIdINSTANCE.Read(reader),
		FfiConverterOrderINSTANCE.Read(reader),
		FfiConverterOrderINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterTypeTokenIdINSTANCE.Read(reader),
		FfiConverterSequenceContractPriceINSTANCE.Read(reader),
		FfiConverterSequenceSpotPriceInfoINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
	}
}

func (c FfiConverterOrderMatchingBuilder) Lower(value OrderMatchingBuilder) C.RustBuffer {
	return LowerIntoRustBuffer[OrderMatchingBuilder](c, value)
}

func (c FfiConverterOrderMatchingBuilder) Write(writer io.Writer, value OrderMatchingBuilder) {
	FfiConverterTypeAccountIdINSTANCE.Write(writer, value.AccountId)
	FfiConverterTypeSubAccountIdINSTANCE.Write(writer, value.SubAccountId)
	FfiConverterOrderINSTANCE.Write(writer, value.Taker)
	FfiConverterOrderINSTANCE.Write(writer, value.Maker)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.Fee)
	FfiConverterTypeTokenIdINSTANCE.Write(writer, value.FeeToken)
	FfiConverterSequenceContractPriceINSTANCE.Write(writer, value.ContractPrices)
	FfiConverterSequenceSpotPriceInfoINSTANCE.Write(writer, value.MarginPrices)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.ExpectBaseAmount)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.ExpectQuoteAmount)
}

type FfiDestroyerOrderMatchingBuilder struct{}

func (_ FfiDestroyerOrderMatchingBuilder) Destroy(value OrderMatchingBuilder) {
	value.Destroy()
}

type SpotPriceInfo struct {
	TokenId TokenId
	Price   BigUint
}

func (r *SpotPriceInfo) Destroy() {
	FfiDestroyerTypeTokenId{}.Destroy(r.TokenId)
	FfiDestroyerTypeBigUint{}.Destroy(r.Price)
}

type FfiConverterSpotPriceInfo struct{}

var FfiConverterSpotPriceInfoINSTANCE = FfiConverterSpotPriceInfo{}

func (c FfiConverterSpotPriceInfo) Lift(rb RustBufferI) SpotPriceInfo {
	return LiftFromRustBuffer[SpotPriceInfo](c, rb)
}

func (c FfiConverterSpotPriceInfo) Read(reader io.Reader) SpotPriceInfo {
	return SpotPriceInfo{
		FfiConverterTypeTokenIdINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
	}
}

func (c FfiConverterSpotPriceInfo) Lower(value SpotPriceInfo) C.RustBuffer {
	return LowerIntoRustBuffer[SpotPriceInfo](c, value)
}

func (c FfiConverterSpotPriceInfo) Write(writer io.Writer, value SpotPriceInfo) {
	FfiConverterTypeTokenIdINSTANCE.Write(writer, value.TokenId)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.Price)
}

type FfiDestroyerSpotPriceInfo struct{}

func (_ FfiDestroyerSpotPriceInfo) Destroy(value SpotPriceInfo) {
	value.Destroy()
}

type TransferBuilder struct {
	AccountId        AccountId
	ToAddress        ZkLinkAddress
	FromSubAccountId SubAccountId
	ToSubAccountId   SubAccountId
	Token            TokenId
	Amount           BigUint
	Fee              BigUint
	Nonce            Nonce
	Timestamp        TimeStamp
}

func (r *TransferBuilder) Destroy() {
	FfiDestroyerTypeAccountId{}.Destroy(r.AccountId)
	FfiDestroyerTypeZkLinkAddress{}.Destroy(r.ToAddress)
	FfiDestroyerTypeSubAccountId{}.Destroy(r.FromSubAccountId)
	FfiDestroyerTypeSubAccountId{}.Destroy(r.ToSubAccountId)
	FfiDestroyerTypeTokenId{}.Destroy(r.Token)
	FfiDestroyerTypeBigUint{}.Destroy(r.Amount)
	FfiDestroyerTypeBigUint{}.Destroy(r.Fee)
	FfiDestroyerTypeNonce{}.Destroy(r.Nonce)
	FfiDestroyerTypeTimeStamp{}.Destroy(r.Timestamp)
}

type FfiConverterTransferBuilder struct{}

var FfiConverterTransferBuilderINSTANCE = FfiConverterTransferBuilder{}

func (c FfiConverterTransferBuilder) Lift(rb RustBufferI) TransferBuilder {
	return LiftFromRustBuffer[TransferBuilder](c, rb)
}

func (c FfiConverterTransferBuilder) Read(reader io.Reader) TransferBuilder {
	return TransferBuilder{
		FfiConverterTypeAccountIdINSTANCE.Read(reader),
		FfiConverterTypeZkLinkAddressINSTANCE.Read(reader),
		FfiConverterTypeSubAccountIdINSTANCE.Read(reader),
		FfiConverterTypeSubAccountIdINSTANCE.Read(reader),
		FfiConverterTypeTokenIdINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterTypeNonceINSTANCE.Read(reader),
		FfiConverterTypeTimeStampINSTANCE.Read(reader),
	}
}

func (c FfiConverterTransferBuilder) Lower(value TransferBuilder) C.RustBuffer {
	return LowerIntoRustBuffer[TransferBuilder](c, value)
}

func (c FfiConverterTransferBuilder) Write(writer io.Writer, value TransferBuilder) {
	FfiConverterTypeAccountIdINSTANCE.Write(writer, value.AccountId)
	FfiConverterTypeZkLinkAddressINSTANCE.Write(writer, value.ToAddress)
	FfiConverterTypeSubAccountIdINSTANCE.Write(writer, value.FromSubAccountId)
	FfiConverterTypeSubAccountIdINSTANCE.Write(writer, value.ToSubAccountId)
	FfiConverterTypeTokenIdINSTANCE.Write(writer, value.Token)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.Amount)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.Fee)
	FfiConverterTypeNonceINSTANCE.Write(writer, value.Nonce)
	FfiConverterTypeTimeStampINSTANCE.Write(writer, value.Timestamp)
}

type FfiDestroyerTransferBuilder struct{}

func (_ FfiDestroyerTransferBuilder) Destroy(value TransferBuilder) {
	value.Destroy()
}

type TxMessage struct {
	Transaction string
	Amount      string
	Fee         string
	Token       string
	To          string
	Nonce       string
}

func (r *TxMessage) Destroy() {
	FfiDestroyerString{}.Destroy(r.Transaction)
	FfiDestroyerString{}.Destroy(r.Amount)
	FfiDestroyerString{}.Destroy(r.Fee)
	FfiDestroyerString{}.Destroy(r.Token)
	FfiDestroyerString{}.Destroy(r.To)
	FfiDestroyerString{}.Destroy(r.Nonce)
}

type FfiConverterTxMessage struct{}

var FfiConverterTxMessageINSTANCE = FfiConverterTxMessage{}

func (c FfiConverterTxMessage) Lift(rb RustBufferI) TxMessage {
	return LiftFromRustBuffer[TxMessage](c, rb)
}

func (c FfiConverterTxMessage) Read(reader io.Reader) TxMessage {
	return TxMessage{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterTxMessage) Lower(value TxMessage) C.RustBuffer {
	return LowerIntoRustBuffer[TxMessage](c, value)
}

func (c FfiConverterTxMessage) Write(writer io.Writer, value TxMessage) {
	FfiConverterStringINSTANCE.Write(writer, value.Transaction)
	FfiConverterStringINSTANCE.Write(writer, value.Amount)
	FfiConverterStringINSTANCE.Write(writer, value.Fee)
	FfiConverterStringINSTANCE.Write(writer, value.Token)
	FfiConverterStringINSTANCE.Write(writer, value.To)
	FfiConverterStringINSTANCE.Write(writer, value.Nonce)
}

type FfiDestroyerTxMessage struct{}

func (_ FfiDestroyerTxMessage) Destroy(value TxMessage) {
	value.Destroy()
}

type TxSignature struct {
	Tx              ZkLinkTx
	Layer1Signature *TxLayer1Signature
}

func (r *TxSignature) Destroy() {
	FfiDestroyerTypeZkLinkTx{}.Destroy(r.Tx)
	FfiDestroyerOptionalTypeTxLayer1Signature{}.Destroy(r.Layer1Signature)
}

type FfiConverterTxSignature struct{}

var FfiConverterTxSignatureINSTANCE = FfiConverterTxSignature{}

func (c FfiConverterTxSignature) Lift(rb RustBufferI) TxSignature {
	return LiftFromRustBuffer[TxSignature](c, rb)
}

func (c FfiConverterTxSignature) Read(reader io.Reader) TxSignature {
	return TxSignature{
		FfiConverterTypeZkLinkTxINSTANCE.Read(reader),
		FfiConverterOptionalTypeTxLayer1SignatureINSTANCE.Read(reader),
	}
}

func (c FfiConverterTxSignature) Lower(value TxSignature) C.RustBuffer {
	return LowerIntoRustBuffer[TxSignature](c, value)
}

func (c FfiConverterTxSignature) Write(writer io.Writer, value TxSignature) {
	FfiConverterTypeZkLinkTxINSTANCE.Write(writer, value.Tx)
	FfiConverterOptionalTypeTxLayer1SignatureINSTANCE.Write(writer, value.Layer1Signature)
}

type FfiDestroyerTxSignature struct{}

func (_ FfiDestroyerTxSignature) Destroy(value TxSignature) {
	value.Destroy()
}

type UpdateGlobalVarBuilder struct {
	FromChainId  ChainId
	SubAccountId SubAccountId
	Parameter    Parameter
	SerialId     uint64
}

func (r *UpdateGlobalVarBuilder) Destroy() {
	FfiDestroyerTypeChainId{}.Destroy(r.FromChainId)
	FfiDestroyerTypeSubAccountId{}.Destroy(r.SubAccountId)
	FfiDestroyerParameter{}.Destroy(r.Parameter)
	FfiDestroyerUint64{}.Destroy(r.SerialId)
}

type FfiConverterUpdateGlobalVarBuilder struct{}

var FfiConverterUpdateGlobalVarBuilderINSTANCE = FfiConverterUpdateGlobalVarBuilder{}

func (c FfiConverterUpdateGlobalVarBuilder) Lift(rb RustBufferI) UpdateGlobalVarBuilder {
	return LiftFromRustBuffer[UpdateGlobalVarBuilder](c, rb)
}

func (c FfiConverterUpdateGlobalVarBuilder) Read(reader io.Reader) UpdateGlobalVarBuilder {
	return UpdateGlobalVarBuilder{
		FfiConverterTypeChainIdINSTANCE.Read(reader),
		FfiConverterTypeSubAccountIdINSTANCE.Read(reader),
		FfiConverterParameterINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterUpdateGlobalVarBuilder) Lower(value UpdateGlobalVarBuilder) C.RustBuffer {
	return LowerIntoRustBuffer[UpdateGlobalVarBuilder](c, value)
}

func (c FfiConverterUpdateGlobalVarBuilder) Write(writer io.Writer, value UpdateGlobalVarBuilder) {
	FfiConverterTypeChainIdINSTANCE.Write(writer, value.FromChainId)
	FfiConverterTypeSubAccountIdINSTANCE.Write(writer, value.SubAccountId)
	FfiConverterParameterINSTANCE.Write(writer, value.Parameter)
	FfiConverterUint64INSTANCE.Write(writer, value.SerialId)
}

type FfiDestroyerUpdateGlobalVarBuilder struct{}

func (_ FfiDestroyerUpdateGlobalVarBuilder) Destroy(value UpdateGlobalVarBuilder) {
	value.Destroy()
}

type WithdrawBuilder struct {
	AccountId        AccountId
	SubAccountId     SubAccountId
	ToChainId        ChainId
	ToAddress        ZkLinkAddress
	L2SourceToken    TokenId
	L1TargetToken    TokenId
	Amount           BigUint
	CallData         *[]uint8
	Fee              BigUint
	Nonce            Nonce
	WithdrawFeeRatio uint16
	WithdrawToL1     bool
	Timestamp        TimeStamp
}

func (r *WithdrawBuilder) Destroy() {
	FfiDestroyerTypeAccountId{}.Destroy(r.AccountId)
	FfiDestroyerTypeSubAccountId{}.Destroy(r.SubAccountId)
	FfiDestroyerTypeChainId{}.Destroy(r.ToChainId)
	FfiDestroyerTypeZkLinkAddress{}.Destroy(r.ToAddress)
	FfiDestroyerTypeTokenId{}.Destroy(r.L2SourceToken)
	FfiDestroyerTypeTokenId{}.Destroy(r.L1TargetToken)
	FfiDestroyerTypeBigUint{}.Destroy(r.Amount)
	FfiDestroyerOptionalSequenceUint8{}.Destroy(r.CallData)
	FfiDestroyerTypeBigUint{}.Destroy(r.Fee)
	FfiDestroyerTypeNonce{}.Destroy(r.Nonce)
	FfiDestroyerUint16{}.Destroy(r.WithdrawFeeRatio)
	FfiDestroyerBool{}.Destroy(r.WithdrawToL1)
	FfiDestroyerTypeTimeStamp{}.Destroy(r.Timestamp)
}

type FfiConverterWithdrawBuilder struct{}

var FfiConverterWithdrawBuilderINSTANCE = FfiConverterWithdrawBuilder{}

func (c FfiConverterWithdrawBuilder) Lift(rb RustBufferI) WithdrawBuilder {
	return LiftFromRustBuffer[WithdrawBuilder](c, rb)
}

func (c FfiConverterWithdrawBuilder) Read(reader io.Reader) WithdrawBuilder {
	return WithdrawBuilder{
		FfiConverterTypeAccountIdINSTANCE.Read(reader),
		FfiConverterTypeSubAccountIdINSTANCE.Read(reader),
		FfiConverterTypeChainIdINSTANCE.Read(reader),
		FfiConverterTypeZkLinkAddressINSTANCE.Read(reader),
		FfiConverterTypeTokenIdINSTANCE.Read(reader),
		FfiConverterTypeTokenIdINSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterOptionalSequenceUint8INSTANCE.Read(reader),
		FfiConverterTypeBigUintINSTANCE.Read(reader),
		FfiConverterTypeNonceINSTANCE.Read(reader),
		FfiConverterUint16INSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterTypeTimeStampINSTANCE.Read(reader),
	}
}

func (c FfiConverterWithdrawBuilder) Lower(value WithdrawBuilder) C.RustBuffer {
	return LowerIntoRustBuffer[WithdrawBuilder](c, value)
}

func (c FfiConverterWithdrawBuilder) Write(writer io.Writer, value WithdrawBuilder) {
	FfiConverterTypeAccountIdINSTANCE.Write(writer, value.AccountId)
	FfiConverterTypeSubAccountIdINSTANCE.Write(writer, value.SubAccountId)
	FfiConverterTypeChainIdINSTANCE.Write(writer, value.ToChainId)
	FfiConverterTypeZkLinkAddressINSTANCE.Write(writer, value.ToAddress)
	FfiConverterTypeTokenIdINSTANCE.Write(writer, value.L2SourceToken)
	FfiConverterTypeTokenIdINSTANCE.Write(writer, value.L1TargetToken)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.Amount)
	FfiConverterOptionalSequenceUint8INSTANCE.Write(writer, value.CallData)
	FfiConverterTypeBigUintINSTANCE.Write(writer, value.Fee)
	FfiConverterTypeNonceINSTANCE.Write(writer, value.Nonce)
	FfiConverterUint16INSTANCE.Write(writer, value.WithdrawFeeRatio)
	FfiConverterBoolINSTANCE.Write(writer, value.WithdrawToL1)
	FfiConverterTypeTimeStampINSTANCE.Write(writer, value.Timestamp)
}

type FfiDestroyerWithdrawBuilder struct{}

func (_ FfiDestroyerWithdrawBuilder) Destroy(value WithdrawBuilder) {
	value.Destroy()
}

type ZkLinkSignature struct {
	PubKey    PackedPublicKey
	Signature PackedSignature
}

func (r *ZkLinkSignature) Destroy() {
	FfiDestroyerTypePackedPublicKey{}.Destroy(r.PubKey)
	FfiDestroyerTypePackedSignature{}.Destroy(r.Signature)
}

type FfiConverterZkLinkSignature struct{}

var FfiConverterZkLinkSignatureINSTANCE = FfiConverterZkLinkSignature{}

func (c FfiConverterZkLinkSignature) Lift(rb RustBufferI) ZkLinkSignature {
	return LiftFromRustBuffer[ZkLinkSignature](c, rb)
}

func (c FfiConverterZkLinkSignature) Read(reader io.Reader) ZkLinkSignature {
	return ZkLinkSignature{
		FfiConverterTypePackedPublicKeyINSTANCE.Read(reader),
		FfiConverterTypePackedSignatureINSTANCE.Read(reader),
	}
}

func (c FfiConverterZkLinkSignature) Lower(value ZkLinkSignature) C.RustBuffer {
	return LowerIntoRustBuffer[ZkLinkSignature](c, value)
}

func (c FfiConverterZkLinkSignature) Write(writer io.Writer, value ZkLinkSignature) {
	FfiConverterTypePackedPublicKeyINSTANCE.Write(writer, value.PubKey)
	FfiConverterTypePackedSignatureINSTANCE.Write(writer, value.Signature)
}

type FfiDestroyerZkLinkSignature struct{}

func (_ FfiDestroyerZkLinkSignature) Destroy(value ZkLinkSignature) {
	value.Destroy()
}

type ChangePubKeyAuthData interface {
	Destroy()
}
type ChangePubKeyAuthDataOnchain struct {
}

func (e ChangePubKeyAuthDataOnchain) Destroy() {
}

type ChangePubKeyAuthDataEthEcdsa struct {
	EthSignature PackedEthSignature
}

func (e ChangePubKeyAuthDataEthEcdsa) Destroy() {
	FfiDestroyerTypePackedEthSignature{}.Destroy(e.EthSignature)
}

type ChangePubKeyAuthDataEthCreate2 struct {
	Data Create2Data
}

func (e ChangePubKeyAuthDataEthCreate2) Destroy() {
	FfiDestroyerCreate2Data{}.Destroy(e.Data)
}

type FfiConverterChangePubKeyAuthData struct{}

var FfiConverterChangePubKeyAuthDataINSTANCE = FfiConverterChangePubKeyAuthData{}

func (c FfiConverterChangePubKeyAuthData) Lift(rb RustBufferI) ChangePubKeyAuthData {
	return LiftFromRustBuffer[ChangePubKeyAuthData](c, rb)
}

func (c FfiConverterChangePubKeyAuthData) Lower(value ChangePubKeyAuthData) C.RustBuffer {
	return LowerIntoRustBuffer[ChangePubKeyAuthData](c, value)
}
func (FfiConverterChangePubKeyAuthData) Read(reader io.Reader) ChangePubKeyAuthData {
	id := readInt32(reader)
	switch id {
	case 1:
		return ChangePubKeyAuthDataOnchain{}
	case 2:
		return ChangePubKeyAuthDataEthEcdsa{
			FfiConverterTypePackedEthSignatureINSTANCE.Read(reader),
		}
	case 3:
		return ChangePubKeyAuthDataEthCreate2{
			FfiConverterCreate2DataINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterChangePubKeyAuthData.Read()", id))
	}
}

func (FfiConverterChangePubKeyAuthData) Write(writer io.Writer, value ChangePubKeyAuthData) {
	switch variant_value := value.(type) {
	case ChangePubKeyAuthDataOnchain:
		writeInt32(writer, 1)
	case ChangePubKeyAuthDataEthEcdsa:
		writeInt32(writer, 2)
		FfiConverterTypePackedEthSignatureINSTANCE.Write(writer, variant_value.EthSignature)
	case ChangePubKeyAuthDataEthCreate2:
		writeInt32(writer, 3)
		FfiConverterCreate2DataINSTANCE.Write(writer, variant_value.Data)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterChangePubKeyAuthData.Write", value))
	}
}

type FfiDestroyerChangePubKeyAuthData struct{}

func (_ FfiDestroyerChangePubKeyAuthData) Destroy(value ChangePubKeyAuthData) {
	value.Destroy()
}

type ChangePubKeyAuthRequest interface {
	Destroy()
}
type ChangePubKeyAuthRequestOnchain struct {
}

func (e ChangePubKeyAuthRequestOnchain) Destroy() {
}

type ChangePubKeyAuthRequestEthEcdsa struct {
}

func (e ChangePubKeyAuthRequestEthEcdsa) Destroy() {
}

type ChangePubKeyAuthRequestEthCreate2 struct {
	Data Create2Data
}

func (e ChangePubKeyAuthRequestEthCreate2) Destroy() {
	FfiDestroyerCreate2Data{}.Destroy(e.Data)
}

type FfiConverterChangePubKeyAuthRequest struct{}

var FfiConverterChangePubKeyAuthRequestINSTANCE = FfiConverterChangePubKeyAuthRequest{}

func (c FfiConverterChangePubKeyAuthRequest) Lift(rb RustBufferI) ChangePubKeyAuthRequest {
	return LiftFromRustBuffer[ChangePubKeyAuthRequest](c, rb)
}

func (c FfiConverterChangePubKeyAuthRequest) Lower(value ChangePubKeyAuthRequest) C.RustBuffer {
	return LowerIntoRustBuffer[ChangePubKeyAuthRequest](c, value)
}
func (FfiConverterChangePubKeyAuthRequest) Read(reader io.Reader) ChangePubKeyAuthRequest {
	id := readInt32(reader)
	switch id {
	case 1:
		return ChangePubKeyAuthRequestOnchain{}
	case 2:
		return ChangePubKeyAuthRequestEthEcdsa{}
	case 3:
		return ChangePubKeyAuthRequestEthCreate2{
			FfiConverterCreate2DataINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterChangePubKeyAuthRequest.Read()", id))
	}
}

func (FfiConverterChangePubKeyAuthRequest) Write(writer io.Writer, value ChangePubKeyAuthRequest) {
	switch variant_value := value.(type) {
	case ChangePubKeyAuthRequestOnchain:
		writeInt32(writer, 1)
	case ChangePubKeyAuthRequestEthEcdsa:
		writeInt32(writer, 2)
	case ChangePubKeyAuthRequestEthCreate2:
		writeInt32(writer, 3)
		FfiConverterCreate2DataINSTANCE.Write(writer, variant_value.Data)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterChangePubKeyAuthRequest.Write", value))
	}
}

type FfiDestroyerChangePubKeyAuthRequest struct{}

func (_ FfiDestroyerChangePubKeyAuthRequest) Destroy(value ChangePubKeyAuthRequest) {
	value.Destroy()
}

type EthSignerError struct {
	err error
}

// Convience method to turn *EthSignerError into error
// Avoiding treating nil pointer as non nil error interface
func (err *EthSignerError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err EthSignerError) Error() string {
	return fmt.Sprintf("EthSignerError: %s", err.err.Error())
}

func (err EthSignerError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrEthSignerErrorInvalidEthSigner = fmt.Errorf("EthSignerErrorInvalidEthSigner")
var ErrEthSignerErrorMissingEthPrivateKey = fmt.Errorf("EthSignerErrorMissingEthPrivateKey")
var ErrEthSignerErrorMissingEthSigner = fmt.Errorf("EthSignerErrorMissingEthSigner")
var ErrEthSignerErrorSigningFailed = fmt.Errorf("EthSignerErrorSigningFailed")
var ErrEthSignerErrorUnlockingFailed = fmt.Errorf("EthSignerErrorUnlockingFailed")
var ErrEthSignerErrorInvalidRawTx = fmt.Errorf("EthSignerErrorInvalidRawTx")
var ErrEthSignerErrorEip712Failed = fmt.Errorf("EthSignerErrorEip712Failed")
var ErrEthSignerErrorNoSigningKey = fmt.Errorf("EthSignerErrorNoSigningKey")
var ErrEthSignerErrorDefineAddress = fmt.Errorf("EthSignerErrorDefineAddress")
var ErrEthSignerErrorRecoverAddress = fmt.Errorf("EthSignerErrorRecoverAddress")
var ErrEthSignerErrorLengthMismatched = fmt.Errorf("EthSignerErrorLengthMismatched")
var ErrEthSignerErrorCryptoError = fmt.Errorf("EthSignerErrorCryptoError")
var ErrEthSignerErrorInvalidSignatureStr = fmt.Errorf("EthSignerErrorInvalidSignatureStr")
var ErrEthSignerErrorCustomError = fmt.Errorf("EthSignerErrorCustomError")
var ErrEthSignerErrorRpcSignError = fmt.Errorf("EthSignerErrorRpcSignError")

// Variant structs
type EthSignerErrorInvalidEthSigner struct {
	message string
}

func NewEthSignerErrorInvalidEthSigner() *EthSignerError {
	return &EthSignerError{err: &EthSignerErrorInvalidEthSigner{}}
}

func (e EthSignerErrorInvalidEthSigner) destroy() {
}

func (err EthSignerErrorInvalidEthSigner) Error() string {
	return fmt.Sprintf("InvalidEthSigner: %s", err.message)
}

func (self EthSignerErrorInvalidEthSigner) Is(target error) bool {
	return target == ErrEthSignerErrorInvalidEthSigner
}

type EthSignerErrorMissingEthPrivateKey struct {
	message string
}

func NewEthSignerErrorMissingEthPrivateKey() *EthSignerError {
	return &EthSignerError{err: &EthSignerErrorMissingEthPrivateKey{}}
}

func (e EthSignerErrorMissingEthPrivateKey) destroy() {
}

func (err EthSignerErrorMissingEthPrivateKey) Error() string {
	return fmt.Sprintf("MissingEthPrivateKey: %s", err.message)
}

func (self EthSignerErrorMissingEthPrivateKey) Is(target error) bool {
	return target == ErrEthSignerErrorMissingEthPrivateKey
}

type EthSignerErrorMissingEthSigner struct {
	message string
}

func NewEthSignerErrorMissingEthSigner() *EthSignerError {
	return &EthSignerError{err: &EthSignerErrorMissingEthSigner{}}
}

func (e EthSignerErrorMissingEthSigner) destroy() {
}

func (err EthSignerErrorMissingEthSigner) Error() string {
	return fmt.Sprintf("MissingEthSigner: %s", err.message)
}

func (self EthSignerErrorMissingEthSigner) Is(target error) bool {
	return target == ErrEthSignerErrorMissingEthSigner
}

type EthSignerErrorSigningFailed struct {
	message string
}

func NewEthSignerErrorSigningFailed() *EthSignerError {
	return &EthSignerError{err: &EthSignerErrorSigningFailed{}}
}

func (e EthSignerErrorSigningFailed) destroy() {
}

func (err EthSignerErrorSigningFailed) Error() string {
	return fmt.Sprintf("SigningFailed: %s", err.message)
}

func (self EthSignerErrorSigningFailed) Is(target error) bool {
	return target == ErrEthSignerErrorSigningFailed
}

type EthSignerErrorUnlockingFailed struct {
	message string
}

func NewEthSignerErrorUnlockingFailed() *EthSignerError {
	return &EthSignerError{err: &EthSignerErrorUnlockingFailed{}}
}

func (e EthSignerErrorUnlockingFailed) destroy() {
}

func (err EthSignerErrorUnlockingFailed) Error() string {
	return fmt.Sprintf("UnlockingFailed: %s", err.message)
}

func (self EthSignerErrorUnlockingFailed) Is(target error) bool {
	return target == ErrEthSignerErrorUnlockingFailed
}

type EthSignerErrorInvalidRawTx struct {
	message string
}

func NewEthSignerErrorInvalidRawTx() *EthSignerError {
	return &EthSignerError{err: &EthSignerErrorInvalidRawTx{}}
}

func (e EthSignerErrorInvalidRawTx) destroy() {
}

func (err EthSignerErrorInvalidRawTx) Error() string {
	return fmt.Sprintf("InvalidRawTx: %s", err.message)
}

func (self EthSignerErrorInvalidRawTx) Is(target error) bool {
	return target == ErrEthSignerErrorInvalidRawTx
}

type EthSignerErrorEip712Failed struct {
	message string
}

func NewEthSignerErrorEip712Failed() *EthSignerError {
	return &EthSignerError{err: &EthSignerErrorEip712Failed{}}
}

func (e EthSignerErrorEip712Failed) destroy() {
}

func (err EthSignerErrorEip712Failed) Error() string {
	return fmt.Sprintf("Eip712Failed: %s", err.message)
}

func (self EthSignerErrorEip712Failed) Is(target error) bool {
	return target == ErrEthSignerErrorEip712Failed
}

type EthSignerErrorNoSigningKey struct {
	message string
}

func NewEthSignerErrorNoSigningKey() *EthSignerError {
	return &EthSignerError{err: &EthSignerErrorNoSigningKey{}}
}

func (e EthSignerErrorNoSigningKey) destroy() {
}

func (err EthSignerErrorNoSigningKey) Error() string {
	return fmt.Sprintf("NoSigningKey: %s", err.message)
}

func (self EthSignerErrorNoSigningKey) Is(target error) bool {
	return target == ErrEthSignerErrorNoSigningKey
}

type EthSignerErrorDefineAddress struct {
	message string
}

func NewEthSignerErrorDefineAddress() *EthSignerError {
	return &EthSignerError{err: &EthSignerErrorDefineAddress{}}
}

func (e EthSignerErrorDefineAddress) destroy() {
}

func (err EthSignerErrorDefineAddress) Error() string {
	return fmt.Sprintf("DefineAddress: %s", err.message)
}

func (self EthSignerErrorDefineAddress) Is(target error) bool {
	return target == ErrEthSignerErrorDefineAddress
}

type EthSignerErrorRecoverAddress struct {
	message string
}

func NewEthSignerErrorRecoverAddress() *EthSignerError {
	return &EthSignerError{err: &EthSignerErrorRecoverAddress{}}
}

func (e EthSignerErrorRecoverAddress) destroy() {
}

func (err EthSignerErrorRecoverAddress) Error() string {
	return fmt.Sprintf("RecoverAddress: %s", err.message)
}

func (self EthSignerErrorRecoverAddress) Is(target error) bool {
	return target == ErrEthSignerErrorRecoverAddress
}

type EthSignerErrorLengthMismatched struct {
	message string
}

func NewEthSignerErrorLengthMismatched() *EthSignerError {
	return &EthSignerError{err: &EthSignerErrorLengthMismatched{}}
}

func (e EthSignerErrorLengthMismatched) destroy() {
}

func (err EthSignerErrorLengthMismatched) Error() string {
	return fmt.Sprintf("LengthMismatched: %s", err.message)
}

func (self EthSignerErrorLengthMismatched) Is(target error) bool {
	return target == ErrEthSignerErrorLengthMismatched
}

type EthSignerErrorCryptoError struct {
	message string
}

func NewEthSignerErrorCryptoError() *EthSignerError {
	return &EthSignerError{err: &EthSignerErrorCryptoError{}}
}

func (e EthSignerErrorCryptoError) destroy() {
}

func (err EthSignerErrorCryptoError) Error() string {
	return fmt.Sprintf("CryptoError: %s", err.message)
}

func (self EthSignerErrorCryptoError) Is(target error) bool {
	return target == ErrEthSignerErrorCryptoError
}

type EthSignerErrorInvalidSignatureStr struct {
	message string
}

func NewEthSignerErrorInvalidSignatureStr() *EthSignerError {
	return &EthSignerError{err: &EthSignerErrorInvalidSignatureStr{}}
}

func (e EthSignerErrorInvalidSignatureStr) destroy() {
}

func (err EthSignerErrorInvalidSignatureStr) Error() string {
	return fmt.Sprintf("InvalidSignatureStr: %s", err.message)
}

func (self EthSignerErrorInvalidSignatureStr) Is(target error) bool {
	return target == ErrEthSignerErrorInvalidSignatureStr
}

type EthSignerErrorCustomError struct {
	message string
}

func NewEthSignerErrorCustomError() *EthSignerError {
	return &EthSignerError{err: &EthSignerErrorCustomError{}}
}

func (e EthSignerErrorCustomError) destroy() {
}

func (err EthSignerErrorCustomError) Error() string {
	return fmt.Sprintf("CustomError: %s", err.message)
}

func (self EthSignerErrorCustomError) Is(target error) bool {
	return target == ErrEthSignerErrorCustomError
}

type EthSignerErrorRpcSignError struct {
	message string
}

func NewEthSignerErrorRpcSignError() *EthSignerError {
	return &EthSignerError{err: &EthSignerErrorRpcSignError{}}
}

func (e EthSignerErrorRpcSignError) destroy() {
}

func (err EthSignerErrorRpcSignError) Error() string {
	return fmt.Sprintf("RpcSignError: %s", err.message)
}

func (self EthSignerErrorRpcSignError) Is(target error) bool {
	return target == ErrEthSignerErrorRpcSignError
}

type FfiConverterEthSignerError struct{}

var FfiConverterEthSignerErrorINSTANCE = FfiConverterEthSignerError{}

func (c FfiConverterEthSignerError) Lift(eb RustBufferI) *EthSignerError {
	return LiftFromRustBuffer[*EthSignerError](c, eb)
}

func (c FfiConverterEthSignerError) Lower(value *EthSignerError) C.RustBuffer {
	return LowerIntoRustBuffer[*EthSignerError](c, value)
}

func (c FfiConverterEthSignerError) Read(reader io.Reader) *EthSignerError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &EthSignerError{&EthSignerErrorInvalidEthSigner{message}}
	case 2:
		return &EthSignerError{&EthSignerErrorMissingEthPrivateKey{message}}
	case 3:
		return &EthSignerError{&EthSignerErrorMissingEthSigner{message}}
	case 4:
		return &EthSignerError{&EthSignerErrorSigningFailed{message}}
	case 5:
		return &EthSignerError{&EthSignerErrorUnlockingFailed{message}}
	case 6:
		return &EthSignerError{&EthSignerErrorInvalidRawTx{message}}
	case 7:
		return &EthSignerError{&EthSignerErrorEip712Failed{message}}
	case 8:
		return &EthSignerError{&EthSignerErrorNoSigningKey{message}}
	case 9:
		return &EthSignerError{&EthSignerErrorDefineAddress{message}}
	case 10:
		return &EthSignerError{&EthSignerErrorRecoverAddress{message}}
	case 11:
		return &EthSignerError{&EthSignerErrorLengthMismatched{message}}
	case 12:
		return &EthSignerError{&EthSignerErrorCryptoError{message}}
	case 13:
		return &EthSignerError{&EthSignerErrorInvalidSignatureStr{message}}
	case 14:
		return &EthSignerError{&EthSignerErrorCustomError{message}}
	case 15:
		return &EthSignerError{&EthSignerErrorRpcSignError{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterEthSignerError.Read()", errorID))
	}

}

func (c FfiConverterEthSignerError) Write(writer io.Writer, value *EthSignerError) {
	switch variantValue := value.err.(type) {
	case *EthSignerErrorInvalidEthSigner:
		writeInt32(writer, 1)
	case *EthSignerErrorMissingEthPrivateKey:
		writeInt32(writer, 2)
	case *EthSignerErrorMissingEthSigner:
		writeInt32(writer, 3)
	case *EthSignerErrorSigningFailed:
		writeInt32(writer, 4)
	case *EthSignerErrorUnlockingFailed:
		writeInt32(writer, 5)
	case *EthSignerErrorInvalidRawTx:
		writeInt32(writer, 6)
	case *EthSignerErrorEip712Failed:
		writeInt32(writer, 7)
	case *EthSignerErrorNoSigningKey:
		writeInt32(writer, 8)
	case *EthSignerErrorDefineAddress:
		writeInt32(writer, 9)
	case *EthSignerErrorRecoverAddress:
		writeInt32(writer, 10)
	case *EthSignerErrorLengthMismatched:
		writeInt32(writer, 11)
	case *EthSignerErrorCryptoError:
		writeInt32(writer, 12)
	case *EthSignerErrorInvalidSignatureStr:
		writeInt32(writer, 13)
	case *EthSignerErrorCustomError:
		writeInt32(writer, 14)
	case *EthSignerErrorRpcSignError:
		writeInt32(writer, 15)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterEthSignerError.Write", value))
	}
}

type FfiDestroyerEthSignerError struct{}

func (_ FfiDestroyerEthSignerError) Destroy(value *EthSignerError) {
	switch variantValue := value.err.(type) {
	case EthSignerErrorInvalidEthSigner:
		variantValue.destroy()
	case EthSignerErrorMissingEthPrivateKey:
		variantValue.destroy()
	case EthSignerErrorMissingEthSigner:
		variantValue.destroy()
	case EthSignerErrorSigningFailed:
		variantValue.destroy()
	case EthSignerErrorUnlockingFailed:
		variantValue.destroy()
	case EthSignerErrorInvalidRawTx:
		variantValue.destroy()
	case EthSignerErrorEip712Failed:
		variantValue.destroy()
	case EthSignerErrorNoSigningKey:
		variantValue.destroy()
	case EthSignerErrorDefineAddress:
		variantValue.destroy()
	case EthSignerErrorRecoverAddress:
		variantValue.destroy()
	case EthSignerErrorLengthMismatched:
		variantValue.destroy()
	case EthSignerErrorCryptoError:
		variantValue.destroy()
	case EthSignerErrorInvalidSignatureStr:
		variantValue.destroy()
	case EthSignerErrorCustomError:
		variantValue.destroy()
	case EthSignerErrorRpcSignError:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerEthSignerError.Destroy", value))
	}
}

type L1SignerType interface {
	Destroy()
}
type L1SignerTypeEth struct {
}

func (e L1SignerTypeEth) Destroy() {
}

type L1SignerTypeStarknet struct {
	ChainId string
	Address string
}

func (e L1SignerTypeStarknet) Destroy() {
	FfiDestroyerString{}.Destroy(e.ChainId)
	FfiDestroyerString{}.Destroy(e.Address)
}

type FfiConverterL1SignerType struct{}

var FfiConverterL1SignerTypeINSTANCE = FfiConverterL1SignerType{}

func (c FfiConverterL1SignerType) Lift(rb RustBufferI) L1SignerType {
	return LiftFromRustBuffer[L1SignerType](c, rb)
}

func (c FfiConverterL1SignerType) Lower(value L1SignerType) C.RustBuffer {
	return LowerIntoRustBuffer[L1SignerType](c, value)
}
func (FfiConverterL1SignerType) Read(reader io.Reader) L1SignerType {
	id := readInt32(reader)
	switch id {
	case 1:
		return L1SignerTypeEth{}
	case 2:
		return L1SignerTypeStarknet{
			FfiConverterStringINSTANCE.Read(reader),
			FfiConverterStringINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterL1SignerType.Read()", id))
	}
}

func (FfiConverterL1SignerType) Write(writer io.Writer, value L1SignerType) {
	switch variant_value := value.(type) {
	case L1SignerTypeEth:
		writeInt32(writer, 1)
	case L1SignerTypeStarknet:
		writeInt32(writer, 2)
		FfiConverterStringINSTANCE.Write(writer, variant_value.ChainId)
		FfiConverterStringINSTANCE.Write(writer, variant_value.Address)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterL1SignerType.Write", value))
	}
}

type FfiDestroyerL1SignerType struct{}

func (_ FfiDestroyerL1SignerType) Destroy(value L1SignerType) {
	value.Destroy()
}

type L1Type uint

const (
	L1TypeEth      L1Type = 1
	L1TypeStarknet L1Type = 2
)

type FfiConverterL1Type struct{}

var FfiConverterL1TypeINSTANCE = FfiConverterL1Type{}

func (c FfiConverterL1Type) Lift(rb RustBufferI) L1Type {
	return LiftFromRustBuffer[L1Type](c, rb)
}

func (c FfiConverterL1Type) Lower(value L1Type) C.RustBuffer {
	return LowerIntoRustBuffer[L1Type](c, value)
}
func (FfiConverterL1Type) Read(reader io.Reader) L1Type {
	id := readInt32(reader)
	return L1Type(id)
}

func (FfiConverterL1Type) Write(writer io.Writer, value L1Type) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerL1Type struct{}

func (_ FfiDestroyerL1Type) Destroy(value L1Type) {
}

type Parameter interface {
	Destroy()
}
type ParameterFeeAccount struct {
	AccountId AccountId
}

func (e ParameterFeeAccount) Destroy() {
	FfiDestroyerTypeAccountId{}.Destroy(e.AccountId)
}

type ParameterInsuranceFundAccount struct {
	AccountId AccountId
}

func (e ParameterInsuranceFundAccount) Destroy() {
	FfiDestroyerTypeAccountId{}.Destroy(e.AccountId)
}

type ParameterMarginInfo struct {
	MarginId MarginId
	TokenId  TokenId
	Ratio    uint8
}

func (e ParameterMarginInfo) Destroy() {
	FfiDestroyerTypeMarginId{}.Destroy(e.MarginId)
	FfiDestroyerTypeTokenId{}.Destroy(e.TokenId)
	FfiDestroyerUint8{}.Destroy(e.Ratio)
}

type ParameterFundingInfos struct {
	Infos []FundingInfo
}

func (e ParameterFundingInfos) Destroy() {
	FfiDestroyerSequenceFundingInfo{}.Destroy(e.Infos)
}

type ParameterContractInfo struct {
	PairId                PairId
	Symbol                string
	InitialMarginRate     uint16
	MaintenanceMarginRate uint16
}

func (e ParameterContractInfo) Destroy() {
	FfiDestroyerTypePairId{}.Destroy(e.PairId)
	FfiDestroyerString{}.Destroy(e.Symbol)
	FfiDestroyerUint16{}.Destroy(e.InitialMarginRate)
	FfiDestroyerUint16{}.Destroy(e.MaintenanceMarginRate)
}

type FfiConverterParameter struct{}

var FfiConverterParameterINSTANCE = FfiConverterParameter{}

func (c FfiConverterParameter) Lift(rb RustBufferI) Parameter {
	return LiftFromRustBuffer[Parameter](c, rb)
}

func (c FfiConverterParameter) Lower(value Parameter) C.RustBuffer {
	return LowerIntoRustBuffer[Parameter](c, value)
}
func (FfiConverterParameter) Read(reader io.Reader) Parameter {
	id := readInt32(reader)
	switch id {
	case 1:
		return ParameterFeeAccount{
			FfiConverterTypeAccountIdINSTANCE.Read(reader),
		}
	case 2:
		return ParameterInsuranceFundAccount{
			FfiConverterTypeAccountIdINSTANCE.Read(reader),
		}
	case 3:
		return ParameterMarginInfo{
			FfiConverterTypeMarginIdINSTANCE.Read(reader),
			FfiConverterTypeTokenIdINSTANCE.Read(reader),
			FfiConverterUint8INSTANCE.Read(reader),
		}
	case 4:
		return ParameterFundingInfos{
			FfiConverterSequenceFundingInfoINSTANCE.Read(reader),
		}
	case 5:
		return ParameterContractInfo{
			FfiConverterTypePairIdINSTANCE.Read(reader),
			FfiConverterStringINSTANCE.Read(reader),
			FfiConverterUint16INSTANCE.Read(reader),
			FfiConverterUint16INSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterParameter.Read()", id))
	}
}

func (FfiConverterParameter) Write(writer io.Writer, value Parameter) {
	switch variant_value := value.(type) {
	case ParameterFeeAccount:
		writeInt32(writer, 1)
		FfiConverterTypeAccountIdINSTANCE.Write(writer, variant_value.AccountId)
	case ParameterInsuranceFundAccount:
		writeInt32(writer, 2)
		FfiConverterTypeAccountIdINSTANCE.Write(writer, variant_value.AccountId)
	case ParameterMarginInfo:
		writeInt32(writer, 3)
		FfiConverterTypeMarginIdINSTANCE.Write(writer, variant_value.MarginId)
		FfiConverterTypeTokenIdINSTANCE.Write(writer, variant_value.TokenId)
		FfiConverterUint8INSTANCE.Write(writer, variant_value.Ratio)
	case ParameterFundingInfos:
		writeInt32(writer, 4)
		FfiConverterSequenceFundingInfoINSTANCE.Write(writer, variant_value.Infos)
	case ParameterContractInfo:
		writeInt32(writer, 5)
		FfiConverterTypePairIdINSTANCE.Write(writer, variant_value.PairId)
		FfiConverterStringINSTANCE.Write(writer, variant_value.Symbol)
		FfiConverterUint16INSTANCE.Write(writer, variant_value.InitialMarginRate)
		FfiConverterUint16INSTANCE.Write(writer, variant_value.MaintenanceMarginRate)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterParameter.Write", value))
	}
}

type FfiDestroyerParameter struct{}

func (_ FfiDestroyerParameter) Destroy(value Parameter) {
	value.Destroy()
}

type SignError struct {
	err error
}

// Convience method to turn *SignError into error
// Avoiding treating nil pointer as non nil error interface
func (err *SignError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err SignError) Error() string {
	return fmt.Sprintf("SignError: %s", err.err.Error())
}

func (err SignError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrSignErrorEthSigningError = fmt.Errorf("SignErrorEthSigningError")
var ErrSignErrorZkSigningError = fmt.Errorf("SignErrorZkSigningError")
var ErrSignErrorStarkSigningError = fmt.Errorf("SignErrorStarkSigningError")
var ErrSignErrorIncorrectTx = fmt.Errorf("SignErrorIncorrectTx")

// Variant structs
type SignErrorEthSigningError struct {
	message string
}

func NewSignErrorEthSigningError() *SignError {
	return &SignError{err: &SignErrorEthSigningError{}}
}

func (e SignErrorEthSigningError) destroy() {
}

func (err SignErrorEthSigningError) Error() string {
	return fmt.Sprintf("EthSigningError: %s", err.message)
}

func (self SignErrorEthSigningError) Is(target error) bool {
	return target == ErrSignErrorEthSigningError
}

type SignErrorZkSigningError struct {
	message string
}

func NewSignErrorZkSigningError() *SignError {
	return &SignError{err: &SignErrorZkSigningError{}}
}

func (e SignErrorZkSigningError) destroy() {
}

func (err SignErrorZkSigningError) Error() string {
	return fmt.Sprintf("ZkSigningError: %s", err.message)
}

func (self SignErrorZkSigningError) Is(target error) bool {
	return target == ErrSignErrorZkSigningError
}

type SignErrorStarkSigningError struct {
	message string
}

func NewSignErrorStarkSigningError() *SignError {
	return &SignError{err: &SignErrorStarkSigningError{}}
}

func (e SignErrorStarkSigningError) destroy() {
}

func (err SignErrorStarkSigningError) Error() string {
	return fmt.Sprintf("StarkSigningError: %s", err.message)
}

func (self SignErrorStarkSigningError) Is(target error) bool {
	return target == ErrSignErrorStarkSigningError
}

type SignErrorIncorrectTx struct {
	message string
}

func NewSignErrorIncorrectTx() *SignError {
	return &SignError{err: &SignErrorIncorrectTx{}}
}

func (e SignErrorIncorrectTx) destroy() {
}

func (err SignErrorIncorrectTx) Error() string {
	return fmt.Sprintf("IncorrectTx: %s", err.message)
}

func (self SignErrorIncorrectTx) Is(target error) bool {
	return target == ErrSignErrorIncorrectTx
}

type FfiConverterSignError struct{}

var FfiConverterSignErrorINSTANCE = FfiConverterSignError{}

func (c FfiConverterSignError) Lift(eb RustBufferI) *SignError {
	return LiftFromRustBuffer[*SignError](c, eb)
}

func (c FfiConverterSignError) Lower(value *SignError) C.RustBuffer {
	return LowerIntoRustBuffer[*SignError](c, value)
}

func (c FfiConverterSignError) Read(reader io.Reader) *SignError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &SignError{&SignErrorEthSigningError{message}}
	case 2:
		return &SignError{&SignErrorZkSigningError{message}}
	case 3:
		return &SignError{&SignErrorStarkSigningError{message}}
	case 4:
		return &SignError{&SignErrorIncorrectTx{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterSignError.Read()", errorID))
	}

}

func (c FfiConverterSignError) Write(writer io.Writer, value *SignError) {
	switch variantValue := value.err.(type) {
	case *SignErrorEthSigningError:
		writeInt32(writer, 1)
	case *SignErrorZkSigningError:
		writeInt32(writer, 2)
	case *SignErrorStarkSigningError:
		writeInt32(writer, 3)
	case *SignErrorIncorrectTx:
		writeInt32(writer, 4)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterSignError.Write", value))
	}
}

type FfiDestroyerSignError struct{}

func (_ FfiDestroyerSignError) Destroy(value *SignError) {
	switch variantValue := value.err.(type) {
	case SignErrorEthSigningError:
		variantValue.destroy()
	case SignErrorZkSigningError:
		variantValue.destroy()
	case SignErrorStarkSigningError:
		variantValue.destroy()
	case SignErrorIncorrectTx:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerSignError.Destroy", value))
	}
}

type StarkSignerError struct {
	err error
}

// Convience method to turn *StarkSignerError into error
// Avoiding treating nil pointer as non nil error interface
func (err *StarkSignerError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err StarkSignerError) Error() string {
	return fmt.Sprintf("StarkSignerError: %s", err.err.Error())
}

func (err StarkSignerError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrStarkSignerErrorInvalidStarknetSigner = fmt.Errorf("StarkSignerErrorInvalidStarknetSigner")
var ErrStarkSignerErrorInvalidSignature = fmt.Errorf("StarkSignerErrorInvalidSignature")
var ErrStarkSignerErrorInvalidPrivKey = fmt.Errorf("StarkSignerErrorInvalidPrivKey")
var ErrStarkSignerErrorSignError = fmt.Errorf("StarkSignerErrorSignError")
var ErrStarkSignerErrorRpcSignError = fmt.Errorf("StarkSignerErrorRpcSignError")

// Variant structs
type StarkSignerErrorInvalidStarknetSigner struct {
	message string
}

func NewStarkSignerErrorInvalidStarknetSigner() *StarkSignerError {
	return &StarkSignerError{err: &StarkSignerErrorInvalidStarknetSigner{}}
}

func (e StarkSignerErrorInvalidStarknetSigner) destroy() {
}

func (err StarkSignerErrorInvalidStarknetSigner) Error() string {
	return fmt.Sprintf("InvalidStarknetSigner: %s", err.message)
}

func (self StarkSignerErrorInvalidStarknetSigner) Is(target error) bool {
	return target == ErrStarkSignerErrorInvalidStarknetSigner
}

type StarkSignerErrorInvalidSignature struct {
	message string
}

func NewStarkSignerErrorInvalidSignature() *StarkSignerError {
	return &StarkSignerError{err: &StarkSignerErrorInvalidSignature{}}
}

func (e StarkSignerErrorInvalidSignature) destroy() {
}

func (err StarkSignerErrorInvalidSignature) Error() string {
	return fmt.Sprintf("InvalidSignature: %s", err.message)
}

func (self StarkSignerErrorInvalidSignature) Is(target error) bool {
	return target == ErrStarkSignerErrorInvalidSignature
}

type StarkSignerErrorInvalidPrivKey struct {
	message string
}

func NewStarkSignerErrorInvalidPrivKey() *StarkSignerError {
	return &StarkSignerError{err: &StarkSignerErrorInvalidPrivKey{}}
}

func (e StarkSignerErrorInvalidPrivKey) destroy() {
}

func (err StarkSignerErrorInvalidPrivKey) Error() string {
	return fmt.Sprintf("InvalidPrivKey: %s", err.message)
}

func (self StarkSignerErrorInvalidPrivKey) Is(target error) bool {
	return target == ErrStarkSignerErrorInvalidPrivKey
}

type StarkSignerErrorSignError struct {
	message string
}

func NewStarkSignerErrorSignError() *StarkSignerError {
	return &StarkSignerError{err: &StarkSignerErrorSignError{}}
}

func (e StarkSignerErrorSignError) destroy() {
}

func (err StarkSignerErrorSignError) Error() string {
	return fmt.Sprintf("SignError: %s", err.message)
}

func (self StarkSignerErrorSignError) Is(target error) bool {
	return target == ErrStarkSignerErrorSignError
}

type StarkSignerErrorRpcSignError struct {
	message string
}

func NewStarkSignerErrorRpcSignError() *StarkSignerError {
	return &StarkSignerError{err: &StarkSignerErrorRpcSignError{}}
}

func (e StarkSignerErrorRpcSignError) destroy() {
}

func (err StarkSignerErrorRpcSignError) Error() string {
	return fmt.Sprintf("RpcSignError: %s", err.message)
}

func (self StarkSignerErrorRpcSignError) Is(target error) bool {
	return target == ErrStarkSignerErrorRpcSignError
}

type FfiConverterStarkSignerError struct{}

var FfiConverterStarkSignerErrorINSTANCE = FfiConverterStarkSignerError{}

func (c FfiConverterStarkSignerError) Lift(eb RustBufferI) *StarkSignerError {
	return LiftFromRustBuffer[*StarkSignerError](c, eb)
}

func (c FfiConverterStarkSignerError) Lower(value *StarkSignerError) C.RustBuffer {
	return LowerIntoRustBuffer[*StarkSignerError](c, value)
}

func (c FfiConverterStarkSignerError) Read(reader io.Reader) *StarkSignerError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &StarkSignerError{&StarkSignerErrorInvalidStarknetSigner{message}}
	case 2:
		return &StarkSignerError{&StarkSignerErrorInvalidSignature{message}}
	case 3:
		return &StarkSignerError{&StarkSignerErrorInvalidPrivKey{message}}
	case 4:
		return &StarkSignerError{&StarkSignerErrorSignError{message}}
	case 5:
		return &StarkSignerError{&StarkSignerErrorRpcSignError{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterStarkSignerError.Read()", errorID))
	}

}

func (c FfiConverterStarkSignerError) Write(writer io.Writer, value *StarkSignerError) {
	switch variantValue := value.err.(type) {
	case *StarkSignerErrorInvalidStarknetSigner:
		writeInt32(writer, 1)
	case *StarkSignerErrorInvalidSignature:
		writeInt32(writer, 2)
	case *StarkSignerErrorInvalidPrivKey:
		writeInt32(writer, 3)
	case *StarkSignerErrorSignError:
		writeInt32(writer, 4)
	case *StarkSignerErrorRpcSignError:
		writeInt32(writer, 5)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterStarkSignerError.Write", value))
	}
}

type FfiDestroyerStarkSignerError struct{}

func (_ FfiDestroyerStarkSignerError) Destroy(value *StarkSignerError) {
	switch variantValue := value.err.(type) {
	case StarkSignerErrorInvalidStarknetSigner:
		variantValue.destroy()
	case StarkSignerErrorInvalidSignature:
		variantValue.destroy()
	case StarkSignerErrorInvalidPrivKey:
		variantValue.destroy()
	case StarkSignerErrorSignError:
		variantValue.destroy()
	case StarkSignerErrorRpcSignError:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerStarkSignerError.Destroy", value))
	}
}

type TypeError struct {
	err error
}

// Convience method to turn *TypeError into error
// Avoiding treating nil pointer as non nil error interface
func (err *TypeError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err TypeError) Error() string {
	return fmt.Sprintf("TypeError: %s", err.err.Error())
}

func (err TypeError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrTypeErrorInvalidAddress = fmt.Errorf("TypeErrorInvalidAddress")
var ErrTypeErrorInvalidTxHash = fmt.Errorf("TypeErrorInvalidTxHash")
var ErrTypeErrorNotStartWithZerox = fmt.Errorf("TypeErrorNotStartWithZerox")
var ErrTypeErrorSizeMismatch = fmt.Errorf("TypeErrorSizeMismatch")
var ErrTypeErrorDecodeFromHexErr = fmt.Errorf("TypeErrorDecodeFromHexErr")
var ErrTypeErrorTooBigInteger = fmt.Errorf("TypeErrorTooBigInteger")
var ErrTypeErrorInvalidBigIntStr = fmt.Errorf("TypeErrorInvalidBigIntStr")

// Variant structs
type TypeErrorInvalidAddress struct {
	message string
}

func NewTypeErrorInvalidAddress() *TypeError {
	return &TypeError{err: &TypeErrorInvalidAddress{}}
}

func (e TypeErrorInvalidAddress) destroy() {
}

func (err TypeErrorInvalidAddress) Error() string {
	return fmt.Sprintf("InvalidAddress: %s", err.message)
}

func (self TypeErrorInvalidAddress) Is(target error) bool {
	return target == ErrTypeErrorInvalidAddress
}

type TypeErrorInvalidTxHash struct {
	message string
}

func NewTypeErrorInvalidTxHash() *TypeError {
	return &TypeError{err: &TypeErrorInvalidTxHash{}}
}

func (e TypeErrorInvalidTxHash) destroy() {
}

func (err TypeErrorInvalidTxHash) Error() string {
	return fmt.Sprintf("InvalidTxHash: %s", err.message)
}

func (self TypeErrorInvalidTxHash) Is(target error) bool {
	return target == ErrTypeErrorInvalidTxHash
}

type TypeErrorNotStartWithZerox struct {
	message string
}

func NewTypeErrorNotStartWithZerox() *TypeError {
	return &TypeError{err: &TypeErrorNotStartWithZerox{}}
}

func (e TypeErrorNotStartWithZerox) destroy() {
}

func (err TypeErrorNotStartWithZerox) Error() string {
	return fmt.Sprintf("NotStartWithZerox: %s", err.message)
}

func (self TypeErrorNotStartWithZerox) Is(target error) bool {
	return target == ErrTypeErrorNotStartWithZerox
}

type TypeErrorSizeMismatch struct {
	message string
}

func NewTypeErrorSizeMismatch() *TypeError {
	return &TypeError{err: &TypeErrorSizeMismatch{}}
}

func (e TypeErrorSizeMismatch) destroy() {
}

func (err TypeErrorSizeMismatch) Error() string {
	return fmt.Sprintf("SizeMismatch: %s", err.message)
}

func (self TypeErrorSizeMismatch) Is(target error) bool {
	return target == ErrTypeErrorSizeMismatch
}

type TypeErrorDecodeFromHexErr struct {
	message string
}

func NewTypeErrorDecodeFromHexErr() *TypeError {
	return &TypeError{err: &TypeErrorDecodeFromHexErr{}}
}

func (e TypeErrorDecodeFromHexErr) destroy() {
}

func (err TypeErrorDecodeFromHexErr) Error() string {
	return fmt.Sprintf("DecodeFromHexErr: %s", err.message)
}

func (self TypeErrorDecodeFromHexErr) Is(target error) bool {
	return target == ErrTypeErrorDecodeFromHexErr
}

type TypeErrorTooBigInteger struct {
	message string
}

func NewTypeErrorTooBigInteger() *TypeError {
	return &TypeError{err: &TypeErrorTooBigInteger{}}
}

func (e TypeErrorTooBigInteger) destroy() {
}

func (err TypeErrorTooBigInteger) Error() string {
	return fmt.Sprintf("TooBigInteger: %s", err.message)
}

func (self TypeErrorTooBigInteger) Is(target error) bool {
	return target == ErrTypeErrorTooBigInteger
}

type TypeErrorInvalidBigIntStr struct {
	message string
}

func NewTypeErrorInvalidBigIntStr() *TypeError {
	return &TypeError{err: &TypeErrorInvalidBigIntStr{}}
}

func (e TypeErrorInvalidBigIntStr) destroy() {
}

func (err TypeErrorInvalidBigIntStr) Error() string {
	return fmt.Sprintf("InvalidBigIntStr: %s", err.message)
}

func (self TypeErrorInvalidBigIntStr) Is(target error) bool {
	return target == ErrTypeErrorInvalidBigIntStr
}

type FfiConverterTypeError struct{}

var FfiConverterTypeErrorINSTANCE = FfiConverterTypeError{}

func (c FfiConverterTypeError) Lift(eb RustBufferI) *TypeError {
	return LiftFromRustBuffer[*TypeError](c, eb)
}

func (c FfiConverterTypeError) Lower(value *TypeError) C.RustBuffer {
	return LowerIntoRustBuffer[*TypeError](c, value)
}

func (c FfiConverterTypeError) Read(reader io.Reader) *TypeError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &TypeError{&TypeErrorInvalidAddress{message}}
	case 2:
		return &TypeError{&TypeErrorInvalidTxHash{message}}
	case 3:
		return &TypeError{&TypeErrorNotStartWithZerox{message}}
	case 4:
		return &TypeError{&TypeErrorSizeMismatch{message}}
	case 5:
		return &TypeError{&TypeErrorDecodeFromHexErr{message}}
	case 6:
		return &TypeError{&TypeErrorTooBigInteger{message}}
	case 7:
		return &TypeError{&TypeErrorInvalidBigIntStr{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterTypeError.Read()", errorID))
	}

}

func (c FfiConverterTypeError) Write(writer io.Writer, value *TypeError) {
	switch variantValue := value.err.(type) {
	case *TypeErrorInvalidAddress:
		writeInt32(writer, 1)
	case *TypeErrorInvalidTxHash:
		writeInt32(writer, 2)
	case *TypeErrorNotStartWithZerox:
		writeInt32(writer, 3)
	case *TypeErrorSizeMismatch:
		writeInt32(writer, 4)
	case *TypeErrorDecodeFromHexErr:
		writeInt32(writer, 5)
	case *TypeErrorTooBigInteger:
		writeInt32(writer, 6)
	case *TypeErrorInvalidBigIntStr:
		writeInt32(writer, 7)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterTypeError.Write", value))
	}
}

type FfiDestroyerTypeError struct{}

func (_ FfiDestroyerTypeError) Destroy(value *TypeError) {
	switch variantValue := value.err.(type) {
	case TypeErrorInvalidAddress:
		variantValue.destroy()
	case TypeErrorInvalidTxHash:
		variantValue.destroy()
	case TypeErrorNotStartWithZerox:
		variantValue.destroy()
	case TypeErrorSizeMismatch:
		variantValue.destroy()
	case TypeErrorDecodeFromHexErr:
		variantValue.destroy()
	case TypeErrorTooBigInteger:
		variantValue.destroy()
	case TypeErrorInvalidBigIntStr:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerTypeError.Destroy", value))
	}
}

type TypedDataMessage interface {
	Destroy()
}
type TypedDataMessageCreateL2Key struct {
	Message Message
}

func (e TypedDataMessageCreateL2Key) Destroy() {
	FfiDestroyerMessage{}.Destroy(e.Message)
}

type TypedDataMessageTransaction struct {
	Message TxMessage
}

func (e TypedDataMessageTransaction) Destroy() {
	FfiDestroyerTxMessage{}.Destroy(e.Message)
}

type FfiConverterTypedDataMessage struct{}

var FfiConverterTypedDataMessageINSTANCE = FfiConverterTypedDataMessage{}

func (c FfiConverterTypedDataMessage) Lift(rb RustBufferI) TypedDataMessage {
	return LiftFromRustBuffer[TypedDataMessage](c, rb)
}

func (c FfiConverterTypedDataMessage) Lower(value TypedDataMessage) C.RustBuffer {
	return LowerIntoRustBuffer[TypedDataMessage](c, value)
}
func (FfiConverterTypedDataMessage) Read(reader io.Reader) TypedDataMessage {
	id := readInt32(reader)
	switch id {
	case 1:
		return TypedDataMessageCreateL2Key{
			FfiConverterMessageINSTANCE.Read(reader),
		}
	case 2:
		return TypedDataMessageTransaction{
			FfiConverterTxMessageINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterTypedDataMessage.Read()", id))
	}
}

func (FfiConverterTypedDataMessage) Write(writer io.Writer, value TypedDataMessage) {
	switch variant_value := value.(type) {
	case TypedDataMessageCreateL2Key:
		writeInt32(writer, 1)
		FfiConverterMessageINSTANCE.Write(writer, variant_value.Message)
	case TypedDataMessageTransaction:
		writeInt32(writer, 2)
		FfiConverterTxMessageINSTANCE.Write(writer, variant_value.Message)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterTypedDataMessage.Write", value))
	}
}

type FfiDestroyerTypedDataMessage struct{}

func (_ FfiDestroyerTypedDataMessage) Destroy(value TypedDataMessage) {
	value.Destroy()
}

type ZkSignerError struct {
	err error
}

// Convience method to turn *ZkSignerError into error
// Avoiding treating nil pointer as non nil error interface
func (err *ZkSignerError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err ZkSignerError) Error() string {
	return fmt.Sprintf("ZkSignerError: %s", err.err.Error())
}

func (err ZkSignerError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrZkSignerErrorCustomError = fmt.Errorf("ZkSignerErrorCustomError")
var ErrZkSignerErrorInvalidSignature = fmt.Errorf("ZkSignerErrorInvalidSignature")
var ErrZkSignerErrorInvalidPrivKey = fmt.Errorf("ZkSignerErrorInvalidPrivKey")
var ErrZkSignerErrorInvalidSeed = fmt.Errorf("ZkSignerErrorInvalidSeed")
var ErrZkSignerErrorInvalidPubkey = fmt.Errorf("ZkSignerErrorInvalidPubkey")
var ErrZkSignerErrorInvalidPubkeyHash = fmt.Errorf("ZkSignerErrorInvalidPubkeyHash")
var ErrZkSignerErrorEthSignerError = fmt.Errorf("ZkSignerErrorEthSignerError")
var ErrZkSignerErrorStarkSignerError = fmt.Errorf("ZkSignerErrorStarkSignerError")

// Variant structs
type ZkSignerErrorCustomError struct {
	message string
}

func NewZkSignerErrorCustomError() *ZkSignerError {
	return &ZkSignerError{err: &ZkSignerErrorCustomError{}}
}

func (e ZkSignerErrorCustomError) destroy() {
}

func (err ZkSignerErrorCustomError) Error() string {
	return fmt.Sprintf("CustomError: %s", err.message)
}

func (self ZkSignerErrorCustomError) Is(target error) bool {
	return target == ErrZkSignerErrorCustomError
}

type ZkSignerErrorInvalidSignature struct {
	message string
}

func NewZkSignerErrorInvalidSignature() *ZkSignerError {
	return &ZkSignerError{err: &ZkSignerErrorInvalidSignature{}}
}

func (e ZkSignerErrorInvalidSignature) destroy() {
}

func (err ZkSignerErrorInvalidSignature) Error() string {
	return fmt.Sprintf("InvalidSignature: %s", err.message)
}

func (self ZkSignerErrorInvalidSignature) Is(target error) bool {
	return target == ErrZkSignerErrorInvalidSignature
}

type ZkSignerErrorInvalidPrivKey struct {
	message string
}

func NewZkSignerErrorInvalidPrivKey() *ZkSignerError {
	return &ZkSignerError{err: &ZkSignerErrorInvalidPrivKey{}}
}

func (e ZkSignerErrorInvalidPrivKey) destroy() {
}

func (err ZkSignerErrorInvalidPrivKey) Error() string {
	return fmt.Sprintf("InvalidPrivKey: %s", err.message)
}

func (self ZkSignerErrorInvalidPrivKey) Is(target error) bool {
	return target == ErrZkSignerErrorInvalidPrivKey
}

type ZkSignerErrorInvalidSeed struct {
	message string
}

func NewZkSignerErrorInvalidSeed() *ZkSignerError {
	return &ZkSignerError{err: &ZkSignerErrorInvalidSeed{}}
}

func (e ZkSignerErrorInvalidSeed) destroy() {
}

func (err ZkSignerErrorInvalidSeed) Error() string {
	return fmt.Sprintf("InvalidSeed: %s", err.message)
}

func (self ZkSignerErrorInvalidSeed) Is(target error) bool {
	return target == ErrZkSignerErrorInvalidSeed
}

type ZkSignerErrorInvalidPubkey struct {
	message string
}

func NewZkSignerErrorInvalidPubkey() *ZkSignerError {
	return &ZkSignerError{err: &ZkSignerErrorInvalidPubkey{}}
}

func (e ZkSignerErrorInvalidPubkey) destroy() {
}

func (err ZkSignerErrorInvalidPubkey) Error() string {
	return fmt.Sprintf("InvalidPubkey: %s", err.message)
}

func (self ZkSignerErrorInvalidPubkey) Is(target error) bool {
	return target == ErrZkSignerErrorInvalidPubkey
}

type ZkSignerErrorInvalidPubkeyHash struct {
	message string
}

func NewZkSignerErrorInvalidPubkeyHash() *ZkSignerError {
	return &ZkSignerError{err: &ZkSignerErrorInvalidPubkeyHash{}}
}

func (e ZkSignerErrorInvalidPubkeyHash) destroy() {
}

func (err ZkSignerErrorInvalidPubkeyHash) Error() string {
	return fmt.Sprintf("InvalidPubkeyHash: %s", err.message)
}

func (self ZkSignerErrorInvalidPubkeyHash) Is(target error) bool {
	return target == ErrZkSignerErrorInvalidPubkeyHash
}

type ZkSignerErrorEthSignerError struct {
	message string
}

func NewZkSignerErrorEthSignerError() *ZkSignerError {
	return &ZkSignerError{err: &ZkSignerErrorEthSignerError{}}
}

func (e ZkSignerErrorEthSignerError) destroy() {
}

func (err ZkSignerErrorEthSignerError) Error() string {
	return fmt.Sprintf("EthSignerError: %s", err.message)
}

func (self ZkSignerErrorEthSignerError) Is(target error) bool {
	return target == ErrZkSignerErrorEthSignerError
}

type ZkSignerErrorStarkSignerError struct {
	message string
}

func NewZkSignerErrorStarkSignerError() *ZkSignerError {
	return &ZkSignerError{err: &ZkSignerErrorStarkSignerError{}}
}

func (e ZkSignerErrorStarkSignerError) destroy() {
}

func (err ZkSignerErrorStarkSignerError) Error() string {
	return fmt.Sprintf("StarkSignerError: %s", err.message)
}

func (self ZkSignerErrorStarkSignerError) Is(target error) bool {
	return target == ErrZkSignerErrorStarkSignerError
}

type FfiConverterZkSignerError struct{}

var FfiConverterZkSignerErrorINSTANCE = FfiConverterZkSignerError{}

func (c FfiConverterZkSignerError) Lift(eb RustBufferI) *ZkSignerError {
	return LiftFromRustBuffer[*ZkSignerError](c, eb)
}

func (c FfiConverterZkSignerError) Lower(value *ZkSignerError) C.RustBuffer {
	return LowerIntoRustBuffer[*ZkSignerError](c, value)
}

func (c FfiConverterZkSignerError) Read(reader io.Reader) *ZkSignerError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &ZkSignerError{&ZkSignerErrorCustomError{message}}
	case 2:
		return &ZkSignerError{&ZkSignerErrorInvalidSignature{message}}
	case 3:
		return &ZkSignerError{&ZkSignerErrorInvalidPrivKey{message}}
	case 4:
		return &ZkSignerError{&ZkSignerErrorInvalidSeed{message}}
	case 5:
		return &ZkSignerError{&ZkSignerErrorInvalidPubkey{message}}
	case 6:
		return &ZkSignerError{&ZkSignerErrorInvalidPubkeyHash{message}}
	case 7:
		return &ZkSignerError{&ZkSignerErrorEthSignerError{message}}
	case 8:
		return &ZkSignerError{&ZkSignerErrorStarkSignerError{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterZkSignerError.Read()", errorID))
	}

}

func (c FfiConverterZkSignerError) Write(writer io.Writer, value *ZkSignerError) {
	switch variantValue := value.err.(type) {
	case *ZkSignerErrorCustomError:
		writeInt32(writer, 1)
	case *ZkSignerErrorInvalidSignature:
		writeInt32(writer, 2)
	case *ZkSignerErrorInvalidPrivKey:
		writeInt32(writer, 3)
	case *ZkSignerErrorInvalidSeed:
		writeInt32(writer, 4)
	case *ZkSignerErrorInvalidPubkey:
		writeInt32(writer, 5)
	case *ZkSignerErrorInvalidPubkeyHash:
		writeInt32(writer, 6)
	case *ZkSignerErrorEthSignerError:
		writeInt32(writer, 7)
	case *ZkSignerErrorStarkSignerError:
		writeInt32(writer, 8)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterZkSignerError.Write", value))
	}
}

type FfiDestroyerZkSignerError struct{}

func (_ FfiDestroyerZkSignerError) Destroy(value *ZkSignerError) {
	switch variantValue := value.err.(type) {
	case ZkSignerErrorCustomError:
		variantValue.destroy()
	case ZkSignerErrorInvalidSignature:
		variantValue.destroy()
	case ZkSignerErrorInvalidPrivKey:
		variantValue.destroy()
	case ZkSignerErrorInvalidSeed:
		variantValue.destroy()
	case ZkSignerErrorInvalidPubkey:
		variantValue.destroy()
	case ZkSignerErrorInvalidPubkeyHash:
		variantValue.destroy()
	case ZkSignerErrorEthSignerError:
		variantValue.destroy()
	case ZkSignerErrorStarkSignerError:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerZkSignerError.Destroy", value))
	}
}

type FfiConverterOptionalString struct{}

var FfiConverterOptionalStringINSTANCE = FfiConverterOptionalString{}

func (c FfiConverterOptionalString) Lift(rb RustBufferI) *string {
	return LiftFromRustBuffer[*string](c, rb)
}

func (_ FfiConverterOptionalString) Read(reader io.Reader) *string {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterStringINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalString) Lower(value *string) C.RustBuffer {
	return LowerIntoRustBuffer[*string](c, value)
}

func (_ FfiConverterOptionalString) Write(writer io.Writer, value *string) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalString struct{}

func (_ FfiDestroyerOptionalString) Destroy(value *string) {
	if value != nil {
		FfiDestroyerString{}.Destroy(*value)
	}
}

type FfiConverterOptionalZkLinkSignature struct{}

var FfiConverterOptionalZkLinkSignatureINSTANCE = FfiConverterOptionalZkLinkSignature{}

func (c FfiConverterOptionalZkLinkSignature) Lift(rb RustBufferI) *ZkLinkSignature {
	return LiftFromRustBuffer[*ZkLinkSignature](c, rb)
}

func (_ FfiConverterOptionalZkLinkSignature) Read(reader io.Reader) *ZkLinkSignature {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterZkLinkSignatureINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalZkLinkSignature) Lower(value *ZkLinkSignature) C.RustBuffer {
	return LowerIntoRustBuffer[*ZkLinkSignature](c, value)
}

func (_ FfiConverterOptionalZkLinkSignature) Write(writer io.Writer, value *ZkLinkSignature) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterZkLinkSignatureINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalZkLinkSignature struct{}

func (_ FfiDestroyerOptionalZkLinkSignature) Destroy(value *ZkLinkSignature) {
	if value != nil {
		FfiDestroyerZkLinkSignature{}.Destroy(*value)
	}
}

type FfiConverterOptionalSequenceUint8 struct{}

var FfiConverterOptionalSequenceUint8INSTANCE = FfiConverterOptionalSequenceUint8{}

func (c FfiConverterOptionalSequenceUint8) Lift(rb RustBufferI) *[]uint8 {
	return LiftFromRustBuffer[*[]uint8](c, rb)
}

func (_ FfiConverterOptionalSequenceUint8) Read(reader io.Reader) *[]uint8 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterSequenceUint8INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalSequenceUint8) Lower(value *[]uint8) C.RustBuffer {
	return LowerIntoRustBuffer[*[]uint8](c, value)
}

func (_ FfiConverterOptionalSequenceUint8) Write(writer io.Writer, value *[]uint8) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterSequenceUint8INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalSequenceUint8 struct{}

func (_ FfiDestroyerOptionalSequenceUint8) Destroy(value *[]uint8) {
	if value != nil {
		FfiDestroyerSequenceUint8{}.Destroy(*value)
	}
}

type FfiConverterOptionalTypeH256 struct{}

var FfiConverterOptionalTypeH256INSTANCE = FfiConverterOptionalTypeH256{}

func (c FfiConverterOptionalTypeH256) Lift(rb RustBufferI) *H256 {
	return LiftFromRustBuffer[*H256](c, rb)
}

func (_ FfiConverterOptionalTypeH256) Read(reader io.Reader) *H256 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterTypeH256INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalTypeH256) Lower(value *H256) C.RustBuffer {
	return LowerIntoRustBuffer[*H256](c, value)
}

func (_ FfiConverterOptionalTypeH256) Write(writer io.Writer, value *H256) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterTypeH256INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalTypeH256 struct{}

func (_ FfiDestroyerOptionalTypeH256) Destroy(value *H256) {
	if value != nil {
		FfiDestroyerTypeH256{}.Destroy(*value)
	}
}

type FfiConverterOptionalTypePackedEthSignature struct{}

var FfiConverterOptionalTypePackedEthSignatureINSTANCE = FfiConverterOptionalTypePackedEthSignature{}

func (c FfiConverterOptionalTypePackedEthSignature) Lift(rb RustBufferI) *PackedEthSignature {
	return LiftFromRustBuffer[*PackedEthSignature](c, rb)
}

func (_ FfiConverterOptionalTypePackedEthSignature) Read(reader io.Reader) *PackedEthSignature {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterTypePackedEthSignatureINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalTypePackedEthSignature) Lower(value *PackedEthSignature) C.RustBuffer {
	return LowerIntoRustBuffer[*PackedEthSignature](c, value)
}

func (_ FfiConverterOptionalTypePackedEthSignature) Write(writer io.Writer, value *PackedEthSignature) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterTypePackedEthSignatureINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalTypePackedEthSignature struct{}

func (_ FfiDestroyerOptionalTypePackedEthSignature) Destroy(value *PackedEthSignature) {
	if value != nil {
		FfiDestroyerTypePackedEthSignature{}.Destroy(*value)
	}
}

type FfiConverterOptionalTypeTxLayer1Signature struct{}

var FfiConverterOptionalTypeTxLayer1SignatureINSTANCE = FfiConverterOptionalTypeTxLayer1Signature{}

func (c FfiConverterOptionalTypeTxLayer1Signature) Lift(rb RustBufferI) *TxLayer1Signature {
	return LiftFromRustBuffer[*TxLayer1Signature](c, rb)
}

func (_ FfiConverterOptionalTypeTxLayer1Signature) Read(reader io.Reader) *TxLayer1Signature {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterTypeTxLayer1SignatureINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalTypeTxLayer1Signature) Lower(value *TxLayer1Signature) C.RustBuffer {
	return LowerIntoRustBuffer[*TxLayer1Signature](c, value)
}

func (_ FfiConverterOptionalTypeTxLayer1Signature) Write(writer io.Writer, value *TxLayer1Signature) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterTypeTxLayer1SignatureINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalTypeTxLayer1Signature struct{}

func (_ FfiDestroyerOptionalTypeTxLayer1Signature) Destroy(value *TxLayer1Signature) {
	if value != nil {
		FfiDestroyerTypeTxLayer1Signature{}.Destroy(*value)
	}
}

type FfiConverterSequenceUint8 struct{}

var FfiConverterSequenceUint8INSTANCE = FfiConverterSequenceUint8{}

func (c FfiConverterSequenceUint8) Lift(rb RustBufferI) []uint8 {
	return LiftFromRustBuffer[[]uint8](c, rb)
}

func (c FfiConverterSequenceUint8) Read(reader io.Reader) []uint8 {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]uint8, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterUint8INSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceUint8) Lower(value []uint8) C.RustBuffer {
	return LowerIntoRustBuffer[[]uint8](c, value)
}

func (c FfiConverterSequenceUint8) Write(writer io.Writer, value []uint8) {
	if len(value) > math.MaxInt32 {
		panic("[]uint8 is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterUint8INSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceUint8 struct{}

func (FfiDestroyerSequenceUint8) Destroy(sequence []uint8) {
	for _, value := range sequence {
		FfiDestroyerUint8{}.Destroy(value)
	}
}

type FfiConverterSequenceContract struct{}

var FfiConverterSequenceContractINSTANCE = FfiConverterSequenceContract{}

func (c FfiConverterSequenceContract) Lift(rb RustBufferI) []*Contract {
	return LiftFromRustBuffer[[]*Contract](c, rb)
}

func (c FfiConverterSequenceContract) Read(reader io.Reader) []*Contract {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*Contract, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterContractINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceContract) Lower(value []*Contract) C.RustBuffer {
	return LowerIntoRustBuffer[[]*Contract](c, value)
}

func (c FfiConverterSequenceContract) Write(writer io.Writer, value []*Contract) {
	if len(value) > math.MaxInt32 {
		panic("[]*Contract is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterContractINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceContract struct{}

func (FfiDestroyerSequenceContract) Destroy(sequence []*Contract) {
	for _, value := range sequence {
		FfiDestroyerContract{}.Destroy(value)
	}
}

type FfiConverterSequenceContractPrice struct{}

var FfiConverterSequenceContractPriceINSTANCE = FfiConverterSequenceContractPrice{}

func (c FfiConverterSequenceContractPrice) Lift(rb RustBufferI) []ContractPrice {
	return LiftFromRustBuffer[[]ContractPrice](c, rb)
}

func (c FfiConverterSequenceContractPrice) Read(reader io.Reader) []ContractPrice {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]ContractPrice, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterContractPriceINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceContractPrice) Lower(value []ContractPrice) C.RustBuffer {
	return LowerIntoRustBuffer[[]ContractPrice](c, value)
}

func (c FfiConverterSequenceContractPrice) Write(writer io.Writer, value []ContractPrice) {
	if len(value) > math.MaxInt32 {
		panic("[]ContractPrice is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterContractPriceINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceContractPrice struct{}

func (FfiDestroyerSequenceContractPrice) Destroy(sequence []ContractPrice) {
	for _, value := range sequence {
		FfiDestroyerContractPrice{}.Destroy(value)
	}
}

type FfiConverterSequenceFundingInfo struct{}

var FfiConverterSequenceFundingInfoINSTANCE = FfiConverterSequenceFundingInfo{}

func (c FfiConverterSequenceFundingInfo) Lift(rb RustBufferI) []FundingInfo {
	return LiftFromRustBuffer[[]FundingInfo](c, rb)
}

func (c FfiConverterSequenceFundingInfo) Read(reader io.Reader) []FundingInfo {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]FundingInfo, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterFundingInfoINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceFundingInfo) Lower(value []FundingInfo) C.RustBuffer {
	return LowerIntoRustBuffer[[]FundingInfo](c, value)
}

func (c FfiConverterSequenceFundingInfo) Write(writer io.Writer, value []FundingInfo) {
	if len(value) > math.MaxInt32 {
		panic("[]FundingInfo is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterFundingInfoINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceFundingInfo struct{}

func (FfiDestroyerSequenceFundingInfo) Destroy(sequence []FundingInfo) {
	for _, value := range sequence {
		FfiDestroyerFundingInfo{}.Destroy(value)
	}
}

type FfiConverterSequenceSpotPriceInfo struct{}

var FfiConverterSequenceSpotPriceInfoINSTANCE = FfiConverterSequenceSpotPriceInfo{}

func (c FfiConverterSequenceSpotPriceInfo) Lift(rb RustBufferI) []SpotPriceInfo {
	return LiftFromRustBuffer[[]SpotPriceInfo](c, rb)
}

func (c FfiConverterSequenceSpotPriceInfo) Read(reader io.Reader) []SpotPriceInfo {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]SpotPriceInfo, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterSpotPriceInfoINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceSpotPriceInfo) Lower(value []SpotPriceInfo) C.RustBuffer {
	return LowerIntoRustBuffer[[]SpotPriceInfo](c, value)
}

func (c FfiConverterSequenceSpotPriceInfo) Write(writer io.Writer, value []SpotPriceInfo) {
	if len(value) > math.MaxInt32 {
		panic("[]SpotPriceInfo is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterSpotPriceInfoINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceSpotPriceInfo struct{}

func (FfiDestroyerSequenceSpotPriceInfo) Destroy(sequence []SpotPriceInfo) {
	for _, value := range sequence {
		FfiDestroyerSpotPriceInfo{}.Destroy(value)
	}
}

type FfiConverterSequenceTypeAccountId struct{}

var FfiConverterSequenceTypeAccountIdINSTANCE = FfiConverterSequenceTypeAccountId{}

func (c FfiConverterSequenceTypeAccountId) Lift(rb RustBufferI) []AccountId {
	return LiftFromRustBuffer[[]AccountId](c, rb)
}

func (c FfiConverterSequenceTypeAccountId) Read(reader io.Reader) []AccountId {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]AccountId, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTypeAccountIdINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTypeAccountId) Lower(value []AccountId) C.RustBuffer {
	return LowerIntoRustBuffer[[]AccountId](c, value)
}

func (c FfiConverterSequenceTypeAccountId) Write(writer io.Writer, value []AccountId) {
	if len(value) > math.MaxInt32 {
		panic("[]AccountId is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTypeAccountIdINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTypeAccountId struct{}

func (FfiDestroyerSequenceTypeAccountId) Destroy(sequence []AccountId) {
	for _, value := range sequence {
		FfiDestroyerTypeAccountId{}.Destroy(value)
	}
}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type AccountId = uint32
type FfiConverterTypeAccountId = FfiConverterUint32
type FfiDestroyerTypeAccountId = FfiDestroyerUint32

var FfiConverterTypeAccountIdINSTANCE = FfiConverterUint32{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type Address = string
type FfiConverterTypeAddress = FfiConverterString
type FfiDestroyerTypeAddress = FfiDestroyerString

var FfiConverterTypeAddressINSTANCE = FfiConverterString{}

/**
 * Typealias from the type name used in the UDL file to the custom type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type BigUint = big.Int

type FfiConverterTypeBigUint struct{}

var FfiConverterTypeBigUintINSTANCE = FfiConverterTypeBigUint{}

func (FfiConverterTypeBigUint) Lower(value BigUint) RustBufferI {
	builtinValue := value.String()
	ffiValue := FfiConverterStringINSTANCE.Lower(builtinValue)
	return GoRustBuffer{
		inner: ffiValue,
	}
}

func (FfiConverterTypeBigUint) Write(writer io.Writer, value BigUint) {
	builtinValue := value.String()
	FfiConverterStringINSTANCE.Write(writer, builtinValue)
}

func (FfiConverterTypeBigUint) Lift(value RustBufferI) BigUint {
	builtinValue := FfiConverterStringINSTANCE.Lift(value)
	n := new(big.Int)
	n, ok := n.SetString(builtinValue, 10)
	if !ok {
		panic("invalid big int")
	}
	return *n

}

func (FfiConverterTypeBigUint) Read(reader io.Reader) BigUint {
	builtinValue := FfiConverterStringINSTANCE.Read(reader)
	n := new(big.Int)
	n, ok := n.SetString(builtinValue, 10)
	if !ok {
		panic("invalid big int")
	}
	return *n

}

type FfiDestroyerTypeBigUint struct{}

func (FfiDestroyerTypeBigUint) Destroy(value BigUint) {
	builtinValue := value.String()
	FfiDestroyerString{}.Destroy(builtinValue)
}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type BlockNumber = uint32
type FfiConverterTypeBlockNumber = FfiConverterUint32
type FfiDestroyerTypeBlockNumber = FfiDestroyerUint32

var FfiConverterTypeBlockNumberINSTANCE = FfiConverterUint32{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type ChainId = uint8
type FfiConverterTypeChainId = FfiConverterUint8
type FfiDestroyerTypeChainId = FfiDestroyerUint8

var FfiConverterTypeChainIdINSTANCE = FfiConverterUint8{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type EthBlockId = uint64
type FfiConverterTypeEthBlockId = FfiConverterUint64
type FfiDestroyerTypeEthBlockId = FfiDestroyerUint64

var FfiConverterTypeEthBlockIdINSTANCE = FfiConverterUint64{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type H256 = string
type FfiConverterTypeH256 = FfiConverterString
type FfiDestroyerTypeH256 = FfiDestroyerString

var FfiConverterTypeH256INSTANCE = FfiConverterString{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type MarginId = uint8
type FfiConverterTypeMarginId = FfiConverterUint8
type FfiDestroyerTypeMarginId = FfiDestroyerUint8

var FfiConverterTypeMarginIdINSTANCE = FfiConverterUint8{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type Nonce = uint32
type FfiConverterTypeNonce = FfiConverterUint32
type FfiDestroyerTypeNonce = FfiDestroyerUint32

var FfiConverterTypeNonceINSTANCE = FfiConverterUint32{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type PackedEthSignature = string
type FfiConverterTypePackedEthSignature = FfiConverterString
type FfiDestroyerTypePackedEthSignature = FfiDestroyerString

var FfiConverterTypePackedEthSignatureINSTANCE = FfiConverterString{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type PackedPublicKey = string
type FfiConverterTypePackedPublicKey = FfiConverterString
type FfiDestroyerTypePackedPublicKey = FfiDestroyerString

var FfiConverterTypePackedPublicKeyINSTANCE = FfiConverterString{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type PackedSignature = string
type FfiConverterTypePackedSignature = FfiConverterString
type FfiDestroyerTypePackedSignature = FfiDestroyerString

var FfiConverterTypePackedSignatureINSTANCE = FfiConverterString{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type PairId = uint16
type FfiConverterTypePairId = FfiConverterUint16
type FfiDestroyerTypePairId = FfiDestroyerUint16

var FfiConverterTypePairIdINSTANCE = FfiConverterUint16{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type PriorityOpId = uint64
type FfiConverterTypePriorityOpId = FfiConverterUint64
type FfiDestroyerTypePriorityOpId = FfiDestroyerUint64

var FfiConverterTypePriorityOpIdINSTANCE = FfiConverterUint64{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type PubKeyHash = string
type FfiConverterTypePubKeyHash = FfiConverterString
type FfiDestroyerTypePubKeyHash = FfiDestroyerString

var FfiConverterTypePubKeyHashINSTANCE = FfiConverterString{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type SlotId = uint32
type FfiConverterTypeSlotId = FfiConverterUint32
type FfiDestroyerTypeSlotId = FfiDestroyerUint32

var FfiConverterTypeSlotIdINSTANCE = FfiConverterUint32{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type StarkEip712Signature = string
type FfiConverterTypeStarkEip712Signature = FfiConverterString
type FfiDestroyerTypeStarkEip712Signature = FfiDestroyerString

var FfiConverterTypeStarkEip712SignatureINSTANCE = FfiConverterString{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type SubAccountId = uint8
type FfiConverterTypeSubAccountId = FfiConverterUint8
type FfiDestroyerTypeSubAccountId = FfiDestroyerUint8

var FfiConverterTypeSubAccountIdINSTANCE = FfiConverterUint8{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type TimeStamp = uint32
type FfiConverterTypeTimeStamp = FfiConverterUint32
type FfiDestroyerTypeTimeStamp = FfiDestroyerUint32

var FfiConverterTypeTimeStampINSTANCE = FfiConverterUint32{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type TokenId = uint32
type FfiConverterTypeTokenId = FfiConverterUint32
type FfiDestroyerTypeTokenId = FfiDestroyerUint32

var FfiConverterTypeTokenIdINSTANCE = FfiConverterUint32{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type TxHash = string
type FfiConverterTypeTxHash = FfiConverterString
type FfiDestroyerTypeTxHash = FfiDestroyerString

var FfiConverterTypeTxHashINSTANCE = FfiConverterString{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type TxLayer1Signature = string
type FfiConverterTypeTxLayer1Signature = FfiConverterString
type FfiDestroyerTypeTxLayer1Signature = FfiDestroyerString

var FfiConverterTypeTxLayer1SignatureINSTANCE = FfiConverterString{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type ZkLinkAddress = string
type FfiConverterTypeZkLinkAddress = FfiConverterString
type FfiDestroyerTypeZkLinkAddress = FfiDestroyerString

var FfiConverterTypeZkLinkAddressINSTANCE = FfiConverterString{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type ZkLinkTx = string
type FfiConverterTypeZkLinkTx = FfiConverterString
type FfiDestroyerTypeZkLinkTx = FfiDestroyerString

var FfiConverterTypeZkLinkTxINSTANCE = FfiConverterString{}

func ClosestPackableFeeAmount(fee BigUint) BigUint {
	return FfiConverterTypeBigUintINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_func_closest_packable_fee_amount(FfiConverterTypeBigUintINSTANCE.Lower(fee).ExternalBuffer(), _uniffiStatus),
		}
	}))
}

func ClosestPackableTokenAmount(amount BigUint) BigUint {
	return FfiConverterTypeBigUintINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_func_closest_packable_token_amount(FfiConverterTypeBigUintINSTANCE.Lower(amount).ExternalBuffer(), _uniffiStatus),
		}
	}))
}

func CreateSignedChangePubkey(zklinkSigner *ZkLinkSigner, tx *ChangePubKey, ethAuthData ChangePubKeyAuthData) (*ChangePubKey, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[SignError](FfiConverterSignError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_zklink_sdk_fn_func_create_signed_change_pubkey(FfiConverterZkLinkSignerINSTANCE.Lower(zklinkSigner), FfiConverterChangePubKeyINSTANCE.Lower(tx), FfiConverterChangePubKeyAuthDataINSTANCE.Lower(ethAuthData), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ChangePubKey
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterChangePubKeyINSTANCE.Lift(_uniffiRV), nil
	}
}

func EthSignatureOfChangePubkey(tx *ChangePubKey, ethSigner *EthSigner) (PackedEthSignature, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[SignError](FfiConverterSignError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_func_eth_signature_of_change_pubkey(FfiConverterChangePubKeyINSTANCE.Lower(tx), FfiConverterEthSignerINSTANCE.Lower(ethSigner), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue PackedEthSignature
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTypePackedEthSignatureINSTANCE.Lift(_uniffiRV), nil
	}
}

func GetPublicKeyHash(publicKey PackedPublicKey) PubKeyHash {
	return FfiConverterTypePubKeyHashINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_func_get_public_key_hash(FfiConverterTypePackedPublicKeyINSTANCE.Lower(publicKey), _uniffiStatus),
		}
	}))
}

func IsFeeAmountPackable(fee BigUint) bool {
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_func_is_fee_amount_packable(FfiConverterTypeBigUintINSTANCE.Lower(fee).ExternalBuffer(), _uniffiStatus)
	}))
}

func IsTokenAmountPackable(amount BigUint) bool {
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_func_is_token_amount_packable(FfiConverterTypeBigUintINSTANCE.Lower(amount).ExternalBuffer(), _uniffiStatus)
	}))
}

func VerifyMusig(signature ZkLinkSignature, msg []uint8) bool {
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_zklink_sdk_fn_func_verify_musig(FfiConverterZkLinkSignatureINSTANCE.Lower(signature), FfiConverterSequenceUint8INSTANCE.Lower(msg), _uniffiStatus)
	}))
}

func ZklinkMainNetUrl() string {
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_func_zklink_main_net_url(_uniffiStatus),
		}
	}))
}

func ZklinkTestNetUrl() string {
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_zklink_sdk_fn_func_zklink_test_net_url(_uniffiStatus),
		}
	}))
}
