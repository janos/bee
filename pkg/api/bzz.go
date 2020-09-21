// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/gorilla/mux"

	"github.com/ethersphere/bee/pkg/collection/entry"
	"github.com/ethersphere/bee/pkg/file"
	"github.com/ethersphere/bee/pkg/file/seekjoiner"
	"github.com/ethersphere/bee/pkg/jsonhttp"
	"github.com/ethersphere/bee/pkg/manifest"
	"github.com/ethersphere/bee/pkg/sctx"
	"github.com/ethersphere/bee/pkg/swarm"
	"github.com/ethersphere/bee/pkg/tracing"
)

func (s *server) bzzDownloadHandler(w http.ResponseWriter, r *http.Request) {
	logger := tracing.NewLoggerWithTraceID(r.Context(), s.Logger)
	targets := r.URL.Query().Get("targets")
	r = r.WithContext(sctx.SetTargets(r.Context(), targets))
	ctx := r.Context()

	nameOrHex := mux.Vars(r)["address"]
	pathVar := mux.Vars(r)["path"]
	pathVar = strings.TrimRight(pathVar, "/")

	address, err := s.resolveNameOrAddress(nameOrHex)
	if err != nil {
		logger.Debugf("bzz download: parse address %s: %v", nameOrHex, err)
		logger.Error("bzz download: parse address")
		jsonhttp.BadRequest(w, "invalid address")
		return
	}

	// this is a hack and is needed because encryption is coupled into manifests
	toDecrypt := len(address.Bytes()) == 64

	// read manifest entry
	j := seekjoiner.NewSimpleJoiner(s.Storer)
	buf := bytes.NewBuffer(nil)
	_, err = file.JoinReadAll(ctx, j, address, buf)
	if err != nil {
		logger.Debugf("bzz download: read entry %s: %v", address, err)
		logger.Errorf("bzz download: read entry %s", address)
		jsonhttp.NotFound(w, nil)
		return
	}
	e := &entry.Entry{}
	err = e.UnmarshalBinary(buf.Bytes())
	if err != nil {
		logger.Debugf("bzz download: unmarshal entry %s: %v", address, err)
		logger.Errorf("bzz download: unmarshal entry %s", address)
		jsonhttp.InternalServerError(w, "error unmarshaling entry")
		return
	}

	// read metadata
	buf = bytes.NewBuffer(nil)
	_, err = file.JoinReadAll(ctx, j, e.Metadata(), buf)
	if err != nil {
		logger.Debugf("bzz download: read metadata %s: %v", address, err)
		logger.Errorf("bzz download: read metadata %s", address)
		jsonhttp.NotFound(w, nil)
		return
	}
	manifestMetadata := &entry.Metadata{}
	err = json.Unmarshal(buf.Bytes(), manifestMetadata)
	if err != nil {
		logger.Debugf("bzz download: unmarshal metadata %s: %v", address, err)
		logger.Errorf("bzz download: unmarshal metadata %s", address)
		jsonhttp.InternalServerError(w, "error unmarshaling metadata")
		return
	}

	// we are expecting manifest Mime type here
	m, err := manifest.NewManifestReference(
		ctx,
		manifestMetadata.MimeType,
		e.Reference(),
		toDecrypt,
		s.Storer,
	)
	if err != nil {
		logger.Debugf("bzz download: not manifest %s: %v", address, err)
		logger.Error("bzz download: not manifest")
		jsonhttp.BadRequest(w, "not manifest")
		return
	}

	if pathVar == "" {
		logger.Tracef("bzz download: handle empty path %s", address)

		indexDocumentSuffixKey := bzzDownloadHandlerManifestRedirect(m, manifestWebsiteIndexDocumentSuffixKey)
		if indexDocumentSuffixKey != "" {
			pathWithIndex := path.Join(pathVar, indexDocumentSuffixKey)
			indexDocumentManifestEntry, err := m.Lookup(pathWithIndex)
			if err == nil {
				// index document exists
				logger.Debugf("bzz download: serving path: %s", pathWithIndex)

				s.bzzDownloadHandlerServeManifestEntry(w, r, ctx, j, address, indexDocumentManifestEntry.Reference())
				return
			}
		}
	}

	me, err := m.Lookup(pathVar)
	if err != nil {
		logger.Debugf("bzz download: invalid path %s/%s: %v", address, pathVar, err)
		logger.Error("bzz download: invalid path")

		if errors.Is(err, manifest.ErrNotFound) {

			// check index path first
			indexDocumentSuffixKey := bzzDownloadHandlerManifestRedirect(m, manifestWebsiteIndexDocumentSuffixKey)
			if indexDocumentSuffixKey != "" {
				if !strings.HasSuffix(pathVar, indexDocumentSuffixKey) {
					// check if path is directory with index
					pathWithIndex := path.Join(pathVar, indexDocumentSuffixKey)
					indexDocumentManifestEntry, err := m.Lookup(pathWithIndex)
					if err == nil {
						// index document exists
						logger.Debugf("bzz download: serving path: %s", pathWithIndex)

						s.bzzDownloadHandlerServeManifestEntry(w, r, ctx, j, address, indexDocumentManifestEntry.Reference())
						return
					}
				}
			}

			// check if error document is to be shown
			errorDocumentPath := bzzDownloadHandlerManifestRedirect(m, manifestWebsiteErrorDocumentPathKey)
			if errorDocumentPath != "" {
				if pathVar != errorDocumentPath {
					errorDocumentManifestEntry, err := m.Lookup(errorDocumentPath)
					if err == nil {
						// error document exists
						logger.Debugf("bzz download: serving path: %s", errorDocumentPath)

						s.bzzDownloadHandlerServeManifestEntry(w, r, ctx, j, address, errorDocumentManifestEntry.Reference())
						return
					}
				}
			}

			jsonhttp.NotFound(w, "path address not found")
		} else {
			jsonhttp.BadRequest(w, "invalid path address")
		}
		return
	}

	// serve requested path
	s.bzzDownloadHandlerServeManifestEntry(w, r, ctx, j, address, me.Reference())
}

