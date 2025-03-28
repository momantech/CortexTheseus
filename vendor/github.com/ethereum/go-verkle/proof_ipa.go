// This is free and unencumbered software released into the public domain.
//
// Anyone is free to copy, modify, publish, use, compile, sell, or
// distribute this software, either in source code form or as a compiled
// binary, for any purpose, commercial or non-commercial, and by any
// means.
//
// In jurisdictions that recognize copyright laws, the author or authors
// of this software dedicate any and all copyright interest in the
// software to the public domain. We make this dedication for the benefit
// of the public at large and to the detriment of our heirs and
// successors. We intend this dedication to be an overt act of
// relinquishment in perpetuity of all present and future rights to this
// software under copyright law.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
// IN NO EVENT SHALL THE AUTHORS BE LIABLE FOR ANY CLAIM, DAMAGES OR
// OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
// OTHER DEALINGS IN THE SOFTWARE.
//
// For more information, please refer to <https://unlicense.org>

package verkle

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"unsafe"

	ipa "github.com/crate-crypto/go-ipa"
	"github.com/crate-crypto/go-ipa/common"
)

const IPA_PROOF_DEPTH = 8

type IPAProof struct {
	CL              [IPA_PROOF_DEPTH][32]byte `json:"cl"`
	CR              [IPA_PROOF_DEPTH][32]byte `json:"cr"`
	FinalEvaluation [32]byte                  `json:"finalEvaluation"`
}

type VerkleProof struct {
	OtherStems            [][StemSize]byte `json:"otherStems"`
	DepthExtensionPresent []byte           `json:"depthExtensionPresent"`
	CommitmentsByPath     [][32]byte       `json:"commitmentsByPath"`
	D                     [32]byte         `json:"d"`
	IPAProof              *IPAProof        `json:"ipa_proof"`
}

func (vp *VerkleProof) Copy() *VerkleProof {
	if vp == nil {
		return nil
	}
	ret := &VerkleProof{
		OtherStems:            make([][StemSize]byte, len(vp.OtherStems)),
		DepthExtensionPresent: make([]byte, len(vp.DepthExtensionPresent)),
		CommitmentsByPath:     make([][32]byte, len(vp.CommitmentsByPath)),
		IPAProof:              &IPAProof{},
	}

	copy(ret.OtherStems, vp.OtherStems)
	copy(ret.DepthExtensionPresent, vp.DepthExtensionPresent)
	copy(ret.CommitmentsByPath, vp.CommitmentsByPath)

	ret.D = vp.D

	if vp.IPAProof != nil {
		ret.IPAProof = vp.IPAProof
	}

	return ret
}

func (vp *VerkleProof) Equal(other *VerkleProof) error {
	if len(vp.OtherStems) != len(other.OtherStems) {
		return fmt.Errorf("different number of other stems: %d != %d", len(vp.OtherStems), len(other.OtherStems))
	}
	for i := range vp.OtherStems {
		if vp.OtherStems[i] != other.OtherStems[i] {
			return fmt.Errorf("different other stem: %x != %x", vp.OtherStems[i], other.OtherStems[i])
		}
	}
	if len(vp.DepthExtensionPresent) != len(other.DepthExtensionPresent) {
		return fmt.Errorf("different number of depth extension present: %d != %d", len(vp.DepthExtensionPresent), len(other.DepthExtensionPresent))
	}
	if !bytes.Equal(vp.DepthExtensionPresent, other.DepthExtensionPresent) {
		return fmt.Errorf("different depth extension present: %x != %x", vp.DepthExtensionPresent, other.DepthExtensionPresent)
	}
	if len(vp.CommitmentsByPath) != len(other.CommitmentsByPath) {
		return fmt.Errorf("different number of commitments by path: %d != %d", len(vp.CommitmentsByPath), len(other.CommitmentsByPath))
	}
	for i := range vp.CommitmentsByPath {
		if vp.CommitmentsByPath[i] != other.CommitmentsByPath[i] {
			return fmt.Errorf("different commitment by path: %x != %x", vp.CommitmentsByPath[i], other.CommitmentsByPath[i])
		}
	}
	if vp.D != other.D {
		return fmt.Errorf("different D: %x != %x", vp.D, other.D)
	}
	return nil
}

