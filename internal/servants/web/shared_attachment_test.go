package web

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	model "github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"github.com/alimy/tryst/cfg"
)

const sharedAttachmentURL = "https://storage.example.test/public/avatar/shared.png"

type sharedAttachmentStorage struct {
	core.ObjectStorageService
	persisted, deleted []string
	putKey             string
}

func (s *sharedAttachmentStorage) ObjectKey(value string) string {
	return strings.TrimPrefix(value, "https://storage.example.test/")
}
func (s *sharedAttachmentStorage) PersistObject(key string) error {
	s.persisted = append(s.persisted, key)
	return nil
}
func (s *sharedAttachmentStorage) DeleteObject(key string) error {
	s.deleted = append(s.deleted, key)
	return nil
}
func (s *sharedAttachmentStorage) DeleteObjects(keys []string) error {
	s.deleted = append(s.deleted, keys...)
	return nil
}
func (s *sharedAttachmentStorage) PutObject(key string, _ io.Reader, _ int64, _ string, _ bool) (string, error) {
	s.putKey = key
	return "https://storage.example.test/" + key, nil
}

type sharedAttachmentData struct {
	core.DataService
	user               *ms.User
	submitted, deleted bool
}

func (*sharedAttachmentData) CheckAttachment(string) error          { return nil }
func (d *sharedAttachmentData) GetUserByID(int64) (*ms.User, error) { return d.user, nil }
func (d *sharedAttachmentData) GetPostByID(int64) (*ms.Post, error) {
	return &ms.Post{Model: &ms.Model{ID: 7}, UserID: d.user.ID, AuditStatus: ms.PostAuditApproved}, nil
}
func (*sharedAttachmentData) GetCourseByID(int64) (*ms.Course, error) {
	return &ms.Course{Model: &ms.Model{ID: 8}}, nil
}
func (d *sharedAttachmentData) SubmitProfileReview(*ms.User, string, string) (*core.ProfileSubmission, error) {
	d.submitted = true
	return nil, errors.New("injected submission failure")
}
func (d *sharedAttachmentData) CreatePostWithContents(*ms.Post, []*ms.PostContent) (*ms.Post, error) {
	d.submitted = true
	return nil, errors.New("injected submission failure")
}
func (d *sharedAttachmentData) CreateCommentWithContents(*ms.Comment, []*ms.CommentContent) (*ms.Comment, error) {
	d.submitted = true
	return nil, errors.New("injected submission failure")
}
func (d *sharedAttachmentData) CreateCourseQuestionWithContents(*ms.CourseComment, []*ms.CourseCommentContent) (*ms.CourseComment, error) {
	d.submitted = true
	return nil, errors.New("injected submission failure")
}
func (d *sharedAttachmentData) DeletePost(*ms.Post) ([]string, error) {
	d.deleted = true
	return []string{sharedAttachmentURL}, nil
}
func (*sharedAttachmentData) CreateAttachment(*ms.Attachment) (int64, error) {
	return 0, errors.New("injected attachment insert failure")
}

type sharedAttachmentSearch struct{ core.TweetSearchService }

func (*sharedAttachmentSearch) DeleteDocuments([]string) error {
	// Stop after the committed content deletion, before asynchronous cache events.
	return errors.New("injected index failure")
}

