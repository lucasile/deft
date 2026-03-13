package i18n

import (
	"embed"
	"encoding/json"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localeFS embed.FS

var bundle *i18n.Bundle
var localizer *i18n.Localizer

func Init(lang string) {
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	// Load all embedded JSON files
	files, _ := localeFS.ReadDir("locales")
	for _, f := range files {
		data, _ := localeFS.ReadFile("locales/" + f.Name())
		bundle.MustParseMessageFileBytes(data, f.Name())
	}

	localizer = i18n.NewLocalizer(bundle, lang)
}

func T(id string, data map[string]interface{}) string {
	if localizer == nil {
		return id
	}
	msg, _ := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    id,
		TemplateData: data,
	})
	if msg == "" {
		return id
	}
	return msg
}
