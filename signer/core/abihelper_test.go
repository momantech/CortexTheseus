// Copyright 2018 The go-ethereum Authors
// This file is part of CortexFoundation.
//
// CortexFoundation is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// CortexFoundation is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with CortexFoundation. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/CortexFoundation/CortexTheseus/accounts/abi"
	"github.com/CortexFoundation/CortexTheseus/common"
)

func verify(t *testing.T, jsondata, calldata string, exp []any) {

	abispec, err := abi.JSON(strings.NewReader(jsondata))
	if err != nil {
		t.Fatal(err)
	}
	cd := common.Hex2Bytes(calldata)
	sigdata, argdata := cd[:4], cd[4:]
	method, err := abispec.MethodById(sigdata)

	if err != nil {
		t.Fatal(err)
	}

	data, err := method.Inputs.UnpackValues(argdata)

	if len(data) != len(exp) {
		t.Fatalf("Mismatched length, expected %d, got %d", len(exp), len(data))
	}
	for i, elem := range data {
		if !reflect.DeepEqual(elem, exp[i]) {
			t.Fatalf("Unpack error, arg %d, got %v, want %v", i, elem, exp[i])
		}
	}
}
func TestNewUnpacker(t *testing.T) {
	type unpackTest struct {
		jsondata string
		calldata string
		exp      []any
	}
	testcases := []unpackTest{
		{ // https://solidity.readthedocs.io/en/develop/abi-spec.html#use-of-dynamic-types
			`[{"type":"function","name":"f", "inputs":[{"type":"uint256"},{"type":"uint32[]"},{"type":"bytes10"},{"type":"bytes"}]}]`,
			// 0x123, [0x456, 0x789], "1234567890", "Hello, world!"
			"8be65246" + "00000000000000000000000000000000000000000000000000000000000001230000000000000000000000000000000000000000000000000000000000000080313233343536373839300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000004560000000000000000000000000000000000000000000000000000000000000789000000000000000000000000000000000000000000000000000000000000000d48656c6c6f2c20776f726c642100000000000000000000000000000000000000",
			[]any{
				big.NewInt(0x123),
				[]uint32{0x456, 0x789},
				[10]byte{49, 50, 51, 52, 53, 54, 55, 56, 57, 48},
				common.Hex2Bytes("48656c6c6f2c20776f726c6421"),
			},
		}, { // https://github.com/cortex/wiki/wiki/Cortex-Contract-ABI#examples
			`[{"type":"function","name":"sam","inputs":[{"type":"bytes"},{"type":"bool"},{"type":"uint256[]"}]}]`,
			//  "dave", true and [1,2,3]
			"a5643bf20000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000000464617665000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003",
			[]any{
				[]byte{0x64, 0x61, 0x76, 0x65},
				true,
				[]*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)},
			},
		}, {
			`[{"type":"function","name":"send","inputs":[{"type":"uint256"}]}]`,
			"a52c101e0000000000000000000000000000000000000000000000000000000000000012",
			[]any{big.NewInt(0x12)},
		}, {
			`[{"type":"function","name":"compareAndApprove","inputs":[{"type":"address"},{"type":"uint256"},{"type":"uint256"}]}]`,
			"751e107900000000000000000000000000000133700000deadbeef00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001",
			[]any{
				common.HexToAddress("0x00000133700000deadbeef000000000000000000"),
				new(big.Int).SetBytes([]byte{0x00}),
				big.NewInt(0x1),
			},
		},
	}
	for _, c := range testcases {
		verify(t, c.jsondata, c.calldata, c.exp)
	}

}

/*
func TestReflect(t *testing.T) {
	a := big.NewInt(0)
	b := new(big.Int).SetBytes([]byte{0x00})
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("Nope, %v != %v", a, b)
	}
}
*/

