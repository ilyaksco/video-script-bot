package i18n

import (
	"encoding/json"
	"log"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

func NewLocalizer(defaultLang string) *i18n.Localizer {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	if _, err := bundle.LoadMessageFile("internal/locales/en.json"); err != nil {
		log.Printf("Warning: could not load en.json file: %v", err)
	}
	if _, err := bundle.LoadMessageFile("internal/locales/id.json"); err != nil {
		log.Printf("Warning: could not load id.json file: %v", err)
	}

	langTag := language.English
	if defaultLang == "id" {
		langTag = language.Indonesian
	}

	return i18n.NewLocalizer(bundle, langTag.String())
}
