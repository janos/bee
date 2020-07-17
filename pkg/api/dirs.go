// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/ethersphere/bee/pkg/jsonhttp"
	"github.com/ethersphere/bee/pkg/logging"
	"github.com/ethersphere/bee/pkg/manifest/jsonmanifest"
	"github.com/ethersphere/bee/pkg/storage"
	"github.com/ethersphere/bee/pkg/swarm"
)

type dirUploadResponse struct {
	Reference swarm.Address `json:"reference"`
}

// dirUploadHandler uploads a directory supplied as a tar in an HTTP Request
func (s *server) dirUploadHandler(w http.ResponseWriter, r *http.Request) {
	dirInfo, err := getDirHTTPInfo(r)
	if err != nil {
		s.Logger.Errorf("dir upload, get dir info: %v", err)
		jsonhttp.BadRequest(w, "could not extract dir info from request")
		return
	}

	reference, err := storeDir(r.Context(), dirInfo, s.Storer, s.Logger)
	if err != nil {
		s.Logger.Errorf("dir upload, store dir: %v", err)
		jsonhttp.InternalServerError(w, "could not store dir")
		return
	}

	jsonhttp.OK(w, dirUploadResponse{
		Reference: reference,
	})
}

// dirUploadInfo contains the data for a directory to be uploaded
type dirUploadInfo struct {
	dirReader io.ReadCloser
	toEncrypt bool
}

// getDirHTTPInfo extracts data for a directory to be uploaded from an HTTP request
func getDirHTTPInfo(r *http.Request) (*dirUploadInfo, error) {
	toEncrypt := strings.ToLower(r.Header.Get(encryptHeader)) == "true"
	return &dirUploadInfo{
		dirReader: r.Body,
		toEncrypt: toEncrypt,
	}, nil
}

// storeDir stores all files contained in the given directory as a tar and returns its reference
func storeDir(ctx context.Context, dirInfo *dirUploadInfo, s storage.Storer, logger logging.Logger) (swarm.Address, error) {
	manifest := jsonmanifest.NewManifest()

	bodyReader := dirInfo.dirReader
	tr := tar.NewReader(bodyReader)
	defer bodyReader.Close()

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return swarm.ZeroAddress, fmt.Errorf("read tar stream error: %v", err)
		}

		// only store regular files
		if !hdr.FileInfo().Mode().IsRegular() {
			continue
		}

		path := hdr.Name
		fileName := hdr.FileInfo().Name()

		contentType := hdr.PAXRecords["SCHILY.xattr."+"user.swarm.content-type"]
		if contentType == "" {
			contentType = mime.TypeByExtension(filepath.Ext(hdr.Name))
		}

		fileInfo := &fileUploadInfo{
			fileName:    fileName,
			fileSize:    hdr.FileInfo().Size(),
			contentType: contentType,
			toEncrypt:   dirInfo.toEncrypt,
			reader:      tr,
		}
		fileReference, err := storeFile(ctx, fileInfo, s)
		if err != nil {
			return swarm.ZeroAddress, fmt.Errorf("store dir file error: %v", err)
		}

		headers := http.Header{}
		headers.Set("Content-Type", contentType)
		entry := &jsonmanifest.JSONEntry{
			Reference: fileReference,
			Name:      fileName,
			Headers:   headers,
		}
		manifest.Add(path, entry)

		logger.Infof("path: %v", path)
		logger.Infof("fileName: %v", fileName)
		logger.Infof("filInfoSize: %v", hdr.FileInfo().Size())
		logger.Infof("fileSize: %v", hdr.Size)
		logger.Infof("contentType: %v", contentType)
		logger.Infof("fileReference: %v", fileReference)
	}

	b, err := manifest.Serialize()
	if err != nil {
		return swarm.ZeroAddress, fmt.Errorf("manifest serialize error: %v", err)
	}
	r := bytes.NewReader(b)
	manifestFileInfo := &fileUploadInfo{
		fileName:    "manifest.json",
		fileSize:    r.Size(),
		contentType: ManifestContentType,
		toEncrypt:   dirInfo.toEncrypt,
		reader:      r,
	}
	manifestReference, err := storeFile(ctx, manifestFileInfo, s)
	if err != nil {
		return swarm.ZeroAddress, fmt.Errorf("store manifest error: %v", err)
	}

	return manifestReference, nil
}
