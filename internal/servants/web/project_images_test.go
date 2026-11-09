package web

import (
	"errors"
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	model "github.com/BZYA-Community/WebsiteCore/internal/model/web"
)

type projectImageCheck struct{ checked []string }

func (s *projectImageCheck) CheckAttachment(raw string) error {
	s.checked = append(s.checked, raw)
	if !strings.HasPrefix(raw, "https://storage.example.test/public/") {
		return errors.New("not a storage URL")
	}
	return nil
}

func TestProjectImagesSeparateExternalReferencesFromStorage(t *testing.T) {
	for _, raw := range []string{
		"https://images.example.test/project.png",
		"http://images.example.test/project.png?signature=abc",
		"https://storage.example.test.evil.test/public/project.png",
		"https://images.example.test/?url=https://storage.example.test/public/project.png",
		"https://images.example.test/https://storage.example.test/public/project.png",
	} {
		t.Run(raw, func(t *testing.T) {
			checker, storage := &projectImageCheck{}, &sharedAttachmentStorage{}
			images, local, err := projectImages(checker, storage, []string{raw, sharedAttachmentURL})
			if err != nil {
				t.Fatal(err)
			}
			if len(images) != 2 || images[0].URL != raw || images[1].Sort != 1 || len(local) != 1 || local[0].Content != sharedAttachmentURL || len(checker.checked) != 1 {
				t.Fatalf("images=%+v local=%+v checked=%v", images, local, checker.checked)
			}
			if err := persistMediaContents(storage, local); err != nil {
				t.Fatal(err)
			}
			if len(storage.persisted) != 1 || storage.persisted[0] != "public/avatar/shared.png" {
				t.Fatalf("external URL reached storage: %v", storage.persisted)
			}
		})
	}
}

func TestProjectImagesRejectInvalidURLsAndCounts(t *testing.T) {
	for _, raw := range []string{
		"", "image.png", "//images.example.test/image.png", "https:///image.png", "https://:443/image.png",
		"javascript:alert(1)", "data:image/png;base64,AAAA", "ftp://images.example.test/image.png",
		"https://user:pass@images.example.test/image.png", "https://images.example.test/a/../image.png",
		"https://images.example.test/%2e%2e/image.png", "https://images.example.test/a%5c..%5cimage.png",
		"https://images.example.test/a\\image.png", "https://images.example.test/image.png\n",
		"https://images.example.test/a b.png", "https://images.example.test/%zz.png",
		"https://storage.example.test/private/image.png", "http://storage.example.test/public/image.png",
		"https://images.example.test/" + strings.Repeat("a", 8192),
	} {
		t.Run(raw, func(t *testing.T) {
			if _, _, err := projectImages(&projectImageCheck{}, &sharedAttachmentStorage{}, []string{raw}); err == nil {
				t.Fatal("accepted invalid image URL")
			}
		})
	}
	for _, urls := range [][]string{nil, {}, make([]string, 7)} {
		if _, _, err := projectImages(&projectImageCheck{}, &sharedAttachmentStorage{}, urls); err == nil {
			t.Fatalf("accepted %d images", len(urls))
		}
	}
}

// The generic media path still persists every validated attachment; project
// external-reference handling must never weaken unrelated upload endpoints.
func TestGenericMediaPersistenceDoesNotSkipExternalLookingURLs(t *testing.T) {
	storage := &sharedAttachmentStorage{}
	if err := persistMediaContents(storage, []*model.PostContentItem{{Content: "https://external.example/file", Type: ms.ContentTypeAttachment}}); err != nil {
		t.Fatal(err)
	}
	if len(storage.persisted) != 1 {
		t.Fatal("ordinary media persistence silently skipped an attachment")
	}
}

var _ core.AttachmentCheckService = (*projectImageCheck)(nil)
