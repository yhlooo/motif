package i18n

import (
	"context"
	"embed"
	"os"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

//go:embed active.*.yaml
var LocaleFS embed.FS

var (
	// defaultBundle 默认 bundle
	defaultBundle = i18n.NewBundle(language.English)
	// defaultLocalizer 默认本地化器
	defaultLocalizer *i18n.Localizer
	// fallbackLocalizer 不使用任何语言的本地化器
	fallbackLocalizer *i18n.Localizer
)

func init() {
	defaultBundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)

	items, _ := LocaleFS.ReadDir(".")
	for _, item := range items {
		_, _ = defaultBundle.LoadMessageFileFS(LocaleFS, item.Name())
	}

	defaultLocalizer = i18n.NewLocalizer(defaultBundle, GetEnvLanguage())
	fallbackLocalizer = i18n.NewLocalizer(defaultBundle)
}

// localizerContextKey 上下文中保存 *i18n.Localizer 的键
type localizerContextKey struct{}

// LocalizerFromContext 从上下文获取 *i18n.Localizer
func LocalizerFromContext(ctx context.Context) *i18n.Localizer {
	l, ok := ctx.Value(localizerContextKey{}).(*i18n.Localizer)
	if !ok {
		return defaultLocalizer
	}
	return l
}

// ContextWithLocalizer 创建包含指定 *i18n.Localizer 的上下文
func ContextWithLocalizer(ctx context.Context, l *i18n.Localizer) context.Context {
	return context.WithValue(ctx, localizerContextKey{}, l)
}

// GetEnvLanguage 获取环境变量中的语言信息
func GetEnvLanguage() string {
	langEnvDivided := strings.SplitN(os.Getenv("LANG"), ".", 2)
	if len(langEnvDivided) < 1 {
		return "en"
	}

	return langEnvDivided[0]
}

// NewLocalizer 创建基于指定语言的本地化器
func NewLocalizer(langs ...string) *i18n.Localizer {
	return i18n.NewLocalizer(defaultBundle, langs...)
}
