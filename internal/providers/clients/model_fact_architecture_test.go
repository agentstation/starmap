package clients

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcquisitionAdaptersContainNoModelFamilyPolicy(t *testing.T) {
	forbidden := []string{
		"inferFeatures",
		"applyFeatureRules",
		"getModelGardenModels",
		"createModelGardenModel",
		"claude-3",
		"claude-opus-4",
		"llama-3-",
		"mistral-large",
		"deepseek-r1",
		"qwen2-",
		"gpt-4o-",
		"gemini-1.5",
		"jamba-1-",
	}
	for _, root := range []string{"../anthropic", "../google", "../openai"} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, token := range forbidden {
				if strings.Contains(string(data), token) {
					t.Errorf("%s contains model-family policy token %q", path, token)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", root, err)
		}
	}
}