func TestCalldataDecoding(t *testing.T) {

	// send(uint256)                              : a52c101e
	// compareAndApprove(address,uint256,uint256) : 751e1079
	// issue(address[],uint256)                   : 42958b54
	jsondata := `
[
	{"type":"function","name":"send","inputs":[{"name":"a","type":"uint256"}]},
	{"type":"function","name":"compareAndApprove","inputs":[{"name":"a","type":"address"},{"name":"a","type":"uint256"},{"name":"a","type":"uint256"}]},
	{"type":"function","name":"issue","inputs":[{"name":"a","type":"address[]"},{"name":"a","type":"uint256"}]},
	{"type":"function","name":"sam","inputs":[{"name":"a","type":"bytes"},{"name":"a","type":"bool"},{"name":"a","type":"uint256[]"}]}
]`
	//Expected failures
	for _, hexdata := range []string{
		"a52c101e00000000000000000000000000000000000000000000000000000000000000120000000000000000000000000000000000000000000000000000000000000042",
		"a52c101e000000000000000000000000000000000000000000000000000000000000001200",
		"a52c101e00000000000000000000000000000000000000000000000000000000000000",
		"a52c101e",
		"a52c10",
		"",
		// Too short
		"751e10790000000000000000000000000000000000000000000000000000000000000012",
		"751e1079FFffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		//Not valid multiple of 32
		"deadbeef00000000000000000000000000000000000000000000000000000000000000",
		//Too short 'issue'
		"42958b5400000000000000000000000000000000000000000000000000000000000000120000000000000000000000000000000000000000000000000000000000000042",
		// Too short compareAndApprove
		"a52c101e00ff0000000000000000000000000000000000000000000000000000000000120000000000000000000000000000000000000000000000000000000000000042",
		// From https://github.com/cortex/wiki/wiki/Cortex-Contract-ABI
		// contains a bool with illegal values
		"a5643bf20000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000001100000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000000464617665000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003",
	} {
		_, err := parseCallData(common.Hex2Bytes(hexdata), jsondata)
		if err == nil {
			t.Errorf("Expected decoding to fail: %s", hexdata)
		}
	}

	//Expected success
	for _, hexdata := range []string{
		// From https://github.com/cortex/wiki/wiki/Cortex-Contract-ABI
		"a5643bf20000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000000464617665000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003",
		"a52c101e0000000000000000000000000000000000000000000000000000000000000012",
		"a52c101eFFffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		"751e1079000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		"42958b54" +
			// start of dynamic type
			"0000000000000000000000000000000000000000000000000000000000000040" +
			//uint256
			"0000000000000000000000000000000000000000000000000000000000000001" +
			// length of  array
			"0000000000000000000000000000000000000000000000000000000000000002" +
			// array values
			"000000000000000000000000000000000000000000000000000000000000dead" +
			"000000000000000000000000000000000000000000000000000000000000beef",
	} {
		_, err := parseCallData(common.Hex2Bytes(hexdata), jsondata)
		if err != nil {
			t.Errorf("Unexpected failure on input %s:\n %v (%d bytes) ", hexdata, err, len(common.Hex2Bytes(hexdata)))
		}
	}
}

//func TestSelectorUnmarshalling(t *testing.T) {
//	var (
//		db        *AbiDb
//		err       error
//		abistring []byte
//		abistruct abi.ABI
//	)
//
//	db, err = NewAbiDBFromFile("../../cmd/clef/4byte.json")
//	if err != nil {
//		t.Fatal(err)
//	}
//	fmt.Printf("DB size %v\n", db.Size())
//	for id, selector := range db.db {
//
//		abistring, err = MethodSelectorToAbi(selector)
//		if err != nil {
//			t.Error(err)
//			return
//		}
//		abistruct, err = abi.JSON(strings.NewReader(string(abistring)))
//		if err != nil {
//			t.Error(err)
//			return
//		}
//		m, err := abistruct.MethodById(common.Hex2Bytes(id[2:]))
//		if err != nil {
//			t.Error(err)
//			return
//		}
//		if m.Sig() != selector {
//			t.Errorf("Expected equality: %v != %v", m.Sig(), selector)
//		}
//	}
//
//}
//
//func TestCustomABI(t *testing.T) {
//	d, err := os.MkdirTemp("", "signer-4byte-test")
//	if err != nil {
//		t.Fatal(err)
//	}
//	filename := fmt.Sprintf("%s/4byte_custom.json", d)
//	abidb, err := NewAbiDBFromFiles("../../cmd/clef/4byte.json", filename)
//	if err != nil {
//		t.Fatal(err)
//	}
//	// Now we'll remove all existing signatures
//	abidb.db = make(map[string]string)
//	calldata := common.Hex2Bytes("a52c101edeadbeef")
//	_, err = abidb.LookupMethodSelector(calldata)
//	if err == nil {
//		t.Fatalf("Should not find a match on empty db")
//	}
//	if err = abidb.AddSignature("send(uint256)", calldata); err != nil {
//		t.Fatalf("Failed to save file: %v", err)
//	}
//	_, err = abidb.LookupMethodSelector(calldata)
//	if err != nil {
//		t.Fatalf("Should find a match for abi signature, got: %v", err)
//	}
//	//Check that it wrote to file
//	abidb2, err := NewAbiDBFromFile(filename)
//	if err != nil {
//		t.Fatalf("Failed to create new abidb: %v", err)
//	}
//	_, err = abidb2.LookupMethodSelector(calldata)
//	if err != nil {
//		t.Fatalf("Save failed: should find a match for abi signature after loading from disk")
//	}
//}