func TestSharedAttachmentsSurviveContentFailuresAndDeletion(t *testing.T) {
	oldApp, oldAudit, oldProfile := conf.AppSetting, conf.AuditSetting, conf.WebProfileSetting
	t.Cleanup(func() { conf.AppSetting, conf.AuditSetting, conf.WebProfileSetting = oldApp, oldAudit, oldProfile })
	conf.AppSetting = nil
	if err := json.Unmarshal([]byte(`{"MaxCommentCount":100}`), &conf.AppSetting); err != nil {
		t.Fatal(err)
	}
	conf.AuditSetting = &conf.AuditConf{Enabled: true}
	conf.WebProfileSetting = &conf.WebProfileConf{DefaultTweetMaxLength: 2000}
	for _, kind := range []string{"avatar", "post", "comment", "course question", "post deletion"} {
		t.Run(kind, func(t *testing.T) {
			actor := permissionUser(1, "profile.edit", "post.create", "post.view", "comment.create", "course.view")
			ds := &sharedAttachmentData{user: actor}
			storage := &sharedAttachmentStorage{}
			base := &base.DaoServant{Ds: ds, Ts: &sharedAttachmentSearch{}}
			contents := []*model.PostContentItem{{Content: sharedAttachmentURL, Type: ms.ContentTypeImage}}
			var err error
			switch kind {
			case "avatar":
				_, err = (&coreSrv{DaoServant: base, oss: storage}).ChangeAvatar(&model.ChangeAvatarReq{BaseInfo: model.BaseInfo{User: actor}, Avatar: sharedAttachmentURL})
			case "post":
				_, err = (&privSrv{DaoServant: base, oss: storage}).CreateTweet(&model.CreateTweetReq{BaseInfo: model.BaseInfo{User: actor}, Contents: contents, Visibility: model.TweetVisitPrivate})
			case "comment":
				_, err = (&privSrv{DaoServant: base, oss: storage}).CreateComment(&model.CreateCommentReq{SimpleInfo: model.SimpleInfo{Uid: actor.ID}, PostID: 7, Contents: contents})
			case "course question":
				_, err = (&coursePrivSrv{DaoServant: base, oss: storage}).CreateCourseComment(&model.CreateCourseCommentReq{SimpleInfo: model.SimpleInfo{Uid: actor.ID}, CourseID: 8, Contents: contents})
			case "post deletion":
				err = (&privSrv{DaoServant: base, oss: storage}).DeleteTweet(&model.DeleteTweetReq{BaseInfo: model.BaseInfo{User: actor}, ID: 7})
			}
			if err == nil || (!ds.submitted && !ds.deleted) {
				t.Fatalf("expected injected failure after database operation: %v", err)
			}
			if len(storage.deleted) != 0 {
				t.Fatalf("shared object was deleted: %v", storage.deleted)
			}
			if kind != "post deletion" && len(storage.persisted) != 1 {
				t.Fatal("test did not reach attachment persistence")
			}
		})
	}
}

func TestFailedNewUploadCleansOnlyItsGeneratedKey(t *testing.T) {
	oldIf, oldStorage := cfg.If, conf.ObjectStorage
	t.Cleanup(func() { cfg.If, conf.ObjectStorage = oldIf, oldStorage })
	conf.ObjectStorage = nil
	if err := json.Unmarshal([]byte(`{"TempDir":"tmp"}`), &conf.ObjectStorage); err != nil {
		t.Fatal(err)
	}
	for _, temporary := range []bool{false, true} {
		cfg.If = func(expression string) bool {
			if expression == "OSS:TempDir" {
				return temporary
			}
			return oldIf(expression)
		}
		fileName := filepath.Join(t.TempDir(), "new-upload.txt")
		if err := os.WriteFile(fileName, []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(fileName)
		if err != nil {
			t.Fatal(err)
		}
		storage := &sharedAttachmentStorage{}
		s := &privSrv{DaoServant: &base.DaoServant{Ds: &sharedAttachmentData{}}, oss: storage}
		_, err = s.UploadAttachment(&model.UploadAttachmentReq{SimpleInfo: model.SimpleInfo{Uid: 1}, UploadType: "attachment", File: file, FileSize: 4, FileExt: ".txt", ContentType: "text/plain"})
		want := storage.putKey
		if temporary {
			want = "tmp/" + want
		}
		if err == nil || storage.putKey == "" || len(storage.deleted) != 1 || storage.deleted[0] != want {
			t.Fatalf("temporary=%v cleanup=%v want=%q error=%v", temporary, storage.deleted, want, err)
		}
	}
}