func (s *server) bzzDownloadHandlerServeManifestEntry(
	w http.ResponseWriter,
	r *http.Request,
	ctx context.Context,
	j file.JoinSeeker,
	address swarm.Address,
	manifestEntryAddress swarm.Address,
) {
	logger := tracing.NewLoggerWithTraceID(r.Context(), s.Logger)

	// read file entry
	buf := bytes.NewBuffer(nil)
	_, err := file.JoinReadAll(ctx, j, manifestEntryAddress, buf)
	if err != nil {
		logger.Debugf("bzz download: read file entry %s: %v", address, err)
		logger.Errorf("bzz download: read file entry %s", address)
		jsonhttp.NotFound(w, nil)
		return
	}
	fe := &entry.Entry{}
	err = fe.UnmarshalBinary(buf.Bytes())
	if err != nil {
		logger.Debugf("bzz download: unmarshal file entry %s: %v", address, err)
		logger.Errorf("bzz download: unmarshal file entry %s", address)
		jsonhttp.InternalServerError(w, "error unmarshaling file entry")
		return
	}

	// read file metadata
	buf = bytes.NewBuffer(nil)
	_, err = file.JoinReadAll(ctx, j, fe.Metadata(), buf)
	if err != nil {
		logger.Debugf("bzz download: read file metadata %s: %v", address, err)
		logger.Errorf("bzz download: read file metadata %s", address)
		jsonhttp.NotFound(w, nil)
		return
	}
	fileMetadata := &entry.Metadata{}
	err = json.Unmarshal(buf.Bytes(), fileMetadata)
	if err != nil {
		logger.Debugf("bzz download: unmarshal metadata %s: %v", address, err)
		logger.Errorf("bzz download: unmarshal metadata %s", address)
		jsonhttp.InternalServerError(w, "error unmarshaling metadata")
		return
	}

	additionalHeaders := http.Header{
		"Content-Disposition": {fmt.Sprintf("inline; filename=\"%s\"", fileMetadata.Filename)},
		"Content-Type":        {fileMetadata.MimeType},
	}

	fileEntryAddress := fe.Reference()

	s.downloadHandler(w, r, fileEntryAddress, additionalHeaders)
}

func bzzDownloadHandlerManifestRedirect(manifest manifest.Interface, metadataKey string) string {
	// check for root path
	me, err := manifest.Lookup(manifestRootPath)
	if err != nil {
		// ignore missing manifest root path
		return ""
	}

	manifestRootMetadata := me.Metadata()
	if val, ok := manifestRootMetadata[metadataKey]; ok {
		return val
	}

	return ""
}
