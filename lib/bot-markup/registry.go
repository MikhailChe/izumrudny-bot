package markup

import (
	"mikhailche/botcomod/tracer"

	tele "gopkg.in/telebot.v3"
)

var (
	HelpMainMenuBtn  = Data("⬅️ Назад в главное меню", "help-main-menu")
	DistrictChatsBtn = Data("💬🏘 Чаты района", "district-chats")

	HelpfulPhonesBtn = Data("☎️ Телефоны", "phone-numbers")

	ResidentsBtn       = Data("🏡 Для резидентов", "authorized-section")
	IntercomCodeBtn    = Data("🔑 Код домофона", "intercom-code")
	VideoCamerasBtn    = Data("📽 Камеры видеонаблюдения", "internal-video-cameras")
	PMWithResidentsBtn = Data("💬 Чат с другими резидентами", "resident-pm")

	RegisterBtn         = Data("📒 Начать регистрацию", "registration")
	ContinueRegisterBtn = Data("📒 Продолжить регистрацию", "registration")
)

func HelpMenuMarkup() *tele.ReplyMarkup {
	defer tracer.Trace("helpMenuMarkup")()
	return InlineMarkup(
		Row(DistrictChatsBtn),
		Row(HelpfulPhonesBtn),
		Row(ResidentsBtn),
		Row(Text("🟢 Без комунальных проблем")),
	)
}
