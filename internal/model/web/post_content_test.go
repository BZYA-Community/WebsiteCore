package web

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func TestPostContentBinding(t *testing.T) {
	const textItem = `{"content":"hello","type":2,"sort":0}`
	requests := []struct {
		name       string
		fields     string
		newRequest func() any
	}{
		{name: "post", fields: `"tags":[]`, newRequest: func() any { return new(CreateTweetReq) }},
		{name: "post comment", fields: `"post_id":1`, newRequest: func() any { return new(CreateCommentReq) }},
		{name: "course comment", fields: `"course_id":1`, newRequest: func() any { return new(CreateCourseCommentReq) }},
	}
	cases := []struct {
		name         string
		contents     string
		invalidField string
	}{
		{name: "zero sort", contents: `[` + textItem + `]`},
		{name: "null item", contents: `[null]`, invalidField: "Contents[0]"},
		{name: "mixed items", contents: `[` + textItem + `,null]`, invalidField: "Contents[1]"},
	}
	for _, req := range requests {
		t.Run(req.name, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					body := []byte(`{"users":[],` + req.fields + `,"contents":` + tc.contents + `}`)
					err := binding.JSON.BindBody(body, req.newRequest())
					if tc.invalidField == "" {
						if err != nil {
							t.Fatalf("bind valid contents: %v", err)
						}
						return
					}
					var validationErrors validator.ValidationErrors
					if !errors.As(err, &validationErrors) || len(validationErrors) != 1 ||
						validationErrors[0].StructField() != tc.invalidField || validationErrors[0].Tag() != "required" {
						t.Fatalf("bind error = %v, want a required error for %s", err, tc.invalidField)
					}
				})
			}
		})
	}
}
