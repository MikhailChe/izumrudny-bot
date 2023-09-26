package markup

import (
	"context"
	"github.com/mikhailche/telebot"
	"mikhailche/botcomod/lib/tracer.v2"
)

var (
	HelpMainMenuBtn  = Data("⬅️ Назад в главное меню", "help-main-menu")
	DistrictChatsBtn = Data("💬🏘 Чаты района", "district-chats")

	HelpfulPhonesBtn = Data("☎️ Телефоны", "phone-numbers")

	ResidentsBtn       = Data("🏡 Для резидентов", "authorized-section")
	BackToResidentsBtn = Data("⬅️ Назад в меню для резидентов", "authorized-section")
	IntercomCodeBtn    = Data("🔑 Код домофона", "intercom-code")
	VideoCamerasBtn    = Data("📽 Камеры видеонаблюдения", "internal-video-cameras")
	PMWithResidentsBtn = Data("💬 Чат с другими резидентами", "resident-pm")

	RegisterBtn         = Data("📒 Начать регистрацию", "registration")
	ContinueRegisterBtn = Data("📒 Продолжить регистрацию", "registration")

	ChatGroupAdminBtn = Data("⚙️ Для админов чатов", "chatgroupadmin")
)

func HelpMenuMarkup(ctx context.Context) *telebot.ReplyMarkup {
	ctx, span := tracer.Open(ctx, tracer.Named("helpMenuMarkup"))
	defer span.Close()
	return InlineMarkup(
		Row(DistrictChatsBtn),
		Row(HelpfulPhonesBtn),
		Row(ResidentsBtn),
		Row(Text("🟢 Без комунальных проблем")),
	)
}

func DynamicHelpMenuMarkup(ctx context.Context) *telebot.ReplyMarkup {
	ctx, span := tracer.Open(ctx, tracer.Named("DynamicHelpMenuMarkup"))
	defer span.Close()
	var rows []telebot.Row
	rows = append(
		rows,
		Row(DistrictChatsBtn),
		Row(HelpfulPhonesBtn),
		Row(ResidentsBtn),
		Row(Text("🟢 Без комунальных проблем")),
	)
	return InlineMarkup(rows...)
}
