package i18n

import (
	"embed"
	"fmt"
	"sync"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	toml "github.com/pelletier/go-toml/v2"
	"golang.org/x/text/language"
)

//go:embed locales/*.toml
var localeFS embed.FS

const (
	LocaleChinese = "zh-CN"
	LocaleEnglish = "en-US"

	DefaultLocale = LocaleChinese
)

var (
	bundle    *i18n.Bundle
	localizer *i18n.Localizer
	mu        sync.RWMutex
)

// Init 初始化国际化
func Init(lang string) error {
	mu.Lock()
	defer mu.Unlock()

	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", unmarshalToml)

	// 加载中文翻译
	if _, err := bundle.LoadMessageFileFS(localeFS, "locales/active.zh-CN.toml"); err != nil {
		return fmt.Errorf("failed to load zh-CN locale: %w", err)
	}

	// 加载英文翻译
	if _, err := bundle.LoadMessageFileFS(localeFS, "locales/active.en-US.toml"); err != nil {
		return fmt.Errorf("failed to load en-US locale: %w", err)
	}

	// 创建 localizer
	if lang == "" {
		lang = DefaultLocale
	}
	localizer = i18n.NewLocalizer(bundle, lang)

	return nil
}

// Get 获取翻译
func Get(key string) string {
	mu.RLock()
	loc := localizer
	mu.RUnlock()

	if loc == nil {
		if err := Init(DefaultLocale); err != nil {
			// 初始化失败，创建一个空的 localizer 作为 fallback
			bun := i18n.NewBundle(language.English)
			loc = i18n.NewLocalizer(bun, "en")
		} else {
			mu.RLock()
			loc = localizer
			mu.RUnlock()
		}
	}

	msg, err := loc.Localize(&i18n.LocalizeConfig{
		MessageID: key,
	})
	if err != nil {
		return key
	}
	return msg
}

// GetWithParams 获取带参数的翻译
func GetWithParams(key string, params map[string]interface{}) string {
	mu.RLock()
	loc := localizer
	mu.RUnlock()

	if loc == nil {
		if err := Init(DefaultLocale); err != nil {
			// 初始化失败，创建一个空的 localizer 作为 fallback
			bun := i18n.NewBundle(language.English)
			loc = i18n.NewLocalizer(bun, "en")
		} else {
			mu.RLock()
			loc = localizer
			mu.RUnlock()
		}
	}

	msg, err := loc.Localize(&i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: params,
	})
	if err != nil {
		return key
	}
	return msg
}

// unmarshalToml TOML 解析函数
func unmarshalToml(data []byte, v interface{}) error {
	return toml.Unmarshal(data, v)
}
