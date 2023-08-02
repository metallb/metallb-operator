package helm

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetImageNameTag(t *testing.T) {
	tests := []struct {
		input         string
		expectedImage string
		expectedTag   string
	}{
		{
			input:         "quay.io/metallb/speaker:v0.13.9",
			expectedImage: "quay.io/metallb/speaker",
			expectedTag:   "v0.13.9",
		},
		{
			input:         "quay.io:5000/metallb/speaker:v0.13.9",
			expectedImage: "quay.io:5000/metallb/speaker",
			expectedTag:   "v0.13.9",
		},
		{
			input:         "quay.io/metallb/speaker",
			expectedImage: "quay.io/metallb/speaker",
			expectedTag:   "",
		},
		{
			input:         "quay.io:5000/metallb/speaker",
			expectedImage: "quay.io:5000/metallb/speaker",
			expectedTag:   "",
		},
		{
			input:         "speaker:v0.13.9",
			expectedImage: "speaker",
			expectedTag:   "v0.13.9",
		},
		{
			input:         "speaker",
			expectedImage: "speaker",
			expectedTag:   "",
		},
	}

	g := NewGomegaWithT(t)
	for _, test := range tests {
		img, tag := getImageNameTag(test.input)
		g.Expect(img).To(Equal(test.expectedImage))
		g.Expect(tag).To(Equal(test.expectedTag))
	}
}
