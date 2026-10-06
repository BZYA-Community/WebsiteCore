package storage

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type failingObjectReader struct{}

func (failingObjectReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestLocalObjectAtomicWriteAndPromotion(t *testing.T) {
	root := t.TempDir()
	writer := &localossCreateServant{savePath: root, domain: "https://storage.test/"}
	s := &localossServant{OssCreateService: writer, savePath: root}
	stage, final := "attachment/course/staging/file.pdf", "attachment/course/resources/file.pdf"
	for _, test := range []struct {
		reader io.Reader
		size   int64
	}{
		{bytes.NewBufferString("short"), 10}, {bytes.NewBufferString("too long"), 2}, {failingObjectReader{}, 10},
	} {
		if _, err := writer.PutObject(stage, test.reader, test.size, "application/pdf", true); err == nil {
			t.Fatal("invalid upload accepted")
		}
		if exists, err := s.IsObjectExist(stage); err != nil || exists {
			t.Fatalf("partial object visible: %v,%v", exists, err)
		}
	}
	payload := []byte("%PDF-1.7\nverified fixture")
	if _, err := writer.PutObject(stage, bytes.NewReader(payload), int64(len(payload)), "application/pdf", true); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.PutObject(stage, bytes.NewReader([]byte("overwrite")), 9, "application/pdf", true); !errors.Is(err, os.ErrExist) {
		t.Fatalf("overwrote immutable stage: %v", err)
	}
	meta, err := s.InspectObject(stage)
	if err != nil || meta.Size != int64(len(payload)) || meta.ContentType != "application/pdf" || !bytes.Equal(meta.Header, payload) {
		t.Fatalf("inspection: %+v,%v", meta, err)
	}
	if err := s.PromoteObject(stage, final, "wrong version"); err == nil {
		t.Fatal("accepted changed source")
	}
	if exists, err := s.IsObjectExist(final); err != nil || exists {
		t.Fatalf("failed verification published: %v,%v", exists, err)
	}
	for i := 0; i < 2; i++ {
		if err := s.PromoteObject(stage, final, meta.ETag); err != nil {
			t.Fatalf("idempotent promotion: %v", err)
		}
	}
	original, _ := os.Stat(filepath.Join(root, stage))
	published, _ := os.Stat(filepath.Join(root, final))
	if !os.SameFile(original, published) {
		t.Fatal("large-object promotion copied rather than linked")
	}
	other := "attachment/course/staging/other.pdf"
	if _, err := writer.PutObject(other, bytes.NewReader(payload), int64(len(payload)), "application/pdf", true); err != nil {
		t.Fatal(err)
	}
	otherMeta, err := s.InspectObject(other)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PromoteObject(other, final, otherMeta.ETag); err == nil {
		t.Fatal("unrelated source replaced final")
	}
	if err := s.DeleteObject(stage); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, final))
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("cleanup damaged published file: %v", err)
	}
	leftovers, err := filepath.Glob(filepath.Join(root, "attachment/course/staging/.upload-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary files leaked: %v,%v", leftovers, err)
	}
}

func TestLocalTwoGiBPromotionUsesFileMetadata(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "large.bin")
	f, err := os.Create(stage)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(2 << 30); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	s := &localossServant{savePath: root}
	meta, err := s.InspectObject("large.bin")
	if err != nil || meta.Size != 2<<30 || len(meta.Header) != 512 {
		t.Fatalf("2GiB metadata: %+v,%v", meta, err)
	}
	if err := s.PromoteObject("large.bin", "published.bin", meta.ETag); err != nil {
		t.Fatal(err)
	}
	a, _ := os.Stat(stage)
	b, _ := os.Stat(filepath.Join(root, "published.bin"))
	if !os.SameFile(a, b) || b.Size() != 2<<30 {
		t.Fatal("2GiB object was not atomically linked")
	}
}

func TestLocalTemporaryPersistenceDoesNotOverwrite(t *testing.T) {
	root := t.TempDir()
	w := &localossCreateTempDirServant{savePath: root, tempDir: "tmp/"}
	payload := []byte("verified bytes")
	if _, err := w.PutObject("public/image/file.png", bytes.NewReader(payload), int64(len(payload)), "image/png", false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := w.PersistObject("public/image/file.png"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "tmp/public/image/file.png")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary object retained: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "public/image/file.png")); err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("persistence lost data: %q,%v", got, err)
	}
}

func TestLocalObjectRejectsSymlinksAndAlternateStreams(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if _, err := jailPath(root, "image/file.png:payload"); err == nil {
		t.Fatal("accepted NTFS alternate stream")
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if f, err := OpenLocalObject(root, "linked/secret.txt"); err == nil {
		f.Close()
		t.Fatal("opened file outside root")
	}
	if _, err := jailPath(root, "linked/new-file"); err == nil {
		t.Fatal("write follows directory link")
	}
}
