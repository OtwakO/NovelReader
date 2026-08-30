package sourceprofile

import (
	"encoding/json"

	"github.com/otwako/novelreader/internal/sourceexec"
)

// Settings is NovelReader's portable source-state document.
// Values are opaque to NovelReader and interpreted only by the BookSource program.
type Settings struct {
	Variable string            `json:"variable,omitempty"`
	Values   map[string]string `json:"values,omitempty"`
}

func DecodeSettings(document json.RawMessage) Settings {
	var settings Settings
	if err := json.Unmarshal(document, &settings); err != nil {
		return Settings{}
	}
	return settings
}

func ApplySettings(session *sourceexec.SourceSession, sourceKey string, settings Settings) {
	if session == nil {
		return
	}
	if settings.Variable != "" {
		session.PutVariable(sourceKey, settings.Variable)
	}
	for key, value := range settings.Values {
		session.PutMemory(key, value)
	}
}
