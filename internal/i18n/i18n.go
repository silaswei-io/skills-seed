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
	loadedBundle, err := loadBundle()
	if err != nil {
		return err
	}
	if lang == "" {
		lang = DefaultLocale
	}

	mu.Lock()
	defer mu.Unlock()
	bundle = loadedBundle
	localizer = i18n.NewLocalizer(bundle, lang)
	return nil
}

// Get 获取翻译
func Get(key string) string {
	return getWithLocalizer(key, ensureLocalizer(DefaultLocale))
}

// GetForLocale 按指定语言获取翻译，不改变全局语言状态。
func GetForLocale(lang, key string) string {
	return getWithLocalizer(key, newLocalizer(lang))
}

func getWithLocalizer(key string, loc *i18n.Localizer) string {
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
	loc := ensureLocalizer(DefaultLocale)
	msg, err := loc.Localize(&i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: params,
	})
	if err != nil {
		return key
	}
	return msg
}

func ensureLocalizer(lang string) *i18n.Localizer {
	mu.RLock()
	loc := localizer
	mu.RUnlock()

	if loc == nil {
		if err := Init(lang); err != nil {
			// 初始化失败，创建一个空的 localizer 作为 fallback
			bun := i18n.NewBundle(language.English)
			loc = i18n.NewLocalizer(bun, "en")
		} else {
			mu.RLock()
			loc = localizer
			mu.RUnlock()
		}
	}
	return loc
}

func newLocalizer(lang string) *i18n.Localizer {
	if lang == "" {
		lang = DefaultLocale
	}
	mu.RLock()
	bun := bundle
	mu.RUnlock()
	if bun == nil {
		loadedBundle, err := loadBundle()
		if err != nil {
			fallback := i18n.NewBundle(language.English)
			return i18n.NewLocalizer(fallback, "en")
		}
		mu.Lock()
		if bundle == nil {
			bundle = loadedBundle
		}
		bun = bundle
		mu.Unlock()
	}
	return i18n.NewLocalizer(bun, lang)
}

func loadBundle() (*i18n.Bundle, error) {
	loadedBundle := i18n.NewBundle(language.English)
	loadedBundle.RegisterUnmarshalFunc("toml", unmarshalToml)

	// 加载中文翻译
	if _, err := loadedBundle.LoadMessageFileFS(localeFS, "locales/active.zh-CN.toml"); err != nil {
		return nil, fmt.Errorf("failed to load zh-CN locale: %w", err)
	}

	// 加载英文翻译
	if _, err := loadedBundle.LoadMessageFileFS(localeFS, "locales/active.en-US.toml"); err != nil {
		return nil, fmt.Errorf("failed to load en-US locale: %w", err)
	}

	return loadedBundle, nil
}

// unmarshalToml TOML 解析函数
func unmarshalToml(data []byte, v interface{}) error {
	return toml.Unmarshal(data, v)
}
