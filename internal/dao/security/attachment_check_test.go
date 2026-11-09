package security

import "testing"

func TestAttachmentCheckKeepsStorageBoundary(t *testing.T) {
	checker := &attachmentCheckServant{domain: "https://storage.example.test/oss/public/"}
	for _, test := range []struct {
		uri string
		ok  bool
	}{
		{"https://storage.example.test/oss/public/image.png", true},
		{"https://storage.example.test/oss/public/image%20name.png", true},
		{"https://external.example/image.png", false},
		{"https://storage.example.test.evil.test/oss/public/image.png", false},
		{"https://storage.example.test@evil.test/oss/public/image.png", false},
		{"http://storage.example.test/oss/public/image.png", false},
		{"https://storage.example.test/oss/publicity/image.png", false},
		{"https://storage.example.test/other/image.png", false},
		{"https://storage.example.test/oss/public/../secret", false},
		{"https://storage.example.test/oss/public/%2e%2e/secret", false},
		{"https://storage.example.test/oss/public/a%2f..%2fsecret", false},
		{"https://storage.example.test/oss/public/a%5c..%5csecret", false},
		{"https://storage.example.test/oss/public/a\\..\\secret", false},
		{"https://storage.example.test/oss/public/image.png?key=other", false},
		{"https://storage.example.test/oss/public/image.png#fragment", false},
		{"https://storage.example.test/oss/public/%zz", false},
	} {
		t.Run(test.uri, func(t *testing.T) {
			if err := checker.CheckAttachment(test.uri); (err == nil) != test.ok {
				t.Fatalf("CheckAttachment() = %v, want accepted=%v", err, test.ok)
			}
		})
	}
	if err := (&attachmentCheckServant{}).CheckAttachment("https://external.example/image.png"); err == nil {
		t.Fatal("empty storage domain accepted attachment")
	}
}
