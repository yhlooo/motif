package i18n

import (
	"context"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// T 使用默认本地化器本地化消息
func T(defaultMessage *i18n.Message) string {
	return localizeMessage(defaultLocalizer, defaultMessage)
}

// TContext 使用上下文携带的本地化器本地化消息
func TContext(ctx context.Context, defaultMessage *i18n.Message) string {
	return localizeMessage(LocalizerFromContext(ctx), defaultMessage)
}

// TContextWithData 使用上下文携带的本地化器本地化带模板数据的消息
func TContextWithData(ctx context.Context, defaultMessage *i18n.Message, data interface{}) string {
	return LocalizeContext(ctx, &i18n.LocalizeConfig{
		MessageID:      defaultMessage.ID,
		DefaultMessage: defaultMessage,
		TemplateData:   data,
	})
}

// LocalizeMessage 本地化消息
func localizeMessage(localizer *i18n.Localizer, defaultMessage *i18n.Message) string {
	if ret, err := localizer.LocalizeMessage(defaultMessage); err == nil {
		return ret
	}
	if ret, err := fallbackLocalizer.LocalizeMessage(defaultMessage); err == nil {
		return ret
	}
	return defaultMessage.Other
}

// LocalizeContext 使用上下文携带的本地化器本地化消息
func LocalizeContext(ctx context.Context, lc *i18n.LocalizeConfig) string {
	localizer := LocalizerFromContext(ctx)

	if ret, err := localizer.Localize(lc); err == nil {
		return ret
	}
	if ret, err := fallbackLocalizer.Localize(lc); err == nil {
		return ret
	}

	if lc.DefaultMessage != nil {
		return lc.DefaultMessage.Other
	}
	return ""
}
