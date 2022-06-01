// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package signing

import (
	"fmt"
	"reflect"

	"github.com/open-component-model/ocm/pkg/common"
	"github.com/open-component-model/ocm/pkg/common/accessio"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	metav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	"github.com/open-component-model/ocm/pkg/errors"
)

func ToDigestSpec(v interface{}) *metav1.DigestSpec {
	if v == nil {
		return nil
	}
	return v.(*metav1.DigestSpec)
}

func Apply(printer common.Printer, state *common.WalkingState, cv ocm.ComponentVersionAccess, opts *Options) (*metav1.DigestSpec, error) {
	if printer == nil {
		printer = common.NewPrinter(nil)
	}
	if state == nil {
		s := common.NewWalkingState()
		state = &s
	}
	return apply(printer, *state, cv, opts)
}

func apply(printer common.Printer, state common.WalkingState, cv ocm.ComponentVersionAccess, opts *Options) (*metav1.DigestSpec, error) {
	nv := common.VersionedElementKey(cv)
	if ok, err := state.Add(ocm.KIND_COMPONENTVERSION, nv); !ok {
		return ToDigestSpec(state.Closure[nv]), err
	}

	cd := cv.GetDescriptor().Copy()
	printer.Printf("applying to version %q...\n", nv)
	for i, reference := range cd.ComponentReferences {
		var calculatedDigest *metav1.DigestSpec
		if reference.Digest == nil && !opts.DoUpdate() {
			return nil, errors.Newf(refMsg(reference, state, "no digest for component reference"))
		}
		if reference.Digest == nil || opts.Verify {
			nested, err := opts.Resolver.LookupComponentVersion(reference.GetComponentName(), reference.GetVersion())
			if err != nil {
				return nil, errors.Wrapf(err, refMsg(reference, state, "failed resolving component reference"))
			}
			closer := accessio.OnceCloser(nested)
			defer closer.Close()
			opts, err := opts.For(reference.Digest)
			if err != nil {
				return nil, errors.Wrapf(err, refMsg(reference, state, "failed resolving hasher for existing digest for component reference"))
			}
			calculatedDigest, err = apply(printer.AddGap("  "), state, nested, opts)
			if err != nil {
				return nil, errors.Wrapf(err, refMsg(reference, state, "failed applying to component reference"))
			}
			closer.Close()
		}

		if reference.Digest == nil {
			cd.ComponentReferences[i].Digest = calculatedDigest
		} else {
			if calculatedDigest != nil && !reflect.DeepEqual(reference.Digest, calculatedDigest) {
				return nil, errors.Newf(refMsg(reference, state, "calculated reference digest (%+v) mismatches existing digest (%+v) for", calculatedDigest, reference.Digest))
			}
		}
	}

	blobdigesters := cv.GetContext().BlobDigesters()
	for i, res := range cv.GetResources() {
		acc, err := res.Access()

		if _, ok := opts.SkipAccessTypes[acc.GetKind()]; ok {
			// set the do not sign digest notation on skip-access-type resources
			cd.Resources[i].Digest = metav1.NewExcludeFromSignatureDigest()
			continue
		}
		// special digest notation indicates to not digest the content
		if cd.Resources[i].Digest != nil && reflect.DeepEqual(cd.Resources[i].Digest, metav1.NewExcludeFromSignatureDigest()) {
			continue
		}

		raw := &cd.Resources[i]
		meth, err := acc.AccessMethod(cv)
		if err != nil {
			return nil, errors.Wrapf(err, resMsg(raw, state, "failed creating access for resource"))
		}
		var req []cpi.DigesterType
		if raw.Digest != nil {
			req = []cpi.DigesterType{
				cpi.DigesterType{
					HashAlgorithm:          raw.Digest.HashAlgorithm,
					NormalizationAlgorithm: raw.Digest.NormalisationAlgorithm,
				},
			}
		}
		digest, err := blobdigesters.DetermineDigests(res.Meta().GetType(), opts.Hasher, opts.Registry, meth, req...)
		if err != nil {
			return nil, errors.Wrapf(err, resMsg(raw, state, "failed determining digest for resource"))
		}
		if len(digest) == 0 {
			return nil, errors.Newf(resMsg(raw, state, "no digester accepts resource"))
		}
		if raw.Digest != nil && !reflect.DeepEqual(*raw.Digest, digest[0]) {
			return nil, errors.Newf(resMsg(raw, state, "calculated resource digest (%+v) mismatches existing digest (%+v) for", digest, raw.Digest))
		}
		cd.Resources[i].Digest = &digest[0]
	}
	digest, err := compdesc.Hash(cd, compdesc.JsonNormalisationV1, opts.Hasher.Create())
	if err != nil {
		return nil, errors.Wrapf(err, "failed hashing component descriptor %s ", state.History)
	}
	spec := &metav1.DigestSpec{
		HashAlgorithm:          opts.Hasher.Algorithm(),
		NormalisationAlgorithm: compdesc.JsonNormalisationV1,
		Value:                  digest,
	}

	found := cd.GetSignatureIndex(opts.SignatureName)
	if opts.DoVerify() {
		if found >= 0 {
			pub := opts.PublicKey()
			sig := &cd.Signatures[found]
			verifier := opts.Registry.GetVerifier(sig.Signature.Algorithm)
			if verifier == nil {
				return nil, errors.ErrUnknown(compdesc.KIND_VERIFY_ALGORITHM, sig.Signature.Algorithm, state.History.String())
			}
			err = verifier.Verify(sig.Digest.Value, sig.Signature.Value, sig.Signature.MediaType, pub)
			if err != nil {
				return nil, errors.ErrInvalidWrap(err, compdesc.KIND_SIGNATURE, opts.SignatureName, state.History.String())
			}
		} else {
			if !opts.DoSign() {
				return nil, errors.Newf("signature %q not found in %s", opts.SignatureName, state.History)
			}
		}
	}
	if opts.DoSign() && (!opts.DoVerify() || found == -1) {
		sig, err := opts.Signer.Sign(digest, opts.PrivateKey())
		if err != nil {
			return nil, errors.Wrapf(err, "failed signing component descriptor %s ", state.History)
		}
		signature := metav1.Signature{
			Name:   opts.SignatureName,
			Digest: *spec,
			Signature: metav1.SignatureSpec{
				Algorithm: sig.Algorithm,
				Value:     sig.Value,
				MediaType: sig.MediaType,
			},
		}
		if found >= 0 {
			cd.Signatures[found] = signature
		} else {
			cd.Signatures = append(cd.Signatures, signature)
		}
	}
	if opts.DoUpdate() {
		orig := cv.GetDescriptor()
		for i, res := range cd.Resources {
			orig.Resources[i].Digest = res.Digest
		}
		for i, res := range cd.ComponentReferences {
			orig.ComponentReferences[i].Digest = res.Digest
		}
		if opts.DoSign() {
			orig.Signatures = cd.Signatures
		}
	}
	state.Closure[nv] = spec
	return spec, nil
}

func refMsg(ref compdesc.ComponentReference, state common.WalkingState, msg string, args ...interface{}) string {
	return fmt.Sprintf("%s %q [%s:%s] in %s", fmt.Sprintf(msg, args...), ref.Name, ref.ComponentName, ref.Version, state.History)
}

func resMsg(ref *compdesc.Resource, state common.WalkingState, msg string, args ...interface{}) string {
	return fmt.Sprintf("%s %s:%s in %s", fmt.Sprintf(msg, args...), ref.Name, ref.Version, state.History)
}