type Proof struct {
	Multipoint *ipa.MultiProof // multipoint argument
	ExtStatus  []byte          // the extension status of each stem
	Cs         []*Point        // commitments, sorted by their path in the tree
	PoaStems   []Stem          // stems proving another stem is absent
	Keys       [][]byte
	PreValues  [][]byte
	PostValues [][]byte
}

type SuffixStateDiff struct {
	Suffix       byte      `json:"suffix"`
	CurrentValue *[32]byte `json:"currentValue"`
	NewValue     *[32]byte `json:"newValue"`
}

type SuffixStateDiffs []SuffixStateDiff

type StemStateDiff struct {
	Stem        [StemSize]byte   `json:"stem"`
	SuffixDiffs SuffixStateDiffs `json:"suffixDiffs"`
}

type StateDiff []StemStateDiff

func (sd StateDiff) Copy() StateDiff {
	ret := make(StateDiff, len(sd))
	for i := range sd {
		copy(ret[i].Stem[:], sd[i].Stem[:])
		ret[i].SuffixDiffs = make([]SuffixStateDiff, len(sd[i].SuffixDiffs))
		for j := range sd[i].SuffixDiffs {
			ret[i].SuffixDiffs[j].Suffix = sd[i].SuffixDiffs[j].Suffix
			if sd[i].SuffixDiffs[j].CurrentValue != nil {
				ret[i].SuffixDiffs[j].CurrentValue = &[32]byte{}
				copy((*ret[i].SuffixDiffs[j].CurrentValue)[:], (*sd[i].SuffixDiffs[j].CurrentValue)[:])
			}
			if sd[i].SuffixDiffs[j].NewValue != nil {
				ret[i].SuffixDiffs[j].NewValue = &[32]byte{}
				copy((*ret[i].SuffixDiffs[j].NewValue)[:], (*sd[i].SuffixDiffs[j].NewValue)[:])
			}
		}
	}
	return ret
}

func (sd StateDiff) Equal(other StateDiff) error {
	if len(sd) != len(other) {
		return fmt.Errorf("different number of stem state diffs: %d != %d", len(sd), len(other))
	}
	for i := range sd {
		if sd[i].Stem != other[i].Stem {
			return fmt.Errorf("different stem: %x != %x", sd[i].Stem, other[i].Stem)
		}
		if len(sd[i].SuffixDiffs) != len(other[i].SuffixDiffs) {
			return fmt.Errorf("different number of suffix state diffs: %d != %d", len(sd[i].SuffixDiffs), len(other[i].SuffixDiffs))
		}
		for j := range sd[i].SuffixDiffs {
			if sd[i].SuffixDiffs[j].Suffix != other[i].SuffixDiffs[j].Suffix {
				return fmt.Errorf("different suffix: %x != %x", sd[i].SuffixDiffs[j].Suffix, other[i].SuffixDiffs[j].Suffix)
			}
			if sd[i].SuffixDiffs[j].CurrentValue != nil && other[i].SuffixDiffs[j].CurrentValue != nil {
				if *sd[i].SuffixDiffs[j].CurrentValue != *other[i].SuffixDiffs[j].CurrentValue {
					return fmt.Errorf("different current value: %x != %x", *sd[i].SuffixDiffs[j].CurrentValue, *other[i].SuffixDiffs[j].CurrentValue)
				}
			} else if sd[i].SuffixDiffs[j].CurrentValue != nil || other[i].SuffixDiffs[j].CurrentValue != nil {
				return fmt.Errorf("different current value: %x != %x", sd[i].SuffixDiffs[j].CurrentValue, other[i].SuffixDiffs[j].CurrentValue)
			}
			if sd[i].SuffixDiffs[j].NewValue != nil && other[i].SuffixDiffs[j].NewValue != nil {
				if *sd[i].SuffixDiffs[j].NewValue != *other[i].SuffixDiffs[j].NewValue {
					return fmt.Errorf("different new value: %x != %x", *sd[i].SuffixDiffs[j].NewValue, *other[i].SuffixDiffs[j].NewValue)
				}
			} else if sd[i].SuffixDiffs[j].NewValue != nil || other[i].SuffixDiffs[j].NewValue != nil {
				return fmt.Errorf("different new value: %x != %x", sd[i].SuffixDiffs[j].NewValue, other[i].SuffixDiffs[j].NewValue)
			}
		}
	}
	return nil
}

