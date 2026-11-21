/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func writeTestArchive(t *testing.T, path string, files map[string]string) {
	t.Helper()

	file, err := os.Create(filepath.Clean(path))
	if err != nil {
		t.Fatalf("creating archive: %s", err)
	}

	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)

	for name, content := range files {
		header := &tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     0o600,
			Size:     int64(len(content)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("writing tar header: %s", err)
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatalf("writing tar content: %s", err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("closing tar writer: %s", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("closing gzip writer: %s", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("closing archive: %s", err)
	}
}

func TestUnTarContent(t *testing.T) {
	t.Run("extracts the files into the destination", func(t *testing.T) {
		tmp := t.TempDir()
		archive := filepath.Join(tmp, "chart.tgz")
		dst := filepath.Join(tmp, "dst")
		writeTestArchive(t, archive, map[string]string{"chart/Chart.yaml": "name: chart\n"})

		assert.NoError(t, unTarContent(archive, dst))

		content, err := os.ReadFile(filepath.Clean(filepath.Join(dst, "chart", "Chart.yaml")))
		assert.NoError(t, err)
		assert.Equal(t, "name: chart\n", string(content))
	})

	t.Run("rejects entries that would be written outside of the destination", func(t *testing.T) {
		tmp := t.TempDir()
		archive := filepath.Join(tmp, "evil.tgz")
		dst := filepath.Join(tmp, "dst")
		writeTestArchive(t, archive, map[string]string{"../escaped.txt": "x"})

		assert.Error(t, unTarContent(archive, dst))
		assert.NoFileExists(t, filepath.Join(tmp, "escaped.txt"))
	})
}