func GetCommitmentsForMultiproof(root VerkleNode, keys [][]byte, resolver NodeResolverFn) (*ProofElements, []byte, []Stem, error) {
	sort.Sort(keylist(keys))
	return root.GetProofItems(keylist(keys), resolver)
}

// getProofElementsFromTree factors the logic that is used both in the proving and verification methods. It takes a pre-state
// tree and an optional post-state tree, extracts the proof data from them and returns all the items required to build/verify
// a proof.
func getProofElementsFromTree(preroot, postroot VerkleNode, keys [][]byte, resolver NodeResolverFn) (*ProofElements, []byte, []Stem, [][]byte, error) {
	// go-ipa won't accept no key as an input, catch this corner case
	// and return an empty result.
	if len(keys) == 0 {
		return nil, nil, nil, nil, errors.New("no key provided for proof")
	}

	pe, es, poas, err := GetCommitmentsForMultiproof(preroot, keys, resolver)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error getting pre-state proof data: %w", err)
	}

	// if a post-state tree is present, merge its proof elements with
	// those of the pre-state tree, so that they can be proved together.
	postvals := make([][]byte, len(keys))
	if postroot != nil {
		// keys were sorted already in the above GetcommitmentsForMultiproof.
		// Set the post values, if they are untouched, leave them `nil`
		for i := range keys {
			val, err := postroot.Get(keys[i], resolver)
			if err != nil {
				return nil, nil, nil, nil, fmt.Errorf("error getting post-state value for key %x: %w", keys[i], err)
			}
			if !bytes.Equal(pe.Vals[i], val) {
				postvals[i] = val
			}
		}
	}

	// [0:3]: proof elements of the pre-state trie for serialization,
	// 3: values to be inserted in the post-state trie for serialization
	return pe, es, poas, postvals, nil
}

func MakeVerkleMultiProof(preroot, postroot VerkleNode, keys [][]byte, resolver NodeResolverFn) (*Proof, []*Point, []byte, []*Fr, error) {
	pe, es, poas, postvals, err := getProofElementsFromTree(preroot, postroot, keys, resolver)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("get commitments for multiproof: %s", err)
	}

	cfg := GetConfig()
	tr := common.NewTranscript("vt")
	mpArg, err := ipa.CreateMultiProof(tr, cfg.conf, pe.Cis, pe.Fis, pe.Zis)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("creating multiproof: %w", err)
	}

	// It's wheel-reinvention time again 🎉: reimplement a basic
	// feature that should be part of the stdlib.
	// "But golang is a high-productivity language!!!" 🤪
	// len()-1, because the root is already present in the
	// parent block, so we don't keep it in the proof.
	paths := make([]string, 0, len(pe.ByPath)-1)
	for path := range pe.ByPath {
		if len(path) > 0 {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	cis := make([]*Point, len(pe.ByPath)-1)
	for i, path := range paths {
		cis[i] = pe.ByPath[path]
	}

	proof := &Proof{
		Multipoint: mpArg,
		Cs:         cis,
		ExtStatus:  es,
		PoaStems:   poas,
		Keys:       keys,
		PreValues:  pe.Vals,
		PostValues: postvals,
	}
	return proof, pe.Cis, pe.Zis, pe.Yis, nil
}

// verifyVerkleProofWithPreState takes a proof and a trusted tree root and verifies that the proof is valid.
func verifyVerkleProofWithPreState(proof *Proof, preroot VerkleNode) error {
	pe, _, _, _, err := getProofElementsFromTree(preroot, nil, proof.Keys, nil)
	if err != nil {
		return fmt.Errorf("error getting proof elements: %w", err)
	}

	if ok, err := verifyVerkleProof(proof, pe.Cis, pe.Zis, pe.Yis, GetConfig()); !ok || err != nil {
		return fmt.Errorf("error verifying proof: verifies=%v, error=%w", ok, err)
	}

	return nil
}

func verifyVerkleProof(proof *Proof, Cs []*Point, indices []uint8, ys []*Fr, tc *Config) (bool, error) {
	tr := common.NewTranscript("vt")
	return ipa.CheckMultiProof(tr, tc.conf, proof.Multipoint, Cs, ys, indices)
}

// SerializeProof serializes the proof in the rust-verkle format:
// * len(Proof of absence stem) || Proof of absence stems
// * len(depths) || serialize(depth || ext statusi)
// * len(commitments) || serialize(commitment)
// * Multipoint proof
// it also returns the serialized keys and values
func SerializeProof(proof *Proof) (*VerkleProof, StateDiff, error) {
	otherstems := make([][StemSize]byte, len(proof.PoaStems))
	for i, stem := range proof.PoaStems {
		copy(otherstems[i][:], stem)
	}

	cbp := make([][32]byte, len(proof.Cs))
	for i, C := range proof.Cs {
		serialized := C.Bytes()
		copy(cbp[i][:], serialized[:])
	}

	var cls, crs [IPA_PROOF_DEPTH][32]byte
	for i := 0; i < IPA_PROOF_DEPTH; i++ {

		l := proof.Multipoint.IPA.L[i].Bytes()
		copy(cls[i][:], l[:])
		r := proof.Multipoint.IPA.R[i].Bytes()
		copy(crs[i][:], r[:])
	}

	var stemdiff *StemStateDiff
	var statediff StateDiff
	for i, key := range proof.Keys {
		stem := KeyToStem(key)
		if stemdiff == nil || !bytes.Equal(stemdiff.Stem[:], stem) {
			statediff = append(statediff, StemStateDiff{})
			stemdiff = &statediff[len(statediff)-1]
			copy(stemdiff.Stem[:], stem)
		}
		stemdiff.SuffixDiffs = append(stemdiff.SuffixDiffs, SuffixStateDiff{Suffix: key[StemSize]})
		newsd := &stemdiff.SuffixDiffs[len(stemdiff.SuffixDiffs)-1]

		var valueLen = len(proof.PreValues[i])
		switch valueLen {
		case 0:
			// null value
		case 32:
			newsd.CurrentValue = (*[32]byte)(proof.PreValues[i])
		default:
			var aligned [32]byte
			copy(aligned[:valueLen], proof.PreValues[i])
			newsd.CurrentValue = (*[32]byte)(unsafe.Pointer(&aligned[0]))
		}

		valueLen = len(proof.PostValues[i])
		switch valueLen {
		case 0:
			// null value
		case 32:
			newsd.NewValue = (*[32]byte)(proof.PostValues[i])
		default:
			// TODO remove usage of unsafe
			var aligned [32]byte
			copy(aligned[:valueLen], proof.PostValues[i])
			newsd.NewValue = (*[32]byte)(unsafe.Pointer(&aligned[0]))
		}
	}

	return &VerkleProof{
		OtherStems:            otherstems,
		DepthExtensionPresent: proof.ExtStatus,
		CommitmentsByPath:     cbp,
		D:                     proof.Multipoint.D.Bytes(),
		IPAProof: &IPAProof{
			CL:              cls,
			CR:              crs,
			FinalEvaluation: proof.Multipoint.IPA.A_scalar.Bytes(),
		},
	}, statediff, nil
}

// DeserializeProof deserializes the proof found in blocks, into a format that
// can be used to rebuild a stateless version of the tree.
func DeserializeProof(vp *VerkleProof, statediff StateDiff) (*Proof, error) {
	var (
		poaStems              []Stem
		keys                  [][]byte
		prevalues, postvalues [][]byte
		extStatus             []byte
		commitments           []*Point
		multipoint            ipa.MultiProof
	)

	poaStems = make([]Stem, len(vp.OtherStems))
	for i, poaStem := range vp.OtherStems {
		poaStems[i] = make([]byte, len(poaStem))
		copy(poaStems[i], poaStem[:])
	}

	extStatus = vp.DepthExtensionPresent

	commitments = make([]*Point, len(vp.CommitmentsByPath))
	for i, commitmentBytes := range vp.CommitmentsByPath {
		var commitment Point
		if err := commitment.SetBytes(commitmentBytes[:]); err != nil {
			return nil, err
		}
		commitments[i] = &commitment
	}

	if err := multipoint.D.SetBytes(vp.D[:]); err != nil {
		return nil, fmt.Errorf("setting D: %w", err)
	}
	multipoint.IPA.A_scalar.SetBytes(vp.IPAProof.FinalEvaluation[:])
	multipoint.IPA.L = make([]Point, IPA_PROOF_DEPTH)
	for i, b := range vp.IPAProof.CL {
		if err := multipoint.IPA.L[i].SetBytes(b[:]); err != nil {
			return nil, fmt.Errorf("setting L[%d]: %w", i, err)
		}
	}
	multipoint.IPA.R = make([]Point, IPA_PROOF_DEPTH)
	for i, b := range vp.IPAProof.CR {
		if err := multipoint.IPA.R[i].SetBytes(b[:]); err != nil {
			return nil, fmt.Errorf("setting R[%d]: %w", i, err)
		}
	}

	// turn statediff into keys and values
	for _, stemdiff := range statediff {
		for _, suffixdiff := range stemdiff.SuffixDiffs {
			var k [32]byte
			copy(k[:StemSize], stemdiff.Stem[:])
			k[StemSize] = suffixdiff.Suffix
			keys = append(keys, k[:])
			if suffixdiff.CurrentValue != nil {
				prevalues = append(prevalues, suffixdiff.CurrentValue[:])
			} else {
				prevalues = append(prevalues, nil)
			}

			if suffixdiff.NewValue != nil {
				postvalues = append(postvalues, suffixdiff.NewValue[:])
			} else {
				postvalues = append(postvalues, nil)
			}
		}
	}

	proof := Proof{
		&multipoint,
		extStatus,
		commitments,
		poaStems,
		keys,
		prevalues,
		postvalues,
	}
	return &proof, nil
}

type stemInfo struct {
	depth          byte
	stemType       byte
	has_c1, has_c2 bool
	values         map[byte][]byte
	stem           []byte
}

// PreStateTreeFromProof builds a stateless prestate tree from the proof.
func PreStateTreeFromProof(proof *Proof, rootC *Point) (VerkleNode, error) { // skipcq: GO-R1005
	if len(proof.Keys) != len(proof.PreValues) {
		return nil, fmt.Errorf("incompatible number of keys and pre-values: %d != %d", len(proof.Keys), len(proof.PreValues))
	}
	if len(proof.Keys) != len(proof.PostValues) {
		return nil, fmt.Errorf("incompatible number of keys and post-values: %d != %d", len(proof.Keys), len(proof.PostValues))
	}
	stems := make([][]byte, 0, len(proof.Keys))
	for _, k := range proof.Keys {
		stem := KeyToStem(k)
		if len(stems) == 0 || !bytes.Equal(stems[len(stems)-1], stem) {
			stems = append(stems, stem)
		}
	}
	if len(stems) != len(proof.ExtStatus) {
		return nil, fmt.Errorf("invalid number of stems and extension statuses: %d != %d", len(stems), len(proof.ExtStatus))
	}
	var (
		info  = map[string]stemInfo{}
		paths [][]byte
		err   error
		poas  = proof.PoaStems
	)

	// The proof of absence stems must be sorted. If that isn't the case, the proof is invalid.
	if !sort.IsSorted(bytesSlice(proof.PoaStems)) {
		return nil, fmt.Errorf("proof of absence stems are not sorted")
	}

	// We build a cache of paths that have a presence extension status.
	pathsWithExtPresent := map[string]struct{}{}
	i := 0
	for _, es := range proof.ExtStatus {
		if es&3 == extStatusPresent {
			pathsWithExtPresent[string(stems[i][:es>>3])] = struct{}{}
		}
		i++
	}

	// assign one or more stem to each stem info
	for i, es := range proof.ExtStatus {
		si := stemInfo{
			depth:    es >> 3,
			stemType: es & 3,
		}
		path := stems[i][:si.depth]
		switch si.stemType {
		case extStatusAbsentEmpty:
			// All keys that are part of a proof of absence, must contain empty
			// prestate values. If that isn't the case, the proof is invalid.
			for j := range proof.Keys { // TODO: DoS risk, use map or binary search.
				if bytes.HasPrefix(proof.Keys[j], stems[i]) && proof.PreValues[j] != nil {
					return nil, fmt.Errorf("proof of absence (empty) stem %x has a value", si.stem)
				}
			}
		case extStatusAbsentOther:
			// All keys that are part of a proof of absence, must contain empty
			// prestate values. If that isn't the case, the proof is invalid.
			for j := range proof.Keys { // TODO: DoS risk, use map or binary search.
				if bytes.HasPrefix(proof.Keys[j], stems[i]) && proof.PreValues[j] != nil {
					return nil, fmt.Errorf("proof of absence (other) stem %x has a value", si.stem)
				}
			}

			// For this absent path, we must first check if this path contains a proof of presence.
			// If that is the case, we don't have to do anything since the corresponding leaf will be
			// constructed by that extension status (already processed or to be processed).
			// In other case, we should get the stem from the list of proof of absence stems.
			if _, ok := pathsWithExtPresent[string(path)]; ok {
				continue
			}

			// Note that this path doesn't have proof of presence (previous if check above), but
			// it can have multiple proof of absence. If a previous proof of absence had already
			// created the stemInfo for this path, we don't have to do anything.
			if _, ok := info[string(path)]; ok {
				continue
			}

			si.stem = poas[0]
			poas = poas[1:]
		case extStatusPresent:
			si.values = map[byte][]byte{}
			si.stem = stems[i]
			for j, k := range proof.Keys { // TODO: DoS risk, use map or binary search.
				if bytes.Equal(KeyToStem(k), si.stem) {
					si.values[k[StemSize]] = proof.PreValues[j]
					si.has_c1 = si.has_c1 || (k[StemSize] < 128)
					si.has_c2 = si.has_c2 || (k[StemSize] >= 128)
				}
			}
		default:
			return nil, fmt.Errorf("invalid extension status: %d", si.stemType)
		}
		info[string(path)] = si
		paths = append(paths, path)
	}

	if len(poas) != 0 {
		return nil, fmt.Errorf("not all proof of absence stems were used: %d", len(poas))
	}

	root := NewStatelessInternal(0, rootC).(*InternalNode)
	comms := proof.Cs
	for _, p := range paths {
		// NOTE: the reconstructed tree won't tell the
		// difference between leaves missing from view
		// and absent leaves. This is enough for verification
		// but not for block validation.
		values := make([][]byte, NodeWidth)
		for i, k := range proof.Keys {
			if len(proof.PreValues[i]) == 0 {
				// Skip the nil keys, they are here to prove
				// an absence.
				continue
			}

			if bytes.Equal(KeyToStem(k), info[string(p)].stem) {
				values[k[StemSize]] = proof.PreValues[i]
			}
		}
		comms, err = root.CreatePath(p, info[string(p)], comms, values)
		if err != nil {
			return nil, err
		}
	}

	return root, nil
}

// PostStateTreeFromProof uses the pre-state trie and the list of updated values
// to produce the stateless post-state trie.
func PostStateTreeFromStateDiff(preroot VerkleNode, statediff StateDiff) (VerkleNode, error) {
	postroot := preroot.Copy()

	for _, stemstatediff := range statediff {
		var (
			values     = make([][]byte, NodeWidth)
			overwrites bool
		)

		for _, suffixdiff := range stemstatediff.SuffixDiffs {
			if /* len(suffixdiff.NewValue) > 0 - this only works for a slice */ suffixdiff.NewValue != nil {
				// if this value is non-nil, it means InsertValuesAtStem should be
				// called, otherwise, skip updating the tree.
				overwrites = true
				values[suffixdiff.Suffix] = suffixdiff.NewValue[:]
			}
		}

		if overwrites {
			var stem [StemSize]byte
			copy(stem[:StemSize], stemstatediff.Stem[:])
			if err := postroot.(*InternalNode).InsertValuesAtStem(stem[:], values, nil); err != nil {
				return nil, fmt.Errorf("error overwriting value in post state: %w", err)
			}
		}
	}
	postroot.Commit()

	return postroot, nil
}

type bytesSlice []Stem

func (x bytesSlice) Len() int           { return len(x) }
func (x bytesSlice) Less(i, j int) bool { return bytes.Compare(x[i], x[j]) < 0 }
func (x bytesSlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

// Verify is the API function that verifies a verkle proofs as found in a block/execution payload.
func Verify(vp *VerkleProof, preStateRoot []byte, postStateRoot []byte, statediff StateDiff) error {

	proof, err := DeserializeProof(vp, statediff)
	if err != nil {
		return fmt.Errorf("verkle proof deserialization error: %w", err)
	}

	rootC := new(Point)
	if err := rootC.SetBytes(preStateRoot); err != nil {
		return fmt.Errorf("error setting prestate root: %w", err)
	}
	pretree, err := PreStateTreeFromProof(proof, rootC)
	if err != nil {
		return fmt.Errorf("error rebuilding the pre-tree from proof: %w", err)
	}
	// TODO this should not be necessary, remove it
	// after the new proof generation code has stabilized.
	for _, stemdiff := range statediff {
		for _, suffixdiff := range stemdiff.SuffixDiffs {
			var key [32]byte
			copy(key[:31], stemdiff.Stem[:])
			key[31] = suffixdiff.Suffix

			val, err := pretree.Get(key[:], nil)
			if err != nil {
				return fmt.Errorf("could not find key %x in tree rebuilt from proof: %w", key, err)
			}
			if len(val) > 0 {
				if !bytes.Equal(val, suffixdiff.CurrentValue[:]) {
					return fmt.Errorf("could not find correct value at %x in tree rebuilt from proof: %x != %x", key, val, *suffixdiff.CurrentValue)
				}
			} else {
				if suffixdiff.CurrentValue != nil && len(suffixdiff.CurrentValue) != 0 {
					return fmt.Errorf("could not find correct value at %x in tree rebuilt from proof: %x != %x", key, val, *suffixdiff.CurrentValue)
				}
			}
		}
	}

	// TODO: this is necessary to verify that the post-values are the correct ones.
	// But all this can be avoided with a even faster way. The EVM block execution can
	// keep track of the written keys, and compare that list with this post-values list.
	// This can avoid regenerating the post-tree which is somewhat expensive.
	posttree, err := PostStateTreeFromStateDiff(pretree, statediff)
	if err != nil {
		return fmt.Errorf("error rebuilding the post-tree from proof: %w", err)
	}
	regeneratedPostTreeRoot := posttree.Commitment().Bytes()
	if !bytes.Equal(regeneratedPostTreeRoot[:], postStateRoot) {
		return fmt.Errorf("post tree root mismatch: %x != %x", regeneratedPostTreeRoot, postStateRoot)
	}

	return verifyVerkleProofWithPreState(proof, pretree)
}
